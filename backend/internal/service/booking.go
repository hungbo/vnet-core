package service

import (
	"errors"
	"time"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
)

type BookingService struct {
	db    *gorm.DB
	audit *AuditService
}

func NewBookingService(db *gorm.DB, audit *AuditService) *BookingService {
	return &BookingService{db: db, audit: audit}
}

type BookingListRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Sort     string `form:"sort"`
	Order    string `form:"order"`
	Search   string `form:"search"`
	Status   string `form:"status"`
	MachineID string `form:"machine_id"`
	DateFrom string `form:"date_from"`
	DateTo   string `form:"date_to"`
}

type BookingResponse struct {
	ID                   string  `json:"id"`
	MachineID            string  `json:"machine_id"`
	MemberID             *string `json:"member_id"`
	CustomerName         string  `json:"customer_name"`
	CustomerPhone        string  `json:"customer_phone"`
	BookedFrom           string  `json:"booked_from"`
	BookedTo             string  `json:"booked_to"`
	DepositAmount        int64   `json:"deposit_amount"`
	DepositTransactionID *string `json:"deposit_transaction_id"`
	Status               string  `json:"status"`
	CancelAt             *string `json:"cancel_at"`
	Notes                string  `json:"notes"`
	CreatedBy            *string `json:"created_by"`
	CreatedAt            string  `json:"created_at"`
	UpdatedAt            string  `json:"updated_at"`
}

type CreateBookingRequest struct {
	MachineID     string `json:"machine_id" binding:"required"`
	MemberID      string `json:"member_id"`
	CustomerName  string `json:"customer_name" binding:"required"`
	CustomerPhone string `json:"customer_phone" binding:"required"`
	BookedFrom    string `json:"booked_from" binding:"required"`
	BookedTo      string `json:"booked_to" binding:"required"`
	DepositAmount int64  `json:"deposit_amount"`
	Notes         string `json:"notes"`
}

type UpdateBookingRequest struct {
	CustomerName  string `json:"customer_name"`
	CustomerPhone string `json:"customer_phone"`
	BookedFrom    string `json:"booked_from"`
	BookedTo      string `json:"booked_to"`
	DepositAmount int64  `json:"deposit_amount"`
	Notes         string `json:"notes"`
}

func (s *BookingService) List(req *BookingListRequest) (*pagination.Result, error) {
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

	var bookings []model.MachineBooking
	query := s.db.Where("deleted_at IS NULL")

	if p.Search != "" {
		query = query.Where("customer_name ILIKE ? OR customer_phone ILIKE ?", "%"+p.Search+"%", "%"+p.Search+"%")
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.MachineID != "" {
		query = query.Where("machine_id = ?", req.MachineID)
	}
	if req.DateFrom != "" {
		if t, err := time.Parse(time.RFC3339, req.DateFrom); err == nil {
			query = query.Where("booked_from >= ?", t)
		}
	}
	if req.DateTo != "" {
		if t, err := time.Parse(time.RFC3339, req.DateTo); err == nil {
			query = query.Where("booked_to <= ?", t)
		}
	}

	var total int64
	query.Model(&model.MachineBooking{}).Count(&total)

	if err := pagination.Apply(query, p).Find(&bookings).Error; err != nil {
		return nil, err
	}

	items := make([]BookingResponse, len(bookings))
	for i, b := range bookings {
		items[i] = bookingToResponse(b)
	}

	return pagination.NewResult(items, total, p), nil
}

func (s *BookingService) GetByID(id string) (*BookingResponse, error) {
	var booking model.MachineBooking
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&booking).Error; err != nil {
		return nil, err
	}
	result := bookingToResponse(booking)
	return &result, nil
}

