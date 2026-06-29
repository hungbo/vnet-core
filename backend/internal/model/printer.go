package model

import "time"

type PrinterConfig struct {
	ID          string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name        string     `gorm:"type:varchar(100);not null" json:"name"`
	PrinterType string     `gorm:"type:varchar(20);not null" json:"printer_type"`
	IPAddress   string     `gorm:"type:varchar(45)" json:"ip_address"`
	Port        int        `gorm:"default:9100" json:"port"`
	IsDefault   bool       `gorm:"default:false" json:"is_default"`
	StoreID     *string    `gorm:"type:uuid;index" json:"store_id"`
	CreatedAt   time.Time  `gorm:"default:now()" json:"created_at,omitempty"`
	DeletedAt   *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

type ProductPrinterMapping struct {
	ID        string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	ProductID string    `gorm:"type:uuid;not null;index" json:"product_id"`
	PrinterID string    `gorm:"type:uuid;not null;index" json:"printer_id"`
	CreatedAt time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}
