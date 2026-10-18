package model

import (
	"time"

	"gorm.io/gorm"
)

type PrinterConfig struct {
	ID          string         `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name        string         `gorm:"type:varchar(100);not null" json:"name"`
	PrinterType string         `gorm:"type:varchar(20);not null" json:"printer_type"`
	IPAddress   string         `gorm:"type:varchar(45)" json:"ip_address"`
	Port        int            `gorm:"default:9100" json:"port"`
	IsDefault   bool           `gorm:"default:false" json:"is_default"`
	// Số ký tự mỗi dòng: 32 cho giấy 58mm, 48 cho giấy 80mm.
	CharsPerLine int `gorm:"default:32" json:"chars_per_line"`
	// "ascii" (bỏ dấu, chạy trên mọi máy) hoặc "cp1258" (in đúng dấu tiếng
	// Việt, cần máy in hỗ trợ). Xem escpos.go.
	Encoding string `gorm:"type:varchar(20);default:'ascii'" json:"encoding"`
	// Số hiệu bảng mã gửi kèm lệnh ESC t. Khác nhau theo hãng máy in; 0 =
	// không gửi lệnh chọn bảng mã.
	CodePage int `gorm:"default:0" json:"code_page"`
	CreatedAt   time.Time      `gorm:"default:now()" json:"created_at,omitempty"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

type ProductPrinterMapping struct {
	ID        string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	ProductID string    `gorm:"type:uuid;not null;index" json:"product_id"`
	PrinterID string    `gorm:"type:uuid;not null;index" json:"printer_id"`
	CreatedAt time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}
