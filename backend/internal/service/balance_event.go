package service

import (
	"github.com/vnet/core/internal/hub"
)

// phatSoDuMoi báo số dư mới cho mọi kết nối của hội viên đó.
//
// Gọi SAU khi transaction đã commit. Gửi trước lúc commit là hứa với máy trạm
// một con số có thể bị rollback ngay sau đó, và máy trạm không có cách nào biết
// để rút lại.
//
// Gửi giá trị TUYỆT ĐỐI chứ không phải phần chênh — theo đúng lối SessionService
// đã làm. Một gói tin rơi mất (máy trạm mất mạng chốc lát) tự lành ở lần đổi số
// dư kế tiếp, không cần đồng bộ lại gì cả; gửi phần chênh thì mỗi gói mất là một
// sai lệch vĩnh viễn.
//
// Chỉ gửi cho chính chủ, KHÔNG broadcast: số dư là chuyện riêng của một người,
// mà broadcast thì đẩy nó sang mọi máy trạm trong quán.
func phatSoDuMoi(h *hub.Hub, memberID string, balance, bonus int64) {
	if h == nil || memberID == "" {
		return
	}
	h.SendToUser(memberID, hub.Event{
		Type: "balance:updated",
		Data: map[string]interface{}{
			"member_id":     memberID,
			"balance":       balance,
			"bonus_balance": bonus,
		},
	})

	// Quản trị cũng phải biết, và phải biết trên MỌI thiết bị đang đăng nhập:
	// nhân viên nạp tiền ở điện thoại thì bảng hội viên đang mở trên máy tính
	// không được đứng nguyên con số cũ. BroadcastToType gửi tới mọi kết nối
	// admin, tức mọi thiết bị của mọi nhân viên đang trực.
	//
	// Đây là sự kiện RIÊNG, không phải gửi lại "balance:updated": máy trạm của
	// khách và trang quản trị cần hai thứ khác nhau, và trộn chung thì không
	// tách được ai đang nghe cái gì.
	h.BroadcastToType(hub.Event{
		Type: "member:updated",
		Data: map[string]interface{}{
			"member_id":     memberID,
			"balance":       balance,
			"bonus_balance": bonus,
		},
	}, hub.ClientTypeAdmin)
}

// phatNapTien báo cho máy trạm hiện một dòng "nạp tiền thành công".
//
// Tách khỏi phatSoDuMoi vì hai việc khác nhau: số dư đổi thì lặng lẽ vẽ lại con
// số, còn nạp tiền thì khách cần thấy xác nhận. Trừ tiền theo giờ mà cũng bắn
// toast thì mỗi phút một cái.
func phatNapTien(h *hub.Hub, memberID string, amount int64) {
	if h == nil || memberID == "" {
		return
	}
	h.SendToUser(memberID, hub.Event{
		Type: "topup:confirmed",
		Data: map[string]interface{}{"amount": amount},
	})
}
