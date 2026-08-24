package service

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/unicode/norm"
)

// Bộ dựng lệnh ESC/POS cho máy in nhiệt.
//
// Chỗ dễ sai nhất là tiếng Việt. Máy in nhiệt không hiểu UTF-8: gửi thẳng vào
// thì ra ký tự rác. Có hai đường:
//
//   - "ascii" (mặc định) — bỏ dấu: "Cà phê sữa đá" → "Ca phe sua da". Không đẹp
//     nhưng đọc được trên MỌI máy in, không cần cấu hình gì.
//   - "cp1258" — mã hoá Windows-1258, in đúng dấu, nhưng chỉ chạy nếu máy in có
//     bảng mã đó và phải khai đúng số hiệu bảng mã của hãng (CodePage).
//
// Mặc định là ascii vì nó không bao giờ ra giấy rác. Ai có máy in hỗ trợ tiếng
// Việt thì đổi sang cp1258 và khai số bảng mã.

const (
	EncodingASCII  = "ascii"
	EncodingCP1258 = "cp1258"
)

// Dấu thanh được CP1258 mã hoá riêng thành một byte tổ hợp. Dấu chất lượng
// nguyên âm (mũ, á, móc) thì không — CP1258 chỉ có â/ê/ô/ă/ơ/ư ở dạng tổ hợp
// sẵn. Vì vậy phải tách đúng loại dấu, không phải tách hết.
var vietnameseTones = map[rune]bool{
	0x0300: true, // huyền
	0x0301: true, // sắc
	0x0303: true, // ngã
	0x0309: true, // hỏi
	0x0323: true, // nặng
}

type escposDoc struct {
	buf   bytes.Buffer
	width int
	enc   string
	// plain giữ đúng phần chữ, không có byte điều khiển — dùng cho bản xem
	// trước trên giao diện. Ghi song song thay vì lọc lại từ buf, vì lọc chuỗi
	// ESC là việc dễ sai và sai thì bản xem trước không còn khớp giấy in.
	plain bytes.Buffer
}

func newESCPOSDoc(width int, encoding string) *escposDoc {
	if width <= 0 {
		width = 32
	}
	if encoding == "" {
		encoding = EncodingASCII
	}
	return &escposDoc{width: width, enc: encoding}
}

func (d *escposDoc) Bytes() []byte { return d.buf.Bytes() }

// PlainText trả về đúng những gì sẽ hiện trên giấy, không kèm lệnh máy in.
func (d *escposDoc) PlainText() string { return d.plain.String() }

// init đưa máy in về trạng thái đã biết. Không có lệnh này thì một hoá đơn in
// đậm mà quên tắt sẽ làm hoá đơn kế tiếp in đậm theo.
func (d *escposDoc) init(codePage int) {
	d.buf.Write([]byte{0x1B, 0x40}) // ESC @
	if codePage > 0 && codePage < 256 {
		d.buf.Write([]byte{0x1B, 0x74, byte(codePage)}) // ESC t n
	}
}

func (d *escposDoc) align(a byte) { d.buf.Write([]byte{0x1B, 0x61, a}) } // 0 trái 1 giữa 2 phải

func (d *escposDoc) bold(on bool) {
	v := byte(0)
	if on {
		v = 1
	}
	d.buf.Write([]byte{0x1B, 0x45, v})
}

// double bật cỡ chữ gấp đôi cả chiều cao lẫn chiều rộng.
func (d *escposDoc) double(on bool) {
	v := byte(0x00)
	if on {
		v = 0x11
	}
	d.buf.Write([]byte{0x1D, 0x21, v})
}

func (d *escposDoc) text(s string) { d.write(s) }

func (d *escposDoc) line(s string) {
	d.write(s)
	d.newline()
}

// write là cửa duy nhất chữ đi ra: mọi chỗ khác phải gọi qua đây, để phần byte
// gửi máy in và phần xem trước không bao giờ lệch nhau.
func (d *escposDoc) write(s string) {
	d.buf.Write(d.encode(s))
	d.plain.WriteString(d.printable(s))
}

func (d *escposDoc) newline() {
	d.buf.WriteByte('\n')
	d.plain.WriteByte('\n')
}

func (d *escposDoc) feed(n int) {
	for i := 0; i < n; i++ {
		d.newline()
	}
}

func (d *escposDoc) separator() { d.line(strings.Repeat("-", d.width)) }

// twoCol xếp nhãn bên trái, giá trị bên phải, căn đúng bề rộng giấy. Nhãn quá
// dài bị cắt chứ không cho tràn dòng — một dòng tiền bị xuống dòng giữa chừng
// đọc còn khó hơn là bị cắt.
func (d *escposDoc) twoCol(left, right string) {
	space := d.width - d.cols(right)
	if space < 1 {
		d.line(left)
		d.line(right)
		return
	}
	if d.cols(left) > space-1 {
		left = truncCols(left, space-1)
	}
	d.line(left + strings.Repeat(" ", space-d.cols(left)) + right)
}

