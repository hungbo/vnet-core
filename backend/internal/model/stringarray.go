package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// StringArray là danh sách chuỗi lưu vào cột jsonb.
//
// Khai `[]string` trần với thẻ `gorm:"type:jsonb"` không dùng được: driver
// không biết chuyển lát cắt Go thành JSON, nên trường bị bỏ qua trong im lặng —
// đúng chuyện đã xảy ra với MachineAsset.CheckPhotos: khai từ đầu, chưa bao giờ
// lưu được gì.
type StringArray []string

func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	b, err := json.Marshal([]string(a))
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (a *StringArray) Scan(src interface{}) error {
	if src == nil {
		*a = nil
		return nil
	}
	var raw []byte
	switch v := src.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("StringArray: không đọc được kiểu %T", src)
	}
	if len(raw) == 0 || string(raw) == "null" {
		*a = nil
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("StringArray: %w", err)
	}
	*a = out
	return nil
}

func (StringArray) GormDataType() string { return "jsonb" }
