//go:build windows

package main

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Lấy icon của tệp chạy game bằng Win32 thuần qua syscall — không CGO, nên vẫn
// biên dịch chéo được từ máy Mac. user32/gdi32 và các hàm GDI dùng chung với
// screenshot_windows.go.

var (
	procPrivateExtractIcons = user32.NewProc("PrivateExtractIconsW")
	procGetIconInfo         = user32.NewProc("GetIconInfo")
	procDestroyIcon         = user32.NewProc("DestroyIcon")
	procGetObject           = gdi32.NewProc("GetObjectW")
)

// ICONINFO.
type iconInfo struct {
	fIcon    int32
	xHotspot uint32
	yHotspot uint32
	hbmMask  uintptr
	hbmColor uintptr
}

// BITMAP.
type gdiBitmap struct {
	bmType       int32
	bmWidth      int32
	bmHeight     int32
	bmWidthBytes int32
	bmPlanes     uint16
	bmBitsPixel  uint16
	bmBits       uintptr
}

// extractIconPixels đọc icon đầu tiên của tệp (exe, dll, ico) ở cỡ gần size nhất.
//
// PrivateExtractIconsW chọn thẳng ảnh phù hợp trong nhóm icon của tệp, kể cả ảnh
// 256px nén PNG của icon kiểu Vista, còn SHGetFileInfo/ExtractIconEx chỉ cho
// 32px. Hàm này cũng không cần COM và không nạp phần mở rộng Shell nào, nên gọi
// được từ goroutine bất kỳ. Tệp không có icon (launcher .bat...) trả errNoGameIcon.
func extractIconPixels(path string, size int) (*iconPixels, error) {
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	var icon uintptr
	procPrivateExtractIcons.Call(
		uintptr(unsafe.Pointer(file)), 0, uintptr(size), uintptr(size),
		uintptr(unsafe.Pointer(&icon)), 0, 1, 0)
	if icon == 0 {
		return nil, errNoGameIcon
	}
	defer procDestroyIcon.Call(icon)

	var ii iconInfo
	if r, _, e := procGetIconInfo.Call(icon, uintptr(unsafe.Pointer(&ii))); r == 0 {
		return nil, fmt.Errorf("GetIconInfo: %w", e)
	}
	// GetIconInfo giao cho ta CẢ HAI bitmap: không xoá thì mỗi lần lấy icon rò
	// hai đối tượng GDI, mà tiến trình giao diện chạy suốt ngày.
	if ii.hbmColor != 0 {
		defer procDeleteObject.Call(ii.hbmColor)
	}
	if ii.hbmMask != 0 {
		defer procDeleteObject.Call(ii.hbmMask)
	}
	if ii.hbmColor == 0 {
		// Icon đơn sắc: không có bitmap màu, mặt nạ xếp chồng AND/XOR. Hiếm và
		// xấu, không đáng viết thêm đường riêng.
		return nil, fmt.Errorf("%w: icon đơn sắc", errNoGameIcon)
	}

	w, h, err := gdiBitmapSize(ii.hbmColor)
	if err != nil {
		return nil, err
	}
	dc, _, _ := procGetDC.Call(0)
	if dc == 0 {
		return nil, errors.New("không lấy được ngữ cảnh màn hình")
	}
	defer procReleaseDC.Call(0, dc)

	color, err := gdiBitmapBits(dc, ii.hbmColor, w, h)
	if err != nil {
		return nil, err
	}
	// Mặt nạ chỉ cần cho icon cũ không có alpha; đọc không được thì bỏ chứ không
	// huỷ cả icon (iconToNRGBA khi đó coi icon là đặc).
	var mask []byte
	if ii.hbmMask != 0 {
		if mw, mh, err := gdiBitmapSize(ii.hbmMask); err == nil && mw == w && mh == h {
			mask, _ = gdiBitmapBits(dc, ii.hbmMask, w, h)
		}
	}
	return &iconPixels{w: w, h: h, color: color, mask: mask}, nil
}

func gdiBitmapSize(hbm uintptr) (w, h int, err error) {
	var bm gdiBitmap
	r, _, _ := procGetObject.Call(hbm, unsafe.Sizeof(bm), uintptr(unsafe.Pointer(&bm)))
	if r == 0 {
		return 0, 0, errors.New("không đọc được kích thước icon")
	}
	w, h = int(bm.bmWidth), int(bm.bmHeight)
	if w <= 0 || h <= 0 || w > maxIconSide || h > maxIconSide {
		return 0, 0, fmt.Errorf("kích thước icon %dx%d không hợp lệ", w, h)
	}
	return w, h, nil
}

// gdiBitmapBits đọc bitmap ra BGRA 32 bpp, hàng từ trên xuống. Bitmap 1 bpp của
// mặt nạ cũng được GDI đổi sang 32 bpp (đen/trắng), nên khỏi tự giải mã bit và
// căn hàng theo 4 byte.
func gdiBitmapBits(dc, hbm uintptr, w, h int) ([]byte, error) {
	// Height âm: hàng từ trên xuống, giống chụp màn hình.
	bi := bitmapInfo{Header: bitmapInfoHeader{
		Size:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		Width:       int32(w),
		Height:      -int32(h),
		Planes:      1,
		BitCount:    32,
		Compression: biRGB,
	}}
	buf := make([]byte, w*h*4)
	r, _, _ := procGetDIBits.Call(dc, hbm, 0, uintptr(h),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bi)), dibRGBColors)
	if r == 0 {
		return nil, errors.New("đọc điểm ảnh icon thất bại")
	}
	return buf, nil
}
