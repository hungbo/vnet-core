package model

import (
	"time"

	"gorm.io/gorm"
)

type Supplier struct {
	ID        string         `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name      string         `gorm:"type:varchar(200);not null" json:"name"`
	Phone     string         `gorm:"type:varchar(20)" json:"phone"`
	Email     string         `gorm:"type:varchar(100)" json:"email"`
	Address   string         `gorm:"type:text" json:"address"`
	IsActive  bool           `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time      `gorm:"default:now()" json:"created_at,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

type StockTransaction struct {
	ID              string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	ProductID       *string   `gorm:"type:uuid;index;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"product_id"`
	TransactionType string    `gorm:"type:varchar(30);not null" json:"transaction_type"`
	Quantity        float64   `gorm:"type:decimal(12,3);not null" json:"quantity"`
	UnitPrice       int64     `gorm:"column:unit_price" json:"unit_price"`
	TotalPrice      int64     `gorm:"column:total_price" json:"total_price"`
	StockBefore     float64   `gorm:"type:decimal(12,3)" json:"stock_before"`
	StockAfter      float64   `gorm:"type:decimal(12,3)" json:"stock_after"`
	ReferenceID     *string   `gorm:"type:uuid" json:"reference_id"`
	SupplierID      *string   `gorm:"type:uuid;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"supplier_id"`
	Description     string    `gorm:"type:text" json:"description"`
	CreatedBy       *string   `gorm:"type:uuid;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"created_by"`
	CreatedAt       time.Time `gorm:"default:now();index" json:"created_at,omitempty"`
}

// InventoryCountSession gom các dòng đếm của một lần kiểm kê.
//
// Không có phiên thì mỗi dòng đếm là một lần điều chỉnh kho riêng lẻ: không
// xem lại được trước khi chốt, và không trả lời được "kỳ kiểm kê tháng 10 lệch
// bao nhiêu".
type InventoryCountSession struct {
	ID          string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Code        string     `gorm:"type:varchar(30);not null;uniqueIndex" json:"code"`
	Note        string     `gorm:"type:text" json:"note"`
	Status      string     `gorm:"type:varchar(20);default:open;index" json:"status"`
	OpenedBy    *string    `gorm:"type:uuid;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"opened_by"`
	OpenedAt    time.Time  `gorm:"default:now()" json:"opened_at"`
	CommittedBy *string    `gorm:"type:uuid;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"committed_by"`
	CommittedAt *time.Time `gorm:"type:timestamptz" json:"committed_at"`
}

type InventoryCount struct {
	ID            string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	// Một dòng đếm luôn thuộc về một phiên.
	SessionID     string    `gorm:"type:uuid;not null;index;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"session_id"`
	ProductID     string    `gorm:"type:uuid;not null;index;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"product_id"`
	ExpectedQty   float64   `gorm:"type:decimal(12,3)" json:"expected_qty"`
	ActualQty     float64   `gorm:"type:decimal(12,3)" json:"actual_qty"`
	DifferenceQty float64   `gorm:"type:decimal(12,3)" json:"difference_qty"`
	CountedBy     *string   `gorm:"type:uuid;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"counted_by"`
	CountedAt     time.Time `gorm:"default:now()" json:"counted_at"`
}
