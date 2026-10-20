package model

import "time"

// WindowsKey là key bản quyền Windows quán ghi cho MỘT máy. Mỗi máy tối đa một key; còn
// một key dùng chung nhiều máy (MAK, key khối lượng) thì ghi lặp ở từng máy — nên Key không
// duy nhất.
//
// VNET chỉ QUẢN LÝ key (ghi, tra, so với máy thật). Việc kích hoạt Windows chưa làm: Kzone xin
// Confirmation ID từ cloud của họ, VNET không có đường hợp pháp tương đương (roadmap X3).
type WindowsKey struct {
	ID        string `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MachineID string `gorm:"type:uuid;not null;uniqueIndex" json:"machine_id"`
	// Dạng chuẩn XXXXX-XXXXX-XXXXX-XXXXX-XXXXX, chữ hoa. json:"-": list chỉ trả bản che,
	// key đầy đủ chỉ đi qua đường "xem" có quyền riêng và có nhật ký.
	Key       string    `gorm:"type:varchar(29);not null" json:"-"`
	Edition   string    `gorm:"type:varchar(40)" json:"edition"`
	Note      string    `gorm:"type:varchar(255)" json:"note"`
	CreatedAt time.Time `gorm:"default:now()" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"default:now()" json:"updated_at,omitempty"`
}
