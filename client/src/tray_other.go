//go:build !windows

package main

// Khay hệ thống chỉ có trên Windows (tray_windows.go). Bản dựng ở nơi khác để phát
// triển giao diện không có gì để dựng.
func trayStart(func()) {}
