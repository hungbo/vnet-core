package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Tải chỉ chọn ổ: công cụ vào <ổ>\Tools, mọi nhóm khác vào <ổ>\Games; nhóm nhận như menu máy trạm gộp tab TOOL.
func TestGocTheoODia(t *testing.T) {
	cases := []struct{ drive, category, want string }{
		{"E:", "Game online", `E:\Games`},
		{`e:\`, "Game offline", `E:\Games`},
		{"F:", "Công cụ", `F:\Tools`},
		{"F:", " cong cu ", `F:\Tools`},
		{"F:", "TOOLS", `F:\Tools`},
		{"G:", "", `G:\Games`},
		{"G:", "Nhóm quán tự đặt", `G:\Games`},
	}
	for _, c := range cases {
		got, ok := gocTheoODia(c.drive, c.category)
		assert.True(t, ok, c.drive)
		assert.Equal(t, c.want, got, "%s %s", c.drive, c.category)
	}
	for _, bad := range []string{"", "E", `E:\Games`, `\\may\chia-se`, "/", "EE:"} {
		_, ok := gocTheoODia(bad, "Game online")
		assert.False(t, ok, "%q không phải chữ ổ", bad)
	}
}
