package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/utils"
	"gorm.io/gorm"
)

type CurfewService struct {
	db    *gorm.DB
	audit *AuditService
}

func NewCurfewService(db *gorm.DB, audit *AuditService) *CurfewService {
	return &CurfewService{db: db, audit: audit}
}

type CurfewListRequest struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	Sort      string `form:"sort"`
	Order     string `form:"order"`
	Search    string `form:"search"`
	DayOfWeek *int   `form:"day_of_week"`
}

type CurfewResponse struct {
	ID              string  `json:"id"`
	DayOfWeek       int     `json:"day_of_week"`
	CurfewStart     string  `json:"curfew_start"`
	CurfewEnd       string  `json:"curfew_end"`
	MaxMinorHours   int     `json:"max_minor_hours"`
	IsActive        bool    `json:"is_active"`
	OverrideByAdmin *string `json:"override_by_admin"`
	OverrideReason  string  `json:"override_reason"`
	OverrideAt      *string `json:"override_at"`
	CreatedAt       string  `json:"created_at"`
}

type CreateCurfewRequest struct {
	// Chủ nhật là 0; "required" sẽ coi đó là thiếu dữ liệu nên chỉ dùng min/max.
	DayOfWeek     int    `json:"day_of_week" binding:"min=0,max=6"`
	CurfewStart   string `json:"curfew_start" binding:"required"`
	CurfewEnd     string `json:"curfew_end" binding:"required"`
	MaxMinorHours int    `json:"max_minor_hours"`
	IsActive      bool   `json:"is_active"`
}

type UpdateCurfewRequest struct {
	DayOfWeek     *int   `json:"day_of_week" binding:"omitempty,min=0,max=6"`
	CurfewStart   string `json:"curfew_start"`
	CurfewEnd     string `json:"curfew_end"`
	MaxMinorHours *int   `json:"max_minor_hours"`
	IsActive      *bool  `json:"is_active"`
}

type OverrideCurfewRequest struct {
	PolicyID       string `json:"policy_id" binding:"required"`
	OverrideReason string `json:"override_reason" binding:"required"`
}

func (s *CurfewService) List(req *CurfewListRequest) (*pagination.Result, error) {
	p := &pagination.Params{
		Page:     req.Page,
		PageSize: req.PageSize,
		Sort:     req.Sort,
		Order:    req.Order,
		Search:   req.Search,
	}
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = 20
	}
	if p.Sort == "" {
		p.Sort = "created_at"
	}
	if p.Order == "" {
		p.Order = "desc"
	}

	var policies []model.CurfewPolicy
	query := s.db.Model(&model.CurfewPolicy{})
	if req.DayOfWeek != nil {
		query = query.Where("day_of_week = ?", *req.DayOfWeek)
	}

	var total int64
	query.Count(&total)

	if err := pagination.Apply(query, p).Find(&policies).Error; err != nil {
		return nil, err
	}

	items := make([]CurfewResponse, len(policies))
	for i, p := range policies {
		items[i] = curfewToResponse(p)
	}

	return pagination.NewResult(items, total, p), nil
}

func (s *CurfewService) GetByID(id string) (*CurfewResponse, error) {
	var policy model.CurfewPolicy
	if err := s.db.First(&policy, "id = ?", id).Error; err != nil {
		return nil, err
	}
	result := curfewToResponse(policy)
	return &result, nil
}

func (s *CurfewService) Create(req *CreateCurfewRequest) (*CurfewResponse, error) {
	policy := model.CurfewPolicy{
		DayOfWeek:     req.DayOfWeek,
		CurfewStart:   req.CurfewStart,
		CurfewEnd:     req.CurfewEnd,
		MaxMinorHours: req.MaxMinorHours,
		IsActive:      req.IsActive,
	}

	if err := s.db.Create(&policy).Error; err != nil {
		return nil, err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "curfew_policy",
		EntityID:   policy.ID,
		Metadata:   map[string]interface{}{"day_of_week": req.DayOfWeek, "curfew_start": req.CurfewStart, "curfew_end": req.CurfewEnd},
	})

	result := curfewToResponse(policy)
	return &result, nil
}

