//go:build !windows

package main

import (
	"errors"
	"time"
)

// Game Disk chỉ có trên máy trạm Windows; bản này để phần còn lại biên dịch được khi phát triển trên Mac/Linux.
func runRunFirstStep(exe, dir, args string, wait time.Duration) error {
	return errors.New("chỉ chạy được lệnh chạy trước trên Windows")
}
