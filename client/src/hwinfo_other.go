//go:build !windows

package main

// Máy trạm chỉ chạy thật trên Windows. Hai hàm này tồn tại để bản dựng trên
// macOS/Linux còn biên dịch và chạy test được, không phải để dùng thật.

func windowsSystemDrive() string { return "/" }

func readGPUName() string { return "" }
