package model

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
)

// IntArray là mảng số nguyên ánh xạ sang kiểu integer[] của PostgreSQL.
//
// Khai `[]int` với thẻ `gorm:"type:integer[]"` KHÔNG chạy: driver mã hoá lát
// cắt Go thành record, và PostgreSQL từ chối:
//
//	column "day_of_week" is of type integer[] but expression is of type record
//
// Kiểu này tự dựng cú pháp mảng của PostgreSQL nên đọc/ghi đều đúng.
type IntArray []int

func (a IntArray) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	parts := make([]string, len(a))
	for i, v := range a {
		parts[i] = strconv.Itoa(v)
	}
	return "{" + strings.Join(parts, ",") + "}", nil
}

func (a *IntArray) Scan(src interface{}) error {
	if src == nil {
		*a = nil
		return nil
	}

	var s string
	switch v := src.(type) {
	case []byte:
		s = string(v)
	case string:
		s = v
	default:
		return fmt.Errorf("IntArray: không đọc được kiểu %T", src)
	}

	s = strings.Trim(strings.TrimSpace(s), "{}")
	if s == "" {
		*a = IntArray{}
		return nil
	}

	parts := strings.Split(s, ",")
	out := make(IntArray, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return fmt.Errorf("IntArray: phần tử %q không phải số nguyên", p)
		}
		out = append(out, n)
	}
	*a = out
	return nil
}

// GormDataType để AutoMigrate tạo đúng kiểu cột.
func (IntArray) GormDataType() string { return "integer[]" }
