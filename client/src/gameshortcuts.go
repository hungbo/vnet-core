package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// Lối tắt game trên desktop của khách: mỗi game sẵn sàng có một "<Tên>.lnk" mở
// thẳng game, khỏi phải qua menu Game.
//
// Chạy trong GIAO DIỆN (tài khoản Windows của khách), không phải dịch vụ nền:
// thư mục là Desktop của chính người đang ngồi máy, và mọi thứ ghi/xoá ở đó đều
// dưới quyền của họ — không có đường nào để dùng quyền SYSTEM đi lạc ra ngoài
// desktop. Dịch vụ nền xoá lối tắt theo TÊN (shortcuts.go) nên tên ở đây phải
// tránh danh sách đó.
//
// Tệp này là phần thuần (chọn lối tắt, đặt tên, lập kế hoạch, chống chồng lệnh) để
// kiểm được ở bất kỳ đâu; COM và đường dẫn Desktop nằm ở gameshortcuts_windows.go.
const (
	// gameLnkPrefix mở đầu Description của mọi lối tắt do VNET tạo — vừa là tooltip
	// thân thiện, vừa là dấu duy nhất để biết tệp .lnk nào được phép xoá/ghi đè.
	gameLnkPrefix = "VNET: "
	// Tên tệp, nhãn và nhãn phân biệt trùng tên tính theo ký tự (rune), không theo byte.
	maxGameLnkStem  = 80
	maxGameLnkLabel = 120
	maxGameLnkTag   = 40
)

// errGameLnkBusy: một tiến trình khác của VNET đang đối chiếu desktop. Không phải
// lỗi thật — lần gọi sau sẽ làm lại — nên không ghi nhật ký.
var errGameLnkBusy = errors.New("tiến trình khác đang đối chiếu lối tắt game")

