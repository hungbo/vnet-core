package service

import (
	"strings"
	"testing"
	"time"

	"github.com/vnet/core/internal/model"
)

// Mã bí mật phải đủ dài để dò mù là vô vọng, và không được chứa ký tự dễ đọc
// nhầm khi nhân viên đọc cho khách qua điện thoại.
func TestGeneratedSecretsAreStrongAndReadable(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 500; i++ {
		s, err := randomString(cardSecretLen)
		if err != nil {
			t.Fatalf("sinh mã lỗi: %v", err)
		}
		if len(s) != cardSecretLen {
			t.Fatalf("mã dài %d, mong %d", len(s), cardSecretLen)
		}
		if seen[s] {
			t.Fatalf("sinh trùng mã sau %d lần — nguồn ngẫu nhiên có vấn đề", i)
		}
		seen[s] = true
		for _, r := range s {
			if !strings.ContainsRune(cardAlphabet, r) {
				t.Fatalf("mã chứa ký tự ngoài bảng: %q", r)
			}
		}
		if strings.ContainsAny(s, "01OIL") {
			t.Fatalf("mã chứa ký tự dễ đọc nhầm: %q", s)
		}
	}
}

// Băm phải một chiều: nhìn giá trị lưu trong database không suy ra được mã.
func TestSecretIsHashedNotStored(t *testing.T) {
	secret := "ABCD2345EFGH6789"
	h := hashSecret(secret)
	if h == secret {
		t.Fatal("mã được lưu thô")
	}
	if strings.Contains(h, secret) {
		t.Fatal("mã thô lọt vào giá trị băm")
	}
	if len(h) != 64 {
		t.Errorf("băm dài %d ký tự, mong 64 (SHA-256 hex)", len(h))
	}
	if hashSecret(secret) != h {
		t.Error("băm không ổn định — tra cứu theo băm sẽ hỏng")
	}
}

// Nhân viên gõ lại mã thường viết thường hoặc dính khoảng trắng; không chuẩn
// hoá thì thẻ hợp lệ vẫn bị từ chối.
func TestSecretMatchIgnoresCaseAndSpaces(t *testing.T) {
	stored := hashSecret("ABCD2345EFGH6789")
	for _, given := range []string{
		"ABCD2345EFGH6789",
		"abcd2345efgh6789",
		"  ABCD2345EFGH6789  ",
		"AbCd2345EfGh6789",
	} {
		if !secretMatches(stored, given) {
			t.Errorf("từ chối mã đúng: %q", given)
		}
	}
	for _, given := range []string{"", "ABCD2345EFGH678", "ABCD2345EFGH6788", "X"} {
		if secretMatches(stored, given) {
			t.Errorf("chấp nhận mã sai: %q", given)
		}
	}
}

func TestNormalizeSerial(t *testing.T) {
	for in, want := range map[string]string{
		"tc23456789ab": "TC23456789AB",
		"  TC2345  ":   "TC2345",
		"TC2345":       "TC2345",
	} {
		if got := normalizeSerial(in); got != want {
			t.Errorf("normalizeSerial(%q) = %q, mong %q", in, got, want)
		}
	}
}

// Mọi lý do thẻ hỏng phải trả CÙNG một thông báo. Nói rõ "sai mã" thay vì "sai
// seri" cho phép kẻ tấn công dò ra seri nào có thật rồi mới đánh phần bí mật.
func TestInvalidCardErrorRevealsNothing(t *testing.T) {
	msg := errCardInvalid.Error()
	for _, leak := range []string{"seri", "hết hạn", "đã huỷ", "không tìm thấy", "sai mã"} {
		if strings.Contains(strings.ToLower(msg), leak) {
			t.Errorf("thông báo lỗi lộ lý do %q: %q", leak, msg)
		}
	}
}

func TestParseOptionalTime(t *testing.T) {
	got, err := parseOptionalTime("")
	if err != nil || got != nil {
		t.Errorf("chuỗi rỗng phải cho nil, không lỗi — được %v, %v", got, err)
	}
	if _, err := parseOptionalTime("2026-12-31T00:00:00Z"); err != nil {
		t.Errorf("ngày hợp lệ bị từ chối: %v", err)
	}
	if _, err := parseOptionalTime("31/12/2026"); err == nil {
		t.Error("ngày sai định dạng phải báo lỗi")
	}
}

// Khoá máy trạm dùng chung bộ sinh mã với thẻ, nên cùng đảm bảo về độ mạnh.
// Dài gấp đôi mã thẻ vì nó nằm trong tệp cấu hình chứ không ai gõ tay.
func TestAgentTokenLength(t *testing.T) {
	tok, err := randomString(cardSecretLen * 2)
	if err != nil {
		t.Fatalf("sinh khoá lỗi: %v", err)
	}
	if len(tok) != 32 {
		t.Errorf("khoá dài %d, mong 32", len(tok))
	}
	if hashSecret(tok) == tok {
		t.Error("khoá được lưu thô")
	}
}

