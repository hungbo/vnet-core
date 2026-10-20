//go:build windows

package main

import (
	"log"
	"path/filepath"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	akUser32                       = windows.NewLazySystemDLL("akUser32.dll")
	akProcEnumWindows              = akUser32.NewProc("EnumWindows")
	akProcIsWindowVisible          = akUser32.NewProc("IsWindowVisible")
	akProcGetWindowTextLengthW     = akUser32.NewProc("GetWindowTextLengthW")
	akProcGetWindowThreadProcessID = akUser32.NewProc("GetWindowThreadProcessId")
	akProcGetWindowLongPtrW        = akUser32.NewProc("GetWindowLongPtrW")
	akProcGetWindow                = akUser32.NewProc("GetWindow")
	akProcRegisterHotKey           = akUser32.NewProc("RegisterHotKey")
	akProcUnregisterHotKey         = akUser32.NewProc("UnregisterHotKey")
	akProcGetMessageW              = akUser32.NewProc("GetMessageW")
	akProcPostThreadMessageW       = akUser32.NewProc("PostThreadMessageW")
)

const (
	akGwlExStyle     = ^uintptr(19) // GWL_EXSTYLE = -20
	akWsExToolWindow = 0x00000080
	akGwOwner        = 4

	akModAlt        = 0x0001
	akModNoRepeat   = 0x4000
	akWmHotKey      = 0x0312
	akWmQuit        = 0x0012
	akCloseHotkeyID = 0x56AB
)

// osListOpenApps trả các tiến trình có cửa sổ cấp cao đang hiện, có tiêu đề, không phải cửa sổ công cụ và không thuộc cửa
// sổ khác (cửa sổ phụ/hộp thoại có chủ). Đó là thứ khách nhìn thấy là "ứng dụng đang mở"; dịch vụ nền không có cửa sổ nên
// không bị tính.
func osListOpenApps() []openApp {
	seen := map[uint32]bool{}
	var apps []openApp
	cb := windows.NewCallback(func(hwnd, _ uintptr) uintptr {
		if vis, _, _ := akProcIsWindowVisible.Call(hwnd); vis == 0 {
			return 1
		}
		if n, _, _ := akProcGetWindowTextLengthW.Call(hwnd); n == 0 {
			return 1
		}
		if owner, _, _ := akProcGetWindow.Call(hwnd, akGwOwner); owner != 0 {
			return 1
		}
		if ex, _, _ := akProcGetWindowLongPtrW.Call(hwnd, akGwlExStyle); ex&akWsExToolWindow != 0 {
			return 1
		}
		var pid uint32
		akProcGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid == 0 || seen[pid] {
			return 1
		}
		seen[pid] = true
		if name := processImageName(pid); name != "" {
			apps = append(apps, openApp{PID: pid, Name: name})
		}
		return 1
	})
	akProcEnumWindows.Call(cb, 0)
	return apps
}

// processImageName: tên tệp chạy của tiến trình pid, hoặc "" nếu không đọc được (tiến trình của phiên khác, hệ thống).
func processImageName(pid uint32) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_PATH*2)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return ""
	}
	return filepath.Base(windows.UTF16ToString(buf[:size]))
}

// osTerminateApp tắt hẳn tiến trình (TerminateProcess): ứng dụng treo thì đóng cửa sổ êm không có tác dụng, và Alt+K có
// mặt đúng cho lúc đó.
func osTerminateApp(a openApp) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, a.PID)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.TerminateProcess(h, 1)
}

type akWinMsg struct {
	hwnd    windows.Handle
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	ptX     int32
	ptY     int32
}

// osRegisterCloseHotkey đăng ký Alt+K trên một luồng riêng (phím nóng gắn với luồng đăng ký và cần vòng lặp thông điệp) rồi
// trả hàm gỡ. Đăng ký không được (phím đã bị ứng dụng khác giữ) thì ghi nhật ký và trả hàm gỡ rỗng.
func osRegisterCloseHotkey(onPress func()) func() {
	var (
		wg  sync.WaitGroup
		tid uint32
		ok  bool
	)
	wg.Add(1)
	ready := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		tid = windows.GetCurrentThreadId()
		r, _, err := akProcRegisterHotKey.Call(0, akCloseHotkeyID, akModAlt|akModNoRepeat, 'K')
		ok = r != 0
		if !ok {
			log.Printf("[alt+k] không đăng ký được phím nóng (có thể ứng dụng khác đang giữ): %v", err)
		}
		close(ready)
		if !ok {
			wg.Done()
			return
		}
		defer wg.Done()
		defer akProcUnregisterHotKey.Call(0, akCloseHotkeyID)
		var m akWinMsg
		for {
			r, _, _ := akProcGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 { // 0 = WM_QUIT, -1 = lỗi
				return
			}
			if m.message == akWmHotKey && m.wParam == akCloseHotkeyID {
				onPress()
			}
		}
	}()
	<-ready
	return func() {
		if ok {
			akProcPostThreadMessageW.Call(uintptr(tid), akWmQuit, 0, 0)
		}
		wg.Wait()
	}
}
