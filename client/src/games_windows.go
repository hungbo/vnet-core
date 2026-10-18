//go:build windows

package main

import (
	"runtime"
	"syscall"

	"golang.org/x/sys/windows"
)

// startGame chạy game bằng ShellExecute "open" chứ không dùng os/exec.
//
// Launcher của game hay có manifest đòi quyền quản trị (Riot, Roblox...).
// CreateProcess gặp manifest đó là lỗi 740 và không chạy; ShellExecute thì biết
// bật hộp thoại UAC. Nó cũng không biến game thành tiến trình con có thể bị kéo
// theo khi cửa sổ phụ đóng: bấm X ở menu game không được tắt game đang chơi.
//
// Thư mục làm việc là thư mục chứa tệp chạy, vì nhiều game đọc tệp cấu hình
// theo đường dẫn tương đối.
func startGame(exe, dir, args string) error {
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	cwd, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}
	verb, _ := windows.UTF16PtrFromString("open")
	var params *uint16
	if args != "" {
		if params, err = windows.UTF16PtrFromString(args); err != nil {
			return err
		}
	}

	// Tài liệu của ShellExecute yêu cầu COM đã khởi tạo trên luồng gọi: một số
	// phần mở rộng của Shell cần apartment STA. Luồng đã khởi tạo kiểu khác thì
	// trả RPC_E_CHANGED_MODE — vẫn gọi được, chỉ là không có gì để gỡ.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE); err == nil || err == syscall.Errno(1) { // 1 = S_FALSE
		defer windows.CoUninitialize()
	}

	return windows.ShellExecute(0, verb, file, params, cwd, windows.SW_SHOWNORMAL)
}
