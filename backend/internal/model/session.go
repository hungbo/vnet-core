package model

import "time"

type MachineSession struct {
	ID               string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MachineID        string     `gorm:"type:uuid;not null;index;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"machine_id"`
	MemberID         *string    `gorm:"type:uuid;index;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"member_id"`
	ComboType        string     `gorm:"type:varchar(20)" json:"combo_type"`
	ComboID          *string    `gorm:"type:uuid;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"combo_id"`
	SlotEnd          *time.Time `gorm:"type:timestamptz" json:"slot_end"`
	RemainingMinutes *int       `gorm:"column:remaining_minutes" json:"remaining_minutes"`
	StartedAt        time.Time  `gorm:"type:timestamptz;not null;default:now()" json:"started_at"`
	EndedAt          *time.Time `gorm:"type:timestamptz" json:"ended_at"`
	DurationMinutes  *int       `gorm:"column:duration_minutes" json:"duration_minutes"`
	TotalCost        *int64     `json:"total_cost"`
	IsOvernight      bool       `gorm:"default:false" json:"is_overnight"`
	IsActive         bool       `gorm:"default:true;index" json:"is_active"`

	// Snapshot fields — frozen at session end for audit
	MachineGroupID   *string `gorm:"type:uuid" json:"machine_group_id,omitempty"`
	MemberGroupID    *string `gorm:"type:uuid" json:"member_group_id,omitempty"`
	MachineCode      string  `gorm:"type:varchar(20)" json:"machine_code,omitempty"`
	MachineGroupName string  `gorm:"type:varchar(50)" json:"machine_group_name,omitempty"`
	PricePerHour     int64   `gorm:"default:0" json:"price_per_hour"`
	BilledMinutes    int     `gorm:"default:0" json:"billed_minutes"`

	CreatedAt        time.Time  `gorm:"default:now()" json:"created_at,omitempty"`
}
