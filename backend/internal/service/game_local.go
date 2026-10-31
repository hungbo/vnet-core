package service

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/model"
)

// Game cục bộ: quán tự tải game (copy, launcher của hãng, cài tay…) rồi chỉ khai báo thư mục và tệp chạy để game
// hiện trên menu máy trạm. Khác game của Hub ở chỗ không có bản, không băm, không đồng bộ, không cập nhật (xem
// model.GameOriginLocal). Thư mục vẫn nằm ở GAME_ROOT/<tên> vì menu máy trạm ghép đường dẫn từ gốc game, tên game và
// launcher.

var (
	// ErrGameLocalOnMaster: master phát hành game, không có khái niệm game riêng của quán.
	ErrGameLocalOnMaster = errors.New("máy master phát hành game, không thêm game cục bộ")
	// ErrGameIsLocal: việc chỉ có nghĩa với game đến từ catalog (tải ngay từ master).
	ErrGameIsLocal = errors.New("game cục bộ do quán tự quản, không đồng bộ với Hub")
	// ErrGameNeedRoot: chưa biết thư mục game trên máy chủ nên không kiểm được thư mục và tệp chạy.
	ErrGameNeedRoot = errors.New("chưa đặt thư mục game trên máy chủ (GAME_ROOT)")
	// ErrGameLocalNeedLauncher: không có tệp chạy thì game không lên menu.
	ErrGameLocalNeedLauncher = errors.New("game cục bộ cần tệp chạy (launcher) hoặc ít nhất một mục menu riêng")
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

// tepTonTai kiểm launcher có thật trong thư mục game (tệp thường, không phải thư mục).
func tepTonTai(root, game, rel string) bool {
	fi, err := os.Stat(filepath.Join(root, game, filepath.FromSlash(strings.ReplaceAll(rel, `\`, "/"))))
	return err == nil && fi.Mode().IsRegular()
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

// CreateLocal thêm một game cục bộ. Thư mục GAME_ROOT/<tên> và tệp chạy phải có sẵn trên máy chủ.
func (s *GameService) CreateLocal(req *CreateGameRequest) (*model.Game, error) {
	c := s.conf()
	if c.Role == config.GameRoleMaster {
		return nil, ErrGameLocalOnMaster
	}
	if c.Root == "" {
		return nil, ErrGameNeedRoot
	}
	name := strings.TrimSpace(req.Name)
	if !tenGameDungDuoc(name) {
		return nil, errors.New("tên game chỉ gồm chữ, số, khoảng trắng và . _ ( ) + -, không quá 100 ký tự, không kết thúc bằng dấu chấm và không trùng tên thiết bị Windows (CON, NUL, COM1…)")
	}
	if err := kiemTraCacTruongChuGame(&req.DisplayName, &req.Category, &req.Launcher, &req.LaunchArgs); err != nil {
		return nil, err
	}
	if fi, err := os.Stat(filepath.Join(c.Root, name)); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("không thấy thư mục %s", filepath.Join(c.Root, name))
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
	if launcher != "" && !tepTonTai(c.Root, name, launcher) {
		return nil, fmt.Errorf("không thấy tệp chạy %s trong thư mục game", launcher)
	}
	for _, e := range entries {
		if !tepTonTai(c.Root, name, e.Launcher) {
			return nil, fmt.Errorf("không thấy tệp chạy %s của mục %q trong thư mục game", e.Launcher, e.DisplayName)
		}
	}
	var existing model.Game
	if err := s.db.Where("name = ?", name).First(&existing).Error; err == nil {
		if existing.Origin == model.GameOriginLocal {
			return nil, errors.New("game này đã có")
		}
		return nil, errors.New("tên này trùng với một game của Hub; đổi tên thư mục hoặc dùng game của Hub")
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
		Priority:    req.Priority,
		Enabled:     true,
		Origin:      model.GameOriginLocal,
		Status:      model.GameStatusReady,
		Progress:    100,
	}
	if err := s.db.Create(&g).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return nil, errors.New("game này đã có")
		}
		return nil, err
	}
	s.baoDoi(g.ID)
	return &g, nil
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

// LocalFolders liệt kê thư mục con của GAME_ROOT chưa là game nào, để chọn khi thêm game cục bộ.
func (s *GameService) LocalFolders() ([]string, error) {
	c := s.conf()
	if c.Role == config.GameRoleMaster {
		return nil, ErrGameLocalOnMaster
	}
	if c.Root == "" {
		return nil, ErrGameNeedRoot
	}
	entries, err := os.ReadDir(c.Root)
	if err != nil {
		return nil, fmt.Errorf("không đọc được thư mục game: %w", err)
	}
	var have []string
	if err := s.db.Model(&model.Game{}).Pluck("name", &have).Error; err != nil {
		return nil, err
	}
	taken := make(map[string]bool, len(have))
	for _, n := range have {
		taken[strings.ToLower(n)] = true
	}
	out := []string{}
	for _, e := range entries {
		n := e.Name()
		if thuMucAn(n) || !tenGameDungDuoc(n) || taken[strings.ToLower(n)] {
			continue
		}
		if !e.IsDir() && !lienKetToiThuMuc(filepath.Join(c.Root, n), e.Type()) {
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
// Tệp ở cấp nông trước. Không đi theo liên kết bên trong thư mục game (chính thư mục game là junction thì vẫn dò),
// bỏ thư mục ẩn và thư mục của hệ thống phân phối.
func (s *GameService) LocalLaunchers(folder string) ([]string, error) {
	c := s.conf()
	if c.Role == config.GameRoleMaster {
		return nil, ErrGameLocalOnMaster
	}
	if c.Root == "" {
		return nil, ErrGameNeedRoot
	}
	folder = strings.TrimSpace(folder)
	if !tenGameDungDuoc(folder) { // cũng là rào chặn "..", dấu gạch và ổ đĩa
		return nil, errors.New("tên thư mục game không hợp lệ")
	}
	base := filepath.Join(c.Root, folder)
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
