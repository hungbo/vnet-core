package main

import (
	"strings"
	"testing"
)

func TestPin_BamRoiKiemLaiDuoc(t *testing.T) {
	h, err := hashPin("246810")
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyPin(h, "246810"); err != nil {
		t.Fatalf("PIN đúng bị từ chối: %v", err)
	}
	if err := verifyPin(h, "246811"); err == nil {
		t.Fatal("PIN sai vẫn được chấp nhận")
	}
	// Khoảng trắng thừa lúc gõ không được tính là sai.
	if err := verifyPin(h, "  246810 "); err != nil {
		t.Fatalf("PIN có khoảng trắng thừa bị từ chối: %v", err)
	}
}

// Hai lần băm cùng một PIN phải ra hai chuỗi khác nhau. Không có muối ngẫu
// nhiên thì so hai tệp config.json là biết hai máy có chung PIN hay không.
func TestPin_MoiLanBamMotMuoiKhac(t *testing.T) {
	a, _ := hashPin("246810")
	b, _ := hashPin("246810")
	if a == b {
		t.Fatal("hai lần băm ra chuỗi giống hệt — thiếu muối ngẫu nhiên")
	}
	if err := verifyPin(b, "246810"); err != nil {
		t.Fatal(err)
	}
}

// Chuỗi băm phải KHÔNG chứa PIN.
func TestPin_ChuoiBamKhongLoPin(t *testing.T) {
	h, _ := hashPin("246810")
	if strings.Contains(h, "246810") {
		t.Fatalf("chuỗi băm chứa nguyên PIN: %s", h)
	}
}

// Máy chưa đặt PIN thì mọi thứ gõ vào đều sai — kể cả chuỗi rỗng. Nếu không,
// máy không cấu hình PIN sẽ mở khoá bằng Enter.
func TestPin_ChuaDatThiKhongMoDuoc(t *testing.T) {
	for _, thu := range []string{"", "246810", "0000"} {
		if err := verifyPin("", thu); err == nil {
			t.Fatalf("máy chưa đặt PIN vẫn mở được bằng %q", thu)
		}
	}
}

// PIN quá ngắn bị từ chối ngay lúc đặt, không phải lúc dùng.
func TestPin_TuChoiPinQuaNgan(t *testing.T) {
	for _, thu := range []string{"", "1", "123", "   "} {
		if _, err := hashPin(thu); err == nil {
			t.Fatalf("nhận PIN quá ngắn: %q", thu)
		}
	}
}

// Chuỗi băm hỏng (tệp bị sửa tay) phải là "không mở được", không phải panic và
// cũng không phải "mở được".
func TestPin_ChuoiBamHongThiKhongMoDuoc(t *testing.T) {
	hong := []string{
		"khong-phai-dinh-dang",
		"pbkdf2-sha256$abc$xx$yy",
		"pbkdf2-sha256$1000$###$yy",
		"md5$1000$aaaa$bbbb",
		"pbkdf2-sha256$1000$aaaa",
	}
	for _, h := range hong {
		if err := verifyPin(h, "246810"); err == nil {
			t.Fatalf("chuỗi băm hỏng vẫn mở được: %s", h)
		}
	}
}
