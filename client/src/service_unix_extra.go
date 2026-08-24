//go:build !windows

package main

import "errors"

// Bản rỗng cho macOS/Linux — xem ghi chú ở service_unix.go.

func ensureServiceRunning() error {
	return errors.New("chỉ bảo đảm được dịch vụ trên Windows")
}
