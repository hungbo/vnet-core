package main

import (
	"fmt"
	"path"
	"strings"
	"unicode"
)

// Chính sách máy trạm C1–C3 (hình nền, ứng dụng tự chạy, Alt+K): phần thuần, dùng chung cho mọi hệ điều hành. Máy chủ đã
// lọc các danh sách này (service/client_extras.go), nhưng đây là chỗ cuối cùng trước khi tải ảnh, chạy tệp hay tắt tiến
// trình thật nên máy trạm kiểm lại theo đúng luật đó và không tin thẳng chuỗi nhận về.

const (
	maxWallpapers    = 20
	maxStartupApps   = 20
	maxStartupAppLen = 260
	minWallpaperSecs = 10
	maxWallpaperSecs = 86400
)

var wallpaperExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true}

// hinhNenHopLeMay: đường dẫn /uploads/... của chính máy chủ, jpg/jpeg/png, không thoát ra ngoài.
func hinhNenHopLeMay(raw string) (string, bool) {
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

// danhSachHinhNen tách chuỗi nhiều dòng của chính sách thành các đường dẫn hợp lệ, bỏ trùng, tối đa maxWallpapers.
func danhSachHinhNen(raw string) []string {
	var out []string
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r", "\n"), "\n") {
		p, ok := hinhNenHopLeMay(line)
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

// khoangTrinhChieuMay ép thời gian đổi ảnh về khoảng hợp lệ; 0 là không trình chiếu.
func khoangTrinhChieuMay(v int64) int64 {
	if v <= 0 {
		return 0
	}
	// Không dùng min/max: gói này tự định nghĩa min/max cho int.
	if v < minWallpaperSecs {
		return minWallpaperSecs
	}
	if v > maxWallpaperSecs {
		return maxWallpaperSecs
	}
	return v
}

// ungDungKhoiDongHopLeMay: cùng luật với máy chủ. Chỉ tệp .exe, không tham số, không ký tự của dòng lệnh; tương đối thì
// tính từ thư mục game trên máy trạm, tuyệt đối thì chỉ trong Program Files.
func ungDungKhoiDongHopLeMay(raw string) (string, error) {
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
		if part == "" {
			return "", fmt.Errorf("có hai dấu gạch ngược liền nhau")
		}
	}
	if len(p) >= 2 && p[1] == ':' {
		low := strings.ToLower(p)
		if !strings.HasPrefix(low, `c:\program files\`) && !strings.HasPrefix(low, `c:\program files (x86)\`) {
			return "", fmt.Errorf("đường dẫn tuyệt đối chỉ được nằm trong Program Files")
		}
	} else if strings.HasPrefix(p, `\`) {
		return "", fmt.Errorf("đường dẫn tương đối không bắt đầu bằng dấu gạch ngược")
	}
	return p, nil
}

// danhSachUngDungKhoiDong tách chuỗi nhiều dòng của chính sách, bỏ dòng sai và trùng, tối đa maxStartupApps.
func danhSachUngDungKhoiDong(raw string) []string {
	var out []string
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r", "\n"), "\n") {
		p, err := ungDungKhoiDongHopLeMay(line)
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

// resolveStartupApp đổi một dòng đã hợp lệ thành đường dẫn tuyệt đối để chạy. Tương đối: dưới thư mục game của máy trạm
// (gameRoot). Tuyệt đối: phải nằm trong một thư mục Program Files THẬT của máy này (programFiles, từ biến môi trường),
// không chỉ theo chữ C:\ — Windows cài trên ổ khác thì "C:\Program Files" có thể là thư mục ai đó tự tạo.
func resolveStartupApp(gameRoot, entry string, programFiles []string) (exe, dir string, err error) {
	entry, err = ungDungKhoiDongHopLeMay(entry)
	if err != nil {
		return "", "", err
	}
	if len(entry) >= 2 && entry[1] == ':' {
		clean := path.Clean(strings.ReplaceAll(entry, `\`, "/"))
		for _, pf := range programFiles {
			base := path.Clean(strings.ReplaceAll(pf, `\`, "/"))
			if base != "." && insideDir(base, clean) {
				exe = strings.ReplaceAll(clean, "/", `\`)
				return exe, exe[:strings.LastIndex(exe, `\`)], nil
			}
		}
		return "", "", fmt.Errorf("%q không nằm trong thư mục Program Files của máy này", entry)
	}
	base, err := gameRootBase(gameRoot)
	if err != nil {
		return "", "", err
	}
	parts := strings.Split(entry, `\`)
	for _, p := range parts {
		if err := checkGameComponent(p); err != nil {
			return "", "", fmt.Errorf("%q: %w", entry, err)
		}
	}
	root := path.Clean(strings.ReplaceAll(base, `\`, "/"))
	clean := path.Clean(root + "/" + strings.Join(parts, "/"))
	if !insideDir(root, clean) {
		return "", "", fmt.Errorf("%q nằm ngoài thư mục game", entry)
	}
	exe = strings.ReplaceAll(clean, "/", `\`)
	return exe, exe[:strings.LastIndex(exe, `\`)], nil
}

// ---- Alt+K: đóng ứng dụng --------------------------------------------------

// openApp là một tiến trình đang có cửa sổ hiện trên màn hình của khách.
type openApp struct {
	PID  uint32
	Name string // chữ thường, không đuôi .exe
}

// protectedApps: tiến trình của Windows và của khung desktop mà Alt+K KHÔNG BAO GIỜ đóng, không phụ thuộc cài đặt của quán:
// đóng chúng là mất thanh tác vụ hay bàn phím cảm ứng chứ không phải "đóng ứng dụng". Tên máy trạm VNET thêm lúc chạy.
var protectedApps = map[string]bool{
	"explorer": true, "dwm": true, "winlogon": true, "csrss": true, "wininit": true, "services": true, "lsass": true,
	"svchost": true, "ctfmon": true, "searchhost": true, "searchapp": true, "searchui": true,
	"shellexperiencehost": true, "startmenuexperiencehost": true, "textinputhost": true, "applicationframehost": true,
	"runtimebroker": true, "sihost": true, "taskhostw": true, "fontdrvhost": true, "logonui": true, "lockapp": true,
	"vnet": true, "vnet-client": true, "vnetagent": true,
}

// danhSachLoaiTruDong: "Discord.exe, steam" → tập {"discord","steam"}; tên không hợp lệ bị bỏ.
func danhSachLoaiTruDong(csv string) map[string]bool {
	out := map[string]bool{}
	for _, raw := range strings.FieldsFunc(csv, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' || r == ';' }) {
		name, err := safeProcessName(raw)
		if err != nil {
			continue
		}
		out[strings.ToLower(name)] = true
	}
	return out
}

// chonDeDong chọn những tiến trình Alt+K đóng: mọi ứng dụng có cửa sổ trừ tiến trình được bảo vệ, chính máy trạm
// (selfPID, selfName) và danh sách loại trừ của quán. Mỗi PID chỉ một lần.
func chonDeDong(apps []openApp, exclude map[string]bool, selfPID uint32, selfName string) []openApp {
	self := strings.ToLower(strings.TrimSuffix(selfName, ".exe"))
	seen := map[uint32]bool{}
	var out []openApp
	for _, a := range apps {
		name := strings.ToLower(strings.TrimSuffix(a.Name, ".exe"))
		switch {
		case a.PID == 0 || a.PID == selfPID || seen[a.PID]:
		case name == "" || protectedApps[name] || exclude[name] || (self != "" && name == self):
		default:
			seen[a.PID] = true
			out = append(out, a)
		}
	}
	return out
}
