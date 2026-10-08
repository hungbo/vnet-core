//go:build windows

package main

import (
	"log"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Màn che lúc khởi động — phần Win32. Lý do và quyết định ở curtain.go.
//
// DPI. Bản build production dùng `--manifest none` (scripts/build-client.sh) nên
// .exe KHÔNG khai DPI; Wails chỉ gọi SetProcessDPIAware khi wails.Run chạy, tức
// SAU màn che. Lúc dựng màn che tiến trình còn là "không biết DPI":
// GetSystemMetrics trả số đã co theo tỉ lệ (1920x1080 ở 150% ra 1280x720) và
// Windows tự phóng cửa sổ lên đúng cỡ vật lý — kích thước tính từ chính các số đó
// nên vẫn phủ kín. Cửa sổ giữ nguyên chế độ DPI lúc tạo, nên việc tiến trình
// chuyển sang "biết DPI" sau đó không làm nó lệch đi. Không gọi API DPI nào ở đây:
// SetProcessDPIAware của Wails mà lỗi thì wails.Run trả lỗi và giao diện không lên.
// Bản dựng bằng `wails build` (manifest per-monitor) thì số đo là pixel vật lý,
// cũng phủ kín. Phần tròn số lẻ do curtainMargin gánh.

// Tên khoá chạy-một-bản Wails tạo cho thanh điều khiển: "wails-app-" + UniqueId
// ("vnet-client-dock", main.go) + "sim", xem single_instance.go của Wails. Khác
// dockDangChay (service_windows.go): dịch vụ ở phiên 0 phải đi đường "Session\<n>\",
// còn đây là chính tiến trình giao diện nên tên trần là tên trong phiên của nó.
const dockMutexName = "wails-app-vnet-client-docksim"

// Màu lấy từ --vnet-bg (#0a0f1e) và --vnet-primary (#4c7dff) trong
// frontend/src/styles/tokens.css — màu nền màn hình khoá và màu logo. COLORREF
// xếp 0x00BBGGRR.
const (
	curtainBG   = 0x0a | 0x0f<<8 | 0x1e<<16
	curtainText = 0x4c | 0x7d<<8 | 0xff<<16
)

const (
	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCXVirtualScreen = 78
	smCYVirtualScreen = 79

	wsPopup          = 0x80000000
	wsExTopmost      = 0x00000008
	wsExToolWindow   = 0x00000080
	wsExNoActivate   = 0x08000000
	swShowNoActivate = 4

	wmDestroy = 0x0002
	wmPaint   = 0x000F
	wmClose   = 0x0010
	wmTimer   = 0x0113

	curtainRaiseTimerID = 1
	hwndTopmost         = ^uintptr(0) // (HWND)-1
	swpNoSize           = 0x0001
	swpNoMove           = 0x0002
	swpNoActivate       = 0x0010

	idcArrow        = 32512
	fwBold          = 700
	defaultCharset  = 1
	antialiasedQual = 4
	bkTransparent   = 1
	dtCenter        = 0x1
	dtVCenter       = 0x4
	dtSingleLine    = 0x20
)

var (
	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procUnregisterClassW = user32.NewProc("UnregisterClassW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procDestroyWindow    = user32.NewProc("DestroyWindow")
	procPostMessageW     = user32.NewProc("PostMessageW")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procShowWindow       = user32.NewProc("ShowWindow")
	procUpdateWindow     = user32.NewProc("UpdateWindow")
	procSetWindowPos     = user32.NewProc("SetWindowPos")
	procSetTimer         = user32.NewProc("SetTimer")
	procBeginPaint       = user32.NewProc("BeginPaint")
	procEndPaint         = user32.NewProc("EndPaint")
	procDrawTextW        = user32.NewProc("DrawTextW")
	procLoadCursorW      = user32.NewProc("LoadCursorW")
	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procCreateFontW      = gdi32.NewProc("CreateFontW")
	procSetBkMode        = gdi32.NewProc("SetBkMode")
	procSetTextColor     = gdi32.NewProc("SetTextColor")
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

type paintStruct struct {
	hdc         uintptr
	fErase      int32
	rcPaint     winRect
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
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
	// curtainMu bảo vệ hwnd và cờ yêu cầu hạ: hideCurtain gọi được từ goroutine
	// nào cũng được, còn cửa sổ thì do thread riêng của nó sở hữu.
	curtainMu      sync.Mutex
	curtainHwnd    uintptr
	curtainHideReq bool // hạ được yêu cầu từ trước khi cửa sổ kịp dựng

	// Chỉ thread của màn che đọc/ghi hai biến này.
	curtainFont     uintptr
	curtainTextRect winRect
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

// showCurtain dựng màn che và CHỜ nó hiện ra rồi mới trả về, để hideCurtain gọi
// sau đó luôn thấy một cửa sổ có thật. Chờ tối đa vài giây: dựng cửa sổ hỏng thì
// giao diện vẫn phải đi tiếp, không phải kẹt ở đây.
func showCurtain() {
	ready := make(chan struct{})
	go runCurtain(sync.OnceFunc(func() { close(ready) }))
	select {
	case <-ready:
	case <-time.After(2 * time.Second):
	}

	time.AfterFunc(curtainFailsafe, func() {
		curtainMu.Lock()
		conSong := curtainHwnd != 0
		curtainMu.Unlock()
		if conSong {
			log.Printf("[ui] màn che đã %v mà giao diện chưa sẵn sàng — tự hạ", curtainFailsafe)
		}
		hideCurtain()
	})
}

// hideCurtain hạ màn che. Gọi từ goroutine nào cũng được, gọi bao nhiêu lần cũng
// được: PostMessage là cách duy nhất đúng để bảo thread sở hữu cửa sổ tự đóng nó.
func hideCurtain() {
	curtainMu.Lock()
	curtainHideReq = true
	h := curtainHwnd
	curtainMu.Unlock()
	if h != 0 {
		procPostMessageW.Call(h, wmClose, 0, 0)
	}
}

// runCurtain sở hữu cửa sổ và vòng thông điệp của nó. Cửa sổ gắn với thread tạo
// ra nó nên phải khoá goroutine vào một OS thread; không bỏ khoá khi thoát để Go
// huỷ luôn thread đó thay vì trả một thread từng sở hữu cửa sổ về cho bể chung.
func runCurtain(ready func()) {
	runtime.LockOSThread()
	defer ready()

	hInst, _, _ := procGetModuleHandleW.Call(0)
	cursor, _, _ := procLoadCursorW.Call(0, idcArrow)
	brush, _, _ := procCreateSolidBrush.Call(curtainBG)
	className, _ := windows.UTF16PtrFromString("VNETCurtain")

	wc := wndClassEx{
		lpfnWndProc:   windows.NewCallback(curtainWndProc),
		hInstance:     hInst,
		hCursor:       cursor,
		hbrBackground: brush,
		lpszClassName: className,
	}
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	if atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		log.Printf("[ui] không dựng được màn che (RegisterClassEx): %v", err)
		return
	}
	// Huỷ lớp thì Windows huỷ luôn cây cọ nền (không tự DeleteObject nó). Dùng
	// closure để className còn được giữ sống tới lúc này.
	defer func() {
		procUnregisterClassW.Call(uintptr(unsafe.Pointer(className)), hInst)
	}()

	vx, _, _ := procGetSystemMetrics.Call(smXVirtualScreen)
	vy, _, _ := procGetSystemMetrics.Call(smYVirtualScreen)
	vw, _, _ := procGetSystemMetrics.Call(smCXVirtualScreen)
	vh, _, _ := procGetSystemMetrics.Call(smCYVirtualScreen)
	pw, _, _ := procGetSystemMetrics.Call(smCXScreen)
	ph, _, _ := procGetSystemMetrics.Call(smCYScreen)
	// Số đo có thể ÂM (màn phụ bên trái/trên màn chính): đọc đúng 32 bit thấp.
	win, text := curtainLayout(int(int32(vx)), int(int32(vy)), int(int32(vw)), int(int32(vh)), int(int32(pw)), int(int32(ph)))
	curtainTextRect = winRect{int32(text.left), int32(text.top), int32(text.right), int32(text.bottom)}

	// Cỡ chữ theo chiều cao màn hình chính nhưng nhỏ: đây là dấu hiệu "máy đang
	// khởi động", không phải màn hình quảng cáo.
	fontH := -int32(ph / 16)
	if fontH > -32 {
		fontH = -32
	}
	curtainFont, _, _ = procCreateFontW.Call(
		uintptr(fontH), 0, 0, 0, fwBold, 1, 0, 0, // cao, rộng, nghiêng góc, góc chữ, đậm, nghiêng chữ, gạch chân, gạch ngang
		defaultCharset, 0, 0, antialiasedQual, 0,
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Segoe UI"))))

	// WS_EX_TOOLWINDOW: không có nút trên thanh tác vụ. WS_EX_NOACTIVATE: không
	// bao giờ cướp tiêu điểm, nên cửa sổ Wails dựng sau vẫn nhận bàn phím như thường.
	hwnd, _, err := procCreateWindowExW.Call(
		wsExTopmost|wsExToolWindow|wsExNoActivate,
		uintptr(unsafe.Pointer(className)), 0, wsPopup,
		uintptr(int32(win.left)), uintptr(int32(win.top)),
		uintptr(int32(win.right-win.left)), uintptr(int32(win.bottom-win.top)),
		0, 0, hInst, 0)
	if hwnd == 0 {
		log.Printf("[ui] không dựng được màn che (CreateWindowEx): %v", err)
		procDeleteObject.Call(curtainFont)
		return
	}

	curtainMu.Lock()
	curtainHwnd = hwnd
	daYeuCauHa := curtainHideReq
	curtainMu.Unlock()

	procShowWindow.Call(hwnd, swShowNoActivate)
	procUpdateWindow.Call(hwnd)
	// Đặt trước khi có thể DestroyWindow bên dưới; DestroyWindow tự huỷ timer của
	// cửa sổ nên không cần KillTimer.
	procSetTimer.Call(hwnd, curtainRaiseTimerID, uintptr(curtainRaiseEvery.Milliseconds()), 0)
	ready()

	if daYeuCauHa {
		procDestroyWindow.Call(hwnd)
	}

	var m curtainMsg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

// curtainWndProc chạy trên thread của màn che. Nền do cây cọ của lớp cửa sổ tự
// tô; ở đây chỉ vẽ chữ VNET và xử lý đóng.
func curtainWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmPaint:
		var ps paintStruct
		hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		procSetBkMode.Call(hdc, bkTransparent)
		procSetTextColor.Call(hdc, curtainText)
		old, _, _ := procSelectObject.Call(hdc, curtainFont)
		rc := curtainTextRect
		procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("VNET"))),
			^uintptr(0), uintptr(unsafe.Pointer(&rc)), dtCenter|dtVCenter|dtSingleLine)
		procSelectObject.Call(hdc, old)
		procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case wmTimer:
		// Thanh taskbar dựng sau màn che sẽ nằm trên nó: nâng màn che lên đỉnh nhóm
		// cửa sổ nổi, không kích hoạt để không cướp tiêu điểm.
		if !curtainRaiseStopped.Load() {
			procSetWindowPos.Call(hwnd, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
		}
		return 0
	case wmClose:
		procDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		curtainMu.Lock()
		curtainHwnd = 0
		curtainMu.Unlock()
		procDeleteObject.Call(curtainFont)
		log.Printf("[ui] hạ màn sau %dms", time.Since(processStart).Milliseconds())
		procPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return ret
}
