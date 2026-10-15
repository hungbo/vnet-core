package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Mỗi bài kiểm dùng một thư mục riêng: tệp nằm trong thư mục cấu hình của người
// dùng, và ghi vào thư mục thật của người đang chạy test là làm bẩn máy họ.
func dungThuMucTam(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // Linux
	t.Setenv("HOME", t.TempDir())            // macOS
	t.Setenv("AppData", t.TempDir())         // Windows
}

// datTepDemGia ghi một offline-staff.json như khách tự tạo trong thư mục của mình.
func datTepDemGia(t *testing.T) string {
	t.Helper()
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "VNET", "offline-staff.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	hash, err := hashPin("khach-tu-dat")
	if err != nil {
		t.Fatal(err)
	}
	body := `[{"username":"gia","hash":"` + hash + `","role":"staff","saved_at":"2099-01-01T00:00:00Z"}]`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// Khách ghi được offline-staff.json: nếu máy còn tin tệp đó, khách tự thêm một
// "nhân viên", rút dây mạng rồi mở máy chơi không mất tiền.
func TestLogin_MatMangKhongMoMayBangTepDemCuaKhach(t *testing.T) {
	dungThuMucTam(t)
	datTepDemGia(t)
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // máy chủ không với tới
	a := &App{cfg: &Config{ServerURL: url}}

	out, err := a.Login("gia", "khach-tu-dat")
	if err == nil {
		t.Fatalf("mất mạng mà vẫn mở máy bằng tệp của khách: %s", out)
	}
	if !strings.Contains(err.Error(), "quản trị máy trạm") {
		t.Errorf("câu lỗi không chỉ đường vào bằng tài khoản quản trị máy trạm: %v", err)
	}
	if a.baoTri.Load() {
		t.Error("máy đã vào chế độ bảo trì")
	}
}

// Tệp bản cũ để lại (trong image, có băm mật khẩu nhân viên thật) bị xoá khi giao
// diện khởi động.
func TestPurgeCredCache_XoaTepCu(t *testing.T) {
	dungThuMucTam(t)
	p := datTepDemGia(t)

	purgeCredCache()
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("tệp vẫn còn: %v", err)
	}
	purgeCredCache() // không có tệp cũng không sao
}