func (s *CurfewService) Update(id string, req *UpdateCurfewRequest) (*CurfewResponse, error) {
	var policy model.CurfewPolicy
	if err := s.db.First(&policy, "id = ?", id).Error; err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.DayOfWeek != nil {
		updates["day_of_week"] = *req.DayOfWeek
	}
	if req.CurfewStart != "" {
		updates["curfew_start"] = req.CurfewStart
	}
	if req.CurfewEnd != "" {
		updates["curfew_end"] = req.CurfewEnd
	}
	if req.MaxMinorHours != nil {
		updates["max_minor_hours"] = *req.MaxMinorHours
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) > 0 {
		if err := s.db.Model(&policy).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	s.db.First(&policy, "id = ?", id)

	s.audit.Log(&LogAuditRequest{
		Action:     "update",
		EntityType: "curfew_policy",
		EntityID:   id,
		Metadata:   map[string]interface{}{"updates": updates},
	})

	result := curfewToResponse(policy)
	return &result, nil
}

// Delete xoá một chính sách giờ giới nghiêm.
//
// Đã rà internal/model: không bảng nào tham chiếu curfew_policies. Cột
// override_by_admin là của chính bảng này, trỏ RA ngoài chứ không phải bị trỏ
// vào. Không cần kiểm phụ thuộc.
func (s *CurfewService) Delete(id string) error {
	var policy model.CurfewPolicy
	if err := s.db.First(&policy, "id = ?", id).Error; err != nil {
		return err
	}
	if err := s.db.Delete(&policy).Error; err != nil {
		return err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "curfew_policy",
		EntityID:   id,
		Metadata:   map[string]interface{}{"day_of_week": policy.DayOfWeek},
	})

	return nil
}

func (s *CurfewService) Override(req *OverrideCurfewRequest, adminID string) (*CurfewResponse, error) {
	var policy model.CurfewPolicy
	if err := s.db.First(&policy, "id = ?", req.PolicyID).Error; err != nil {
		return nil, errors.New("không tìm thấy chính sách giới nghiêm")
	}

	now := time.Now()
	updates := map[string]interface{}{
		"override_by_admin": adminID,
		"override_reason":   req.OverrideReason,
		"override_at":       now,
	}

	if err := s.db.Model(&policy).Updates(updates).Error; err != nil {
		return nil, err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "override",
		EntityType: "curfew_policy",
		EntityID:   req.PolicyID,
		Metadata:   map[string]interface{}{"reason": req.OverrideReason, "admin_id": adminID},
	})

	policy.OverrideByAdmin = &adminID
	policy.OverrideReason = req.OverrideReason
	policy.OverrideAt = &now

	result := curfewToResponse(policy)
	return &result, nil
}

func curfewToResponse(p model.CurfewPolicy) CurfewResponse {
	resp := CurfewResponse{
		ID:              p.ID,
		DayOfWeek:       p.DayOfWeek,
		CurfewStart:     p.CurfewStart,
		CurfewEnd:       p.CurfewEnd,
		MaxMinorHours:   p.MaxMinorHours,
		IsActive:        p.IsActive,
		OverrideByAdmin: p.OverrideByAdmin,
		OverrideReason:  p.OverrideReason,
		CreatedAt:       p.CreatedAt.Format(time.RFC3339),
	}
	if p.OverrideAt != nil {
		s := p.OverrideAt.Format(time.RFC3339)
		resp.OverrideAt = &s
	}
	return resp
}

// MinorAgeThreshold is the age below which curfew rules apply.
const MinorAgeThreshold = 18

// IsMinor reports whether the member is under age at the given moment. A member
// with no recorded birthday is treated as an adult: refusing service on missing
// data would lock out every walk-in account, so the registration flow is where
// that gap belongs.
func IsMinor(member *model.Member, at time.Time) bool {
	if member == nil || member.DateOfBirth == nil {
		return false
	}
	return ageAt(*member.DateOfBirth, at) < MinorAgeThreshold
}

func ageAt(dob, at time.Time) int {
	years := at.Year() - dob.Year()
	if at.YearDay() < dob.YearDay() {
		years--
	}
	return years
}

