//go:build !windows

package main

import (
	"context"
	"errors"
)

// Bản rỗng cho macOS/Linux. Máy khách chỉ chạy thật trên Windows; những hàm này
// tồn tại để cả gói còn biên dịch và chạy được `go test` ở nơi khác — không có
// chúng thì không ai kiểm được phần logic nền dùng chung trong service.go.

func runAsWindowsService(context.Context, context.CancelFunc) bool { return false }

func installService() error {
	return errors.New("dịch vụ Windows chỉ cài được trên Windows")
}

func uninstallService() error {
	return errors.New("dịch vụ Windows chỉ gỡ được trên Windows")
}

// uiIsRunning luôn báo "đang chạy" để superviseUI không cố bật giao diện trên
// máy không phải Windows.
func uiIsRunning() bool { return true }

func launchUIInUserSession() error {
	return errors.New("chỉ bật được giao diện trong phiên người dùng trên Windows")
}
