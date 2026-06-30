package model

import "time"

type Shift struct {
	ID             string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID         string     `gorm:"type:uuid;not null;index;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user_id"`
	StartedAt      time.Time  `gorm:"type:timestamptz;not null" json:"started_at"`
	EndedAt        *time.Time `gorm:"type:timestamptz" json:"ended_at"`
	Status         string     `gorm:"type:varchar(20);default:open" json:"status"`
	OpeningBalance int64      `gorm:"default:0" json:"opening_balance"`
	ClosingBalance *int64     `gorm:"column:closing_balance" json:"closing_balance"`
	ExpectedTotal  *int64     `gorm:"column:expected_total" json:"expected_total"`
	Notes          string     `gorm:"type:text" json:"notes"`
	CreatedAt      time.Time  `gorm:"default:now()" json:"created_at,omitempty"`
}

type CashHandover struct {
	ID           string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	ShiftID      string    `gorm:"type:uuid;not null;index;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"shift_id"`
	Amount       int64     `gorm:"not null" json:"amount"`
	HandoverType string    `gorm:"type:varchar(20);not null" json:"handover_type"`
	Reason       string    `gorm:"type:text" json:"reason"`
	CreatedBy    *string   `gorm:"type:uuid;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"created_by"`
	CreatedAt    time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}
