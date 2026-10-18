package main

import (
	"errors"
	"strings"
)

// Tài khoản quản trị máy trạm: nhân viên kỹ thuật gõ vào chính ô đăng nhập trên
// màn hình khoá để mở máy ở chế độ bảo trì — không mở phiên, không tính tiền,
// và không cần máy chủ.
//
// Hai nguồn, theo thứ tự ưu tiên:
//  1. Trang quản trị (Cài đặt → Máy trạm), gửi xuống kèm nhịp tim rồi lưu vào
//     policy.json. Đổi ở đó là cả quán đổi theo.
//  2. Bộ cài (config.json). Dùng khi máy chủ chưa đặt tài khoản nào.

const localAdminMinPass = 6

func checkLocalAdminInput(user, pass string) error {
	if user == "" {
		return errors.New("thiếu tên tài khoản quản trị máy trạm")
	}
	if strings.ContainsAny(user, " \t\"") {
		return errors.New("tên tài khoản không được có khoảng trắng hay dấu nháy")
	}
	if len([]rune(strings.TrimSpace(pass))) < localAdminMinPass {
		return errors.New("mật khẩu quản trị máy trạm phải có ít nhất 6 ký tự")
	}
	return nil
}

// localAdminCredential trả về tài khoản đang có hiệu lực.
//
// config.json đọc lại mỗi lần: tiến trình giao diện sống cả ngày, còn bộ cài
// có thể đặt lại tài khoản trong lúc đó.
func localAdminCredential() (user, hash string) {
	if p := currentPolicy(); p.LocalAdminUsername != "" && p.LocalAdminHash != "" {
		return p.LocalAdminUsername, p.LocalAdminHash
	}
	fc := loadFileConfig()
	return strings.TrimSpace(fc.LocalAdminUsername), fc.LocalAdminHash
}

// verifyLocalAdmin: trungTen cho biết tên gõ vào có phải tài khoản quản trị máy
// trạm không; dung là mật khẩu cũng khớp. Chỉ băm khi trùng tên, để đăng nhập
// của hội viên không phải chịu thêm một lượt PBKDF2.
func verifyLocalAdmin(username, password string) (trungTen, dung bool) {
	user, hash := localAdminCredential()
	if user == "" || hash == "" || !strings.EqualFold(strings.TrimSpace(username), user) {
		return false, false
	}
	return true, verifyPin(hash, password) == nil
}
