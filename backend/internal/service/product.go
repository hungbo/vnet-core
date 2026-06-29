package service

import (
	"errors"
	"math"
	"time"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
)

type ProductService struct {
	db    *gorm.DB
	audit *AuditService
}

func NewProductService(db *gorm.DB, audit *AuditService) *ProductService {
	return &ProductService{db: db, audit: audit}
}

type ProductOptionResponse struct {
	ID             string  `json:"id"`
	GroupID        string  `json:"group_id"`
	Name           string  `json:"name"`
	CurrentPrice   int64   `json:"current_price"`
	IngredientID   *string `json:"ingredient_id"`
	IngredientName string  `json:"ingredient_name"`
	Quantity       float64 `json:"quantity"`
	SortOrder      int     `json:"sort_order"`
}

type ProductResponse struct {
	ID           string                      `json:"id"`
	CategoryID   *string                     `json:"category_id"`
	Name         string                      `json:"name"`
	Description  string                      `json:"description"`
	Price        int64                       `json:"price"`
	ImageURL     string                      `json:"image_url"`
	SupplierID   *string                     `json:"supplier_id"`
	SupplierName string                      `json:"supplier_name"`
	UnitID       *string                     `json:"unit_id"`
	MinStock     float64                     `json:"min_stock"`
	IsRetail     bool                        `json:"is_retail"`
	IsActive     bool                        `json:"is_active"`
	HasStock     bool                        `json:"has_stock"`
	CurrentStock float64                     `json:"current_stock"`
	SortOrder    int                         `json:"sort_order"`
	CreatedAt    string                      `json:"created_at"`
	Options      []ProductOptionResponse     `json:"options,omitempty"`
	Ingredients  []ProductIngredientResponse `json:"ingredients,omitempty"`
}

type ProductOptionItem struct {
	GroupID      string  `json:"group_id"`
	Name         string  `json:"name" binding:"required"`
	IngredientID *string `json:"ingredient_id"`
	Quantity     float64 `json:"quantity"`
	SortOrder    int     `json:"sort_order"`
}

type CreateProductRequest struct {
	CategoryID   *string             `json:"category_id"`
	Name         string              `json:"name" binding:"required"`
	Description  string              `json:"description"`
	Price        int64               `json:"price" binding:"min=0"`
	ImageURL     string              `json:"image_url"`
	SupplierID   *string             `json:"supplier_id"`
	UnitID       *string             `json:"unit_id"`
	MinStock     float64             `json:"min_stock"`
	IsRetail     *bool               `json:"is_retail"`
	IsActive     *bool               `json:"is_active"`
	HasStock     bool                `json:"has_stock"`
	CurrentStock float64             `json:"current_stock"`
	SortOrder    int                 `json:"sort_order"`
	Options      []ProductOptionItem `json:"options"`
}

type UpdateProductRequest struct {
	CategoryID   *string             `json:"category_id"`
	Name         *string             `json:"name"`
	Description  *string             `json:"description"`
	Price        *int64              `json:"price" binding:"min=0"`
	ImageURL     *string             `json:"image_url"`
	SupplierID   *string             `json:"supplier_id"`
	UnitID       *string             `json:"unit_id"`
	MinStock     *float64            `json:"min_stock"`
	IsRetail     *bool               `json:"is_retail"`
	IsActive     *bool               `json:"is_active"`
	HasStock     *bool               `json:"has_stock"`
	CurrentStock *float64            `json:"current_stock"`
	SortOrder    *int                `json:"sort_order"`
	Options      []ProductOptionItem `json:"options"`
}

func (s *ProductService) List(isRetail *bool, search string, page int, pageSize int) (*pagination.Result, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	query := s.db.Where("deleted_at IS NULL")
	if isRetail != nil {
		query = query.Where("is_retail = ?", *isRetail)
	}
	if search != "" {
		query = query.Where("unaccent(name) ILIKE unaccent(?) OR unaccent(description) ILIKE unaccent(?)", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Model(&model.Product{}).Count(&total)

	var products []model.Product
	if err := query.Order("sort_order asc, name asc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&products).Error; err != nil {
		return nil, err
	}

	responses := make([]ProductResponse, len(products))
	for i, p := range products {
		responses[i] = productToResponse(s.db, p)
		responses[i].Options = s.loadOptions(p.ID)
		ingredients, stock := s.loadProductStock(p)
		responses[i].Ingredients = ingredients
		if p.HasStock {
			responses[i].CurrentStock = stock
		}
	}

	return pagination.NewResult(responses, total, &pagination.Params{Page: page, PageSize: pageSize}), nil
}

func (s *ProductService) GetByID(id string) (*ProductResponse, error) {
	var product model.Product
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy sản phẩm")
		}
		return nil, err
	}

	result := productToResponse(s.db, product)
	result.Options = s.loadOptions(product.ID)
	ingredients, stock := s.loadProductStock(product)
	result.Ingredients = ingredients
	if product.HasStock {
		result.CurrentStock = stock
	}
	return &result, nil
}

