package model

import "time"

type CurfewPolicy struct {
	ID              string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	DayOfWeek       int        `gorm:"not null" json:"day_of_week"`
	CurfewStart     string     `gorm:"type:time;not null" json:"curfew_start"`
	CurfewEnd       string     `gorm:"type:time;not null" json:"curfew_end"`
	MaxMinorHours   int        `gorm:"default:2" json:"max_minor_hours"`
	IsActive        bool       `gorm:"default:true" json:"is_active"`
	StoreID         *string    `gorm:"type:uuid;index" json:"store_id"`
	OverrideByAdmin *string    `gorm:"type:uuid" json:"override_by_admin"`
	OverrideReason  string     `gorm:"type:text" json:"override_reason"`
	OverrideAt      *time.Time `gorm:"type:timestamptz" json:"override_at"`
	CreatedAt       time.Time  `gorm:"default:now()" json:"created_at,omitempty"`
}
