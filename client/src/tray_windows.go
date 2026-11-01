//go:build windows

package main

import (
	"log"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Biểu tượng VNET ở khay hệ thống (góc phải dưới, cạnh đồng hồ của Windows).
//
// Nút Tắt của thanh điều khiển chỉ ẨN cửa sổ (App.HideDock): mất khỏi thanh tác vụ và
// Alt+Tab, nhưng tiến trình vẫn chạy — đồng hồ tính tiền và đường nhận lệnh khoá máy
// không được dừng. Biểu tượng này là đường đưa nó về: bấm chuột trái hay phải đều hiện
// lại thanh điều khiển (App.ShowWindow), như lối tắt VNET ngoài desktop.
//
// Wails v2 không có khay hệ thống, nên tự làm bằng Shell_NotifyIcon: một cửa sổ ẩn trên
// thread riêng nhận thông điệp của biểu tượng. Cửa sổ thường (không phải message-only)
// để nhận được "TaskbarCreated": explorer khởi động muộn hơn máy trạm, hay bị khởi động
// lại, và mỗi lần như vậy khay mới trống trơn — phải thêm biểu tượng lại.

const (
	nimAdd     = 0x0
	nimDelete  = 0x2
	nifMessage = 0x1
	nifIcon    = 0x2
	nifTip     = 0x4

	wmTrayIcon  = 0x8000 + 10 // WM_APP + 10
	wmLButtonUp = 0x0202
	wmRButtonUp = 0x0205

	trayIconID  = 1
	trayRetryID = 1
	// Thêm biểu tượng hỏng (explorer chưa lên hẳn, khay bận) thì thử lại sau chừng này.
	trayRetryMs = 3000

	idiApplication = 32512
)

var (
	shell32              = windows.NewLazySystemDLL("shell32.dll")
	procShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")
	procExtractIconExW   = shell32.NewProc("ExtractIconExW")

	procRegisterWindowMessageW = user32.NewProc("RegisterWindowMessageW")
	procKillTimer              = user32.NewProc("KillTimer")
	procLoadIconW              = user32.NewProc("LoadIconW")

	// Chỉ thread của khay đọc/ghi.
	trayTaskbarCreated uintptr
	trayIcon           uintptr
	trayOnClick        func()
)

type notifyIconData struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         windows.GUID
	hBalloonIcon     uintptr
}

// trayStart dựng biểu tượng khay trên thread riêng. onClick chạy trong goroutine mới
// để thread khay không bao giờ phải chờ giao diện.
func trayStart(onClick func()) {
	go runTray(onClick)
}

// runTray sở hữu cửa sổ ẩn và vòng thông điệp của nó — cùng lý do khoá OS thread như
// runNativeLock. Thanh điều khiển sống tới lúc tắt máy nên vòng này cũng vậy; tắt máy
// thì Windows tự dọn biểu tượng.
func runTray(onClick func()) {
	runtime.LockOSThread()
	trayOnClick = onClick

	hInst, _, _ := procGetModuleHandleW.Call(0)
	className, _ := windows.UTF16PtrFromString("VNETTray")
	wc := wndClassEx{
		lpfnWndProc:   windows.NewCallback(trayWndProc),
		hInstance:     hInst,
		lpszClassName: className,
	}
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	if atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		log.Printf("[khay] không dựng được biểu tượng khay (RegisterClassEx): %v", err)
		return
	}

	taskbar, _ := windows.UTF16PtrFromString("TaskbarCreated")
	trayTaskbarCreated, _, _ = procRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(taskbar)))
	trayIcon = trayLoadIcon()

	// Không bao giờ hiện. WS_EX_TOOLWINDOW cho chắc nó không lọt vào Alt+Tab.
	hwnd, _, err := procCreateWindowExW.Call(
		wsExToolWindow,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("VNET"))),
		wsPopup, 0, 0, 0, 0, 0, 0, hInst, 0)
	if hwnd == 0 {
		log.Printf("[khay] không dựng được biểu tượng khay (CreateWindowEx): %v", err)
		return
	}
	trayAdd(hwnd)

	var m curtainMsg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

// trayLoadIcon lấy biểu tượng nhỏ của chính vnet-client.exe (wails build gắn
// build/windows/icon.ico vào tệp). Tệp không có biểu tượng thì dùng biểu tượng ứng dụng
// mặc định của Windows, còn hơn một ô trống bấm không thấy.
func trayLoadIcon() uintptr {
	if exe, err := os.Executable(); err == nil {
		var nho uintptr
		p, _ := windows.UTF16PtrFromString(exe)
		if n, _, _ := procExtractIconExW.Call(uintptr(unsafe.Pointer(p)), 0, 0,
			uintptr(unsafe.Pointer(&nho)), 1); n != 0 && nho != 0 {
			return nho
		}
	}
	icon, _, _ := procLoadIconW.Call(0, idiApplication)
	return icon
}

func trayData(hwnd uintptr) notifyIconData {
	d := notifyIconData{
		hWnd:             hwnd,
		uID:              trayIconID,
		uFlags:           nifMessage | nifIcon | nifTip,
		uCallbackMessage: wmTrayIcon,
		hIcon:            trayIcon,
	}
	d.cbSize = uint32(unsafe.Sizeof(d))
	tip, _ := windows.UTF16FromString("VNET — bấm để mở")
	copy(d.szTip[:len(d.szTip)-1], tip)
	return d
}

// trayAdd thêm biểu tượng; hỏng thì hẹn giờ thử lại tới khi được.
func trayAdd(hwnd uintptr) {
	d := trayData(hwnd)
	// Explorer khởi động lại thì biểu tượng cũ có thể vẫn được nhớ: xoá trước để lần
	// thêm không hỏng vì trùng.
	procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&d)))
	if ok, _, err := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&d))); ok == 0 {
		log.Printf("[khay] chưa thêm được biểu tượng khay, thử lại sau %d ms: %v", trayRetryMs, err)
		procSetTimer.Call(hwnd, trayRetryID, trayRetryMs, 0)
		return
	}
	procKillTimer.Call(hwnd, trayRetryID)
}

// trayWndProc chạy trên thread của khay.
func trayWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch {
	case msg == wmTrayIcon:
		switch lParam & 0xffff {
		case wmLButtonUp, wmRButtonUp:
			if f := trayOnClick; f != nil {
				go f()
			}
		}
		return 0
	case msg == wmTimer && wParam == trayRetryID:
		trayAdd(hwnd)
		return 0
	case trayTaskbarCreated != 0 && msg == trayTaskbarCreated:
		trayAdd(hwnd)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return r
}
