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

// peripheralScript liệt kê thiết bị ngoại vi qua WMI, mỗi dòng "loại|tên".
//
// Tên màn hình trong WmiMonitorID là mảng mã ký tự (EDID), phải ghép lại; ba
// lớp còn lại có tên sẵn. Lỗi từng lớp (máy không có card âm thanh, máy ảo
// không có EDID) bị nuốt để các lớp khác vẫn in ra.
const peripheralScript = `
$ErrorActionPreference = 'SilentlyContinue'
[Console]::OutputEncoding = [Text.Encoding]::UTF8
Get-CimInstance -Namespace root\wmi -ClassName WmiMonitorID | ForEach-Object {
  $n = (($_.UserFriendlyName | Where-Object { $_ -ne 0 } | ForEach-Object { [char]$_ }) -join '').Trim()
  if ($n) { "monitor|$n" }
}
Get-CimInstance Win32_Keyboard | ForEach-Object { "keyboard|" + $_.Description }
Get-CimInstance Win32_PointingDevice | ForEach-Object { "mouse|" + $_.Name }
Get-CimInstance Win32_SoundDevice | ForEach-Object { "sound|" + $_.Name }
`

var peripheralLabels = map[string]string{
	"monitor":  "Màn hình",
	"keyboard": "Bàn phím",
	"mouse":    "Chuột",
	"sound":    "Âm thanh",
}

// readPeripherals chạy MỘT lần lúc khởi động, cùng lý do với readGPUName.
//
// Windows liệt kê một con chuột USB thành hai ba mục HID giống hệt nhau, nên
// trùng tên thì chỉ giữ một. Trần 20 mục: đủ cho máy nhiều màn hình, và không
// để một máy lạ nhồi hàng trăm dòng vào database.
func readPeripherals() []string {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", peripheralScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	seen := map[string]bool{}
	var list []string
	for _, line := range strings.Split(string(out), "\n") {
		kind, name, ok := strings.Cut(strings.TrimSpace(line), "|")
		name = strings.TrimSpace(name)
		label := peripheralLabels[kind]
		if !ok || name == "" || label == "" {
			continue
		}
		item := label + ": " + name
		if seen[item] {
			continue
		}
		seen[item] = true
		list = append(list, item)
		if len(list) == 20 {
			break
		}
	}
	return list
}
