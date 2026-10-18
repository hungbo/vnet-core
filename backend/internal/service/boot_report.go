package service

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// BootReport là các mốc thời gian của LẦN KHỞI ĐỘNG hiện tại của máy trạm, đi
// kèm nhịp tim. Phòng máy không ổ cứng (diskless) cần đo tốc độ khởi động
// (kernel → màn khoá → giao diện web sẵn sàng); trước đây người vận hành phải
// mở khoá máy rồi đọc ui.log/service.log bằng tay, giờ chỉ cần đọc log máy chủ.
//
// Mọi trường *_s là GIÂY UPTIME của Windows (tính từ lúc kernel khởi động), độ
// chính xác 0,1 giây. Bỏ trống = máy trạm chưa biết. Máy trạm gửi lại cả gói ở
// mỗi nhịp tim, các trường điền dần theo tiến trình khởi động.
//
// Kiểu số đều là float và mốc khởi động là chuỗi, và UnmarshalJSON nuốt lỗi kiểu:
// một giá trị sai kiểu không được làm hỏng việc giải mã CẢ nhịp tim (400), máy bị
// tính là mất tín hiệu chỉ vì một con số đo đạc.
type BootReport struct {
	// Mốc kernel khởi động, UTC, RFC3339 (= giờ máy trạm − uptime).
	BootAt        string   `json:"boot_at,omitempty"`
	ClientVersion string   `json:"client_version,omitempty"`
	ServiceStartS *float64 `json:"service_start_s,omitempty"`
	// Đọc trước ổ đĩa lúc khởi động; bỏ trống cả ba khi bước này bị bỏ qua.
	PrefetchDoneS *float64 `json:"prefetch_done_s,omitempty"`
	PrefetchMB    *float64 `json:"prefetch_mb,omitempty"`
	PrefetchMS    *float64 `json:"prefetch_ms,omitempty"`
	// Lúc dịch vụ nền lấy được phiên người dùng Windows lần đầu.
	LogonS    *float64 `json:"logon_s,omitempty"`
	GUIStartS *float64 `json:"gui_start_s,omitempty"`
	// Lúc màn khoá ĐẦU TIÊN hiện ra; lock_kind là loại của nó: "native" hoặc "web".
	LockShownS    *float64 `json:"lock_shown_s,omitempty"`
	LockKind      string   `json:"lock_kind,omitempty"`
	WebviewReadyS *float64 `json:"webview_ready_s,omitempty"`
	DOMReadyS     *float64 `json:"dom_ready_s,omitempty"`
	// Lần mở khoá đầu tiên của lần khởi động này.
	UnlockS *float64 `json:"unlock_s,omitempty"`
	// Chuỗi thời gian nạp trang do giao diện tự đo, tối đa 200 ký tự.
	Page string `json:"page,omitempty"`
}

const (
	bootGiayToiDa = 86400 // mốc nào quá một ngày uptime thì không còn là "lúc khởi động"
	bootMBToiDa   = 1 << 20
	bootMSToiDa   = 86400 * 1000
	// boot_at cũ hơn thế là rác (đồng hồ máy trạm sai) chứ không phải lần khởi động này.
	bootAtCuNhat = 7 * 24 * time.Hour
	// Đồng hồ máy trạm chạy nhanh hơn máy chủ vài giây là chuyện thường; lúc mới
	// khởi động uptime chỉ vài chục giây, nên boot_at có thể rơi nhẹ vào tương lai.
	bootAtLechToiDa = 10 * time.Minute
	// Số máy tối đa giữ dòng [boot] gần nhất. Mã máy tự khai được qua route mở (máy
	// lạ tự đăng ký), xoá máy không xoá dòng của nó: đầy thì xoá trắng, giá chỉ là
	// mỗi máy ghi lại một dòng.
	bootMayToiDa = 4096
)

