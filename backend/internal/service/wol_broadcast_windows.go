//go:build windows

package service

import "syscall"

// batBroadcast cho phép socket gửi tới địa chỉ broadcast.
func batBroadcast(fd uintptr) error {
	return syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
}
