package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
)

// Đánh giá dịch vụ.
//
// Khách chấm điểm 1–5 kèm nhận xét, gắn với máy đang ngồi và (tuỳ chọn) một đơn
// hàng cụ thể.
//
// Một quyết định về an toàn dữ liệu: mã máy KHÔNG lấy từ request. Hội viên gửi
// đánh giá thì máy được suy ra từ phiên đang chạy của chính họ — nếu tin vào
// request thì ai cũng gán được nhận xét xấu cho máy bất kỳ, và báo cáo theo máy
// trở thành vô nghĩa.

type FeedbackService struct {
	db    *gorm.DB
	audit *AuditService
}

func NewFeedbackService(db *gorm.DB, audit *AuditService) *FeedbackService {
	return &FeedbackService{db: db, audit: audit}
}

type CreateFeedbackRequest struct {
	// 1–5. Không dùng "required" vì validator coi 0 là thiếu dữ liệu; min=1 đã
	// loại 0 rồi mà thông báo lỗi lại đúng nghĩa hơn.
	Rating  int    `json:"rating" binding:"min=1,max=5"`
	Content string `json:"content"`
	OrderID string `json:"order_id"`
	// Chỉ nhân viên gửi hộ mới được chỉ định máy; hội viên tự gửi thì bỏ qua.
	MachineID string `json:"machine_id"`
}

type FeedbackResponse struct {
	model.ServiceFeedback
	MachineCode    string `json:"machine_code"`
	MemberName     string `json:"member_name,omitempty"`
	MemberUsername string `json:"member_username,omitempty"`
	OrderCode      string `json:"order_code,omitempty"`
}

var ErrAlreadyRated = errors.New("đơn hàng này đã được đánh giá")

// Create ghi nhận một đánh giá.
//
// memberID rỗng nghĩa là nhân viên gửi hộ khách vãng lai; khi đó machineID lấy
// từ request vì không có phiên nào để suy ra.
func (s *FeedbackService) Create(req *CreateFeedbackRequest, memberID string) (*FeedbackResponse, error) {
	if !FeatureEnabled(s.db, "feedback_enabled") {
		return nil, errors.New("quán đang tắt tính năng đánh giá")
	}
	if req.Rating < 1 || req.Rating > 5 {
		return nil, errors.New("điểm đánh giá phải từ 1 đến 5")
	}

	machineID := strings.TrimSpace(req.MachineID)
	var memberPtr *string

	if memberID != "" {
		memberPtr = &memberID
		// Suy máy từ phiên đang chạy, không tin request.
		var session model.MachineSession
		err := s.db.Select("machine_id").
			Where("member_id = ? AND is_active = ?", memberID, true).
			Order("started_at desc").First(&session).Error
		if err != nil {
			return nil, errors.New("bạn cần đang trong phiên chơi để đánh giá")
		}
		machineID = session.MachineID
	}

	if machineID == "" {
		return nil, errors.New("thiếu mã máy")
	}
	// Unscoped: máy bị xoá là xoá MỀM. Khách đang ngồi ở một máy vừa bị quầy gỡ
	// khỏi danh sách vẫn phải gửi được đánh giá — đánh giá là ghi nhận việc đã
	// xảy ra, không phải thao tác trên máy còn sống.
	var machine model.Machine
	if err := s.db.Unscoped().Select("id, machine_code").First(&machine, "id = ?", machineID).Error; err != nil {
		return nil, errors.New("không tìm thấy máy")
	}

	var orderPtr *string
	if id := strings.TrimSpace(req.OrderID); id != "" {
		var order model.Order
		if err := s.db.Select("id, member_id").First(&order, "id = ?", id).Error; err != nil {
			return nil, errors.New("không tìm thấy đơn hàng")
		}
		// Không cho đánh giá đơn của người khác.
		if memberID != "" && (order.MemberID == nil || *order.MemberID != memberID) {
			return nil, errors.New("đơn hàng này không phải của bạn")
		}
		orderPtr = &order.ID
	}

	fb := model.ServiceFeedback{
		MachineID: machineID,
		MemberID:  memberPtr,
		OrderID:   orderPtr,
		Rating:    req.Rating,
		Content:   strings.TrimSpace(req.Content),
		CreatedAt: time.Now(),
	}
	if err := s.db.Create(&fb).Error; err != nil {
		// Ràng buộc duy nhất trên order_id chặn đánh giá cùng một đơn hai lần.
		if isUniqueViolation(err) {
			return nil, ErrAlreadyRated
		}
		return nil, err
	}

	s.audit.Log(&LogAuditRequest{
		Action: "service_feedback", EntityType: "machine", EntityID: machineID,
		Metadata: map[string]interface{}{
			"rating": fb.Rating, "has_content": fb.Content != "",
		},
	})

	return &FeedbackResponse{ServiceFeedback: fb, MachineCode: machine.MachineCode}, nil
}

type FeedbackListRequest struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	MachineID string `form:"machine_id"`
	MinRating int    `form:"min_rating"`
	MaxRating int    `form:"max_rating"`
	DateFrom  string `form:"date_from"`
	DateTo    string `form:"date_to"`
	// Chỉ lấy đánh giá có viết nhận xét — cái đáng đọc nằm ở đây.
	WithContent bool `form:"with_content"`
}

