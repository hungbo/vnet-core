//go:build windows

package main

import (
	"fmt"
	"image"
	"syscall"
	"unsafe"
)

// Chụp màn hình bằng GDI qua syscall — không cần CGO, nên vẫn biên dịch chéo
// được từ máy Mac cho cả amd64 lẫn arm64.
//
// Về quyền riêng tư: hàm này CHỈ được gọi từ nút trong khung chat của chính
// khách. Không nối nó vào lệnh điều khiển từ xa — chụp lén màn hình khách là
// việc khác hẳn với khách tự gửi ảnh để nhờ hỗ trợ.

var (
	gdi32 = syscall.NewLazyDLL("gdi32.dll")

	procGetDC                 = user32.NewProc("GetDC")
	procReleaseDC             = user32.NewProc("ReleaseDC")
	procGetSystemMetrics      = user32.NewProc("GetSystemMetrics")
	procCreateCompatibleDC    = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBmp   = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject          = gdi32.NewProc("SelectObject")
	procBitBlt                = gdi32.NewProc("BitBlt")
	procGetDIBits             = gdi32.NewProc("GetDIBits")
	procDeleteObject          = gdi32.NewProc("DeleteObject")
	procDeleteDC              = gdi32.NewProc("DeleteDC")
)

const (
	smCXScreen = 0
	smCYScreen = 1
	srcCopy    = 0x00CC0020
	biRGB        = 0
	dibRGBColors = 0
)

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [1]uint32
}

// captureScreen trả về ảnh màn hình chính dưới dạng data URI JPEG.
func captureScreen() (string, error) {
	w, _, _ := procGetSystemMetrics.Call(smCXScreen)
	h, _, _ := procGetSystemMetrics.Call(smCYScreen)
	width, height := int(w), int(h)
	if width <= 0 || height <= 0 {
		return "", fmt.Errorf("không đọc được kích thước màn hình")
	}

	screenDC, _, _ := procGetDC.Call(0)
	if screenDC == 0 {
		return "", fmt.Errorf("không lấy được ngữ cảnh màn hình")
	}
	defer procReleaseDC.Call(0, screenDC)

	memDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	if memDC == 0 {
		return "", fmt.Errorf("không tạo được ngữ cảnh bộ nhớ")
	}
	defer procDeleteDC.Call(memDC)

	bmp, _, _ := procCreateCompatibleBmp.Call(screenDC, uintptr(width), uintptr(height))
	if bmp == 0 {
		return "", fmt.Errorf("không tạo được vùng ảnh")
	}
	defer procDeleteObject.Call(bmp)

	old, _, _ := procSelectObject.Call(memDC, bmp)
	defer procSelectObject.Call(memDC, old)

	ret, _, _ := procBitBlt.Call(memDC, 0, 0, uintptr(width), uintptr(height),
		screenDC, 0, 0, srcCopy)
	if ret == 0 {
		return "", fmt.Errorf("sao chép màn hình thất bại")
	}

	// Height âm để GDI trả về hàng theo thứ tự từ trên xuống; để dương thì ảnh
	// bị lộn ngược.
	bi := bitmapInfo{Header: bitmapInfoHeader{
		Size:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		Width:       int32(width),
		Height:      -int32(height),
		Planes:      1,
		BitCount:    32,
		Compression: biRGB,
	}}

	buf := make([]byte, width*height*4)
	ret, _, _ = procGetDIBits.Call(memDC, bmp, 0, uintptr(height),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bi)), dibRGBColors)
	if ret == 0 {
		return "", fmt.Errorf("đọc điểm ảnh thất bại")
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	// GDI trả về BGRA; đảo lại thành RGBA.
	for i := 0; i < width*height; i++ {
		b, g, r := buf[i*4], buf[i*4+1], buf[i*4+2]
		img.Pix[i*4] = r
		img.Pix[i*4+1] = g
		img.Pix[i*4+2] = b
		img.Pix[i*4+3] = 0xFF
	}

	return encodeScreenshot(img)
}