// ActivePolicy returns the curfew rule covering the given moment, if any.
// Windows that wrap past midnight (22:00–06:00) are handled explicitly.
func (s *CurfewService) ActivePolicy(at time.Time) (*model.CurfewPolicy, error) {
	var policies []model.CurfewPolicy
	if err := s.db.Where("is_active = ? AND day_of_week = ?", true, int(at.Weekday())).
		Find(&policies).Error; err != nil {
		return nil, err
	}

	clock := at.Format("15:04:05")
	for i := range policies {
		p := &policies[i]
		if withinCurfew(clock, p.CurfewStart, p.CurfewEnd) {
			return p, nil
		}
	}
	return nil, nil
}

func withinCurfew(now, start, end string) bool {
	if start == "" || end == "" {
		return false
	}
	if start <= end {
		return now >= start && now < end
	}
	// Wraps midnight.
	return now >= start || now < end
}

// gioNganGon cắt phần giây khỏi chuỗi giờ để hiện cho người đọc: "22:00:00"
// thành "22:00". Cột lưu kiểu time nên luôn có giây.
func gioNganGon(gio string) string {
	if len(gio) >= 5 {
		return gio[:5]
	}
	return gio
}

// CheckStart reports whether a member may begin a session right now.
func (s *CurfewService) CheckStart(member *model.Member, at time.Time) error {
	if !IsMinor(member, at) {
		return nil
	}

	policy, err := s.ActivePolicy(at)
	if err != nil {
		return err
	}
	if policy != nil && !s.overridden(policy, at) {
		// Câu này hiện thẳng lên màn hình khoá của khách — một bạn 15 tuổi ở
		// quán net Việt Nam — nên phải là tiếng Việt, giống câu giới hạn giờ
		// chơi ngay bên dưới. Cắt phần giây: cột lưu "22:00:00", người đọc chỉ
		// cần "22:00".
		return fmt.Errorf("đang trong khung giờ cấm: khách vị thành niên không được chơi từ %s đến %s",
			gioNganGon(policy.CurfewStart), gioNganGon(policy.CurfewEnd))
	}

	// Trần giờ chơi trong ngày áp cả ngày, không chỉ trong khung giờ cấm, nên
	// phải tra chính sách của HÔM NAY chứ không phải chính sách đang phủ lúc này.
	return s.checkMinorHours(member, at)
}

// checkMinorHours áp max_minor_hours — cột lưu được từ đầu nhưng chưa nơi nào
// đọc, nên giới hạn số giờ chơi của vị thành niên chỉ là con số trong database.
func (s *CurfewService) checkMinorHours(member *model.Member, at time.Time) error {
	policy, err := s.TodayPolicy(at)
	if err != nil || policy == nil || policy.MaxMinorHours <= 0 {
		return err
	}
	if s.overridden(policy, at) {
		return nil
	}

	played := s.MinutesPlayedToday(member.ID, at)
	limit := policy.MaxMinorHours * 60
	if played >= limit {
		return fmt.Errorf("vị thành niên chỉ được chơi %d giờ mỗi ngày; hôm nay đã chơi %d phút",
			policy.MaxMinorHours, played)
	}
	return nil
}

func (s *CurfewService) overridden(policy *model.CurfewPolicy, at time.Time) bool {
	// An administrator lifted the rule for today.
	return policy.OverrideByAdmin != nil && policy.OverrideAt != nil && sameDay(*policy.OverrideAt, at)
}

// TodayPolicy trả về chính sách giới nghiêm của hôm nay bất kể giờ hiện tại có
// nằm trong khung cấm hay không. ActivePolicy chỉ trả về khi ĐANG trong khung.
func (s *CurfewService) TodayPolicy(at time.Time) (*model.CurfewPolicy, error) {
	var policy model.CurfewPolicy
	err := s.db.Where("is_active = ? AND day_of_week = ?", true, int(at.Weekday())).
		Order("max_minor_hours desc").First(&policy).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &policy, nil
}