func (s *FeedbackService) List(req *FeedbackListRequest) (*pagination.Result, error) {
	page, size := req.Page, req.PageSize
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}

	q := s.db.Model(&model.ServiceFeedback{})
	if req.MachineID != "" {
		q = q.Where("machine_id = ?", req.MachineID)
	}
	if req.MinRating > 0 {
		q = q.Where("rating >= ?", req.MinRating)
	}
	if req.MaxRating > 0 {
		q = q.Where("rating <= ?", req.MaxRating)
	}
	if req.WithContent {
		q = q.Where("content <> ''")
	}
	if from, err := parseDateOnly(req.DateFrom); err == nil && from != nil {
		q = q.Where("created_at >= ?", *from)
	}
	if to, err := parseDateOnly(req.DateTo); err == nil && to != nil {
		q = q.Where("created_at < ?", to.AddDate(0, 0, 1))
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []model.ServiceFeedback
	if err := q.Order("created_at desc").Offset((page - 1) * size).Limit(size).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	return &pagination.Result{
		Items: s.enrich(rows), Total: total, Page: page, PageSize: size,
	}, nil
}

// enrich gắn mã máy, tên hội viên và mã đơn vào từng dòng, mỗi loại một truy vấn.
func (s *FeedbackService) enrich(rows []model.ServiceFeedback) []FeedbackResponse {
	out := make([]FeedbackResponse, 0, len(rows))
	if len(rows) == 0 {
		return out
	}

	machineIDs := make([]string, 0, len(rows))
	memberIDs := make([]string, 0)
	orderIDs := make([]string, 0)
	for _, r := range rows {
		machineIDs = append(machineIDs, r.MachineID)
		if r.MemberID != nil {
			memberIDs = append(memberIDs, *r.MemberID)
		}
		if r.OrderID != nil {
			orderIDs = append(orderIDs, *r.OrderID)
		}
	}

	machines := map[string]string{}
	var ms []struct{ ID, MachineCode string }
	// Unscoped: đánh giá là chứng từ về việc đã xảy ra. Máy gỡ khỏi danh sách
	// sau đó vẫn phải hiện mã, nếu không cột "Máy" trong danh sách đánh giá cũ
	// trống trơn — đúng chỗ quán cần để biết máy nào hay bị chê.
	s.db.Unscoped().Model(&model.Machine{}).Where("id IN ?", machineIDs).Find(&ms)
	for _, m := range ms {
		machines[m.ID] = m.MachineCode
	}

	members := map[string]struct{ FullName, Username string }{}
	if len(memberIDs) > 0 {
		var mem []struct{ ID, FullName, Username string }
		s.db.Model(&model.Member{}).Where("id IN ?", memberIDs).Find(&mem)
		for _, m := range mem {
			members[m.ID] = struct{ FullName, Username string }{m.FullName, m.Username}
		}
	}

	orders := map[string]string{}
	if len(orderIDs) > 0 {
		var os []struct{ ID, OrderCode string }
		s.db.Model(&model.Order{}).Where("id IN ?", orderIDs).Find(&os)
		for _, o := range os {
			orders[o.ID] = o.OrderCode
		}
	}

	for _, r := range rows {
		item := FeedbackResponse{ServiceFeedback: r, MachineCode: machines[r.MachineID]}
		if r.MemberID != nil {
			if m, ok := members[*r.MemberID]; ok {
				item.MemberName, item.MemberUsername = m.FullName, m.Username
			}
		}
		if r.OrderID != nil {
			item.OrderCode = orders[*r.OrderID]
		}
		out = append(out, item)
	}
	return out
}

type FeedbackSummary struct {
	Total        int64            `json:"total"`
	Average      float64          `json:"average"`
	Distribution map[string]int64 `json:"distribution"`
	WithContent  int64            `json:"with_content"`
}

// Summary tổng hợp điểm trung bình và phân bố theo từng mức sao.
func (s *FeedbackService) Summary(req *FeedbackListRequest) (*FeedbackSummary, error) {
	q := s.db.Model(&model.ServiceFeedback{})
	if req.MachineID != "" {
		q = q.Where("machine_id = ?", req.MachineID)
	}
	if from, err := parseDateOnly(req.DateFrom); err == nil && from != nil {
		q = q.Where("created_at >= ?", *from)
	}
	if to, err := parseDateOnly(req.DateTo); err == nil && to != nil {
		q = q.Where("created_at < ?", to.AddDate(0, 0, 1))
	}

	var rows []struct {
		Rating int
		N      int64
	}
	if err := q.Select("rating, count(*) as n").Group("rating").Find(&rows).Error; err != nil {
		return nil, err
	}

	// Phân bố luôn có đủ 5 mức, kể cả mức chưa ai chấm — biểu đồ thiếu cột sẽ
	// khiến "không ai chấm 1 sao" trông giống "chưa có dữ liệu".
	res := &FeedbackSummary{Distribution: map[string]int64{}}
	for i := 1; i <= 5; i++ {
		res.Distribution[fmt.Sprintf("%d", i)] = 0
	}
	var sum int64
	for _, r := range rows {
		if r.Rating < 1 || r.Rating > 5 {
			continue
		}
		res.Distribution[fmt.Sprintf("%d", r.Rating)] = r.N
		res.Total += r.N
		sum += int64(r.Rating) * r.N
	}
	if res.Total > 0 {
		res.Average = float64(sum) / float64(res.Total)
	}

	countQ := s.db.Model(&model.ServiceFeedback{}).Where("content <> ''")
	if req.MachineID != "" {
		countQ = countQ.Where("machine_id = ?", req.MachineID)
	}
	countQ.Count(&res.WithContent)
	return res, nil
}

func parseDateOnly(s string) (*time.Time, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
