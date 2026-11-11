package service

import (
	"errors"

	"github.com/vnet/core/internal/model"
	"gorm.io/gorm"
)

// QuyenDoiTrangThai trả mã quyền cần để chuyển đơn id sang trạng thái status qua
// POST /orders/:id/status (xem quyenDoiTrangThai).
func (s *OrderService) QuyenDoiTrangThai(id, status string) (string, error) {
	var order model.Order
	if err := s.db.Select("id", "order_type").Where("id = ?", id).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", errors.New("không tìm thấy đơn hàng")
		}
		return "", err
	}
	return quyenDoiTrangThai(order.OrderType, status), nil
}

// quyenDoiTrangThai: một API đổi trạng thái làm ba việc khác nhau.
//   - Huỷ đơn, kể cả từ chối đơn nạp: orders.cancel.
//   - "completed" của đơn món là THU TIỀN (UpdateStatus gọi settleOrder), nên đòi
//     orders.pay như POST /orders/:id/pay. Trước đây chỉ cần quyền đổi trạng thái
//     là thu được tiền, dù nút Thanh toán đã ẩn với người thiếu orders.pay.
//   - Còn lại là xác nhận: xác nhận đơn món, duyệt đơn nạp tiền mặt (trang Đơn
//     hàng gửi "completed", UpdateStatus đổi thành "confirmed"): orders.confirm.
//
// Trạng thái không hợp lệ cũng rơi vào orders.confirm; UpdateStatus từ chối nó sau.
func quyenDoiTrangThai(orderType, status string) string {
	switch {
	case status == "cancelled":
		return "orders.cancel"
	case status == "completed" && orderType != model.OrderTypeTopup:
		return "orders.pay"
	default:
		return "orders.confirm"
	}
}

// ChuyenQuyenTrangThaiDon đưa quyền cũ "Đổi trạng thái đơn" (orders.status) sang
// hai quyền mới Xác nhận đơn và Huỷ đơn, chạy mỗi lần máy chủ khởi động.
//
// Quán cập nhật bằng cách thay vnet-server.exe, không chạy cmd/migrate, nên mã
// mới phải tự có ở đây; và vai trò nào đang giữ quyền cũ nhận cả hai quyền mới,
// để nhân viên không mất nút Xác nhận/Huỷ ngay sau khi thay bản.
//
// Chỉ THÊM, không gỡ orders.status: quay về bản trước (bản đó gác API bằng mã cũ)
// vẫn chạy. Trang Phân quyền ẩn mã cũ (authz.RetiredCodes), nên lần đầu lưu quyền
// của một vai trò thì mã cũ rời vai trò đó, và lần khởi động sau không cấp lại
// quyền mà quản trị viên vừa bỏ tick.
func ChuyenQuyenTrangThaiDon(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for _, p := range []model.Permission{
			{Code: "orders.confirm", Name: "Xác nhận đơn (cả duyệt đơn nạp tiền mặt, trạng thái món)", Module: "orders"},
			{Code: "orders.cancel", Name: "Huỷ đơn (cả từ chối đơn nạp)", Module: "orders"},
		} {
			if err := tx.Exec(`INSERT INTO permissions (code, name, module) VALUES (?, ?, ?)
				ON CONFLICT (code) DO NOTHING`, p.Code, p.Name, p.Module).Error; err != nil {
				return err
			}
		}
		return tx.Exec(`INSERT INTO role_permissions (role_id, permission_id)
			SELECT rp.role_id, moi.id
			FROM role_permissions rp
			JOIN permissions cu ON cu.id = rp.permission_id AND cu.code = 'orders.status'
			JOIN permissions moi ON moi.code IN ('orders.confirm', 'orders.cancel')
			ON CONFLICT DO NOTHING`).Error
	})
}
