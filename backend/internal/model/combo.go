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

type TopupCard struct {
	ID         string         `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	// Code là SERI in trên thẻ: không bí mật, dùng để tra cứu và hỗ trợ khách.
	Code string `gorm:"type:varchar(50);not null;unique" json:"code"`
	// Pin là phần bí mật, lưu dưới dạng băm SHA-256 và KHÔNG BAO GIỜ trả ra
	// API. Mã thô chỉ xuất hiện đúng một lần trong phản hồi lúc sinh thẻ; lưu
	// thô nghĩa là ai đọc được database là tiêu được toàn bộ thẻ chưa dùng.
	Pin string `gorm:"type:varchar(64);not null" json:"-"`
	FaceValue  int64          `gorm:"not null" json:"face_value"`
	BonusValue int64          `gorm:"default:0" json:"bonus_value"`
	Status     string         `gorm:"type:varchar(20);default:active" json:"status"`
	SoldTo     *string        `gorm:"type:uuid" json:"sold_to"`
	SoldAt     *time.Time     `gorm:"type:timestamptz" json:"sold_at"`
	// Ai đã nạp thẻ và lúc nào — khác với SoldTo (bán ở quầy cho ai).
	UsedBy    *string    `gorm:"type:uuid" json:"used_by"`
	UsedAt    *time.Time `gorm:"type:timestamptz" json:"used_at"`
	ExpiresAt *time.Time `gorm:"type:timestamptz" json:"expires_at"`
	CreatedAt  time.Time      `gorm:"default:now()" json:"created_at,omitempty"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

type GiftCard struct {
	ID             string         `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	// Serial in trên thẻ, không bí mật — nhân viên tra cứu bằng nó.
	Serial string `gorm:"type:varchar(50);not null;uniqueIndex" json:"serial"`
	// Code là phần bí mật, lưu băm SHA-256, không bao giờ trả ra API. Xem
	// ghi chú ở TopupCard.Pin.
	Code string `gorm:"type:varchar(64);not null;unique" json:"-"`
	Balance        int64          `gorm:"default:0" json:"balance"`
	InitialBalance int64          `gorm:"not null" json:"initial_balance"`
	Status         string         `gorm:"type:varchar(20);default:active" json:"status"`
	ExpiresAt      *time.Time     `gorm:"type:timestamptz" json:"expires_at"`
	CreatedAt      time.Time      `gorm:"default:now()" json:"created_at,omitempty"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

type GiftCardTransaction struct {
	ID            string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	GiftCardID    string    `gorm:"type:uuid;not null;index" json:"gift_card_id"`
	Amount        int64     `gorm:"not null" json:"amount"`
	BalanceBefore int64     `gorm:"column:balance_before" json:"balance_before"`
	BalanceAfter  int64     `gorm:"column:balance_after" json:"balance_after"`
	OrderID       *string   `gorm:"type:uuid" json:"order_id"`
	CreatedAt     time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}
