package service

import (
	"bytes"
	"strings"
	"testing"
)

func TestFormatMoney(t *testing.T) {
	cases := map[int64]string{
		0: "0", 5: "5", 999: "999", 1000: "1.000",
		20000: "20.000", 280000: "280.000", 1234567: "1.234.567",
		-50000: "-50.000",
	}
	for in, want := range cases {
		if got := formatMoney(in); got != want {
			t.Errorf("formatMoney(%d) = %q, mong %q", in, got, want)
		}
	}
}

func TestFoldVietnamese(t *testing.T) {
	cases := map[string]string{
		"Cà phê sữa đá":      "Ca phe sua da",
		"Bánh mì thịt nướng": "Banh mi thit nuong",
		"Trà tắc":            "Tra tac",
		"ĐẬU PHỘNG":          "DAU PHONG",
		"Coca":               "Coca",
	}
	for in, want := range cases {
		if got := foldVietnamese(in); got != want {
			t.Errorf("foldVietnamese(%q) = %q, mong %q", in, got, want)
		}
	}
	// Ký tự ngoài ASCII không phải tiếng Việt không được lọt xuống máy in.
	if got := foldVietnamese("珍珠"); got != "??" {
		t.Errorf("ký tự lạ = %q, mong \"??\"", got)
	}
}

// CP1258 biểu diễn tiếng Việt bằng chữ cái tổ hợp sẵn (â/ê/ô/ă/ơ/ư) CỘNG một
// byte dấu thanh riêng. Tách sai loại dấu là ra giấy rác.
//
// Test này đi qua line() — ĐÚNG đường mà hoá đơn thật đi. Bản trước gọi thẳng
// hàm mã hoá với chuỗi còn dấu, một đường mà code thật không bao giờ dùng: nó
// xanh trong khi máy in thật nhận được chữ không dấu.
func TestCP1258GoesThroughTheRealPath(t *testing.T) {
	d := newESCPOSDoc(32, EncodingCP1258)
	d.line("Cà phê sữa đá")
	want := []byte{
		0x43, 0x61, 0xCC, // C, a, dấu huyền
		0x20, 0x70, 0x68, 0xEA, // ' ', p, h, ê
		0x20, 0x73, 0xFD, 0xDE, 0x61, // ' ', s, ư, dấu ngã, a
		0x20, 0xF0, 0x61, 0xEC, // ' ', đ, a, dấu sắc
		0x0A,
	}
	if string(d.Bytes()) != string(want) {
		t.Errorf("cp1258 qua line() = % x\n                    mong % x", d.Bytes(), want)
	}
}

// Chốt chặn chống hồi quy: chế độ có dấu không được gửi xuống chữ đã bỏ dấu.
func TestCP1258DoesNotStripAccents(t *testing.T) {
	d := newESCPOSDoc(32, EncodingCP1258)
	d.twoCol("Cà phê sữa đá", "50.000")
	d.wrap("Bánh mì thịt nướng đặc biệt", 2)
	if bytes.Contains(d.Bytes(), []byte("Ca phe sua da")) {
		t.Error("twoCol bỏ dấu ở chế độ cp1258")
	}
	if bytes.Contains(d.Bytes(), []byte("Banh mi")) {
		t.Error("wrap bỏ dấu ở chế độ cp1258")
	}
	if !bytes.Contains(d.Bytes(), []byte{0xEA}) { // ê
		t.Error("không thấy byte có dấu nào trong dữ liệu gửi máy in")
	}
}

func TestASCIIModeStripsAccents(t *testing.T) {
	d := newESCPOSDoc(32, "")
	d.line("Cà phê")
	if got := string(d.Bytes()); got != "Ca phe\n" {
		t.Errorf("mặc định = %q, mong bỏ dấu", got)
	}
}

// Nếu có ký tự CP1258 không nuốt được, phải rơi về bỏ dấu chứ không được trả
// chuỗi rỗng — mất cả dòng trên hoá đơn là tệ hơn mất dấu.
func TestCP1258FallsBackInsteadOfDroppingLine(t *testing.T) {
	d := newESCPOSDoc(32, EncodingCP1258)
	d.line("Trà 珍珠")
	if len(d.Bytes()) <= 1 {
		t.Fatal("cả dòng biến mất")
	}
	if !bytes.Contains(d.Bytes(), []byte("Tra")) {
		t.Errorf("bản dự phòng = %q, mong chứa \"Tra\"", d.Bytes())
	}
}

