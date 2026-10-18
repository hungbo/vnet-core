package service

import (
	"strings"
	"testing"
	"time"

	"github.com/vnet/core/internal/model"
)

func TestRenderComboTicket(t *testing.T) {
	start, end := "22:00:00", "06:00:00"
	exp := time.Date(2026, 11, 3, 0, 0, 0, 0, time.Local)
	combo := &model.Combo{
		Name: "Gói Đêm", Type: "fixed_slot", SlotStart: &start, SlotEnd: &end,
		ApplyDays: model.IntArray{5, 6},
	}
	purchase := &model.ComboPurchase{
		Price: 50000, PaymentMethod: "cash", ExpiresAt: &exp,
		CreatedAt: time.Date(2026, 10, 4, 21, 30, 0, 0, time.Local),
	}
	shop := shopInfo{Name: "VNET", Footer: defaultReceiptFooter}
	p := &model.PrinterConfig{CharsPerLine: 32}

	text := renderComboTicket(shop, combo, purchase, "GoiDem7", "123456", p).PlainText()
	for _, want := range []string{
		"HOA DON MUA COMBO", "GoiDem7", "MAT KHAU", "123456", "Goi Dem",
		"22:00 - 06:00", "T6,T7", "03/11/2026", "50.000", "Tien mat",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("phiếu thiếu %q:\n%s", want, text)
		}
	}

	// Không có mật khẩu (mua cho hội viên có sẵn) thì không in dòng mật khẩu.
	text = renderComboTicket(shop, combo, purchase, "GoiDem7", "", p).PlainText()
	if strings.Contains(text, "MAT KHAU") {
		t.Errorf("không có mật khẩu mà vẫn in dòng MAT KHAU:\n%s", text)
	}
}

func TestFormatMinutes(t *testing.T) {
	cases := map[int]string{45: "45 phut", 60: "1h", 90: "1h30", 300: "5h"}
	for in, want := range cases {
		if got := formatMinutes(in); got != want {
			t.Errorf("formatMinutes(%d) = %q, mong %q", in, got, want)
		}
	}
}