func (s *BookingService) Create(req *CreateBookingRequest, storeID string, userID string) (*BookingResponse, error) {
	bookedFrom, err := time.Parse(time.RFC3339, req.BookedFrom)
	if err != nil {
		return nil, errors.New("invalid booked_from format, use RFC3339")
	}
	bookedTo, err := time.Parse(time.RFC3339, req.BookedTo)
	if err != nil {
		return nil, errors.New("invalid booked_to format, use RFC3339")
	}

	if bookedTo.Before(bookedFrom) || bookedTo.Equal(bookedFrom) {
		return nil, errors.New("booked_to must be after booked_from")
	}

	var memberID *string
	if req.MemberID != "" {
		memberID = &req.MemberID
	}

	booking := model.MachineBooking{
		MachineID:     req.MachineID,
		MemberID:      memberID,
		CustomerName:  req.CustomerName,
		CustomerPhone: req.CustomerPhone,
		BookedFrom:    bookedFrom,
		BookedTo:      bookedTo,
		DepositAmount: req.DepositAmount,
		Status:        "pending",
		Notes:         req.Notes,
	}

	if userID != "" {
		booking.CreatedBy = &userID
	}

	if req.DepositAmount > 0 {
		booking.DepositAmount = req.DepositAmount
	}

	if err := s.db.Create(&booking).Error; err != nil {
		return nil, err
	}

	result := bookingToResponse(booking)
	var uid *string
	if userID != "" {
		uid = &userID
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "machine_booking",
		EntityID:   booking.ID,
		UserID:     uid,
		Metadata: map[string]interface{}{
			"machine_id":     booking.MachineID,
			"customer_name":  booking.CustomerName,
			"customer_phone": booking.CustomerPhone,
			"deposit_amount": booking.DepositAmount,
			"status":         booking.Status,
			"booked_from":    booking.BookedFrom.Format(time.RFC3339),
			"booked_to":      booking.BookedTo.Format(time.RFC3339),
		},
	})
	return &result, nil
}

func (s *BookingService) Update(id string, req *UpdateBookingRequest, userID string) (*BookingResponse, error) {
	var booking model.MachineBooking
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&booking).Error; err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.CustomerName != "" {
		updates["customer_name"] = req.CustomerName
	}
	if req.CustomerPhone != "" {
		updates["customer_phone"] = req.CustomerPhone
	}
	if req.BookedFrom != "" {
		t, err := time.Parse(time.RFC3339, req.BookedFrom)
		if err != nil {
			return nil, errors.New("invalid booked_from format")
		}
		updates["booked_from"] = t
	}
	if req.BookedTo != "" {
		t, err := time.Parse(time.RFC3339, req.BookedTo)
		if err != nil {
			return nil, errors.New("invalid booked_to format")
		}
		updates["booked_to"] = t
	}
	if req.DepositAmount > 0 {
		updates["deposit_amount"] = req.DepositAmount
	}
	updates["notes"] = req.Notes
	updates["updated_at"] = time.Now()

	if err := s.db.Model(&booking).Updates(updates).Error; err != nil {
		return nil, err
	}

	s.db.First(&booking, "id = ?", id)
	result := bookingToResponse(booking)
	var uid *string
	if userID != "" {
		uid = &userID
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "update",
		EntityType: "machine_booking",
		EntityID:   id,
		UserID:     uid,
		Metadata: map[string]interface{}{
			"machine_id":     booking.MachineID,
			"customer_name":  booking.CustomerName,
			"customer_phone": booking.CustomerPhone,
			"status":         booking.Status,
		},
	})
	return &result, nil
}

func (s *BookingService) Delete(id string) error {
	var booking model.MachineBooking
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&booking).Error; err != nil {
		return err
	}
	now := time.Now()
	if err := s.db.Model(&booking).Update("deleted_at", &now).Error; err != nil {
		return err
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "machine_booking",
		EntityID:   id,
		Metadata: map[string]interface{}{
			"machine_id":     booking.MachineID,
			"customer_name":  booking.CustomerName,
			"customer_phone": booking.CustomerPhone,
			"status":         booking.Status,
		},
	})
	return nil
}

func (s *BookingService) CheckIn(id string) (*BookingResponse, error) {
	result, err := s.updateStatus(id, "checked_in", nil)
	if err != nil {
		return nil, err
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "check_in",
		EntityType: "machine_booking",
		EntityID:   id,
		Metadata: map[string]interface{}{
			"status": "checked_in",
		},
	})
	return result, nil
}

