package model

import "time"

type MemberNotification struct {
	ID        string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MemberID  string    `gorm:"type:uuid;not null;index" json:"member_id"`
	Title     string    `gorm:"type:varchar(255);not null" json:"title"`
	Body      string    `gorm:"type:text" json:"body"`
	IsRead    bool      `gorm:"default:false" json:"is_read"`
	StoreID   *string   `gorm:"type:uuid;index" json:"store_id,omitempty"`
	CreatedAt time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}
