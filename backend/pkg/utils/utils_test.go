package utils

import (
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
		input       string
		wantFirst   string
		wantLast    string
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
