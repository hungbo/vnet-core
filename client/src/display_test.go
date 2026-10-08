package main

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func dm(w, h, hz int) DisplayMode { return DisplayMode{Width: w, Height: h, Hz: hz} }

// raw32 là dòng thô "bình thường": 32 bit, quét liên tục, hướng mặc định.
func raw32(w, h, hz int) rawDisplayMode {
	return rawDisplayMode{DisplayMode: dm(w, h, hz), BitsPerPel: 32}
}

func TestFilterDisplayModes(t *testing.T) {
	mau16 := raw32(1600, 900, 60)
	mau16.BitsPerPel = 16
	xenKe := raw32(1920, 1080, 30)
	xenKe.Interlaced = true
	xoay := raw32(1080, 1920, 60)
	xoay.Orientation = 1

	cases := []struct {
		ten         string
		raw         []rawDisplayMode
		orientation int
		want        []DisplayMode
	}{
		{
			"đủ cả năm tiêu chí: màu, xen kẽ, hướng xoay, cỡ tối thiểu, trùng",
			[]rawDisplayMode{
				raw32(1920, 1080, 60), raw32(1920, 1080, 144), raw32(1920, 1080, 144), // trùng (khác kiểu co giãn)
				raw32(1280, 720, 60), raw32(800, 600, 60), raw32(1024, 768, 75),
				raw32(640, 480, 60),   // dưới 800x600
				raw32(1280, 512, 60),  // đủ rộng nhưng thấp hơn 600
				raw32(700, 900, 60),   // cao nhưng hẹp hơn 800
				raw32(2560, 1440, 60), // xếp lên đầu
				mau16, xenKe, xoay,
			},
			0,
			[]DisplayMode{dm(2560, 1440, 60), dm(1920, 1080, 144), dm(1920, 1080, 60), dm(1280, 720, 60), dm(1024, 768, 75), dm(800, 600, 60)},
		},
		{
			"đúng 800x600 vẫn giữ",
			[]rawDisplayMode{raw32(800, 600, 60)},
			0,
			[]DisplayMode{dm(800, 600, 60)},
		},
		{
			"màn đang xoay dọc: chỉ giữ chế độ cùng hướng xoay",
			[]rawDisplayMode{raw32(1920, 1080, 60), xoay},
			1,
			[]DisplayMode{dm(1080, 1920, 60)},
		},
		{
			"thứ tự: rộng, rồi cao, rồi tần số",
			[]rawDisplayMode{raw32(1920, 1080, 60), raw32(1920, 1200, 60), raw32(1920, 1080, 144), raw32(1680, 1050, 60)},
			0,
			[]DisplayMode{dm(1920, 1200, 60), dm(1920, 1080, 144), dm(1920, 1080, 60), dm(1680, 1050, 60)},
		},
		{
			"không còn gì: mảng rỗng chứ không phải nil (JSON ra [] chứ không phải null)",
			[]rawDisplayMode{raw32(640, 480, 60), mau16},
			0,
			[]DisplayMode{},
		},
	}
	for _, c := range cases {
		got := filterDisplayModes(c.raw, c.orientation)
		if got == nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s:\n  got  %v\n  want %v", c.ten, got, c.want)
		}
	}
}

func TestValidateDisplayMode(t *testing.T) {
	list := []DisplayMode{dm(1920, 1080, 144), dm(1920, 1080, 60), dm(1280, 720, 60)}
	cases := []struct {
		ten  string
		list []DisplayMode
		m    DisplayMode
		ok   bool
	}{
		{"có trong danh sách", list, dm(1920, 1080, 144), true},
		{"phần tử cuối", list, dm(1280, 720, 60), true},
		{"đúng độ phân giải, sai tần số", list, dm(1920, 1080, 120), false},
		{"độ phân giải lạ", list, dm(1234, 567, 60), false},
		{"giá trị không", list, DisplayMode{}, false},
		{"danh sách rỗng", nil, dm(1920, 1080, 60), false},
	}
	for _, c := range cases {
		err := validateDisplayMode(c.list, c.m)
		if (err == nil) != c.ok {
			t.Errorf("%s: validateDisplayMode(%v) = %v, mong ok=%v", c.ten, c.m, err, c.ok)
		}
		if err != nil && !strings.Contains(err.Error(), c.m.String()) {
			t.Errorf("%s: lỗi %q không nêu chế độ %s", c.ten, err, c.m)
		}
	}
}

