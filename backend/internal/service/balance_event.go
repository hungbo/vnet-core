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
