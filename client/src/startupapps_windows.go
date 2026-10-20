//go:build windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sys/windows"
)

// osProgramFiles: các thư mục Program Files THẬT của máy này, đọc từ biến môi trường của tiến trình (không theo chữ "C:\").
func osProgramFiles() []string {
	var out []string
	for _, k := range []string{"ProgramFiles", "ProgramFiles(x86)", "ProgramW6432"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			out = append(out, filepath.Clean(v))
		}
	}
	return out
}

// osRunningProcessNames: tên các tiến trình đang chạy (chữ thường, không đuôi .exe).
func osRunningProcessNames() map[string]bool {
	out := map[string]bool{}
	procs, err := process.Processes()
	if err != nil {
		return out
	}
	for _, p := range procs {
		if n, err := p.Name(); err == nil {
			out[strings.ToLower(strings.TrimSuffix(n, filepath.Ext(n)))] = true
		}
	}
	return out
}

// osLaunchDetached chạy tệp bằng ShellExecuteEx "open" và KHÔNG chờ: cùng cách mở game (chương trình đòi quyền quản trị hay
// manifest đặc biệt vẫn chạy được), khác ở chỗ ứng dụng tự chạy ở lại suốt phiên của khách.
func osLaunchDetached(exe, dir string) error {
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	cwd, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}
	verb, _ := windows.UTF16PtrFromString("open")

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE); err == nil || err == syscall.Errno(1) { // 1 = S_FALSE
		defer windows.CoUninitialize()
	}
	info := shellExecuteInfo{
		fMask:  seeMaskFlagNoUI,
		lpVerb: verb, lpFile: file, lpDirectory: cwd, nShow: windows.SW_SHOWNORMAL,
	}
	info.cbSize = uint32(unsafe.Sizeof(info))
	if r, _, callErr := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&info))); r == 0 {
		if callErr != nil && !errors.Is(callErr, syscall.Errno(0)) {
			return callErr
		}
		return errors.New("ShellExecuteEx không chạy được")
	}
	return nil
}