func TestClampMouseSpeed(t *testing.T) {
	cases := []struct{ in, want int }{
		{-5, 1}, {0, 1}, {1, 1}, {10, 10}, {20, 20}, {21, 20}, {1000, 20},
	}
	for _, c := range cases {
		if got := clampMouseSpeed(c.in); got != c.want {
			t.Errorf("clampMouseSpeed(%d) = %d, mong %d", c.in, got, c.want)
		}
	}
}

func TestWithMousePrecision(t *testing.T) {
	cases := []struct {
		ten  string
		in   [3]int32
		on   bool
		want [3]int32
	}{
		{"bật từ trạng thái tắt hẳn 0,0,0: đặt ngưỡng mặc định", [3]int32{0, 0, 0}, true, [3]int32{6, 10, 1}},
		{"bật khi đã bật: không đổi gì", [3]int32{6, 10, 1}, true, [3]int32{6, 10, 1}},
		{"bật: ngưỡng người dùng tự chỉnh được giữ", [3]int32{4, 8, 0}, true, [3]int32{4, 8, 1}},
		{"bật: chỉ ngưỡng nào bằng 0 mới bị thay", [3]int32{3, 0, 0}, true, [3]int32{3, 10, 1}},
		{"bật: ngưỡng một bằng 0", [3]int32{0, 10, 0}, true, [3]int32{6, 10, 1}},
		{"bật khi đang ở mức 2 của Windows cũ: giữ mức 2", [3]int32{6, 10, 2}, true, [3]int32{6, 10, 2}},
		{"tắt: giữ nguyên hai ngưỡng", [3]int32{6, 10, 1}, false, [3]int32{6, 10, 0}},
		{"tắt khi đã tắt", [3]int32{0, 0, 0}, false, [3]int32{0, 0, 0}},
	}
	for _, c := range cases {
		if got := withMousePrecision(c.in, c.on); got != c.want {
			t.Errorf("%s: withMousePrecision(%v, %v) = %v, mong %v", c.ten, c.in, c.on, got, c.want)
		}
		if got := mousePrecisionOn(c.want); got != c.on {
			t.Errorf("%s: mousePrecisionOn(%v) = %v, mong %v", c.ten, c.want, got, c.on)
		}
	}
}

func TestDisplayChangeError(t *testing.T) {
	if err := displayChangeError(0); err != nil {
		t.Errorf("DISP_CHANGE_SUCCESSFUL phải là nil, được %v", err)
	}

	// Mỗi mã lỗi một câu riêng, đều là tiếng Việt: khách đọc câu này trên màn hình.
	seen := map[string]int32{}
	for _, code := range []int32{1, -1, -2, -3, -4, -5, -6} {
		err := displayChangeError(code)
		if err == nil || err.Error() == "" {
			t.Fatalf("mã %d không ra lỗi", code)
		}
		if other, dup := seen[err.Error()]; dup {
			t.Errorf("mã %d và %d dùng chung câu %q", code, other, err)
		}
		seen[err.Error()] = code
	}
	if err := displayChangeError(1); !strings.Contains(err.Error(), "khởi động lại") {
		t.Errorf("DISP_CHANGE_RESTART phải nói rõ cần khởi động lại: %q", err)
	}
	if err := displayChangeError(-2); !strings.Contains(err.Error(), "không hỗ trợ") {
		t.Errorf("DISP_CHANGE_BADMODE phải nói rõ không hỗ trợ: %q", err)
	}
	if err := displayChangeError(42); err == nil || !strings.Contains(err.Error(), "42") {
		t.Errorf("mã lạ phải nêu mã trong câu lỗi: %v", err)
	}
}

func TestUnsupportedDisplayInfo_JSON(t *testing.T) {
	got := unsupportedDisplayInfo().json()
	// "modes" là [] chứ không phải null; chuột có giá trị mặc định.
	for _, want := range []string{`"supported":false`, `"modes":[]`, `"mouse_speed":10`, `"mouse_precision":false`} {
		if !strings.Contains(got, want) {
			t.Errorf("thiếu %s trong %s", want, got)
		}
	}
}

// --- đồng hồ giả ---------------------------------------------------------------

type fakeTimer struct {
	d       time.Duration
	f       func()
	stopped bool
	fired   bool
}

// fakeClock thay time.AfterFunc: bài kiểm tự quyết lúc nào đồng hồ "hết giờ".
type fakeClock struct{ timers []*fakeTimer }

func (c *fakeClock) after(d time.Duration, f func()) func() bool {
	t := &fakeTimer{d: d, f: f}
	c.timers = append(c.timers, t)
	return func() bool {
		conSong := !t.stopped && !t.fired
		t.stopped = true
		return conSong
	}
}

