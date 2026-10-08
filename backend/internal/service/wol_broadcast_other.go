//go:build !windows

package service

import "syscall"

// batBroadcast cho phép socket gửi tới địa chỉ broadcast. Thiếu nó Linux và
// macOS từ chối gói tới x.x.x.255 với lỗi "permission denied".
func batBroadcast(fd uintptr) error {
	return syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
}
