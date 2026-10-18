package model

import "time"

// ErrorEvent là MỘT LOẠI lỗi, không phải một lần xảy ra: các lần cùng dấu vân
// tay (nguồn + vị trí + thông báo đã bỏ số/uuid) dồn vào một dòng và tăng
// Count. Nhờ vậy một lỗi lặp mỗi phút trên 50 máy vẫn chỉ là một dòng để đọc,
// và bảng không phình theo số lần lỗi.
//
// Các cột mô tả (Message, Detail, MachineCode, …) giữ MẪU GẦN NHẤT.
type ErrorEvent struct {
	ID          string `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Fingerprint string `gorm:"type:varchar(64);not null;uniqueIndex" json:"fingerprint"`
	// server | admin | client
	Source string `gorm:"type:varchar(20);not null;index" json:"source"`
	// error | panic | crash
	Kind     string `gorm:"type:varchar(20);not null" json:"kind"`
	Location string `gorm:"type:varchar(300)" json:"location"`
	Message  string `gorm:"type:text;not null" json:"message"`
	// Stack trace / thông tin phụ của lần gần nhất.
	Detail string `gorm:"type:text" json:"detail"`
	// Ai gặp: mã máy với máy trạm, tên đăng nhập với admin và server.
	Reporter    string    `gorm:"type:varchar(100)" json:"reporter"`
	RequestID   string    `gorm:"type:varchar(32)" json:"request_id"`
	Version     string    `gorm:"type:varchar(50)" json:"version"`
	Count       int64     `gorm:"not null;default:1" json:"count"`
	FirstSeenAt time.Time `gorm:"not null;default:now()" json:"first_seen_at"`
	LastSeenAt  time.Time `gorm:"not null;default:now();index" json:"last_seen_at"`
	// Dev đánh dấu đã sửa. Lỗi tái phát thì tự mở lại (về NULL).
	ResolvedAt *time.Time `json:"resolved_at"`
}
