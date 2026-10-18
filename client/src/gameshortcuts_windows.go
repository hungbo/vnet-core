//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// Phía Windows của gameshortcuts.go: Desktop của người dùng hiện tại và đọc/ghi
// lối tắt .lnk bằng COM (hạ tầng ở shortcuts_windows.go).

// userDesktopDir là Desktop của tài khoản Windows đang chạy giao diện — KHÔNG phải
// Public Desktop. Hỏi hệ điều hành chứ không ghép %USERPROFILE%\Desktop: Desktop
// có thể đã được chuyển sang chỗ khác (OneDrive, thư mục chuyển hướng).
func userDesktopDir() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_Desktop, 0)
}

// Mỗi lần đăng nhập hay khôi phục phiên, thanh chính đối chiếu desktop trên một
// goroutine riêng; mutex tên này cho các lần chồng nhau đi lần lượt, kẻo cùng thấy
// "chưa có" rồi cùng ghi một tệp. "Local\" = trong phiên đăng nhập này.
const (
	gameLnkMutexName = `Local\VNETGameShortcuts`
	gameLnkMutexWait = 8 * time.Second
)

// syncDesktopShortcuts đưa desktop trong dir về đúng danh sách desired. Trả về các
// tên tệp bị lối tắt không phải của VNET chiếm.
func syncDesktopShortcuts(dir string, desired []gameShortcutSpec) ([]string, error) {
	// Mutex và COM đều gắn với luồng hệ điều hành: khoá luồng suốt từ lúc lấy tới
	// lúc thả.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	name, err := windows.UTF16PtrFromString(gameLnkMutexName)
	if err != nil {
		return nil, err
	}
	// ERROR_ALREADY_EXISTS đi kèm handle hợp lệ (mutex đã có tiến trình kia tạo).
	h, err := windows.CreateMutex(nil, false, name)
	if h == 0 {
		return nil, fmt.Errorf("CreateMutex: %w", err)
	}
	defer windows.CloseHandle(h)
	switch ev, _ := windows.WaitForSingleObject(h, uint32(gameLnkMutexWait/time.Millisecond)); ev {
	case windows.WAIT_OBJECT_0, windows.WAIT_ABANDONED: // tiến trình giữ trước đã chết: vẫn là của ta
		defer windows.ReleaseMutex(h)
	default:
		return nil, errGameLnkBusy
	}

	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE); err == nil || err == syscall.Errno(1) { // 1 = S_FALSE
		defer windows.CoUninitialize()
	}

	return reconcileDesktop(dir, desired, scanDesktopLnks, func(path string, s gameShortcutSpec) error {
		return writeLinkArgs(path, s.Target, s.Args, s.WorkDir, s.Desc)
	})
}

// scanDesktopLnks liệt kê các tệp .lnk NẰM TRỰC TIẾP trong dir (không đi vào thư
// mục con) và đánh dấu cái nào của VNET. Chỉ tệp thường có Description mở đầu bằng
// gameLnkPrefix mới là của VNET; liên kết mềm, thư mục trùng tên và lối tắt của
// người khác chỉ được ghi nhận là "đã có tên này", không đọc thêm.
func scanDesktopLnks(dir string) ([]desktopLnk, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []desktopLnk
	for _, e := range entries {
		if !strings.EqualFold(filepath.Ext(e.Name()), ".lnk") {
			continue
		}
		l := desktopLnk{File: e.Name()}
		path := filepath.Join(dir, e.Name())
		if fi, err := os.Lstat(path); err == nil && fi.Mode().IsRegular() {
			// Không đọc được (tệp hỏng, bị khoá) thì coi như không phải của VNET.
			if info, err := readLinkInfo(path); err == nil && strings.HasPrefix(info.Desc, gameLnkPrefix) {
				l.Ours, l.Desc, l.Target, l.Args, l.WorkDir = true, info.Desc, info.Target, info.Args, info.WorkDir
			}
		}
		out = append(out, l)
	}
	return out, nil
}
