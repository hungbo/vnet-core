package model

import "time"

type MachineBooking struct {
	ID                    string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MachineID             string     `gorm:"type:uuid;not null;index;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"machine_id"`
	MemberID              *string    `gorm:"type:uuid;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"member_id"`
	CustomerName          string     `gorm:"type:varchar(100)" json:"customer_name"`
	CustomerPhone         string     `gorm:"type:varchar(20)" json:"customer_phone"`
	BookedFrom            time.Time  `gorm:"type:timestamptz;not null" json:"booked_from"`
	BookedTo              time.Time  `gorm:"type:timestamptz;not null" json:"booked_to"`
	DepositAmount         int64      `gorm:"default:0" json:"deposit_amount"`
	DepositTransactionID  *string    `gorm:"type:uuid" json:"deposit_transaction_id"`
	Status                string     `gorm:"type:varchar(20);default:pending;index" json:"status"`
	CancelAt              *time.Time `gorm:"type:timestamptz" json:"cancel_at"`
	Notes                 string     `gorm:"type:text" json:"notes"`
	CreatedBy             *string    `gorm:"type:uuid;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"created_by"`
	CreatedAt             time.Time  `gorm:"default:now()" json:"created_at,omitempty"`
	UpdatedAt             time.Time  `gorm:"default:now()" json:"updated_at,omitempty"`
	DeletedAt             *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}
