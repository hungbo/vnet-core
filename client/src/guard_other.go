//go:build !windows

package main

import "time"

// Máy trạm chỉ chạy thật trên Windows. Trả về một quan sát "không phán được"
// nên luật ở guard.go luôn dừng lại — bản dựng trên macOS/Linux không bao giờ
// tắt máy của người đang phát triển.
func observeGuard(uiUptime time.Duration) guardObservation {
	return guardObservation{UIUptime: uiUptime}
}
