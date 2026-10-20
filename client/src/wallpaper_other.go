//go:build !windows

package main

import "os"

// Hình nền chỉ đặt được trên máy trạm Windows; bản dựng khác để phần còn lại biên dịch được.
func wallpaperDir() string { return os.TempDir() }

func osSetWallpaper(string) error { return nil }
