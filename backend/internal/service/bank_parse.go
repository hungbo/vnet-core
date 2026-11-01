package service

import (
	"regexp"
	"strconv"
	"strings"
)

// Đọc thông báo biến động số dư mà điện thoại ở quầy gửi về.
//
// MB (com.mbmobile), mẫu thật 31/10/2026, title "Thông báo biến động số dư":
//
//	TK 80xxx868|GD: +5,000VND 31/10/26 17:41 |SD: 1,999,339VND|TU: LIENG IN HUNG - 6522392389|ND: Qacziz5650  APPMI9041643 1  LIENG IN HUNG Chuyen tien- Ma GD ACSP/ 60605154
//
// Vietcombank chưa có mẫu thật: đi bộ đọc chung (dấu +/- trước số tiền VND).
// Đọc sai thì tiền rơi vào "chưa khớp" để nhân viên xử lý, không bao giờ cộng
// nhầm: muốn cộng phải khớp cả mã lẫn số tiền của một yêu cầu đang chờ.

type bankNotif struct {
	Bank          string
	Amount        int64
	Incoming      bool
	Parsed        bool // đọc được số tiền
	Content       string
	BankRef       string
	SenderName    string
	SenderAccount string
}

var (
	reSignedAmount = regexp.MustCompile(`([+-])\s*([\d.,]+)\s*(?:VND|VNĐ|đ)`)
	reMBRef        = regexp.MustCompile(`(?i)Ma GD\s*([A-Z]+/?\s*\d+)`)
	reContentTag   = regexp.MustCompile(`(?i)(?:\bND|Nội dung|Noi dung)\s*:\s*`)
	// Mã nạp: VN + 6 ký tự, bỏ 0/O, 1/I/L cho khỏi đọc nhầm (xem taoMaNap).
	reQRCode     = regexp.MustCompile(`VN[2-9A-HJKMNP-Z]{6}`)
	reSeparators = regexp.MustCompile(`[\s.\-_/]`)
)

func bankFromPackage(pkg string) string {
	for code, b := range qrBanks {
		if b.Package == pkg {
			return code
		}
	}
	return ""
}

func parseVNDAmount(s string) (int64, bool) {
	digits := strings.NewReplacer(",", "", ".", "").Replace(s)
	n, err := strconv.ParseInt(digits, 10, 64)
	return n, err == nil
}

// parseBankNotification đọc số tiền, chiều tiền, nội dung chuyển khoản và người
// chuyển từ một thông báo.
func parseBankNotification(pkg, title, text string) bankNotif {
	out := bankNotif{Bank: bankFromPackage(pkg)}
	if out.Bank == "mb" && strings.Contains(text, "|") {
		parseMB(text, &out)
	}
	if !out.Parsed {
		parseGeneric(text, &out)
	}
	return out
}

func parseMB(text string, out *bankNotif) {
	// ND đặt cuối và có thể chứa bất cứ gì: lấy hết phần sau "|ND:" trước khi tách.
	head := text
	if i := strings.Index(text, "|ND:"); i >= 0 {
		out.Content = strings.TrimSpace(text[i+len("|ND:"):])
		head = text[:i]
	}
	for _, field := range strings.Split(head, "|") {
		field = strings.TrimSpace(field)
		switch {
		case strings.HasPrefix(field, "GD:"):
			if m := reSignedAmount.FindStringSubmatch(field); m != nil {
				if n, ok := parseVNDAmount(m[2]); ok {
					out.Amount, out.Incoming, out.Parsed = n, m[1] == "+", true
				}
			}
		case strings.HasPrefix(field, "TU:"):
			who := strings.TrimSpace(strings.TrimPrefix(field, "TU:"))
			if i := strings.LastIndex(who, " - "); i >= 0 {
				out.SenderName, out.SenderAccount = strings.TrimSpace(who[:i]), strings.TrimSpace(who[i+3:])
			} else {
				out.SenderName = who
			}
		}
	}
	if m := reMBRef.FindStringSubmatch(out.Content); m != nil {
		out.BankRef = strings.ReplaceAll(m[1], " ", "")
	}
}

func parseGeneric(text string, out *bankNotif) {
	if m := reSignedAmount.FindStringSubmatch(text); m != nil {
		if n, ok := parseVNDAmount(m[2]); ok {
			out.Amount, out.Incoming, out.Parsed = n, m[1] == "+", true
		}
	}
	if out.Content == "" {
		if loc := reContentTag.FindStringIndex(text); loc != nil {
			out.Content = strings.TrimSpace(text[loc[1]:])
		} else {
			out.Content = text
		}
	}
}

// qrCodesIn trả về mọi chuỗi có dạng mã nạp trong nội dung, cả khi ngân hàng
// người gửi chèn dấu cách hay dấu chấm vào giữa. Có thể lẫn chuỗi không phải
// mã (vd. đuôi tên người): nơi gọi chỉ nhận chuỗi trùng một yêu cầu đang chờ.
func qrCodesIn(content string) []string {
	upper := strings.ToUpper(content)
	seen := map[string]bool{}
	var out []string
	for _, s := range []string{upper, reSeparators.ReplaceAllString(upper, "")} {
		for _, c := range reQRCode.FindAllString(s, -1) {
			if !seen[c] {
				seen[c] = true
				out = append(out, c)
			}
		}
	}
	return out
}
