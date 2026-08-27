package model

import "time"

// IdempotencyKey chặn một thao tác chuyển tiền bị thực hiện hai lần.
//
// Khoá row (xem khoaDonHang trong service) chỉ chặn được những thao tác có trạng
// thái để mà kiểm — duyệt một đơn đang chờ, huỷ một lịch đặt đang giữ. Nạp tiền
// tại quầy thì khác: nạp hai lần cùng số tiền cho cùng khách là chuyện hoàn toàn
// hợp lệ, máy chủ không có cách nào tự phân biệt với một cú bấm đúp hay một lần
// gửi lại của trình duyệt. Chỉ phía gọi mới biết, nên nó gửi kèm một khoá; khoá
// trùng nghĩa là cùng MỘT ý định, không phải hai.
//
// Bảng này cố tình không có ràng buộc khoá ngoại: nó phục vụ nhiều loại thao tác
// khác nhau và chỉ cần biết "khoá này đã dùng chưa".
type IdempotencyKey struct {
	Key       string    `gorm:"type:varchar(64);primaryKey" json:"key"`
	Scope     string    `gorm:"type:varchar(40);not null;index" json:"scope"`
	CreatedAt time.Time `gorm:"default:now();index" json:"created_at,omitempty"`
}
