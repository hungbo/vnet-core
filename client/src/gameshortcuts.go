package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Lối tắt "Menu Game" trên desktop của khách: mở menu Game của VNET. Lối tắt chạy một
// bản thứ hai của vnet-client.exe với tham số menuGameArg; bản đó gõ cửa thanh điều
// khiển đang chạy (SingleInstanceLock, main.go) rồi thoát, và thanh mở menu Game.
//
// Trước đây mỗi game có một lối tắt riêng trên desktop. Quán muốn desktop gọn, chỉ
// một cửa vào là menu Game (15/10/2026): lần đối chiếu đầu tiên sau khi đăng nhập xoá
// các lối tắt game cũ, nhận ra nhờ Description mở đầu bằng gameLnkPrefix.
//
// Chạy trong GIAO DIỆN (tài khoản Windows của khách), không phải dịch vụ nền: thư mục
// là Desktop của chính người đang ngồi máy, và mọi thứ ghi/xoá ở đó đều dưới quyền của
// họ. Tệp này là phần thuần (chọn lối tắt, lập kế hoạch) để kiểm được ở bất kỳ đâu;
// COM và đường dẫn Desktop nằm ở gameshortcuts_windows.go.
const (
	// gameLnkPrefix mở đầu Description của mọi lối tắt do VNET tạo — vừa là tooltip
	// thân thiện, vừa là dấu duy nhất để biết tệp .lnk nào được phép xoá/ghi đè.
	gameLnkPrefix = "VNET: "

	menuGameLnkFile = "Menu Game.lnk"
	// menuGameArg: tham số lối tắt truyền cho vnet-client.exe (xem moBangTuThamSo).
	menuGameArg = "--open=games"
)

// errGameLnkBusy: một tiến trình khác của VNET đang đối chiếu desktop. Không phải
// lỗi thật — lần đăng nhập sau sẽ làm lại — nên không ghi nhật ký.
var errGameLnkBusy = errors.New("tiến trình khác đang đối chiếu lối tắt game")

// gameShortcutSpec là một lối tắt cần có trên desktop.
type gameShortcutSpec struct {
	File    string // "<Tên>.lnk", không đường dẫn
	Desc    string // gameLnkPrefix + tên hiển thị
	Target  string // tệp lối tắt chạy
	Args    string // tham số dòng lệnh
	WorkDir string // thư mục chứa tệp chạy, giống startGame
}

// desktopLnk là một tệp .lnk đang có trên desktop.
type desktopLnk struct {
	File string
	// Ours: tệp thường có Description mở đầu bằng gameLnkPrefix. Chỉ khi đó phần
	// còn lại (Desc..WorkDir) mới có nghĩa; tệp của người khác không bị đọc thêm.
	Ours                        bool
	Desc, Target, Args, WorkDir string
}

// gameShortcutPlan là việc cần làm để desktop khớp với danh sách mong muốn.
type gameShortcutPlan struct {
	Create []gameShortcutSpec
	Update []gameShortcutSpec
	Delete []string // tên tệp .lnk của VNET không còn được mong muốn
	Skip   []string // tên tệp mong muốn nhưng đã bị tệp không phải của VNET chiếm
}

// sameGameLnk: lối tắt đang có đã đúng như mong muốn chưa. Đường dẫn so không
// phân biệt hoa thường như Windows; tham số và Description phải khớp từng chữ.
func sameGameLnk(want gameShortcutSpec, have desktopLnk) bool {
	samePath := func(a, b string) bool { return strings.EqualFold(filepath.Clean(a), filepath.Clean(b)) }
	return samePath(want.Target, have.Target) && samePath(want.WorkDir, have.WorkDir) &&
		want.Args == have.Args && want.Desc == have.Desc
}

// planGameShortcuts đối chiếu mong muốn với desktop. Không ghi lại lối tắt đã
// đúng; không bao giờ đề nghị xoá hay ghi đè tệp không phải của VNET.
func planGameShortcuts(desired []gameShortcutSpec, existing []desktopLnk) gameShortcutPlan {
	have := make(map[string]desktopLnk, len(existing))
	for _, e := range existing {
		have[strings.ToLower(e.File)] = e
	}
	want := make(map[string]bool, len(desired))
	var p gameShortcutPlan
	for _, s := range desired {
		key := strings.ToLower(s.File)
		want[key] = true
		switch e, ok := have[key]; {
		case !ok:
			p.Create = append(p.Create, s)
		case !e.Ours:
			p.Skip = append(p.Skip, s.File)
		case !sameGameLnk(s, e):
			p.Update = append(p.Update, s)
		}
	}
	for _, e := range existing {
		if e.Ours && !want[strings.ToLower(e.File)] {
			p.Delete = append(p.Delete, e.File)
		}
	}
	sort.Strings(p.Delete)
	return p
}

