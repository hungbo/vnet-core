package service

import (
	"errors"

	"github.com/vnet/core/internal/model"
	"gorm.io/gorm"
)

// ErrDuplicateRequest báo rằng khoá idempotency đã được dùng rồi.
var ErrDuplicateRequest = errors.New("yêu cầu này đã được xử lý")

// giuKhoaIdempotency chèn khoá TRONG CÙNG transaction với việc chuyển tiền.
//
// Không cần khoá row hay kiểm tra trước: chèn trùng khoá chính là vi phạm ràng
// buộc, transaction hỏng và toàn bộ phần chuyển tiền bị rút lại. Kiểm tra tồn
// tại trước rồi mới chèn lại đúng là cái lỗi TOCTOU mà hàm này sinh ra để tránh.
//
// Khoá rỗng nghĩa là phía gọi không tham gia — máy trạm Wails và các lần gọi API
// cũ không gửi khoá, và chúng vẫn phải chạy được như trước.
func giuKhoaIdempotency(tx *gorm.DB, scope, key string) error {
	if key == "" {
		return nil
	}
	if err := tx.Create(&model.IdempotencyKey{Key: key, Scope: scope}).Error; err != nil {
		return ErrDuplicateRequest
	}
	return nil
}
