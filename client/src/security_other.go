//go:build !windows

package main

// Boot và bảo mật chỉ đọc được trên máy trạm Windows; bản dựng ở nơi khác để phần còn
// lại biên dịch được.
func readSecurity() MachineSecurity { return MachineSecurity{} }
