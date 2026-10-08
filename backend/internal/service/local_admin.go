package service

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// Tài khoản quản trị máy trạm: nhân viên kỹ thuật gõ vào chính ô đăng nhập trên
// màn hình khoá để mở máy ở chế độ bảo trì, kể cả khi mất mạng. Bộ cài đặt nó
// lần đầu; đặt ở đây thì mọi máy nhận bản mới ở nhịp tim kế tiếp.
//
// Máy chủ chỉ giữ BĂM. Mật khẩu gửi lên qua khoá ghi-một-chiều
// local_admin_password, băm xong là bỏ, không bao giờ nằm trong database.
const (
	LocalAdminUserKey = "local_admin_username"
	LocalAdminHashKey = "local_admin_hash"
	LocalAdminPassKey = "local_admin_password"

	localAdminMinPass = 6
	localAdminMaxUser = 64

	// Cùng định dạng với máy trạm (client/src/pin.go): máy trạm so mật khẩu
	// bằng đúng chuỗi này, lệch một tham số là không ai mở được máy.
	localAdminIterations = 210000
	localAdminSaltLen    = 16
	localAdminKeyLen     = 32
)

func hashLocalAdminPassword(password string) (string, error) {
	salt := make([]byte, localAdminSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	return hashLocalAdminWithSalt(password, salt)
}

func hashLocalAdminWithSalt(password string, salt []byte) (string, error) {
	key, err := pbkdf2.Key(sha256.New, strings.TrimSpace(password), salt, localAdminIterations, localAdminKeyLen)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", localAdminIterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// isSecretSetting: khoá không bao giờ trả ra qua API đọc cài đặt. GET
// /settings/:group mở cho mọi tài khoản đã đăng nhập, kể cả hội viên — lộ băm
// ở đó là cho khách mang về dò mật khẩu mở máy.
func isSecretSetting(group, key string) bool {
	return group == ClientGroup && key == LocalAdminHashKey
}

// prepareClientSettings biến mật khẩu trần thành băm trước khi lưu nhóm client.
// current là giá trị đang lưu (để biết đã có mật khẩu chưa).
func prepareClientSettings(current map[string]string, in map[string]interface{}) (map[string]interface{}, error) {
	if _, ok := in[LocalAdminHashKey]; ok {
		return nil, errors.New("không ghi thẳng băm mật khẩu — gửi local_admin_password")
	}
	rawPass, hasPass := in[LocalAdminPassKey]
	delete(in, LocalAdminPassKey)
	pass := ""
	if hasPass && rawPass != nil {
		s, ok := rawPass.(string)
		if !ok {
			return nil, errors.New("mật khẩu quản trị máy trạm phải là chuỗi")
		}
		pass = strings.TrimSpace(s)
	}

	user := current[LocalAdminUserKey]
	rawUser, hasUser := in[LocalAdminUserKey]
	if hasUser {
		s, ok := rawUser.(string)
		if rawUser != nil && !ok {
			return nil, errors.New("tên tài khoản quản trị máy trạm phải là chuỗi")
		}
		user = strings.TrimSpace(s)
		if strings.ContainsAny(user, " \t\"") {
			return nil, errors.New("tên tài khoản quản trị máy trạm không được có khoảng trắng hay dấu nháy")
		}
		if len([]rune(user)) > localAdminMaxUser {
			return nil, fmt.Errorf("tên tài khoản quản trị máy trạm tối đa %d ký tự", localAdminMaxUser)
		}
		in[LocalAdminUserKey] = user
	}

	// Xoá tên = bỏ tài khoản trên máy chủ: máy trạm quay về tài khoản đặt lúc cài.
	if user == "" {
		if pass != "" {
			return nil, errors.New("nhập tên tài khoản quản trị máy trạm trước khi đặt mật khẩu")
		}
		if hasUser {
			in[LocalAdminHashKey] = ""
		}
		return in, nil
	}

	if pass == "" {
		if current[LocalAdminHashKey] == "" {
			return nil, errors.New("đặt mật khẩu cho tài khoản quản trị máy trạm")
		}
		return in, nil
	}
	if len([]rune(pass)) < localAdminMinPass {
		return nil, fmt.Errorf("mật khẩu quản trị máy trạm phải có ít nhất %d ký tự", localAdminMinPass)
	}
	hash, err := hashLocalAdminPassword(pass)
	if err != nil {
		return nil, err
	}
	in[LocalAdminHashKey] = hash
	return in, nil
}
