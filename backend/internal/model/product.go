package model

import (
	"time"

	"gorm.io/gorm"
)

type Product struct {
	ID           string         `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	CategoryID   *string        `gorm:"type:uuid;index;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"category_id"`
	Name         string         `gorm:"type:varchar(200);not null;uniqueIndex" json:"name"`
	Description  string         `gorm:"type:text" json:"description"`
	Price        int64          `gorm:"not null" json:"price"`
	ImageURL     string         `gorm:"type:text" json:"image_url"`
	SupplierID   *string        `gorm:"type:uuid;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"supplier_id"`
	UnitID       *string        `gorm:"type:varchar(20)" json:"unit_id"`
	MinStock     float64        `gorm:"type:decimal(12,3);default:0" json:"min_stock"`
	IsRetail     bool           `gorm:"<-:create" json:"is_retail"`
	IsActive     bool           `gorm:"default:true" json:"is_active"`
	HasStock     bool           `gorm:"default:false" json:"has_stock"`
	CurrentStock float64        `gorm:"type:decimal(12,3);default:0" json:"current_stock"`
	SortOrder    int            `gorm:"default:0" json:"sort_order"`
	CreatedAt    time.Time      `gorm:"default:now()" json:"created_at,omitempty"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

type ProductIngredient struct {
	ID           string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	ProductID    string    `gorm:"type:uuid;not null;uniqueIndex:idx_product_ingredient;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"product_id"`
	IngredientID string    `gorm:"type:uuid;not null;uniqueIndex:idx_product_ingredient;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"ingredient_id"`
	Quantity     float64   `gorm:"type:decimal(12,3);not null" json:"quantity"`
	UnitID       string    `gorm:"type:varchar(20);not null" json:"unit_id"`
	CreatedAt    time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}

// ProductOptionGroup: hệ thống chỉ tạo đúng một nhóm "_default" (service/product.go)
// và không có CRUD nhóm tuỳ chọn, nên hai cột is_required và max_select trước đây
// nằm trong model mà không nơi nào đọc.
//
// Đã GỠ khỏi model thay vì thực thi: bật chúng lên với giá trị mặc định
// (bắt buộc chọn, tối đa 1) sẽ chặn những đơn nhiều tuỳ chọn mà quán đang tạo
// bình thường — một quy tắc chưa ai cấu hình được thì không nên tự bật. Cột cũ
// còn lại trong database là vô hại; AutoMigrate không xoá cột.
type ProductOptionGroup struct {
	ID        string `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name      string `gorm:"type:varchar(100);not null;uniqueIndex" json:"name"`
	SortOrder int    `gorm:"default:0" json:"sort_order"`
}

type ProductOption struct {
	ID           string  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	GroupID      string  `gorm:"type:uuid;not null;uniqueIndex:idx_option_group_name;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"group_id"`
	ProductID    string  `gorm:"type:uuid;not null;index;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"product_id"`
	Name         string  `gorm:"type:varchar(100);not null;uniqueIndex:idx_option_group_name" json:"name"`
	IngredientID *string `gorm:"type:uuid;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"ingredient_id"`
	Quantity     float64 `gorm:"type:decimal(12,3);default:1" json:"quantity"`
	SortOrder    int     `gorm:"default:0" json:"sort_order"`
}
