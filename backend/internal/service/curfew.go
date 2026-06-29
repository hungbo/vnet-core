package service

import (
	"errors"
	"time"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
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
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Sort     string `form:"sort"`
	Order    string `form:"order"`
	Search   string `form:"search"`
	DayOfWeek *int  `form:"day_of_week"`
}

type CurfewResponse struct {
	ID              string  `json:"id"`
	DayOfWeek       int     `json:"day_of_week"`
	CurfewStart     string  `json:"curfew_start"`
	CurfewEnd       string  `json:"curfew_end"`
	MaxMinorHours   int     `json:"max_minor_hours"`
	IsActive        bool    `json:"is_active"`
	StoreID         *string `json:"store_id"`
	OverrideByAdmin *string `json:"override_by_admin"`
	OverrideReason  string  `json:"override_reason"`
	OverrideAt      *string `json:"override_at"`
	CreatedAt       string  `json:"created_at"`
}

type CreateCurfewRequest struct {
	DayOfWeek     int    `json:"day_of_week" binding:"required,min=0,max=6"`
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
	PolicyID        string `json:"policy_id" binding:"required"`
	OverrideReason  string `json:"override_reason" binding:"required"`
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
		return nil, errors.New("curfew policy not found")
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
		StoreID:         p.StoreID,
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
