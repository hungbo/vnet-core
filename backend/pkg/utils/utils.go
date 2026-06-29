package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func GeneratePassword(length int) (string, error) {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		result[i] = chars[n.Int64()]
	}
	return string(result), nil
}

func GenerateRandomToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func GenerateCode(prefix string, seq int64, width int) string {
	return fmt.Sprintf("%s-%05d", prefix, seq)
}

func VietnamTime() time.Time {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		return time.Now()
	}
	return time.Now().In(loc)
}

func StartOfDay(t time.Time) time.Time {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

func EndOfDay(t time.Time) time.Time {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 23, 59, 59, 0, loc)
}

func RoundUp(v int64, base int64) int64 {
	if base <= 0 {
		return v
	}
	remainder := v % base
	if remainder == 0 {
		return v
	}
	return v + (base - remainder)
}

func SplitFullName(fullName string) (string, string) {
	fullName = strings.TrimSpace(fullName)
	parts := strings.Fields(fullName)
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return "", parts[0]
	}
	return strings.Join(parts[:len(parts)-1], " "), parts[len(parts)-1]
}

func IsValidPhone(phone string) bool {
	if len(phone) < 10 || len(phone) > 11 {
		return false
	}
	for _, c := range phone {
		if !unicode.IsDigit(c) {
			return false
		}
	}
	return true
}

func IsValidEmail(email string) bool {
	at := strings.LastIndex(email, "@")
	if at < 1 || at >= len(email)-1 {
		return false
	}
	local := email[:at]
	domain := email[at+1:]
	if len(local) == 0 || len(domain) < 3 {
		return false
	}
	dot := strings.LastIndex(domain, ".")
	return dot > 0 && dot < len(domain)-1
}
