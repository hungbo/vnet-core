//go:build !windows

package main

// Ngoài Windows không có taskbar để né, nên vùng làm việc là cả màn hình. Hàm
// này tồn tại để bản dựng trên macOS/Linux còn biên dịch và chạy test được.
func doVienTaskbar() vienTaskbar { return vienTaskbar{} }
