package main

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// fakeScreen là màn hình + chuột giả cho displayCtl: nhớ trạng thái và ghi lại mọi
// lần ghi, để bài kiểm khẳng định được cả "đã đổi gì" lẫn "không đụng gì".
type fakeScreen struct {
	raw       []rawDisplayMode
	cur       rawDisplayMode
	speed     int
	precision bool

	reg        DisplayMode // chế độ trong registry (mặc định của máy)
	readErr    error       // làm modes() hỏng
	regErr     error       // làm registryMode() hỏng
	setModeErr error
	// setModeFails: số lần setMode kế tiếp sẽ hỏng rồi mới tốt lại (card đồ hoạ bận tạm thời).
	setModeFails int

	setModes []DisplayMode // lịch sử setMode (chỉ những lần thành công)
	restores int           // số lần restoreDefault
	speeds   []int
	precs    []bool
}

func (f *fakeScreen) writes() int {
	return len(f.setModes) + f.restores + len(f.speeds) + len(f.precs)
}

func (f *fakeScreen) hw() displayHW {
	return displayHW{
		modes: func() ([]rawDisplayMode, rawDisplayMode, error) {
			if f.readErr != nil {
				return nil, rawDisplayMode{}, f.readErr
			}
			return f.raw, f.cur, nil
		},
		setMode: func(m DisplayMode) error {
			if f.setModeErr != nil {
				return f.setModeErr
			}
			if f.setModeFails > 0 {
				f.setModeFails--
				return errors.New("Card đồ hoạ từ chối đổi sang chế độ này")
			}
			f.setModes = append(f.setModes, m)
			f.cur.DisplayMode = m
			return nil
		},
		registryMode: func() (DisplayMode, error) { return f.reg, f.regErr },
		restoreDefault: func() error {
			f.restores++
			f.cur.DisplayMode = f.reg
			return nil
		},
		getMouse: func() (int, bool, error) { return f.speed, f.precision, nil },
		setSpeed: func(s int) error {
			f.speeds = append(f.speeds, s)
			f.speed = s
			return nil
		},
		setPrecision: func(on bool) error {
			f.precs = append(f.precs, on)
			f.precision = on
			return nil
		},
	}
}

type displayRig struct {
	ctl    *displayCtl
	screen *fakeScreen
	clock  *fakeClock
	redock int // số lần thanh điều khiển được dán lại
}

func (r *displayRig) onRedock() { r.redock++ }

var (
	modeGoc = dm(1920, 1080, 60)
	modeA   = dm(1280, 720, 60)
	modeB   = dm(1920, 1080, 144)
)

// newDisplayRig: màn hình 1920x1080@60 với ba chế độ chọn được, chuột tốc độ 10,
// "độ chính xác con trỏ" đang bật (đúng mặc định của Windows).
func newDisplayRig() *displayRig {
	s := &fakeScreen{
		raw:       []rawDisplayMode{raw32(1920, 1080, 60), raw32(1920, 1080, 144), raw32(1280, 720, 60), raw32(640, 480, 60)},
		cur:       raw32(1920, 1080, 60),
		reg:       modeGoc,
		speed:     10,
		precision: true,
	}
	c := &fakeClock{}
	return &displayRig{ctl: newDisplayCtl(s.hw(), c.after), screen: s, clock: c}
}

func (r *displayRig) info(t *testing.T) displayInfo {
	t.Helper()
	out, err := r.ctl.info()
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	var got displayInfo
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("info không phải JSON: %v\n%s", err, out)
	}
	return got
}

func TestDisplayInfo_HopDong(t *testing.T) {
	r := newDisplayRig()
	out, _ := r.ctl.info()

	// Khoá JSON là hợp đồng với phần Vue: đủ sáu khoá, không thừa không thiếu.
	var raw map[string]any
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := []string{"baseline", "current", "modes", "mouse_precision", "mouse_speed", "supported"}
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("khoá JSON = %v, mong %v", keys, want)
	}

	got := r.info(t)
	if !got.Supported || got.Current != modeGoc || got.Baseline != modeGoc || got.MouseSpeed != 10 || !got.MousePrecision {
		t.Errorf("info = %+v", got)
	}
	// 640x480 bị lọc; còn lại xếp nhanh nhất lên đầu.
	if !reflect.DeepEqual(got.Modes, []DisplayMode{modeB, modeGoc, modeA}) {
		t.Errorf("modes = %v", got.Modes)
	}
}