// live: số đồng hồ cỡ d chưa chạy và chưa bị dừng.
func (c *fakeClock) live(d time.Duration) int {
	n := 0
	for _, t := range c.timers {
		if t.d == d && !t.stopped && !t.fired {
			n++
		}
	}
	return n
}

// fire cho các đồng hồ cỡ d còn sống hết giờ. Trả số đồng hồ đã chạy.
func (c *fakeClock) fire(d time.Duration) int {
	return c.run(d, false)
}

// fireStopped chạy cả những đồng hồ ĐÃ BỊ DỪNG: mô phỏng hàm hẹn giờ đã khởi động
// trên goroutine của nó ngay trước khi Stop() được gọi — Stop() trả false mà hàm vẫn chạy.
func (c *fakeClock) fireStopped(d time.Duration) int {
	return c.run(d, true)
}

func (c *fakeClock) run(d time.Duration, includeStopped bool) int {
	n := 0
	// Chép danh sách trước: hàm chạy có thể hẹn thêm đồng hồ mới.
	for _, t := range append([]*fakeTimer(nil), c.timers...) {
		if t.d != d || t.fired || (t.stopped && !includeStopped) {
			continue
		}
		t.fired = true
		t.f()
		n++
	}
	return n
}

// --- revertGuard -----------------------------------------------------------------

type guardRig struct {
	clock *fakeClock
	g     *revertGuard
	fired []uint64 // số lượt mà onExpire đã nhận
}

func newGuardRig() *guardRig {
	c := &fakeClock{}
	return &guardRig{clock: c, g: &revertGuard{after: c.after, timeout: displayRevertAfter}}
}

func (r *guardRig) arm(prev DisplayMode) {
	r.g.arm(prev, func(gen uint64) { r.fired = append(r.fired, gen) })
}

func TestRevertGuard_ArmRoiHetGioThiVe(t *testing.T) {
	r := newGuardRig()
	r.arm(dm(1920, 1080, 60))
	if !r.g.pending || r.clock.live(displayRevertAfter) != 1 {
		t.Fatalf("sau arm: pending=%v, đồng hồ 20s còn sống=%d", r.g.pending, r.clock.live(displayRevertAfter))
	}

	if n := r.clock.fire(displayRevertAfter); n != 1 || len(r.fired) != 1 {
		t.Fatalf("đồng hồ phải chạy đúng một lần: n=%d fired=%v", n, r.fired)
	}
	prev, ok := r.g.expire(r.fired[0])
	if !ok || prev != dm(1920, 1080, 60) {
		t.Fatalf("hết giờ phải trả chế độ cũ: %v, %v", prev, ok)
	}
	if r.g.pending {
		t.Error("về xong mà vẫn còn pending")
	}
	if _, ok := r.g.expire(r.fired[0]); ok {
		t.Error("cùng một lượt hết giờ mà về được hai lần")
	}
}

func TestRevertGuard_XacNhanThiHuyDongHo(t *testing.T) {
	r := newGuardRig()
	r.arm(dm(1920, 1080, 60))
	r.g.clear()
	if r.g.pending || r.clock.live(displayRevertAfter) != 0 {
		t.Fatalf("sau xác nhận: pending=%v, đồng hồ còn sống=%d", r.g.pending, r.clock.live(displayRevertAfter))
	}

	// Hàm hẹn giờ đã lỡ chạy (Stop không kịp) cũng phải vô hiệu.
	if n := r.clock.fireStopped(displayRevertAfter); n != 1 {
		t.Fatalf("mô phỏng đồng hồ chạy muộn: n=%d", n)
	}
	if _, ok := r.g.expire(r.fired[0]); ok {
		t.Error("đồng hồ chạy muộn sau xác nhận vẫn kéo máy về chế độ cũ")
	}
}

func TestRevertGuard_ArmLanHaiGiuCheDoGoc(t *testing.T) {
	r := newGuardRig()
	goc, thu1 := dm(1920, 1080, 144), dm(1280, 720, 60)
	r.arm(goc)  // khách đổi 1920x1080 -> 1280x720
	r.arm(thu1) // chưa xác nhận đã đổi tiếp sang 1600x900: "trước đó" lúc này là 1280x720

	if r.g.prev != goc {
		t.Fatalf("lần arm thứ hai ghi đè chế độ gốc: prev = %v, mong %v", r.g.prev, goc)
	}
	if r.clock.live(displayRevertAfter) != 1 {
		t.Errorf("đồng hồ cũ phải bị dừng, chỉ còn một: còn sống %d", r.clock.live(displayRevertAfter))
	}
	// Đồng hồ của lần arm thứ nhất có chạy muộn cũng không được về.
	r.clock.fireStopped(displayRevertAfter) // chạy cả hai
	if len(r.fired) != 2 {
		t.Fatalf("mong hai lần gọi onExpire, được %v", r.fired)
	}
	if _, ok := r.g.expire(r.fired[0]); ok {
		t.Error("lượt cũ được về dù đã có lượt mới")
	}
	prev, ok := r.g.expire(r.fired[1])
	if !ok || prev != goc {
		t.Errorf("lượt mới phải về chế độ GỐC %v, được %v (ok=%v)", goc, prev, ok)
	}
}