func (s *BookingService) Cancel(id string) (*BookingResponse, error) {
	var booking model.MachineBooking
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&booking).Error; err != nil {
		return nil, err
	}

	if booking.Status == "cancelled" {
		return nil, errors.New("booking already cancelled")
	}
	if booking.Status == "checked_in" {
		return nil, errors.New("cannot cancel a checked-in booking")
	}
	if booking.Status == "no_show" {
		return nil, errors.New("cannot cancel a no-show booking")
	}

	tx := s.db.Begin()

	if booking.DepositAmount > 0 {
		var member model.Member
		if booking.MemberID != nil {
			if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&member, "id = ?", *booking.MemberID).Error; err == nil {
				trans := model.MemberTransaction{
					MemberID:        *booking.MemberID,
					TransactionType: "deposit_refund",
					Amount:          booking.DepositAmount,
					BalanceBefore:   member.Balance,
					BalanceAfter:    member.Balance + booking.DepositAmount,
					ReferenceID:     &booking.ID,
					Description:     "Deposit refund for cancelled booking",
					CreatedAt:       time.Now(),
				}
				if err := tx.Create(&trans).Error; err != nil {
					tx.Rollback()
					return nil, err
				}
				tx.Model(&member).Update("balance", member.Balance+booking.DepositAmount)
			}
		}
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":    "cancelled",
		"cancel_at": &now,
		"updated_at": now,
	}
	if err := tx.Model(&booking).Updates(updates).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	tx.Commit()

	booking.Status = "cancelled"
	booking.CancelAt = &now
	result := bookingToResponse(booking)
	refundAmount := int64(0)
	if booking.DepositAmount > 0 && booking.MemberID != nil {
		refundAmount = booking.DepositAmount
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "cancel",
		EntityType: "machine_booking",
		EntityID:   id,
		Metadata: map[string]interface{}{
			"machine_id":     booking.MachineID,
			"customer_name":  booking.CustomerName,
			"status":         "cancelled",
			"deposit_amount": booking.DepositAmount,
			"refund_amount":  refundAmount,
			"member_id":      booking.MemberID,
		},
	})
	return &result, nil
}

func (s *BookingService) NoShow(id string) (*BookingResponse, error) {
	var booking model.MachineBooking
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&booking).Error; err != nil {
		return nil, err
	}

	if booking.Status == "no_show" {
		return nil, errors.New("booking already marked as no-show")
	}
	if booking.Status == "cancelled" {
		return nil, errors.New("cannot mark cancelled booking as no-show")
	}

	result, err := s.updateStatus(id, "no_show", nil)
	if err != nil {
		return nil, err
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "no_show",
		EntityType: "machine_booking",
		EntityID:   id,
		Metadata: map[string]interface{}{
			"status": "no_show",
		},
	})
	return result, nil
}

func (s *BookingService) updateStatus(id, status string, cancelAt *time.Time) (*BookingResponse, error) {
	var booking model.MachineBooking
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&booking).Error; err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	if cancelAt != nil {
		updates["cancel_at"] = cancelAt
	}

	if err := s.db.Model(&booking).Updates(updates).Error; err != nil {
		return nil, err
	}

	booking.Status = status
	if cancelAt != nil {
		booking.CancelAt = cancelAt
	}
	result := bookingToResponse(booking)
	return &result, nil
}

func bookingToResponse(b model.MachineBooking) BookingResponse {
	resp := BookingResponse{
		ID:            b.ID,
		MachineID:     b.MachineID,
		MemberID:      b.MemberID,
		CustomerName:  b.CustomerName,
		CustomerPhone: b.CustomerPhone,
		BookedFrom:    b.BookedFrom.Format(time.RFC3339),
		BookedTo:      b.BookedTo.Format(time.RFC3339),
		DepositAmount: b.DepositAmount,
		Status:        b.Status,
		Notes:         b.Notes,
		CreatedBy:     b.CreatedBy,
		CreatedAt:     b.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     b.UpdatedAt.Format(time.RFC3339),
	}
	if b.DepositTransactionID != nil {
		resp.DepositTransactionID = b.DepositTransactionID
	}
	if b.CancelAt != nil {
		s := b.CancelAt.Format(time.RFC3339)
		resp.CancelAt = &s
	}
	return resp
}
