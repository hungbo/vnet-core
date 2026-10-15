package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Báo cáo khởi động: các mốc của một lần bật máy, gửi kèm nhịp tim để máy chủ ghi một
// dòng [boot] cho mỗi máy mỗi lần bật. Người đo tốc độ khởi động ở phòng máy không ổ
// cứng (nhân → màn hình khoá → giao diện web dùng được) đọc được từ xa, không phải mở
// khoá từng máy rồi đọc ui.log/service.log.
//
// Mỗi mốc là số giây máy đã chạy (systemUptime) làm tròn 0,1 s, ghi ở đúng chỗ đã ghi
// nhật ký thời gian tương ứng; 0 là chưa biết và không có trong JSON. Mọi trường GHI
// MỘT LẦN cho cả lần bật máy: giao diện bị bật lại giữa buổi không được đè mốc của lần
// đầu, dịch vụ nền khởi động lại cũng không (dichVuBatDau).
//
// Hai tiến trình cùng ghi. Giao diện ghi phần bootGUI và gửi nó cho dịch vụ nền trong
// báo cáo hai giây một lần (uilink_windows.go); dịch vụ ghi phần của nó, gộp phần của
// giao diện vào (gopGiaoDien) rồi gửi cả báo cáo kèm nhịp tim (agent.go).

// bootPageMax: chuỗi mốc nạp trang dài nhất được gửi, tính theo ký tự.
const bootPageMax = 200

// bootAtLech: hai boot_at lệch nhau trong khoảng này là cùng một lần bật máy. boot_at =
// giờ hệ thống − uptime; giờ hệ thống được chỉnh theo máy chủ thời gian còn uptime thì
// không, nên hai lần tính trôi nhau vài giây mỗi ngày, cắt tới giây lệch thêm một giây.
// Hai lần bật máy khác nhau cách nhau cả thời gian lần trước đã chạy cộng thời gian
// khởi động lại — hơn hẳn khoảng này.
const bootAtLech = 30 * time.Second

// bootGUI là phần mốc do giao diện ghi.
type bootGUI struct {
	GUIStartS     float64 `json:"gui_start_s,omitempty"`     // tiến trình giao diện bắt đầu
	LockShownS    float64 `json:"lock_shown_s,omitempty"`    // màn hình khoá đầu tiên hiện ra
	LockKind      string  `json:"lock_kind,omitempty"`       // màn đó là "native" hay "web"
	WebviewReadyS float64 `json:"webview_ready_s,omitempty"` // Startup: WebView2 đã dựng
	DomReadyS     float64 `json:"dom_ready_s,omitempty"`
	UnlockS       float64 `json:"unlock_s,omitempty"` // lần mở khoá đầu tiên, đường nào cũng được
	Page          string  `json:"page,omitempty"`     // các mốc nạp trang (ghiMocNapTrang)
}

// bootReport là trường boot_report của nhịp tim; phần bootGUI nằm phẳng cùng cấp.
type bootReport struct {
	BootAt        string  `json:"boot_at,omitempty"` // lúc bật máy, UTC, tới giây
	ClientVersion string  `json:"client_version,omitempty"`
	ServiceStartS float64 `json:"service_start_s,omitempty"` // runAgent bắt đầu
	PrefetchDoneS float64 `json:"prefetch_done_s,omitempty"` // runBootPrefetch đọc xong; bỏ qua thì không có
	// Con trỏ: đọc trước không tới 1 MB hay 1 ms là số đo thật, vẫn phải gửi số 0.
	PrefetchMB *int64  `json:"prefetch_mb,omitempty"`
	PrefetchMS *int64  `json:"prefetch_ms,omitempty"`
	LogonS     float64 `json:"logon_s,omitempty"` // lần đầu mượn được token của phiên Windows
	bootGUI
}

type bootRecorder struct {
	mu  sync.Mutex
	r   bootReport
	tep string // dịch vụ nền: tệp giữ báo cáo qua lần dịch vụ khởi động lại; "" ở giao diện
	doi bool   // báo cáo đã đổi từ lần ghi tệp trước
	tat bool   // dịch vụ bật quá muộn mà không có báo cáo nào của lần bật máy này: không báo
}

// bootRec là báo cáo của tiến trình này. Ở giao diện chỉ phần bootGUI có số.
var bootRec bootRecorder

func (b *bootRecorder) ghi(f func(r *bootReport)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	truoc := b.r
	f(&b.r)
	if b.r != truoc {
		b.doi = true
	}
}

