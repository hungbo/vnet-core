package main

import (
	"errors"
	"log"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Điều phối đổi độ phân giải và chuột: nhớ mốc ban đầu, hẹn giờ tự hoàn tác, trả
// mọi thứ về mốc khi máy khoá hoặc khách đăng xuất. Phần thuần và lý do ở display.go,
// phần Win32 ở display_windows.go.

// displayCtl giữ MỘT khoá cho mọi thao tác: đổi chế độ, đọc, hoàn tác, và cả lúc đồng
// hồ tự hoàn tác báo hết giờ. Các thao tác này đều vài trăm mili giây trở lên
// (ChangeDisplaySettingsEx làm tối màn hình), nên xếp hàng là đúng; chạy chồng lên
// nhau mới là lỗi — xem revertGuard về chuyện đồng hồ hết giờ đúng lúc khách bấm đổi tiếp.
type displayCtl struct {
	mu    sync.Mutex
	hw    displayHW
	after func(d time.Duration, f func()) (stop func() bool)
	guard revertGuard
	base  *displayBaseline
	// retries: số lần đã thử lại việc về chế độ cũ sau khi nó hỏng; đặt về 0 mỗi lần đổi.
	retries int
}

// displayBaseline là mốc phải trả về: cài đặt của máy trước khi khách nào động tới.
// Nhớ lúc tiến trình giao diện KHỞI ĐỘNG (startup), chứ không phải lúc có người mở
// trang Cài đặt: lúc đó một game đang chạy có thể đã đổi độ phân giải, và cái chế độ
// tạm của game sẽ thành "mặc định" cho mọi khách sau.
type displayBaseline struct {
	mode      DisplayMode
	speed     int
	precision bool
}

type displaySnapshot struct {
	modes     []DisplayMode // đã lọc
	current   DisplayMode
	speed     int
	precision bool
}

func newDisplayCtl(hw displayHW, after func(time.Duration, func()) func() bool) *displayCtl {
	return &displayCtl{
		hw:    hw,
		after: after,
		guard: revertGuard{after: after, timeout: displayRevertAfter},
	}
}

func realAfter(d time.Duration, f func()) func() bool { return time.AfterFunc(d, f).Stop }

// displays là bộ điều khiển duy nhất của tiến trình giao diện này.
var displays = newDisplayCtl(osDisplayHW(), realAfter)

func (d *displayCtl) read() (displaySnapshot, error) {
	raw, cur, err := d.hw.modes()
	if err != nil {
		return displaySnapshot{}, err
	}
	speed, precision, err := d.hw.getMouse()
	if err != nil {
		return displaySnapshot{}, err
	}
	return displaySnapshot{filterDisplayModes(raw, cur.Orientation), cur.DisplayMode, speed, precision}, nil
}

// begin đọc trạng thái hiện tại và, nếu startup chưa kịp nhớ mốc (đọc lúc đó hỏng),
// nhớ nó làm mốc ở lần đầu. Mọi thao tác ĐỔI phải đi qua đây trước khi đổi, nên mốc luôn
// là thứ có trước thay đổi đầu tiên của khách.
func (d *displayCtl) begin() (displaySnapshot, error) {
	s, err := d.read()
	if err == nil && d.base == nil {
		d.base = &displayBaseline{s.current, s.speed, s.precision}
	}
	return s, err
}

// startup chạy MỘT lần khi tiến trình giao diện khởi động, trước khi máy phủ màn hình
// khoá. Lúc này chưa khách nào đụng tới, nên đây là chỗ lấy mốc đáng tin nhất.
//
// Nếu chế độ đang chạy lệch mặc định trong registry thì trả về mặc định TRƯỚC khi nhớ
// mốc. Hai việc nó chặn: tiến trình giao diện chết (hoặc bị bật lại) đúng lúc một chế độ
// chưa xác nhận đang treo — máy đen màn mà không đồng hồ nào còn sống để hoàn tác, và
// mốc lại ghi nhầm chế độ hỏng đó; và giao diện bật lại giữa lúc game đang giữ một chế
// độ tạm. Không đọc được registry hoặc không thấy lệch thì không đụng vào gì.
func (d *displayCtl) startup() {
	d.mu.Lock()
	defer d.mu.Unlock()
	s, err := d.read()
	if err != nil {
		return // không phải Windows, hoặc không đọc được: mốc sẽ nhớ ở lần đổi đầu tiên
	}
	if reg, err := d.hw.registryMode(); err == nil && staleDisplayMode(reg, s.current) {
		log.Printf("[display] khởi động ở %s, lệch mặc định của máy (%s) — về mặc định", s.current, reg)
		if err := d.hw.restoreDefault(); err != nil {
			log.Printf("[display] không về được mặc định của máy: %v", err)
		}
	}
	d.begin()
}

func (d *displayCtl) redockLater(redock func()) {
	if redock != nil {
		d.after(displaySettle, redock)
	}
}

func (d *displayCtl) info() (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	s, err := d.begin()
	if err != nil {
		// Không phải Windows là chuyện bình thường khi phát triển; không ghi log cho ồn.
		if !errors.Is(err, errDisplayUnsupported) {
			log.Printf("[display] không đọc được cấu hình màn hình: %v", err)
		}
		return unsupportedDisplayInfo().json(), nil
	}
	return displayInfo{
		Supported:      true,
		Modes:          s.modes,
		Current:        s.current,
		Baseline:       d.base.mode,
		MouseSpeed:     s.speed,
		MousePrecision: s.precision,
	}.json(), nil
}

// apply đổi sang m rồi bắt đầu đếm ngược displayRevertAfter. redock chạy displaySettle
// sau mỗi lần đổi thật.
func (d *displayCtl) apply(m DisplayMode, redock func()) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	s, err := d.begin()
	if err != nil {
		return err
	}
	if err := validateDisplayMode(s.modes, m); err != nil {
		return err
	}
	// Đang đúng chế độ đó và không có gì chờ xác nhận thì chẳng có gì để đổi hay để
	// hoàn tác: khỏi làm tối màn hình một lần vô ích.
	if s.current == m && !d.guard.pending {
		return nil
	}
	if err := d.hw.setMode(m); err != nil {
		return err
	}
	log.Printf("[display] đổi %s -> %s, chờ xác nhận %v", s.current, m, displayRevertAfter)
	d.retries = 0
	d.guard.arm(s.current, func(gen uint64) { d.expire(gen, redock) })
	d.redockLater(redock)
	return nil
}

