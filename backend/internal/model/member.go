package model

import (
	"time"

	"gorm.io/gorm"
)

type MemberGroup struct {
	ID              string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name            string    `gorm:"type:varchar(50);not null" json:"name"`
	MinSpent        int64     `gorm:"default:0" json:"min_spent"`
	DiscountPercent float64   `gorm:"type:decimal(5,2);default:0" json:"discount_percent"`
	IsDefault       bool      `gorm:"default:false" json:"is_default"`
	CreatedAt       time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}

type Member struct {
	ID             string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Username       string     `gorm:"type:varchar(50);uniqueIndex;not null" json:"username"`
	FullName       string     `gorm:"type:varchar(100)" json:"full_name"`
	Phone          string     `gorm:"type:varchar(20);index" json:"phone"`
	Email          string     `gorm:"type:varchar(100)" json:"email"`
	PasswordHash   string     `gorm:"type:varchar(255)" json:"-"`
	Role           string     `gorm:"type:varchar(20);default:member" json:"role"`
	IDCardNumber   string     `gorm:"type:varchar(20)" json:"id_card_number"`
	IDCardImageURL string     `gorm:"type:text" json:"id_card_image_url"`
	AvatarURL      string     `gorm:"type:text" json:"avatar_url"`
	DateOfBirth    *time.Time `gorm:"type:date" json:"date_of_birth"`
	Balance        int64      `gorm:"default:0" json:"balance"`
	BonusBalance   int64      `gorm:"default:0" json:"bonus_balance"`
	TotalSpent     int64      `gorm:"default:0" json:"total_spent"`
	// Đơn vị là PHÚT, không phải giờ. Cột cũ tên total_played_hours chưa bao giờ
	// được ghi; cộng phút vào một cột mang tên "hours" là đúng cái bẫy đã làm
	// hỏng phần thưởng free_minutes trước đây — phiên 40 phút sẽ cộng ra 0.
	TotalPlayedMinutes   int            `gorm:"default:0" json:"total_played_minutes"`
	GroupID              *string        `gorm:"type:uuid;index;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"group_id"`
	Notes                string         `gorm:"type:text" json:"notes"`
	ParentConsentFileURL string         `gorm:"type:text" json:"parent_consent_file_url"`
	IsActive             bool           `gorm:"default:true" json:"is_active"`
	LastVisitAt          *time.Time     `gorm:"type:timestamptz" json:"last_visit_at"`
	CreatedAt            time.Time      `gorm:"default:now()" json:"created_at,omitempty"`
	UpdatedAt            time.Time      `gorm:"default:now()" json:"updated_at,omitempty"`
	DeletedAt            gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

type MemberTransaction struct {
	ID              string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MemberID        string    `gorm:"type:uuid;not null;index;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"member_id"`
	TransactionType string    `gorm:"type:varchar(30);not null" json:"transaction_type"`
	Amount          int64     `gorm:"not null" json:"amount"`
	BalanceBefore   int64     `gorm:"not null" json:"balance_before"`
	BalanceAfter    int64     `gorm:"not null" json:"balance_after"`
	BonusBefore     int64     `gorm:"default:0" json:"bonus_before"`
	BonusAfter      int64     `gorm:"default:0" json:"bonus_after"`
	PaymentMethod   string    `gorm:"type:varchar(30)" json:"payment_method"`
	ReferenceID     *string   `gorm:"type:uuid" json:"reference_id"`
	Description     string    `gorm:"type:text" json:"description"`
	CreatedBy       *string   `gorm:"type:uuid;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"created_by"`
	CreatedAt       time.Time `gorm:"default:now();index" json:"created_at,omitempty"`
}

// MemberAttendance là một lượt điểm danh trong NGÀY của hội viên.
type MemberAttendance struct {
	ID       string `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MemberID string `gorm:"type:uuid;not null;index" json:"member_id"`
	// CheckinDate là ngày điểm danh, tính theo múi giờ máy chủ và lưu thành
	// cột date riêng. Không dựa vào checkin_at::date: ép kiểu đó phụ thuộc múi
	// giờ của phiên kết nối database, nên "một lần mỗi ngày" sẽ lệch khi máy
	// chủ và database đặt múi giờ khác nhau.
	//
	// Cặp (member_id, checkin_date) có ràng buộc DUY NHẤT ở tầng database —
	// xem cmd/migrate. Đó là thứ duy nhất chặn được hai lần điểm danh gửi
	// đồng thời.
	CheckinDate   time.Time `gorm:"type:date;not null;index" json:"checkin_date"`
	CheckinAt     time.Time `gorm:"default:now()" json:"checkin_at"`
	StreakDays    int       `gorm:"default:1" json:"streak_days"`
	RewardAmount  int64     `gorm:"default:0" json:"reward_amount"`
	RewardClaimed bool      `gorm:"default:false" json:"reward_claimed"`
}