// Phiên bản do bản build đặt (1.0.12, dev); thứ khác thì bỏ, vì nó đứng trần trong
// dòng log. Mã máy ngoài bộ ký tự này thì in trong ngoặc kép (maMayTrongLog).
var (
	bootPhienBanHopLe = regexp.MustCompile(`^[0-9A-Za-z._+-]{1,32}$`)
	bootMaMayAnToan   = regexp.MustCompile(`^[0-9A-Za-z._-]+$`)
)

// UnmarshalJSON không bao giờ trả lỗi: boot_report sai kiểu (cả gói hay một trường)
// thì bỏ CẢ gói, nhịp tim vẫn nhận. Bỏ cả gói chứ không giữ phần đúng: trường sai
// kiểu được encoding/json cấp con trỏ trước khi báo lỗi, giữ lại là in ra số 0 giả.
// Cú pháp của cả thân JSON đã được kiểm trước khi tới đây, nên ở đây chỉ có lỗi kiểu.
func (r *BootReport) UnmarshalJSON(b []byte) error {
	type tho BootReport
	var t tho
	if json.Unmarshal(b, &t) == nil {
		*r = BootReport(t)
	}
	return nil
}

// boChuoiDieuKhien bỏ ký tự điều khiển (kể cả \r \n) và ký tự định dạng vô
// hình: route nhịp tim mở cho cả mạng LAN, chuỗi từ máy trạm không được phép
// chèn thêm dòng giả vào log máy chủ.
func boChuoiDieuKhien(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) {
			return -1
		}
		return r
	}, s)
}

func bootChuoi(s string, max int) string {
	// Cắt theo byte trước (một ký tự UTF-8 dài tối đa 4 byte): chuỗi vài MB từ LAN
	// không phải đi hết qua strings.Map và []rune chỉ để giữ max ký tự.
	if n := 4 * max; len(s) > n {
		for n > 0 && !utf8.RuneStart(s[n]) {
			n--
		}
		s = s[:n]
	}
	return truncateRunes(strings.TrimSpace(boChuoiDieuKhien(s)), max)
}

func bootGiay(p *float64) *float64 {
	if p == nil || math.IsNaN(*p) || *p < 0 || *p > bootGiayToiDa {
		return nil
	}
	v := math.Round(*p*10) / 10
	return &v
}

func bootSoNguyen(p *float64, max float64) *float64 {
	if p == nil || math.IsNaN(*p) || *p < 0 || *p > max {
		return nil
	}
	v := math.Round(*p)
	return &v
}

// lamSach trả về bản đã kiểm tra: giá trị ngoài khoảng hợp lệ bị BỎ (coi như
// chưa biết), không ép về biên — một con số ép biên trông như số đo thật.
// Trả nil khi không còn gì để ghi.
func (r *BootReport) lamSach(now time.Time) *BootReport {
	if r == nil {
		return nil
	}
	out := BootReport{
		ServiceStartS: bootGiay(r.ServiceStartS),
		PrefetchDoneS: bootGiay(r.PrefetchDoneS),
		PrefetchMB:    bootSoNguyen(r.PrefetchMB, bootMBToiDa),
		PrefetchMS:    bootSoNguyen(r.PrefetchMS, bootMSToiDa),
		LogonS:        bootGiay(r.LogonS),
		GUIStartS:     bootGiay(r.GUIStartS),
		LockShownS:    bootGiay(r.LockShownS),
		WebviewReadyS: bootGiay(r.WebviewReadyS),
		DOMReadyS:     bootGiay(r.DOMReadyS),
		UnlockS:       bootGiay(r.UnlockS),
		Page:          bootChuoi(r.Page, 200),
	}
	if v := strings.TrimSpace(r.ClientVersion); bootPhienBanHopLe.MatchString(v) {
		out.ClientVersion = v
	}
	if k := strings.TrimSpace(r.LockKind); k == "native" || k == "web" {
		out.LockKind = k
	}
	if t, err := time.Parse(time.RFC3339, strings.TrimSpace(r.BootAt)); err == nil && !t.After(now.Add(bootAtLechToiDa)) {
		// Máy chạy quá bảy ngày vẫn gửi đúng boot_at đó: bỏ cả gói, đừng ghi thêm một
		// dòng boot=- cho một máy không hề khởi động lại.
		if t.Before(now.Add(-bootAtCuNhat)) {
			return nil
		}
		out.BootAt = t.UTC().Truncate(time.Second).Format(time.RFC3339)
	}
	if out == (BootReport{}) {
		return nil
	}
	return &out
}

