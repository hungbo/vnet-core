//go:build !windows

package main

// Máy trạm thật luôn là Windows. Bản này để phần còn lại biên dịch và chạy được
// khi phát triển trên Mac/Linux; giao diện đã xử lý chuỗi rỗng bằng thông báo
// "không khả dụng trên nền tảng này".
func captureScreen() (string, error) { return "", nil }