// expire chạy ở goroutine của đồng hồ khi hết displayRevertAfter.
func (d *displayCtl) expire(gen uint64, redock func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	prev, ok := d.guard.expire(gen)
	if !ok {
		return
	}
	log.Printf("[display] không ai xác nhận sau %v — về %s", displayRevertAfter, prev)
	d.restoreOrRetry(prev, redock)
}

func (d *displayCtl) confirm() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.guard.pending {
		log.Printf("[display] khách xác nhận giữ chế độ mới")
	}
	d.guard.clear()
}

// revert về chế độ trước lần đổi chưa xác nhận. Không có gì chờ xác nhận (đã xác nhận,
// hoặc đồng hồ đã tự về rồi) thì không làm gì và không phải lỗi: khách bấm "Huỷ" đúng
// lúc đồng hồ hết giờ không đáng nhận một thông báo lỗi.
func (d *displayCtl) revert(redock func()) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	prev, ok := d.guard.take()
	if !ok {
		return nil
	}
	log.Printf("[display] khách huỷ — về %s", prev)
	return d.restoreOrRetry(prev, redock)
}

func (d *displayCtl) restore(prev DisplayMode, redock func()) error {
	if err := d.hw.setMode(prev); err != nil {
		return err
	}
	d.redockLater(redock)
	return nil
}

