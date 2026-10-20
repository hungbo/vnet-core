package model

import (
	"time"

	"gorm.io/gorm"
)

type MachineGroup struct {
	ID           string         `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name         string         `gorm:"type:varchar(50);not null;uniqueIndex" json:"name"`
	Description  string         `gorm:"type:text" json:"description"`
	Color        string         `gorm:"type:varchar(7)" json:"color"`
	PricePerHour int64          `gorm:"not null;default:0" json:"price_per_hour"`
	SortOrder    int            `gorm:"default:0" json:"sort_order"`
	CreatedAt    time.Time      `gorm:"default:now()" json:"created_at,omitempty"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

type Machine struct {
	ID            string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MachineCode   string     `gorm:"type:varchar(256);not null;uniqueIndex" json:"machine_code"`
	GroupID       *string    `gorm:"type:uuid;index" json:"group_id"`
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
	// Thời điểm máy khởi động lần gần nhất, suy ra từ uptime trong heartbeat.
	// Trên máy đóng băng (diskless), reboot làm giá trị này nhảy về hiện tại;
	// phiên nào bắt đầu TRƯỚC mốc này là phiên còn sót từ trước khi reboot.
	BootedAt *time.Time `gorm:"type:timestamptz" json:"booted_at,omitempty"`
	// Hai dấu hiệu máy trạm tự báo để trang Máy cảnh báo chủ quán.
	// UserIsAdmin: tài khoản Windows đang đăng nhập có quyền quản trị — khi đó
	// người ngồi máy gỡ được mọi lớp bảo vệ của máy trạm.
	// ExtraNetwork: card mạng xuất hiện SAU khi máy khởi động (Wi-Fi, USB phát
	// mạng), dạng "tên (IP)". Rỗng là không có.
	UserIsAdmin  bool   `gorm:"default:false" json:"user_is_admin"`
	ExtraNetwork string `gorm:"type:varchar(200)" json:"extra_network"`
	// Thiết bị ngoại vi máy trạm tự đọc lúc khởi động: màn hình, bàn phím,
	// chuột, âm thanh. Mỗi dòng dạng "Màn hình: Samsung S24F350 (serial)".
	Peripherals StringArray `gorm:"type:jsonb" json:"peripherals"`
	// Boot và bảo mật máy trạm tự đọc (client security.go): game chống gian lận như
	// Vanguard cần UEFI + Secure Boot + TPM 2.0. Rỗng = máy trạm chưa báo (bản cũ,
	// hoặc không đọc được); khi đó giữ nguyên giá trị đang có.
	BootMode       string `gorm:"type:varchar(10)" json:"boot_mode"`      // uefi | legacy
	SecureBoot     string `gorm:"type:varchar(12)" json:"secure_boot"`    // on | off | unsupported
	TPM            string `gorm:"column:tpm;type:varchar(10)" json:"tpm"` // 2.0 | 1.2 | none
	HVCI           string `gorm:"column:hvci;type:varchar(5)" json:"hvci"`
	Virtualization string `gorm:"type:varchar(5)" json:"virtualization"` // on | off
	BIOSVersion    string `gorm:"column:bios_version;type:varchar(64)" json:"bios_version"`
	// Chế độ màn hình đang dùng, "1920x1080 144Hz". Là trạng thái hiện tại; nhật ký
	// thay đổi phần cứng chỉ ghi khi tần số quét TỤT (game đổi độ phân giải rồi trả
	// lại mỗi lần chạy, ghi mọi thay đổi thì tràn).
	DisplayMode string `gorm:"type:varchar(32)" json:"display_mode"`
	// Số đo mới nhất của nhịp tim gần nhất, cho bảng Hoạt động: đọc từ đây thay vì lấy dòng mới
	// nhất trong machine_hardware_snapshots (bảng lớn nhất hệ thống) cho từng máy.
	CPUUsage  float64 `gorm:"type:decimal(5,2)" json:"cpu_usage"`
	RAMUsage  float64 `gorm:"type:decimal(5,2)" json:"ram_usage"`
	DiskUsage float64 `gorm:"type:decimal(5,2)" json:"disk_usage"`
	// Ứng dụng đang ở trên cùng (tên tệp .exe), băng thông byte/giây giữa hai nhịp tim, và độ
	// trễ tới máy chủ (mili giây). Rỗng/0 = máy trạm chưa báo (bản cũ hoặc chưa đo).
	ActiveApp  string `gorm:"type:varchar(64)" json:"active_app"`
	NetDownBps int64  `gorm:"default:0" json:"net_down_bps"`
	NetUpBps   int64  `gorm:"default:0" json:"net_up_bps"`
	PingMS     int    `gorm:"column:ping_ms;default:0" json:"ping_ms"`
	// Bản quyền Windows máy trạm đọc từ chính Windows: licensed | unlicensed | grace |
	// notification, và 5 ký tự cuối key đang cài (WindowsKey.Key so với nó).
	WinLicense string `gorm:"type:varchar(14)" json:"win_license"`
	WinKeyTail string `gorm:"type:varchar(5)" json:"win_key_tail"`
	// Lần đầu thấy cấu hình máy trạm tự đo. Trước mốc này các cột cpu/gpu/ram… có thể là
	// số người nhập tay, nên khác với số đo không phải là "thay đổi phần cứng".
	HWBaselineAt *time.Time `gorm:"column:hw_baseline_at;type:timestamptz" json:"hw_baseline_at,omitempty"`
	// Mật khẩu TightVNC trên máy trạm, máy trạm nhận qua phản hồi nhịp tim.
	// json:"-": chỉ trả riêng qua GET /machines/:id/remote-desktop.
	VNCPassword string `gorm:"type:varchar(8)" json:"-"`
	// Ghi chú nội bộ của nhân viên ("hỏng phím, chờ linh kiện"), sửa thẳng trên
	// bảng ở trang Máy và Bảng điều khiển. GetByCode không trả cột này cho máy trạm.
	Note      string         `gorm:"type:varchar(255)" json:"note"`
	IsActive  bool           `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time      `gorm:"default:now()" json:"created_at,omitempty"`
	UpdatedAt time.Time      `gorm:"default:now()" json:"updated_at,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

type MachinePrice struct {
	ID             string  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MachineGroupID *string `gorm:"type:uuid;index" json:"machine_group_id"`
	MemberGroupID  *string `gorm:"type:uuid;index" json:"member_group_id"`
	PricePerHour   int64   `gorm:"not null" json:"price_per_hour"`
	MinDuration    int     `gorm:"default:1" json:"min_duration"`
	EffectiveFrom  string  `gorm:"type:date;not null" json:"effective_from"`
	// Con trỏ chứ không phải string: cột kiểu date, và PostgreSQL từ chối
	// chuỗi rỗng làm date. Để trống nghĩa là không có ngày kết thúc — cùng
	// lớp lỗi đã gặp ở AuditLog.EntityID và Notification.ReferenceID.
	EffectiveTo *string   `gorm:"type:date" json:"effective_to"`
	CreatedAt   time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}

type TimeBasedPricing struct {
	ID             string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MachineGroupID *string   `gorm:"type:uuid;index" json:"machine_group_id"`
	DayOfWeek      int       `gorm:"column:day_of_week" json:"day_of_week"`
	StartTime      string    `gorm:"type:time without time zone;not null" json:"start_time"`
	EndTime        string    `gorm:"type:time without time zone;not null" json:"end_time"`
	PricePerHour   int64     `gorm:"not null" json:"price_per_hour"`
	IsActive       bool      `gorm:"default:true" json:"is_active"`
	CreatedAt      time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}

type MachineAsset struct {
	ID          string         `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MachineID   string         `gorm:"type:uuid;not null;index" json:"machine_id"`
	AssetType   string         `gorm:"type:varchar(20);not null" json:"asset_type"`
	Brand       string         `gorm:"type:varchar(100)" json:"brand"`
	Model       string         `gorm:"type:varchar(100)" json:"model"`
	Serial      string         `gorm:"type:varchar(100)" json:"serial"`
	Status      string         `gorm:"type:varchar(20);default:good" json:"status"`
	Notes       string         `gorm:"type:text" json:"notes"`
	CheckedBy   *string        `gorm:"type:uuid" json:"checked_by"`
	CheckedAt   *time.Time     `gorm:"type:timestamptz" json:"checked_at"`
	CheckPhotos StringArray    `gorm:"type:jsonb" json:"check_photos"`
	CreatedAt   time.Time      `gorm:"default:now()" json:"created_at,omitempty"`
	UpdatedAt   time.Time      `gorm:"default:now()" json:"updated_at,omitempty"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

type MachineHardwareSnapshot struct {
	ID        string  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MachineID string  `gorm:"type:uuid;not null;index" json:"machine_id"`
	CPUTemp   float64 `gorm:"type:decimal(5,1)" json:"cpu_temp"`
	GPUTemp   float64 `gorm:"type:decimal(5,1)" json:"gpu_temp"`
	CPUUsage  float64 `gorm:"type:decimal(5,2)" json:"cpu_usage"`
	RAMUsage  float64 `gorm:"type:decimal(5,2)" json:"ram_usage"`
	DiskUsage float64 `gorm:"type:decimal(5,2)" json:"disk_usage"`
	// Số giây máy đã bật. Máy trạm vẫn gửi lên từ trước nhưng không có cột nào
	// nhận, nên con số rơi vào hư không ở mỗi lần báo.
	Uptime int64 `json:"uptime"`
	// Có chỉ mục vì hai đường đọc đều đi qua cột này: bảng nhật ký sắp theo
	// thời gian giảm dần, và tác vụ dọn dẹp xoá theo mốc thời gian. Không có
	// chỉ mục thì cả hai quét toàn bảng — mà đây là bảng lớn nhất hệ thống.
	CreatedAt time.Time `gorm:"default:now();index" json:"created_at,omitempty"`
}

// MachineHardwareChange là một dòng nhật ký thay đổi phần cứng của máy: linh kiện (CPU,
// GPU, RAM, ổ đĩa), thiết bị ngoại vi cắm/rút, và các thiết lập Boot/bảo mật đổi
// (Secure Boot, TPM, UEFI…). Ghi khi nhịp tim mang số đo khác số đã lưu — trừ lần đo
// đầu tiên của một máy (xem Machine.HWBaselineAt).
type MachineHardwareChange struct {
	ID        string `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MachineID string `gorm:"type:uuid;not null;index" json:"machine_id"`
	// cpu | gpu | ram | storage | monitor | keyboard | mouse | sound | display (tần số
	// quét tụt) | boot_mode | secure_boot | tpm | hvci | virtualization | bios
	Component string `gorm:"type:varchar(20);not null" json:"component"`
	// changed | added | removed
	Action     string    `gorm:"type:varchar(10);not null" json:"action"`
	OldValue   string    `gorm:"type:varchar(200)" json:"old_value"`
	NewValue   string    `gorm:"type:varchar(200)" json:"new_value"`
	DetectedAt time.Time `gorm:"default:now();index" json:"detected_at"`
	// Nhân viên đã xem và xác nhận. Nil = chưa xác nhận, hiện ở đầu danh sách.
	AcknowledgedAt *time.Time `gorm:"type:timestamptz;index" json:"acknowledged_at,omitempty"`
	AcknowledgedBy string     `gorm:"type:varchar(50)" json:"acknowledged_by,omitempty"`

	// Chỉ điền khi liệt kê (join sang machines), không phải cột.
	MachineCode string `gorm:"->;-:migration" json:"machine_code,omitempty"`
}
