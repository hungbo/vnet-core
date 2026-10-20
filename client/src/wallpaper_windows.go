//go:build windows

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
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
	spiGetDeskWallpaper = 0x0073
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
	// Windows báo đặt được chưa chắc là hình nền đã đổi (chính sách nhóm ép hình nền thì lệnh vẫn "thành công"): đọc lại,
	// lệch thì ghi nhật ký kèm dấu hiệu chính sách để lần chạy thật nói rõ chỗ gãy.
	if cur := osCurrentWallpaper(); !strings.EqualFold(cur, file) {
		log.Printf("[hình nền] Windows nhận lệnh nhưng hình nền hiện tại là %q chứ không phải %q%s", cur, file, wallpaperPolicyHint())
	}
	return nil
}

// osCurrentWallpaper: đường dẫn hình nền Windows đang dùng cho người dùng này, "" nếu không đọc được.
func osCurrentWallpaper() string {
	buf := make([]uint16, windows.MAX_PATH)
	r, _, _ := procSystemParametersInfoW.Call(spiGetDeskWallpaper, uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])), 0)
	if r == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}

// wallpaperPolicyHint: nhắc các khoá chính sách Windows thường dùng để khoá hình nền, nếu máy này có; "" nếu không có khoá nào.
func wallpaperPolicyHint() string {
	var found []string
	for _, c := range []struct {
		root registry.Key
		path string
		name string
	}{
		{registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Policies\System`, "Wallpaper"},
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Policies\System`, "Wallpaper"},
		{registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Policies\ActiveDesktop`, "NoChangingWallPaper"},
	} {
		k, err := registry.OpenKey(c.root, c.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		if _, _, err := k.GetStringValue(c.name); err == nil {
			found = append(found, c.name)
		} else if _, _, err := k.GetIntegerValue(c.name); err == nil {
			found = append(found, c.name)
		}
		k.Close()
	}
	if len(found) == 0 {
		return ""
	}
	return " (máy có khoá chính sách Windows: " + strings.Join(found, ", ") + ")"
}
