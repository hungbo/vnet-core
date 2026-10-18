package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// GameMenuEntry là một mục trên menu máy trạm, chạy một chương trình trong thư mục
// game. Launcher và Icon tương đối trong thư mục game, như Game.Launcher.
type GameMenuEntry struct {
	DisplayName string `json:"display_name"`
	Launcher    string `json:"launcher"`
	LaunchArgs  string `json:"launch_args"`
	Icon        string `json:"icon"`
}

// GameMenuEntries lưu vào cột jsonb (xem StringArray: lát cắt Go trần với thẻ jsonb
// bị bỏ qua trong im lặng). nil và rỗng đều lưu NULL: "không có mục riêng".
type GameMenuEntries []GameMenuEntry

func (e GameMenuEntries) Value() (driver.Value, error) {
	if len(e) == 0 {
		return nil, nil
	}
	b, err := json.Marshal([]GameMenuEntry(e))
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (e *GameMenuEntries) Scan(src interface{}) error {
	if src == nil {
		*e = nil
		return nil
	}
	var raw []byte
	switch v := src.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("GameMenuEntries: không đọc được kiểu %T", src)
	}
	if len(raw) == 0 || string(raw) == "null" {
		*e = nil
		return nil
	}
	var out []GameMenuEntry
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("GameMenuEntries: %w", err)
	}
	*e = out
	return nil
}

func (GameMenuEntries) GormDataType() string { return "jsonb" }
