package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tệp hosts là tệp hệ thống dùng chung. Ghi đè cả tệp sẽ xoá mục của người dùng
// và của phần mềm khác — máy có thể mất luôn khả năng phân giải tên miền nội bộ.
func TestWriteHostsBlockKeepsOtherEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hosts")
	original := "127.0.0.1\tlocalhost\n192.168.1.10\tmay-chu-noi-bo\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VNET_HOSTS_FILE", path)

	if err := writeHostsBlock([]string{"facebook.com", "tiktok.com"}); err != nil {
		t.Fatalf("ghi hosts lỗi: %v", err)
	}
	got := readFile(t, path)

	for _, keep := range []string{"localhost", "192.168.1.10\tmay-chu-noi-bo"} {
		if !strings.Contains(got, keep) {
			t.Errorf("mục có sẵn %q bị xoá mất:\n%s", keep, got)
		}
	}
	for _, want := range []string{"127.0.0.1\tfacebook.com", "127.0.0.1\twww.facebook.com",
		"127.0.0.1\ttiktok.com", hostsMarkerBegin, hostsMarkerEnd} {
		if !strings.Contains(got, want) {
			t.Errorf("thiếu %q trong:\n%s", want, got)
		}
	}
}

// Áp danh sách nhiều lần không được nhân bản khối lên.
func TestWriteHostsBlockIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hosts")
	os.WriteFile(path, []byte("127.0.0.1\tlocalhost\n"), 0o644)
	t.Setenv("VNET_HOSTS_FILE", path)

	for i := 0; i < 3; i++ {
		if err := writeHostsBlock([]string{"facebook.com"}); err != nil {
			t.Fatal(err)
		}
	}
	got := readFile(t, path)
	if n := strings.Count(got, hostsMarkerBegin); n != 1 {
		t.Errorf("có %d khối VNET, mong 1:\n%s", n, got)
	}
	if n := strings.Count(got, "127.0.0.1\tfacebook.com\n"); n != 1 {
		t.Errorf("facebook.com xuất hiện %d lần:\n%s", n, got)
	}
}

// Danh sách rỗng phải gỡ sạch khối, trả tệp về nguyên trạng.
func TestWriteHostsBlockEmptyRemovesBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hosts")
	original := "127.0.0.1\tlocalhost\n"
	os.WriteFile(path, []byte(original), 0o644)
	t.Setenv("VNET_HOSTS_FILE", path)

	writeHostsBlock([]string{"facebook.com"})
	if err := writeHostsBlock(nil); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, path)
	if strings.Contains(got, hostsMarkerBegin) || strings.Contains(got, "facebook.com") {
		t.Errorf("khối chặn chưa được gỡ:\n%s", got)
	}
	if !strings.Contains(got, "localhost") {
		t.Errorf("mục có sẵn bị mất:\n%s", got)
	}
}

// Khối dở dang (có mốc mở, mất mốc đóng) không được làm tệp phình mãi.
func TestStripHostsBlockHandlesUnclosedBlock(t *testing.T) {
	content := "127.0.0.1\tlocalhost\n" + hostsMarkerBegin + "\n127.0.0.1\tfacebook.com\n"
	got := stripHostsBlock(content)
	if strings.Contains(got, "facebook.com") || strings.Contains(got, hostsMarkerBegin) {
		t.Errorf("khối dở dang chưa bị cắt: %q", got)
	}
	if !strings.Contains(got, "localhost") {
		t.Errorf("mục có sẵn bị mất: %q", got)
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
