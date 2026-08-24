package main

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Mã PIN kỹ thuật: đường vào DUY NHẤT không cần máy chủ.
//
// Không có nó thì mất mạng là máy chết cứng — màn hình khoá phủ kín, mà mọi
// cách đăng nhập (hội viên, quét QR, tài khoản quản trị) đều phải hỏi máy chủ.
// Nhân viên đứng trước một cỗ máy không dùng được và không có gì để gõ vào.
//
// Băm bằng PBKDF2-SHA256 của thư viện chuẩn, không phải SHA256 trần: tệp
// config.json nằm trong Program Files nên KHÁCH ĐỌC ĐƯỢC. Một mã PIN sáu chữ số
// băm bằng SHA256 trần thì dò hết cả triệu khả năng mất chưa tới một giây.

const (
	pinIterations = 210000
	pinSaltLen    = 16
	pinKeyLen     = 32
	pinPrefix     = "pbkdf2-sha256"
)

var errPinFormat = errors.New("chuỗi băm PIN sai định dạng")

// hashPin sinh chuỗi để ghi vào config.json.
func hashPin(pin string) (string, error) {
	pin = strings.TrimSpace(pin)
	if len(pin) < 4 {
		return "", errors.New("PIN phải có ít nhất 4 ký tự")
	}

	salt := make([]byte, pinSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, pin, salt, pinIterations, pinKeyLen)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s$%d$%s$%s", pinPrefix, pinIterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// verifyPin so PIN người dùng gõ với chuỗi băm đã lưu.
//
// So bằng ConstantTimeCompare: máy trạm đứng ngay trước mặt người muốn phá, và
// so sánh chuỗi thường rò rỉ độ dài tiền tố khớp qua thời gian chạy.
func verifyPin(stored, pin string) error {
	if stored == "" {
		return errors.New("máy này chưa đặt PIN kỹ thuật")
	}

	phan := strings.Split(stored, "$")
	if len(phan) != 4 || phan[0] != pinPrefix {
		return errPinFormat
	}
	iter, err := strconv.Atoi(phan[1])
	if err != nil || iter < 1 {
		return errPinFormat
	}
	salt, err := base64.RawStdEncoding.DecodeString(phan[2])
	if err != nil {
		return errPinFormat
	}
	want, err := base64.RawStdEncoding.DecodeString(phan[3])
	if err != nil {
		return errPinFormat
	}

	got, err := pbkdf2.Key(sha256.New, strings.TrimSpace(pin), salt, iter, len(want))
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return errors.New("PIN không đúng")
	}
	return nil
}
