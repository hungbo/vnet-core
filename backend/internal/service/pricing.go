package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/utils"
	"gorm.io/gorm"
)

// Bảng giá.
//
// Máy tính tiền ở session.go chọn giá theo ba tầng, tầng trước thắng tầng sau:
//
//	1. giá riêng cho hạng hội viên  (machine_prices)
//	2. giá theo khung giờ           (time_based_pricings)
//	3. giá cơ bản của nhóm máy      (machine_groups.price_per_hour)
//
// Hai bảng đầu tồn tại từ đầu và máy tính tiền ĐÃ đọc chúng, nhưng không có
// CRUD, không route, không seed — nghĩa là chỉ chèn tay vào database mới có
// giá. Tệp này bổ sung đường ghi còn thiếu.

type PricingService struct {
	db    *gorm.DB
	audit *AuditService
}

func NewPricingService(db *gorm.DB, audit *AuditService) *PricingService {
	return &PricingService{db: db, audit: audit}
}

// --- giá theo hạng hội viên -------------------------------------------------

type MachinePriceRequest struct {
	MachineGroupID string `json:"machine_group_id" binding:"required"`
	MemberGroupID  string `json:"member_group_id" binding:"required"`
	PricePerHour   int64  `json:"price_per_hour" binding:"gt=0"`
	MinDuration    int    `json:"min_duration"`
	EffectiveFrom  string `json:"effective_from"`
	EffectiveTo    string `json:"effective_to"`
}

type MachinePriceResponse struct {
	model.MachinePrice
	MemberGroupName string `json:"member_group_name"`
	// IsCurrent cho biết dòng nào đang thực sự được máy tính tiền dùng hôm nay.
	// Nhiều dòng chồng khoảng hiệu lực là hợp lệ (đặt giá mới cho tháng sau mà
	// không phải đóng dòng cũ) — dòng có effective_from mới nhất thắng, đúng
	// như truy vấn ở session.go. Không đánh dấu thì nhân viên không biết giá
	// nào đang chạy.
	IsCurrent bool `json:"is_current"`
}