// wrap ngắt dòng theo bề rộng giấy, ưu tiên ngắt ở khoảng trắng.
func (d *escposDoc) wrap(s string, indent int) {
	avail := d.width - indent
	if avail < 8 {
		avail = d.width
		indent = 0
	}
	pad := strings.Repeat(" ", indent)
	for _, chunk := range wrapCols(s, avail) {
		d.line(pad + chunk)
	}
}

// cut đẩy giấy rồi cắt. Đẩy trước là bắt buộc: dao cắt nằm cách đầu in vài
// milimet, không đẩy thì cắt mất mấy dòng cuối. Giấy trắng và lệnh cắt không
// vào bản xem trước.
func (d *escposDoc) cut() {
	for i := 0; i < 4; i++ {
		d.buf.WriteByte('\n')
	}
	d.buf.Write([]byte{0x1D, 0x56, 0x00})
}

// openDrawer đá ngăn kéo đựng tiền (chân 2, xung 100ms).
func (d *escposDoc) openDrawer() {
	d.buf.Write([]byte{0x1B, 0x70, 0x00, 0x32, 0x32})
}

// --- mã hoá và đo bề rộng --------------------------------------------------
//
// Ba khái niệm khác nhau, trước đây bị gộp làm một và đó chính là lỗi: chế độ
// cp1258 bỏ dấu để căn cột rồi mã hoá chuỗi ĐÃ bỏ dấu, nên máy in có bảng mã
// tiếng Việt vẫn nhận được chữ không dấu.
//
//   cols(s)      — chữ chiếm bao nhiêu cột trên giấy (dùng để căn)
//   printable(s) — chữ sẽ hiện trên giấy (dùng cho bản xem trước)
//   encode(s)    — byte thật gửi xuống máy in

// cols đếm theo bản bỏ dấu: mỗi chữ cái tiếng Việt in ra đúng một ô, dù trên
// đường truyền cp1258 nó có thể tốn hai byte.
func (d *escposDoc) cols(s string) int {
	return len([]rune(foldVietnamese(s)))
}

func (d *escposDoc) printable(s string) string {
	if d.enc == EncodingCP1258 {
		return s
	}
	return foldVietnamese(s)
}

func (d *escposDoc) encode(s string) []byte {
	if d.enc != EncodingCP1258 {
		// Bỏ dấu ở đây, không sớm hơn: đây là cửa cuối trước khi byte rời máy
		// chủ, và bỏ dấu sớm sẽ làm chế độ cp1258 mất dấu.
		return []byte(foldVietnamese(s))
	}
	out, err := charmap.Windows1258.NewEncoder().Bytes([]byte(splitVietnameseTones(s)))
	if err != nil {
		// Ký tự nào không mã hoá được thì rơi về bỏ dấu, thay vì in ra rác
		// hoặc bỏ trắng cả dòng.
		return []byte(foldVietnamese(s))
	}
	return out
}

// truncCols cắt chuỗi còn đúng n cột, tính theo ký tự chứ không theo byte.
func truncCols(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// splitVietnameseTones tách dấu thanh khỏi chữ, giữ nguyên â/ê/ô/ă/ơ/ư — đúng
// cách CP1258 biểu diễn tiếng Việt.
func splitVietnameseTones(s string) string {
	var b strings.Builder
	for _, r := range s {
		var base []rune
		var tone rune
		for _, c := range norm.NFD.String(string(r)) {
			if vietnameseTones[c] {
				tone = c
			} else {
				base = append(base, c)
			}
		}
		b.WriteString(norm.NFC.String(string(base)))
		if tone != 0 {
			b.WriteRune(tone)
		}
	}
	return b.String()
}

// foldVietnamese bỏ toàn bộ dấu và đổi đ/Đ thành d/D.
func foldVietnamese(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		switch {
		case unicode.Is(unicode.Mn, r): // dấu tổ hợp
			continue
		case r == 'đ':
			b.WriteRune('d')
		case r == 'Đ':
			b.WriteRune('D')
		case r > 127:
			// Ký tự ngoài ASCII mà không phải tiếng Việt (ví dụ tiếng Trung):
			// thay bằng '?' để không đẩy byte lạ xuống máy in.
			b.WriteRune('?')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// wrapCols ngắt dòng theo SỐ CỘT (ký tự), không phải số byte — chữ tiếng Việt
// có dấu chiếm nhiều byte nhưng vẫn chỉ một ô trên giấy.
func wrapCols(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	var out []string
	r := []rune(s)
	for len(r) > width {
		cut := -1
		for i := width; i > 0; i-- {
			if r[i] == ' ' {
				cut = i
				break
			}
		}
		if cut <= 0 {
			cut = width
		}
		out = append(out, strings.TrimRight(string(r[:cut]), " "))
		r = []rune(strings.TrimLeft(string(r[cut:]), " "))
	}
	if len(r) > 0 || len(out) == 0 {
		out = append(out, string(r))
	}
	return out
}

// formatMoney in tiền kiểu Việt Nam: 1.234.567
func formatMoney(v int64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	s := fmt.Sprintf("%d", v)
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	out := strings.Join(parts, ".")
	if neg {
		return "-" + out
	}
	return out
}
