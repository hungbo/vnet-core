package service

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/model"
	"gorm.io/gorm"
)

// Game cục bộ: quán tự tải game (copy, launcher của hãng, cài tay…) rồi chỉ khai báo thư mục và tệp chạy để game
// hiện trên menu máy trạm. Khác game của Hub ở chỗ không có bản, không băm, không đồng bộ, không cập nhật (xem
// model.GameOriginLocal). Thư mục nằm ở GAME_ROOT/<tên> hoặc ở bất kỳ đâu trên máy chủ (thư mục của Kzone…); khi đó
// gốc game riêng (custom.client_root) là thư mục cha. Menu máy trạm ghép đường dẫn từ gốc game, thư mục game và
// launcher; tên game khác tên thư mục thì thư mục nằm ở custom.folder.

var (
	// ErrGameLocalOnMaster: master phát hành game, không có khái niệm game riêng của quán.
	ErrGameLocalOnMaster = errors.New("máy master phát hành game, không thêm game cục bộ")
	// ErrGameIsLocal: việc chỉ có nghĩa với game đến từ catalog (tải ngay từ master).
	ErrGameIsLocal = errors.New("game cục bộ do quán tự quản, không đồng bộ với Hub")
	// ErrGameNeedRoot: chưa biết thư mục game trên máy chủ nên không kiểm được thư mục và tệp chạy.
	ErrGameNeedRoot = errors.New("chưa đặt thư mục game trên máy chủ (GAME_ROOT)")
	// ErrGameLocalNeedLauncher: không có tệp chạy thì game không lên menu.
	ErrGameLocalNeedLauncher = errors.New("game cục bộ cần tệp chạy (launcher) hoặc ít nhất một mục menu riêng")
	// ErrGameHubChuaTaiCanXacNhan: tên trùng một game Hub quán chưa tải; thay được nhưng phải hỏi người dùng trước.
	ErrGameHubChuaTaiCanXacNhan = errors.New("tên này trùng với một game của Hub mà quán chưa tải: xác nhận thay bằng game cục bộ, hoặc đặt tên game khác")
)

// Giới hạn danh sách gợi ý cho hộp "Thêm game cục bộ": thư mục game ở quán lớn có thể rất nhiều tệp.
const (
	maxLocalFolders   = 500
	maxLocalLaunchers = 300
	maxLocalDepth     = 3 // số cấp thư mục dò tìm tệp chạy, tính từ thư mục game
)

// laTepChay: đuôi tệp có thể mở được từ menu máy trạm.
func laTepChay(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".exe", ".bat", ".cmd", ".lnk":
		return true
	}
	return false
}