// restoreOrRetry về prev. Hỏng thì KHÔNG coi là xong: hẹn lại displayRetryAfter (tối đa
// displayRetryMax lần) bằng chính revertGuard, nên lượt hoàn tác vẫn sống và xác nhận,
// huỷ tay, đổi tiếp hay reset đều xử lý nó như bình thường. Bản cũ đã xoá lượt trước
// khi về, nên một lần hỏng là hết đồng hồ, hết hy vọng. Lỗi vẫn trả cho nơi gọi.
func (d *displayCtl) restoreOrRetry(prev DisplayMode, redock func()) error {
	err := d.restore(prev, redock)
	if err == nil {
		return nil
	}
	log.Printf("[display] không về lại được %s: %v", prev, err)
	if d.retries < displayRetryMax {
		d.retries++
		log.Printf("[display] thử lại sau %v (lần %d/%d)", displayRetryAfter, d.retries, displayRetryMax)
		d.guard.rearm(prev, displayRetryAfter, func(gen uint64) { d.expire(gen, redock) })
	}
	return err
}

func (d *displayCtl) setMouseSpeed(speed int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, err := d.begin(); err != nil {
		return err
	}
	return d.hw.setSpeed(clampMouseSpeed(speed))
}

func (d *displayCtl) setMousePrecision(on bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, err := d.begin(); err != nil {
		return err
	}
	return d.hw.setPrecision(on)
}

// reset trả độ phân giải, tần số, tốc độ chuột và độ chính xác con trỏ về mốc.
// Chưa ai chạm vào gì (chưa có mốc) hoặc mọi thứ đã đúng mốc thì không làm gì cả.
// changed báo ĐỘ PHÂN GIẢI có đổi thật không, để nơi gọi biết cửa sổ cần đo lại.
//
// Một thứ lỗi không chặn các thứ còn lại: không về được độ phân giải thì chuột vẫn
// phải được trả.
func (d *displayCtl) reset(redock func()) (changed bool, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.base == nil {
		return false, nil
	}
	// Lượt hẹn tự hoàn tác còn treo thì phải huỷ: nó sẽ kéo máy về chế độ giữa chừng
	// của khách trước, ngay trong phiên của khách sau.
	d.guard.clear()
	s, readErr := d.read()
	if readErr != nil {
		return false, readErr
	}
	b := d.base
	if s.current == b.mode && s.speed == b.speed && s.precision == b.precision {
		return false, nil
	}

	log.Printf("[display] trả về cài đặt ban đầu: %s, chuột %d", b.mode, b.speed)
	var errs []error
	if s.current != b.mode {
		modeErr := d.hw.setMode(b.mode)
		changed = modeErr == nil
		errs = append(errs, modeErr)
	}
	if s.speed != b.speed {
		errs = append(errs, d.hw.setSpeed(b.speed))
	}
	if s.precision != b.precision {
		errs = append(errs, d.hw.setPrecision(b.precision))
	}
	if changed {
		d.redockLater(redock)
	}
	return changed, errors.Join(errs...)
}

// --- Các phương thức Wails gắn vào giao diện ---------------------------------

// GetDisplayInfo trả JSON mô tả màn hình và chuột; xem displayInfo cho các khoá.
// supported=false (không phải Windows, hoặc không đọc được) thì giao diện ẩn mục này.
func (a *App) GetDisplayInfo() (string, error) {
	return displays.info()
}

// ApplyDisplayMode đổi sang chế độ width x height @ hz và bắt đầu đếm ngược 20 giây:
// giao diện phải gọi ConfirmDisplayMode trong thời gian đó, không thì máy tự về chế độ
// cũ. Chỉ nhận chế độ nằm trong danh sách GetDisplayInfo trả về.
func (a *App) ApplyDisplayMode(width, height, hz int) error {
	return displays.apply(DisplayMode{Width: width, Height: height, Hz: hz}, a.fitWindowToDisplay)
}

