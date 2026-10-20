//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// wallpaperDir: thư mục của KHÁCH (giao diện chạy bằng quyền của khách), không phải thư mục cài chỉ dịch vụ nền ghi được.
func wallpaperDir() string {
	if d := os.Getenv("LOCALAPPDATA"); d != "" {
		return filepath.Join(d, "VNET", "wallpaper")
	}
	return filepath.Join(os.TempDir(), "vnet-wallpaper")
}

const (
	spiSetDeskWallpaper = 0x0014
	spifUpdateIniFile   = 0x01
	spifSendChange      = 0x02
)

// procSystemParametersInfoW đã khai ở workarea_windows.go.

// osSetWallpaper đặt ảnh làm hình nền, kiểu "Lấp đầy" (WallpaperStyle 10, không lát). Hai giá trị registry phải đặt TRƯỚC
// khi gọi SystemParametersInfo, không thì Windows vẫn dùng kiểu cũ cho lần đổi này.
func osSetWallpaper(file string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, `Control Panel\Desktop`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("mở registry: %w", err)
	}
	defer k.Close()
	if err := k.SetStringValue("WallpaperStyle", "10"); err != nil {
		return err
	}
	if err := k.SetStringValue("TileWallpaper", "0"); err != nil {
		return err
	}
	p, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return err
	}
	r, _, callErr := procSystemParametersInfoW.Call(spiSetDeskWallpaper, 0, uintptr(unsafe.Pointer(p)), spifUpdateIniFile|spifSendChange)
	if r == 0 {
		return fmt.Errorf("SystemParametersInfo: %v", callErr)
	}
	return nil
}
