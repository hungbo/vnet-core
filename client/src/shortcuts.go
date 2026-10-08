package main

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Dọn lối tắt rác trên desktop máy trạm.
//
// Phần mềm bên thứ ba thả lối tắt ra desktop mỗi lần máy khởi động: Cloud Update
// ("Game Menu.lnk" do lwclient64.exe tạo lại mỗi lần boot, không có công tắc tắt)
// và Riot ("Riot Client.lnk", "Teamfight Tactics.lnk"). Máy diskless về bản gốc
// sau mỗi lần khởi động nên xoá một lần là vô ích; chủ quán cũng không chịu chặn
// quyền tạo tệp. Cách chọn: dịch vụ nền (SYSTEM) xoá lối tắt có TÊN nằm trong
// danh sách admin đặt (clientPolicy.HiddenShortcuts) mỗi vài giây.
//
// Tệp này là phần thuần — chọn tệp nào bị xoá, không đụng Windows — để kiểm
// được ở bất kỳ đâu. Việc tìm thư mục desktop và lối tắt VNET nằm ở
// shortcuts_windows.go.
const (
	// Nhịp của vòng giữ lối tắt; lần đầu chạy ngay lúc dịch vụ bật.
	shortcutEvery = 10 * time.Second
	// Mỗi tên tối đa chừng này ký tự, cả danh sách tối đa chừng này tên.
	maxShortcutName  = 100
	maxShortcutNames = 50
	// Tên (không đuôi) của lối tắt VNET. Không bao giờ bị xoá, kể cả khi lỡ có
	// trong danh sách: xoá nó là khách mất đường mở lại cửa sổ VNET.
	vnetShortcutName = "VNET"
)

// chuanHoaLoiTat: "Game Menu.lnk, game menu\nTeamfight Tactics" → "Game Menu,Teamfight Tactics".
//
// Giữ nguyên chữ hoa/thường và thứ tự của lần xuất hiện đầu (tên lối tắt là tên
// hiển thị, không phải mã), bỏ trùng không phân biệt hoa thường. Tên không phải
// tên tệp bình thường bị bỏ chứ không sửa lại: sửa đi là có thể xoá nhầm tệp khác.
func chuanHoaLoiTat(raw string) string {
	seen := map[string]bool{}
	out := []string{}
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == '\n' || r == '\r' || r == ',' }) {
		name, ok := tenLoiTatHopLe(part)
		key := strings.ToLower(name)
		if !ok || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, name)
		if len(out) == maxShortcutNames {
			break
		}
	}
	return strings.Join(out, ",")
}

// tenLoiTatHopLe bỏ khoảng trắng và đuôi .lnk/.url (Windows ẩn đuôi nên admin
// thường gõ tên không đuôi), rồi từ chối mọi thứ không phải một tên tệp trơn:
// đường dẫn, ký tự cấm của Windows, ký tự điều khiển, "." và "..".
func tenLoiTatHopLe(raw string) (string, bool) {
	name := strings.TrimSpace(raw)
	// Lặp tới khi hết đuôi: "x.lnk.lnk" phải ra "x" như phía máy chủ, để chuẩn hoá
	// hai lần không cho ra giá trị khác lần đầu.
	for len(name) >= 4 {
		ext := name[len(name)-4:]
		if !strings.EqualFold(ext, ".lnk") && !strings.EqualFold(ext, ".url") {
			break
		}
		name = strings.TrimSpace(name[:len(name)-4])
	}
	if name == "" || name == "." || name == ".." || utf8.RuneCountInString(name) > maxShortcutName {
		return "", false
	}
	for _, r := range name {
		if unicode.IsControl(r) || strings.ContainsRune(`/\:*?<>"|`, r) {
			return "", false
		}
	}
	return name, true
}

// parseHiddenShortcuts đọc csv chính sách thành tập tên (viết thường). Chuẩn hoá
// lại một lần nữa dù máy chủ và normalizePolicy đã làm: đây là chỗ cuối cùng
// trước khi xoá tệp thật.
func parseHiddenShortcuts(csv string) map[string]bool {
	set := map[string]bool{}
	for _, name := range strings.Split(chuanHoaLoiTat(csv), ",") {
		if name != "" {
			set[strings.ToLower(name)] = true
		}
	}
	return set
}

