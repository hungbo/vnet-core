package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// taoTep tạo tệp path dài n byte (dùng write của riotmeta_test.go).
func taoTep(t *testing.T, path string, n int) {
	t.Helper()
	write(t, path, strings.Repeat("x", n), time.Date(2026, 10, 13, 20, 57, 0, 0, time.UTC))
}

func TestNewestWebView2(t *testing.T) {
	root := t.TempDir()
	// So theo từng số: 141.0.3537.57 mới hơn 141.0.3537.9.
	for _, v := range []string{"140.0.3485.94", "141.0.3537.57", "141.0.3537.9"} {
		taoTep(t, filepath.Join(root, v, "msedgewebview2.exe"), 1)
	}
	// Mới hơn nhưng không có msedgewebview2.exe: trình nạp WebView2 không dùng.
	taoTep(t, filepath.Join(root, "142.0.0.1", "msedge.dll"), 1)
	// Không phải thư mục phiên bản.
	for _, v := range []string{"SetupMetrics", "+150.0.0.0", "150..0.0", "150.0.0.0a"} {
		taoTep(t, filepath.Join(root, v, "msedgewebview2.exe"), 1)
	}
	taoTep(t, filepath.Join(root, "150.0.0.0"), 1) // tệp, không phải thư mục
	// Liên kết tới một bản có thật: không đi theo.
	if err := os.Symlink(filepath.Join(root, "140.0.3485.94"), filepath.Join(root, "160.0.0.0")); err != nil {
		t.Log("không tạo được symlink, bỏ phần kiểm liên kết:", err)
	}

	dir, ver := newestWebView2(root)
	if want := filepath.Join(root, "141.0.3537.57"); dir != want || ver != "141.0.3537.57" {
		t.Errorf("newestWebView2 = %q, %q; mong %q", dir, ver, want)
	}
	if dir, ver := newestWebView2(filepath.Join(root, "khong-co")); dir != "" || ver != "" {
		t.Errorf("thư mục không có: %q, %q; mong rỗng", dir, ver)
	}
}

func TestPrefetchPlan(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(t.TempDir(), "vnet-client.exe")
	taoTep(t, exe, 10)
	for rel, n := range map[string]int{
		"icudtl.dat":                               8,
		"resources.pak":                            9,
		"msedge_100_percent.pak":                   2,
		"msedgewebview2.exe":                       5,
		"EBWebView/x64/EmbeddedBrowserWebView.dll": 7,
		"msedge.dll":                               100,
		"d3dcompiler_47.dll":                       4,
		// Không đọc: không có trong webView2First.
		"big.dll":                        50,
		"mid.BIN":                        20,
		"notes.txt":                      1000,
		"swiftshader/vk_swiftshader.dll": 30,
	} {
		taoTep(t, filepath.Join(dir, filepath.FromSlash(rel)), n)
	}
	// Liên kết không bao giờ được đi theo, kể cả thư mục con của webView2First.
	outside := t.TempDir()
	taoTep(t, filepath.Join(outside, "vi.pak"), 2)
	if os.Symlink(outside, filepath.Join(dir, "Locales")) != nil ||
		os.Symlink(filepath.Join(dir, "big.dll"), filepath.Join(dir, "libEGL.dll")) != nil {
		t.Log("không tạo được symlink, bỏ phần kiểm liên kết")
	}

	// Tệp dữ liệu trước, rồi DLL/EXE, exe của giao diện cuối cùng.
	all := []string{"icudtl.dat 8", "resources.pak 9", "msedge_100_percent.pak 2",
		"msedgewebview2.exe 5", "EBWebView/x64/EmbeddedBrowserWebView.dll 7", "msedge.dll 100",
		"d3dcompiler_47.dll 4", "exe 10"}
	cases := []struct {
		ten  string
		exe  string
		max  int64
		want []string
	}{
		{"đủ chỗ", exe, prefetchMaxBytes, all},
		{"vừa khít trần", exe, 8 + 9 + 2 + 5 + 7 + 100, all[:6]},
		// msedge.dll vượt trần thì dừng: d3dcompiler_47.dll vừa chỗ cũng không lấy.
		{"dừng ở tệp đầu tiên vượt trần", exe, 8 + 9 + 2 + 5 + 7 + 5, all[:5]},
		{"trần 0", exe, 0, nil},
		{"không biết exe", "", prefetchMaxBytes, all[:len(all)-1]},
	}
	for _, c := range cases {
		var got []string
		for _, f := range prefetchPlan(c.exe, dir, c.max) {
			rel := "exe"
			if f.path != exe {
				r, _ := filepath.Rel(dir, f.path)
				rel = filepath.ToSlash(r)
			}
			got = append(got, fmt.Sprintf("%s %d", rel, f.size))
		}
		if strings.Join(got, ", ") != strings.Join(c.want, ", ") {
			t.Errorf("%s:\n  được %v\n  mong %v", c.ten, got, c.want)
		}
	}
}

func TestReadAhead(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.dll"), filepath.Join(dir, "b.pak")
	taoTep(t, a, 3*prefetchChunk+5) // nhiều khối
	taoTep(t, b, 10)
	plan := []prefetchFile{
		{a, 3*prefetchChunk + 5},
		{filepath.Join(dir, "mat.dll"), 100}, // biến mất sau lúc liệt kê: bỏ qua
		{b, 4},                               // phình ra sau lúc liệt kê: chỉ đọc 4 byte
	}
	if n, total := readAhead(context.Background(), plan); n != 2 || total != 3*prefetchChunk+5+4 {
		t.Errorf("readAhead = %d tệp, %d byte; mong 2 tệp, %d byte", n, total, 3*prefetchChunk+5+4)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if n, total := readAhead(ctx, plan); n != 0 || total != 0 {
		t.Errorf("ctx đã huỷ: %d tệp, %d byte; mong 0", n, total)
	}
}
