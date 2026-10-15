//go:build windows

package main

import (
	"log"
	"runtime"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Màn khoá native — phần cửa sổ và thread. Lý do và quyết định ở curtain.go; ô nhập
// và cách vẽ ở curtain_form_windows.go.
//
// DPI. Bản build production dùng `--manifest none` (scripts/build-client.sh) nên .exe
// KHÔNG khai DPI; Wails chỉ gọi SetProcessDPIAware khi wails.Run chạy, tức SAU màn
// khoá. Không gọi API DPI cấp TIẾN TRÌNH nào ở đây: SetProcessDPIAware của Wails mà lỗi
// thì wails.Run trả lỗi và giao diện không lên. Thread của màn khoá tự khai "biết DPI
// hệ thống" cho RIÊNG nó (SetThreadDpiAwarenessContext, Windows 10 1607 trở đi): số đo
// là pixel vật lý nên chữ nét ở mọi tỉ lệ, và cửa sổ giữ chế độ DPI của thread tạo ra
// nó — đo lại lúc đổi độ phân giải vẫn cùng một hệ đơn vị, dù lúc đó tiến trình đã
// thành "biết DPI". Windows cũ hơn không có hàm đó thì thread "không biết DPI":
// GetSystemMetrics trả số đã co theo tỉ lệ và Windows tự phóng cửa sổ lên đúng cỡ vật
// lý (chữ hơi nhoè), vẫn phủ kín; phần tròn số lẻ do curtainMargin gánh.

// Tên khoá chạy-một-bản Wails tạo cho thanh điều khiển: "wails-app-" + UniqueId
// ("vnet-client-dock", main.go) + "sim", xem single_instance.go của Wails. Khác
// dockDangChay (service_windows.go): dịch vụ ở phiên 0 phải đi đường "Session\<n>\",
// còn đây là chính tiến trình giao diện nên tên trần là tên trong phiên của nó.
const dockMutexName = "wails-app-vnet-client-docksim"

// Màu lấy từ frontend/src/styles/tokens.css, cùng bộ với LockScreen.vue; mauNen cũng
// là BackgroundColour của cửa sổ Wails (main.go). COLORREF xếp 0x00BBGGRR.
const (
	mauNen   = 0x0a | 0x0f<<8 | 0x1e<<16 // --vnet-bg, cũng là nền ô nhập
	mauKhung = 0x11 | 0x18<<8 | 0x33<<16 // --vnet-surface
	mauVien  = 0x25 | 0x30<<8 | 0x56<<16 // --vnet-border
	mauChu   = 0xe9 | 0xed<<8 | 0xf8<<16 // --vnet-text
	mauChuMo = 0x94 | 0xa0<<8 | 0xc2<<16 // --vnet-text-muted
	mauChinh = 0x4c | 0x7d<<8 | 0xff<<16 // --vnet-primary
	mauTim   = 0x7b | 0x5c<<8 | 0xff<<16 // --vnet-violet
	mauLoi   = 0xff | 0x5d<<8 | 0x5d<<16 // --vnet-danger
)

const (
	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCXVirtualScreen = 78
	smCYVirtualScreen = 79

	wsPopup        = 0x80000000
	wsChild        = 0x40000000
	wsVisible      = 0x10000000
	wsClipChildren = 0x02000000
	wsTabStop      = 0x00010000
	wsExTopmost    = 0x00000008
	wsExToolWindow = 0x00000080
	swShow         = 5

	wmNull          = 0x0000
	wmDestroy       = 0x0002
	wmSetFocus      = 0x0007
	wmPaint         = 0x000F
	wmClose         = 0x0010
	wmEraseBkgnd    = 0x0014
	wmDrawItem      = 0x002B
	wmDisplayChange = 0x007E
	wmKeyDown       = 0x0100
	wmCommand       = 0x0111
	wmTimer         = 0x0113
	wmCtlColorEdit  = 0x0133

	// Thông điệp riêng (WM_APP+n). WM_CLOSE thì ai cũng gửi được, Alt+F4 cũng thành
	// WM_CLOSE, nên cửa sổ bỏ qua WM_CLOSE và chỉ đóng theo wmNativeClose — mà cũng chỉ
	// khi khoá đã được trả.
	wmNativeClose  = 0x8000 + 1
	wmNativeFocus  = 0x8000 + 2
	wmNativeResult = 0x8000 + 3

	nativeTickID  = 1
	hwndTopmost   = ^uintptr(0) // (HWND)-1
	swpNoSize     = 0x0001
	swpNoMove     = 0x0002
	swpNoActivate = 0x0010

	vkReturn = 0x0D
	idcArrow = 32512

	smtoBlock       = 0x0001
	smtoAbortIfHung = 0x0002
	// nlLayBanPhim chỉ gắn hàng đợi nhập vào thread trả lời WM_NULL trong chừng này, và
	// mỗi nlGanCach một lần là nhiều nhất (xem nlLayBanPhim).
	nlHoiToiDa = 50 * time.Millisecond
	nlGanCach  = 500 * time.Millisecond

	gwlExStyle = ^uintptr(19) // GWL_EXSTYLE, tức (int)-20

	dpiAwarenessContextSystemAware = ^uintptr(1) // (DPI_AWARENESS_CONTEXT)-2
)

var (
	procRegisterClassExW             = user32.NewProc("RegisterClassExW")
	procUnregisterClassW             = user32.NewProc("UnregisterClassW")
	procCreateWindowExW              = user32.NewProc("CreateWindowExW")
	procDefWindowProcW               = user32.NewProc("DefWindowProcW")
	procDestroyWindow                = user32.NewProc("DestroyWindow")
	procPostMessageW                 = user32.NewProc("PostMessageW")
	procPostQuitMessage              = user32.NewProc("PostQuitMessage")
	procTranslateMessage             = user32.NewProc("TranslateMessage")
	procDispatchMessageW             = user32.NewProc("DispatchMessageW")
	procShowWindow                   = user32.NewProc("ShowWindow")
	procUpdateWindow                 = user32.NewProc("UpdateWindow")
	procSetWindowPos                 = user32.NewProc("SetWindowPos")
	procSetTimer                     = user32.NewProc("SetTimer")
	procSetFocus                     = user32.NewProc("SetFocus")
	procGetForegroundWindow          = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessId     = user32.NewProc("GetWindowThreadProcessId")
	procSetForegroundWindow          = user32.NewProc("SetForegroundWindow")
	procAttachThreadInput            = user32.NewProc("AttachThreadInput")
	procSendMessageTimeoutW          = user32.NewProc("SendMessageTimeoutW")
	procIsWindowVisible              = user32.NewProc("IsWindowVisible")
	procGetWindowLongW               = user32.NewProc("GetWindowLongW")
	procGetClassNameW                = user32.NewProc("GetClassNameW")
	procLoadCursorW                  = user32.NewProc("LoadCursorW")
	procSetThreadDpiAwarenessContext = user32.NewProc("SetThreadDpiAwarenessContext")
	procGetDpiForSystem              = user32.NewProc("GetDpiForSystem")
	procGetModuleHandleW             = kernel32.NewProc("GetModuleHandleW")
)

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type curtainMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

var (
	// nlHwnd: cửa sổ màn khoá, 0 khi chưa dựng hoặc đã huỷ. Goroutine khác chỉ dùng nó
	// để PostMessage — cách duy nhất đúng để bảo thread sở hữu cửa sổ làm gì đó.
	nlHwnd atomic.Uintptr

	// Chỉ thread của màn khoá đọc/ghi các biến dưới đây.
	nlScale   = 1.0 // DPI/96 của thread; 1 khi thread không khai được DPI
	nlNen     uintptr
	nlDaHien  bool
	nlTenMay  string
	nlSubmit  func(username, password string) error
	nlGio     string
	nlDangGui bool
	nlLoi     string
	// nlKhoiPhuc: bản của nativeLockDangKhoiPhuc mà nút "Vào máy" đang vẽ theo.
	nlKhoiPhuc bool
	// Đã ghi nhật ký lần đầu không giành được bàn phím / lần đầu giành lại sau đó / lần
	// đầu cửa sổ tiền cảnh không trả lời nên không gắn hàng đợi nhập.
	nlBaoMatBanPhim, nlBaoLayLai, nlBaoKhongTraLoi bool
	// nlLanGanCuoi: lần cuối nlLayBanPhim thử gắn hàng đợi nhập.
	nlLanGanCuoi time.Time
)

func systemUptime() time.Duration { return windows.DurationSinceBoot() }

// dockAlreadyRunning: thanh điều khiển đã chạy trong phiên này chưa.
func dockAlreadyRunning() bool {
	h, err := windows.OpenMutex(windows.SYNCHRONIZE, false, windows.StringToUTF16Ptr(dockMutexName))
	if err != nil {
		return false
	}
	windows.CloseHandle(h)
	return true
}

func startNativeLockThread(machineCode string, submit func(string, string) error, ready func()) bool {
	go runNativeLock(machineCode, submit, ready)
	return true
}

func postNativeLockClose() {
	if h := nlHwnd.Load(); h != 0 {
		procPostMessageW.Call(h, wmNativeClose, 0, 0)
	}
}

func postNativeLockFocus() {
	if h := nlHwnd.Load(); h != 0 {
		procPostMessageW.Call(h, wmNativeFocus, 0, 0)
	}
}

// runNativeLock sở hữu cửa sổ và vòng thông điệp của nó. Cửa sổ gắn với thread tạo ra
// nó nên phải khoá goroutine vào một OS thread; không bỏ khoá khi thoát để Go huỷ luôn
// thread đó thay vì trả một thread từng sở hữu cửa sổ (và mang chế độ DPI riêng) về cho
// bể chung.
//
// ready được gọi khi cửa sổ đã hiện, hoặc khi bỏ cuộc (defer) — nativeLockStart khỏi
// phải chờ hết nativeLockReadyWait mới biết dựng hỏng.
func runNativeLock(machineCode string, submit func(string, string) error, ready func()) {
	runtime.LockOSThread()
	defer ready()

	// Việc ĐẦU TIÊN trên thread: mọi số đo và cửa sổ sau đây theo chế độ DPI này.
	if procSetThreadDpiAwarenessContext.Find() == nil && procGetDpiForSystem.Find() == nil {
		if cu, _, _ := procSetThreadDpiAwarenessContext.Call(dpiAwarenessContextSystemAware); cu != 0 {
			if dpi, _, _ := procGetDpiForSystem.Call(); dpi > 0 {
				nlScale = float64(dpi) / 96
			}
		}
	}

	hInst, _, _ := procGetModuleHandleW.Call(0)
	cursor, _, _ := procLoadCursorW.Call(0, idcArrow)
	nlNen, _, _ = procCreateSolidBrush.Call(mauNen)
	className, _ := windows.UTF16PtrFromString("VNETCurtain")

	wc := wndClassEx{
		lpfnWndProc:   windows.NewCallback(nativeWndProc),
		hInstance:     hInst,
		hCursor:       cursor,
		hbrBackground: nlNen,
		lpszClassName: className,
	}
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	if atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		log.Printf("[ui] không dựng được màn khoá native (RegisterClassEx): %v", err)
		return
	}
	// Huỷ lớp thì Windows huỷ luôn cây cọ nền (không tự DeleteObject nó). Dùng
	// closure để className còn được giữ sống tới lúc này.
	defer func() {
		procUnregisterClassW.Call(uintptr(unsafe.Pointer(className)), hInst)
	}()

	nlTaoDoVe()
	// Chạy sau khi cửa sổ (và các ô nhập đang dùng phông) đã huỷ.
	defer nlXoaDoVe()

	nlTenMay = machineCode
	nlSubmit = submit
	nlGio = time.Now().Format("15:04")
	win, scr := nlDoManHinh()
	nlBoCuc = nativeLayout(scr, nlScale, nlLineH)

	// KHÔNG còn WS_EX_NOACTIVATE như màn che cũ: màn khoá phải nhận bàn phím.
	// WS_EX_TOOLWINDOW: không có nút trên thanh tác vụ, không có trong Alt+Tab.
	hwnd, _, err := procCreateWindowExW.Call(
		wsExTopmost|wsExToolWindow,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("VNET"))),
		wsPopup|wsClipChildren,
		uintptr(int32(win.left)), uintptr(int32(win.top)),
		uintptr(int32(win.right-win.left)), uintptr(int32(win.bottom-win.top)),
		0, 0, hInst, 0)
	if hwnd == 0 {
		log.Printf("[ui] không dựng được màn khoá native (CreateWindowEx): %v", err)
		return
	}
	nlTaoO(hwnd, hInst)
	nlHwnd.Store(hwnd)

	// Startup đã thôi chờ (hoặc khoá đã được trả): giao diện đang đi đường màn hình khoá
	// web, một cửa sổ hiện muộn ở đây sẽ đè lên nó.
	if !nativeLockClaim() {
		procDestroyWindow.Call(hwnd)
		return
	}

	procShowWindow.Call(hwnd, swShow)
	procUpdateWindow.Call(hwnd)
	// DestroyWindow tự huỷ timer của cửa sổ nên không cần KillTimer.
	procSetTimer.Call(hwnd, nativeTickID, uintptr(curtainRaiseEvery.Milliseconds()), 0)
	nlDaHien = true
	log.Printf("[ui] màn khoá native hiện sau %dms", time.Since(processStart).Milliseconds())
	bootRec.khoaNativeHien(systemUptime())
	// Báo đã hiện TRƯỚC khi giành bàn phím: runDock chờ tin này mới bật hook chặn phím.
	ready()
	nlLayBanPhim(hwnd)

	var m curtainMsg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		if m.message == wmKeyDown && nlPhimRieng(m.hwnd, m.wParam) {
			continue
		}
		// TranslateMessage sinh WM_CHAR từ phím bấm — cả chữ có dấu Unikey gửi vào
		// (VK_PACKET) lẫn phím đi qua IME: thiếu nó thì ô nhập không gõ được chữ nào.
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