// laLoiTatCanXoa: tên tệp có phải "<tên>.lnk" / "<tên>.url" với <tên> trong danh
// sách không (không phân biệt hoa thường). Đuôi khác thì không đụng — "Game
// Menu.txt" là của khách.
func laLoiTatCanXoa(fileName string, hidden map[string]bool) bool {
	ext := filepath.Ext(fileName)
	if !strings.EqualFold(ext, ".lnk") && !strings.EqualFold(ext, ".url") {
		return false
	}
	name := strings.ToLower(strings.TrimSuffix(fileName, ext))
	return name != strings.ToLower(vnetShortcutName) && hidden[name]
}

// shortcutsToDelete liệt kê lối tắt cần xoá TRỰC TIẾP trong dir: chỉ tệp thường.
// Thư mục trùng tên ("Game Menu.lnk\"), liên kết mềm, junction hay bất kỳ điểm
// phân tích nào khác đều bị bỏ qua, và không đi vào thư mục con. Bản thân dir
// cũng phải là thư mục thật — một junction thay chỗ "Desktop" sẽ dẫn dịch vụ
// SYSTEM ra ngoài thư mục desktop.
func shortcutsToDelete(dir string, hidden map[string]bool) []string {
	if len(hidden) == 0 {
		return nil
	}
	if fi, err := os.Lstat(dir); err != nil || fi.Mode()&os.ModeType != os.ModeDir {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !laLoiTatCanXoa(e.Name(), hidden) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if fi, err := os.Lstat(path); err != nil || !fi.Mode().IsRegular() {
			continue
		}
		out = append(out, path)
	}
	return out
}

// userDesktopDirs trả về thư mục Desktop của từng hồ sơ người dùng dưới root
// (C:\Users), trừ những hồ sơ không phải của ai: All Users, Default, Default
// User và Public (Desktop chung được xử lý riêng). Hồ sơ là liên kết bị bỏ qua.
func userDesktopDirs(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		switch strings.ToLower(e.Name()) {
		case "all users", "default", "default user", "public":
			continue
		}
		profile := filepath.Join(root, e.Name())
		if fi, err := os.Lstat(profile); err != nil || fi.Mode()&os.ModeType != os.ModeDir {
			continue
		}
		out = append(out, filepath.Join(profile, "Desktop"))
	}
	return out
}

// sweepFailed nhớ những tệp đã báo lỗi xoá để vòng 10 giây không ghi lại cùng
// một dòng mãi. Chỉ vòng giữ lối tắt (một goroutine) dùng nó.
var sweepFailed = map[string]bool{}

// sweepShortcuts xoá lối tắt trùng danh sách trong các thư mục desktop. Không có
// gì khớp thì không ghi gì.
func sweepShortcuts(dirs []string, hidden map[string]bool) {
	for _, dir := range dirs {
		for _, path := range shortcutsToDelete(dir, hidden) {
			err := os.Remove(path)
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				// Explorer hay giữ tệp một thoáng ngay sau khi nó xuất hiện.
				time.Sleep(200 * time.Millisecond)
				err = os.Remove(path)
			}
			switch {
			case err == nil:
				delete(sweepFailed, path)
				log.Printf("[shortcut] đã xoá %s", path)
			case errors.Is(err, fs.ErrNotExist):
				// Có ai xoá trước rồi.
			case !sweepFailed[path]:
				sweepFailed[path] = true
				log.Printf("[shortcut] không xoá được %s: %v", path, err)
			}
		}
	}
}

// runShortcutKeeper giữ desktop máy trạm đúng ý quán: có lối tắt VNET, không có
// lối tắt trong danh sách ẩn. Đọc chính sách mỗi nhịp nên admin đổi danh sách là
// áp được ngay mà không cần khởi động lại.
func runShortcutKeeper(ctx context.Context) {
	t := time.NewTicker(shortcutEvery)
	defer t.Stop()
	for {
		ensureVNETShortcut()
		if hidden := parseHiddenShortcuts(currentPolicy().HiddenShortcuts); len(hidden) > 0 {
			sweepShortcuts(desktopDirs(), hidden)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
