package service

import (
	"errors"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
)

type StoreService struct {
	db    *gorm.DB
	audit *AuditService
}

func NewStoreService(db *gorm.DB, audit *AuditService) *StoreService {
	return &StoreService{db: db, audit: audit}
}

type CreateStoreRequest struct {
	Name    string `json:"name" binding:"required"`
	Code    string `json:"code" binding:"required"`
	Address string `json:"address"`
	Phone   string `json:"phone"`
}

type UpdateStoreRequest struct {
	Name    *string `json:"name"`
	Code    *string `json:"code"`
	Address *string `json:"address"`
	Phone   *string `json:"phone"`
	IsActive *bool  `json:"is_active"`
}

func (s *StoreService) List(params pagination.Params) ([]model.Store, int64, int, int, error) {
	query := s.db.Model(&model.Store{})

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	var stores []model.Store
	if err := pagination.Apply(query, &params).Find(&stores).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	return stores, total, params.Page, params.PageSize, nil
}

func (s *StoreService) GetByID(id string) (*model.Store, error) {
	var store model.Store
	if err := s.db.Where("id = ?", id).First(&store).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("store not found")
		}
		return nil, err
	}
	return &store, nil
}

func (s *StoreService) Create(req *CreateStoreRequest) (*model.Store, error) {
	store := model.Store{
		Name:    req.Name,
		Code:    req.Code,
		Address: req.Address,
		Phone:   req.Phone,
		IsActive: true,
	}
	if err := s.db.Create(&store).Error; err != nil {
		return nil, err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "store",
		EntityID:   store.ID,
		UserID:     nil,
		Metadata:   map[string]interface{}{"name": store.Name, "code": store.Code},
		IPAddress:  "",
	})
	return &store, nil
}

func (s *StoreService) Update(id string, req *UpdateStoreRequest) (*model.Store, error) {
	var store model.Store
	if err := s.db.Where("id = ?", id).First(&store).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("store not found")
		}
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Code != nil {
		updates["code"] = *req.Code
	}
	if req.Address != nil {
		updates["address"] = *req.Address
	}
	if req.Phone != nil {
		updates["phone"] = *req.Phone
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) > 0 {
		if err := s.db.Model(&store).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "update",
		EntityType: "store",
		EntityID:   id,
		UserID:     nil,
		Metadata:   updates,
		IPAddress:  "",
	})
	return &store, nil
}

func (s *StoreService) Delete(id string) error {
	var store model.Store
	if err := s.db.Where("id = ?", id).First(&store).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("store not found")
		}
		return err
	}
	if err := s.db.Delete(&store).Error; err != nil {
		return err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "store",
		EntityID:   store.ID,
		UserID:     nil,
		Metadata:   map[string]interface{}{"name": store.Name, "code": store.Code},
		IPAddress:  "",
	})
	return nil
}