// MinutesPlayedToday cộng số phút đã chơi trong ngày: các phiên đã trả máy cộng
// phiên đang chạy. Phiên đang chạy chưa có duration_minutes nên phải tính từ
// started_at, nếu không khách chỉ cần không trả máy là trần giờ mất tác dụng.
func (s *CurfewService) MinutesPlayedToday(memberID string, at time.Time) int {
	dayStart := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, at.Location())

	var sessions []model.MachineSession
	if err := s.db.Where("member_id = ? AND started_at >= ?", memberID, dayStart).
		Find(&sessions).Error; err != nil {
		return 0
	}

	total := 0
	for _, sess := range sessions {
		if sess.IsActive {
			elapsed := int(at.Sub(sess.StartedAt).Minutes())
			if elapsed > 0 {
				total += elapsed
			}
			continue
		}
		if sess.DurationMinutes != nil {
			total += *sess.DurationMinutes
		}
	}
	return total
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// EnforceNow ends the sessions of under-age members once curfew starts. This is
// the piece that made the whole CurfewPolicy table meaningful: the rules were
// configurable and auditable from the start, but nothing ever acted on them.
func (s *CurfewService) EnforceNow(sessions *SessionService, wsHub *hub.Hub) error {
	at := utils.VietnamTime()

	// Hai luật khác nhau, hai phạm vi khác nhau:
	//   - khung giờ cấm: chỉ áp khi ĐANG trong khung
	//   - trần giờ mỗi ngày: áp cả ngày
	// Bản cũ thoát sớm khi không có khung nào phủ lúc này, nên trần giờ không
	// bao giờ được kiểm — khách vị thành niên ngồi bao lâu cũng không ai đuổi.
	windowPolicy, err := s.ActivePolicy(at)
	if err != nil {
		return err
	}
	if windowPolicy != nil && s.overridden(windowPolicy, at) {
		windowPolicy = nil
	}

	dayPolicy, err := s.TodayPolicy(at)
	if err != nil {
		return err
	}
	if dayPolicy != nil && (dayPolicy.MaxMinorHours <= 0 || s.overridden(dayPolicy, at)) {
		dayPolicy = nil
	}

	if windowPolicy == nil && dayPolicy == nil {
		return nil
	}

	var active []model.MachineSession
	if err := s.db.Where("is_active = ? AND member_id IS NOT NULL", true).Find(&active).Error; err != nil {
		return err
	}

	for _, sess := range active {
		var member model.Member
		if err := s.db.Where("id = ?", *sess.MemberID).First(&member).Error; err != nil {
			continue
		}
		if !IsMinor(&member, at) {
			continue
		}

		policy := windowPolicy
		reason := "curfew_window"
		if policy == nil {
			if s.MinutesPlayedToday(member.ID, at) < dayPolicy.MaxMinorHours*60 {
				continue
			}
			policy = dayPolicy
			reason = "max_minor_hours"
		}

		if _, err := sessions.EndSession(sess.ID); err != nil {
			continue
		}

		// EndSession chụp lại machine_code lên chính dòng phiên, nên tra ở đây là
		// đủ. Thiếu nó thì thông báo cho nhân viên chỉ hiện UUID, vô dụng khi đang
		// cần biết máy nào vừa bị buộc trả.
		machineCode := ""
		s.db.Model(&model.Machine{}).Where("id = ?", sess.MachineID).
			Pluck("machine_code", &machineCode)

		wsHub.SendToAdminsAndMachine(machineCode, hub.Event{
			Type: "curfew:enforced",
			Data: map[string]interface{}{
				"session_id":   sess.ID,
				"member_id":    member.ID,
				"machine_id":   sess.MachineID,
				"machine_code": machineCode,
				"reason":       reason,
			},
		})
		s.audit.Log(&LogAuditRequest{
			Action:     "curfew_enforced",
			EntityType: "machine_session",
			EntityID:   sess.ID,
			Metadata: map[string]interface{}{
				"member_id":       member.ID,
				"reason":          reason,
				"curfew_start":    policy.CurfewStart,
				"curfew_end":      policy.CurfewEnd,
				"max_minor_hours": policy.MaxMinorHours,
			},
		})
	}
	return nil
}