// gameShortcutSpec là một lối tắt cần có trên desktop.
type gameShortcutSpec struct {
	File    string // "<Tên>.lnk", không đường dẫn
	Desc    string // gameLnkPrefix + tên hiển thị
	Target  string // tệp chạy game
	Args    string // launch_args
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

// cleanShortcutName biến một chuỗi thành tên tệp (không đuôi) dùng được trên
// Windows, hoặc "" nếu không còn gì dùng được: rỗng sau khi lọc, tên thiết bị
// (CON, NUL, COM1...), hay trùng tên lối tắt VNET.
//
// Lọc chứ không thay: "Counter-Strike: GO" ra "Counter-Strike GO". U+FFFD cũng bị
// lọc, nhưng tên hiển thị có U+FFFD đã bị shortcutStem loại từ trước — bỏ đúng chữ
// hỏng rồi dùng phần còn lại sẽ ra một tên sai chính tả mà trông như thật.
func cleanShortcutName(s string) string {
	name := strings.Map(func(r rune) rune {
		if r == utf8.RuneError || unicode.IsControl(r) || strings.ContainsRune(`<>:"/\|?*`, r) {
			return -1
		}
		return r
	}, s)
	name = cutRunes(strings.TrimSpace(name), maxGameLnkStem)
	// Windows bỏ dấu chấm và khoảng trắng ở cuối tên tệp; cắt xong mới bỏ để phần
	// bị cắt không để lại đuôi như vậy.
	name = strings.TrimRightFunc(name, func(r rune) bool { return r == '.' || unicode.IsSpace(r) })
	if name == "" || strings.EqualFold(name, vnetShortcutName) || laTenThietBiWindows(name) {
		return ""
	}
	return name
}

func cutRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// laTenThietBiWindows: "CON", "nul.txt", "COM1 "... — Windows coi mọi tên bắt đầu
// bằng tên thiết bị (trước dấu chấm đầu tiên) là thiết bị, cả khi có đuôi, và
// "CON.lnk" không tạo được như một tệp.
func laTenThietBiWindows(name string) bool {
	stem, _, _ := strings.Cut(name, ".")
	up := strings.ToUpper(strings.TrimRight(stem, " "))
	switch up {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	return len(up) == 4 && (strings.HasPrefix(up, "COM") || strings.HasPrefix(up, "LPT")) &&
		up[3] >= '1' && up[3] <= '9'
}

// shortcutStem chọn tên tệp: tên hiển thị đã lọc, hoặc slug nếu tên hiển thị hỏng
// bảng mã (chứa U+FFFD — dữ liệu cũ mất dấu tiếng Việt, xem Menu() phía máy chủ)
// hay lọc xong không còn dùng được. "" nghĩa là cả hai đều không dùng được.
func shortcutStem(display, slug string) string {
	if !strings.ContainsRune(display, utf8.RuneError) {
		if s := cleanShortcutName(display); s != "" {
			return s
		}
	}
	return cleanShortcutName(slug)
}

// shortcutLabel là tên hiện trong tooltip (Description). Khác tên tệp ở chỗ không
// phải kiêng ký tự cấm của tên tệp, nhưng cũng dùng slug khi tên hiển thị hỏng.
func shortcutLabel(display, slug string) string {
	if strings.ContainsRune(display, utf8.RuneError) {
		display = slug
	}
	clean := func(s string) string {
		s = strings.Map(func(r rune) rune {
			if r == utf8.RuneError {
				return -1
			}
			if unicode.IsControl(r) {
				return ' '
			}
			return r
		}, s)
		return cutRunes(strings.TrimSpace(s), maxGameLnkLabel)
	}
	if l := clean(display); l != "" {
		return l
	}
	return clean(slug)
}

// desiredGameShortcuts dựng danh sách lối tắt cần có từ menu game.
//
// Một game có lối tắt khi: status ready, dựng được đường dẫn bằng cùng
// resolveGameLaunch mà LaunchGame dùng, và tệp chạy là tệp thường (isFile) — tức
// đúng những game bấm Chơi mở được. Tên bị hidden (danh sách lối tắt cần ẩn của
// chính sách) thì bỏ qua, vì dịch vụ nền sẽ xoá nó ngay.
//
// Kết quả không phụ thuộc thứ tự máy chủ trả: duyệt theo slug để hai game trùng
// tên luôn phân định giống nhau (game có slug nhỏ hơn giữ tên gốc), không đổi chỗ
// qua mỗi lần tải.
func desiredGameShortcuts(m *gameMenu, hidden map[string]bool, isFile func(string) bool) []gameShortcutSpec {
	items := append([]gameMenuItem(nil), m.Items...)
	sort.SliceStable(items, func(i, j int) bool { return items[i].Name < items[j].Name })

	used := map[string]bool{} // tên tệp viết thường đã có game giữ
	var out []gameShortcutSpec
	for _, g := range items {
		if g.Status != gameStatusReady {
			continue
		}
		exe, dir, err := resolveGameLaunch(m.Root, g.Name, g.Launcher)
		// NUL trong tham số: startGame từ chối, và COM cũng không nhận chuỗi như vậy.
		if err != nil || strings.ContainsRune(g.LaunchArgs, 0) || !isFile(exe) {
			continue
		}
		stem := shortcutStem(g.DisplayName, g.Name)
		if stem == "" || laLoiTatCanXoa(stem+".lnk", hidden) {
			continue
		}
		file := stem + ".lnk"
		if used[strings.ToLower(file)] {
			// Trùng tên: thêm slug để phân biệt; vẫn trùng (tên khác tình cờ giống)
			// thì thêm số, có giới hạn để không lặp mãi.
			tag := cutRunes(strings.ReplaceAll(g.Name, string(utf8.RuneError), ""), maxGameLnkTag)
			file = fmt.Sprintf("%s (%s).lnk", stem, tag)
			for n := 2; used[strings.ToLower(file)] && n < 100; n++ {
				file = fmt.Sprintf("%s (%s %d).lnk", stem, tag, n)
			}
			if used[strings.ToLower(file)] || laLoiTatCanXoa(file, hidden) {
				continue
			}
		}
		used[strings.ToLower(file)] = true
		out = append(out, gameShortcutSpec{
			File:    file,
			Desc:    gameLnkPrefix + shortcutLabel(g.DisplayName, g.Name),
			Target:  exe,
			Args:    strings.TrimSpace(g.LaunchArgs),
			WorkDir: dir,
		})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].File) < strings.ToLower(out[j].File) })
	return out
}