func TestRevertGuard_Take(t *testing.T) {
	r := newGuardRig()
	if _, ok := r.g.take(); ok {
		t.Error("chưa arm mà take có kết quả")
	}
	r.arm(dm(1280, 720, 60))
	prev, ok := r.g.take()
	if !ok || prev != dm(1280, 720, 60) {
		t.Fatalf("take = %v, %v", prev, ok)
	}
	if r.g.pending || r.clock.live(displayRevertAfter) != 0 {
		t.Error("take xong phải dừng đồng hồ")
	}
	if _, ok := r.g.take(); ok {
		t.Error("take hai lần")
	}
}

func TestRevertGuard_ArmLaiSauKhiXong(t *testing.T) {
	r := newGuardRig()
	r.arm(dm(1920, 1080, 60))
	r.g.clear()
	r.arm(dm(1280, 720, 60)) // lần đổi mới SAU khi đã xác nhận: chế độ gốc là cái vừa xác nhận
	if r.g.prev != dm(1280, 720, 60) {
		t.Errorf("prev = %v, mong 1280x720 (không mang chế độ gốc của lượt trước sang)", r.g.prev)
	}
}

func TestRevertGuard_Rearm(t *testing.T) {
	r := newGuardRig()
	r.arm(dm(1920, 1080, 60))
	r.clock.fire(displayRevertAfter)
	prev, ok := r.g.expire(r.fired[0])
	if !ok {
		t.Fatal("expire không trả chế độ cũ")
	}

	// Lần về hỏng: hẹn lại, vẫn đang chờ, ngắn hơn lần đầu, mang đúng chế độ cũ.
	r.g.rearm(prev, displayRetryAfter, func(gen uint64) { r.fired = append(r.fired, gen) })
	if !r.g.pending || r.g.prev != prev || r.clock.live(displayRetryAfter) != 1 {
		t.Fatalf("sau rearm: pending=%v, prev=%v, đồng hồ=%d", r.g.pending, r.g.prev, r.clock.live(displayRetryAfter))
	}
	r.clock.fire(displayRetryAfter)
	if p, ok := r.g.expire(r.fired[len(r.fired)-1]); !ok || p != prev {
		t.Errorf("lượt thử lại hết giờ phải về %v: %v, %v", prev, p, ok)
	}

	// Xác nhận chặn được lượt thử lại như mọi lượt khác.
	r.g.rearm(prev, displayRetryAfter, func(gen uint64) { r.fired = append(r.fired, gen) })
	r.g.clear()
	r.clock.fireStopped(displayRetryAfter)
	if _, ok := r.g.expire(r.fired[len(r.fired)-1]); ok {
		t.Error("lượt thử lại chạy muộn sau khi xác nhận vẫn về được")
	}
}

func TestStaleDisplayMode(t *testing.T) {
	cases := []struct {
		name     string
		reg, cur DisplayMode
		want     bool
	}{
		{"giống hệt", dm(1920, 1080, 60), dm(1920, 1080, 60), false},
		{"lệch độ phân giải", dm(1920, 1080, 60), dm(1280, 720, 60), true},
		{"lệch tần số", dm(1920, 1080, 60), dm(1920, 1080, 144), true},
		{"registry tần số 0 là mặc định card", dm(1920, 1080, 0), dm(1920, 1080, 144), false},
		{"registry tần số 1 là mặc định card", dm(1920, 1080, 1), dm(1920, 1080, 144), false},
		{"registry rỗng", DisplayMode{}, dm(1280, 720, 60), false},
		{"registry dưới cỡ tối thiểu", dm(640, 480, 60), dm(1920, 1080, 60), false},
	}
	for _, c := range cases {
		if got := staleDisplayMode(c.reg, c.cur); got != c.want {
			t.Errorf("%s: staleDisplayMode(%v, %v) = %v, mong %v", c.name, c.reg, c.cur, got, c.want)
		}
	}
}
