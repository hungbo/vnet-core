package utils

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
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

// JSONDate là *time.Time nhưng chấp nhận thêm hai dạng mà giao diện thật sự
// gửi, ngoài RFC3339 mà encoding/json đòi hỏi:
//
//	""            → không có ngày (nil)
//	"2006-01-02"  → dạng ElDatePicker sinh ra với value-format="YYYY-MM-DD"
//
// Không có kiểu này, ô "Ngày sinh" trong hộp thoại Thêm hội viên hỏng theo CẢ
// HAI hướng: bỏ trống thì gửi "" (json: cannot unmarshal), chọn ngày thì gửi
// "2000-01-01" (thiếu phần giờ). Cả hai đều rơi vào cùng một câu trả lời
// "Dữ liệu không hợp lệ" không nói field nào sai.
type JSONDate struct {
	Time *time.Time
}

func (d *JSONDate) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		d.Time = nil
		return nil
	}
	// Phải giải mã qua string chứ không cắt dấu nháy khỏi byte thô: byte thô còn
	// nguyên các chuỗi escape (\u00f4), nên "hôm qua" sẽ lọt vào bộ đọc dưới
	// dạng "h\u00f4m qua" và hiện y như vậy trong thông báo lỗi cho người dùng.
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("ngày phải là chuỗi: %w", err)
	}
	if strings.TrimSpace(s) == "" {
		d.Time = nil
		return nil
	}
	s = strings.TrimSpace(s)

	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.UTC
	}

	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		// Ngày sinh chỉ có ý nghĩa theo lịch địa phương: dựng ở UTC thì một
		// người sinh 01/01 thành 31/12 và phép kiểm vị thành niên lệch một ngày.
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			d.Time = &t
			return nil
		}
	}
	return fmt.Errorf("ngày %q không đọc được: cần YYYY-MM-DD hoặc RFC3339", s)
}

func (d JSONDate) MarshalJSON() ([]byte, error) {
	if d.Time == nil {
		return []byte("null"), nil
	}
	return json.Marshal(d.Time)
}
