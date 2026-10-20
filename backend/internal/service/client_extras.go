package service

import (
	"fmt"
	"path"
	"strings"
	"unicode"
)

// Cài đặt máy trạm thêm (roadmap C1–C4): hình nền và trình chiếu, ứng dụng tự chạy khi khách đăng nhập, phím nóng Alt+K
// đóng ứng dụng, và logo trên menu game. Hình nền, danh sách chạy và Alt+K đi xuống máy qua ClientPolicy trong phản
// hồi nhịp tim; tên quán và logo đi qua GET /api/game-menu.

const (
	WallpaperURLsKey     = "wallpaper_urls"             // mỗi dòng một đường dẫn /uploads/...
	WallpaperIntervalKey = "wallpaper_interval_seconds" // 0: không trình chiếu, dùng ảnh đầu tiên
	StartupAppsKey       = "startup_apps"               // mỗi dòng một tệp .exe, xem chuanHoaUngDungKhoiDong
	CloseHotkeyKey       = "close_hotkey"               // "true": Alt+K đóng ứng dụng đang mở
	CloseExcludeKey      = "close_exclude"              // tên tiến trình KHÔNG bị Alt+K đóng
	MenuLogoKey          = "menu_logo"                  // nhóm general: đường dẫn /uploads/... của logo menu

	maxWallpapers        = 20
	maxStartupApps       = 20
	minWallpaperInterval = 10
	maxWallpaperInterval = 86400
	maxStartupAppLen     = 260
)

var wallpaperExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true}

// hinhNenHopLe: đường dẫn ảnh do chính máy chủ giữ (/uploads/...), không thoát ra ngoài, chỉ jpg/jpeg/png (đuôi mà
// Windows đặt làm hình nền được và trang tải lên chấp nhận).
func hinhNenHopLe(raw string) (string, bool) {
	p := strings.TrimSpace(raw)
	if !strings.HasPrefix(p, "/uploads/") || strings.ContainsAny(p, "?#\\") || strings.Contains(p, "..") || len(p) > 200 {
		return "", false
	}
	for _, r := range p {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return "", false
		}
	}
	if path.Clean(p) != p || !wallpaperExts[strings.ToLower(path.Ext(p))] {
		return "", false
	}
	return p, true
}

// chuanHoaHinhNen: nhiều dòng → các đường dẫn hợp lệ, bỏ trùng, tối đa maxWallpapers. Dòng sai bị bỏ.
func chuanHoaHinhNen(raw string) []string {
	var out []string
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r", "\n"), "\n") {
		p, ok := hinhNenHopLe(line)
		if !ok || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
		if len(out) == maxWallpapers {
			break
		}
	}
	return out
}

