package main

import (
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Màn che lúc khởi động.
//
// Máy trạm không ổ cứng: Windows tự đăng nhập xong là desktop hiện ra, nhưng
// giao diện VNET chưa kịp dựng màn hình khoá — Wails chỉ hiện cửa sổ ở DomReady,
// sau khi WebView2 khởi động lạnh từ ổ đĩa mạng mất vài giây. Suốt quãng đó khách
// thấy desktop, và bấm vào nó được.
//
// Màn che là một cửa sổ GDI trơn, cùng màu nền màn hình khoá, dựng ngay đầu
// runDock (trước cả khi Wails khởi tạo) và hạ xuống khi cửa sổ Wails đã hiện.
// CHỈ thanh điều khiển chính (runDock) dùng nó: cửa sổ phụ và dịch vụ nền thì
// không bao giờ.
//
// Phần Win32 ở curtain_windows.go; tệp này giữ quyết định và các hàm dùng chung.

const (
	// Chỉ che lúc mới bật máy. Giao diện bị bật lại giữa buổi (treo, bị tắt)
	// mà phủ màn che lên thì khách đang chơi game bị chặn ngang mặt.
	curtainMaxUptime = 5 * time.Minute
	// Chốt an toàn: giao diện không lên nổi thì màn che vẫn tự hạ, không thì máy
	// kẹt sau một tấm màn không ai tắt được.
	curtainFailsafe = 30 * time.Second
	// Cửa sổ Wails luôn nổi và dựng SAU nên nằm trên màn che; chờ thêm chút để
	// chắc nó đã lên trên rồi mới hạ, khỏi chớp desktop giữa hai cửa sổ.
	curtainHideDelay = 400 * time.Millisecond
	// Màn che rộng hơn màn hình mỗi mép chừng này: tiến trình chưa khai DPI nên
	// Windows co giãn cửa sổ theo tỉ lệ (xem curtain_windows.go), mà tỉ lệ lẻ như
	// 175% làm tròn hụt một pixel ở mép phải/dưới.
	curtainMargin = 16
	// Thanh taskbar cũng là cửa sổ "luôn nổi" và explorer.exe dựng nó SAU màn che
	// (giao diện được bật ngay khi phiên có token, trước khi explorer chạy), nên nó
	// nằm trên màn che và khách thấy nút Start. Màn che không bao giờ được kích hoạt
	// (WS_EX_NOACTIVATE) để tự giành lại, nên cứ chừng này lại tự nâng lên đỉnh
	// nhóm cửa sổ nổi, cho tới khi curtainStopRaise.
	curtainRaiseEvery = 150 * time.Millisecond
)

// processStart là mốc đo thời gian cho nhật ký: khởi tạo cùng gói nên gần đúng
// lúc tiến trình bắt đầu.
var processStart = time.Now()

var domReadyOnce sync.Once

// curtainRaiseStopped: màn che thôi nâng mình lên đỉnh. Một khi đã đặt thì không
// bao giờ bỏ, nên gọi trước hay sau lúc cửa sổ được dựng đều đúng.
var curtainRaiseStopped atomic.Bool

// curtainStopRaise gọi ở ĐẦU OnDomReady, TRƯỚC ApplyWindowState: ở cuối nhánh khoá
// của nó, WindowSetAlwaysOnTop(true) của Wails đưa cửa sổ khoá lên đỉnh nhóm cửa
// sổ nổi — màn che còn tiếp tục tự nâng thì sẽ đè lên chính màn hình khoá đó.
// Gọi lại ở các lần nạp trang sau vẫn vô hại.
func curtainStopRaise() { curtainRaiseStopped.Store(true) }

// shouldShowCurtain: có dựng màn che không.
//
//	uptime:             thời gian máy đã chạy kể từ lúc bật.
//	maintenanceFlag:    có tệp maintenance.flag cạnh .exe (nhân viên kỹ thuật đang sửa máy).
//	dockAlreadyRunning: thanh điều khiển đã chạy trong phiên này. Bản chạy thứ hai
//	                    sẽ gõ cửa bản đầu rồi thoát ngay — dựng màn che chỉ để nó
//	                    chớp lên một cái.
//
// Hàm thuần để kiểm bằng bảng; việc "chỉ trên Windows" nằm ở chỗ bản không phải
// Windows của showCurtain không làm gì.
func shouldShowCurtain(uptime time.Duration, maintenanceFlag, dockAlreadyRunning bool) bool {
	if maintenanceFlag || dockAlreadyRunning {
		return false
	}
	return uptime >= 0 && uptime < curtainMaxUptime
}

// startCurtain là việc ĐẦU TIÊN của runDock, trước cả NewApp và wails.Run: mỗi
// mili giây đi trước nó là một mili giây khách còn thấy desktop.
func startCurtain() {
	uptime := systemUptime()
	log.Printf("[ui] khởi động, máy đã chạy %ds", int(uptime.Seconds()))
	if shouldShowCurtain(uptime, maintenanceMode(), dockAlreadyRunning()) {
		showCurtain()
	}
}

// curtainDomReady gọi từ OnDomReady, sau ApplyWindowState: ngay lúc đó Wails hiện
// cửa sổ, nên đó là lúc màn che hết việc.
//
// OnDomReady chạy lại ở mỗi lần nạp trang; chỉ lần đầu là có nghĩa.
func curtainDomReady() {
	domReadyOnce.Do(func() {
		log.Printf("[ui] DOM sẵn sàng sau %dms", time.Since(processStart).Milliseconds())
		time.AfterFunc(curtainHideDelay, hideCurtain)
	})
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
