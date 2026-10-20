package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// GameRunStep là một lệnh chạy TRƯỚC khi mở game (RunFirst): một chương trình trong thư mục game, ví dụ tập lệnh dựng
// cấu hình hay dịch vụ phụ game cần. Launcher tương đối trong thư mục game, như Game.Launcher.
type GameRunStep struct {
	Launcher   string `json:"launcher"`
	LaunchArgs string `json:"launch_args"`
}

// GameRunSteps lưu vào cột jsonb (cùng cách với GameMenuEntries). nil và rỗng đều lưu NULL: "không có lệnh chạy trước".
type GameRunSteps []GameRunStep

func (r GameRunSteps) Value() (driver.Value, error) {
	if len(r) == 0 {
		return nil, nil
	}
	b, err := json.Marshal([]GameRunStep(r))
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (r *GameRunSteps) Scan(src interface{}) error {
	raw, err := jsonBytes("GameRunSteps", src)
	if err != nil || raw == nil {
		*r = nil
		return err
	}
	var out []GameRunStep
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("GameRunSteps: %w", err)
	}
	*r = out
	return nil
}

func (GameRunSteps) GormDataType() string { return "jsonb" }

// GameCustom là phần RIÊNG của quán trên một game: catalog của Hub không bao giờ ghi đè nó. Trường nào rỗng là theo
// game gốc.
type GameCustom struct {
	// DisplayName và Icon chỉ áp cho game một mục (game có mục menu riêng giữ tên và icon của từng mục).
	DisplayName string `json:"display_name,omitempty"`
	Icon        string `json:"icon,omitempty"`
	// ClientRoot: gốc game NHƯ MÁY TRẠM THẤY riêng cho game này (khác ổ chung của quán), dạng D:\Games2.
	ClientRoot string `json:"client_root,omitempty"`
	// RunFirst: nil là theo game gốc; có mặt (kể cả rỗng, nghĩa là bỏ hết) thì thay hẳn lệnh chạy trước của game gốc.
	RunFirst *GameRunSteps `json:"run_first,omitempty"`
}

// IsZero: quán chưa tuỳ chỉnh gì.
func (c GameCustom) IsZero() bool {
	return c.DisplayName == "" && c.Icon == "" && c.ClientRoot == "" && c.RunFirst == nil
}

func (c GameCustom) Value() (driver.Value, error) {
	if c.IsZero() {
		return nil, nil
	}
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (c *GameCustom) Scan(src interface{}) error {
	raw, err := jsonBytes("GameCustom", src)
	if err != nil || raw == nil {
		*c = GameCustom{}
		return err
	}
	var out GameCustom
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("GameCustom: %w", err)
	}
	*c = out
	return nil
}

func (GameCustom) GormDataType() string { return "jsonb" }

// jsonBytes đọc giá trị cột jsonb; nil nghĩa là NULL hoặc "null".
func jsonBytes(who string, src interface{}) ([]byte, error) {
	var raw []byte
	switch v := src.(type) {
	case nil:
		return nil, nil
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return nil, fmt.Errorf("%s: không đọc được kiểu %T", who, src)
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	return raw, nil
}
