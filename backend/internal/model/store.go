package model

import "time"

type Store struct {
	ID        string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name      string     `gorm:"type:varchar(100);not null" json:"name"`
	Code      string     `gorm:"type:varchar(20);not null;uniqueIndex" json:"code"`
	Address   string     `gorm:"type:text" json:"address"`
	Phone     string     `gorm:"type:varchar(20)" json:"phone"`
	IsActive  bool       `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time  `gorm:"default:now()" json:"created_at,omitempty"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}
