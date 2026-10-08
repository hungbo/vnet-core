package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

// Đổi độ phân giải, tần số quét và chuột ngay trên máy trạm: khách chọn trong
// giao diện, không phải mở Control Panel (mà màn hình khoá cũng chặn).
//
// CHỈ màn hình CHÍNH. Máy quán gần như luôn một màn hình; máy hai màn hình thì màn
// phụ giữ nguyên mọi thứ, vì chọn "màn nào" là một giao diện nữa mà chưa ai cần.
//
// Mọi thứ chạy trong tiến trình GIAO DIỆN (phiên của khách, token của khách), không
// bao giờ ở dịch vụ nền: dịch vụ ở session 0 không có desktop của khách, nên đổi
// chế độ hay SystemParametersInfo ở đó không tới được màn hình khách đang nhìn.
//
// Phần Win32 ở display_windows.go, bản không phải Windows ở display_other.go,
// việc điều phối (mốc, hẹn giờ tự hoàn tác) ở display_app.go; tệp này giữ phần
// thuần để kiểm bằng bảng trên bất kỳ máy nào.

const (
	// Dưới cỡ này là chế độ chẩn đoán (640x480, 800x500...) mà giao diện VNET không
	// còn dùng được.
	displayMinWidth  = 800
	displayMinHeight = 600

	// Đổi độ phân giải sang chế độ màn hình không nhận (đen màn, lệch tần số) thì
	// khách không thấy gì để bấm "Huỷ". Vì vậy chế độ mới chỉ giữ nếu được xác nhận
	// trong chừng này; hết giờ là tự về chế độ cũ.
	displayRevertAfter = 20 * time.Second
	// Về chế độ cũ mà hỏng (card đồ hoạ đang bận đồng bộ lại tấm nền) thì màn hình vẫn
	// đen: bỏ cuộc sau một lần là để máy tối cho tới khi nhân viên khởi động lại. Thử
	// lại ngắn, chừng này lần, rồi mới thôi.
	displayRetryAfter = 4 * time.Second
	displayRetryMax   = 5
	// Đổi chế độ xong, explorer.exe cần một lúc mới dời taskbar và cập nhật vùng làm
	// việc; đo ngay thì vẫn ra vùng cũ và thanh điều khiển đè lên taskbar. Chờ chừng
	// này rồi mới dán lại thanh.
	displaySettle = time.Second

	mouseSpeedMin     = 1
	mouseSpeedMax     = 20
	mouseSpeedDefault = 10 // mặc định của Windows (mức 6/11 trên thanh trượt)

	// Ngưỡng của "Nâng cao độ chính xác của con trỏ" khi Windows tự bật nó.
	mouseAccelThreshold1 = 6
	mouseAccelThreshold2 = 10
)

// errDisplayUnsupported: bản dựng không phải Windows, hoặc không đọc được màn hình.
var errDisplayUnsupported = errors.New("Chỉ máy trạm Windows mới đổi được độ phân giải và cài đặt chuột")

// DisplayMode là một chế độ màn hình: độ phân giải và tần số quét.
type DisplayMode struct {
	Width  int `json:"width"`
	Height int `json:"height"`
	Hz     int `json:"hz"`
}

func (m DisplayMode) String() string {
	return fmt.Sprintf("%dx%d %dHz", m.Width, m.Height, m.Hz)
}

// rawDisplayMode là một dòng liệt kê thô của Windows, chưa lọc. Windows trả cả chế độ
// 16 bit, xen kẽ, xoay ngang/dọc và trùng nhau chỉ khác kiểu co giãn.
type rawDisplayMode struct {
	DisplayMode
	BitsPerPel  int
	Orientation int
	Interlaced  bool
}

// displayInfo là JSON trả cho giao diện; khoá khớp hợp đồng với phần Vue.
type displayInfo struct {
	Supported      bool          `json:"supported"`
	Modes          []DisplayMode `json:"modes"`
	Current        DisplayMode   `json:"current"`
	Baseline       DisplayMode   `json:"baseline"`
	MouseSpeed     int           `json:"mouse_speed"`
	MousePrecision bool          `json:"mouse_precision"`
}

// unsupportedDisplayInfo làm giao diện ẩn cả mục này. "modes" phải là mảng rỗng chứ
// không phải null, để phía Vue không phải đoán.
func unsupportedDisplayInfo() displayInfo {
	return displayInfo{Modes: []DisplayMode{}, MouseSpeed: mouseSpeedDefault}
}

func (i displayInfo) json() string {
	out, _ := json.Marshal(i)
	return string(out)
}