// giayMayChay: thời gian máy đã chạy ra giây, làm tròn 0,1 s.
func giayMayChay(up time.Duration) float64 { return math.Round(up.Seconds()*10) / 10 }

// tinhBootAt: lúc bật máy theo giờ hệ thống, UTC, tới giây.
func tinhBootAt(now time.Time, up time.Duration) time.Time {
	return now.Add(-up).UTC().Truncate(time.Second)
}

// cungLanBat: boot_at s (RFC3339) có thuộc lần bật máy tính ra at không.
func cungLanBat(s string, at time.Time) bool {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return false
	}
	d := t.Sub(at)
	return d <= bootAtLech && d >= -bootAtLech
}

// ganDau ghi v vào *dst nếu *dst chưa có. v không dương là không biết (systemUptime
// ngoài Windows luôn bằng 0).
func ganDau(dst *float64, v float64) {
	if *dst == 0 && v > 0 {
		*dst = v
	}
}

// ganKhoa ghi mốc màn hình khoá cùng loại của nó: cả hai, hoặc không gì.
func (g *bootGUI) ganKhoa(v float64, kind string) {
	if g.LockShownS == 0 && v > 0 {
		g.LockShownS, g.LockKind = v, kind
	}
}

func (g *bootGUI) ganTrang(s string) {
	if g.Page == "" {
		if r := []rune(s); len(r) > bootPageMax {
			s = string(r[:bootPageMax])
		}
		g.Page = s
	}
}

// Mốc của dịch vụ nền.

// bootFilePath: báo cáo của dịch vụ nền nằm cạnh .exe như policy.json — thư mục cài chỉ
// dịch vụ (SYSTEM) và quản trị ghi được, khách không sửa được mốc.
func bootFilePath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), "boot.json")
}

// dichVuBatDau: mốc dịch vụ nền bắt đầu, gọi một lần ở đầu runAgent.
//
// Dịch vụ khởi động lại giữa một lần bật máy (tự cập nhật thoát với mã 3 từ giây thứ 90,
// chế độ khôi phục của SCM) thì báo cáo trong bộ nhớ mất theo, và tiến trình mới ghi lại
// mọi mốc bằng giờ của NÓ: máy chủ nhận thêm một dòng [boot] trông như một lần khởi động
// chậm. Nên dịch vụ giữ báo cáo trong tệp (baoCao ghi) và nạp lại ở đây nếu tệp là của
// chính lần bật máy này. Máy không ổ cứng thì tệp mất ở lần bật sau; máy có ổ cứng thì
// tệp của lần trước có boot_at lệch xa, bỏ qua.
//
// Không có báo cáo nào của lần bật máy này mà máy đã chạy quá curtainMaxUptime (tệp hỏng,
// bản cũ chưa có báo cáo vừa tự cập nhật): những gì tiến trình này đo được không phải một
// lần khởi động, nên thôi báo cáo hẳn.
func (b *bootRecorder) dichVuBatDau(tep string, now time.Time, up time.Duration) {
	at := tinhBootAt(now, up)
	var cu bootReport
	coCu := false
	if data, err := os.ReadFile(tep); err == nil && json.Unmarshal(data, &cu) == nil {
		coCu = cungLanBat(cu.BootAt, at)
	}
	b.mu.Lock()
	b.tep = tep
	if coCu {
		b.r = cu
	} else if up >= curtainMaxUptime {
		b.tat = true
	}
	b.mu.Unlock()
	b.ghi(func(r *bootReport) {
		if r.BootAt == "" {
			r.BootAt = at.Format(time.RFC3339)
		}
		// Bản đã đo lần bật máy này, kể cả khi nó vừa tự cập nhật lên bản khác.
		if r.ClientVersion == "" {
			r.ClientVersion = version
		}
		ganDau(&r.ServiceStartS, giayMayChay(up))
	})
}

func (b *bootRecorder) docTruocXong(up time.Duration, mb, ms int64) {
	b.ghi(func(r *bootReport) {
		if v := giayMayChay(up); r.PrefetchDoneS == 0 && v > 0 {
			r.PrefetchDoneS, r.PrefetchMB, r.PrefetchMS = v, &mb, &ms
		}
	})
}

func (b *bootRecorder) coPhienWindows(up time.Duration) {
	b.ghi(func(r *bootReport) { ganDau(&r.LogonS, giayMayChay(up)) })
}

// Mốc của giao diện.

