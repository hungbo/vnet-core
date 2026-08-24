//go:build windows

package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// smShuttingDown là chỉ số GetSystemMetrics báo Windows đang tắt hoặc khởi động
// lại. Không kiểm nó thì mỗi lần tắt máy bình thường đều bị đọc thành "dịch vụ
// bị giết" — dịch vụ dừng trước giao diện là chuyện thường trong lúc tắt máy.
const smShuttingDown = 0x2000

// user32 và GetSystemMetrics đã được khai ở locker.go / screenshot_windows.go.
func systemShuttingDown() bool {
	r, _, _ := procGetSystemMetrics.Call(uintptr(smShuttingDown))
	return r != 0
}

// maintenanceFlagPath là lối thoát cho nhân viên kỹ thuật: có tệp này thì lớp
// bảo vệ đứng yên. Nó nằm cạnh .exe, tức trong Program Files, nên chỉ tài khoản
// quản trị tạo được — khách không tự tạo ra để vô hiệu hoá lớp bảo vệ.
func maintenanceFlagPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), "maintenance.flag")
}

func maintenanceMode() bool {
	p := maintenanceFlagPath()
	if p == "" {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
}

// observeGuard đọc trạng thái dịch vụ và thử bật lại nếu nó không chạy.
//
// Mọi lỗi đều trả về dưới dạng "không đọc được" chứ không phải "dịch vụ chết":
// máy không mở nổi Service Control Manager là chuyện quyền hạn, không phải
// chuyện có kẻ phá — và nhầm hai thứ đó với nhau là tắt máy oan.
func observeGuard(uiUptime time.Duration) guardObservation {
	o := guardObservation{
		UIUptime:           uiUptime,
		SystemShuttingDown: systemShuttingDown(),
		MaintenanceMode:    maintenanceMode(),
	}
	if o.SystemShuttingDown || o.MaintenanceMode {
		return o
	}

	m, err := mgr.Connect()
	if err != nil {
		log.Printf("[guard] không mở được Service Control Manager: %v", err)
		return o
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		// Không đăng ký dịch vụ là bản chạy rời (portable) hoặc máy đang phát
		// triển. Không có gì để canh, và tuyệt đối không được tắt máy.
		return o
	}
	defer s.Close()
	o.ServiceInstalled = true

	status, err := s.Query()
	if err != nil {
		log.Printf("[guard] không đọc được trạng thái dịch vụ: %v", err)
		return o
	}
	o.ServiceQueryOK = true
	o.ServiceRunning = status.State == svc.Running || status.State == svc.StartPending

	if o.ServiceRunning {
		return o
	}

	// Thử cứu trước khi kết tội. Bộ cài mở quyền Start của dịch vụ cho người
	// dùng thường, nên bước này chạy được mà không cần quyền quản trị.
	if err := s.Start(); err != nil {
		log.Printf("[guard] không bật lại được dịch vụ: %v", err)
		return o
	}
	o.StartOK = true
	return o
}
