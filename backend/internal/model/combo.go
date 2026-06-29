package model

import "time"

type Combo struct {
	ID            string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name          string     `gorm:"type:varchar(100);not null;uniqueIndex" json:"name"`
	Description   string     `gorm:"type:text" json:"description"`
	Type          string     `gorm:"type:varchar(20);not null" json:"type"`
	SlotStart     string     `gorm:"type:time" json:"slot_start"`
	SlotEnd       string     `gorm:"type:time" json:"slot_end"`
	ApplyDays     []int      `gorm:"type:integer[]" json:"apply_days"`
	TotalMinutes  int        `gorm:"column:total_minutes" json:"total_minutes"`
	ValidityDays  int        `gorm:"column:validity_days" json:"validity_days"`
	Price         int64      `gorm:"not null" json:"price"`
	MemberPrefix  string     `gorm:"type:varchar(20)" json:"member_prefix"`
	MemberCount   int        `gorm:"default:0" json:"member_count"`
	IsActive      bool       `gorm:"default:true" json:"is_active"`
	CreatedAt     time.Time  `gorm:"default:now()" json:"created_at,omitempty"`
	DeletedAt     *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

type ComboItem struct {
	ID        string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	ComboID   string    `gorm:"type:uuid;not null;index;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"combo_id"`
	ItemType  string    `gorm:"type:varchar(20);not null" json:"item_type"`
	ItemID    *string   `gorm:"type:uuid" json:"item_id"`
	Quantity  int       `gorm:"default:1" json:"quantity"`
	CreatedAt time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}

type ComboPurchase struct {
	ID                string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	ComboID           string     `gorm:"type:uuid;not null;index" json:"combo_id"`
	MemberID          string     `gorm:"type:uuid;not null;index;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"member_id"`
	Price             int64      `gorm:"not null" json:"price"`
	PaymentMethod     string     `gorm:"type:varchar(30)" json:"payment_method"`
	Activated         bool       `gorm:"default:false" json:"activated"`
	ActivatedAt       *time.Time `gorm:"type:timestamptz" json:"activated_at"`
	CurrentSessionID  *string    `gorm:"type:uuid;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"current_session_id"`
	RemainingMinutes  int        `gorm:"column:remaining_minutes" json:"remaining_minutes"`
	ExpiresAt         *time.Time `gorm:"type:timestamptz" json:"expires_at"`
	CreatedAt         time.Time  `gorm:"default:now()" json:"created_at,omitempty"`
}

type TopupCard struct {
	ID         string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Code       string     `gorm:"type:varchar(50);not null;unique" json:"code"`
	Pin        string     `gorm:"type:varchar(20)" json:"pin"`
	FaceValue  int64      `gorm:"not null" json:"face_value"`
	BonusValue int64      `gorm:"default:0" json:"bonus_value"`
	Status     string     `gorm:"type:varchar(20);default:active" json:"status"`
	SoldTo     *string    `gorm:"type:uuid;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"sold_to"`
	SoldAt     *time.Time `gorm:"type:timestamptz" json:"sold_at"`
	CreatedAt  time.Time  `gorm:"default:now()" json:"created_at,omitempty"`
	DeletedAt  *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

type GiftCard struct {
	ID             string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Code           string     `gorm:"type:varchar(50);not null;unique" json:"code"`
	Balance        int64      `gorm:"default:0" json:"balance"`
	InitialBalance int64      `gorm:"not null" json:"initial_balance"`
	Status         string     `gorm:"type:varchar(20);default:active" json:"status"`
	ExpiresAt      *time.Time `gorm:"type:timestamptz" json:"expires_at"`
	CreatedAt      time.Time  `gorm:"default:now()" json:"created_at,omitempty"`
	DeletedAt      *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

type GiftCardTransaction struct {
	ID            string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	GiftCardID    string    `gorm:"type:uuid;not null;index;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"gift_card_id"`
	Amount        int64     `gorm:"not null" json:"amount"`
	BalanceBefore int64     `gorm:"column:balance_before" json:"balance_before"`
	BalanceAfter  int64     `gorm:"column:balance_after" json:"balance_after"`
	OrderID       *string   `gorm:"type:uuid;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"order_id"`
	CreatedAt     time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}
