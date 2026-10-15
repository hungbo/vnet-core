//go:build windows

package main

import (
	"math"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Màn khoá native — ô nhập và cách vẽ. Bố cục tính ở nativeLayout (curtain.go), cửa
// sổ và thread ở curtain_windows.go.
//
// Ô tài khoản và mật khẩu là control EDIT thật của Windows, không tự vẽ: gõ tiếng Việt
// bằng IME hay Unikey, dán, chọn chữ, ẩn mật khẩu — đều có sẵn. Khung quanh ô, chữ và
// nút "Vào máy" thì vẽ tay bằng GDI cho cùng màu với màn hình khoá web.

const (
	nlIdUser = 101
	nlIdPass = 102
	nlIdNut  = 103

	esPassword        = 0x0020
	esAutoHScroll     = 0x0080
	bsOwnerDraw       = 0x000B
	wmSetFont         = 0x0030
	emSetPasswordChar = 0x00CC
	enSetFocus        = 0x0100
	enKillFocus       = 0x0200
	bnClicked         = 0

	// Không có manifest nên Windows dùng bộ control cũ, ô mật khẩu hiện "*"; đổi sang
	// chấm tròn như ô mật khẩu web.
	chuMatKhau = 0x25CF

	fwNormal         = 400
	fwSemiBold       = 600
	fwBold           = 700
	defaultCharset   = 1
	cleartypeQuality = 5
	psInsideFrame    = 6
	bkTransparent    = 1

	dtLeft        = 0x0
	dtCenter      = 0x1
	dtVCenter     = 0x4
	dtWordBreak   = 0x10
	dtSingleLine  = 0x20
	dtNoPrefix    = 0x800
	dtEndEllipsis = 0x8000

	gradientFillRectH = 0
)

var (
	procBeginPaint           = user32.NewProc("BeginPaint")
	procEndPaint             = user32.NewProc("EndPaint")
	procDrawTextW            = user32.NewProc("DrawTextW")
	procFillRect             = user32.NewProc("FillRect")
	procInvalidateRect       = user32.NewProc("InvalidateRect")
	procMoveWindow           = user32.NewProc("MoveWindow")
	procSendMessageW         = user32.NewProc("SendMessageW")
	procGetWindowTextW       = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW = user32.NewProc("GetWindowTextLengthW")
	procSetWindowTextW       = user32.NewProc("SetWindowTextW")
	procCreateSolidBrush     = gdi32.NewProc("CreateSolidBrush")
	procCreateFontW          = gdi32.NewProc("CreateFontW")
	procCreatePen            = gdi32.NewProc("CreatePen")
	procSetBkMode            = gdi32.NewProc("SetBkMode")
	procSetBkColor           = gdi32.NewProc("SetBkColor")
	procSetTextColor         = gdi32.NewProc("SetTextColor")
	procRoundRect            = gdi32.NewProc("RoundRect")
	procGetTextMetricsW      = gdi32.NewProc("GetTextMetricsW")
	procCreateRoundRectRgn   = gdi32.NewProc("CreateRoundRectRgn")
	procSelectClipRgn        = gdi32.NewProc("SelectClipRgn")
	procGradientFill         = windows.NewLazySystemDLL("msimg32.dll").NewProc("GradientFill")
)

type paintStruct struct {
	hdc         uintptr
	fErase      int32
	rcPaint     winRect
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

type drawItemStruct struct {
	ctlType, ctlID, itemID, itemAction, itemState uint32
	hwndItem, hDC                                 uintptr
	rcItem                                        winRect
	itemData                                      uintptr
}

type textMetric struct {
	tmHeight, tmAscent, tmDescent, tmInternalLeading, tmExternalLeading int32
	tmAveCharWidth, tmMaxCharWidth, tmWeight, tmOverhang                int32
	tmDigitizedAspectX, tmDigitizedAspectY                              int32
	tmFirstChar, tmLastChar, tmDefaultChar, tmBreakChar                 uint16
	tmItalic, tmUnderlined, tmStruckOut, tmPitchAndFamily, tmCharSet    byte
}

type triVertex struct {
	x, y                    int32
	red, green, blue, alpha uint16
}

type gradientRect struct{ upperLeft, lowerRight uint32 }

// Chỉ thread của màn khoá đọc/ghi các biến dưới đây.
var (
	nlBoCuc nativeFormLayout
	nlLineH int

	nlUser, nlPass, nlNut uintptr
	nlOVuaGo              uintptr // ô khách gõ gần nhất: kích hoạt lại cửa sổ thì trả bàn phím về đây
	nlODangGo             uintptr // ô đang có bàn phím, 0 nếu không ô nào: khung của nó viền màu chính

	nlPhongVNET, nlPhongTenMay, nlPhongGio, nlPhongTieuDe uintptr
	nlPhongNho, nlPhongO, nlPhongNut                      uintptr
	nlCoKhung, nlCoVien, nlButVien, nlButChinh            uintptr
	nlDoVe                                                []uintptr // mọi đối tượng GDI tự tạo, để xoá một lượt
)

func nlPx(v float64) int { return int(math.Round(v * nlScale)) }

func nlRect(r curtainRect) winRect {
	return winRect{int32(r.left), int32(r.top), int32(r.right), int32(r.bottom)}
}

// nlTaoDoVe tạo phông, cọ, bút một lần cho cả đời cửa sổ (DPI hệ thống không đổi khi
// chưa đăng xuất Windows), rồi đo chiều cao dòng của phông ô nhập cho nativeLayout.
//
// Cỡ phông đưa số DƯƠNG cho CreateFont, tức chiều cao cả ô chữ chứ không phải cỡ chữ:
// chữ vừa khít đúng hàng nativeLayout chừa cho nó.
func nlTaoDoVe() {
	segoe := windows.StringToUTF16Ptr("Segoe UI")
	phong := func(cao float64, dam uintptr, nghieng bool) uintptr {
		var it uintptr
		if nghieng {
			it = 1
		}
		h, _, _ := procCreateFontW.Call(
			uintptr(nlPx(cao)), 0, 0, 0, dam, it, 0, 0, // cao, rộng, nghiêng góc, góc chữ, đậm, nghiêng chữ, gạch chân, gạch ngang
			defaultCharset, 0, 0, cleartypeQuality, 0,
			uintptr(unsafe.Pointer(segoe)))
		nlDoVe = append(nlDoVe, h)
		return h
	}
	co := func(mau uintptr) uintptr {
		h, _, _ := procCreateSolidBrush.Call(mau)
		nlDoVe = append(nlDoVe, h)
		return h
	}
	but := func(mau uintptr) uintptr {
		h, _, _ := procCreatePen.Call(psInsideFrame, uintptr(max(1, nlPx(1))), mau)
		nlDoVe = append(nlDoVe, h)
		return h
	}

	nlPhongVNET = phong(56, fwBold, true)
	nlPhongTenMay = phong(60, fwBold, true)
	nlPhongGio = phong(40, fwSemiBold, false)
	nlPhongTieuDe = phong(28, fwSemiBold, false)
	nlPhongNho = phong(20, fwNormal, false)
	nlPhongO = phong(22, fwNormal, false)
	nlPhongNut = phong(22, fwSemiBold, false)
	nlCoKhung = co(mauKhung)
	nlCoVien = co(mauVien)
	nlButVien = but(mauVien)
	nlButChinh = but(mauChinh)

	hdc, _, _ := procGetDC.Call(0)
	cu, _, _ := procSelectObject.Call(hdc, nlPhongO)
	var tm textMetric
	procGetTextMetricsW.Call(hdc, uintptr(unsafe.Pointer(&tm)))
	procSelectObject.Call(hdc, cu)
	procReleaseDC.Call(0, hdc)
	nlLineH = int(tm.tmHeight)
}

func nlXoaDoVe() {
	for _, h := range nlDoVe {
		procDeleteObject.Call(h)
	}
	nlDoVe = nil
}

// nlDoManHinh đo màn hình ảo và màn hình chính theo chế độ DPI của thread này.
func nlDoManHinh() (win, scr curtainRect) {
	so := func(i uintptr) int {
		v, _, _ := procGetSystemMetrics.Call(i)
		return int(int32(v)) // có thể ÂM (màn phụ bên trái/trên màn chính): đọc đúng 32 bit thấp
	}
	return curtainLayout(so(smXVirtualScreen), so(smYVirtualScreen), so(smCXVirtualScreen),
		so(smCYVirtualScreen), so(smCXScreen), so(smCYScreen))
}

// nlTaoO dựng hai ô nhập và nút "Vào máy" theo nlBoCuc.
func nlTaoO(hwnd, hInst uintptr) {
	tao := func(lop, chu string, kieu, id uintptr, r curtainRect) uintptr {
		h, _, _ := procCreateWindowExW.Call(0,
			uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(lop))),
			uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(chu))),
			wsChild|wsVisible|kieu,
			uintptr(int32(r.left)), uintptr(int32(r.top)),
			uintptr(int32(r.right-r.left)), uintptr(int32(r.bottom-r.top)),
			hwnd, id, hInst, 0)
		procSendMessageW.Call(h, wmSetFont, nlPhongO, 0)
		return h
	}
	nlUser = tao("EDIT", "", wsTabStop|esAutoHScroll, nlIdUser, nlBoCuc.userEdit)
	nlPass = tao("EDIT", "", wsTabStop|esAutoHScroll|esPassword, nlIdPass, nlBoCuc.passEdit)
	procSendMessageW.Call(nlPass, emSetPasswordChar, chuMatKhau, 0)
	// Nút tự vẽ (WM_DRAWITEM): nút thường của bộ control cũ là ô xám nổi giữa nền tối.
	nlNut = tao("BUTTON", "Vào máy", bsOwnerDraw, nlIdNut, nlBoCuc.button)
	nlOVuaGo = nlUser
}

