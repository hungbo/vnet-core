//go:build !windows

package main

import "errors"

// Game Disk chỉ có trên máy trạm Windows. Bản này để phần còn lại biên dịch
// được khi phát triển trên Mac/Linux.
func startGame(exe, dir, args string) error {
	return errors.New("chỉ chạy được game trên Windows")
}
