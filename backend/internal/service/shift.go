package service

import (
	"errors"
	"time"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
)

type ShiftService struct {
	db    *gorm.DB
	audit *AuditService
}

func NewShiftService(db *gorm.DB, audit *AuditService) *ShiftService {
	return &ShiftService{db: db, audit: audit}
}

type ShiftResponse struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`
	// Cột "Người dùng" trên trang Ca làm việc đọc trường này. Không trả về thì
	// bảng hiện UUID thô — nhân viên không đọc được ai đã trực ca nào.
	UserName       string  `json:"user_name"`
	StartedAt      string  `json:"started_at"`
	EndedAt        *string `json:"ended_at"`
	Status         string  `json:"status"`
	OpeningBalance int64   `json:"opening_balance"`
	ClosingBalance *int64  `json:"closing_balance"`
	ExpectedTotal  *int64  `json:"expected_total"`
	Discrepancy    *int64  `json:"discrepancy"`
	Notes          string  `json:"notes"`
	CreatedAt      string  `json:"created_at"`
}

type OpenShiftRequest struct {
	OpeningBalance int64  `json:"opening_balance"`
	Notes          string `json:"notes"`
}

type CloseShiftRequest struct {
	// Đóng ca với két rỗng là hợp lệ, nên 0 phải được chấp nhận.
	ClosingBalance int64  `json:"closing_balance"`
	Notes          string `json:"notes"`
}

type HandoverRequest struct {
	Amount       int64  `json:"amount" binding:"gt=0"`
	HandoverType string `json:"handover_type" binding:"required,oneof=cash_in cash_out"`
	Reason       string `json:"reason"`
}

// userNames nạp tên nhân viên cho cả trang một lần thay vì mỗi dòng một truy vấn.
// Unscoped: nhân viên nghỉ việc rồi thì ca trực cũ vẫn phải hiện tên.
func (s *ShiftService) userNames(shifts []model.Shift) map[string]string {
	ids := make([]string, 0, len(shifts))
	for _, sh := range shifts {
		ids = append(ids, sh.UserID)
	}
	out := map[string]string{}
	if len(ids) == 0 {
		return out
	}
	var rows []struct{ ID, Username, FullName string }
	s.db.Unscoped().Model(&model.User{}).Select("id, username, full_name").
		Where("id IN ?", ids).Find(&rows)
	for _, r := range rows {
		if r.FullName != "" {
			out[r.ID] = r.FullName
		} else {
			out[r.ID] = r.Username
		}
	}
	return out
}

func (s *ShiftService) List(params *pagination.Params) (*pagination.Result, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}
	if params.Sort == "" {
		params.Sort = "started_at"
	}
	if params.Order == "" {
		params.Order = "desc"
	}

	var shifts []model.Shift
	query := s.db
	if params.Search != "" {
		query = query.Where("user_id = ?", params.Search)
	}

	var total int64
	query.Model(&model.Shift{}).Count(&total)

	if err := pagination.Apply(query, params).Find(&shifts).Error; err != nil {
		return nil, err
	}

	items := make([]ShiftResponse, len(shifts))
	names := s.userNames(shifts)
	for i, sh := range shifts {
		items[i] = shiftToResponse(sh)
		items[i].UserName = names[sh.UserID]
	}

	return pagination.NewResult(items, total, params), nil
}

func (s *ShiftService) GetByID(id string) (*ShiftResponse, error) {
	var shift model.Shift
	if err := s.db.Where("id = ?", id).First(&shift).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("shift not found")
		}
		return nil, err
	}

	result := shiftToResponse(shift)
	return &result, nil
}

func (s *ShiftService) OpenShift(req *OpenShiftRequest, userID string) (*ShiftResponse, error) {
	var activeShift model.Shift
	if err := s.db.Where("user_id = ? AND status = ?", userID, "open").First(&activeShift).Error; err == nil {
		return nil, errors.New("user already has an open shift")
	}

	now := time.Now()

	shift := model.Shift{
		UserID:         userID,
		StartedAt:      now,
		Status:         "open",
		OpeningBalance: req.OpeningBalance,
		Notes:          req.Notes,
	}

	if err := s.db.Create(&shift).Error; err != nil {
		return nil, err
	}

	result := shiftToResponse(shift)

	s.audit.Log(&LogAuditRequest{
		Action:     "open_shift",
		EntityType: "shift",
		EntityID:   shift.ID,
		UserID:     &userID,
		Metadata:   map[string]interface{}{"opening_cash": req.OpeningBalance},
	})

	return &result, nil
}

// expectedCash is what the drawer should hold on top of the opening float:
// only money that physically entered or left it during the shift.
//
// Balance-settled orders and session fees never touch the drawer, so they are
// deliberately excluded — the previous implementation summed every order with
// status "paid", a status this codebase never assigns, so the expected total
// was always zero and every shift reconciled against nothing.
//
// Orders carry no shift_id, so the window is the shift's own time range. With
// overlapping shifts on one till the attribution is approximate; giving orders
// a shift_id is the proper fix and is tracked separately.
func (s *ShiftService) expectedCash(shift *model.Shift, until time.Time) (int64, error) {
	var cashPayments int64
	if err := s.db.Model(&model.Payment{}).
		Where("payment_method = ? AND status = ? AND paid_at >= ? AND paid_at <= ?",
			"cash", "completed", shift.StartedAt, until).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&cashPayments).Error; err != nil {
		return 0, err
	}

	// Top-ups add cash; combo purchases paid in cash do too, and are stored as
	// a negative ledger amount, hence the sign flip.
	var cashTopups int64
	if err := s.db.Model(&model.MemberTransaction{}).
		Where("payment_method = ? AND transaction_type IN ? AND created_at >= ? AND created_at <= ?",
			"cash", []string{"topup", "topup_bonus"}, shift.StartedAt, until).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&cashTopups).Error; err != nil {
		return 0, err
	}

	var cashCombos int64
	if err := s.db.Model(&model.MemberTransaction{}).
		Where("payment_method = ? AND transaction_type = ? AND created_at >= ? AND created_at <= ?",
			"cash", "combo_purchase", shift.StartedAt, until).
		Select("COALESCE(SUM(-amount), 0)").
		Scan(&cashCombos).Error; err != nil {
		return 0, err
	}

	var handoverIn int64
	if err := s.db.Model(&model.CashHandover{}).
		Where("shift_id = ? AND handover_type = ?", shift.ID, "cash_in").
		Select("COALESCE(SUM(amount), 0)").
		Scan(&handoverIn).Error; err != nil {
		return 0, err
	}

	var handoverOut int64
	if err := s.db.Model(&model.CashHandover{}).
		Where("shift_id = ? AND handover_type = ?", shift.ID, "cash_out").
		Select("COALESCE(SUM(amount), 0)").
		Scan(&handoverOut).Error; err != nil {
		return 0, err
	}

	return cashPayments + cashTopups + cashCombos + handoverIn - handoverOut, nil
}

func (s *ShiftService) CloseShift(id string, req *CloseShiftRequest) (*ShiftResponse, error) {
	var shift model.Shift
	if err := s.db.Where("id = ?", id).First(&shift).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("shift not found")
		}
		return nil, err
	}

	if shift.Status == "closed" {
		return nil, errors.New("shift is already closed")
	}

	now := time.Now()
	expectedTotal, err := s.expectedCash(&shift, now)
	if err != nil {
		return nil, err
	}

	discrepancy := req.ClosingBalance - (shift.OpeningBalance + expectedTotal)

	updates := map[string]interface{}{
		"status":          "closed",
		"ended_at":        now,
		"closing_balance": req.ClosingBalance,
		"expected_total":  expectedTotal,
		"discrepancy":     discrepancy,
		"notes":           shift.Notes + "\n" + req.Notes,
	}

	if err := s.db.Model(&shift).Updates(updates).Error; err != nil {
		return nil, err
	}

	s.db.First(&shift, "id = ?", id)
	result := shiftToResponse(shift)
	result.Discrepancy = &discrepancy

	s.audit.Log(&LogAuditRequest{
		Action:     "close_shift",
		EntityType: "shift",
		EntityID:   shift.ID,
		Metadata:   map[string]interface{}{"closing_cash": req.ClosingBalance, "expected_total": expectedTotal, "discrepancy": discrepancy},
	})

	return &result, nil
}

func (s *ShiftService) Handover(shiftID string, req *HandoverRequest, userID string) (interface{}, error) {
	var shift model.Shift
	if err := s.db.Where("id = ?", shiftID).First(&shift).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("shift not found")
		}
		return nil, err
	}

	var userIDStr *string
	if userID != "" {
		userIDStr = &userID
	}

	handover := model.CashHandover{
		ShiftID:      shiftID,
		Amount:       req.Amount,
		HandoverType: req.HandoverType,
		Reason:       req.Reason,
		CreatedBy:    userIDStr,
	}

	if err := s.db.Create(&handover).Error; err != nil {
		return nil, err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "handover",
		EntityType: "cash_handover",
		EntityID:   handover.ID,
		UserID:     &userID,
		Metadata:   map[string]interface{}{"type": req.HandoverType, "amount": req.Amount},
	})

	return map[string]interface{}{
		"id":            handover.ID,
		"shift_id":      handover.ShiftID,
		"amount":        handover.Amount,
		"handover_type": handover.HandoverType,
		"reason":        handover.Reason,
		"created_at":    handover.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

func shiftToResponse(s model.Shift) ShiftResponse {
	var endedAt *string
	if s.EndedAt != nil {
		f := s.EndedAt.Format("2006-01-02T15:04:05Z07:00")
		endedAt = &f
	}

	return ShiftResponse{
		ID:             s.ID,
		UserID:         s.UserID,
		StartedAt:      s.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
		EndedAt:        endedAt,
		Status:         s.Status,
		OpeningBalance: s.OpeningBalance,
		ClosingBalance: s.ClosingBalance,
		ExpectedTotal:  s.ExpectedTotal,
		Discrepancy:    s.Discrepancy,
		Notes:          s.Notes,
		CreatedAt:      s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
