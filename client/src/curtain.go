package main

import (
	"log"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// Màn khoá native lúc khởi động.
//
// Máy trạm không ổ cứng: tiến trình giao diện bật lên ở giây ~35 sau khi máy khởi
// động, nhưng màn hình khoá web chỉ dùng được ở giây ~49 — WebView2 khởi động lạnh từ
// ổ đĩa mạng mất chừng 14 giây. Màn khoá native là một cửa sổ Win32 dựng ngay đầu
// runDock, trước cả khi Wails khởi tạo: phủ kín mọi màn hình như màn che cũ, và có sẵn
// ô đăng nhập để khách vào máy ngay chứ không phải chờ WebView2. WebView2 vẫn nạp sau
// lưng nó.
//
// Nó GIỮ khoá từ lúc giao diện bật tới lần mở khoá thành công đầu tiên, kiểu gì cũng
// được: đăng nhập ngay trên nó, tài khoản quản trị máy trạm, hoặc web khôi phục được
// phiên cũ. Trả khoá rồi thì mọi thứ là web: thanh điều khiển, và cả các lần khoá sau
// (đăng xuất, hết giờ, mất kết nối) — lúc đó WebView2 đã nạp xong nên màn hình khoá web
// hiện ngay, mà lý do khoá cùng phần khôi phục phiên cũng chỉ có ở web.
//
// CHỈ thanh điều khiển chính (runDock) dùng nó: cửa sổ phụ và dịch vụ nền thì không
// bao giờ.
//
// Phần Win32 ở curtain_windows.go (cửa sổ, thread, giữ bàn phím) và
// curtain_form_windows.go (ô nhập, vẽ); tệp này giữ quyết định, bố cục và trạng thái
// giữ khoá.

const (
	// Máy đã chạy quá chừng này thì không còn là lúc "vừa bật máy" (agent.go,
	// prefetch.go).
	curtainMaxUptime = 5 * time.Minute
	// Màn khoá rộng hơn màn hình mỗi mép chừng này: trên Windows cũ thread của nó
	// không khai được DPI nên Windows co giãn cửa sổ theo tỉ lệ (xem
	// curtain_windows.go), mà tỉ lệ lẻ như 175% làm tròn hụt một pixel ở mép phải/dưới.
	curtainMargin = 16
	// Thanh taskbar cũng là cửa sổ "luôn nổi" và explorer.exe dựng nó SAU màn khoá
	// (giao diện được bật ngay khi phiên có token, trước khi explorer chạy), nên nó nằm
	// trên màn khoá và khách thấy nút Start. Cứ chừng này màn khoá lại tự nâng mình lên
	// đỉnh nhóm cửa sổ nổi và giành lại bàn phím, chừng nào còn giữ khoá.
	curtainRaiseEvery = 150 * time.Millisecond
	// nativeLockStart chờ cửa sổ hiện tối đa chừng này rồi đi tiếp, không kẹt ở đây.
	// Dựng hỏng thì dùng màn hình khoá web như trước. Còn đang dựng (máy không ổ cứng
	// khởi động nặng) thì KHÔNG bỏ nó: nó hiện lúc nào xong lúc đó, và chỉ Startup mới
	// thôi chờ nó — bỏ ngay ở đây là desktop trống trơn tới tận Startup.
	nativeLockReadyWait = 2 * time.Second
)

// processStart là mốc đo thời gian cho nhật ký: khởi tạo cùng gói nên gần đúng
// lúc tiến trình bắt đầu.
var processStart = time.Now()

// Trạng thái giữ khoá của màn khoá native, chỉ đi tới:
//
//	nlChoHien → nlGiuKhoa → nlDaTra   cửa sổ hiện kịp, về sau được trả khoá
//	nlChoHien → nlBoLo                dựng hỏng, hoặc tới Startup vẫn chưa hiện: không bao giờ hiện
//
// Ở nlChoHien hai bên tranh nhau — thread của cửa sổ sắp hiện nó, Startup thì thôi chờ
// (nativeLockStart thôi chờ chỉ khi dựng hỏng) — và CompareAndSwap cho đúng một bên
// thắng: đã thôi chờ thì không có cửa sổ nào hiện lên muộn để đè lên màn hình khoá web.
const (
	nlChoHien int32 = iota
	nlGiuKhoa
	nlBoLo
	nlDaTra
)

var nativeState atomic.Int32

// nativeLockStart là việc giao diện đầu tiên của runDock, ngay sau NewApp và trước
// wails.Run: mỗi mili giây đi trước nó là một mili giây khách còn thấy desktop.
//
// Trả true khi màn khoá native đã hiện và đang giữ khoá, HOẶC còn đang dựng sau
// nativeLockReadyWait (choNativeLockHien); false thì không bao giờ có cửa sổ native nào
// hiện. submit chạy trên goroutine riêng (không bao giờ trên thread của cửa sổ), mỗi
// lúc một lượt: trả nil nghĩa là máy đã mở VÀ nativeLockClose đã được gọi; trả lỗi thì
// câu lỗi hiện nguyên văn dưới nút, ô mật khẩu được xoá để gõ lại.
func nativeLockStart(machineCode string, submit func(username, password string) error) bool {
	log.Printf("[ui] khởi động, máy đã chạy %ds", int(systemUptime().Seconds()))
	bootRec.giaoDienBatDau(systemUptime() - time.Since(processStart))
	if !shouldShowNativeLock(maintenanceMode(), dockAlreadyRunning()) {
		return false
	}
	ready := make(chan struct{})
	if !startNativeLockThread(machineCode, submit, sync.OnceFunc(func() { close(ready) })) {
		return false
	}
	return choNativeLockHien(ready, nativeLockReadyWait)
}

// choNativeLockHien chờ thread của màn khoá báo xong (ready: đã hiện, hoặc dựng hỏng),
// tối đa cho.
//
// Dựng hỏng thì thôi hẳn: false, giao diện đi đường màn hình khoá web. Quá hạn mà còn
// đang dựng thì vẫn trả true, KHÔNG thôi chờ: cửa sổ Wails dựng ẩn và hook chặn phím bật
// ngay (runDock) như khi nó đã hiện, còn nó hiện lúc nào xong lúc đó. Màn che cũ cũng
// hiện muộn như thế; bỏ nó ở đây thì từ giây thứ hai tới Startup không có gì che desktop.
// Tới Startup mà vẫn chưa hiện thì Startup thôi chờ (nativeLockAbandon) và phủ màn hình
// bằng cửa sổ Wails.
func choNativeLockHien(ready <-chan struct{}, cho time.Duration) bool {
	select {
	case <-ready:
		if nativeLockAbandon() {
			log.Printf("[ui] màn khoá native không dựng được — dùng màn hình khoá web")
			return false
		}
	case <-time.After(cho):
		log.Printf("[ui] màn khoá native chưa hiện sau %v — vẫn chờ nó tới Startup", cho)
	}
	return true
}

// shouldShowNativeLock: có dựng màn khoá native không.
//
//	maintenanceFlag:    có tệp maintenance.flag cạnh .exe (nhân viên kỹ thuật đang sửa máy).
//	dockAlreadyRunning: thanh điều khiển đã chạy trong phiên này. Bản chạy thứ hai
//	                    sẽ gõ cửa bản đầu rồi thoát ngay — dựng màn khoá chỉ để nó
//	                    chớp lên một cái.
//
// KHÔNG xét máy đã chạy bao lâu như màn che cũ: giao diện bật lại giữa buổi thì màn hình
// vẫn bị phủ kín cho tới khi khôi phục xong phiên (trước đây bằng màn hình khoá web), nên
// chừa lúc đó ra chẳng giữ được gì cho khách đang chơi. Trong quãng đó màn khoá native
// báo "Đang khôi phục phiên…" và không nhận Vào máy (nativeLockRestoring).
//
// Hàm thuần để kiểm bằng bảng; việc "chỉ trên Windows" nằm ở startNativeLockThread.
func shouldShowNativeLock(maintenanceFlag, dockAlreadyRunning bool) bool {
	return !maintenanceFlag && !dockAlreadyRunning
}

// nativeLockOwns: màn khoá native đang giữ khoá — từ lúc cửa sổ của nó hiện
// (nativeLockClaim) tới lần nativeLockClose đầu tiên. Không khoá, gọi từ đâu cũng được.
func nativeLockOwns() bool { return nativeState.Load() == nlGiuKhoa }

// nativeLockClaim: thread của cửa sổ gọi khi cửa sổ đã dựng xong, ngay TRƯỚC khi hiện
// nó. false nghĩa là đã thôi chờ nó (hoặc khoá đã được trả): huỷ cửa sổ, không được
// hiện.
func nativeLockClaim() bool { return nativeState.CompareAndSwap(nlChoHien, nlGiuKhoa) }

// nativeLockAbandon: thôi chờ cửa sổ — nativeLockStart khi nó dựng hỏng, Startup khi nó
// vẫn chưa hiện. false nghĩa là cửa sổ đã kịp giành khoá (hoặc đã thôi chờ từ trước).
func nativeLockAbandon() bool { return nativeState.CompareAndSwap(nlChoHien, nlBoLo) }

// nativeLockClose trả khoá: thôi giữ khoá NGAY, nên nhịp nâng/giành bàn phím kế tiếp tự
// dừng, rồi nhắn thread của cửa sổ tự đóng nó. Gọi bao nhiêu lần, từ goroutine nào cũng
// được; chưa từng hiện thì không làm gì.
func nativeLockClose() {
	nativeLockRelease()
	postNativeLockClose()
}

// nativeLockRelease thôi giữ khoá mà CHƯA đóng cửa sổ: nó vẫn phủ màn hình nhưng không
// nâng mình lên, không giành bàn phím nữa, cho tới nativeLockClose. SetLoggedIn dùng quãng
// đó để hiện thanh điều khiển lên trên nó trước khi nó đóng.
func nativeLockRelease() { nativeState.Store(nlDaTra) }

// Giao diện web đang khôi phục phiên cũ (giao diện bật lại giữa buổi, RestoreSession):
// màn khoá native báo "Đang khôi phục phiên…" và không nhận Vào máy, để không có lần
// đăng nhập thứ hai giành máy với phiên đang khôi phục. Giữ MỐC HẾT HẠN (tính từ
// processStart, đồng hồ đơn điệu) chứ không giữ cờ: trang chết giữa chừng thì không ai
// báo xong, mà ô đăng nhập thì không được khoá mãi.
var nativeKhoiPhucDen atomic.Int64

// nativeKhoiPhucToiDa: lâu nhất một lần khôi phục của web — RestoreSession rồi GetSession,
// mỗi bước tối đa HAN_KHOI_PHUC_MS (20 giây, session.store.ts) — cộng chút dư.
const nativeKhoiPhucToiDa = 45 * time.Second

// nativeLockRestoring bật/tắt trạng thái "đang khôi phục phiên". Gọi từ đâu cũng được;
// màn khoá native đọc ở nhịp kế tiếp.
func nativeLockRestoring(on bool) {
	var den time.Duration
	if on {
		den = time.Since(processStart) + nativeKhoiPhucToiDa
	}
	nativeKhoiPhucDen.Store(int64(den))
}

func nativeLockDangKhoiPhuc() bool {
	return int64(time.Since(processStart)) < nativeKhoiPhucDen.Load()
}

// nativeLockFocus đưa màn khoá native lên trên và trả bàn phím về ô vừa gõ (lối tắt
// VNET bấm lần nữa). Chỉ nhắn tin chứ không chờ, nên gọi được cả từ luồng chính của
// Wails.
func nativeLockFocus() {
	if nativeLockOwns() {
		postNativeLockFocus()
	}
}

var domReadyLogged sync.Once

// logDomReady ghi mốc DOM sẵn sàng, một lần cho cả tiến trình: OnDomReady chạy lại ở
// mỗi lần nạp trang.
func logDomReady() {
	domReadyLogged.Do(func() {
		log.Printf("[ui] DOM sẵn sàng sau %dms", time.Since(processStart).Milliseconds())
		bootRec.domSan(systemUptime())
	})
}

// nativeEnter là việc phải làm khi khách bấm Enter hoặc nút "Vào máy".
type nativeEnter int

const (
	enterIgnore    nativeEnter = iota // đang gửi (không gửi đôi) hoặc đang khôi phục phiên: bỏ qua
	enterFocusUser                    // chưa gõ tài khoản
	enterFocusPass                    // chưa gõ mật khẩu
	enterSubmit
)

// nativeEnterAction: thiếu ô nào thì đưa con trỏ về ô đó. Không cắt khoảng trắng, như
// LockScreen.vue: máy chủ mới là nơi quyết tài khoản nào đúng.
func nativeEnterAction(busy bool, user, pass string) nativeEnter {
	switch {
	case busy:
		return enterIgnore
	case user == "":
		return enterFocusUser
	case pass == "":
		return enterFocusPass
	}
	return enterSubmit
}

// nativeFormLayout: chỗ của từng phần trên màn khoá native, theo toạ độ CỬA SỔ. Các ô
// *Edit là control EDIT thật, nằm trong khung *Box vẽ tay.
type nativeFormLayout struct {
	brand, seatLabel, seatName, clock curtainRect
	panel, title                      curtainRect
	userLabel, userBox, userEdit      curtainRect
	passLabel, passBox, passEdit      curtainRect
	button, status                    curtainRect
}

// nativeLayout xếp màn khoá native thành một cột giữa màn hình chính: chữ VNET, "Bạn
// đang ngồi máy", tên máy, đồng hồ, rồi khung đăng nhập.
//
//	scr:   màn hình chính, toạ độ cửa sổ (text của curtainLayout).
//	scale: DPI/96 của thread vẽ (1 khi không khai DPI). Mọi số đo dưới đây là đơn vị
//	       logic, nhân với nó.
//	lineH: chiều cao dòng (tmHeight) của phông ô nhập, đo trên Windows. Ô nhập cao
//	       đúng một dòng và đặt giữa khung: EDIT một dòng luôn vẽ chữ sát mép trên,
//	       không tự căn giữa.
//
// Màn thấp (1024x600, 800x600) không đủ chỗ thì co đôi các khoảng cách và bỏ dòng "Bạn
// đang ngồi máy"; vẫn không đủ thì xếp từ sát mép trên xuống.
func nativeLayout(scr curtainRect, scale float64, lineH int) nativeFormLayout {
	px := func(v float64) int { return int(math.Round(v * scale)) }
	scrW, scrH := scr.right-scr.left, scr.bottom-scr.top

	seatW := min(scrW-px(64), px(720))
	seatX := scr.left + (scrW-seatW)/2
	panelW := min(px(400), scrW-px(32))
	panelX := scr.left + (scrW-panelW)/2
	pad := px(24)

	// xep xếp cả cột từ y trở xuống, trả bố cục và mép dưới. g nhân vào mọi khoảng cách.
	xep := func(y int, g float64, coNhan bool) (L nativeFormLayout, day int) {
		hang := func(x, w int, cao float64) curtainRect {
			r := curtainRect{x, y, x + w, y + px(cao)}
			y = r.bottom
			return r
		}
		cach := func(v float64) { y += px(v * g) }
		cot := func(cao float64) curtainRect { return hang(seatX, seatW, cao) }
		trong := func(cao float64) curtainRect { return hang(panelX+pad, panelW-2*pad, cao) }

		L.brand = cot(56)
		cach(24)
		if coNhan {
			L.seatLabel = cot(22)
			cach(4)
		}
		L.seatName = cot(60)
		cach(8)
		L.clock = cot(40)
		cach(24)

		panelTop := y
		y += pad
		L.title = trong(30)
		cach(12)
		L.userLabel = trong(20)
		cach(6)
		L.userBox = trong(44)
		cach(12)
		L.passLabel = trong(20)
		cach(6)
		L.passBox = trong(44)
		cach(20)
		L.button = trong(46)
		cach(8)
		L.status = trong(60) // ba dòng chữ cao 20: câu lỗi dài của máy chủ vẫn đọc hết
		y += pad
		L.panel = curtainRect{panelX, panelTop, panelX + panelW, y}
		return L, y
	}

	gioiHan := scrH - px(32)
	g, coNhan := 1.0, true
	if _, cao := xep(0, g, coNhan); cao > gioiHan {
		g, coNhan = 0.5, false
	}
	_, cao := xep(0, g, coNhan)
	y := scr.top + (scrH-cao)/2
	if cao > gioiHan {
		y = scr.top + px(16)
	}
	L, _ := xep(y, g, coNhan)

	// Ô nhập: thụt vào mỗi bên, cao một dòng, giữa khung theo chiều dọc.
	oNhap := func(box curtainRect) curtainRect {
		cao := box.bottom - box.top
		h := lineH
		if h <= 0 || h > cao-4 {
			h = cao - 4
		}
		top := box.top + (cao-h)/2
		return curtainRect{box.left + px(12), top, box.right - px(12), top + h}
	}
	L.userEdit = oNhap(L.userBox)
	L.passEdit = oNhap(L.passBox)
	return L
}

// ghiMocNapTrang ghi các mốc nạp trang do frontend gửi (main.ts, sự kiện vnet:ui:moc).
// data[0] là performance.now() lúc gửi, tức số mili giây kể từ lúc trang bắt đầu nạp,
// nên lúc trang bắt đầu nạp = bây giờ trừ đi chừng đó. Tính bằng đồng hồ đơn điệu của
// tiến trình chứ không bằng giờ hệ thống: máy không ổ cứng hay bị chỉnh giờ theo máy
// chủ ngay lúc khởi động. data[1] là chuỗi các mốc tính từ lúc trang bắt đầu nạp; cắt
// ngắn và đặt trong ngoặc kép vì trang nào cũng gửi được sự kiện này.
func ghiMocNapTrang(data ...interface{}) {
	if len(data) < 2 {
		return
	}
	now, ok := data[0].(float64)
	chiTiet, ok2 := data[1].(string)
	if !ok || !ok2 {
		return
	}
	log.Printf("[ui] trang bắt đầu nạp sau %dms; tính từ đó: %.400q",
		time.Since(processStart).Milliseconds()-int64(now), chiTiet)
	bootRec.trang(chiTiet)
}

// curtainRect là hình chữ nhật theo toạ độ mép, cùng nghĩa với RECT của Win32.
type curtainRect struct {
	left, top, right, bottom int
}

// curtainLayout tính chỗ đặt màn che và chỗ vẽ chữ.
//
//	vx, vy, vw, vh: màn hình ẢO (gộp mọi màn hình); gốc có thể ÂM khi màn phụ
//	                nằm bên trái hoặc phía trên màn chính.
//	pw, ph:         kích thước màn hình chính.
//
// win là toạ độ màn hình của cửa sổ; text là chỗ vẽ chữ, tính theo toạ độ CỬA SỔ
// và khớp màn hình chính — căn giữa cả màn hình ảo thì chữ rơi vào khe giữa hai
// màn hình.
func curtainLayout(vx, vy, vw, vh, pw, ph int) (win, text curtainRect) {
	if vw <= 0 || vh <= 0 {
		vx, vy, vw, vh = 0, 0, pw, ph
	}
	win = curtainRect{
		left:   vx - curtainMargin,
		top:    vy - curtainMargin,
		right:  vx + vw + curtainMargin,
		bottom: vy + vh + curtainMargin,
	}
	// Màn hình chính luôn nằm ở (0,0) của toạ độ màn hình ảo.
	text = curtainRect{
		left:   -win.left,
		top:    -win.top,
		right:  -win.left + pw,
		bottom: -win.top + ph,
	}
	return win, text
}
