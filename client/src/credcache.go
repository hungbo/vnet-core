package main

import (
	"errors"
	"log"
	"os"
	"path/filepath"
)

var errServerUnreachable = errors.New("không kết nối được máy chủ")

// Bản đệm tài khoản nhân viên để mở máy lúc mất mạng (offline-staff.json) đã bỏ.
//
// Tệp đó nằm trong %APPDATA% của tài khoản Windows dùng chung của KHÁCH: khách đọc
// được băm mật khẩu nhân viên để dò offline, và GHI được: tự thêm một "nhân viên" với
// mật khẩu tự chọn, rút dây mạng rồi mở máy chơi không mất tiền. Lúc mất mạng, nhân
// viên kỹ thuật mở máy bằng tài khoản quản trị máy trạm (localadmin.go), kiểm trước
// cả khi hỏi máy chủ.

// purgeCredCache xoá tệp bản cũ để lại (nằm sẵn trong image, hay trên máy chưa khởi
// động lại sau khi cập nhật).
func purgeCredCache() {
	dir, err := os.UserConfigDir()
	if err != nil {
		return
	}
	p := filepath.Join(dir, "VNET", "offline-staff.json")
	if err := os.Remove(p); err == nil {
		log.Printf("[offline] đã xoá bản đệm nhân viên của bản cũ: %s", p)
	}
}