func (s *ProductService) Create(req *CreateProductRequest) (*ProductResponse, error) {
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	isRetail := true
	if req.IsRetail != nil {
		isRetail = *req.IsRetail
	}

	product := model.Product{
		CategoryID:   req.CategoryID,
		Name:         req.Name,
		Description:  req.Description,
		Price:        req.Price,
		ImageURL:     req.ImageURL,
		SupplierID:   req.SupplierID,
		UnitID:       req.UnitID,
		MinStock:     req.MinStock,
		IsRetail:     isRetail,
		IsActive:     active,
		HasStock:     req.HasStock,
		CurrentStock: req.CurrentStock,
		SortOrder:    req.SortOrder,
	}

	if err := s.db.Create(&product).Error; err != nil {
		return nil, err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "product",
		EntityID:   product.ID,
		UserID:     nil,
		Metadata:   map[string]interface{}{"name": product.Name},
		IPAddress:  "",
	})

	if len(req.Options) > 0 {
		defaultGroup := s.getOrCreateDefaultOptionGroup()
		for _, opt := range req.Options {
			qty := opt.Quantity
			if qty <= 0 {
				qty = 1
			}
			option := model.ProductOption{
				GroupID:      defaultGroup.ID,
				ProductID:    product.ID,
				Name:         opt.Name,
				IngredientID: opt.IngredientID,
				Quantity:     qty,
				SortOrder:    opt.SortOrder,
			}
			if err := s.db.Create(&option).Error; err != nil {
				return nil, err
			}
		}
	}

	result := productToResponse(s.db, product)
	result.Options = s.loadOptions(product.ID)
	ingredients, stock := s.loadProductStock(product)
	result.Ingredients = ingredients
	if product.HasStock {
		result.CurrentStock = stock
	}
	return &result, nil
}

func (s *ProductService) Update(id string, req *UpdateProductRequest) (*ProductResponse, error) {
	var product model.Product
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy sản phẩm")
		}
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.CategoryID != nil {
		updates["category_id"] = *req.CategoryID
	}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Price != nil {
		updates["price"] = *req.Price
	}
	if req.ImageURL != nil {
		updates["image_url"] = *req.ImageURL
	}
	if req.SupplierID != nil {
		updates["supplier_id"] = *req.SupplierID
	}
	if req.UnitID != nil {
		updates["unit_id"] = *req.UnitID
	}
	if req.MinStock != nil {
		updates["min_stock"] = *req.MinStock
	}
	if req.IsRetail != nil {
		updates["is_retail"] = *req.IsRetail
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.HasStock != nil {
		updates["has_stock"] = *req.HasStock
	}
	if req.CurrentStock != nil {
		updates["current_stock"] = *req.CurrentStock
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}

	if len(updates) > 0 {
		if err := s.db.Model(&product).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "update",
		EntityType: "product",
		EntityID:   id,
		UserID:     nil,
		Metadata:   updates,
		IPAddress:  "",
	})

	if req.Options != nil {
		s.db.Where("product_id = ?", id).Delete(&model.ProductOption{})
		defaultGroup := s.getOrCreateDefaultOptionGroup()
		for _, opt := range req.Options {
			qty := opt.Quantity
			if qty <= 0 {
				qty = 1
			}
			option := model.ProductOption{
				GroupID:      defaultGroup.ID,
				ProductID:    id,
				Name:         opt.Name,
				IngredientID: opt.IngredientID,
				Quantity:     qty,
				SortOrder:    opt.SortOrder,
			}
			if err := s.db.Create(&option).Error; err != nil {
				return nil, err
			}
		}
	}

	s.db.First(&product, "id = ?", id)
	result := productToResponse(s.db, product)
	result.Options = s.loadOptions(product.ID)
	ingredients, stock := s.loadProductStock(product)
	result.Ingredients = ingredients
	if product.HasStock {
		result.CurrentStock = stock
	}
	return &result, nil
}

