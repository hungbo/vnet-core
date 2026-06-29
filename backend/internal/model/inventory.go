package model

import "time"

type Supplier struct {
	ID        string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name      string     `gorm:"type:varchar(200);not null" json:"name"`
	Phone     string     `gorm:"type:varchar(20)" json:"phone"`
	Email     string     `gorm:"type:varchar(100)" json:"email"`
	Address   string     `gorm:"type:text" json:"address"`
	IsActive  bool       `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time  `gorm:"default:now()" json:"created_at,omitempty"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

type Warehouse struct {
	ID        string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name      string     `gorm:"type:varchar(100);not null" json:"name"`
	Address   string     `gorm:"type:text" json:"address"`
	IsActive  bool       `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time  `gorm:"default:now()" json:"created_at,omitempty"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

type StockTransaction struct {
	ID              string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	ProductID       *string   `gorm:"type:uuid;index" json:"product_id"`
	TransactionType string    `gorm:"type:varchar(30);not null" json:"transaction_type"`
	Quantity        float64   `gorm:"type:decimal(12,3);not null" json:"quantity"`
	UnitPrice       int64     `gorm:"column:unit_price" json:"unit_price"`
	TotalPrice      int64     `gorm:"column:total_price" json:"total_price"`
	StockBefore     float64   `gorm:"type:decimal(12,3)" json:"stock_before"`
	StockAfter      float64   `gorm:"type:decimal(12,3)" json:"stock_after"`
	ReferenceID     *string   `gorm:"type:uuid" json:"reference_id"`
	SupplierID      *string   `gorm:"type:uuid" json:"supplier_id"`
	WarehouseID     *string   `gorm:"type:uuid" json:"warehouse_id"`
	Description     string    `gorm:"type:text" json:"description"`
	StoreID         *string   `gorm:"type:uuid;index" json:"store_id"`
	CreatedBy       *string   `gorm:"type:uuid" json:"created_by"`
	CreatedAt       time.Time `gorm:"default:now();index" json:"created_at,omitempty"`
}

type InventoryCount struct {
	ID            string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	WarehouseID   *string   `gorm:"type:uuid" json:"warehouse_id"`
	ProductID     string    `gorm:"type:uuid;not null;index" json:"product_id"`
	ExpectedQty   float64   `gorm:"type:decimal(12,3)" json:"expected_qty"`
	ActualQty     float64   `gorm:"type:decimal(12,3)" json:"actual_qty"`
	DifferenceQty float64   `gorm:"type:decimal(12,3)" json:"difference_qty"`
	CountedBy     *string   `gorm:"type:uuid" json:"counted_by"`
	CountedAt     time.Time `gorm:"default:now()" json:"counted_at"`
}