// nativeWndProc chạy trên thread của màn khoá.
func nativeWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmPaint:
		nlVe(hwnd)
		return 0
	case wmEraseBkgnd:
		// WM_PAINT tự tô nền ngay trước khi vẽ: tô ở đây nữa là đồng hồ nháy mỗi phút.
		return 1
	case wmDrawItem:
		nlVeNut(lParam)
		return 1
	case wmCtlColorEdit:
		procSetTextColor.Call(wParam, mauChu)
		procSetBkColor.Call(wParam, mauNen)
		return nlNen
	case wmCommand:
		nlLenh(hwnd, wParam, lParam)
		return 0
	case wmSetFocus:
		// Cửa sổ được kích hoạt thì Windows trao bàn phím cho chính nó; chuyển về ô
		// khách đang gõ dở.
		procSetFocus.Call(nlOVuaGo)
		return 0
	case wmTimer:
		nlNhip(hwnd)
		return 0
	case wmDisplayChange:
		log.Printf("[ui] màn khoá native: chế độ màn hình đổi — xếp lại")
		nlXep(hwnd)
		return 0
	case wmNativeFocus:
		if nativeLockOwns() {
			nlGianhBanPhim(hwnd)
		}
		return 0
	case wmNativeResult:
		nlKetQua(hwnd)
		return 0
	case wmNativeClose:
		// Chỉ đóng khi khoá đã trả: thông điệp WM_APP thì tiến trình khác cũng gửi được.
		if !nativeLockOwns() {
			procDestroyWindow.Call(hwnd)
		}
		return 0
	case wmClose:
		// Alt+F4 và mọi WM_CLOSE khác: màn khoá không đóng theo lệnh của ai cả.
		return 0
	case wmDestroy:
		nlHwnd.Store(0)
		if nlDaHien {
			log.Printf("[ui] đóng màn khoá native sau %dms", time.Since(processStart).Milliseconds())
		}
		procPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return ret
}

