package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Tệp .old của những lần cập nhật trước (kể cả bản đổi tên dự phòng .old-<giờ>)
// phải được dọn, nếu không mỗi lần cập nhật lại để thêm một tệp 13 MB.
func TestDonTepCu_XoaMoiBanCu(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skip(err)
	}
	dir := filepath.Dir(exe)
	files := []string{exe + ".old", exe + ".old-1700000000"}
	for _, f := range files {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Skipf("không ghi được vào %s: %v", dir, err)
		}
	}
	tai := filepath.Join(os.TempDir(), "vnet-update")
	if err := os.MkdirAll(tai, 0o755); err == nil {
		_ = os.WriteFile(filepath.Join(tai, "vnet-client-0.0.1.exe"), []byte("x"), 0o644)
		files = append(files, filepath.Join(tai, "vnet-client-0.0.1.exe"))
	}
	donTepCu()
	for _, f := range files {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("%s vẫn còn", filepath.Base(f))
			os.Remove(f)
		}
	}
	if _, err := os.Stat(exe); err != nil {
		t.Fatalf("xoá nhầm chính tệp đang chạy: %v", err)
	}
}
