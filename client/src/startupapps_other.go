//go:build !windows

package main

import "errors"

// Chạy ứng dụng ngoài giao diện chỉ có trên máy trạm Windows.
func osProgramFiles() []string { return nil }

func osRunningProcessNames() map[string]bool { return map[string]bool{} }

func osLaunchDetached(string, string) error { return errors.New("chỉ máy trạm Windows mới chạy được ứng dụng tự chạy") }