func TestDisplayInfo_KhongHoTro(t *testing.T) {
	for _, loi := range []error{errDisplayUnsupported, errors.New("EnumDisplaySettings hỏng")} {
		r := newDisplayRig()
		r.screen.readErr = loi
		out, err := r.ctl.info()
		if err != nil {
			t.Fatalf("%v: info phải trả JSON supported=false chứ không phải lỗi: %v", loi, err)
		}
		for _, want := range []string{`"supported":false`, `"modes":[]`} {
			if !strings.Contains(out, want) {
				t.Errorf("%v: thiếu %s trong %s", loi, want, out)
			}
		}
	}
}

// Mốc là lúc giao diện nhìn vào LẦN ĐẦU; các lần sau không được cập nhật nó, không
// thì "trả về mốc" trả về chính chế độ khách vừa chọn.
func TestDisplayInfo_MocChiLayLanDau(t *testing.T) {
	r := newDisplayRig()
	r.info(t)
	if err := r.ctl.apply(modeA, r.onRedock); err != nil {
		t.Fatal(err)
	}
	r.ctl.confirm()
	if err := r.ctl.setMouseSpeed(18); err != nil {
		t.Fatal(err)
	}

	got := r.info(t)
	if got.Baseline != modeGoc {
		t.Errorf("mốc bị cập nhật: %v, mong %v", got.Baseline, modeGoc)
	}
	if got.Current != modeA || got.MouseSpeed != 18 {
		t.Errorf("trạng thái hiện tại sai: %+v", got)
	}
}

func TestDisplayApply_TuChoiCheDoLa(t *testing.T) {
	r := newDisplayRig()
	for _, m := range []DisplayMode{dm(1234, 567, 60), dm(640, 480, 60), dm(1920, 1080, 120), {}} {
		if err := r.ctl.apply(m, r.onRedock); err == nil {
			t.Errorf("apply(%v) không bị từ chối", m)
		}
	}
	if r.screen.writes() != 0 || len(r.clock.timers) != 0 {
		t.Errorf("chế độ lạ mà vẫn đụng vào máy: ghi %d, hẹn giờ %d", r.screen.writes(), len(r.clock.timers))
	}
}

func TestDisplayApply_XacNhanThiGiu(t *testing.T) {
	r := newDisplayRig()
	if err := r.ctl.apply(modeA, r.onRedock); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.screen.setModes, []DisplayMode{modeA}) {
		t.Fatalf("setMode = %v", r.screen.setModes)
	}
	if r.clock.live(displayRevertAfter) != 1 {
		t.Fatalf("phải có đúng một đồng hồ tự hoàn tác 20s, có %d", r.clock.live(displayRevertAfter))
	}

	// Thanh điều khiển chỉ được dán lại SAU displaySettle (explorer cần thời gian dời taskbar).
	if r.redock != 0 {
		t.Errorf("dán lại thanh quá sớm: %d lần", r.redock)
	}
	r.clock.fire(displaySettle)
	if r.redock != 1 {
		t.Errorf("sau displaySettle phải dán lại đúng một lần, được %d", r.redock)
	}

	r.ctl.confirm()
	if r.clock.live(displayRevertAfter) != 0 {
		t.Error("xác nhận rồi mà đồng hồ tự hoàn tác vẫn sống")
	}
	r.clock.fireStopped(displayRevertAfter) // hàm hẹn giờ lỡ chạy muộn
	if !reflect.DeepEqual(r.screen.setModes, []DisplayMode{modeA}) {
		t.Errorf("đã xác nhận mà máy vẫn bị kéo về: %v", r.screen.setModes)
	}
}

