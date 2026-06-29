package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/utils"
	"gorm.io/gorm"
)

type ComboService struct {
	db    *gorm.DB
	audit *AuditService
}

func NewComboService(db *gorm.DB, audit *AuditService) *ComboService {
	return &ComboService{db: db, audit: audit}
}

type ComboListRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Sort     string `form:"sort"`
	Order    string `form:"order"`
	Search   string `form:"search"`
}

type ComboResponse struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Type         string   `json:"type"`
	SlotStart    string   `json:"slot_start"`
	SlotEnd      string   `json:"slot_end"`
	ApplyDays    []int    `json:"apply_days"`
	TotalMinutes int      `json:"total_minutes"`
	ValidityDays int      `json:"validity_days"`
	Price        int64    `json:"price"`
	MemberPrefix string   `json:"member_prefix"`
	MemberCount  int      `json:"member_count"`
	IsActive     bool     `json:"is_active"`
	CreatedAt    string   `json:"created_at"`
}

type CreateComboRequest struct {
	Name         string `json:"name" binding:"required"`
	Description  string `json:"description"`
	Type         string `json:"type" binding:"required,oneof=fixed_slot prepaid"`
	SlotStart    string `json:"slot_start"`
	SlotEnd      string `json:"slot_end"`
	ApplyDays    []int  `json:"apply_days"`
	TotalMinutes int    `json:"total_minutes"`
	ValidityDays int    `json:"validity_days"`
	Price        int64  `json:"price" binding:"required,min=0"`
	IsActive     bool   `json:"is_active"`
}

type UpdateComboRequest struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Type         string `json:"type" binding:"omitempty,oneof=fixed_slot prepaid"`
	SlotStart    string `json:"slot_start"`
	SlotEnd      string `json:"slot_end"`
	ApplyDays    []int  `json:"apply_days"`
	TotalMinutes int    `json:"total_minutes"`
	ValidityDays int    `json:"validity_days"`
	Price        int64  `json:"price" binding:"min=0"`
	IsActive     *bool  `json:"is_active"`
}

type PurchaseComboRequest struct {
	MemberID      string `json:"member_id"`
	CustomerName  string `json:"customer_name"`
	CustomerPhone string `json:"customer_phone"`
	PaymentMethod string `json:"payment_method" binding:"required"`
}



type ActivateComboRequest struct {
	MachineID string `json:"machine_id" binding:"required"`
}

func (s *ComboService) List(req *ComboListRequest) (*pagination.Result, error) {
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

	var combos []model.Combo
	query := s.db.Where("deleted_at IS NULL")
	if p.Search != "" {
		query = query.Where("name ILIKE ?", "%"+p.Search+"%")
	}

	var total int64
	query.Model(&model.Combo{}).Count(&total)

	if err := pagination.Apply(query, p).Find(&combos).Error; err != nil {
		return nil, err
	}

	items := make([]ComboResponse, len(combos))
	for i, c := range combos {
		items[i] = comboToResponse(c)
	}

	return pagination.NewResult(items, total, p), nil
}

func (s *ComboService) GetByID(id string) (*ComboResponse, error) {
	var combo model.Combo
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&combo).Error; err != nil {
		return nil, err
	}
	result := comboToResponse(combo)
	return &result, nil
}

func (s *ComboService) Create(req *CreateComboRequest) (*ComboResponse, error) {
	prefix := generateMemberPrefix(req.Name)

	combo := model.Combo{
		Name:         req.Name,
		Description:  req.Description,
		Type:         req.Type,
		SlotStart:    req.SlotStart,
		SlotEnd:      req.SlotEnd,
		ApplyDays:    req.ApplyDays,
		TotalMinutes: req.TotalMinutes,
		ValidityDays: req.ValidityDays,
		Price:        req.Price,
		MemberPrefix: prefix,
		MemberCount:  0,
		IsActive:     true,
	}

	if err := s.db.Create(&combo).Error; err != nil {
		return nil, err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "combo",
		EntityID:   combo.ID,
		Metadata: map[string]interface{}{
			"name":  combo.Name,
			"type":  combo.Type,
			"price": combo.Price,
		},
	})

	result := comboToResponse(combo)
	return &result, nil
}

