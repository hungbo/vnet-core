package main

import (
	"math"
	"testing"
	"time"
)

func TestShouldShowNativeLock(t *testing.T) {
	cases := []struct {
		ten          string
		baoTri       bool
		dockDangChay bool
		want         bool
	}{
		{"máy vừa bật, hay giao diện bật lại giữa buổi: đều dựng", false, false, true},
		{"có maintenance.flag: nhân viên đang sửa máy", true, false, false},
		{"thanh điều khiển đã chạy: bản thứ hai không được chớp màn khoá", false, true, false},
		{"cả hai cờ", true, true, false},
	}
	for _, c := range cases {
		if got := shouldShowNativeLock(c.baoTri, c.dockDangChay); got != c.want {
			t.Errorf("%s: shouldShowNativeLock(%v, %v) = %v, mong %v",
				c.ten, c.baoTri, c.dockDangChay, got, c.want)
		}
	}
}

func TestCurtainLayout(t *testing.T) {
	const m = curtainMargin
	cases := []struct {
		ten               string
		vx, vy, vw, vh    int
		pw, ph            int
		wantWin, wantText curtainRect
	}{
		{
			"một màn hình",
			0, 0, 1920, 1080, 1920, 1080,
			curtainRect{-m, -m, 1920 + m, 1080 + m},
			curtainRect{m, m, m + 1920, m + 1080},
		},
		{
			// 1920x1080 ở 150% khi tiến trình chưa khai DPI: Windows báo 1280x720.
			"tỉ lệ 150%: số đo đã co",
			0, 0, 1280, 720, 1280, 720,
			curtainRect{-m, -m, 1280 + m, 720 + m},
			curtainRect{m, m, m + 1280, m + 720},
		},
		{
			// Màn phụ 1280x1024 nằm bên TRÁI màn chính 1920x1080: gốc ảo âm.
			"màn phụ bên trái: gốc âm, chữ vẫn ở màn chính",
			-1280, 0, 3200, 1080, 1920, 1080,
			curtainRect{-1280 - m, -m, 1920 + m, 1080 + m},
			curtainRect{1280 + m, m, 1280 + m + 1920, m + 1080},
		},
		{
			"màn phụ phía trên",
			0, -1080, 1920, 2160, 1920, 1080,
			curtainRect{-m, -1080 - m, 1920 + m, 1080 + m},
			curtainRect{m, 1080 + m, m + 1920, 1080 + m + 1080},
		},
		{
			"Windows không báo màn hình ảo: lùi về màn hình chính",
			0, 0, 0, 0, 1366, 768,
			curtainRect{-m, -m, 1366 + m, 768 + m},
			curtainRect{m, m, m + 1366, m + 768},
		},
	}
	for _, c := range cases {
		win, text := curtainLayout(c.vx, c.vy, c.vw, c.vh, c.pw, c.ph)
		if win != c.wantWin {
			t.Errorf("%s: cửa sổ %+v, mong %+v", c.ten, win, c.wantWin)
		}
		if text != c.wantText {
			t.Errorf("%s: chữ %+v, mong %+v", c.ten, text, c.wantText)
		}
		// Bất biến: màn che phải phủ kín màn hình ảo, chừa lề ở mọi mép.
		if c.vw > 0 && (win.left > c.vx-m || win.top > c.vy-m || win.right < c.vx+c.vw+m || win.bottom < c.vy+c.vh+m) {
			t.Errorf("%s: cửa sổ %+v không phủ hết màn hình ảo", c.ten, win)
		}
	}
}

