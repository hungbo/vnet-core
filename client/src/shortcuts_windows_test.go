//go:build windows

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/sys/windows"
)

// Số thứ tự phương thức COM trong shortcuts_windows.go không kiểm được ngoài
// Windows: sai một số là gọi nhầm hàm của đối tượng và dịch vụ nền sập. Ghi một
// lối tắt rồi đọc lại, rồi ghi đè sang đích khác — đi qua mọi phương thức COM mà tệp đó dùng.
func TestLnk_GhiRoiDocLai(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE); err == nil || err == syscall.Errno(1) {
		defer windows.CoUninitialize()
	}

	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.exe"), filepath.Join(dir, "b.exe")
	for _, f := range []string{a, b} {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	lnk := filepath.Join(dir, "VNET.lnk")

	for _, target := range []string{a, b} {
		if err := writeLink(lnk, target, dir, "VNET"); err != nil {
			t.Fatalf("writeLink(%s): %v", target, err)
		}
		got, err := readLinkTarget(lnk)
		if err != nil {
			t.Fatalf("readLinkTarget: %v", err)
		}
		// Chỉ so tên tệp: thư mục tạm có thể ở dạng 8.3 (RUNNER~1) khi ghi
		// và dạng dài khi đọc.
		if !strings.EqualFold(filepath.Base(got), filepath.Base(target)) {
			t.Errorf("đích đọc lại = %q, mong tên %q", got, filepath.Base(target))
		}
	}

	if _, err := readLinkTarget(filepath.Join(dir, "khong-co.lnk")); err == nil {
		t.Error("đọc lối tắt không tồn tại mà không lỗi")
	}
}
