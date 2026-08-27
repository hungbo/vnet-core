package model

import "time"

type MachineSession struct {
	ID               string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MachineID        string     `gorm:"type:uuid;not null;index" json:"machine_id"`
	MemberID         *string    `gorm:"type:uuid;index" json:"member_id"`
	ComboType        string     `gorm:"type:varchar(20)" json:"combo_type"`
	ComboID          *string    `gorm:"type:uuid" json:"combo_id"`
	SlotEnd          *time.Time `gorm:"type:timestamptz" json:"slot_end"`
	RemainingMinutes *int       `gorm:"column:remaining_minutes" json:"remaining_minutes"`
	StartedAt        time.Time  `gorm:"type:timestamptz;not null;default:now()" json:"started_at"`
	EndedAt          *time.Time `gorm:"type:timestamptz" json:"ended_at"`
	DurationMinutes  *int       `gorm:"column:duration_minutes" json:"duration_minutes"`
	TotalCost        *int64     `json:"total_cost"`
	IsOvernight      bool       `gorm:"default:false" json:"is_overnight"`
	IsActive         bool       `gorm:"default:true;index" json:"is_active"`

	// ChargedAmount là số tiền ĐÃ trừ cho phiên này.
	//
	// Đây là trạng thái duy nhất khiến việc trừ tiền theo phút idempotent: mỗi
	// lượt tính hỏi "tới giờ đáng lẽ đã thu bao nhiêu" rồi chỉ thu phần chênh.
	// Thiếu cột này thì mỗi lần khởi động lại máy chủ là trừ lại từ đầu.
	ChargedAmount int64 `gorm:"default:0" json:"charged_amount"`

	// AffordableUntil là thời điểm số dư cạn theo đơn giá đang áp dụng.
	//
	// Tính sẵn ở máy chủ để mọi màn hình — máy trạm, trang Phiên, Bảng điều
	// khiển — chỉ việc đếm ngược tới một mốc, không màn hình nào phải tự tra giá
	// và số dư. nil nghĩa là không giới hạn (máy chưa có giá).
	AffordableUntil *time.Time `gorm:"type:timestamptz" json:"affordable_until"`

	// Snapshot fields — frozen at session end for audit.
	// PricePerHour là ngoại lệ: nay được ghi NGAY khi mở máy để màn hình biết
	// đơn giá của phiên đang chạy.
	MachineGroupID   *string `gorm:"type:uuid" json:"machine_group_id,omitempty"`
	MemberGroupID    *string `gorm:"type:uuid" json:"member_group_id,omitempty"`
	MachineCode      string  `gorm:"type:varchar(20)" json:"machine_code,omitempty"`
	MachineGroupName string  `gorm:"type:varchar(50)" json:"machine_group_name,omitempty"`
	PricePerHour     int64   `gorm:"default:0" json:"price_per_hour"`
	BilledMinutes    int     `gorm:"default:0" json:"billed_minutes"`

	CreatedAt time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}