func TestNativeLayout(t *testing.T) {
	cases := []struct {
		ten            string
		vx, vy, vw, vh int
		pw, ph         int
		scale          float64
		gon            bool // phải co lại: bỏ dòng "Bạn đang ngồi máy"
	}{
		{"800x600", 0, 0, 800, 600, 800, 600, 1, true},
		{"1024x600", 0, 0, 1024, 600, 1024, 600, 1, true},
		{"1366x768", 0, 0, 1366, 768, 1366, 768, 1, false},
		{"1920x1080", 0, 0, 1920, 1080, 1920, 1080, 1, false},
		{"1920x1080 ở 150%", 0, 0, 1920, 1080, 1920, 1080, 1.5, false},
		{"3840x2160 ở 200%", 0, 0, 3840, 2160, 3840, 2160, 2, false},
		// Màn phụ 1280x1024 bên TRÁI màn chính: màn chính lệch hẳn sang phải trong cửa sổ.
		{"màn phụ bên trái", -1280, 0, 3200, 1080, 1920, 1080, 1, false},
	}
	type o struct {
		ten string
		r   curtainRect
	}
	for _, c := range cases {
		_, scr := curtainLayout(c.vx, c.vy, c.vw, c.vh, c.pw, c.ph)
		// Phông ô nhập cao 22 đơn vị logic (curtain_form_windows.go).
		L := nativeLayout(scr, c.scale, int(math.Round(22*c.scale)))

		if gon := L.seatLabel == (curtainRect{}); gon != c.gon {
			t.Errorf("%s: co lại = %v, mong %v", c.ten, gon, c.gon)
		}
		nam := func(ten string, r, ngoai curtainRect, tenNgoai string) {
			if r.left < ngoai.left || r.top < ngoai.top || r.right > ngoai.right || r.bottom > ngoai.bottom {
				t.Errorf("%s: %s %+v lọt ra ngoài %s %+v", c.ten, ten, r, tenNgoai, ngoai)
			}
		}
		// Từ trên xuống, không chồng lên nhau; ô rỗng (dòng đã bỏ) thì bỏ qua.
		theoThuTu := func(ds []o, ngoai curtainRect, tenNgoai string) {
			truoc := o{"mép trên", curtainRect{bottom: ngoai.top}}
			for _, x := range ds {
				if x.r == (curtainRect{}) {
					continue
				}
				if x.r.right <= x.r.left || x.r.bottom <= x.r.top {
					t.Errorf("%s: %s rỗng %+v", c.ten, x.ten, x.r)
				}
				if x.r.top < truoc.r.bottom {
					t.Errorf("%s: %s %+v chồng lên %s %+v", c.ten, x.ten, x.r, truoc.ten, truoc.r)
				}
				nam(x.ten, x.r, ngoai, tenNgoai)
				truoc = x
			}
		}
		theoThuTu([]o{{"brand", L.brand}, {"seatLabel", L.seatLabel}, {"seatName", L.seatName},
			{"clock", L.clock}, {"panel", L.panel}}, scr, "màn hình chính")
		theoThuTu([]o{{"title", L.title}, {"userLabel", L.userLabel}, {"userBox", L.userBox},
			{"passLabel", L.passLabel}, {"passBox", L.passBox}, {"button", L.button},
			{"status", L.status}}, L.panel, "khung đăng nhập")

		for _, e := range []struct {
			ten       string
			edit, box curtainRect
		}{{"userEdit", L.userEdit, L.userBox}, {"passEdit", L.passEdit, L.passBox}} {
			nam(e.ten, e.edit, e.box, "khung của nó")
			if e.edit.bottom <= e.edit.top || e.edit.right <= e.edit.left {
				t.Errorf("%s: %s rỗng %+v", c.ten, e.ten, e.edit)
			}
			if d := (e.edit.top - e.box.top) - (e.box.bottom - e.edit.bottom); d > 1 || d < -1 {
				t.Errorf("%s: %s %+v không ở giữa khung %+v theo chiều dọc", c.ten, e.ten, e.edit, e.box)
			}
		}

		if d := (L.panel.left - scr.left) - (scr.right - L.panel.right); d > 1 || d < -1 {
			t.Errorf("%s: khung đăng nhập %+v không ở giữa màn hình %+v", c.ten, L.panel, scr)
		}
	}
}

// Thấp tới mức co lại vẫn không vừa (800x600 ở 125%): xếp từ sát mép trên xuống.
func TestNativeLayoutQuaThap(t *testing.T) {
	scr := curtainRect{16, 16, 816, 616}
	L := nativeLayout(scr, 1.25, 28)
	if L.seatLabel != (curtainRect{}) {
		t.Errorf("chưa co lại: %+v", L.seatLabel)
	}
	if L.brand.top != scr.top+20 {
		t.Errorf("chữ VNET ở %d, mong sát mép trên %d", L.brand.top, scr.top+20)
	}
}

func TestNativeEnterAction(t *testing.T) {
	cases := []struct {
		ten        string
		busy       bool
		user, pass string
		want       nativeEnter
	}{
		{"đang gửi: không gửi đôi", true, "an", "123", enterIgnore},
		{"đang gửi, ô trống vẫn bỏ qua", true, "", "", enterIgnore},
		{"trống cả hai: về ô tài khoản", false, "", "", enterFocusUser},
		{"chỉ có mật khẩu: về ô tài khoản", false, "", "123", enterFocusUser},
		{"thiếu mật khẩu", false, "an", "", enterFocusPass},
		{"đủ", false, "an", "123", enterSubmit},
		{"không cắt khoảng trắng, như LockScreen.vue", false, " ", " ", enterSubmit},
	}
	for _, c := range cases {
		if got := nativeEnterAction(c.busy, c.user, c.pass); got != c.want {
			t.Errorf("%s: nativeEnterAction(%v, %q, %q) = %v, mong %v",
				c.ten, c.busy, c.user, c.pass, got, c.want)
		}
	}
}

