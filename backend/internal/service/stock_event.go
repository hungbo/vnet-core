package service

import (
	"github.com/vnet/core/internal/hub"
)

// phatTonKhoDoi báo cho mọi màn hình rằng tồn kho của vài sản phẩm vừa đổi.
//
// Gọi SAU khi transaction đã commit. Báo trước lúc commit là hứa một con số có
// thể bị rollback ngay sau đó.
//
// Chỉ mang DANH SÁCH ID, không mang số lượng — khác hẳn balance:updated vốn đẩy
// giá trị tuyệt đối. Lý do: tồn kho mà API trả về là giá trị SUY DIỄN, không
// phải cột thô. Món có công thức thì tồn kho của nó là min(tồn nguyên liệu /
// định mức), nên trừ MỘT nguyên liệu làm đổi số suất của MỌI món dùng nguyên
// liệu đó. Đẩy con số buộc tầng phát phải tra ngược product_ingredients rồi tự
// tính lại cho N món — tính sai một chỗ là hiện sai, mà không ai phát hiện ra.
// Đẩy tín hiệu rồi để client hỏi lại máy chủ thì luôn đúng.
//
// Vẫn tự lành như quy tắc gửi giá trị tuyệt đối: một gói tin rơi mất thì lần
// đổi kho kế tiếp, hoặc lần khách đổi danh mục, sẽ kéo lại số đúng.
//
// Broadcast cho tất cả, không nhắm ai: tồn kho không riêng tư như số dư, và mọi
// thực đơn đang mở lẫn trang quản trị đều cần biết.
func phatTonKhoDoi(h *hub.Hub, productIDs ...string) {
	if h == nil || len(productIDs) == 0 {
		return
	}

	// Gộp trùng: một đơn nhiều dòng cùng sản phẩm, hoặc nhiều món chung một
	// nguyên liệu, không cần lặp id trong gói tin.
	daCo := make(map[string]bool, len(productIDs))
	ids := make([]string, 0, len(productIDs))
	for _, id := range productIDs {
		if id == "" || daCo[id] {
			continue
		}
		daCo[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return
	}

	h.Broadcast(hub.Event{
		Type: "stock:changed",
		Data: map[string]interface{}{"product_ids": ids},
	})
}