// gameShortcutsKey là dấu vân tay của tập lối tắt mong muốn: bằng nhau thì không
// có gì để ghi.
func gameShortcutsKey(desired []gameShortcutSpec) string {
	h := sha256.New()
	for _, s := range desired {
		for _, f := range []string{s.File, s.Desc, s.Target, s.Args, s.WorkDir} {
			h.Write([]byte(f))
			h.Write([]byte{0})
		}
		h.Write([]byte{1})
	}
	return hex.EncodeToString(h.Sum(nil))
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
// tệp bị lối tắt không phải của VNET chiếm (game đó bị bỏ qua).
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

// gameShortcutsPresent: mọi lối tắt trong expect còn nằm trên desktop dưới dạng
// tệp thường. Phép kiểm rẻ (một Lstat mỗi tệp) để biết có cần quét lại không.
func gameShortcutsPresent(dir string, expect []gameShortcutSpec) bool {
	for _, s := range expect {
		if fi, err := os.Lstat(filepath.Join(dir, s.File)); err != nil || !fi.Mode().IsRegular() {
			return false
		}
	}
	return true
}

func isRegularFile(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular()
}

// gameShortcutJob là một lần tải menu cần đối chiếu.
type gameShortcutJob struct {
	menu   *gameMenu
	hidden func() map[string]bool
}

// gameShortcutSyncer chạy việc đối chiếu ở nền, tối đa MỘT lần cùng lúc.
//
// Menu được tải lại mỗi 10 giây khi cửa sổ Game đang mở, nên nhiều lần gọi chồng
// nhau là chuyện thường: chỉ giữ bản mới nhất đang chờ (bản cũ hơn đã lỗi thời),
// không dựng một hàng goroutine.
type gameShortcutSyncer struct {
	// Tách ra để bài kiểm thay được phần chạm hệ điều hành.
	desktopDir func() (string, error)
	apply      func(dir string, desired []gameShortcutSpec) (skipped []string, err error)
	isFile     func(path string) bool

	mu      sync.Mutex
	pending *gameShortcutJob
	running bool
	logged  map[string]bool

	// Chỉ goroutine đang chạy (running) đụng tới hai trường này.
	lastKey string             // khoá tập mong muốn của lần đối chiếu thành công gần nhất
	expect  []gameShortcutSpec // lối tắt phải còn trên desktop sau lần đó
}

func newGameShortcutSyncer() *gameShortcutSyncer {
	return &gameShortcutSyncer{
		desktopDir: userDesktopDir,
		apply:      syncDesktopShortcuts,
		isFile:     isRegularFile,
		logged:     map[string]bool{},
	}
}

// gameLnks là bộ đối chiếu của tiến trình này. Thanh chính và cửa sổ phụ là hai
// tiến trình, mỗi bên một bộ; syncDesktopShortcuts tự chặn chúng ghi chồng nhau.
var gameLnks = newGameShortcutSyncer()

// submit nhận một menu mới và trả về ngay; không bao giờ chặn người gọi.
func (s *gameShortcutSyncer) submit(m *gameMenu, hidden func() map[string]bool) {
	s.mu.Lock()
	s.pending = &gameShortcutJob{menu: m, hidden: hidden}
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()
	go s.drain()
}

func (s *gameShortcutSyncer) drain() {
	for {
		s.mu.Lock()
		job := s.pending
		s.pending = nil
		if job == nil {
			s.running = false
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()
		s.runJob(job)
	}
}

func (s *gameShortcutSyncer) runJob(job *gameShortcutJob) {
	// Lối tắt chỉ là tiện ích: lỗi gì cũng không được kéo cả giao diện xuống.
	defer func() {
		if r := recover(); r != nil {
			s.lastKey = ""
			s.logOnce(fmt.Sprintf("lỗi không lường trước: %v", r))
		}
	}()

	dir, err := s.desktopDir()
	if err != nil {
		s.logOnce(fmt.Sprintf("không tìm thấy thư mục desktop: %v", err))
		return
	}
	if dir == "" { // nền tảng không có desktop kiểu Windows
		return
	}

	desired := desiredGameShortcuts(job.menu, job.hidden(), s.isFile)
	key := gameShortcutsKey(desired)
	if key == s.lastKey && gameShortcutsPresent(dir, s.expect) {
		return
	}

	skipped, err := s.apply(dir, desired)
	if err != nil {
		s.lastKey = "" // lần sau làm lại từ đầu
		if !errors.Is(err, errGameLnkBusy) {
			s.logOnce(err.Error())
		}
		return
	}
	skip := map[string]bool{}
	for _, f := range skipped {
		skip[strings.ToLower(f)] = true
		s.logOnce(fmt.Sprintf("%s đã có trên desktop và không phải của VNET — bỏ qua game này, không ghi đè", f))
	}
	s.expect = nil
	for _, d := range desired {
		if !skip[strings.ToLower(d.File)] {
			s.expect = append(s.expect, d)
		}
	}
	s.lastKey = key
}

// logOnce ghi mỗi nội dung một lần: lỗi dai dẳng (không ghi được vào Desktop...)
// sẽ lặp lại ở mỗi lần tải menu, và nhật ký không cần mười dòng giống nhau.
func (s *gameShortcutSyncer) logOnce(msg string) {
	s.mu.Lock()
	seen := s.logged[msg]
	if !seen {
		if len(s.logged) >= 64 {
			s.logged = map[string]bool{}
		}
		s.logged[msg] = true
	}
	s.mu.Unlock()
	if !seen {
		log.Printf("[lối tắt game] %s", msg)
	}
}

// hiddenShortcutNames là danh sách tên lối tắt mà dịch vụ nền sẽ xoá.
//
// Thanh chính nhận chính sách sống qua kênh dịch vụ. Cửa sổ phụ (nơi menu Game
// chạy) thì không có kênh đó, nên đọc policy.json mà dịch vụ ghi mỗi khi chính
// sách đổi — file nhỏ, đọc mỗi lần tải menu cũng không đáng kể.
func (a *App) hiddenShortcutNames() map[string]bool {
	if a.windowMode != "" {
		loadPolicyFile()
	}
	return parseHiddenShortcuts(currentPolicy().HiddenShortcuts)
}

// queueGameShortcuts đưa một menu vừa tải được vào hàng đối chiếu lối tắt desktop.
func (a *App) queueGameShortcuts(m *gameMenu) {
	gameLnks.submit(m, a.hiddenShortcutNames)
}

// refreshGameShortcuts dựng lối tắt ngay sau khi đăng nhập hoặc khôi phục phiên,
// không đợi khách mở menu Game. Chỉ thanh chính làm: cửa sổ phụ cũng khôi phục
// phiên, nhưng nó đối chiếu theo mỗi lần tải menu của chính nó.
func (a *App) refreshGameShortcuts() {
	if a.windowMode != "" || !a.loggedIn() {
		return
	}
	gameLnkTickerOnce.Do(func() { go a.gameShortcutTicker() })
	go func() {
		// fetchGameMenu tự đưa kết quả vào hàng đối chiếu.
		if _, err := a.fetchGameMenu(); err != nil {
			gameLnks.logOnce(fmt.Sprintf("không tải được menu game để dựng lối tắt: %v", err))
		}
	}()
}

// gameShortcutEvery: nhịp đối chiếu lại lối tắt khi khách đang ngồi máy.
const gameShortcutEvery = time.Minute

var gameLnkTickerOnce sync.Once

// gameShortcutTicker đối chiếu lại lối tắt mỗi phút trong lúc có người đăng nhập.
// Menu đổi sau khi khách đã vào (game chuyển sang đang cập nhật, bị tắt, bị xoá),
// mà lối tắt mở thẳng tệp chạy, không qua kiểm tra "sẵn sàng" của LaunchGame —
// không làm mới thì khách bấm vào lối tắt của game đang tải dở.
//
// Menu tải lỗi (mất mạng) thì fetchGameMenu trả lỗi trước khi đưa gì vào hàng
// đối chiếu, nên lối tắt hiện có được giữ nguyên.
func (a *App) gameShortcutTicker() {
	t := time.NewTicker(gameShortcutEvery)
	defer t.Stop()
	for range t.C {
		if !a.loggedIn() {
			continue
		}
		if _, err := a.fetchGameMenu(); err != nil {
			gameLnks.logOnce(fmt.Sprintf("không tải được menu game để đối chiếu lối tắt: %v", err))
		}
	}
}
