//go:build !windows

package main

import "time"

// Màn khoá native chỉ có trên Windows. Bản dựng ở nơi khác (để phát triển giao
// diện) không dựng gì: nativeLockStart trả false ngay và màn hình khoá web lo hết.

func systemUptime() time.Duration { return 0 }

func dockAlreadyRunning() bool { return false }

func startNativeLockThread(string, func(string, string) error, func()) bool { return false }

func postNativeLockClose() {}

func postNativeLockFocus() {}