func TestDisplayApply_HetGioTuVe(t *testing.T) {
	r := newDisplayRig()
	if err := r.ctl.apply(modeA, r.onRedock); err != nil {
		t.Fatal(err)
	}
	r.clock.fire(displaySettle)
	r.redock = 0

	if n := r.clock.fire(displayRevertAfter); n != 1 {
		t.Fatalf("đồng hồ tự hoàn tác chạy %d lần", n)
	}
	if !reflect.DeepEqual(r.screen.setModes, []DisplayMode{modeA, modeGoc}) {
		t.Errorf("hết giờ phải về chế độ cũ: %v", r.screen.setModes)
	}
	r.clock.fire(displaySettle)
	if r.redock != 1 {
		t.Errorf("về xong phải dán lại thanh: %d lần", r.redock)
	}
	// Bấm "Huỷ" đúng lúc đồng hồ vừa về: không còn gì để hoàn tác, và không phải lỗi.
	if err := r.ctl.revert(r.onRedock); err != nil {
		t.Errorf("revert sau khi đã tự về: %v", err)
	}
	if len(r.screen.setModes) != 2 {
		t.Errorf("revert thừa đã đổi chế độ lần nữa: %v", r.screen.setModes)
	}
}

func TestDisplayApply_LanHaiGiuCheDoGoc(t *testing.T) {
	r := newDisplayRig()
	if err := r.ctl.apply(modeA, r.onRedock); err != nil {
		t.Fatal(err)
	}
	if err := r.ctl.apply(modeB, r.onRedock); err != nil { // chưa xác nhận đã đổi tiếp
		t.Fatal(err)
	}
	if r.clock.live(displayRevertAfter) != 1 {
		t.Fatalf("đổi lần hai phải đặt lại đồng hồ: còn sống %d", r.clock.live(displayRevertAfter))
	}

	// Cả hai hàm hẹn giờ cùng chạy (cái cũ chạy muộn): máy về chế độ GỐC đúng một lần.
	r.clock.fireStopped(displayRevertAfter)
	want := []DisplayMode{modeA, modeB, modeGoc}
	if !reflect.DeepEqual(r.screen.setModes, want) {
		t.Errorf("setMode = %v, mong %v (về 1920x1080@60 gốc, không phải 1280x720 chưa ai xác nhận)", r.screen.setModes, want)
	}
}

func TestDisplayApply_LoiDoiCheDo(t *testing.T) {
	r := newDisplayRig()
	r.screen.setModeErr = errors.New("Card đồ hoạ từ chối đổi sang chế độ này")
	err := r.ctl.apply(modeA, r.onRedock)
	if err == nil || !strings.Contains(err.Error(), "từ chối") {
		t.Fatalf("lỗi của Windows phải tới tay khách: %v", err)
	}
	if r.ctl.guard.pending || len(r.clock.timers) != 0 {
		t.Error("đổi hỏng mà vẫn hẹn giờ tự hoàn tác")
	}
}

func TestDisplayApply_DangDungCheDoDo(t *testing.T) {
	r := newDisplayRig()
	if err := r.ctl.apply(modeGoc, r.onRedock); err != nil {
		t.Fatal(err)
	}
	if r.screen.writes() != 0 || len(r.clock.timers) != 0 {
		t.Errorf("đang ở đúng chế độ đó mà vẫn đổi: ghi %d, hẹn giờ %d", r.screen.writes(), len(r.clock.timers))
	}
}

func TestDisplayRevert_Tay(t *testing.T) {
	r := newDisplayRig()
	if err := r.ctl.revert(r.onRedock); err != nil || r.screen.writes() != 0 {
		t.Fatalf("revert khi chưa đổi gì: %v, ghi %d", err, r.screen.writes())
	}

	if err := r.ctl.apply(modeA, r.onRedock); err != nil {
		t.Fatal(err)
	}
	if err := r.ctl.revert(r.onRedock); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.screen.setModes, []DisplayMode{modeA, modeGoc}) {
		t.Errorf("revert phải về chế độ cũ: %v", r.screen.setModes)
	}
	if r.clock.live(displayRevertAfter) != 0 {
		t.Error("revert tay mà đồng hồ vẫn sống")
	}
	r.clock.fire(displaySettle)
	if r.redock != 2 { // một lần sau apply, một lần sau revert
		t.Errorf("dán lại thanh %d lần, mong 2", r.redock)
	}
}

