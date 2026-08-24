package service

import (
	"errors"
	"time"

	"github.com/vnet/core/internal/model"
	"gorm.io/gorm"
)

type CategoryService struct {
	db    *gorm.DB
	audit *AuditService
}

func NewCategoryService(db *gorm.DB, audit *AuditService) *CategoryService {
	return &CategoryService{db: db, audit: audit}
}

type CategoryResponse struct {
	ID        string             `json:"id"`
	Name      string             `json:"name"`
	ParentID  *string            `json:"parent_id"`
	Icon      string             `json:"icon"`
	PrinterID *string            `json:"printer_id"`
	SortOrder int                `json:"sort_order"`
	IsActive  bool               `json:"is_active"`
	CreatedAt string             `json:"created_at"`
	Children  []CategoryResponse `json:"children,omitempty"`
}

type CreateCategoryRequest struct {
	Name      string  `json:"name" binding:"required"`
	ParentID  *string `json:"parent_id"`
	Icon      string  `json:"icon"`
	PrinterID *string `json:"printer_id"`
	SortOrder int     `json:"sort_order"`
	IsActive  *bool   `json:"is_active"`
}

type UpdateCategoryRequest struct {
	Name      *string `json:"name"`
	ParentID  *string `json:"parent_id"`
	Icon      *string `json:"icon"`
	PrinterID *string `json:"printer_id"`
	SortOrder *int    `json:"sort_order"`
	IsActive  *bool   `json:"is_active"`
}

func (s *CategoryService) List() ([]CategoryResponse, error) {
	var categories []model.Category
	if err := s.db.Order("sort_order asc, name asc").Find(&categories).Error; err != nil {
		return nil, err
	}

	catMap := make(map[string]CategoryResponse)
	for _, c := range categories {
		catMap[c.ID] = categoryToResponse(c)
	}

	var roots []CategoryResponse
	for _, c := range categories {
		resp := catMap[c.ID]
		if c.ParentID == nil {
			roots = append(roots, resp)
		} else {
			parent, ok := catMap[*c.ParentID]
			if ok {
				parent.Children = append(parent.Children, resp)
				catMap[*c.ParentID] = parent
			}
		}
	}

	return roots, nil
}

func (s *CategoryService) GetByID(id string) (*CategoryResponse, error) {
	var category model.Category
	if err := s.db.Where("id = ?", id).First(&category).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("category not found")
		}
		return nil, err
	}

	var children []model.Category
	s.db.Where("parent_id = ?", id).Order("sort_order asc").Find(&children)

	result := categoryToResponse(category)
	for _, ch := range children {
		result.Children = append(result.Children, categoryToResponse(ch))
	}
	return &result, nil
}

func (s *CategoryService) Create(req *CreateCategoryRequest) (*CategoryResponse, error) {
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}

	category := model.Category{
		Name:      req.Name,
		ParentID:  req.ParentID,
		Icon:      req.Icon,
		PrinterID: req.PrinterID,
		SortOrder: req.SortOrder,
		IsActive:  active,
	}

	if err := s.db.Create(&category).Error; err != nil {
		return nil, err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "category",
		EntityID:   category.ID,
		UserID:     nil,
		Metadata:   map[string]interface{}{"name": category.Name},
		IPAddress:  "",
	})

	result := categoryToResponse(category)
	return &result, nil
}

func (s *CategoryService) Update(id string, req *UpdateCategoryRequest) (*CategoryResponse, error) {
	var category model.Category
	if err := s.db.Where("id = ?", id).First(&category).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("category not found")
		}
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.ParentID != nil {
		updates["parent_id"] = *req.ParentID
	}
	if req.Icon != nil {
		updates["icon"] = *req.Icon
	}
	if req.PrinterID != nil {
		updates["printer_id"] = *req.PrinterID
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) > 0 {
		if err := s.db.Model(&category).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "update",
		EntityType: "category",
		EntityID:   id,
		UserID:     nil,
		Metadata:   updates,
		IPAddress:  "",
	})

	s.db.First(&category, "id = ?", id)
	result := categoryToResponse(category)
	return &result, nil
}

func (s *CategoryService) Delete(id string) error {
	var category model.Category
	if err := s.db.Where("id = ?", id).First(&category).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("category not found")
		}
		return err
	}

	// Xoá danh mục vẫn còn sản phẩm sẽ để lại sản phẩm mồ côi: giao diện tra tên
	// danh mục trong danh sách đang hoạt động, không thấy, nên hiện UUID thô.
	// Chỉ đếm sản phẩm chưa xoá — sản phẩm đã xoá mềm mà vẫn chặn thì danh mục
	// không bao giờ xoá được nữa.
	var productCount int64
	if err := s.db.Model(&model.Product{}).Where("category_id = ?", id).Count(&productCount).Error; err != nil {
		return err
	}
	if productCount > 0 {
		return errors.New("cannot delete category with products")
	}

	var childCount int64
	if err := s.db.Model(&model.Category{}).Where("parent_id = ?", id).Count(&childCount).Error; err != nil {
		return err
	}
	if childCount > 0 {
		return errors.New("cannot delete category with sub-categories")
	}

	now := time.Now()
	if err := s.db.Model(&category).Update("deleted_at", &now).Error; err != nil {
		return err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "category",
		EntityID:   category.ID,
		UserID:     nil,
		Metadata:   map[string]interface{}{"name": category.Name},
		IPAddress:  "",
	})
	return nil
}

func categoryToResponse(c model.Category) CategoryResponse {
	return CategoryResponse{
		ID:        c.ID,
		Name:      c.Name,
		ParentID:  c.ParentID,
		Icon:      c.Icon,
		PrinterID: c.PrinterID,
		SortOrder: c.SortOrder,
		IsActive:  c.IsActive,
		CreatedAt: c.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
