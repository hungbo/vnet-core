package model

import "time"

type Category struct {
	ID        string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name      string     `gorm:"type:varchar(100);not null;uniqueIndex" json:"name"`
	ParentID  *string    `gorm:"type:uuid;index;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"parent_id"`
	Icon      string     `gorm:"type:varchar(50)" json:"icon"`
	PrinterID *string    `gorm:"type:uuid;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"printer_id"`
	SortOrder int        `gorm:"default:0" json:"sort_order"`
	IsActive  bool       `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time  `gorm:"default:now()" json:"created_at,omitempty"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}