// nlXep đo lại màn hình rồi xếp lại cửa sổ và các ô: chế độ màn hình đổi (độ phân giải,
// cắm thêm màn hình) thì màn khoá phải phủ kín theo chế độ mới.
func nlXep(hwnd uintptr) {
	win, scr := nlDoManHinh()
	nlBoCuc = nativeLayout(scr, nlScale, nlLineH)
	procSetWindowPos.Call(hwnd, hwndTopmost,
		uintptr(int32(win.left)), uintptr(int32(win.top)),
		uintptr(int32(win.right-win.left)), uintptr(int32(win.bottom-win.top)), swpNoActivate)
	dat := func(h uintptr, r curtainRect) {
		procMoveWindow.Call(h, uintptr(int32(r.left)), uintptr(int32(r.top)),
			uintptr(int32(r.right-r.left)), uintptr(int32(r.bottom-r.top)), 1)
	}
	dat(nlUser, nlBoCuc.userEdit)
	dat(nlPass, nlBoCuc.passEdit)
	dat(nlNut, nlBoCuc.button)
	procInvalidateRect.Call(hwnd, 0, 0)
}

// nlLenh: WM_COMMAND từ nút và hai ô nhập.
func nlLenh(hwnd, wParam, lParam uintptr) {
	id, ma := wParam&0xffff, wParam>>16&0xffff
	switch {
	case id == nlIdNut && ma == bnClicked:
		nlEnter()
	case (id == nlIdUser || id == nlIdPass) && ma == enSetFocus:
		nlOVuaGo, nlODangGo = lParam, lParam
		nlVeLai(hwnd, nlBoCuc.userBox)
		nlVeLai(hwnd, nlBoCuc.passBox)
	case (id == nlIdUser || id == nlIdPass) && ma == enKillFocus:
		nlODangGo = 0
		nlVeLai(hwnd, nlBoCuc.userBox)
		nlVeLai(hwnd, nlBoCuc.passBox)
	}
}

