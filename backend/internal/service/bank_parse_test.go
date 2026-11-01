package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Mẫu thật lấy từ app đo trên máy OPPO A5, 31/10/2026 (research/vnet-bank-notification-mau).
const mauMB = "TK 80xxx868|GD: +5,000VND 31/10/26 17:41 |SD: 1,999,339VND|TU: LIENG IN HUNG - 6522392389|ND: Qacziz5650  APPMI9041643 1  LIENG IN HUNG Chuyen tien- Ma GD ACSP/ 60605154"

func TestParseBankNotification_MBMauThat(t *testing.T) {
	got := parseBankNotification("com.mbmobile", "Thông báo biến động số dư", mauMB)
	assert.Equal(t, "mb", got.Bank)
	assert.True(t, got.Parsed)
	assert.True(t, got.Incoming)
	assert.Equal(t, int64(5000), got.Amount)
	assert.Equal(t, "LIENG IN HUNG", got.SenderName)
	assert.Equal(t, "6522392389", got.SenderAccount)
	assert.Equal(t, "ACSP/60605154", got.BankRef)
	assert.True(t, strings.HasPrefix(got.Content, "Qacziz5650"))
	assert.Empty(t, qrCodesIn(got.Content))
}

func TestParseBankNotification_MBTienRa(t *testing.T) {
	got := parseBankNotification("com.mbmobile", "Thông báo biến động số dư",
		"TK 80xxx868|GD: -120,000VND 31/10/26 18:02 |SD: 1,879,339VND|ND: thanh toan dien")
	assert.True(t, got.Parsed)
	assert.False(t, got.Incoming)
	assert.Equal(t, int64(120000), got.Amount)
	assert.Equal(t, "thanh toan dien", got.Content)
}

func TestParseBankNotification_MBCoMaNap(t *testing.T) {
	text := strings.Replace(mauMB, "Qacziz5650", "VN7K2Q9M", 1)
	got := parseBankNotification("com.mbmobile", "Thông báo biến động số dư", text)
	assert.Equal(t, []string{"VN7K2Q9M"}, qrCodesIn(got.Content))
}

func TestParseBankNotification_KhongCoMauDungBoDocChung(t *testing.T) {
	got := parseBankNotification("com.VCB", "Biến động số dư",
		"Số dư TK VCB 0123456789 +50,000 VND lúc 31-10-2026 17:41:00. Số dư 1,234,567 VND. Ref MBVCB.1.2.vn7k2q9m chuyen tien")
	assert.Equal(t, "vcb", got.Bank)
	assert.True(t, got.Parsed)
	assert.True(t, got.Incoming)
	assert.Equal(t, int64(50000), got.Amount)
	assert.Contains(t, qrCodesIn(got.Content), "VN7K2Q9M")
}

func TestParseBankNotification_KhongDocDuoc(t *testing.T) {
	got := parseBankNotification("com.mbmobile", "Kích hoạt thành công Digital OTP",
		"Quý khách đã kích hoạt thành công Digital OTP trên thiết bị A5")
	assert.False(t, got.Parsed)
}

func TestQRCodesIn_ChenDauCach(t *testing.T) {
	assert.Equal(t, []string{"VN7K2Q9M"}, qrCodesIn("chuyen tien VN7K 2Q9M"))
	assert.Equal(t, []string{"VN7K2Q9M"}, qrCodesIn("APP123.vn7k2q9m.CT"))
	// 0/O, 1/I/L không có trong bảng chữ của mã.
	assert.Empty(t, qrCodesIn("VN012345 VNOIL123"))
}