func (s *ComboService) Update(id string, req *UpdateComboRequest) (*ComboResponse, error) {
	var combo model.Combo
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&combo).Error; err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
		updates["member_prefix"] = generateMemberPrefix(req.Name)
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Type != "" {
		updates["type"] = req.Type
	}
	if req.SlotStart != "" {
		updates["slot_start"] = req.SlotStart
	}
	if req.SlotEnd != "" {
		updates["slot_end"] = req.SlotEnd
	}
	if req.ApplyDays != nil {
		updates["apply_days"] = req.ApplyDays
	}
	if req.TotalMinutes > 0 {
		updates["total_minutes"] = req.TotalMinutes
	}
	if req.ValidityDays > 0 {
		updates["validity_days"] = req.ValidityDays
	}
	if req.Price > 0 {
		updates["price"] = req.Price
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if err := s.db.Model(&combo).Updates(updates).Error; err != nil {
		return nil, err
	}

	s.db.First(&combo, "id = ?", id)

	s.audit.Log(&LogAuditRequest{
		Action:     "update",
		EntityType: "combo",
		EntityID:   id,
		Metadata:   updates,
	})

	result := comboToResponse(combo)
	return &result, nil
}

func (s *ComboService) Delete(id string) error {
	var combo model.Combo
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&combo).Error; err != nil {
		return err
	}
	now := time.Now()
	if err := s.db.Model(&combo).Update("deleted_at", &now).Error; err != nil {
		return err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "combo",
		EntityID:   id,
		Metadata: map[string]interface{}{
			"name": combo.Name,
		},
	})

	return nil
}

func (s *ComboService) Purchase(comboID string, req *PurchaseComboRequest, storeID string, userID string) (*ComboPurchaseResponse, error) {
	var combo model.Combo
	if err := s.db.Where("id = ? AND deleted_at IS NULL AND is_active = ?", comboID, true).First(&combo).Error; err != nil {
		return nil, errors.New("combo not found or inactive")
	}

	memberID := req.MemberID

	if memberID == "" {
		if combo.MemberPrefix == "" {
			return nil, errors.New("member prefix not configured on this combo")
		}

		combo.MemberCount++
		memberCode := fmt.Sprintf("%s-%04d", combo.MemberPrefix, combo.MemberCount)

		randomBytes := make([]byte, 16)
		rand.Read(randomBytes)
		randomPass := hex.EncodeToString(randomBytes)
		passHash, _ := utils.HashPassword(randomPass)

		member := model.Member{
			Username:     memberCode,
			PasswordHash: passHash,
			FullName:     req.CustomerName,
			Phone:        req.CustomerPhone,
			IsActive:     true,
		}
		if storeID != "" {
			storeStr := storeID
			member.StoreID = &storeStr
		}

		if err := s.db.Create(&member).Error; err != nil {
			return nil, err
		}
		memberID = member.ID

		s.db.Model(&combo).Update("member_count", combo.MemberCount)
	}

	var expiresAt *time.Time
	if combo.ValidityDays > 0 {
		t := time.Now().AddDate(0, 0, combo.ValidityDays)
		expiresAt = &t
	}

	purchase := model.ComboPurchase{
		ComboID:          comboID,
		MemberID:         memberID,
		Price:            combo.Price,
		PaymentMethod:    req.PaymentMethod,
		Activated:        false,
		RemainingMinutes: combo.TotalMinutes,
		ExpiresAt:        expiresAt,
	}

	if err := s.db.Create(&purchase).Error; err != nil {
		return nil, err
	}

	var member model.Member
	if err := s.db.First(&member, "id = ?", memberID).Error; err == nil {
		trans := model.MemberTransaction{
			MemberID:        memberID,
			TransactionType: "combo_purchase",
			Amount:          -combo.Price,
			BalanceBefore:   member.Balance,
			BalanceAfter:    member.Balance - combo.Price,
			PaymentMethod:   req.PaymentMethod,
			ReferenceID:     &purchase.ID,
			Description:     fmt.Sprintf("Purchase combo: %s", combo.Name),
			CreatedAt:       time.Now(),
		}
		if storeID != "" {
			storeStr := storeID
			trans.StoreID = &storeStr
		}
		if userID != "" {
			trans.CreatedBy = &userID
		}
		s.db.Create(&trans)

		s.db.Model(&member).Update("total_spent", member.TotalSpent+combo.Price)
	}

	auditMetadata := map[string]interface{}{
		"combo_name":     combo.Name,
		"amount":         combo.Price,
		"payment_method": req.PaymentMethod,
		"member_id":      memberID,
	}
	if req.MemberID == "" {
		auditMetadata["new_member"] = true
	}
	var uid *string
	if userID != "" {
		uid = &userID
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "purchase",
		EntityType: "combo",
		EntityID:   purchase.ID,
		UserID:     uid,
		Metadata:   auditMetadata,
	})

	result := purchaseToResponse(purchase)
	return &result, nil
}

