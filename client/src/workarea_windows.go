//go:build windows

package main

import "unsafe"

var procSystemParametersInfoW = user32.NewProc("SystemParametersInfoW")

const spiGetWorkArea = 0x0030

// winRect khớp bố cục RECT của Win32: bốn số LONG theo thứ tự trái, trên, phải,
// dưới. Toạ độ là mép, không phải bề rộng.
type winRect struct {
	trai, tren, phai, duoi int32
}

// doVienTaskbar hỏi Windows xem thanh taskbar ăn mất bao nhiêu ở mỗi mép, tính
// bằng pixel VẬT LÝ.
//
// Win32 không có hàm "taskbar dày bao nhiêu". Cách chính thống là lấy vùng làm
// việc rồi trừ ra khỏi kích thước màn hình — và nó tự đúng cho cả bốn vị trí
// taskbar (dưới, trên, trái, phải) lẫn chế độ tự ẩn.
//
// SPI_GETWORKAREA chỉ nói về màn hình CHÍNH, nên trên máy hai màn hình con số
// này bị áp cho cả màn phụ. Windows mặc định vẽ taskbar cùng chiều dày trên mọi
// màn hình nên xấp xỉ đó gần như luôn đúng; sai thì cũng chỉ lệch vài chục
// pixel trên màn phụ, không phải thứ làm hỏng máy.
func doVienTaskbar() vienTaskbar {
	var lamViec winRect
	ok, _, _ := procSystemParametersInfoW.Call(
		spiGetWorkArea, 0, uintptr(unsafe.Pointer(&lamViec)), 0)
	if ok == 0 {
		return vienTaskbar{}
	}

	rong, _, _ := procGetSystemMetrics.Call(smCXScreen)
	cao, _, _ := procGetSystemMetrics.Call(smCYScreen)
	if rong == 0 || cao == 0 {
		return vienTaskbar{}
	}

	return vienTaskbar{
		trai: int(lamViec.trai),
		tren: int(lamViec.tren),
		phai: int(rong) - int(lamViec.phai),
		duoi: int(cao) - int(lamViec.duoi),
	}
}
