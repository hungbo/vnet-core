package utils

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashAndCheckPassword(t *testing.T) {
	password := "MySecureP@ss123!"

	hash, err := HashPassword(password)
	require.NoError(t, err)
	require.NotEmpty(t, hash)

	assert.True(t, CheckPassword(password, hash))
	assert.False(t, CheckPassword("wrong-password", hash))
	assert.False(t, CheckPassword(password, "$2a$10$invalidhash"))
}

func TestHashPassword_DifferentEachTime(t *testing.T) {
	password := "test-password"
	h1, _ := HashPassword(password)
	h2, _ := HashPassword(password)

	assert.NotEqual(t, h1, h2)
	assert.True(t, CheckPassword(password, h1))
	assert.True(t, CheckPassword(password, h2))
}

func TestGeneratePassword(t *testing.T) {
	tests := []int{8, 16, 32, 64}
	for _, length := range tests {
		pwd, err := GeneratePassword(length)
		require.NoError(t, err)
		assert.Equal(t, length, len(pwd), "password length %d", length)
	}
}

func TestGeneratePassword_Random(t *testing.T) {
	p1, _ := GeneratePassword(16)
	p2, _ := GeneratePassword(16)
	assert.NotEqual(t, p1, p2)
}

func TestGenerateRandomToken(t *testing.T) {
	token, err := GenerateRandomToken(16)
	require.NoError(t, err)
	assert.Equal(t, 32, len(token))

	token2, err := GenerateRandomToken(32)
	require.NoError(t, err)
	assert.Equal(t, 64, len(token2))
}

func TestGenerateRandomToken_Different(t *testing.T) {
	t1, _ := GenerateRandomToken(8)
	t2, _ := GenerateRandomToken(8)
	assert.NotEqual(t, t1, t2)
}

func TestGenerateCode(t *testing.T) {
	assert.Equal(t, "KH-00001", GenerateCode("KH", 1, 5))
	assert.Equal(t, "SP-00123", GenerateCode("SP", 123, 5))
	assert.Equal(t, "NV-00000", GenerateCode("NV", 0, 5))
	assert.Equal(t, "M-00001", GenerateCode("M", 1, 5))
	assert.Equal(t, "KH-99999", GenerateCode("KH", 99999, 5))
}

func TestVietnamTime(t *testing.T) {
	vt := VietnamTime()
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	assert.Equal(t, loc, vt.Location())
}

func TestStartOfDay(t *testing.T) {
	now := time.Date(2025, 6, 15, 14, 30, 45, 0, time.UTC)
	sod := StartOfDay(now)

	assert.Equal(t, 2025, sod.Year())
	assert.Equal(t, time.Month(6), sod.Month())
	assert.Equal(t, 15, sod.Day())
	assert.Equal(t, 0, sod.Hour())
	assert.Equal(t, 0, sod.Minute())
	assert.Equal(t, 0, sod.Second())
	assert.Equal(t, 0, sod.Nanosecond())
}

func TestEndOfDay(t *testing.T) {
	now := time.Date(2025, 6, 15, 14, 30, 45, 0, time.UTC)
	eod := EndOfDay(now)

	assert.Equal(t, 2025, eod.Year())
	assert.Equal(t, time.Month(6), eod.Month())
	assert.Equal(t, 15, eod.Day())
	assert.Equal(t, 23, eod.Hour())
	assert.Equal(t, 59, eod.Minute())
	assert.Equal(t, 59, eod.Second())
}

func TestRoundUp(t *testing.T) {
	tests := []struct {
		v, base, want int64
	}{
		{100, 50, 100},
		{101, 50, 150},
		{149, 50, 150},
		{150, 50, 150},
		{0, 50, 0},
		{47, 10, 50},
		{50, 10, 50},
		{123, 100, 200},
		{999, 500, 1000},
		{100, 0, 100},
		{100, -1, 100},
	}

	for _, tt := range tests {
		got := RoundUp(tt.v, tt.base)
		assert.Equal(t, tt.want, got, "RoundUp(%d, %d)", tt.v, tt.base)
	}
}

func TestSplitFullName(t *testing.T) {
	tests := []struct {
		input     string
		wantFirst string
		wantLast  string
	}{
		{"Nguyen Van A", "Nguyen Van", "A"},
		{"Le Thi B", "Le Thi", "B"},
		{"John", "", "John"},
		{"", "", ""},
		{"  Tran Van  C  ", "Tran Van", "C"},
		{"Hoang", "", "Hoang"},
	}

	for _, tt := range tests {
		first, last := SplitFullName(tt.input)
		assert.Equal(t, tt.wantFirst, first, "SplitFullName(%q) first", tt.input)
		assert.Equal(t, tt.wantLast, last, "SplitFullName(%q) last", tt.input)
	}
}

func TestIsValidPhone(t *testing.T) {
	assert.True(t, IsValidPhone("0123456789"))
	assert.True(t, IsValidPhone("09876543210"))
	assert.True(t, IsValidPhone("0000000000"))
	assert.True(t, IsValidPhone("12345678901"))
	assert.False(t, IsValidPhone(""))
	assert.False(t, IsValidPhone("12345"))
	assert.False(t, IsValidPhone("123456789012"))
	assert.False(t, IsValidPhone("012345678a"))
	assert.False(t, IsValidPhone("phone12345"))
}

func TestIsValidEmail(t *testing.T) {
	assert.True(t, IsValidEmail("user@example.com"))
	assert.True(t, IsValidEmail("test.user@domain.co"))
	assert.True(t, IsValidEmail("a@b.cd"))
	assert.True(t, IsValidEmail("email+tag@example.com"))
	assert.False(t, IsValidEmail(""))
	assert.False(t, IsValidEmail("invalid"))
	assert.False(t, IsValidEmail("@domain.com"))
	assert.False(t, IsValidEmail("user@"))
	assert.False(t, IsValidEmail("user@.com"))
	assert.False(t, IsValidEmail("user@domain"))
	assert.True(t, IsValidEmail("user@domain.c"))
	assert.True(t, IsValidEmail(strings.Repeat("a", 300)+"@b.com"))
}