// thuMucAn: thư mục của hệ thống phân phối, của launcher hay công cụ chứ không phải game.
func thuMucAn(name string) bool { return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") }

// tepTonTai kiểm launcher có thật trong thư mục game dir (tệp thường, không phải thư mục).
func tepTonTai(dir, rel string) bool {
	fi, err := os.Stat(filepath.Join(dir, filepath.FromSlash(strings.ReplaceAll(rel, `\`, "/"))))
	return err == nil && fi.Mode().IsRegular()
}

// duongDanMayChu kiểm và làm gọn một thư mục tuyệt đối trên máy chủ (game cục bộ ngoài GAME_ROOT). Không nhận đường
// dẫn mạng (\\máy\chia-sẻ, \\?\…): đọc thư mục ở máy khác làm máy chủ gửi thông tin đăng nhập Windows tới máy đó.
func duongDanMayChu(p string) (string, error) {
	p = strings.TrimSpace(p)
	bad := !filepath.IsAbs(p) || strings.HasPrefix(p, `\\`) || strings.HasPrefix(p, "//")
	if runtime.GOOS == "windows" && len(filepath.VolumeName(p)) != 2 { // chỉ "E:"; UNC ra "\\máy\chia-sẻ"
		bad = true
	}
	if bad {
		// %s chứ không %q: %q nhân đôi mọi dấu "\" của đường dẫn Windows.
		return "", fmt.Errorf("\"%s\" không phải đường dẫn đầy đủ trên máy chủ, ví dụ E:\\Online Games\\VALORANT (không nhận thư mục chia sẻ qua mạng)", p)
	}
	return filepath.Clean(p), nil
}

// tenThuMucDungDuoc: tên thư mục game dưới gốc (custom.folder) dùng được trên máy trạm: cùng luật checkGameComponent
// của client (client/src/games.go), cộng tên thiết bị Windows và chữ hỏng bảng mã. Tiếng Việt có dấu vẫn qua.
func tenThuMucDungDuoc(c string) bool {
	return c != "" && len(c) <= 255 && !coKyTuLoi(c) && strings.Trim(c, ". ") != "" && !thanhPhanWindowsXau(c) &&
		strings.IndexFunc(c, func(r rune) bool { return r < 0x20 || strings.ContainsRune(`<>:"|?*\/`, r) }) < 0
}

// cungThuMuc: hai đường dẫn (đã Clean) chỉ cùng một thư mục; Windows không phân biệt hoa thường.
func cungThuMuc(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// lienKetToiThuMuc: mục là junction hay symlink trỏ tới một thư mục. Từ Go 1.23, junction trên Windows mang
// ModeIrregular chứ không ModeDir, nên e.IsDir() bỏ sót thư mục game quán dẫn bằng junction sang chỗ khác.
func lienKetToiThuMuc(p string, t fs.FileMode) bool {
	if t&(fs.ModeSymlink|fs.ModeIrregular) == 0 {
		return false
	}
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// CreateLocal thêm một game cục bộ. Thư mục game (req.Folder, hoặc GAME_ROOT/<tên> khi không gửi) và tệp chạy phải có
// sẵn trên máy chủ.
func (s *GameService) CreateLocal(req *CreateGameRequest) (*model.Game, error) {
	c := s.conf()
	if c.Role == config.GameRoleMaster {
		return nil, ErrGameLocalOnMaster
	}
	if c.Root == "" {
		return nil, ErrGameNeedRoot
	}
	name := strings.TrimSpace(req.Name)
	dir := filepath.Join(c.Root, name)
	clientRoot := strings.TrimSpace(req.ClientRoot)
	folder := "" // custom.folder: chỉ khi tên game khác tên thư mục
	if strings.TrimSpace(req.Folder) != "" {
		p, err := duongDanMayChu(req.Folder)
		if err != nil {
			return nil, err
		}
		base := filepath.Base(p)
		if !tenThuMucDungDuoc(base) {
			return nil, fmt.Errorf("tên thư mục %q máy trạm không mở được: đổi tên thư mục", base)
		}
		if name == "" {
			if !tenGameDungDuoc(base) {
				return nil, fmt.Errorf("thư mục %q không dùng làm tên game được: nhập tên game riêng (chữ không dấu, số, khoảng trắng và . _ ( ) + -)", base)
			}
			name = base
		}
		if name != base {
			folder = base
		}
		dir = p
		// Máy trạm thấy ổ đĩa của máy chủ như nhau nên gốc riêng của game là thư mục cha. Game nằm ngay trong
		// GAME_ROOT thì theo ổ chung của quán như game thêm kiểu cũ (máy trạm có thể thấy GAME_ROOT dưới chữ ổ khác).
		if parent := filepath.Dir(p); clientRoot == "" && !cungThuMuc(parent, filepath.Clean(c.Root)) {
			clientRoot = parent
		}
	}
	if !tenGameDungDuoc(name) {
		return nil, errors.New("tên game chỉ gồm chữ, số, khoảng trắng và . _ ( ) + -, không quá 100 ký tự, không kết thúc bằng dấu chấm và không trùng tên thiết bị Windows (CON, NUL, COM1…)")
	}
	if err := kiemTraCacTruongChuGame(&req.DisplayName, &req.Category, &req.Launcher, &req.LaunchArgs); err != nil {
		return nil, err
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("không thấy thư mục %s", dir)
	}
	if clientRoot != "" {
		p, err := parseGameClientRoot(clientRoot)
		if err != nil {
			return nil, fmt.Errorf("gốc game trên máy trạm %q: %w", clientRoot, err)
		}
		clientRoot = p
	}
	if err := checkLauncher(req.Launcher); err != nil {
		return nil, err
	}
	if err := kiemTraIcon(req.Icon); err != nil {
		return nil, err
	}
	entries, err := chuanHoaMucMenu(req.MenuEntries)
	if err != nil {
		return nil, err
	}
	runFirst, err := chuanHoaRunFirst(req.RunFirst)
	if err != nil {
		return nil, err
	}
	launcher := strings.TrimSpace(req.Launcher)
	if launcher == "" && len(entries) == 0 {
		return nil, ErrGameLocalNeedLauncher
	}
	if launcher != "" && !tepTonTai(dir, launcher) {
		return nil, fmt.Errorf("không thấy tệp chạy %s trong thư mục game", launcher)
	}
	for _, e := range entries {
		if !tepTonTai(dir, e.Launcher) {
			return nil, fmt.Errorf("không thấy tệp chạy %s của mục %q trong thư mục game", e.Launcher, e.DisplayName)
		}
	}
	// Tên trùng một game Hub chưa có gì trên ổ (chưa tải, không bản nào): req.ReplaceHub thì xoá dòng đó và thêm game
	// cục bộ cùng tên. Lượt đọc catalog sau thấy game cục bộ thì giữ nó (noteLocalClash). Game Hub đã tải hay đang tải
	// thì không: mất bản cập nhật và bỏ rơi tệp đã nằm trong GAME_ROOT.
	replaceID := ""
	var existing model.Game
	if err := s.db.Where("name = ?", name).First(&existing).Error; err == nil {
		switch {
		case existing.Origin == model.GameOriginLocal:
			return nil, errors.New("game này đã có")
		case existing.Status != model.GameStatusNotInstalled || existing.Version != 0:
			return nil, errors.New("tên này trùng với một game của Hub đã tải hoặc đang tải: đặt tên game khác (thư mục giữ nguyên) hoặc dùng game của Hub")
		case !req.ReplaceHub:
			return nil, ErrGameHubChuaTaiCanXacNhan
		}
		replaceID = existing.ID
	}
	g := model.Game{
		Name:        name,
		DisplayName: orDefault(strings.TrimSpace(req.DisplayName), name),
		Category:    strings.TrimSpace(req.Category),
		Launcher:    launcher,
		LaunchArgs:  strings.TrimSpace(req.LaunchArgs),
		Icon:        strings.TrimSpace(req.Icon),
		MenuEntries: entries,
		RunFirst:    runFirst,
		Custom:      model.GameCustom{ClientRoot: clientRoot, Folder: folder},
		Priority:    req.Priority,
		Enabled:     true,
		Origin:      model.GameOriginLocal,
		Status:      model.GameStatusReady,
		Progress:    100,
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if replaceID != "" {
			// Cùng điều kiện như lúc kiểm: vòng nền vừa xếp game Hub vào hàng đợi thì không xoá.
			res := tx.Where("id = ? AND status = ? AND version = 0", replaceID, model.GameStatusNotInstalled).Delete(&model.Game{})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				return errors.New("game của Hub cùng tên vừa bắt đầu tải: tải lại trang rồi thử lại")
			}
		}
		return tx.Create(&g).Error
	})
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return nil, errors.New("game này đã có")
		}
		return nil, err
	}
	if replaceID != "" {
		log.Printf("[game] %s: thay game Hub chưa tải bằng game cục bộ", name)
	}
	s.baoDoi(g.ID)
	return &g, nil
}

// doiThuMucCucBo trỏ game cục bộ g sang thư mục folder (đường dẫn đầy đủ trên máy chủ) khi sửa game, cùng luật như
// CreateLocal: tên game giữ nguyên, thư mục khác tên game thì vào custom.folder, gốc máy trạm không gửi thì là thư mục
// cha. Tệp chạy (mới nếu đang sửa, không thì cũ) phải có trong thư mục mới.
func (s *GameService) doiThuMucCucBo(g *model.Game, folder string, req *UpdateGameRequest, updates map[string]interface{}, custom *model.GameCustom) error {
	if g.Origin != model.GameOriginLocal {
		return errors.New("chỉ game cục bộ đổi được thư mục game")
	}
	root := s.conf().Root
	if root == "" {
		return ErrGameNeedRoot
	}
	dir, err := duongDanMayChu(folder)
	if err != nil {
		return err
	}
	base := filepath.Base(dir)
	if !tenThuMucDungDuoc(base) {
		return fmt.Errorf("tên thư mục %q máy trạm không mở được: đổi tên thư mục", base)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return fmt.Errorf("không thấy thư mục %s", dir)
	}
	launcher, entries := g.Launcher, g.MenuEntries
	if v, ok := updates["launcher"].(string); ok {
		launcher = v
	}
	if v, ok := updates["menu_entries"].(model.GameMenuEntries); ok {
		entries = v
	}
	if launcher != "" && !tepTonTai(dir, launcher) {
		return fmt.Errorf("không thấy tệp chạy %s trong thư mục %s", launcher, dir)
	}
	for _, e := range entries {
		if !tepTonTai(dir, e.Launcher) {
			return fmt.Errorf("không thấy tệp chạy %s của mục %q trong thư mục %s", e.Launcher, e.DisplayName, dir)
		}
	}
	custom.Folder = ""
	if base != g.Name {
		custom.Folder = base
	}
	if req.ClientRoot == nil || strings.TrimSpace(*req.ClientRoot) == "" {
		custom.ClientRoot = ""
		if parent := filepath.Dir(dir); !cungThuMuc(parent, filepath.Clean(root)) {
			p, err := parseGameClientRoot(parent)
			if err != nil {
				return fmt.Errorf("gốc game trên máy trạm %q: %w", parent, err)
			}
			custom.ClientRoot = p
		}
	}
	return nil
}

// noteLocalClash ghi nhận một game cục bộ trùng tên với game của catalog; nhật ký chỉ một dòng cho mỗi tên.
func (s *GameService) noteLocalClash(name string) {
	s.mu.Lock()
	if s.localClash == nil {
		s.localClash = map[string]bool{}
	}
	first := !s.localClash[name]
	s.localClash[name] = true
	s.mu.Unlock()
	if first {
		log.Printf("[game] %s: catalog cũng có game này nhưng quán đã thêm game cục bộ cùng tên, giữ bản cục bộ", name)
	}
}

// LocalFolders liệt kê thư mục con chưa là game nào của root (thư mục tuyệt đối trên máy chủ; rỗng là GAME_ROOT), để
// chọn khi thêm game cục bộ. Bỏ tên và thư mục của các game cục bộ đã có. Tên game của Hub chỉ bỏ trong GAME_ROOT (đó
// là thư mục Hub tải về); ở nơi khác, thư mục trùng tên game Hub là bản riêng của quán, thêm được dưới tên khác.
func (s *GameService) LocalFolders(root string) ([]string, error) {
	c := s.conf()
	if c.Role == config.GameRoleMaster {
		return nil, ErrGameLocalOnMaster
	}
	if c.Root == "" {
		return nil, ErrGameNeedRoot
	}
	dir := c.Root
	if strings.TrimSpace(root) != "" {
		p, err := duongDanMayChu(root)
		if err != nil {
			return nil, err
		}
		dir = p
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("không đọc được thư mục game: %w", err)
	}
	var have []model.Game
	if err := s.db.Select("name", "origin", "custom").Find(&have).Error; err != nil {
		return nil, err
	}
	inRoot := cungThuMuc(filepath.Clean(dir), filepath.Clean(c.Root))
	taken := make(map[string]bool, len(have))
	for _, g := range have {
		if g.Origin == model.GameOriginLocal || inRoot {
			taken[strings.ToLower(g.Name)] = true
		}
		if g.Custom.Folder != "" {
			taken[strings.ToLower(g.Custom.Folder)] = true
		}
	}
	out := []string{}
	for _, e := range entries {
		n := e.Name()
		if thuMucAn(n) || !tenThuMucDungDuoc(n) || taken[strings.ToLower(n)] {
			continue
		}
		if !e.IsDir() && !lienKetToiThuMuc(filepath.Join(dir, n), e.Type()) {
			continue
		}
		out = append(out, n)
		if len(out) >= maxLocalFolders {
			break
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	return out, nil
}

// LocalLaunchers liệt kê tệp chạy (.exe, .bat, .cmd, .lnk) trong một thư mục game, đường dẫn tương đối kiểu "Bin/Game.exe".
// folder là tên thư mục trong GAME_ROOT hoặc đường dẫn tuyệt đối trên máy chủ. Tệp ở cấp nông trước. Không đi theo liên
// kết bên trong thư mục game (chính thư mục game là junction thì vẫn dò), bỏ thư mục ẩn và thư mục của hệ thống phân phối.
func (s *GameService) LocalLaunchers(folder string) ([]string, error) {
	c := s.conf()
	if c.Role == config.GameRoleMaster {
		return nil, ErrGameLocalOnMaster
	}
	if c.Root == "" {
		return nil, ErrGameNeedRoot
	}
	folder = strings.TrimSpace(folder)
	var base string
	if tenGameDungDuoc(folder) { // cũng là rào chặn "..", dấu gạch và ổ đĩa
		base = filepath.Join(c.Root, folder)
	} else if p, err := duongDanMayChu(folder); err == nil {
		base = p
	} else {
		return nil, errors.New("thư mục game không hợp lệ: cần tên thư mục trong GAME_ROOT hoặc đường dẫn đầy đủ trên máy chủ")
	}
	if fi, err := os.Stat(base); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("không thấy thư mục %s", base)
	}
	out := []string{}
	// fs.WalkDir trên os.DirFS lấy gốc "." bằng Stat (đi theo junction/symlink của thư mục game), còn các mục bên
	// trong đến từ ReadDir nên liên kết lồng bên trong không được đi theo. filepath.WalkDir dùng Lstat cho gốc nên
	// gặp thư mục game là junction thì không dò gì.
	_ = fs.WalkDir(os.DirFS(base), ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // thư mục không đọc được thì bỏ qua, không dừng cả lượt dò
		}
		if rel == "." {
			return nil
		}
		depth := strings.Count(rel, "/")
		if d.IsDir() {
			if thuMucAn(d.Name()) || depth >= maxLocalDepth {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() && laTepChay(d.Name()) {
			out = append(out, rel)
			if len(out) >= maxLocalLaunchers {
				return fs.SkipAll
			}
		}
		return nil
	})
	sort.SliceStable(out, func(i, j int) bool {
		di, dj := strings.Count(out[i], "/"), strings.Count(out[j], "/")
		if di != dj {
			return di < dj
		}
		return strings.ToLower(out[i]) < strings.ToLower(out[j])
	})
	return out, nil
}