// ConfirmDisplayMode giữ chế độ vừa đổi, dừng đếm ngược.
func (a *App) ConfirmDisplayMode() error {
	displays.confirm()
	return nil
}

// RevertDisplayMode về ngay chế độ trước lần đổi chưa xác nhận.
func (a *App) RevertDisplayMode() error {
	return displays.revert(a.fitWindowToDisplay)
}

// SetMouseSpeed đặt tốc độ con trỏ (1..20, ngoài khoảng thì bị ép vào).
func (a *App) SetMouseSpeed(speed int) error {
	return displays.setMouseSpeed(speed)
}

// SetMousePrecision bật/tắt "Nâng cao độ chính xác của con trỏ" (gia tốc chuột).
func (a *App) SetMousePrecision(on bool) error {
	return displays.setMousePrecision(on)
}

// ResetDisplayAndMouse trả mọi thứ về mốc ban đầu của tiến trình này.
func (a *App) ResetDisplayAndMouse() error {
	_, err := displays.reset(a.fitWindowToDisplay)
	return err
}

// resetDisplayForNextUser là chỗ máy khoá hoặc khách đăng xuất (applyLoginLock(true),
// Logout) gọi, để người ngồi sau không thừa hưởng độ phân giải hay chuột của người
// trước. KHÔNG dán lại thanh điều khiển — ngay sau đó máy phủ màn hình khoá — nhưng
// phải đo lại cửa sổ nếu nó đang toàn màn hình.
//
// Lỗi chỉ ghi log: không về được mốc không bao giờ được chặn việc khoá máy.
func (a *App) resetDisplayForNextUser() {
	changed, err := displays.reset(nil)
	if err != nil {
		log.Printf("[display] không trả hết cài đặt về mức ban đầu: %v", err)
	}
	if changed {
		a.refitFullscreen()
	}
}

// refitFullscreen đo lại cửa sổ toàn màn hình (màn hình khoá) theo chế độ màn hình mới.
// Wails chỉ đọc kích thước màn hình LÚC vào toàn màn hình và không theo dõi việc đổi
// độ phân giải, còn WindowFullscreen bỏ qua khi đã toàn màn hình — nên màn hình khoá
// dựng ở 1920x1080 mà máy về 1280x720 thì hoặc tràn ra ngoài, hoặc để hở một mảng
// desktop bấm được. Thoát rồi vào lại thì Wails đo lại. Trả true nếu cửa sổ đang toàn
// màn hình.
func (a *App) refitFullscreen() bool {
	if a.windowMode != "" || a.ctx == nil || !wailsruntime.WindowIsFullscreen(a.ctx) {
		return false
	}
	wailsruntime.WindowUnfullscreen(a.ctx)
	wailsruntime.WindowFullscreen(a.ctx)
	return true
}

// fitWindowToDisplay chạy displaySettle sau khi chế độ màn hình đổi, ở goroutine của
// đồng hồ (hoặc của lệnh Wails): toàn màn hình thì đo lại màn hình khoá, còn lại thì
// dán lại thanh điều khiển vì vùng làm việc đã đổi.
//
// KHÔNG gọi ApplyWindowState khi đang toàn màn hình: nó chỉ nhìn token, mà màn hình
// khoá vì mất kết nối máy chủ thì khách vẫn đang đăng nhập — nó sẽ coi là "đã mở
// khoá" và gỡ luôn màn hình khoá đó. Toàn màn hình chỉ có ở hai kiểu khoá, nên đó là
// dấu hiệu đủ để rẽ nhánh. Các lệnh cửa sổ của Wails dùng được từ goroutine bất kỳ
// (OnStartup cũng chạy trong goroutine); ApplyWindowState vốn là phương thức Wails.
func (a *App) fitWindowToDisplay() {
	if a.refitFullscreen() {
		return
	}
	a.ApplyWindowState()
}