func bootSo(p *float64, format string) string {
	if p == nil {
		return "-"
	}
	return fmt.Sprintf(format, *p)
}

func bootHoacGach(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// maMayTrongLog: mã máy đến từ URL (máy lạ tự đăng ký). Chỉ in trần khi nó gồm ký tự
// an toàn; còn lại in trong ngoặc kép có thoát ký tự, để dấu cách, "[" hay "=" không
// dựng được một đoạn giả của máy khác trong cùng dòng (grep "[boot] MAY2" khớp nhầm).
func maMayTrongLog(maMay string) string {
	if bootMaMayAnToan.MatchString(maMay) {
		return maMay
	}
	return strconv.Quote(maMay)
}

// dongLog dựng đúng một dòng [boot]; "-" cho giá trị chưa biết. Gọi trên bản đã lamSach.
func (r *BootReport) dongLog(maMay string) string {
	version := "-"
	if r.ClientVersion != "" {
		version = "v" + r.ClientVersion
	}
	prefetch := "-"
	if r.PrefetchDoneS != nil || r.PrefetchMB != nil || r.PrefetchMS != nil {
		prefetch = fmt.Sprintf("%s(%sMB/%sms)",
			bootSo(r.PrefetchDoneS, "%.1f"), bootSo(r.PrefetchMB, "%.0f"), bootSo(r.PrefetchMS, "%.0f"))
	}
	lock := "-"
	if r.LockShownS != nil || r.LockKind != "" {
		lock = fmt.Sprintf("%s(%s)", bootSo(r.LockShownS, "%.1f"), bootHoacGach(r.LockKind))
	}
	page := "-"
	if r.Page != "" {
		page = strconv.Quote(r.Page)
	}
	return fmt.Sprintf("[boot] %s %s boot=%s svc=%s prefetch=%s logon=%s gui=%s lock=%s webview=%s dom=%s unlock=%s page=%s",
		maMayTrongLog(maMay), version, bootHoacGach(r.BootAt),
		bootSo(r.ServiceStartS, "%.1f"), prefetch, bootSo(r.LogonS, "%.1f"), bootSo(r.GUIStartS, "%.1f"),
		lock, bootSo(r.WebviewReadyS, "%.1f"), bootSo(r.DOMReadyS, "%.1f"), bootSo(r.UnlockS, "%.1f"), page)
}

// ghiBootReport ghi dòng [boot] CHỈ khi nó khác dòng đã ghi lần trước cho máy
// này. Máy trạm gửi lại cả gói mỗi 15 giây; không lọc thì log máy chủ là 240
// dòng một giờ cho mỗi máy, giống hệt nhau. Bản đồ chỉ giữ một dòng cho mỗi mã
// máy nên lớn tối đa bằng số máy; nằm trong bộ nhớ — khởi động lại máy chủ thì
// mỗi máy ghi lại một dòng, vô hại.
func (s *MachineService) ghiBootReport(maMay string, r *BootReport) {
	r = r.lamSach(time.Now())
	if r == nil {
		return
	}
	dong := r.dongLog(maMay)
	s.bootMu.Lock()
	if s.bootDaGhi == nil {
		s.bootDaGhi = make(map[string]string)
	}
	moi := s.bootDaGhi[maMay] != dong
	if moi {
		if len(s.bootDaGhi) >= bootMayToiDa {
			clear(s.bootDaGhi)
		}
		s.bootDaGhi[maMay] = dong
	}
	s.bootMu.Unlock()
	if moi {
		log.Print(dong)
	}
}