func TestDisplayMouse(t *testing.T) {
	r := newDisplayRig()
	// Gọi đầu tiên là một lệnh GHI: mốc phải là giá trị TRƯỚC khi ghi.
	if err := r.ctl.setMouseSpeed(99); err != nil {
		t.Fatal(err)
	}
	if err := r.ctl.setMouseSpeed(-4); err != nil {
		t.Fatal(err)
	}
	if err := r.ctl.setMousePrecision(false); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.screen.speeds, []int{20, 1}) || !reflect.DeepEqual(r.screen.precs, []bool{false}) {
		t.Errorf("tốc độ %v, độ chính xác %v", r.screen.speeds, r.screen.precs)
	}
	if got := r.info(t); got.MouseSpeed != 1 || got.MousePrecision {
		t.Errorf("info sau khi đổi: %+v", got)
	}
	if r.ctl.base == nil || r.ctl.base.speed != 10 || !r.ctl.base.precision {
		t.Errorf("mốc chuột = %+v, mong tốc độ 10 và độ chính xác bật", r.ctl.base)
	}
}

func TestDisplayCtl_KhongHoTro(t *testing.T) {
	r := newDisplayRig()
	r.screen.readErr = errDisplayUnsupported
	if err := r.ctl.apply(modeA, r.onRedock); !errors.Is(err, errDisplayUnsupported) {
		t.Errorf("apply = %v", err)
	}
	if err := r.ctl.setMouseSpeed(5); !errors.Is(err, errDisplayUnsupported) {
		t.Errorf("setMouseSpeed = %v", err)
	}
	if err := r.ctl.setMousePrecision(true); !errors.Is(err, errDisplayUnsupported) {
		t.Errorf("setMousePrecision = %v", err)
	}
	if r.screen.writes() != 0 {
		t.Error("không hỗ trợ mà vẫn ghi")
	}
}

func TestDisplayReset_TraVeMoc(t *testing.T) {
	r := newDisplayRig()
	if err := r.ctl.setMouseSpeed(20); err != nil {
		t.Fatal(err)
	}
	if err := r.ctl.setMousePrecision(false); err != nil {
		t.Fatal(err)
	}
	if err := r.ctl.apply(modeA, r.onRedock); err != nil { // còn treo, chưa xác nhận
		t.Fatal(err)
	}
	r.clock.fire(displaySettle)
	r.redock = 0

	changed, err := r.ctl.reset(r.onRedock)
	if err != nil || !changed {
		t.Fatalf("reset = %v, %v", changed, err)
	}
	if r.screen.cur.DisplayMode != modeGoc || r.screen.speed != 10 || !r.screen.precision {
		t.Errorf("chưa về mốc: %v, tốc độ %d, độ chính xác %v", r.screen.cur.DisplayMode, r.screen.speed, r.screen.precision)
	}
	r.clock.fire(displaySettle)
	if r.redock != 1 {
		t.Errorf("đổi độ phân giải thì phải dán lại thanh: %d lần", r.redock)
	}

	// Lượt hẹn tự hoàn tác còn treo của khách trước không được kéo máy khách sau về
	// chế độ giữa chừng.
	writes := r.screen.writes()
	r.clock.fireStopped(displayRevertAfter)
	if r.screen.writes() != writes {
		t.Error("đồng hồ cũ vẫn kéo máy đi sau khi reset")
	}

	// Đã đúng mốc: gọi lại không làm gì.
	changed, err = r.ctl.reset(r.onRedock)
	if err != nil || changed || r.screen.writes() != writes {
		t.Errorf("reset lần hai = %v, %v, ghi thêm %d", changed, err, r.screen.writes()-writes)
	}
}

