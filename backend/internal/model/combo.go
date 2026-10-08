package model

import (
	"time"

	"gorm.io/gorm"
)

type Combo struct {
	ID           string         `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name         string         `gorm:"type:varchar(100);not null;uniqueIndex" json:"name"`
	Description  string         `gorm:"type:text" json:"description"`
	Type         string         `gorm:"type:varchar(20);not null" json:"type"`
	SlotStart    *string        `gorm:"type:time without time zone" json:"slot_start"`
	SlotEnd      *string        `gorm:"type:time without time zone" json:"slot_end"`
	ApplyDays    IntArray       `gorm:"type:integer[]" json:"apply_days"`
	TotalMinutes int            `gorm:"column:total_minutes" json:"total_minutes"`
	ValidityDays int            `gorm:"column:validity_days" json:"validity_days"`
	Price        int64          `gorm:"not null" json:"price"`
	MemberPrefix string         `gorm:"type:varchar(20)" json:"member_prefix"`
	MemberCount  int            `gorm:"default:0" json:"member_count"`
	IsActive     bool           `gorm:"default:true" json:"is_active"`
	CreatedAt    time.Time      `gorm:"default:now()" json:"created_at,omitempty"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

type ComboPurchase struct {
	ID               string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	ComboID          string     `gorm:"type:uuid;not null;index" json:"combo_id"`
	MemberID         string     `gorm:"type:uuid;not null;index" json:"member_id"`
	Price            int64      `gorm:"not null" json:"price"`
	PaymentMethod    string     `gorm:"type:varchar(30)" json:"payment_method"`
	Activated        bool       `gorm:"default:false" json:"activated"`
	ActivatedAt      *time.Time `gorm:"type:timestamptz" json:"activated_at"`
	CurrentSessionID *string    `gorm:"type:uuid" json:"current_session_id"`
	RemainingMinutes int        `gorm:"column:remaining_minutes" json:"remaining_minutes"`
	ExpiresAt        *time.Time `gorm:"type:timestamptz" json:"expires_at"`
	CreatedAt        time.Time  `gorm:"default:now()" json:"created_at,omitempty"`
}
