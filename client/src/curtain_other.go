//go:build !windows

package main

import "time"

// Màn che chỉ có trên Windows. Bản dựng ở nơi khác (để phát triển giao diện)
// không che gì, nên cả bốn hàm đều đứng yên.

func systemUptime() time.Duration { return 0 }

func dockAlreadyRunning() bool { return false }

func showCurtain() {}

func hideCurtain() {}