// filterDisplayModes giữ lại những chế độ giao diện cho chọn: 32 bit màu, quét liên
// tục, đúng hướng xoay hiện tại, từ 800x600 trở lên; bỏ trùng; xếp rộng nhất, cao
// nhất, nhanh nhất lên đầu.
func filterDisplayModes(raw []rawDisplayMode, orientation int) []DisplayMode {
	seen := make(map[DisplayMode]bool, len(raw))
	modes := []DisplayMode{}
	for _, r := range raw {
		if r.BitsPerPel != 32 || r.Interlaced || r.Orientation != orientation {
			continue
		}
		if r.Width < displayMinWidth || r.Height < displayMinHeight || seen[r.DisplayMode] {
			continue
		}
		seen[r.DisplayMode] = true
		modes = append(modes, r.DisplayMode)
	}
	sort.Slice(modes, func(i, j int) bool {
		a, b := modes[i], modes[j]
		if a.Width != b.Width {
			return a.Width > b.Width
		}
		if a.Height != b.Height {
			return a.Height > b.Height
		}
		return a.Hz > b.Hz
	})
	return modes
}

// staleDisplayMode: chế độ đang chạy có lệch mặc định của máy (chế độ lưu trong
// registry) không. Ta không bao giờ ghi registry (cờ 0), nên registry luôn là chế độ
// lành; lệch tức là một tiến trình trước — game, hoặc chính giao diện này lúc chưa
// kịp hoàn tác — đã đổi động rồi để lại.
//
// Chỉ kết luận khi registry nói RÕ: registry không đọc ra kích thước hợp lý thì bỏ qua
// (đoán sai là kéo cả phòng máy về một chế độ lạ). Tần số 0 hoặc 1 là "mặc định của card"
// chứ không phải một con số, nên không đem ra so.
func staleDisplayMode(registry, current DisplayMode) bool {
	if registry.Width < displayMinWidth || registry.Height < displayMinHeight {
		return false
	}
	if registry.Width != current.Width || registry.Height != current.Height {
		return true
	}
	return registry.Hz > 1 && registry.Hz != current.Hz
}

// validateDisplayMode: chỉ nhận chế độ nằm trong danh sách đã liệt kê. Số do giao
// diện gửi lên không bao giờ được tin — đưa thẳng cho Windows là cho phép ép card
// đồ hoạ vào một chế độ tuỳ ý.
func validateDisplayMode(modes []DisplayMode, m DisplayMode) error {
	for _, c := range modes {
		if c == m {
			return nil
		}
	}
	return fmt.Errorf("Màn hình không hỗ trợ chế độ %s", m)
}

// clampMouseSpeed ép tốc độ chuột vào thang 1..20 của Windows.
func clampMouseSpeed(speed int) int {
	return min(max(speed, mouseSpeedMin), mouseSpeedMax)
}

// SPI_GETMOUSE / SPI_SETMOUSE làm việc với ba số: [ngưỡng 1, ngưỡng 2, gia tốc].
// "Nâng cao độ chính xác của con trỏ" chính là số thứ ba khác 0.

func mousePrecisionOn(p [3]int32) bool { return p[2] != 0 }

// withMousePrecision chỉ đổi số gia tốc, giữ hai ngưỡng như máy đang để. Bật mà
// ngưỡng đang bằng 0 (do trước đó tắt kiểu "0,0,0") thì gia tốc không có tác dụng
// gì, nên lúc đó đặt ngưỡng mặc định của Windows. Gia tốc đã khác 0 thì giữ nguyên
// giá trị (Windows cũ còn có mức 2).
func withMousePrecision(p [3]int32, on bool) [3]int32 {
	if !on {
		p[2] = 0
		return p
	}
	if p[0] == 0 {
		p[0] = mouseAccelThreshold1
	}
	if p[1] == 0 {
		p[1] = mouseAccelThreshold2
	}
	if p[2] == 0 {
		p[2] = 1
	}
	return p
}

// displayChangeError dịch mã DISP_CHANGE_* của ChangeDisplaySettingsEx thành câu
// tiếng Việt khách đọc được.
func displayChangeError(code int32) error {
	switch code {
	case 0: // DISP_CHANGE_SUCCESSFUL
		return nil
	case 1: // DISP_CHANGE_RESTART
		return errors.New("Chế độ này phải khởi động lại máy mới dùng được nên không áp dụng")
	case -1: // DISP_CHANGE_FAILED
		return errors.New("Card đồ hoạ từ chối đổi sang chế độ này")
	case -2: // DISP_CHANGE_BADMODE
		return errors.New("Màn hình hoặc card đồ hoạ không hỗ trợ chế độ này")
	case -3: // DISP_CHANGE_NOTUPDATED
		return errors.New("Windows không ghi nhận được chế độ mới")
	case -4: // DISP_CHANGE_BADFLAGS
		return errors.New("Yêu cầu đổi chế độ màn hình không hợp lệ")
	case -5: // DISP_CHANGE_BADPARAM
		return errors.New("Thông số chế độ màn hình không hợp lệ")
	case -6: // DISP_CHANGE_BADDUALVIEW
		return errors.New("Không đổi được chế độ khi máy đang chạy DualView")
	}
	return fmt.Errorf("Không đổi được chế độ màn hình (mã lỗi %d)", code)
}