func (s *ComboService) Activate(purchaseID string, req *ActivateComboRequest, storeID string) (*ComboPurchaseResponse, error) {
	var purchase model.ComboPurchase
	if err := s.db.Where("id = ?", purchaseID).First(&purchase).Error; err != nil {
		return nil, errors.New("purchase not found")
	}

	if purchase.Activated {
		return nil, errors.New("purchase already activated")
	}

	var combo model.Combo
	if err := s.db.First(&combo, "id = ?", purchase.ComboID).Error; err != nil {
		return nil, errors.New("combo not found")
	}

	now := time.Now()
	activatedAt := now
	var slotEnd *time.Time
	var remainingMinutes *int

	if combo.SlotEnd != "" {
		parts := strings.Split(combo.SlotEnd, ":")
		if len(parts) >= 2 {
			h, m := 0, 0
			fmt.Sscanf(parts[0], "%d", &h)
			fmt.Sscanf(parts[1], "%d", &m)
			endTime := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
			if endTime.Before(now) {
				endTime = endTime.Add(24 * time.Hour)
			}
			slotEnd = &endTime
		}
	}

	rm := combo.TotalMinutes
	remainingMinutes = &rm

	session := model.MachineSession{
		MachineID:        req.MachineID,
		MemberID:         &purchase.MemberID,
		ComboType:        combo.Type,
		ComboID:          &combo.ID,
		SlotEnd:          slotEnd,
		RemainingMinutes: remainingMinutes,
		StartedAt:        now,
		IsActive:         true,
	}
	if storeID != "" {
		storeStr := storeID
		session.StoreID = &storeStr
	}

	if err := s.db.Create(&session).Error; err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"activated":         true,
		"activated_at":      activatedAt,
		"current_session_id": session.ID,
	}
	s.db.Model(&purchase).Updates(updates)

	purchase.Activated = true
	purchase.ActivatedAt = &activatedAt
	purchase.CurrentSessionID = &session.ID

	s.audit.Log(&LogAuditRequest{
		Action:     "activate",
		EntityType: "combo_purchase",
		EntityID:   purchaseID,
		Metadata: map[string]interface{}{
			"combo_id":   combo.ID,
			"combo_name": combo.Name,
			"machine_id": req.MachineID,
			"session_id": session.ID,
		},
	})

	result := purchaseToResponse(purchase)
	return &result, nil
}

func comboToResponse(c model.Combo) ComboResponse {
	return ComboResponse{
		ID:           c.ID,
		Name:         c.Name,
		Description:  c.Description,
		Type:         c.Type,
		SlotStart:    c.SlotStart,
		SlotEnd:      c.SlotEnd,
		ApplyDays:    c.ApplyDays,
		TotalMinutes: c.TotalMinutes,
		ValidityDays: c.ValidityDays,
		Price:        c.Price,
		MemberPrefix: c.MemberPrefix,
		MemberCount:  c.MemberCount,
		IsActive:     c.IsActive,
		CreatedAt:    c.CreatedAt.Format(time.RFC3339),
	}
}

type ComboPurchaseResponse struct {
	ID               string     `json:"id"`
	ComboID          string     `json:"combo_id"`
	MemberID         string     `json:"member_id"`
	Price            int64      `json:"price"`
	PaymentMethod    string     `json:"payment_method"`
	Activated        bool       `json:"activated"`
	ActivatedAt      *time.Time `json:"activated_at"`
	CurrentSessionID *string    `json:"current_session_id"`
	RemainingMinutes int        `json:"remaining_minutes"`
	ExpiresAt        *time.Time `json:"expires_at"`
	CreatedAt        time.Time  `json:"created_at"`
}

func purchaseToResponse(p model.ComboPurchase) ComboPurchaseResponse {
	return ComboPurchaseResponse{
		ID:               p.ID,
		ComboID:          p.ComboID,
		MemberID:         p.MemberID,
		Price:            p.Price,
		PaymentMethod:    p.PaymentMethod,
		Activated:        p.Activated,
		ActivatedAt:      p.ActivatedAt,
		CurrentSessionID: p.CurrentSessionID,
		RemainingMinutes: p.RemainingMinutes,
		ExpiresAt:        p.ExpiresAt,
		CreatedAt:        p.CreatedAt,
	}
}

func generateMemberPrefix(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "VIP"
	}
	prefix := strings.ToUpper(parts[0])
	if len(prefix) > 5 {
		prefix = prefix[:5]
	}
	return prefix
}
