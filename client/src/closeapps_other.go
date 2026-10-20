//go:build !windows

package main

// Phím nóng và đóng ứng dụng chỉ có trên máy trạm Windows.
func osListOpenApps() []openApp { return nil }

func osTerminateApp(openApp) error { return nil }

func osRegisterCloseHotkey(func()) func() { return func() {} }