func TestDisplayReset_ChuaChamGiThiKhongLamGi(t *testing.T) {
	r := newDisplayRig()
	if changed, err := r.ctl.reset(r.onRedock); err != nil || changed || r.screen.writes() != 0 {
		t.Errorf("chưa có mốc: reset = %v, %v, ghi %d", changed, err, r.screen.writes())
	}
	r.info(t) // có mốc, nhưng chưa đổi gì
	if changed, err := r.ctl.reset(r.onRedock); err != nil || changed || r.screen.writes() != 0 {
		t.Errorf("chưa đổi gì: reset = %v, %v, ghi %d", changed, err, r.screen.writes())
	}
	if len(r.clock.timers) != 0 {
		t.Error("reset không đổi gì mà vẫn hẹn giờ dán lại thanh")
	}
}

// Không về được độ phân giải thì chuột vẫn phải được trả; và gọi reset(nil) (đường
// khoá máy) thì không hẹn việc dán lại thanh.
func TestDisplayReset_LoiKhongChanCaiConLai(t *testing.T) {
	r := newDisplayRig()
	if err := r.ctl.apply(modeA, r.onRedock); err != nil {
		t.Fatal(err)
	}
	r.ctl.confirm()
	if err := r.ctl.setMouseSpeed(3); err != nil {
		t.Fatal(err)
	}
	r.screen.setModeErr = errors.New("Màn hình không hỗ trợ")
	hen := len(r.clock.timers)

	changed, err := r.ctl.reset(nil)
	if err == nil || !strings.Contains(err.Error(), "không hỗ trợ") {
		t.Errorf("lỗi đổi chế độ bị nuốt: %v", err)
	}
	if changed {
		t.Error("đổi hỏng mà báo changed")
	}
	if r.screen.speed != 10 {
		t.Errorf("chuột không được trả dù chỉ độ phân giải hỏng: tốc độ %d", r.screen.speed)
	}
	if len(r.clock.timers) != hen {
		t.Error("redock=nil mà vẫn hẹn giờ")
	}
}

// --- Mốc lấy lúc khởi động ---------------------------------------------------------

// Lỗi gốc: mốc lấy ở lần đầu có người mở Cài đặt. Game đang giữ 1280x720 lúc đó thì
// 1280x720 thành "mặc định" cho mọi khách sau. Mốc lấy lúc khởi động thì không.
func TestDisplayStartup_MocKhongTheoGame(t *testing.T) {
	r := newDisplayRig()
	r.ctl.startup()

	r.screen.cur.DisplayMode = modeA // game vào toàn màn hình 1280x720
	if got := r.info(t); got.Baseline != modeGoc || got.Current != modeA {
		t.Fatalf("mốc theo game: baseline %v, current %v", got.Baseline, got.Current)
	}
	r.screen.cur.DisplayMode = modeGoc // game thoát, Windows trả desktop về
	if err := r.ctl.apply(modeB, r.onRedock); err != nil {
		t.Fatal(err)
	}
	r.ctl.confirm()

	// Khách đăng xuất: máy về 1920x1080@60, không phải 1280x720 của game.
	if changed, err := r.ctl.reset(nil); err != nil || !changed {
		t.Fatalf("reset = %v, %v", changed, err)
	}
	if r.screen.cur.DisplayMode != modeGoc {
		t.Errorf("sau đăng xuất máy ở %v, mong %v", r.screen.cur.DisplayMode, modeGoc)
	}
}

// Tiến trình giao diện chết giữa lúc một chế độ chưa xác nhận đang treo: máy đen màn
// và đồng hồ cũ đã mất. Tiến trình mới phải kéo máy về mặc định và nhớ mặc định làm mốc,
// không phải nhớ chế độ hỏng làm mốc.
func TestDisplayStartup_VeMacDinhKhiMayKetOCheDoLa(t *testing.T) {
	r := newDisplayRig()
	r.screen.cur.DisplayMode = modeA // chế độ treo của tiến trình trước, registry vẫn là modeGoc

	r.ctl.startup()
	if r.screen.restores != 1 || r.screen.cur.DisplayMode != modeGoc {
		t.Fatalf("restoreDefault %d lần, máy ở %v", r.screen.restores, r.screen.cur.DisplayMode)
	}
	if got := r.info(t); got.Baseline != modeGoc {
		t.Errorf("mốc = %v, mong mặc định %v (không phải chế độ hỏng)", got.Baseline, modeGoc)
	}
	if changed, err := r.ctl.reset(nil); err != nil || changed {
		t.Errorf("máy đã ở mốc mà reset vẫn đổi: %v, %v", changed, err)
	}
}

