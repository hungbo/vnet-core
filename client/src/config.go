package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	ServerURL   string
	MachineCode       string
	HeartbeatInterval time.Duration
	HighTempThreshold float64
	// Lớp chống phá giờ chơi. Bật mặc định; đặt VNET_GUARD=0 để tắt khi cần
	// gỡ rối trên một máy cụ thể.
	GuardEnabled bool
	WatchdogInterval   time.Duration
	BlocklistInterval  time.Duration
}

// fileConfig là tệp config.json nằm cạnh .exe, do bộ cài ghi ra.
//
// Dịch vụ Windows chạy ở session 0 và KHÔNG thừa hưởng biến môi trường của
// người đăng nhập, nên cấu hình chỉ qua env là không tới được nó. Tệp là nguồn
// chính; env vẫn được giữ làm lớp ghi đè để còn thử tay bằng dòng lệnh.
type fileConfig struct {
	ServerURL   string `json:"server_url"`
	MachineCode string `json:"machine_code"`
	// Tài khoản quản trị máy trạm do bộ cài đặt. Chỉ lưu BĂM mật khẩu: tệp này
	// nằm trong Program Files nên khách đọc được.
	LocalAdminUsername string `json:"local_admin_username,omitempty"`
	LocalAdminHash     string `json:"local_admin_hash,omitempty"`
}

// loadFileConfig đọc config.json cạnh tệp thực thi. Không có tệp không phải lỗi:
// bản chạy portable vẫn cấu hình bằng biến môi trường như trước.
func loadFileConfig() fileConfig {
	var fc fileConfig
	exe, err := os.Executable()
	if err != nil {
		return fc
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(exe), "config.json"))
	if err != nil {
		return fc
	}
	_ = json.Unmarshal(data, &fc)
	return fc
}

func LoadConfig() *Config {
	fc := loadFileConfig()

	cfg := &Config{
		ServerURL:          envOverride("VNET_SERVER_URL", firstNonEmpty(fc.ServerURL, "http://localhost:20800")),
		MachineCode:        envOverride("VNET_MACHINE_CODE", fc.MachineCode),
		HeartbeatInterval:  15 * time.Second,
		HighTempThreshold:  85.0,
		GuardEnabled:       envOverride("VNET_GUARD", "1") != "0",
		WatchdogInterval:   30 * time.Second,
		BlocklistInterval:  60 * time.Second,
	}
	if cfg.MachineCode == "" {
		hostname, err := os.Hostname()
		if err == nil {
			cfg.MachineCode = hostname
		} else {
			cfg.MachineCode = "unknown"
		}
	}
	return cfg
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// envOverride cho biến môi trường ghi đè cấu hình CHỈ ở bản phát triển.
//
// Bản phát hành bỏ qua hẳn: giao diện chạy với môi trường của tài khoản đang
// ngồi máy, mà biến môi trường của mình thì tài khoản nào cũng tự đặt được —
// tức là đổi được máy chủ của giao diện hoặc tắt lớp canh mà không cần quyền gì.
// Bản phát hành chỉ có một nguồn cấu hình: config.json trong thư mục cài.
func envOverride(key, fallback string) string {
	if version != "dev" {
		return fallback
	}
	return getEnv(key, fallback)
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}