// displayHW là phần chạm vào hệ điều hành, gom lại để bài kiểm thay bằng bản giả.
// display_windows.go và display_other.go đều đưa ra một osDisplayHW().
type displayHW struct {
	// modes liệt kê TOÀN BỘ chế độ thô của màn hình chính, kèm chế độ đang dùng.
	modes func() (raw []rawDisplayMode, current rawDisplayMode, err error)
	// setMode đổi động (không ghi registry) sang chế độ đã được kiểm là hợp lệ.
	setMode func(DisplayMode) error
	// registryMode là chế độ lưu trong registry của màn hình chính; restoreDefault bảo
	// Windows trả màn hình về đúng chế độ đó (cách Windows dành cho "về lại sau một
	// lần đổi động", không cần biết chế độ là gì). Chỉ dùng lúc khởi động.
	registryMode   func() (DisplayMode, error)
	restoreDefault func() error
	getMouse       func() (speed int, precision bool, err error)
	setSpeed       func(speed int) error
	setPrecision   func(on bool) error
}

// revertGuard là máy trạng thái của hộp thoại "giữ chế độ này?": đổi chế độ xong thì
// nhớ chế độ CŨ và hẹn giờ; xác nhận thì xoá; hết giờ thì trả chế độ cũ ra để về.
//
// Không có khoá riêng và không tự gọi gì khi hết giờ: người dùng nó (displayCtl)
// giữ một khoá cho cả việc đổi chế độ lẫn các thao tác ở đây. Hai khoá lồng nhau thì
// hoặc là bế tắc (đồng hồ giữ khoá này chờ khoá kia, lệnh đổi giữ ngược lại), hoặc
// là đồng hồ đã lấy chế độ cũ ra rồi mà lần đổi mới chen vào trước khi nó kịp về.
type revertGuard struct {
	// after là time.AfterFunc, tách ra để bài kiểm chạy đồng hồ bằng tay.
	after   func(d time.Duration, f func()) (stop func() bool)
	timeout time.Duration

	pending bool
	prev    DisplayMode
	// gen tăng mỗi lần hẹn mới hoặc huỷ. Dừng đồng hồ không đảm bảo hàm hẹn giờ chưa
	// chạy: nó có thể đã bắt đầu và đang chờ khoá. Nó mang số lượt của chính nó và
	// bị bỏ qua nếu lượt đó không còn là lượt hiện hành.
	gen  uint64
	stop func() bool
}

// arm gọi SAU KHI đổi chế độ thành công; prev là chế độ ngay trước lần đổi này.
// Đang chờ xác nhận sẵn thì GIỮ chế độ gốc: khách thử 1280x720 rồi thử tiếp
// 1600x900 mà không xác nhận thì "huỷ" phải về chế độ họ đang dùng ban đầu, không
// phải về 1280x720 chưa ai xác nhận. Đồng hồ thì tính lại từ đầu cho lần đổi mới.
// onExpire chạy ở goroutine của đồng hồ với số lượt của lần hẹn này.
func (g *revertGuard) arm(prev DisplayMode, onExpire func(gen uint64)) {
	if g.pending {
		g.stop()
	} else {
		g.pending, g.prev = true, prev
	}
	g.gen++
	gen := g.gen
	g.stop = g.after(g.timeout, func() { onExpire(gen) })
}

// rearm hẹn lại sau d khi lần về prev vừa HỎNG (đã take hoặc expire rồi): vẫn coi là
// đang chờ hoàn tác, để xác nhận, huỷ tay, đổi tiếp hay reset đều xử lý nó như một lượt
// bình thường. Khác arm ở chỗ prev do nơi gọi đưa vào chứ không giữ lượt trước.
func (g *revertGuard) rearm(prev DisplayMode, d time.Duration, onExpire func(gen uint64)) {
	g.pending, g.prev = true, prev
	g.gen++
	gen := g.gen
	g.stop = g.after(d, func() { onExpire(gen) })
}

// clear: đã xác nhận (hoặc không còn gì để hoàn tác). Huỷ đồng hồ, nên lần hẹn cũ
// có chạy muộn cũng thành vô hiệu.
func (g *revertGuard) clear() {
	if !g.pending {
		return
	}
	g.stop()
	g.pending = false
	g.gen++
}

// take: hoàn tác bằng tay. Trả chế độ cũ rồi coi như xong việc.
func (g *revertGuard) take() (prev DisplayMode, ok bool) {
	if !g.pending {
		return DisplayMode{}, false
	}
	prev = g.prev
	g.clear()
	return prev, true
}

// expire: đồng hồ báo hết giờ cho lượt gen. Chỉ lượt hiện hành mới được về.
func (g *revertGuard) expire(gen uint64) (prev DisplayMode, ok bool) {
	if gen != g.gen {
		return DisplayMode{}, false
	}
	return g.take()
}
