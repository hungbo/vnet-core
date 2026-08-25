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
	ScreenLockEnabled bool
	HighTempThreshold float64
	// Lớp chống phá giờ chơi. Bật mặc định; đặt VNET_GUARD=0 để tắt khi cần
	// gỡ rối trên một máy cụ thể.
	GuardEnabled bool
	// Chuỗi băm PIN kỹ thuật — đường vào duy nhất khi mất mạng. Rỗng nghĩa là
	// máy này không mở khoá được nếu máy chủ không với tới.
	MaintenancePinHash string
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
	// Băm của PIN kỹ thuật, KHÔNG phải PIN. Tệp này nằm trong Program Files nên
	// khách đọc được; ghi PIN trần vào đây là dán chìa khoá lên cửa.
	MaintenancePin string `json:"maintenance_pin"`
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
		ServerURL:          getEnv("VNET_SERVER_URL", firstNonEmpty(fc.ServerURL, "http://localhost:8080")),
		MachineCode:        getEnv("VNET_MACHINE_CODE", fc.MachineCode),
		MaintenancePinHash: fc.MaintenancePin,
		HeartbeatInterval:  15 * time.Second,
		ScreenLockEnabled:  true,
		HighTempThreshold:  85.0,
		GuardEnabled:       getEnv("VNET_GUARD", "1") != "0",
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

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}
