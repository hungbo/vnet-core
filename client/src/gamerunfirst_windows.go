//go:build windows

package main

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// shellExecuteInfo là SHELLEXECUTEINFOW của Windows. x/sys không có sẵn ShellExecuteEx, mà ta cần tay nắm tiến trình để
// chờ nó thoát (ShellExecute thường chỉ trả về lúc đã khởi chạy).
type shellExecuteInfo struct {
	cbSize         uint32
	fMask          uint32
	hwnd           windows.Handle
	lpVerb         *uint16
	lpFile         *uint16
	lpParameters   *uint16
	lpDirectory    *uint16
	nShow          int32
	hInstApp       windows.Handle
	lpIDList       uintptr
	lpClass        *uint16
	hkeyClass      windows.Handle
	dwHotKey       uint32
	hIconOrMonitor windows.Handle
	hProcess       windows.Handle
}

const (
	seeMaskNoCloseProcess = 0x00000040
	seeMaskNoAsync        = 0x00000100
	seeMaskFlagNoUI       = 0x00000400
)

var procShellExecuteExW = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW")

// runRunFirstStep chạy một bước chuẩn bị bằng ShellExecuteEx "open" (cùng cách startGame mở tệp chạy chính, nên
// tập lệnh .bat và chương trình đòi quyền quản trị đều chạy được) rồi chờ tiến trình thoát tối đa wait. Quá hạn thì bỏ
// chờ và KHÔNG tắt tiến trình: nó có thể là dịch vụ phụ game cần chạy lâu. Tập lệnh .bat/.cmd chạy ẩn để khách không
// thấy cửa sổ lệnh nhấp nháy; chương trình khác hiện bình thường vì có thể là công cụ có giao diện.
func runRunFirstStep(exe, dir, args string, wait time.Duration) error {
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
	show := int32(windows.SW_SHOWNORMAL)
	if l := strings.ToLower(exe); strings.HasSuffix(l, ".bat") || strings.HasSuffix(l, ".cmd") {
		show = windows.SW_HIDE
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE); err == nil || err == syscall.Errno(1) { // 1 = S_FALSE
		defer windows.CoUninitialize()
	}

	info := shellExecuteInfo{
		fMask:  seeMaskNoCloseProcess | seeMaskNoAsync | seeMaskFlagNoUI,
		lpVerb: verb, lpFile: file, lpParameters: params, lpDirectory: cwd, nShow: show,
	}
	info.cbSize = uint32(unsafe.Sizeof(info))
	if r, _, callErr := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&info))); r == 0 {
		if callErr != nil && !errors.Is(callErr, syscall.Errno(0)) {
			return callErr
		}
		return errors.New("ShellExecuteEx không chạy được")
	}
	if info.hProcess == 0 {
		return nil // Shell xử lý không qua tiến trình riêng (hiếm): không có gì để chờ
	}
	defer windows.CloseHandle(info.hProcess)
	ms := uint32(wait / time.Millisecond)
	ev, err := windows.WaitForSingleObject(info.hProcess, ms)
	switch {
	case err != nil:
		return err
	case ev == uint32(windows.WAIT_TIMEOUT):
		return fmt.Errorf("chưa xong sau %s, bỏ chờ", wait.Round(time.Second))
	}
	return nil
}