// ungDungKhoiDongHopLe kiểm MỘT dòng và trả đường dẫn đã chuẩn hoá (dấu gạch ngược, đã bỏ khoảng trắng đầu cuối).
//
// Đây là danh sách tệp chạy trên MỌI máy trạm khi khách đăng nhập, nên hẹp có chủ ý, cùng tinh thần với RunFirst: chỉ
// tệp .exe, không tham số, không ký tự của dòng lệnh, không thoát lên thư mục trên; tương đối thì tính từ thư mục game
// trên máy trạm, tuyệt đối thì chỉ trong Program Files.
func ungDungKhoiDongHopLe(raw string) (string, error) {
	p := strings.TrimSpace(strings.ReplaceAll(raw, "/", `\`))
	if p == "" {
		return "", fmt.Errorf("dòng trống")
	}
	if len(p) > maxStartupAppLen {
		return "", fmt.Errorf("đường dẫn dài quá %d ký tự", maxStartupAppLen)
	}
	for _, r := range p {
		if unicode.IsControl(r) || strings.ContainsRune(`"&|<>^%*?`, r) {
			return "", fmt.Errorf("có ký tự không được phép: %q", string(r))
		}
	}
	if !strings.HasSuffix(strings.ToLower(p), ".exe") {
		return "", fmt.Errorf("chỉ nhận tệp .exe")
	}
	if strings.HasPrefix(p, `\\`) {
		return "", fmt.Errorf("không nhận đường dẫn mạng")
	}
	for _, part := range strings.Split(p, `\`) {
		if part == ".." || part == "." {
			return "", fmt.Errorf("không được có . hoặc .. trong đường dẫn")
		}
		// Hai dấu gạch ngược liền nhau (hay gặp khi dán từ chuỗi đã escape) thì máy trạm từ chối cả dòng.
		if part == "" {
			return "", fmt.Errorf("có hai dấu gạch ngược liền nhau")
		}
	}
	if len(p) >= 2 && p[1] == ':' { // tuyệt đối
		low := strings.ToLower(p)
		if !strings.HasPrefix(low, `c:\program files\`) && !strings.HasPrefix(low, `c:\program files (x86)\`) {
			return "", fmt.Errorf("đường dẫn tuyệt đối chỉ được nằm trong C:\\Program Files hoặc C:\\Program Files (x86)")
		}
	} else if strings.HasPrefix(p, `\`) {
		return "", fmt.Errorf("đường dẫn tương đối không bắt đầu bằng dấu gạch ngược")
	}
	return p, nil
}

// chuanHoaUngDungKhoiDong: nhiều dòng → các dòng hợp lệ, bỏ trùng không phân biệt hoa thường. Dòng sai bị bỏ ở đây
// (máy trạm cũng kiểm lại); kiemTraUngDungKhoiDong từ chối chúng ngay lúc lưu để quản trị viên thấy lỗi.
func chuanHoaUngDungKhoiDong(raw string) []string {
	var out []string
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r", "\n"), "\n") {
		p, err := ungDungKhoiDongHopLe(line)
		if err != nil || seen[strings.ToLower(p)] {
			continue
		}
		seen[strings.ToLower(p)] = true
		out = append(out, p)
		if len(out) == maxStartupApps {
			break
		}
	}
	return out
}

// kiemTraCaiDatMayTramThem từ chối giá trị sai của các khoá C1–C3 ngay lúc lưu, kèm số dòng: để quản trị viên sửa chứ
// không để dòng sai biến mất lặng lẽ.
func kiemTraCaiDatMayTramThem(in map[string]interface{}) error {
	str := func(key string) (string, bool, error) {
		v, ok := in[key]
		if !ok || v == nil {
			return "", false, nil
		}
		s, isStr := v.(string)
		if !isStr {
			return "", true, fmt.Errorf("%s phải là chuỗi", key)
		}
		return s, true, nil
	}
	if raw, ok, err := str(StartupAppsKey); err != nil {
		return err
	} else if ok {
		n := 0
		for i, line := range strings.Split(strings.ReplaceAll(raw, "\r", "\n"), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if n++; n > maxStartupApps {
				return fmt.Errorf("ứng dụng tự chạy: tối đa %d dòng", maxStartupApps)
			}
			if _, err := ungDungKhoiDongHopLe(line); err != nil {
				return fmt.Errorf("ứng dụng tự chạy, dòng %d: %w", i+1, err)
			}
		}
	}
	if raw, ok, err := str(WallpaperURLsKey); err != nil {
		return err
	} else if ok {
		n := 0
		for i, line := range strings.Split(strings.ReplaceAll(raw, "\r", "\n"), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if n++; n > maxWallpapers {
				return fmt.Errorf("hình nền: tối đa %d ảnh", maxWallpapers)
			}
			if _, ok := hinhNenHopLe(line); !ok {
				return fmt.Errorf("hình nền, dòng %d: chỉ nhận ảnh jpg/png đã tải lên (/uploads/...)", i+1)
			}
		}
	}
	return nil
}

// khoangTrinhChieu ép thời gian đổi ảnh về khoảng hợp lệ; 0 (hoặc rỗng, hoặc sai) là không trình chiếu.
func khoangTrinhChieu(v int64) int64 {
	if v <= 0 {
		return 0
	}
	return min(max(v, minWallpaperInterval), maxWallpaperInterval)
}

// GeneralGroup là nhóm cài đặt chung của quán: tên quán, logo menu...
const GeneralGroup = "general"

var logoExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}

// logoMenuHopLe: đường dẫn ảnh đã tải lên máy chủ này (/uploads/...), đuôi ảnh mà trang tải lên chấp nhận.
func logoMenuHopLe(raw string) (string, bool) {
	p := strings.TrimSpace(raw)
	if !strings.HasPrefix(p, "/uploads/") || strings.ContainsAny(p, "?#\\") || strings.Contains(p, "..") || len(p) > 200 || path.Clean(p) != p {
		return "", false
	}
	for _, r := range p {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return "", false
		}
	}
	if !logoExts[strings.ToLower(path.Ext(p))] {
		return "", false
	}
	return p, true
}

// kiemTraLogoMenu từ chối logo không phải ảnh đã tải lên; rỗng là bỏ logo.
func kiemTraLogoMenu(in map[string]interface{}) error {
	v, ok := in[MenuLogoKey]
	if !ok || v == nil {
		return nil
	}
	s, isStr := v.(string)
	if !isStr {
		return fmt.Errorf("logo menu phải là chuỗi")
	}
	if strings.TrimSpace(s) == "" {
		return nil
	}
	if _, ok := logoMenuHopLe(s); !ok {
		return fmt.Errorf("logo menu: chỉ nhận ảnh đã tải lên (/uploads/...) dạng jpg, png, gif hoặc webp")
	}
	return nil
}