func (s *ProductService) Delete(id string) error {
	var product model.Product
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("không tìm thấy sản phẩm")
		}
		return err
	}

	now := time.Now()
	if err := s.db.Model(&product).Update("deleted_at", &now).Error; err != nil {
		return err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "product",
		EntityID:   product.ID,
		UserID:     nil,
		Metadata:   map[string]interface{}{"name": product.Name},
		IPAddress:  "",
	})
	return nil
}

func (s *ProductService) loadProductStock(product model.Product) ([]ProductIngredientResponse, float64) {
	var pms []model.ProductIngredient
	s.db.Where("product_id = ?", product.ID).Find(&pms)

	if len(pms) > 0 {
		responses := make([]ProductIngredientResponse, len(pms))
		for i, pm := range pms {
			responses[i] = productIngredientToResponse(s.db, pm)
		}

		if !product.HasStock {
			return responses, 0
		}

		minStock := math.MaxFloat64
		for _, pm := range pms {
			var ingredient model.Product
			if err := s.db.Where("id = ?", pm.IngredientID).First(&ingredient).Error; err == nil {
				materialStock := ingredient.CurrentStock / pm.Quantity
				if materialStock < minStock {
					minStock = materialStock
				}
			}
		}
		return responses, minStock
	}

	if product.HasStock {
		return nil, product.CurrentStock
	}
	return nil, 0
}

func (s *ProductService) getOrCreateDefaultOptionGroup() model.ProductOptionGroup {
	var group model.ProductOptionGroup
	s.db.Where("name = ?", "_default").FirstOrCreate(&group, model.ProductOptionGroup{
		Name: "_default",
	})
	return group
}

func (s *ProductService) loadOptions(productID string) []ProductOptionResponse {
	var options []model.ProductOption
	s.db.Where("product_id = ?", productID).Order("sort_order asc").Find(&options)

	var ingredientIDs []string
	for _, o := range options {
		if o.IngredientID != nil {
			ingredientIDs = append(ingredientIDs, *o.IngredientID)
		}
	}

	ingredientPrices := make(map[string]int64)
	ingredientNames := make(map[string]string)
	if len(ingredientIDs) > 0 {
		var ingredients []model.Product
		s.db.Select("id, name, price").Where("id IN ?", ingredientIDs).Find(&ingredients)
		for _, ing := range ingredients {
			ingredientPrices[ing.ID] = ing.Price
			ingredientNames[ing.ID] = ing.Name
		}
	}

	resp := make([]ProductOptionResponse, len(options))
	for i, o := range options {
		price := int64(0)
		name := ""
		if o.IngredientID != nil {
			if p, ok := ingredientPrices[*o.IngredientID]; ok && p > 0 {
				price = p
			}
			name = ingredientNames[*o.IngredientID]
		}
		resp[i] = ProductOptionResponse{
			ID:             o.ID,
			GroupID:        o.GroupID,
			Name:           o.Name,
			CurrentPrice:   price,
			IngredientID:   o.IngredientID,
			IngredientName: name,
			Quantity:       o.Quantity,
			SortOrder:      o.SortOrder,
		}
	}

	return resp
}

func productToResponse(db *gorm.DB, p model.Product) ProductResponse {
	supplierName := ""
	if p.SupplierID != nil {
		var supplier model.Supplier
		if err := db.Where("id = ?", *p.SupplierID).First(&supplier).Error; err == nil {
			supplierName = supplier.Name
		}
	}

	return ProductResponse{
		ID:           p.ID,
		CategoryID:   p.CategoryID,
		Name:         p.Name,
		Description:  p.Description,
		Price:        p.Price,
		ImageURL:     p.ImageURL,
		SupplierID:   p.SupplierID,
		SupplierName: supplierName,
		UnitID:       p.UnitID,
		MinStock:     p.MinStock,
		IsRetail:     p.IsRetail,
		IsActive:     p.IsActive,
		HasStock:     p.HasStock,
		CurrentStock: p.CurrentStock,
		SortOrder:    p.SortOrder,
		CreatedAt:    p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
