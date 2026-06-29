package model

import "time"

type MemberGroup struct {
	ID              string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name            string    `gorm:"type:varchar(50);not null" json:"name"`
	MinSpent        int64     `gorm:"default:0" json:"min_spent"`
	DiscountPercent float64   `gorm:"type:decimal(5,2);default:0" json:"discount_percent"`
	IsDefault       bool      `gorm:"default:false" json:"is_default"`
	CreatedAt       time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}

type Member struct {
	ID                   string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Username             string     `gorm:"type:varchar(50);uniqueIndex;not null" json:"username"`
	FullName             string     `gorm:"type:varchar(100)" json:"full_name"`
	Phone                string     `gorm:"type:varchar(20);index" json:"phone"`
	Email                string     `gorm:"type:varchar(100)" json:"email"`
	PasswordHash         string     `gorm:"type:varchar(255)" json:"-"`
	Role                 string     `gorm:"type:varchar(20);default:member" json:"role"`
	IDCardNumber         string     `gorm:"type:varchar(20)" json:"id_card_number"`
	IDCardImageURL       string     `gorm:"type:text" json:"id_card_image_url"`
	AvatarURL            string     `gorm:"type:text" json:"avatar_url"`
	DateOfBirth          *time.Time `gorm:"type:date" json:"date_of_birth"`
	Balance              int64      `gorm:"default:0" json:"balance"`
	BonusBalance         int64      `gorm:"default:0" json:"bonus_balance"`
	TotalSpent           int64      `gorm:"default:0" json:"total_spent"`
	TotalPlayedHours     int        `gorm:"default:0" json:"total_played_hours"`
	GroupID              *string    `gorm:"type:uuid;index" json:"group_id"`
	StoreID              *string    `gorm:"type:uuid;index" json:"store_id"`
	Notes                string     `gorm:"type:text" json:"notes"`
	ParentConsentFileURL string     `gorm:"type:text" json:"parent_consent_file_url"`
	IsActive             bool       `gorm:"default:true" json:"is_active"`
	LastVisitAt          *time.Time `gorm:"type:timestamptz" json:"last_visit_at"`
	CreatedAt            time.Time  `gorm:"default:now()" json:"created_at,omitempty"`
	UpdatedAt            time.Time  `gorm:"default:now()" json:"updated_at,omitempty"`
	DeletedAt            *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

type MemberTransaction struct {
	ID              string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MemberID        string    `gorm:"type:uuid;not null;index" json:"member_id"`
	TransactionType string    `gorm:"type:varchar(30);not null" json:"transaction_type"`
	Amount          int64     `gorm:"not null" json:"amount"`
	BalanceBefore   int64     `gorm:"not null" json:"balance_before"`
	BalanceAfter    int64     `gorm:"not null" json:"balance_after"`
	BonusBefore     int64     `gorm:"default:0" json:"bonus_before"`
	BonusAfter      int64     `gorm:"default:0" json:"bonus_after"`
	PaymentMethod   string    `gorm:"type:varchar(30)" json:"payment_method"`
	ReferenceID     *string   `gorm:"type:uuid" json:"reference_id"`
	Description     string    `gorm:"type:text" json:"description"`
	StoreID         *string   `gorm:"type:uuid;index" json:"store_id"`
	CreatedBy       *string   `gorm:"type:uuid" json:"created_by"`
	CreatedAt       time.Time `gorm:"default:now();index" json:"created_at,omitempty"`
}

type MemberAttendance struct {
	ID            string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MemberID      string    `gorm:"type:uuid;not null;index" json:"member_id"`
	CheckinAt     time.Time `gorm:"default:now()" json:"checkin_at"`
	RewardClaimed bool      `gorm:"default:false" json:"reward_claimed"`
	StoreID       *string   `gorm:"type:uuid" json:"store_id"`
}
