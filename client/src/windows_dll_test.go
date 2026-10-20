package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Tên DLL trong NewLazyDLL/NewLazySystemDLL chỉ biết sai lúc chạy trên Windows: LazyProc.Call panic "Failed to load ...",
// và panic trong một goroutine nền làm sập CẢ tiến trình giao diện. Máy phát triển là macOS nên không chạy được các tệp
// *_windows.go; bài kiểm này đọc mã nguồn và chỉ nhận đúng các DLL hệ thống đang dùng.
//
// (Lỗi thật đã xảy ra: closeapps_windows.go khai "akUser32.dll" — tiền tố ak của tên biến bị lọt vào tên DLL — nên bật Alt+K
// là giao diện sập.) Thêm DLL mới thì thêm tên thật của nó vào đây.
var systemDLLs = map[string]bool{
	"user32.dll": true, "kernel32.dll": true, "gdi32.dll": true, "shell32.dll": true,
	"ole32.dll": true, "msimg32.dll": true, "powrprof.dll": true,
}

func TestTenDLLLaDLLHeThongThat(t *testing.T) {
	re := regexp.MustCompile(`NewLazy(?:System)?DLL\("([^"]+)"\)`)
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			found++
			if !systemDLLs[strings.ToLower(m[1])] || m[1] != strings.ToLower(m[1]) {
				t.Errorf("%s: DLL %q không phải tên DLL hệ thống (viết thường, ví dụ \"user32.dll\"); sai tên thì lời gọi đầu tiên panic và làm sập giao diện", f, m[1])
			}
		}
	}
	if found == 0 {
		t.Fatal("không tìm thấy lời gọi NewLazyDLL nào: biểu thức tìm đã hỏng")
	}
}
