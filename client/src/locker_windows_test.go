//go:build windows

package main

import "testing"

// Bảng phím bị chặn là thứ quyết định khách có thoát được lớp phủ khoá máy hay
// không. Không chạy được trên máy Mac/Linux vì cả tệp locker.go là //go:build
// windows — chạy `go test ./...` NGAY TRÊN MÁY WINDOWS để kiểm.
//
// Chỉ kiểm phần logic thuần (blockedKey/ctrlDown không gọi syscall khi
// ctrl không được nhấn); phần hook thật phải thử tay: khoá máy rồi bấm
// Win / Alt+Tab / Alt+F4 / Ctrl+Esc và xem có thoát ra được không.

func TestBlockedKeyBlocksEscapeRoutes(t *testing.T) {
	cases := []struct {
		name  string
		key   kbdllhookstruct
		block bool
	}{
		{"phím Windows trái", kbdllhookstruct{VkCode: vkLWin}, true},
		{"phím Windows phải", kbdllhookstruct{VkCode: vkRWin}, true},
		{"Alt+Tab", kbdllhookstruct{VkCode: vkTab, Flags: llkhfAltDown}, true},
		{"Alt+F4", kbdllhookstruct{VkCode: vkF4, Flags: llkhfAltDown}, true},
		{"Alt+Esc", kbdllhookstruct{VkCode: vkEscape, Flags: llkhfAltDown}, true},

		// Những phím này KHÔNG được chặn: khách vẫn phải gõ được mật khẩu và
		// dùng máy bình thường khi chưa khoá.
		{"Tab một mình", kbdllhookstruct{VkCode: vkTab}, false},
		{"F4 một mình", kbdllhookstruct{VkCode: vkF4}, false},
		{"chữ A", kbdllhookstruct{VkCode: 0x41}, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := blockedKey(&c.key); got != c.block {
				t.Fatalf("blockedKey = %v, mong %v", got, c.block)
			}
		})
	}
}

// Ctrl+Alt+Del KHÔNG chặn được bằng hook cấp thấp — Windows xử lý riêng ở tầng
// dưới. Bài kiểm này ghi lại giới hạn đó để không ai tưởng lớp khoá là tuyệt đối.
func TestCtrlAltDelIsNotClaimedToBeBlocked(t *testing.T) {
	const vkDelete = 0x2E
	k := kbdllhookstruct{VkCode: vkDelete, Flags: llkhfAltDown}
	if blockedKey(&k) {
		t.Fatal("đừng giả vờ chặn được Ctrl+Alt+Del: Windows không cho hook cấp thấp thấy tổ hợp này")
	}
}