// nlVe vẽ mọi thứ trừ hai ô nhập và nút (cửa sổ con tự vẽ; WS_CLIPCHILDREN giữ chúng
// khỏi bị vẽ đè).
func nlVe(hwnd uintptr) {
	var ps paintStruct
	hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	defer procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&ps.rcPaint)), nlNen)
	procSetBkMode.Call(hdc, bkTransparent)

	const giua = dtCenter | dtVCenter | dtSingleLine
	const trai = dtLeft | dtVCenter | dtSingleLine
	L := nlBoCuc
	nlVeChu(hdc, "VNET", L.brand, nlPhongVNET, mauChinh, giua)
	// Tên máy là thứ to nhất màn hình, như LockScreen.vue: khách báo quầy "máy nào"
	// thì đọc ngay ở đây.
	if nlTenMay != "" {
		nlVeChu(hdc, "Bạn đang ngồi máy", L.seatLabel, nlPhongNho, mauChuMo, giua)
		nlVeChu(hdc, nlTenMay, L.seatName, nlPhongTenMay, mauChu, giua|dtEndEllipsis)
	}
	nlVeChu(hdc, nlGio, L.clock, nlPhongGio, mauChu, giua)

	nlVeKhung(hdc, L.panel, nlCoKhung, nlButVien, 14)
	// Vạch màu logo ở mép trên khung, như .login-panel::before của LockScreen.vue.
	p24 := nlPx(24)
	veDaiMau(hdc, curtainRect{L.panel.left + p24, L.panel.top, L.panel.right - p24, L.panel.top + nlPx(3)}, mauChinh, mauTim)
	nlVeChu(hdc, "Đăng nhập để chơi", L.title, nlPhongTieuDe, mauChu, trai)
	nlVeChu(hdc, "Tài khoản", L.userLabel, nlPhongNho, mauChuMo, trai)
	nlVeChu(hdc, "Mật khẩu", L.passLabel, nlPhongNho, mauChuMo, trai)
	for _, o := range []struct {
		h   uintptr
		box curtainRect
	}{{nlUser, L.userBox}, {nlPass, L.passBox}} {
		vien := nlButVien
		if o.h == nlODangGo {
			vien = nlButChinh
		}
		nlVeKhung(hdc, o.box, nlNen, vien, 8)
	}
	if nlLoi != "" {
		nlVeChu(hdc, nlLoi, L.status, nlPhongNho, mauLoi, dtLeft|dtWordBreak)
	}
}

