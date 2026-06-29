package model

import "time"

type Promotion struct {
	ID          string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name        string     `gorm:"type:varchar(200);not null" json:"name"`
	Description string     `gorm:"type:text" json:"description"`
	Type        string     `gorm:"type:varchar(20);not null" json:"type"`
	Priority    int        `gorm:"default:0" json:"priority"`
	IsActive    bool       `gorm:"default:true" json:"is_active"`
	ValidFrom   *time.Time `gorm:"type:timestamptz" json:"valid_from"`
	ValidTo     *time.Time `gorm:"type:timestamptz" json:"valid_to"`
	CreatedAt   time.Time  `gorm:"default:now()" json:"created_at,omitempty"`
	DeletedAt   *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

type PromotionCondition struct {
	ID             string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	PromotionID    string    `gorm:"type:uuid;not null;index" json:"promotion_id"`
	ConditionKey   string    `gorm:"type:varchar(50);not null" json:"condition_key"`
	ConditionValue string    `gorm:"type:jsonb;not null" json:"condition_value"`
	CreatedAt      time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}

type PromotionReward struct {
	ID          string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	PromotionID string    `gorm:"type:uuid;not null;index" json:"promotion_id"`
	RewardType  string    `gorm:"type:varchar(20);not null" json:"reward_type"`
	RewardValue string    `gorm:"type:jsonb;not null" json:"reward_value"`
	CreatedAt   time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}

type LuckySpinReward struct {
	ID          string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name        string    `gorm:"type:varchar(100);not null" json:"name"`
	RewardType  string    `gorm:"type:varchar(20);not null" json:"reward_type"`
	RewardValue string    `gorm:"type:jsonb;not null" json:"reward_value"`
	Probability float64   `gorm:"type:decimal(5,4);not null" json:"probability"`
	MaxPerDay   int       `gorm:"default:0" json:"max_per_day"`
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}

type LuckySpinLog struct {
	ID       string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MemberID string    `gorm:"type:uuid;not null;index" json:"member_id"`
	RewardID *string   `gorm:"type:uuid" json:"reward_id"`
	IsWin    bool      `gorm:"default:false" json:"is_win"`
	SpunAt   time.Time `gorm:"default:now()" json:"spun_at"`
}