// nlNhip chạy mỗi curtainRaiseEvery. Còn giữ khoá thì nâng mình lên đỉnh nhóm cửa sổ
// nổi (thanh taskbar dựng sau sẽ nằm trên) và giành lại bàn phím nếu cửa sổ khác vừa
// lấy mất — khách gõ mật khẩu vào đâu thì phải là vào đây. Trả khoá rồi thì thôi ngay,
// kể cả trong lúc chờ cửa sổ đóng: lúc đó thanh điều khiển web đang lên.
//
// Chừa đúng một thứ: hộp thông báo đang hiện của chính tiến trình này (cảnh báo nhiệt
// độ, agent.go) — màn hình khoá web chưa bao giờ đè lên nó, và nó chặn luôn goroutine gọi
// nó cho tới khi có người bấm OK. Chừa là không giành bàn phím của nó và nằm NGAY DƯỚI
// nó, chứ không thôi nâng: taskbar không chen được vào giữa. Cửa sổ Wails thì không được
// chừa: Wails tự hiện nó khi trình hiển thị của WebView2 chết (RENDER_PROCESS_EXITED,
// frontend.go của Wails), và một cửa sổ 360px ở tiền cảnh là explorer đưa taskbar lên
// trên màn khoá.
func nlNhip(hwnd uintptr) {
	if nativeLockOwns() {
		fg, _, _ := procGetForegroundWindow.Call()
		if hopThoaiCuaMinh(fg) {
			procSetWindowPos.Call(hwnd, fg, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
		} else {
			procSetWindowPos.Call(hwnd, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
			if fg != hwnd {
				nlLayBanPhim(hwnd)
			}
		}
	}
	if gio := time.Now().Format("15:04"); gio != nlGio {
		nlGio = gio
		nlVeLai(hwnd, nlBoCuc.clock)
	}
	if kp := nativeLockDangKhoiPhuc(); kp != nlKhoiPhuc {
		nlKhoiPhuc = kp
		procInvalidateRect.Call(nlNut, 0, 0)
	}
}

func cuaTienTrinhNay(h uintptr) bool {
	var pid uint32
	procGetWindowThreadProcessId.Call(h, uintptr(unsafe.Pointer(&pid)))
	return pid != 0 && pid == windows.GetCurrentProcessId()
}

// hopThoaiCuaMinh: h là hộp thông báo đang hiện, luôn nổi, của chính tiến trình này
// ("#32770" là lớp hộp thoại của Windows). Mọi hộp thông báo của máy trạm đi qua
// ScreenLocker.ShowMessage nên đều MB_TOPMOST; vẫn hỏi lại vì nằm ngay dưới một cửa sổ
// KHÔNG nổi là màn khoá rơi khỏi nhóm cửa sổ nổi, xuống dưới taskbar.
func hopThoaiCuaMinh(h uintptr) bool {
	if h == 0 || !cuaTienTrinhNay(h) {
		return false
	}
	if v, _, _ := procIsWindowVisible.Call(h); v == 0 {
		return false
	}
	if ex, _, _ := procGetWindowLongW.Call(h, gwlExStyle); ex&wsExTopmost == 0 {
		return false
	}
	var lop [8]uint16
	n, _, _ := procGetClassNameW.Call(h, uintptr(unsafe.Pointer(&lop[0])), uintptr(len(lop)))
	return windows.UTF16ToString(lop[:n]) == "#32770"
}

// nlGianhBanPhim: lối tắt VNET bấm lần nữa, hoặc hook chặn phím thấy phím sắp rơi ra
// ngoài (nativeLockFocus).
func nlGianhBanPhim(hwnd uintptr) {
	procSetWindowPos.Call(hwnd, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
	nlLayBanPhim(hwnd)
}

// nlLayBanPhim đưa màn khoá ra tiền cảnh và trả bàn phím về ô khách gõ gần nhất.
//
// SetForegroundWindow trần không đủ: giao diện do dịch vụ ở phiên 0 bật chứ không do
// tiến trình đang ở tiền cảnh bật, nên khi explorer (desktop, ứng dụng khởi động cùng
// Windows) đã chiếm tiền cảnh thì Windows lặng lẽ từ chối — màn khoá vẫn nổi trên cùng
// mà phím khách gõ rơi xuống desktop phía sau. Gắn tạm hàng đợi nhập của thread này vào
// thread đang ở tiền cảnh thì Windows coi hai bên là một và cho qua. Vẫn bị từ chối thì
// hook chặn phím (nativeLockMatBanPhim) giữ phím khỏi rơi xuống dưới, bấm chuột vào màn
// khoá vẫn lấy lại được, và nhật ký ghi lại một lần để đo ở phòng máy thử.
//
// Gắn rồi thì chuyển tiền cảnh thành việc ĐỒNG BỘ: Windows gửi WM_NCACTIVATE, WM_ACTIVATE,
// WM_KILLFOCUS sang thread kia và chờ nó xử lý xong. Thread kia đang bận (explorer nạp từ
// ổ đĩa mạng lúc khởi động, game đang nạp màn chơi) hay treo hẳn thì thread này kẹt theo:
// màn khoá đơ, không vẽ, không nhận phím, và không đóng được cả khi máy đã mở. Nên chỉ
// gắn khi thread kia trả lời ngay (nlTraLoiNgay), mỗi nlGanCach nhiều nhất một lần, và gỡ
// ra ngay sau SetForegroundWindow — SetFocus chỉ đụng tới hàng đợi của chính mình.
func nlLayBanPhim(hwnd uintptr) {
	fg, _, _ := procGetForegroundWindow.Call()
	var minh, cuaFg uintptr
	if fg != 0 && fg != hwnd && !cuaTienTrinhNay(fg) && time.Since(nlLanGanCuoi) >= nlGanCach {
		nlLanGanCuoi = time.Now()
		if nlTraLoiNgay(fg) {
			minh, _, _ = procGetCurrentThreadId.Call()
			cuaFg, _, _ = procGetWindowThreadProcessId.Call(fg, 0)
			if r, _, _ := procAttachThreadInput.Call(minh, cuaFg, 1); r == 0 {
				cuaFg = 0
			}
		} else if !nlBaoKhongTraLoi {
			nlBaoKhongTraLoi = true
			var pid uint32
			procGetWindowThreadProcessId.Call(fg, uintptr(unsafe.Pointer(&pid)))
			log.Printf("[ui] màn khoá native: cửa sổ tiền cảnh (tiến trình %d) không trả lời — không gắn hàng đợi nhập", pid)
		}
	}
	ok, _, _ := procSetForegroundWindow.Call(hwnd)
	if cuaFg != 0 {
		procAttachThreadInput.Call(minh, cuaFg, 0)
	}
	procSetFocus.Call(nlOVuaGo)

	switch {
	case ok == 0 && !nlBaoMatBanPhim:
		nlBaoMatBanPhim = true
		var pid uint32
		procGetWindowThreadProcessId.Call(fg, uintptr(unsafe.Pointer(&pid)))
		log.Printf("[ui] màn khoá native không giành được bàn phím sau %dms (tiền cảnh là tiến trình %d)",
			time.Since(processStart).Milliseconds(), pid)
	case ok != 0 && nlBaoMatBanPhim && !nlBaoLayLai:
		nlBaoLayLai = true
		log.Printf("[ui] màn khoá native giành lại được bàn phím sau %dms", time.Since(processStart).Milliseconds())
	}
}

// nlTraLoiNgay: thread sở hữu cửa sổ h đang bơm thông điệp — trả lời WM_NULL trong
// nlHoiToiDa. Windows đã coi nó là treo thì trả false ngay, không chờ hết hạn.
func nlTraLoiNgay(h uintptr) bool {
	var kq uintptr
	r, _, _ := procSendMessageTimeoutW.Call(h, wmNull, 0, 0, smtoBlock|smtoAbortIfHung,
		uintptr(nlHoiToiDa.Milliseconds()), uintptr(unsafe.Pointer(&kq)))
	return r != 0
}

// nativeLockMatBanPhim: màn khoá native đang giữ khoá mà cửa sổ tiền cảnh là của tiến
// trình khác, tức phím gõ lúc này rơi xuống desktop hay ứng dụng phía sau nó. Hook chặn
// phím (locker.go) hỏi ở mỗi phím nên chỉ đọc, không khoá.
func nativeLockMatBanPhim() bool {
	if !nativeLockOwns() {
		return false
	}
	fg, _, _ := procGetForegroundWindow.Call()
	return !cuaTienTrinhNay(fg)
}

// nlPhimRieng xử lý Enter và Tab trước khi chúng tới ô nhập: EDIT một dòng không làm
// gì với hai phím này (chỉ kêu "bíp"), và đây không phải hộp thoại nên không có
// IsDialogMessage lo hộ. Trả true là đã xử lý xong, không chuyển tiếp.
//
// Bộ gõ đang ghép chữ thì Enter tới đây dưới dạng VK_PROCESSKEY, nên Enter để chốt chữ
// không bị hiểu nhầm thành "Vào máy".
func nlPhimRieng(hwnd, vk uintptr) bool {
	if hwnd != nlUser && hwnd != nlPass && hwnd != nlNut && hwnd != nlHwnd.Load() {
		return false
	}
	switch vk {
	case vkReturn:
		nlEnter()
		return true
	case vkTab:
		// Hai ô thì Tab và Shift+Tab cùng là "sang ô kia".
		if hwnd == nlUser {
			procSetFocus.Call(nlPass)
		} else {
			procSetFocus.Call(nlUser)
		}
		return true
	}
	return false
}

// nlKetQuaCh: lỗi của lượt gửi đang chạy, từ goroutine gửi sang thread của cửa sổ.
// Mỗi lúc chỉ một lượt nên chỗ chứa một là đủ.
var nlKetQuaCh = make(chan string, 1)

// nlEnter: Enter trong ô nhập, hoặc bấm "Vào máy".
func nlEnter() {
	user, pass := nlDocChu(nlUser), nlDocChu(nlPass)
	switch nativeEnterAction(nlDangGui || nativeLockDangKhoiPhuc(), user, pass) {
	case enterFocusUser:
		procSetFocus.Call(nlUser)
	case enterFocusPass:
		procSetFocus.Call(nlPass)
	case enterSubmit:
		hwnd := nlHwnd.Load()
		nlDangGui = true
		nlLoi = ""
		nlVeLai(hwnd, nlBoCuc.status)
		procInvalidateRect.Call(nlNut, 0, 0)
		submit := nlSubmit
		// Không bao giờ gọi submit trên thread này: nó chờ Startup và chờ máy chủ, còn
		// thread này phải bơm thông điệp liên tục, không thì cửa sổ "Not responding".
		go func() {
			err := submit(user, pass)
			if err == nil {
				// Máy đã mở và nativeLockClose đã gọi: cửa sổ sắp đóng, giữ nguyên chữ
				// "Đang vào máy…" tới lúc đó.
				return
			}
			nlKetQuaCh <- err.Error()
			procPostMessageW.Call(hwnd, wmNativeResult, 0, 0)
		}()
	}
}

// nlKetQua: lượt gửi trả lỗi. Hiện nguyên văn câu lỗi (máy chủ đã nói bằng tiếng Việt),
// xoá mật khẩu và đưa con trỏ về đó để gõ lại.
func nlKetQua(hwnd uintptr) {
	select {
	case loi := <-nlKetQuaCh:
		nlLoi = loi
	default:
		return
	}
	nlDangGui = false
	nlDatChu(nlPass, "")
	procSetFocus.Call(nlPass)
	nlVeLai(hwnd, nlBoCuc.status)
	procInvalidateRect.Call(nlNut, 0, 0)
}