// applyGameShortcutPlan thực hiện kế hoạch trong dir. Chỉ đụng tệp THƯỜNG: liên
// kết mềm, junction hay thư mục trùng tên bị bỏ qua (xoá hay ghi qua chúng là
// động tới thứ khác). Lỗi từng tệp được gom lại, không dừng giữa chừng.
func applyGameShortcutPlan(dir string, p gameShortcutPlan, write func(path string, s gameShortcutSpec) error) error {
	var errs []error
	for _, name := range p.Delete {
		path := filepath.Join(dir, name)
		fi, err := os.Lstat(path)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				errs = append(errs, err)
			}
			continue
		}
		if !fi.Mode().IsRegular() {
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
			continue
		}
		log.Printf("[lối tắt game] đã xoá %s", path)
	}
	for _, s := range p.Create {
		path := filepath.Join(dir, s.File)
		// Lúc quét chưa có, nhưng có thể vừa có ai tạo: Create không bao giờ ghi đè.
		if _, err := os.Lstat(path); err == nil {
			errs = append(errs, fmt.Errorf("%s vừa xuất hiện, không ghi đè", path))
			continue
		}
		if err := write(path, s); err != nil {
			errs = append(errs, fmt.Errorf("tạo %s: %w", path, err))
			continue
		}
		log.Printf("[lối tắt game] đã tạo %s → %s", path, s.Target)
	}
	for _, s := range p.Update {
		path := filepath.Join(dir, s.File)
		if fi, err := os.Lstat(path); err != nil || !fi.Mode().IsRegular() {
			errs = append(errs, fmt.Errorf("%s không còn là tệp thường, không ghi", path))
			continue
		}
		if err := write(path, s); err != nil {
			errs = append(errs, fmt.Errorf("cập nhật %s: %w", path, err))
			continue
		}
		log.Printf("[lối tắt game] đã cập nhật %s → %s", path, s.Target)
	}
	return errors.Join(errs...)
}

// reconcileDesktop quét dir bằng scan, lập kế hoạch rồi làm theo. Trả về các tên
// tệp bị lối tắt không phải của VNET chiếm (bỏ qua, không ghi đè).
func reconcileDesktop(dir string, desired []gameShortcutSpec,
	scan func(dir string) ([]desktopLnk, error), write func(path string, s gameShortcutSpec) error) ([]string, error) {
	fi, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("%s không phải thư mục", dir)
	}
	existing, err := scan(dir)
	if err != nil {
		return nil, fmt.Errorf("đọc desktop %s: %w", dir, err)
	}
	plan := planGameShortcuts(desired, existing)
	return plan.Skip, applyGameShortcutPlan(dir, plan, write)
}

// desiredMenuShortcuts: lối tắt VNET cần có trên desktop của khách — chỉ "Menu Game",
// trừ khi quán đã ẩn tên đó bằng chính sách (dịch vụ nền sẽ xoá nó ngay). Danh sách
// rỗng vẫn có việc: đối chiếu xoá mọi lối tắt game cũ của VNET.
func desiredMenuShortcuts(exe string, hidden map[string]bool) []gameShortcutSpec {
	if exe == "" || laLoiTatCanXoa(menuGameLnkFile, hidden) {
		return nil
	}
	return []gameShortcutSpec{{
		File:    menuGameLnkFile,
		Desc:    gameLnkPrefix + "Menu Game",
		Target:  exe,
		Args:    menuGameArg,
		WorkDir: filepath.Dir(exe),
	}}
}

// moBangTuThamSo: màn hình mà lối tắt đòi mở, đọc từ tham số dòng lệnh của bản thứ
// hai (SecondInstanceData.Args). Chỉ nhận menu Game; tham số khác là "".
func moBangTuThamSo(args []string) string {
	for i, a := range args {
		var v string
		switch {
		case a == "--open" || a == "-open":
			if i+1 < len(args) {
				v = args[i+1]
			}
		case strings.HasPrefix(a, "--open="):
			v = strings.TrimPrefix(a, "--open=")
		case strings.HasPrefix(a, "-open="):
			v = strings.TrimPrefix(a, "-open=")
		default:
			continue
		}
		if strings.EqualFold(v, "games") {
			return "games"
		}
	}
	return ""
}

// refreshGameShortcuts đối chiếu desktop ngay sau khi đăng nhập hoặc khôi phục phiên.
// Chỉ thanh chính làm: cửa sổ phụ cũng khôi phục phiên nhưng không đụng desktop.
// Lối tắt chỉ là tiện ích: lỗi gì cũng chỉ ghi nhật ký, không kéo giao diện xuống.
func (a *App) refreshGameShortcuts() {
	if a.windowMode != "" || !a.loggedIn() {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[lối tắt game] lỗi không lường trước: %v", r)
			}
		}()
		dir, err := userDesktopDir()
		if err != nil {
			log.Printf("[lối tắt game] không tìm thấy thư mục desktop: %v", err)
			return
		}
		if dir == "" { // nền tảng không có desktop kiểu Windows
			return
		}
		exe, err := os.Executable()
		if err != nil {
			exe = ""
		}
		desired := desiredMenuShortcuts(exe, parseHiddenShortcuts(currentPolicy().HiddenShortcuts))
		skipped, err := syncDesktopShortcuts(dir, desired)
		if err != nil && !errors.Is(err, errGameLnkBusy) {
			log.Printf("[lối tắt game] %v", err)
		}
		for _, f := range skipped {
			log.Printf("[lối tắt game] %s đã có trên desktop và không phải của VNET — không ghi đè", f)
		}
	}()
}