// Thread của cửa sổ và nativeLockStart tranh nhau đúng một lần; bên thua không được
// làm gì nữa. Sai chỗ này là có lúc hai màn khoá đè nhau, hoặc không màn nào cả.
func TestNativeLockOwnership(t *testing.T) {
	t.Cleanup(func() { nativeState.Store(nlChoHien) })

	nativeState.Store(nlChoHien)
	if !nativeLockClaim() || !nativeLockOwns() {
		t.Fatal("cửa sổ hiện kịp mà không giữ được khoá")
	}
	if nativeLockAbandon() {
		t.Error("cửa sổ đã giữ khoá mà nativeLockStart vẫn thôi chờ được")
	}
	nativeLockClose()
	nativeLockClose() // gọi lại vô hại
	if nativeLockOwns() {
		t.Error("đã trả khoá mà vẫn giữ")
	}
	if nativeLockClaim() {
		t.Error("đã trả khoá mà cửa sổ giành lại được")
	}

	nativeState.Store(nlChoHien)
	if !nativeLockAbandon() {
		t.Fatal("chưa có cửa sổ mà không thôi chờ được")
	}
	if nativeLockClaim() || nativeLockOwns() {
		t.Error("cửa sổ dựng muộn vẫn giành được khoá sau khi đã thôi chờ nó")
	}

	nativeState.Store(nlChoHien)
	nativeLockClose()
	if nativeLockClaim() {
		t.Error("khoá đã trả trước khi cửa sổ kịp hiện mà nó vẫn hiện")
	}
}

// nativeLockStart chờ cửa sổ hiện. Dựng hỏng thì đi đường màn hình khoá web; dựng chậm
// (quá nativeLockReadyWait) thì KHÔNG bỏ cửa sổ: nó vẫn hiện được khi xong, cho tới khi
// Startup thôi chờ — bỏ ngay lúc hết giờ là desktop trống trơn tới tận Startup.
func TestChoNativeLockHien(t *testing.T) {
	t.Cleanup(func() { nativeState.Store(nlChoHien) })
	xong := make(chan struct{})
	close(xong)
	chuaXong := make(chan struct{})

	nativeState.Store(nlChoHien) // thread báo xong mà không giành khoá: dựng hỏng
	if choNativeLockHien(xong, time.Second) {
		t.Error("cửa sổ dựng hỏng mà vẫn đi đường màn khoá native")
	}
	if nativeLockClaim() {
		t.Error("đã đi đường màn hình khoá web mà cửa sổ native vẫn giành được khoá")
	}

	nativeState.Store(nlChoHien)
	nativeLockClaim() // hiện kịp
	if !choNativeLockHien(xong, time.Second) || !nativeLockOwns() {
		t.Error("cửa sổ hiện kịp mà không đi đường màn khoá native")
	}

	nativeState.Store(nlChoHien) // dựng chậm
	if !choNativeLockHien(chuaXong, 10*time.Millisecond) {
		t.Fatal("cửa sổ dựng chậm bị bỏ ngay lúc hết giờ chờ")
	}
	if !nativeLockClaim() || !nativeLockOwns() {
		t.Error("cửa sổ dựng xong sau giờ chờ (trước Startup) không giành được khoá")
	}
	if nativeLockAbandon() {
		t.Error("Startup thôi chờ được một cửa sổ đã hiện")
	}

	nativeState.Store(nlChoHien) // dựng chậm hơn cả Startup
	choNativeLockHien(chuaXong, 10*time.Millisecond)
	if !nativeLockAbandon() {
		t.Fatal("Startup không thôi chờ được cửa sổ chưa hiện")
	}
	if nativeLockClaim() {
		t.Error("cửa sổ xong sau Startup vẫn hiện đè lên màn hình khoá web")
	}
}

// "Đang khôi phục phiên" có hạn: trang chết giữa lúc khôi phục thì không ai báo xong,
// và ô đăng nhập native không được khoá mãi.
func TestNativeLockRestoring(t *testing.T) {
	t.Cleanup(func() { nativeLockRestoring(false) })

	nativeLockRestoring(true)
	if !nativeLockDangKhoiPhuc() {
		t.Fatal("vừa bật mà không thấy đang khôi phục")
	}
	nativeLockRestoring(false)
	if nativeLockDangKhoiPhuc() {
		t.Error("đã tắt mà vẫn thấy đang khôi phục")
	}

	nativeLockRestoring(true)
	nativeKhoiPhucDen.Store(int64(time.Since(processStart) - time.Second)) // quá hạn
	if nativeLockDangKhoiPhuc() {
		t.Error("quá nativeKhoiPhucToiDa mà ô đăng nhập vẫn bị khoá")
	}
}
