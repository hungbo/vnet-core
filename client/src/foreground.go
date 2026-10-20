package main

import "strings"

// Ứng dụng đang ở trên cùng (cửa sổ foreground) của máy trạm — cột "Đang chạy" trên trang
// Phần cứng của admin. Chỉ tên tệp .exe, không lấy tiêu đề cửa sổ: tiêu đề có thể chứa nội
// dung khách đang xem, còn tên tiến trình đủ để biết là game nào hay trình duyệt.

// foregroundMaxLen: cột ở máy chủ rộng 64 ký tự.
const foregroundMaxLen = 64

// exeBaseName lấy tên tệp từ đường dẫn Windows (hoặc Unix, cho bài kiểm), cắt cho vừa cột.
func exeBaseName(path string) string {
	if i := strings.LastIndexAny(path, `\/`); i >= 0 {
		path = path[i+1:]
	}
	path = strings.TrimSpace(path)
	if r := []rune(path); len(r) > foregroundMaxLen {
		path = string(r[:foregroundMaxLen])
	}
	return path
}
