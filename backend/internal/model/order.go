package model

import "time"

type Order struct {
	ID             string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	OrderCode      string     `gorm:"type:varchar(20);not null;uniqueIndex" json:"order_code"`
	Status         string     `gorm:"type:varchar(20);default:pending;index" json:"status"`
	MemberID       *string    `gorm:"type:uuid;index" json:"member_id"`
	MachineID      *string    `gorm:"type:uuid;index" json:"machine_id"`
	StoreID        *string    `gorm:"type:uuid;index" json:"store_id"`
	TableNumber    string     `gorm:"type:varchar(10)" json:"table_number"`
	TotalAmount    int64      `gorm:"not null" json:"total_amount"`
	DiscountAmount int64      `gorm:"default:0" json:"discount_amount"`
	FinalAmount    int64      `gorm:"not null" json:"final_amount"`
	Note           string     `gorm:"type:text" json:"note"`
	CreatedBy      *string    `gorm:"type:uuid" json:"created_by"`
	UpdatedBy      *string    `gorm:"type:uuid" json:"updated_by"`
	CompletedAt    *time.Time `gorm:"type:timestamptz" json:"completed_at"`
	CreatedAt      time.Time  `gorm:"default:now();index" json:"created_at,omitempty"`
	DeletedAt      *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

type OrderItem struct {
	ID          string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	OrderID     string    `gorm:"type:uuid;not null;index" json:"order_id"`
	ProductID   string    `gorm:"type:uuid;not null" json:"product_id"`
	ProductName string    `gorm:"type:varchar(200);not null" json:"product_name"`
	Quantity    int       `gorm:"not null;default:1" json:"quantity"`
	UnitPrice   int64     `gorm:"not null" json:"unit_price"`
	Options     string    `gorm:"type:jsonb" json:"options"`
	Subtotal    int64     `gorm:"not null" json:"subtotal"`
	StoreID     *string   `gorm:"type:uuid;index" json:"store_id"`
	Status      string    `gorm:"type:varchar(20);default:pending" json:"status"`
	Note        string    `gorm:"type:text" json:"note"`
	CreatedAt   time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}

type Payment struct {
	ID            string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	OrderID       string     `gorm:"type:uuid;not null;index" json:"order_id"`
	PaymentMethod string     `gorm:"type:varchar(30);not null" json:"payment_method"`
	Amount        int64      `gorm:"not null" json:"amount"`
	ReferenceCode string     `gorm:"type:varchar(100)" json:"reference_code"`
	Status        string     `gorm:"type:varchar(20);default:pending" json:"status"`
	PaidAt        *time.Time `gorm:"type:timestamptz" json:"paid_at"`
	CreatedAt     time.Time  `gorm:"default:now()" json:"created_at,omitempty"`
	DeletedAt     *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}
