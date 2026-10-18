package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Thông báo từ quầy (lệnh remote:message) hiện trong cửa sổ phụ chế độ "notice".
//
// Bản cũ gọi MessageBoxW: hộp xám mặc định của Windows, không đổi được giao diện. Thanh
// chính vẫn là nơi nhận lệnh (cửa sổ phụ không đăng ký handler điều khiển, xem
// connectWS), nhưng thay vì vẽ gì nó gõ cửa cửa sổ phụ kèm nội dung — cùng cơ chế
// SingleInstanceLock mà cửa sổ Hỗ trợ dùng để hiện lên khi có tin nhắn tới.
//
// Nội dung đi qua tham số dòng lệnh (--notice=<base64>) chứ không qua kênh nào khác vì
// lần gõ cửa chỉ mang được đúng thứ đó; hơn nữa tiến trình chưa chạy thì lần khởi động
// đầu cũng đọc được y như vậy. Đây là chữ quầy gõ cho khách, không phải bí mật.
// Không dựng được cửa sổ thì rơi về hộp Windows cũ (hienThongBao): thông báo của quầy
// tới trễ còn hơn không tới.

const (
	noticeMode = "notice"

	// Kích thước cửa sổ thông báo: nhỏ như một hộp thoại, không phải cửa sổ 70% màn hình
	// của các màn phụ khác.
	noticeW = 520
	noticeH = 320

	noticeFlag = "--notice="

	// Cỡ tối thiểu của cửa sổ phụ (đặt ở runChildWindow).
	panelMinW = 640
	panelMinH = 480

	// Dòng lệnh Windows tối đa 32767 ký tự; base64 của 2000 chữ tiếng Việt (tới 3 byte
	// mỗi chữ) vẫn dưới 10.000.
	noticeTitleMax = 80
	noticeBodyMax  = 2000
	// Quầy gửi dồn quá nhiều thì giữ những cái mới nhất.
	noticeQueueMax = 20
)

// Notice là một thông báo quầy gửi cho máy này.
type Notice struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	// At: mili giây Unix lúc thanh chính nhận lệnh. Cũng là khoá của thông báo: tăng
	// dần, nên DismissNotices xoá được đúng những cái khách đã đọc.
	At int64 `json:"at"`
}

// cutRunes cắt theo ký tự (không theo byte, kẻo chẻ đôi một chữ có dấu).
func cutRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}

func cleanNotice(n Notice) Notice {
	n.Title = cutRunes(strings.TrimSpace(n.Title), noticeTitleMax)
	n.Body = cutRunes(strings.TrimSpace(n.Body), noticeBodyMax)
	return n
}

// encodeNoticeArg dựng tham số --notice=... cho lần gõ cửa.
func encodeNoticeArg(n Notice) string {
	b, _ := json.Marshal(cleanNotice(n))
	return noticeFlag + base64.RawURLEncoding.EncodeToString(b)
}

// decodeNotice đọc lại phần sau dấu "=" của tham số --notice.
func decodeNotice(v string) (Notice, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil {
		return Notice{}, false
	}
	var n Notice
	if json.Unmarshal(raw, &n) != nil {
		return Notice{}, false
	}
	n = cleanNotice(n)
	return n, n.Body != ""
}

// noticeArgOf lấy giá trị của --notice trong dòng lệnh của lần gõ cửa.
func noticeArgOf(args []string) string {
	for i, a := range args {
		if a == "--notice" && i+1 < len(args) {
			return args[i+1]
		}
		if v, ok := strings.CutPrefix(a, noticeFlag); ok {
			return v
		}
	}
	return ""
}

// addNotice xếp một thông báo vào hàng chờ khách bấm "Đã hiểu".
func (a *App) addNotice(n Notice) {
	a.noticeMu.Lock()
	defer a.noticeMu.Unlock()
	if last := len(a.notices); last > 0 && n.At <= a.notices[last-1].At {
		n.At = a.notices[last-1].At + 1
	}
	a.notices = append(a.notices, n)
	if over := len(a.notices) - noticeQueueMax; over > 0 {
		a.notices = a.notices[over:]
	}
}

func (a *App) clearNotices() {
	a.noticeMu.Lock()
	a.notices = nil
	a.noticeMu.Unlock()
}