// Cột date của PostgreSQL trả về là nửa đêm THEO UTC, còn "hôm nay" ở máy chủ
// là nửa đêm theo giờ địa phương. So bằng time.Equal luôn sai — lỗi này từng
// làm chuỗi điểm danh báo về 0 dù hôm qua vẫn điểm danh.
func TestSameDateIgnoresTimezone(t *testing.T) {
	vn := time.FixedZone("ICT", 7*3600)
	fromDB := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	local := time.Date(2026, 8, 21, 0, 0, 0, 0, vn)

	if fromDB.Equal(local) {
		t.Fatal("mốc kiểm không còn đúng: hai giá trị này lẽ ra khác instant")
	}
	if !sameDate(fromDB, local) {
		t.Error("sameDate không nhận ra hai giá trị cùng một ngày lịch")
	}
	if sameDate(fromDB, local.AddDate(0, 0, 1)) {
		t.Error("sameDate nhận nhầm hai ngày khác nhau")
	}
}

// Khai []int với gorm:"type:integer[]" KHÔNG chạy: driver mã hoá lát cắt Go
// thành record và PostgreSQL từ chối. Lỗi này từng làm hỏng cả lịch chặn
// website lẫn khung ngày áp dụng của gói cước.
func TestIntArrayRoundTrip(t *testing.T) {
	cases := []struct {
		in   model.IntArray
		want string
	}{
		{model.IntArray{}, "{}"},
		{model.IntArray{0}, "{0}"},
		{model.IntArray{1, 2, 3}, "{1,2,3}"},
		{model.IntArray{0, 6}, "{0,6}"},
	}
	for _, tc := range cases {
		v, err := tc.in.Value()
		if err != nil {
			t.Fatalf("Value(%v) lỗi: %v", tc.in, err)
		}
		if v != tc.want {
			t.Errorf("Value(%v) = %q, mong %q", tc.in, v, tc.want)
		}

		var back model.IntArray
		if err := back.Scan(tc.want); err != nil {
			t.Fatalf("Scan(%q) lỗi: %v", tc.want, err)
		}
		if len(back) != len(tc.in) {
			t.Errorf("Scan(%q) ra %v, mong %v", tc.want, back, tc.in)
			continue
		}
		for i := range back {
			if back[i] != tc.in[i] {
				t.Errorf("Scan(%q) ra %v, mong %v", tc.want, back, tc.in)
				break
			}
		}
	}

	// NULL trong database phải thành nil, không phải mảng rỗng lỗi.
	var n model.IntArray
	if err := n.Scan(nil); err != nil || n != nil {
		t.Errorf("Scan(nil) = %v, %v; mong nil, nil", n, err)
	}
	if v, err := model.IntArray(nil).Value(); err != nil || v != nil {
		t.Errorf("Value(nil) = %v, %v; mong nil, nil", v, err)
	}

	// Đọc được cả []byte, vì driver có thể trả về kiểu đó.
	var b model.IntArray
	if err := b.Scan([]byte("{4,5}")); err != nil || len(b) != 2 || b[0] != 4 || b[1] != 5 {
		t.Errorf("Scan([]byte) = %v, %v", b, err)
	}
}

// So phiên bản bằng chuỗi là sai: "1.10.0" < "1.9.0" theo thứ tự chữ cái, nên
// máy đang ở 1.9.0 sẽ không bao giờ thấy bản 1.10.0.
func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"1.10.0", "1.9.0", 1},  // chỗ so chuỗi sẽ sai
		{"2.0", "1.99.99", 1},
		{"1.2", "1.2.0", 0},     // thiếu thành phần coi như 0
		{"1.2.1", "1.2", 1},
		{"v1.3.0", "1.2.0", 1},  // tiền tố v không ảnh hưởng
		{"1.0.0", "", 1},        // máy chưa khai phiên bản thì mọi bản đều mới hơn
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q, %q) = %d, mong %d", c.a, c.b, got, c.want)
		}
	}
}

// Khai []string trần với gorm:"type:jsonb" không dùng được — driver không biết
// chuyển lát cắt Go thành JSON, nên trường bị bỏ qua trong im lặng. Đó là lý do
// MachineAsset.CheckPhotos khai từ đầu mà chưa bao giờ lưu được ảnh nào.
func TestStringArrayRoundTrip(t *testing.T) {
	in := model.StringArray{"/uploads/a.jpg", "/uploads/b.jpg"}
	v, err := in.Value()
	if err != nil {
		t.Fatalf("Value lỗi: %v", err)
	}
	if v != `["/uploads/a.jpg","/uploads/b.jpg"]` {
		t.Errorf("Value = %v", v)
	}

	var back model.StringArray
	if err := back.Scan(v); err != nil || len(back) != 2 || back[0] != in[0] {
		t.Errorf("Scan = %v, %v", back, err)
	}

	// NULL và "null" của jsonb đều phải thành nil, không phải lỗi.
	for _, src := range []interface{}{nil, []byte("null"), []byte("")} {
		var n model.StringArray
		if err := n.Scan(src); err != nil || n != nil {
			t.Errorf("Scan(%v) = %v, %v; mong nil, nil", src, n, err)
		}
	}
	if v, err := model.StringArray(nil).Value(); err != nil || v != nil {
		t.Errorf("Value(nil) = %v, %v", v, err)
	}

	// Mảng rỗng khác nil: rỗng là "đã kiểm, không chụp ảnh nào".
	empty, _ := model.StringArray{}.Value()
	if empty != "[]" {
		t.Errorf("mảng rỗng = %v, mong []", empty)
	}
}