func (s *PricingService) ListMachinePrices(machineGroupID string) ([]MachinePriceResponse, error) {
	q := s.db.Model(&model.MachinePrice{})
	if machineGroupID != "" {
		q = q.Where("machine_group_id = ?", machineGroupID)
	}
	var rows []model.MachinePrice
	if err := q.Order("member_group_id, effective_from desc").Find(&rows).Error; err != nil {
		return nil, err
	}

	names := map[string]string{}
	var groups []struct{ ID, Name string }
	s.db.Model(&model.MemberGroup{}).Find(&groups)
	for _, g := range groups {
		names[g.ID] = g.Name
	}

	today := utils.VietnamTime().Format("2006-01-02")
	// Dòng đang hiệu lực cho mỗi cặp (nhóm máy, hạng hội viên): dòng đầu tiên
	// trong danh sách đã sắp giảm dần theo effective_from mà còn trong hạn.
	seen := map[string]bool{}

	out := make([]MachinePriceResponse, 0, len(rows))
	for _, r := range rows {
		// Driver trả cột date thành timestamp đầy đủ ("2026-08-22T00:00:00Z"),
		// nên so chuỗi với "2026-08-22" luôn sai, và ô chọn ngày trên giao diện
		// cũng không đọc được. Cắt về dạng ngày trước khi dùng và trước khi trả.
		r.EffectiveFrom = dateOnly(r.EffectiveFrom)
		if r.EffectiveTo != nil {
			d := dateOnly(*r.EffectiveTo)
			r.EffectiveTo = &d
		}
		item := MachinePriceResponse{MachinePrice: r}
		if r.MemberGroupID != nil {
			item.MemberGroupName = names[*r.MemberGroupID]
		}
		key := deref(r.MachineGroupID) + "|" + deref(r.MemberGroupID)
		if !seen[key] && r.EffectiveFrom <= today &&
			(r.EffectiveTo == nil || *r.EffectiveTo >= today) {
			item.IsCurrent = true
			seen[key] = true
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *PricingService) CreateMachinePrice(req *MachinePriceRequest, actorID string) (*model.MachinePrice, error) {
	from, to, err := s.validateWindow(req.EffectiveFrom, req.EffectiveTo)
	if err != nil {
		return nil, err
	}
	if err := s.checkGroups(req.MachineGroupID, req.MemberGroupID); err != nil {
		return nil, err
	}
	if req.PricePerHour <= 0 {
		return nil, errors.New("giá mỗi giờ phải lớn hơn 0")
	}

	row := model.MachinePrice{
		MachineGroupID: &req.MachineGroupID,
		MemberGroupID:  &req.MemberGroupID,
		PricePerHour:   req.PricePerHour,
		MinDuration:    req.MinDuration,
		EffectiveFrom:  from,
		EffectiveTo:    optionalDate(to),
	}
	if row.MinDuration < 0 {
		row.MinDuration = 0
	}
	if err := s.db.Create(&row).Error; err != nil {
		return nil, err
	}
	s.log("create_machine_price", row.ID, actorID, map[string]interface{}{
		"machine_group_id": req.MachineGroupID, "member_group_id": req.MemberGroupID,
		"price_per_hour": req.PricePerHour, "effective_from": from,
	})
	return &row, nil
}

func (s *PricingService) UpdateMachinePrice(id string, req *MachinePriceRequest, actorID string) (*model.MachinePrice, error) {
	var row model.MachinePrice
	if err := s.db.First(&row, "id = ?", id).Error; err != nil {
		return nil, errors.New("không tìm thấy dòng giá")
	}
	from, to, err := s.validateWindow(req.EffectiveFrom, req.EffectiveTo)
	if err != nil {
		return nil, err
	}
	if req.PricePerHour <= 0 {
		return nil, errors.New("giá mỗi giờ phải lớn hơn 0")
	}

	updates := map[string]interface{}{
		"price_per_hour": req.PricePerHour,
		"min_duration":   req.MinDuration,
		"effective_from": from,
		"effective_to":   optionalDate(to),
	}
	if err := s.db.Model(&row).Updates(updates).Error; err != nil {
		return nil, err
	}
	s.log("update_machine_price", id, actorID, updates)
	return &row, nil
}

// DeleteMachinePrice xoá một dòng bảng giá theo hạng.
//
// Không bảng nào tham chiếu tới machine_prices nên không cần kiểm phụ thuộc —
// đã rà toàn bộ internal/model. Chỉ cần đọc trước để ID sai trả 404 thay vì
// báo thành công như bản cũ.
func (s *PricingService) DeleteMachinePrice(id, actorID string) error {
	var row model.MachinePrice
	if err := s.db.First(&row, "id = ?", id).Error; err != nil {
		return errors.New("không tìm thấy dòng giá")
	}
	if err := s.db.Delete(&row).Error; err != nil {
		return err
	}
	s.log("delete_machine_price", id, actorID, nil)
	return nil
}

// --- giá theo khung giờ -----------------------------------------------------

type TimePricingRequest struct {
	MachineGroupID string `json:"machine_group_id" binding:"required"`
	DayOfWeek      int    `json:"day_of_week" binding:"min=0,max=6"`
	StartTime      string `json:"start_time" binding:"required"`
	EndTime        string `json:"end_time" binding:"required"`
	PricePerHour   int64  `json:"price_per_hour" binding:"gt=0"`
	IsActive       *bool  `json:"is_active"`
}

func (s *PricingService) ListTimePricing(machineGroupID string) ([]model.TimeBasedPricing, error) {
	q := s.db.Model(&model.TimeBasedPricing{})
	if machineGroupID != "" {
		q = q.Where("machine_group_id = ?", machineGroupID)
	}
	var rows []model.TimeBasedPricing
	if err := q.Order("day_of_week, start_time").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *PricingService) CreateTimePricing(req *TimePricingRequest, actorID string) (*model.TimeBasedPricing, error) {
	if err := s.validateTimePricing(req, ""); err != nil {
		return nil, err
	}
	row := model.TimeBasedPricing{
		MachineGroupID: &req.MachineGroupID,
		DayOfWeek:      req.DayOfWeek,
		StartTime:      clockHHMM(req.StartTime),
		EndTime:        clockHHMM(req.EndTime),
		PricePerHour:   req.PricePerHour,
		IsActive:       req.IsActive == nil || *req.IsActive,
	}
	if err := s.db.Create(&row).Error; err != nil {
		return nil, err
	}
	s.log("create_time_pricing", row.ID, actorID, map[string]interface{}{
		"machine_group_id": req.MachineGroupID, "day_of_week": req.DayOfWeek,
		"window": row.StartTime + "-" + row.EndTime, "price_per_hour": req.PricePerHour,
	})
	return &row, nil
}

func (s *PricingService) UpdateTimePricing(id string, req *TimePricingRequest, actorID string) (*model.TimeBasedPricing, error) {
	var row model.TimeBasedPricing
	if err := s.db.First(&row, "id = ?", id).Error; err != nil {
		return nil, errors.New("không tìm thấy khung giá")
	}
	if err := s.validateTimePricing(req, id); err != nil {
		return nil, err
	}
	updates := map[string]interface{}{
		"day_of_week":    req.DayOfWeek,
		"start_time":     clockHHMM(req.StartTime),
		"end_time":       clockHHMM(req.EndTime),
		"price_per_hour": req.PricePerHour,
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if err := s.db.Model(&row).Updates(updates).Error; err != nil {
		return nil, err
	}
	s.log("update_time_pricing", id, actorID, updates)
	return &row, nil
}

// DeleteTimePricing xoá một khung giá theo giờ.
//
// Không bảng nào tham chiếu tới time_based_pricings. Đọc trước để ID sai trả
// 404 thay vì báo thành công.
func (s *PricingService) DeleteTimePricing(id, actorID string) error {
	var row model.TimeBasedPricing
	if err := s.db.First(&row, "id = ?", id).Error; err != nil {
		return errors.New("không tìm thấy khung giá")
	}
	if err := s.db.Delete(&row).Error; err != nil {
		return err
	}
	s.log("delete_time_pricing", id, actorID, nil)
	return nil
}

// validateTimePricing chặn khung giờ CHỒNG NHAU trong cùng một ngày của cùng
// nhóm máy.
//
// Khác với bảng giá theo hạng (chồng khoảng ngày là hợp lệ, dòng mới nhất
// thắng), truy vấn khung giờ ở session.go dùng First() KHÔNG sắp xếp — hai
// khung chồng nhau sẽ cho ra giá nào tuỳ database trả về trước. Không xác định
// được thì phải chặn ngay lúc nhập.
func (s *PricingService) validateTimePricing(req *TimePricingRequest, excludeID string) error {
	if req.PricePerHour <= 0 {
		return errors.New("giá mỗi giờ phải lớn hơn 0")
	}
	if req.DayOfWeek < 0 || req.DayOfWeek > 6 {
		return errors.New("thứ trong tuần phải từ 0 (Chủ nhật) đến 6")
	}
	start, end := clockHHMM(req.StartTime), clockHHMM(req.EndTime)
	if !validClock(start) || !validClock(end) {
		return errors.New("giờ phải có dạng HH:MM")
	}
	if start == end {
		return errors.New("giờ bắt đầu và kết thúc không được trùng nhau")
	}

	var group model.MachineGroup
	if err := s.db.First(&group, "id = ?", req.MachineGroupID).Error; err != nil {
		return errors.New("không tìm thấy nhóm máy")
	}

	var rows []model.TimeBasedPricing
	q := s.db.Where("machine_group_id = ? AND day_of_week = ?", req.MachineGroupID, req.DayOfWeek)
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	if err := q.Find(&rows).Error; err != nil {
		return err
	}
	for _, r := range rows {
		if clockWindowsOverlap(start, end, clockHHMM(r.StartTime), clockHHMM(r.EndTime)) {
			return fmt.Errorf("khung giờ %s–%s chồng lên khung %s–%s đã có",
				start, end, clockHHMM(r.StartTime), clockHHMM(r.EndTime))
		}
	}
	return nil
}

// clockWindowsOverlap so hai khung giờ trong ngày, xử lý được khung vắt qua
// nửa đêm bằng cách trải thành các đoạn phút trên trục 0–1440.
func clockWindowsOverlap(aStart, aEnd, bStart, bEnd string) bool {
	for _, a := range clockSegments(aStart, aEnd) {
		for _, b := range clockSegments(bStart, bEnd) {
			if a[0] < b[1] && b[0] < a[1] {
				return true
			}
		}
	}
	return false
}

// clockSegments trải một khung giờ thành 1 đoạn, hoặc 2 đoạn nếu vắt qua nửa đêm.
func clockSegments(start, end string) [][2]int {
	s, e := clockMinutes(start), clockMinutes(end)
	if s < e {
		return [][2]int{{s, e}}
	}
	return [][2]int{{s, 1440}, {0, e}}
}

func clockMinutes(v string) int {
	var h, m int
	fmt.Sscanf(clockHHMM(v), "%d:%d", &h, &m)
	return h*60 + m
}

func validClock(v string) bool {
	if len(v) != 5 || v[2] != ':' {
		return false
	}
	var h, m int
	if _, err := fmt.Sscanf(v, "%d:%d", &h, &m); err != nil {
		return false
	}
	return h >= 0 && h < 24 && m >= 0 && m < 60
}

// --- tiện ích ---------------------------------------------------------------

func (s *PricingService) validateWindow(from, to string) (string, string, error) {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if from == "" {
		from = utils.VietnamTime().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", from); err != nil {
		return "", "", errors.New("ngày bắt đầu hiệu lực sai định dạng, cần YYYY-MM-DD")
	}
	if to != "" {
		t, err := time.Parse("2006-01-02", to)
		if err != nil {
			return "", "", errors.New("ngày kết thúc hiệu lực sai định dạng, cần YYYY-MM-DD")
		}
		f, _ := time.Parse("2006-01-02", from)
		if t.Before(f) {
			return "", "", errors.New("ngày kết thúc phải sau ngày bắt đầu")
		}
	}
	return from, to, nil
}

func (s *PricingService) checkGroups(machineGroupID, memberGroupID string) error {
	var n int64
	s.db.Model(&model.MachineGroup{}).Where("id = ?", machineGroupID).Count(&n)
	if n == 0 {
		return errors.New("không tìm thấy nhóm máy")
	}
	s.db.Model(&model.MemberGroup{}).Where("id = ?", memberGroupID).Count(&n)
	if n == 0 {
		return errors.New("không tìm thấy hạng hội viên")
	}
	return nil
}

// dateOnly cắt "2026-08-22T00:00:00Z" về "2026-08-22".
func dateOnly(v string) string {
	if len(v) >= 10 {
		return v[:10]
	}
	return v
}

// optionalDate biến chuỗi rỗng thành NULL cho cột date.
func optionalDate(v string) *string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return &v
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (s *PricingService) log(action, id, actorID string, meta map[string]interface{}) {
	var actor *string
	if actorID != "" {
		actor = &actorID
	}
	s.audit.Log(&LogAuditRequest{
		Action: action, EntityType: "pricing", EntityID: id,
		UserID: actor, Metadata: meta,
	})
}