// noticesJSON: hàng chờ dạng JSON, mới nhất trước. Luôn là mảng, không bao giờ "null".
func (a *App) noticesJSON() string {
	a.noticeMu.Lock()
	defer a.noticeMu.Unlock()
	out := make([]Notice, 0, len(a.notices))
	for i := len(a.notices) - 1; i >= 0; i-- {
		out = append(out, a.notices[i])
	}
	b, _ := json.Marshal(out)
	return string(b)
}

// GetNotices cho màn hình thông báo biết đang có gì chờ khách đọc.
func (a *App) GetNotices() string {
	return a.noticesJSON()
}

// DismissNotices: khách bấm "Đã hiểu". Chỉ xoá những thông báo từ mốc upTo trở về trước —
// cái tới giữa lúc khách đang đọc (có At lớn hơn) không được mất. Còn gì thì trả lại để
// màn hình vẽ tiếp; hết thì ẩn cửa sổ.
func (a *App) DismissNotices(upTo int64) string {
	a.noticeMu.Lock()
	keep := a.notices[:0:0]
	for _, n := range a.notices {
		if n.At > upTo {
			keep = append(keep, n)
		}
	}
	a.notices = keep
	a.noticeMu.Unlock()

	// Cửa sổ đã đổi sang màn khác (tin nhắn tới đúng lúc khách bấm) thì để yên, đừng ẩn nhầm.
	if len(keep) == 0 && a.panelModeNow() == noticeMode {
		a.HidePanel()
	}
	return a.noticesJSON()
}

// panelModeNow đọc màn hình đang hiện của cửa sổ phụ.
func (a *App) panelModeNow() string {
	a.panelMu.Lock()
	defer a.panelMu.Unlock()
	return a.panelMode
}

// fixedGeometry căn một cửa sổ kích thước cố định giữa phần màn hình bên trái thanh dọc
// (cùng quy ước với childGeometry), thu lại nếu màn hình nhỏ hơn nó.
func fixedGeometry(screenW, screenH, w, h int) (outW, outH, x, y int) {
	area := screenW
	if screenW-dockWidth >= w {
		area = screenW - dockWidth
	}
	if w > screenW {
		w = screenW
	}
	if h > screenH {
		h = screenH
	}
	return w, h, (area - w) / 2, (screenH - h) / 2
}

// canhGiuaCoDinh đặt cửa sổ phụ về đúng w×h giữa màn hình. Cỡ tối thiểu của cửa sổ phụ
// (640×480) lớn hơn hộp thông báo nên phải hạ nó xuống trước, nếu không Wails kẹp lại.
func canhGiuaCoDinh(ctx context.Context, w, h int) {
	screen, ok := manHinhHienTai(ctx)
	if !ok {
		return
	}
	w, h, x, y := fixedGeometry(screen.Size.Width, screen.Size.Height, w, h)
	wailsruntime.WindowSetMinSize(ctx, w, h)
	wailsruntime.WindowSetSize(ctx, w, h)
	wailsruntime.WindowSetPosition(ctx, x, y)
}

// canhGiuaPanel căn cửa sổ phụ cho màn hình mode: hộp thông báo nhỏ cố định, các màn khác
// theo tỉ lệ màn hình như trước.
func (a *App) canhGiuaPanel(ctx context.Context, mode string) {
	if mode == noticeMode {
		canhGiuaCoDinh(ctx, noticeW, noticeH)
		return
	}
	// Vừa là hộp thông báo thì cỡ tối thiểu đã bị hạ; trả về như lúc dựng cửa sổ.
	wailsruntime.WindowSetMinSize(ctx, panelMinW, panelMinH)
	canhGiuaManHinh(ctx, panelMinW, panelMinH)
}

// hienThongBao hiện thông báo quầy gửi cho khách. Chạy ở thanh chính.
//
// Chưa đăng nhập, hoặc màn khoá native đang giữ máy, thì cửa sổ phụ không có chỗ để hiện
// (nó bị đóng khi hết phiên, và màn khoá phủ kín): dùng hộp Windows, thứ luôn nổi lên
// được. Dựng cửa sổ phụ lỗi cũng vậy.
func (a *App) hienThongBao(title, body string) {
	if a.loggedIn() && !nativeLockOwns() {
		n := Notice{Title: title, Body: body, At: time.Now().UnixMilli()}
		err := a.spawnWindow(noticeMode, encodeNoticeArg(n))
		if err == nil {
			return
		}
		log.Printf("[notice] không mở được cửa sổ thông báo, dùng hộp Windows: %v", err)
	}
	a.locker.ShowMessage(title, body)
}
