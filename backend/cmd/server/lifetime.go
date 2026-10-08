//go:build !windows

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// lifetime trả về context kết thúc server và hàm main gọi sau khi dọn dẹp xong.
// Ngoài Windows chỉ có một cách dừng: tín hiệu từ hệ điều hành hoặc Docker.
func lifetime() (context.Context, func()) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	return ctx, stop
}
