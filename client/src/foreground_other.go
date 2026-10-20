//go:build !windows

package main

// osForegroundApp chỉ có nghĩa trên máy trạm Windows.
func osForegroundApp() (string, bool) { return "", false }
