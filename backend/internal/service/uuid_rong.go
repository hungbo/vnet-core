package service

import "strings"

// uuidRongThanhNil biến con trỏ trỏ vào chuỗi rỗng thành nil.
//
// Mọi ô chọn trên trang quản trị khi bị bỏ trống hoặc xoá lựa chọn đều gửi
// xuống "" chứ không phải null. Các cột khoá ngoại là uuid cho phép rỗng, mà
// PostgreSQL từ chối "" cho kiểu uuid — lỗi thô
// `invalid input syntax for type uuid: ""` lọt thẳng ra người dùng. Đã gặp ở ba
// chỗ khác nhau (nhóm của máy, danh mục cha, danh mục của sản phẩm), nên đặt
// chung một hàm thay vì viết lại lần thứ tư.
//
// AGENTS.md §2 gọi đây là bẫy đã âm thầm nuốt dữ liệu thật trước đây.
func uuidRongThanhNil(id *string) *string {
	if id == nil || strings.TrimSpace(*id) == "" {
		return nil
	}
	return id
}
