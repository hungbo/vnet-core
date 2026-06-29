package model

import "time"

type MachineSession struct {
	ID               string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MachineID        string     `gorm:"type:uuid;not null;index" json:"machine_id"`
	MemberID         *string    `gorm:"type:uuid;index" json:"member_id"`
	ComboType        string     `gorm:"type:varchar(20)" json:"combo_type"`
	ComboID          *string    `gorm:"type:uuid" json:"combo_id"`
	SlotEnd          *time.Time `gorm:"type:timestamptz" json:"slot_end"`
	RemainingMinutes *int       `gorm:"column:remaining_minutes" json:"remaining_minutes"`
	StartedAt        time.Time  `gorm:"type:timestamptz;not null;default:now()" json:"started_at"`
	EndedAt          *time.Time `gorm:"type:timestamptz" json:"ended_at"`
	DurationMinutes  *int       `gorm:"column:duration_minutes" json:"duration_minutes"`
	TotalCost        *int64     `json:"total_cost"`
	IsOvernight      bool       `gorm:"default:false" json:"is_overnight"`
	StoreID          *string    `gorm:"type:uuid;index" json:"store_id"`
	IsActive         bool       `gorm:"default:true;index" json:"is_active"`
	CreatedAt        time.Time  `gorm:"default:now()" json:"created_at,omitempty"`
}
