package model

import "time"

type SystemSetting struct {
	ID          string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	GroupName   string    `gorm:"type:varchar(50);not null;uniqueIndex:idx_settings_group_key" json:"group_name"`
	Key         string    `gorm:"type:varchar(100);not null;uniqueIndex:idx_settings_group_key" json:"key"`
	Value       string    `gorm:"type:jsonb;not null" json:"value"`
	Description string    `gorm:"type:text" json:"description"`
	CreatedAt   time.Time `gorm:"default:now()" json:"created_at,omitempty"`
	UpdatedAt   time.Time `gorm:"default:now()" json:"updated_at,omitempty"`
}

type AuditLog struct {
	ID          string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Action      string    `gorm:"type:varchar(100);not null;index" json:"action"`
	EntityType  string    `gorm:"type:varchar(50);index:idx_audit_logs_entity" json:"entity_type"`
	EntityID    string    `gorm:"type:uuid;index:idx_audit_logs_entity" json:"entity_id"`
	UserID      *string   `gorm:"type:uuid;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"user_id"`
	Description string    `gorm:"type:text" json:"description"`
	Metadata    string    `gorm:"type:jsonb" json:"metadata"`
	IPAddress   string    `gorm:"type:varchar(45)" json:"ip_address"`
	CreatedAt   time.Time `gorm:"default:now();index" json:"created_at,omitempty"`
}

type Notification struct {
	ID          string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Type        string    `gorm:"type:varchar(30);not null" json:"type"`
	Title       string    `gorm:"type:varchar(200);not null" json:"title"`
	Content     string    `gorm:"type:text" json:"content"`
	ReferenceID string    `gorm:"type:uuid" json:"reference_id"`
	CreatedAt   time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}

type NotificationRecipient struct {
	ID             string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	NotificationID string     `gorm:"type:uuid;not null;index;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"notification_id"`
	RecipientID    string     `gorm:"type:uuid;not null" json:"recipient_id"`
	IsRead         bool       `gorm:"default:false" json:"is_read"`
	ReadAt         *time.Time `gorm:"type:timestamptz" json:"read_at"`
	CreatedAt      time.Time  `gorm:"default:now()" json:"created_at,omitempty"`
}

type BackupLog struct {
	ID          string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	FileName    string     `gorm:"type:varchar(255);not null" json:"file_name"`
	FileSize    int64      `gorm:"column:file_size" json:"file_size"`
	FilePath    string     `gorm:"type:text" json:"file_path"`
	Status      string     `gorm:"type:varchar(20);default:running" json:"status"`
	Notes       string     `gorm:"type:text" json:"notes"`
	CreatedBy   *string    `gorm:"type:uuid;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"created_by"`
	StartedAt   time.Time  `gorm:"default:now()" json:"started_at"`
	CompletedAt *time.Time `gorm:"type:timestamptz" json:"completed_at"`
}

type EInvoiceConfig struct {
	ID        string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Provider  string    `gorm:"type:varchar(30);not null" json:"provider"`
	APIKey    string    `gorm:"type:text" json:"api_key"`
	APISecret string    `gorm:"type:text" json:"api_secret"`
	Endpoint  string    `gorm:"type:text" json:"endpoint"`
	IsActive  bool      `gorm:"default:false" json:"is_active"`
	StoreID   *string   `gorm:"type:uuid;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"store_id"`
	CreatedAt time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}

type EInvoice struct {
	ID            string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	OrderID       string    `gorm:"type:uuid;not null;index;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"order_id"`
	InvoiceCode   string    `gorm:"type:varchar(50)" json:"invoice_code"`
	InvoiceNumber string    `gorm:"type:varchar(50)" json:"invoice_number"`
	Provider      string    `gorm:"type:varchar(30);not null" json:"provider"`
	RawRequest    string    `gorm:"type:jsonb" json:"raw_request"`
	RawResponse   string    `gorm:"type:jsonb" json:"raw_response"`
	Status        string    `gorm:"type:varchar(20);default:pending" json:"status"`
	CreatedAt     time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}

type WebsiteBlockingRule struct {
	ID          string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Pattern     string     `gorm:"type:varchar(500);not null" json:"pattern"`
	RuleType    string     `gorm:"type:varchar(10);not null" json:"rule_type"`
	Category    string     `gorm:"type:varchar(30)" json:"category"`
	Description string     `gorm:"type:text" json:"description"`
	IsActive    bool       `gorm:"default:true;index" json:"is_active"`
	CreatedAt   time.Time  `gorm:"default:now()" json:"created_at,omitempty"`
	DeletedAt   *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

type WebsiteRuleMapping struct {
	ID             string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	RuleID         string    `gorm:"type:uuid;not null;index;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"rule_id"`
	MachineGroupID *string   `gorm:"type:uuid;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"machine_group_id"`
	CreatedAt      time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}

type WebsiteBlockingSchedule struct {
	ID        string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	RuleID    string    `gorm:"type:uuid;not null;index" json:"rule_id"`
	DayOfWeek []int     `gorm:"type:integer[]" json:"day_of_week"`
	StartTime string    `gorm:"type:time" json:"start_time"`
	EndTime   string    `gorm:"type:time" json:"end_time"`
	IsActive  bool      `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}

type WebsiteBlockingViolation struct {
	ID          string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MachineID   string    `gorm:"type:uuid;not null;index:idx_website_violations_machine;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"machine_id"`
	RuleID      *string   `gorm:"type:uuid;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"rule_id"`
	Domain      string    `gorm:"type:varchar(500);not null" json:"domain"`
	URL         string    `gorm:"type:text" json:"url"`
	ProcessName string    `gorm:"type:varchar(200)" json:"process_name"`
	BlockedAt   time.Time `gorm:"default:now()" json:"blocked_at"`
}

type AppUpdate struct {
	ID         string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Version    string    `gorm:"type:varchar(20);not null" json:"version"`
	Platform   string    `gorm:"type:varchar(20);not null" json:"platform"`
	FileURL    string    `gorm:"type:text;not null" json:"file_url"`
	Changelog  string    `gorm:"type:text" json:"changelog"`
	IsRequired bool      `gorm:"default:false" json:"is_required"`
	CreatedAt  time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}
