//go:build windows

package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func windowsSystemDrive() string {
	// SystemDrive là "C:" (không có dấu gạch chéo), mà GetDiskFreeSpaceEx cần
	// gốc thư mục nên phải thêm vào.
	if d := os.Getenv("SystemDrive"); d != "" {
		return d + `\`
	}
	return `C:\`
}

// readGPUName hỏi WMI qua PowerShell.
//
// gopsutil không có API nào cho GPU, và đây là thứ duy nhất trong cả gói cấu
// hình phải gọi ra ngoài tiến trình — nên nó chỉ được chạy MỘT lần lúc khởi
// động, không phải mỗi lần báo cáo.
//
// Máy hai card (đồ hoạ tích hợp + card rời) trả về nhiều dòng; lấy dòng cuối
// vì card rời thường được liệt kê sau, và đó mới là card người ta quan tâm.
func readGPUName() string {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command",
		"(Get-CimInstance Win32_VideoController).Name")
	// Không để cửa sổ console nhấp nháy trên màn hình khách.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	var last string
	for _, line := range strings.Split(string(out), "\n") {
		if name := strings.TrimSpace(line); name != "" {
			last = name
		}
	}
	return last
}