// Không có bằng chứng lệch thì không đụng vào máy.
func TestDisplayStartup_KhongLechThiKhongDung(t *testing.T) {
	cases := map[string]func(r *displayRig){
		"đúng mặc định":                func(r *displayRig) {},
		"registry không đọc được":      func(r *displayRig) { r.screen.regErr = errors.New("hỏng"); r.screen.cur.DisplayMode = modeA },
		"registry ra kích thước vô lý": func(r *displayRig) { r.screen.reg = DisplayMode{}; r.screen.cur.DisplayMode = modeA },
		"registry tần số 0 (mặc định card)": func(r *displayRig) {
			r.screen.reg = dm(1920, 1080, 0)
		},
	}
	for name, setup := range cases {
		r := newDisplayRig()
		setup(r)
		want := r.screen.cur.DisplayMode
		r.ctl.startup()
		if r.screen.restores != 0 || r.screen.cur.DisplayMode != want {
			t.Errorf("%s: vẫn đụng vào máy (restore %d, chế độ %v)", name, r.screen.restores, r.screen.cur.DisplayMode)
		}
		if r.ctl.base == nil || r.ctl.base.mode != want {
			t.Errorf("%s: mốc = %+v, mong %v", name, r.ctl.base, want)
		}
	}
}

func TestDisplayStartup_KhongHoTro(t *testing.T) {
	r := newDisplayRig()
	r.screen.readErr = errDisplayUnsupported
	r.ctl.startup()
	if r.ctl.base != nil || r.screen.writes() != 0 {
		t.Errorf("không đọc được mà vẫn có mốc %+v / ghi %d", r.ctl.base, r.screen.writes())
	}
}

// --- Về chế độ cũ hỏng thì thử lại ---------------------------------------------------

// Lỗi gốc: đồng hồ hết giờ gọi setMode một lần, card đồ hoạ từ chối, và không còn gì
// hẹn lại — máy đen cho tới khi có người khởi động lại.
func TestDisplayApply_HetGioVeHongThiThuLai(t *testing.T) {
	r := newDisplayRig()
	if err := r.ctl.apply(modeA, r.onRedock); err != nil {
		t.Fatal(err)
	}
	r.screen.setModeFails = 2

	r.clock.fire(displayRevertAfter) // lần 1 hỏng
	if !r.ctl.guard.pending || r.clock.live(displayRetryAfter) != 1 {
		t.Fatalf("về hỏng mà không hẹn lại: pending=%v, đồng hồ thử lại=%d", r.ctl.guard.pending, r.clock.live(displayRetryAfter))
	}
	if r.screen.cur.DisplayMode != modeA {
		t.Fatalf("máy lẽ ra vẫn ở chế độ hỏng: %v", r.screen.cur.DisplayMode)
	}
	r.clock.fire(displayRetryAfter) // lần 2 hỏng
	r.clock.fire(displayRetryAfter) // lần 3 được
	if r.screen.cur.DisplayMode != modeGoc || r.ctl.guard.pending {
		t.Errorf("sau thử lại: máy %v, pending=%v", r.screen.cur.DisplayMode, r.ctl.guard.pending)
	}
	if r.clock.live(displayRetryAfter) != 0 {
		t.Error("về xong mà vẫn còn đồng hồ thử lại")
	}
}

