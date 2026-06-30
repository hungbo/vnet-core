package model

import "time"

type MachineGroup struct {
	ID          string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name        string     `gorm:"type:varchar(50);not null;uniqueIndex" json:"name"`
	Description string     `gorm:"type:text" json:"description"`
	Color       string     `gorm:"type:varchar(7)" json:"color"`
	PricePerHour int64     `gorm:"not null;default:0" json:"price_per_hour"`
	SortOrder   int        `gorm:"default:0" json:"sort_order"`
	CreatedAt   time.Time  `gorm:"default:now()" json:"created_at,omitempty"`
	DeletedAt   *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

type Machine struct {
	ID            string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MachineCode   string     `gorm:"type:varchar(20);not null;uniqueIndex" json:"machine_code"`
	GroupID       *string    `gorm:"type:uuid;index;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"group_id"`
	Status        string     `gorm:"type:varchar(20);default:offline;index" json:"status"`
	CPUName       string     `gorm:"type:varchar(100)" json:"cpu_name"`
	RAMGB         int        `gorm:"column:ram_gb" json:"ram_gb"`
	GPUName       string     `gorm:"type:varchar(100)" json:"gpu_name"`
	StorageGB     int        `gorm:"column:storage_gb" json:"storage_gb"`
	IPAddress     string     `gorm:"type:varchar(45)" json:"ip_address"`
	MacAddress    string     `gorm:"type:varchar(17)" json:"mac_address"`
	OSInfo        string     `gorm:"type:varchar(100)" json:"os_info"`
	CPUTemp       float64    `gorm:"type:decimal(5,1)" json:"cpu_temp"`
	GPUTemp       float64    `gorm:"type:decimal(5,1)" json:"gpu_temp"`
	LastHeartbeat *time.Time `gorm:"type:timestamptz" json:"last_heartbeat"`
	IsActive      bool       `gorm:"default:true" json:"is_active"`
	CreatedAt     time.Time  `gorm:"default:now()" json:"created_at,omitempty"`
	UpdatedAt     time.Time  `gorm:"default:now()" json:"updated_at,omitempty"`
	DeletedAt     *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

type MachinePrice struct {
	ID             string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MachineGroupID *string   `gorm:"type:uuid;index;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"machine_group_id"`
	MemberGroupID  *string   `gorm:"type:uuid;index;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"member_group_id"`
	PricePerHour   int64     `gorm:"not null" json:"price_per_hour"`
	MinDuration    int       `gorm:"default:1" json:"min_duration"`
	EffectiveFrom  string    `gorm:"type:date;not null" json:"effective_from"`
	EffectiveTo    string    `gorm:"type:date" json:"effective_to"`
	CreatedAt      time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}

type TimeBasedPricing struct {
	ID             string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MachineGroupID *string   `gorm:"type:uuid;index;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"machine_group_id"`
	DayOfWeek      int       `gorm:"column:day_of_week" json:"day_of_week"`
	StartTime      string    `gorm:"type:time;not null" json:"start_time"`
	EndTime        string    `gorm:"type:time;not null" json:"end_time"`
	PricePerHour   int64     `gorm:"not null" json:"price_per_hour"`
	IsActive       bool      `gorm:"default:true" json:"is_active"`
	CreatedAt      time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}

type MachineAsset struct {
	ID          string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MachineID   string     `gorm:"type:uuid;not null;index;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"machine_id"`
	AssetType   string     `gorm:"type:varchar(20);not null" json:"asset_type"`
	Brand       string     `gorm:"type:varchar(100)" json:"brand"`
	Model       string     `gorm:"type:varchar(100)" json:"model"`
	Serial      string     `gorm:"type:varchar(100)" json:"serial"`
	Status      string     `gorm:"type:varchar(20);default:good" json:"status"`
	Notes       string     `gorm:"type:text" json:"notes"`
	CheckedBy   *string    `gorm:"type:uuid" json:"checked_by"`
	CheckedAt   *time.Time `gorm:"type:timestamptz" json:"checked_at"`
	CheckPhotos []string   `gorm:"type:jsonb" json:"check_photos"`
	CreatedAt   time.Time  `gorm:"default:now()" json:"created_at,omitempty"`
	UpdatedAt   time.Time  `gorm:"default:now()" json:"updated_at,omitempty"`
	DeletedAt   *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

type MachineHardwareSnapshot struct {
	ID        string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MachineID string    `gorm:"type:uuid;not null;index" json:"machine_id"`
	CPUTemp   float64   `gorm:"type:decimal(5,1)" json:"cpu_temp"`
	GPUTemp   float64   `gorm:"type:decimal(5,1)" json:"gpu_temp"`
	CPUUsage  float64   `gorm:"type:decimal(5,2)" json:"cpu_usage"`
	RAMUsage  float64   `gorm:"type:decimal(5,2)" json:"ram_usage"`
	DiskUsage float64   `gorm:"type:decimal(5,2)" json:"disk_usage"`
	CreatedAt time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}