// nlVeNut vẽ nút "Vào máy": dải màu logo như nút web; đang gửi hoặc đang khôi phục
// phiên thì xám đi và đổi chữ.
func nlVeNut(lParam uintptr) {
	d := (*drawItemStruct)(unsafe.Pointer(lParam))
	rc := d.rcItem
	r := curtainRect{int(rc.trai), int(rc.tren), int(rc.phai), int(rc.duoi)}
	// Bốn góc bo lộ ra nền khung đăng nhập.
	procFillRect.Call(d.hDC, uintptr(unsafe.Pointer(&rc)), nlCoKhung)
	bo := uintptr(nlPx(16))
	vung, _, _ := procCreateRoundRectRgn.Call(uintptr(rc.trai), uintptr(rc.tren), uintptr(rc.phai+1), uintptr(rc.duoi+1), bo, bo)
	procSelectClipRgn.Call(d.hDC, vung)
	chu, mau := "Vào máy", uintptr(mauChu)
	switch {
	case nlDangGui:
		procFillRect.Call(d.hDC, uintptr(unsafe.Pointer(&rc)), nlCoVien)
		chu, mau = "Đang vào máy…", mauChuMo
	case nlKhoiPhuc:
		procFillRect.Call(d.hDC, uintptr(unsafe.Pointer(&rc)), nlCoVien)
		chu, mau = "Đang khôi phục phiên…", mauChuMo
	default:
		veDaiMau(d.hDC, r, mauChinh, mauTim)
	}
	procSelectClipRgn.Call(d.hDC, 0)
	procDeleteObject.Call(vung)
	procSetBkMode.Call(d.hDC, bkTransparent)
	nlVeChu(d.hDC, chu, r, nlPhongNut, mau, dtCenter|dtVCenter|dtSingleLine)
}

// nlVeKhung vẽ khung bo góc: nền co, viền but nằm trọn bên trong r.
func nlVeKhung(hdc uintptr, r curtainRect, co, but uintptr, banKinh float64) {
	cuCo, _, _ := procSelectObject.Call(hdc, co)
	cuBut, _, _ := procSelectObject.Call(hdc, but)
	d := uintptr(nlPx(2 * banKinh))
	procRoundRect.Call(hdc, uintptr(int32(r.left)), uintptr(int32(r.top)), uintptr(int32(r.right)), uintptr(int32(r.bottom)), d, d)
	procSelectObject.Call(hdc, cuBut)
	procSelectObject.Call(hdc, cuCo)
}

// veDaiMau tô r bằng dải màu ngang từ tu sang den — --vnet-brand của web.
func veDaiMau(hdc uintptr, r curtainRect, tu, den uint32) {
	kenh := func(m uint32, dich uint) uint16 { return uint16(m>>dich&0xff) << 8 }
	v := [2]triVertex{
		{int32(r.left), int32(r.top), kenh(tu, 0), kenh(tu, 8), kenh(tu, 16), 0},
		{int32(r.right), int32(r.bottom), kenh(den, 0), kenh(den, 8), kenh(den, 16), 0},
	}
	g := gradientRect{0, 1}
	procGradientFill.Call(hdc, uintptr(unsafe.Pointer(&v[0])), 2, uintptr(unsafe.Pointer(&g)), 1, gradientFillRectH)
}

// nlVeChu vẽ chu vào r. DT_NOPREFIX: tên máy hay câu lỗi có dấu "&" thì vẫn hiện đúng
// dấu "&", không bị hiểu thành phím tắt.
func nlVeChu(hdc uintptr, chu string, r curtainRect, phong, mau, kieu uintptr) {
	p, err := windows.UTF16PtrFromString(chu)
	if err != nil {
		return
	}
	rc := nlRect(r)
	cu, _, _ := procSelectObject.Call(hdc, phong)
	procSetTextColor.Call(hdc, mau)
	procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(p)), ^uintptr(0), uintptr(unsafe.Pointer(&rc)), kieu|dtNoPrefix)
	procSelectObject.Call(hdc, cu)
}

// nlVeLai đánh dấu r cần vẽ lại, không tô nền trước (WM_PAINT tự tô).
func nlVeLai(hwnd uintptr, r curtainRect) {
	rc := nlRect(r)
	procInvalidateRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)), 0)
}

func nlDocChu(h uintptr) string {
	n, _, _ := procGetWindowTextLengthW.Call(h)
	buf := make([]uint16, n+1)
	procGetWindowTextW.Call(h, uintptr(unsafe.Pointer(&buf[0])), n+1)
	return windows.UTF16ToString(buf)
}

func nlDatChu(h uintptr, chu string) {
	p, _ := windows.UTF16PtrFromString(chu)
	procSetWindowTextW.Call(h, uintptr(unsafe.Pointer(p)))
}
