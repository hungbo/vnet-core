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
	ID             string  `json:"id"`
	UserID         string  `json:"user_id"`
	StoreID        *string `json:"store_id"`
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
	ClosingBalance int64  `json:"closing_balance" binding:"required"`
	Notes          string `json:"notes"`
}

type HandoverRequest struct {
	Amount       int64  `json:"amount" binding:"required"`
	HandoverType string `json:"handover_type" binding:"required,oneof=cash_in cash_out"`
	Reason       string `json:"reason"`
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
	for i, sh := range shifts {
		items[i] = shiftToResponse(sh)
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

func (s *ShiftService) OpenShift(req *OpenShiftRequest, userID string, storeID string) (*ShiftResponse, error) {
	var activeShift model.Shift
	if err := s.db.Where("user_id = ? AND status = ?", userID, "open").First(&activeShift).Error; err == nil {
		return nil, errors.New("user already has an open shift")
	}

	now := time.Now()
	var storeIDStr *string
	if storeID != "" {
		storeIDStr = &storeID
	}

	shift := model.Shift{
		UserID:         userID,
		StoreID:        storeIDStr,
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

	var expectedTotal int64
	s.db.Model(&model.Order{}).
		Where("store_id = ? AND status = ? AND created_at >= ? AND created_at <= ?",
			shift.StoreID, "paid", shift.StartedAt, time.Now()).
		Select("COALESCE(SUM(final_amount), 0)").
		Scan(&expectedTotal)

	now := time.Now()
	discrepancy := req.ClosingBalance - (shift.OpeningBalance + expectedTotal)

	updates := map[string]interface{}{
		"status":          "closed",
		"ended_at":        now,
		"closing_balance": req.ClosingBalance,
		"expected_total":  expectedTotal,
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
		StoreID:        s.StoreID,
		StartedAt:      s.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
		EndedAt:        endedAt,
		Status:         s.Status,
		OpeningBalance: s.OpeningBalance,
		ClosingBalance: s.ClosingBalance,
		ExpectedTotal:  s.ExpectedTotal,
		Notes:          s.Notes,
		CreatedAt:      s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}