// Ô "Ngày sinh" trong hộp thoại Thêm hội viên từng hỏng theo CẢ HAI hướng: bỏ
// trống gửi "", chọn ngày gửi "2000-01-01", và *time.Time không đọc được dạng
// nào trong hai dạng đó. Bảng dưới đây khoá lại đúng những gì client thật gửi.
func TestJSONDateAcceptsWhatTheAdminActuallySends(t *testing.T) {
	type payload struct {
		D JSONDate `json:"d"`
	}

	cases := []struct {
		raw     string
		wantNil bool
		wantYMD string
	}{
		{`{"d":""}`, true, ""},
		{`{"d":null}`, true, ""},
		{`{}`, true, ""},
		{`{"d":"2000-01-01"}`, false, "2000-01-01"},
		{`{"d":"2000-01-01T00:00:00+07:00"}`, false, "2000-01-01"},
		{`{"d":"2000-01-01T00:00:00"}`, false, "2000-01-01"},
	}

	for _, c := range cases {
		t.Run(c.raw, func(t *testing.T) {
			var p payload
			if err := json.Unmarshal([]byte(c.raw), &p); err != nil {
				t.Fatalf("không đọc được %s: %v", c.raw, err)
			}
			if c.wantNil {
				if p.D.Time != nil {
					t.Fatalf("mong nil, nhận %v", p.D.Time)
				}
				return
			}
			if p.D.Time == nil {
				t.Fatal("mong có ngày, nhận nil")
			}
			// Ngày sinh phải giữ nguyên ngày theo lịch Việt Nam. Dựng ở UTC thì
			// 01/01 thành 31/12 và phép kiểm vị thành niên lệch một ngày.
			if got := p.D.Time.Format("2006-01-02"); got != c.wantYMD {
				t.Fatalf("ngày = %s, mong %s", got, c.wantYMD)
			}
		})
	}
}

func TestJSONDateRejectsGarbage(t *testing.T) {
	var p struct {
		D JSONDate `json:"d"`
	}
	if err := json.Unmarshal([]byte(`{"d":"hôm qua"}`), &p); err == nil {
		t.Fatal("chuỗi rác phải bị từ chối, không được lặng lẽ thành nil")
	}
}

// Chuỗi escape phải được giải mã trước khi đọc ngày. Bản đầu tôi cắt dấu nháy
// khỏi byte JSON thô, nên "hôm qua" tới bộ đọc dưới dạng "hôm qua" và hiện
// nguyên chuỗi escape đó trong thông báo lỗi cho người dùng.
func TestJSONDateDecodesEscapedString(t *testing.T) {
	var p struct {
		D JSONDate `json:"d"`
	}
	err := json.Unmarshal([]byte(`{"d":"hôm qua"}`), &p)
	if err == nil {
		t.Fatal("chuỗi rác phải bị từ chối")
	}
	if !strings.Contains(err.Error(), "hôm qua") {
		t.Fatalf("thông báo phải nêu giá trị đã giải mã, nhận: %v", err)
	}
}

// Ô ngày để trắng rồi lỡ gõ dấu cách vẫn là "không có ngày".
func TestJSONDateTreatsBlankAsEmpty(t *testing.T) {
	var p struct {
		D JSONDate `json:"d"`
	}
	if err := json.Unmarshal([]byte(`{"d":"   "}`), &p); err != nil {
		t.Fatalf("chuỗi toàn khoảng trắng không được coi là lỗi: %v", err)
	}
	if p.D.Time != nil {
		t.Fatalf("mong nil, nhận %v", p.D.Time)
	}
}

// Chuỗi ngày người dùng nhập phải được hiểu theo giờ Việt Nam, không phải UTC.
//
// time.Parse mặc định neo vào UTC. Lọc "từ 13/09" khi đó thành 13/09 07:00 giờ
// Việt Nam, nên mọi bản ghi rạng sáng — ca đêm của quán net — biến mất khỏi
// chính ngày của nó, còn bản ghi tối hôm trước lại lọt vào.
func TestVietnamLocation_PhanTichNgayTheoGioVietNam(t *testing.T) {
	loc := VietnamLocation()

	parsed, err := time.ParseInLocation("2006-01-02", "2026-09-13", loc)
	if err != nil {
		t.Fatalf("không phân tích được: %v", err)
	}

	_, offset := parsed.Zone()
	if offset != 7*60*60 {
		t.Fatalf("lệch múi giờ: được %d giây, cần %d", offset, 7*60*60)
	}

	// Giao dịch lúc 01:35 giờ Việt Nam ngày 13/09 phải nằm TRONG ngày 13/09.
	giaoDich := time.Date(2026, 9, 13, 1, 35, 7, 0, loc)
	if giaoDich.Before(StartOfDay(parsed)) || giaoDich.After(EndOfDay(parsed)) {
		t.Fatalf("giao dịch %v rơi ra ngoài khoảng %v – %v",
			giaoDich, StartOfDay(parsed), EndOfDay(parsed))
	}

	// Còn nếu neo vào UTC như bản cũ thì chính giao dịch đó bị loại.
	utcParsed, _ := time.Parse("2006-01-02", "2026-09-13")
	if !giaoDich.Before(utcParsed) {
		t.Fatal("phép thử mất ý nghĩa: mốc UTC lẽ ra phải muộn hơn giao dịch rạng sáng")
	}
}