// Căn cột phải tính theo Ô CHỮ trên giấy, không theo byte: chữ có dấu tốn nhiều
// byte hơn nhưng vẫn chỉ chiếm một ô.
func TestColumnsCountGlyphsNotBytes(t *testing.T) {
	for _, enc := range []string{EncodingASCII, EncodingCP1258} {
		d := newESCPOSDoc(32, enc)
		d.twoCol("Cà phê sữa đá", "50.000")
		line := strings.TrimRight(d.PlainText(), "\n")
		if n := len([]rune(line)); n != 32 {
			t.Errorf("chế độ %s: dòng rộng %d ô, mong 32: %q", enc, n, line)
		}
	}
}

func TestTwoColAlignsToPaperWidth(t *testing.T) {
	d := newESCPOSDoc(32, EncodingASCII)
	d.twoCol("Tam tinh", "280.000")
	line := strings.TrimRight(d.PlainText(), "\n")
	if len(line) != 32 {
		t.Errorf("dài %d ký tự, mong đúng 32: %q", len(line), line)
	}
	if !strings.HasSuffix(line, "280.000") {
		t.Errorf("cột phải không sát lề: %q", line)
	}
}

// Nhãn quá dài bị cắt chứ không cho tràn: một dòng tiền xuống dòng giữa chừng
// đọc còn khó hơn bị cắt.
func TestTwoColTruncatesRatherThanWrapping(t *testing.T) {
	d := newESCPOSDoc(32, EncodingASCII)
	d.twoCol("Ten mon rat dai vuot qua be rong giay", "99.000")
	out := strings.TrimRight(d.PlainText(), "\n")
	if strings.Contains(out, "\n") {
		t.Errorf("bị tràn thành nhiều dòng: %q", out)
	}
	if len(out) != 32 {
		t.Errorf("dài %d, mong 32: %q", len(out), out)
	}
}

func TestWrapBreaksOnSpaces(t *testing.T) {
	got := wrapCols("Banh mi thit nuong dac biet", 12)
	for _, line := range got {
		if len(line) > 12 {
			t.Errorf("dòng %q dài %d, vượt 12", line, len(line))
		}
	}
	if strings.Join(got, " ") != "Banh mi thit nuong dac biet" {
		t.Errorf("ngắt dòng làm mất chữ: %q", got)
	}
}

func TestWrapHandlesWordLongerThanWidth(t *testing.T) {
	got := wrapCols("AAAAAAAAAAAAAAAAAAAA", 8)
	if len(got) != 3 || got[0] != "AAAAAAAA" {
		t.Errorf("từ dài hơn khổ giấy bị xử lý sai: %q", got)
	}
}

// init phải là ESC @ (đưa máy in về mặc định). Thiếu nó thì một hoá đơn in đậm
// mà quên tắt sẽ làm hoá đơn kế tiếp in đậm theo.
func TestInitResetsPrinter(t *testing.T) {
	d := newESCPOSDoc(32, EncodingASCII)
	d.init(0)
	if string(d.Bytes()) != "\x1b@" {
		t.Errorf("init = % x, mong 1b 40", d.Bytes())
	}

	d2 := newESCPOSDoc(32, EncodingASCII)
	d2.init(30)
	if string(d2.Bytes()) != "\x1b@\x1bt\x1e" {
		t.Errorf("init có bảng mã = % x, mong 1b 40 1b 74 1e", d2.Bytes())
	}
}

// Dao cắt nằm cách đầu in vài milimet: không đẩy giấy trước khi cắt là mất mấy
// dòng cuối hoá đơn.
func TestCutFeedsBeforeCutting(t *testing.T) {
	d := newESCPOSDoc(32, EncodingASCII)
	d.cut()
	if string(d.Bytes()) != "\n\n\n\n\x1dV\x00" {
		t.Errorf("cut = % x", d.Bytes())
	}
	// Giấy trắng không được lọt vào bản xem trước.
	if d.PlainText() != "" {
		t.Errorf("bản xem trước dính lệnh cắt: %q", d.PlainText())
	}
}

// Bản xem trước phải là chữ thuần — nếu lẫn byte điều khiển thì nó không còn
// phản ánh đúng tờ giấy.
func TestPlainTextHasNoControlBytes(t *testing.T) {
	d := newESCPOSDoc(32, EncodingASCII)
	d.init(0)
	d.align(1)
	d.bold(true)
	d.line("VNET")
	d.bold(false)
	d.separator()
	d.twoCol("TONG", "280.000")
	d.cut()

	for _, r := range d.PlainText() {
		if r < 0x20 && r != '\n' {
			t.Fatalf("bản xem trước còn byte điều khiển %#x trong %q", r, d.PlainText())
		}
	}
	if !strings.Contains(d.PlainText(), "VNET") || !strings.Contains(d.PlainText(), "280.000") {
		t.Errorf("bản xem trước thiếu nội dung:\n%s", d.PlainText())
	}
}
