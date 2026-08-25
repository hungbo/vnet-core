package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Dịch vụ Windows chạy ở session 0 và KHÔNG thừa hưởng biến môi trường của
// người đăng nhập, nên config.json cạnh tệp .exe là đường DUY NHẤT để bộ cài
// nói chuyện với nó. Biến môi trường vẫn phải thắng, để còn thử tay bằng dòng
// lệnh mà không phải sửa tệp.
func TestLoadConfig_FileAndEnvPrecedence(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skip("không xác định được đường dẫn tệp thực thi")
	}

	data, _ := json.Marshal(fileConfig{
		ServerURL:   "http://tu-tep:8080",
		MachineCode: "TU-TEP",
	})
	path := filepath.Join(filepath.Dir(exe), "config.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Skipf("không ghi được tệp cạnh binary kiểm thử: %v", err)
	}
	defer os.Remove(path)

	os.Unsetenv("VNET_SERVER_URL")
	os.Unsetenv("VNET_MACHINE_CODE")

	cfg := LoadConfig()
	if cfg.ServerURL != "http://tu-tep:8080" {
		t.Fatalf("ServerURL = %q, mong đọc được từ config.json", cfg.ServerURL)
	}
	if cfg.MachineCode != "TU-TEP" {
		t.Fatalf("đọc thiếu trường từ config.json: %+v", cfg)
	}

	t.Setenv("VNET_MACHINE_CODE", "TU-ENV")
	if cfg := LoadConfig(); cfg.MachineCode != "TU-ENV" {
		t.Fatalf("MachineCode = %q — biến môi trường phải thắng tệp", cfg.MachineCode)
	}
}

// Không có tệp cấu hình vẫn phải chạy: bản portable dùng biến môi trường như
// trước, và mã máy rơi về tên máy.
func TestLoadConfig_NoFileStillWorks(t *testing.T) {
	os.Unsetenv("VNET_MACHINE_CODE")

	cfg := LoadConfig()
	if cfg.MachineCode == "" {
		t.Fatal("MachineCode rỗng — phải rơi về tên máy")
	}
	if cfg.ServerURL == "" {
		t.Fatal("ServerURL rỗng")
	}
}