func (b *bootRecorder) giaoDienBatDau(up time.Duration) {
	b.ghi(func(r *bootReport) { ganDau(&r.GUIStartS, giayMayChay(up)) })
}

func (b *bootRecorder) khoaNativeHien(up time.Duration) {
	b.ghi(func(r *bootReport) { r.ganKhoa(giayMayChay(up), "native") })
}

func (b *bootRecorder) webViewSan(up time.Duration) {
	b.ghi(func(r *bootReport) { ganDau(&r.WebviewReadyS, giayMayChay(up)) })
}

// domSan: DOM sẵn sàng. Tới đây mà chưa có màn khoá native nào hiện (dựng hỏng, Startup
// đã thôi chờ nó, máy đang bảo trì) thì màn hình khoá đầu tiên là của web, và nó có nội
// dung từ lúc này.
func (b *bootRecorder) domSan(up time.Duration) {
	b.ghi(func(r *bootReport) {
		v := giayMayChay(up)
		ganDau(&r.DomReadyS, v)
		r.ganKhoa(v, "web")
	})
}

func (b *bootRecorder) moKhoa(up time.Duration) {
	b.ghi(func(r *bootReport) { ganDau(&r.UnlockS, giayMayChay(up)) })
}

func (b *bootRecorder) trang(s string) {
	b.ghi(func(r *bootReport) { r.ganTrang(s) })
}

// phanGiaoDien: phần bootGUI đã ghi, để gửi cho dịch vụ nền.
func (b *bootRecorder) phanGiaoDien() bootGUI {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.r.bootGUI
}

// gopGiaoDien: dịch vụ nền gộp phần giao diện gửi sang. Trường nào đã có thì giữ: giao
// diện bị bật lại giữa buổi gửi mốc của NÓ, không phải của lần bật máy.
func (b *bootRecorder) gopGiaoDien(g bootGUI) {
	b.ghi(func(r *bootReport) {
		d := &r.bootGUI
		ganDau(&d.GUIStartS, g.GUIStartS)
		// Loại màn khoá đi thẳng vào nhịp tim: chỉ nhận hai giá trị giao diện thật gửi.
		if g.LockKind == "native" || g.LockKind == "web" {
			d.ganKhoa(g.LockShownS, g.LockKind)
		}
		ganDau(&d.WebviewReadyS, g.WebviewReadyS)
		ganDau(&d.DomReadyS, g.DomReadyS)
		ganDau(&d.UnlockS, g.UnlockS)
		d.ganTrang(g.Page)
	})
}

// baoCao là boot_report gửi kèm nhịp tim; nil là chưa gửi. Chỉ vòng nhịp tim của dịch
// vụ nền gọi, nên tệp của dichVuBatDau được ghi ở đây, khi báo cáo đổi: ngoài khoá, và
// không bao giờ trên goroutine của kênh giao diện.
//
// Máy chủ ghi một dòng mỗi lần báo cáo của một máy KHÁC lần trước, nên gửi từng mảnh
// theo các mốc lần lượt tới là năm sáu dòng cho một lần bật máy. Chờ trang nạp xong
// (mốc cuối trước lần mở khoá), hoặc máy đã chạy quá curtainMaxUptime: giao diện không
// lên được thì những mốc đã có càng cần thấy. Lần mở khoá tới sau thêm một dòng nữa.
//
// boot_at tính một lần rồi giữ, chỉ tính lại khi lệch quá bootAtLech — đồng hồ máy vừa
// bị chỉnh hẳn. Tính lại theo từng giây lệch thì mỗi lần lệch là thêm một dòng.
func (b *bootRecorder) baoCao(now time.Time, up time.Duration) *bootReport {
	at := tinhBootAt(now, up)
	b.mu.Lock()
	if b.tat {
		b.mu.Unlock()
		return nil
	}
	if !cungLanBat(b.r.BootAt, at) {
		b.r.BootAt = at.Format(time.RFC3339)
		b.doi = true
	}
	r, tep := b.r, b.tep
	luu := b.doi && tep != ""
	if luu {
		b.doi = false
	}
	b.mu.Unlock()

	if luu {
		data, _ := json.Marshal(r)
		if os.WriteFile(tep, data, 0o644) != nil {
			// Ghi lại ở nhịp tim sau.
			b.mu.Lock()
			b.doi = true
			b.mu.Unlock()
		}
	}
	if r.Page == "" && up < curtainMaxUptime {
		return nil
	}
	return &r
}
