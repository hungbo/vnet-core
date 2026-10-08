//go:build !windows

package main

// Desktop và lối tắt .lnk chỉ có trên máy trạm Windows. Bản này để phần còn lại
// biên dịch được khi phát triển trên Mac/Linux: không có thư mục nào để quét,
// không có lối tắt nào để tạo.

func desktopDirs() []string { return nil }

func ensureVNETShortcut() {}