// Hỏng mãi thì có giới hạn: không chạy vòng vô tận trên một card đồ hoạ đã chết.
func TestDisplayApply_ThuLaiCoGioiHan(t *testing.T) {
	r := newDisplayRig()
	if err := r.ctl.apply(modeA, r.onRedock); err != nil {
		t.Fatal(err)
	}
	r.screen.setModeErr = errors.New("Card đồ hoạ từ chối đổi sang chế độ này")

	r.clock.fire(displayRevertAfter)
	for i := 0; i < displayRetryMax+3; i++ {
		r.clock.fire(displayRetryAfter)
	}
	if r.clock.live(displayRetryAfter) != 0 || r.ctl.guard.pending {
		t.Errorf("vượt %d lần thử mà vẫn còn hẹn: pending=%v, sống=%d", displayRetryMax, r.ctl.guard.pending, r.clock.live(displayRetryAfter))
	}
	if n := len(r.clock.timers); n > 1+1+displayRetryMax+1 { // apply: hoàn tác + dán lại thanh; thử lại
		t.Errorf("tạo %d đồng hồ, nhiều hơn mức cho phép", n)
	}
}

// Huỷ tay hỏng: lỗi vẫn tới tay khách, nhưng lượt hoàn tác còn sống nên máy vẫn được về.
func TestDisplayRevert_TayHongThiVanThuLai(t *testing.T) {
	r := newDisplayRig()
	if err := r.ctl.apply(modeA, r.onRedock); err != nil {
		t.Fatal(err)
	}
	r.screen.setModeFails = 1

	if err := r.ctl.revert(r.onRedock); err == nil || !strings.Contains(err.Error(), "từ chối") {
		t.Fatalf("lỗi phải tới tay khách: %v", err)
	}
	if !r.ctl.guard.pending {
		t.Fatal("huỷ tay hỏng mà lượt hoàn tác biến mất")
	}
	r.clock.fire(displayRetryAfter)
	if r.screen.cur.DisplayMode != modeGoc || r.ctl.guard.pending {
		t.Errorf("sau thử lại: máy %v, pending=%v", r.screen.cur.DisplayMode, r.ctl.guard.pending)
	}
}

// Lượt thử lại là một lượt hoàn tác bình thường: xác nhận thì dừng, đổi tiếp thì bị thay,
// reset thì bị huỷ — đồng hồ cũ lỡ chạy muộn không kéo máy đi đâu.
func TestDisplayRetry_BiHuyKhiKhachLamViecKhac(t *testing.T) {
	hong := func() *displayRig {
		r := newDisplayRig()
		if err := r.ctl.apply(modeA, r.onRedock); err != nil {
			t.Fatal(err)
		}
		r.screen.setModeFails = 1
		r.clock.fire(displayRevertAfter)
		if r.clock.live(displayRetryAfter) != 1 {
			t.Fatal("chưa có lượt thử lại")
		}
		return r
	}

	r := hong()
	r.ctl.confirm()
	writes := r.screen.writes()
	r.clock.fireStopped(displayRetryAfter)
	if r.clock.live(displayRetryAfter) != 0 || r.screen.writes() != writes {
		t.Error("xác nhận rồi mà lượt thử lại vẫn kéo máy")
	}

	r = hong()
	if err := r.ctl.apply(modeB, r.onRedock); err != nil { // đổi tiếp khi chưa về được
		t.Fatal(err)
	}
	if r.ctl.guard.prev != modeGoc {
		t.Errorf("đổi tiếp làm mất chế độ gốc: prev = %v", r.ctl.guard.prev)
	}
	r.clock.fireStopped(displayRetryAfter) // lượt thử lại cũ chạy muộn
	if r.screen.cur.DisplayMode != modeB {
		t.Errorf("lượt thử lại cũ kéo máy khỏi chế độ mới: %v", r.screen.cur.DisplayMode)
	}

	r = hong()
	if _, err := r.ctl.reset(nil); err != nil {
		t.Fatal(err)
	}
	writes = r.screen.writes()
	r.clock.fireStopped(displayRetryAfter)
	if r.screen.writes() != writes {
		t.Error("reset rồi mà lượt thử lại vẫn kéo máy")
	}
}
