//go:build windows

package main

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

// Khoá máy trạm.
//
// KHÔNG dùng LockWorkStation: Windows cố ý không có API mở khoá phiên, nên
// khoá kiểu đó là cửa một chiều — nhân viên sẽ phải ra tận máy gõ mật khẩu
// Windows. Thay vào đó cửa sổ ứng dụng phủ kín màn hình (xem App.LockScreen)
// và một hook bàn phím cấp thấp chặn các phím thoát ra.
//
// Giới hạn không vượt qua được: Ctrl+Alt+Del là Secure Attention Sequence, chỉ
// winlogon nhận được — không hook nào chặn được. Đó là thiết kế của Windows.

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procMessageBoxW             = user32.NewProc("MessageBoxW")
	procSetWindowsHookExW       = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx     = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx          = user32.NewProc("CallNextHookEx")
	procGetMessageW             = user32.NewProc("GetMessageW")
	procPostThreadMessageW      = user32.NewProc("PostThreadMessageW")
	procGetAsyncKeyState        = user32.NewProc("GetAsyncKeyState")
	procGetCurrentThreadId      = kernel32.NewProc("GetCurrentThreadId")
)

const (
	whKeyboardLL = 13
	hcAction     = 0
	wmQuit       = 0x0012

	vkTab    = 0x09
	vkEscape = 0x1B
	vkLWin   = 0x5B
	vkRWin   = 0x5C
	vkF4     = 0x73
	vkControl = 0x11

	llkhfAltDown = 0x20
)

type kbdllhookstruct struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type ScreenLocker struct {
	mu       sync.Mutex
	locked   atomic.Bool
	hookTID  uint32
	hookDone chan struct{}
}

func NewScreenLocker() *ScreenLocker {
	return &ScreenLocker{}
}

// Locked cho phép phần còn lại của ứng dụng biết trạng thái mà không cần giữ khoá.
func (l *ScreenLocker) Locked() bool {
	return l.locked.Load()
}

// Lock bật hook chặn phím. Việc phủ kín màn hình do App.LockScreen lo, vì đó là
// thao tác trên cửa sổ Wails chứ không phải trên bàn phím.
func (l *ScreenLocker) Lock() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.locked.Load() {
		return nil
	}
	l.locked.Store(true)

	ready := make(chan error, 1)
	done := make(chan struct{})
	l.hookDone = done

	go l.runHook(ready, done)

	if err := <-ready; err != nil {
		l.locked.Store(false)
		l.hookDone = nil
		return err
	}
	return nil
}

func (l *ScreenLocker) Unlock() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.locked.Load() {
		return
	}
	l.locked.Store(false)

	// Vòng lặp thông điệp nằm ở thread khác; đánh thức nó bằng WM_QUIT để nó
	// tự gỡ hook rồi thoát.
	if l.hookTID != 0 {
		procPostThreadMessageW.Call(uintptr(l.hookTID), wmQuit, 0, 0)
	}
	if l.hookDone != nil {
		<-l.hookDone
		l.hookDone = nil
	}
}

// runHook phải chạy trọn vẹn trên một OS thread cố định: SetWindowsHookEx gắn
// hook vào thread gọi nó, và chỉ thread đó bơm được thông điệp cho hook.
func (l *ScreenLocker) runHook(ready chan<- error, done chan<- struct{}) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(done)

	tid, _, _ := procGetCurrentThreadId.Call()
	l.hookTID = uint32(tid)

	cb := syscall.NewCallback(l.hookProc)
	hook, _, err := procSetWindowsHookExW.Call(whKeyboardLL, cb, 0, 0)
	if hook == 0 {
		ready <- fmt.Errorf("SetWindowsHookEx thất bại: %v", err)
		return
	}
	defer procUnhookWindowsHookEx.Call(hook)
	ready <- nil

	// GetMessage chặn cho tới khi có thông điệp; WM_QUIT trả về 0 và thoát.
	var msg struct {
		Hwnd    uintptr
		Message uint32
		WParam  uintptr
		LParam  uintptr
		Time    uint32
		Pt      struct{ X, Y int32 }
	}
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(ret) <= 0 {
			return
		}
	}
}

// hookProc trả về 1 để nuốt phím, hoặc chuyển tiếp cho hook kế.
//
// Hàm này chạy trong ngữ cảnh xử lý sự kiện bàn phím của Windows: nếu nó chậm,
// Windows lặng lẽ gỡ hook. Không cấp phát, không khoá, không ghi log ở đây.
func (l *ScreenLocker) hookProc(nCode int32, wParam uintptr, lParam uintptr) uintptr {
	if nCode == hcAction && l.locked.Load() {
		k := (*kbdllhookstruct)(unsafe.Pointer(lParam))
		if blockedKey(k) {
			return 1
		}
	}
	ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return ret
}

func blockedKey(k *kbdllhookstruct) bool {
	altDown := k.Flags&llkhfAltDown != 0

	switch k.VkCode {
	case vkLWin, vkRWin:
		return true // phím Windows: mở Start, thoát khỏi lớp phủ
	case vkTab:
		return altDown // Alt+Tab: chuyển cửa sổ
	case vkEscape:
		// Alt+Esc chuyển cửa sổ, Ctrl+Esc mở Start.
		return altDown || ctrlDown()
	case vkF4:
		return altDown // Alt+F4: đóng ứng dụng đang khoá
	}
	return false
}

func ctrlDown() bool {
	ret, _, _ := procGetAsyncKeyState.Call(vkControl)
	return ret&0x8000 != 0
}

func (l *ScreenLocker) ShowMessage(title, message string) error {
	titlePtr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return err
	}
	messagePtr, err := syscall.UTF16PtrFromString(message)
	if err != nil {
		return err
	}
	// MB_OK | MB_TOPMOST | MB_SETFOREGROUND — hộp thoại phải nổi lên trên lớp
	// phủ, nếu không nhân viên gửi thông báo mà khách không thấy gì.
	const flags = 0x00000000 | 0x00040000 | 0x00010000
	ret, _, callErr := procMessageBoxW.Call(0,
		uintptr(unsafe.Pointer(messagePtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		flags)
	if ret == 0 {
		log.Printf("[locker] MessageBox thất bại: %v", callErr)
		return fmt.Errorf("MessageBox thất bại: %v", callErr)
	}
	return nil
}
