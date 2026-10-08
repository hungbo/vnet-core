package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/gameupdate"
	"github.com/vnet/core/internal/model"
	"gorm.io/gorm"
)

// Cập nhật game (research/game-update-plan.md).
//
// Master giữ bản gốc của mọi game trong GAME_ROOT/<tên game>. Khi thư mục một
// game đứng yên đủ GAME_STABLE_FOR sau lần đổi cuối, master băm lại, lưu .torrent
// và tăng số phiên bản. Các quán đọc catalog của master, tải torrent rồi để
// engine ghi đè thẳng lên thư mục game của mình, chỉ tải các piece đã đổi; dữ
// liệu đi qua webseed HTTP của master (GET /seed/...).
//
// Máy trạm Cloud Update chỉ thấy Game Disk mới sau khi khởi động lại (đã kiểm
// chứng 07/10/2026), nên quán có thể giới hạn việc ghi vào một khung giờ.

// GamesGroup là nhóm cài đặt của tính năng cập nhật game.
const GamesGroup = "games"

// GameUpdateWindowKey: khung giờ quán được phép tải game, "HH:MM-HH:MM", rỗng là lúc nào cũng được.
const GameUpdateWindowKey = "update_window"

// GameClientRootKey: đường dẫn GAME_ROOT như MÁY TRẠM nhìn thấy. GAME_ROOT là
// đường dẫn trên máy chủ; máy trạm diskless thấy cùng thư mục đó qua Game Disk
// với một chữ ổ riêng, nên menu game cần biết chữ ổ để chạy game.
const GameClientRootKey = "client_root"

// GameClientRootDefault là đường dẫn dùng khi quán chưa đặt hoặc đặt sai.
const GameClientRootDefault = `D:\Games`

// duongDanOWindows: "X:\...", "X:/..." hoặc riêng ổ "X:". Chữ ổ dính liền tên thư
// mục ("D:Games") là đường dẫn tương đối theo thư mục hiện hành của ổ đó, không
// chạy game được. Riêng ổ "X:" phải nhận vì đó chính là dạng normalizeGameClientRoot
// trả ra cho ổ gốc: ô trên trang quản trị nạp lại giá trị đó rồi Lưu thì không được hỏng.
var duongDanOWindows = regexp.MustCompile(`^[A-Za-z]:([\\/]|$)`)

// Tên game là tên thư mục trên ổ game: chặn mọi thứ có thể thoát ra ngoài GAME_ROOT.
var tenGameHopLe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._()+-]{0,99}$`)

// tenGameDungDuoc: tenGameHopLe cộng các tên Windows hiểu khác ý người đặt (xem thanhPhanWindowsXau).
// Dùng ở mọi nơi nhận tên game — Create, catalog của master, và SeedServable.
func tenGameDungDuoc(name string) bool {
	return tenGameHopLe.MatchString(name) && !strings.Contains(name, "..") && !thanhPhanWindowsXau(name)
}

// thanhPhanWindowsXau: Windows bỏ dấu chấm và khoảng trắng ở cuối ("Game." thành "Game") và coi
// CON, PRN, AUX, NUL, COM1-9, LPT1-9 (kể cả khi có đuôi, "con.txt") là thiết bị chứ không phải tệp —
// thư mục hay tệp mang tên đó không tạo hay mở được như người đặt tên tưởng.
func thanhPhanWindowsXau(c string) bool {
	if strings.TrimRight(c, ". ") != c {
		return true
	}
	base := strings.ToUpper(strings.TrimRight(strings.SplitN(c, ".", 2)[0], " "))
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return true
	}
	return len(base) == 4 && (base[:3] == "COM" || base[:3] == "LPT") && base[3] >= '1' && base[3] <= '9'
}

var (
	ErrGameRoleOff    = errors.New("máy chủ này chưa bật cập nhật game (GAME_ROLE)")
	ErrGameMasterOnly = errors.New("chỉ máy master mới làm được việc này")
	ErrGameCafeOnly   = errors.New("chỉ máy chủ quán mới làm được việc này")
)

type GameService struct {
	db     *gorm.DB
	cfg    config.GameConfig
	engine *gameupdate.Engine
	http   *http.Client

	mu          sync.Mutex
	runCtx      context.Context // context của vòng nền; nil khi chưa chạy
	pubMu       sync.Mutex      // một lần publish một lúc: số phiên bản không được trùng
	stabilizers map[string]*gameupdate.Stabilizer
	kick        chan struct{}
	wake        chan struct{}         // vòng đọc catalog đánh thức luồng tải game (quán)
	current     *installJob           // game đang tải; nil khi luồng tải rảnh
	retry       map[string]*retryInfo // game tải lỗi: bao giờ được thử lại
}

// installJob là lượt tải đang chạy, để lượt đọc catalog sau hủy được khi master đã có bản mới hơn.
type installJob struct {
	name     string
	infohash string
	cancel   context.CancelCauseFunc
}

// retryInfo: game tải lỗi nhiều lần thì giãn dần khoảng chờ; bản mới trên master (infohash khác) xoá lịch sử.
type retryInfo struct {
	infohash  string
	fails     int
	notBefore time.Time
}

// errSuperseded là lý do hủy một lượt tải: master đã publish bản mới hơn bản đang tải.
var errSuperseded = errors.New("master đã có bản mới hơn")

// Khoảng chờ thử lại một game tải lỗi: gấp đôi sau mỗi lần lỗi, từ 5 phút tới tối đa 1 giờ.
const (
	gameRetryBase = 5 * time.Minute
	gameRetryMax  = time.Hour
)

func NewGameService(db *gorm.DB, cfg config.GameConfig) *GameService {
	return &GameService{
		db:          db,
		cfg:         cfg,
		http:        &http.Client{Timeout: 60 * time.Second},
		stabilizers: map[string]*gameupdate.Stabilizer{},
		kick:        make(chan struct{}, 1),
		wake:        make(chan struct{}, 1),
		retry:       map[string]*retryInfo{},
	}
}

func (s *GameService) Role() string { return s.cfg.Role }

func (s *GameService) getRunCtx() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runCtx
}

// getEngine: engine do vòng nền tạo và đóng, còn handler đọc nó từ goroutine khác.
func (s *GameService) getEngine() *gameupdate.Engine {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.engine
}

// GameInfo cho trang quản trị biết máy chủ này đóng vai gì.
type GameInfo struct {
	Role         string `json:"role"`
	Root         string `json:"root"`
	UpstreamURL  string `json:"upstream_url"`
	UpdateWindow string `json:"update_window"`
	// ClientRoot đã chuẩn hoá: trang quản trị thấy đúng giá trị menu game đang dùng.
	ClientRoot string `json:"client_root"`
}

func (s *GameService) Info() GameInfo {
	st := settingsGroup(s.db, GamesGroup)
	return GameInfo{
		Role:         s.cfg.Role,
		Root:         s.cfg.Root,
		UpstreamURL:  s.cfg.UpstreamURL,
		UpdateWindow: st[GameUpdateWindowKey],
		ClientRoot:   normalizeGameClientRoot(st[GameClientRootKey]),
	}
}

// normalizeGameClientRoot trả gốc game để máy trạm dùng, hoặc mặc định nếu cài đặt
// rỗng/sai — một ô gõ sai không được làm menu game mở ra mà không chạy được game nào.
func normalizeGameClientRoot(raw string) string {
	if p, err := parseGameClientRoot(raw); err == nil {
		return p
	}
	return GameClientRootDefault
}

// parseGameClientRoot: cắt khoảng trắng, đổi "/" thành "\", gộp các dấu "\" liền
// nhau và bỏ dấu "\" ở cuối. Mỗi thành phần phải qua cùng luật với gameRootBase của
// máy trạm (client/src/games.go) — máy chủ mà nhận cái máy trạm từ chối thì menu
// hiện "Sẵn sàng" mà không game nào chạy được. Chỉ nhận chữ ổ, không nhận UNC:
// ShellExecute tới một thư mục chia sẻ lạ là chạy mã từ xa.
//
// Ổ gốc "D:\" ra "D:": máy trạm luôn ghép root + `\` + tên game, không dùng
// filepath.Join (trên Windows Join("D:", "x") ra "D:x", đường dẫn tương đối).
func parseGameClientRoot(raw string) (string, error) {
	p := strings.ReplaceAll(strings.TrimSpace(raw), "/", `\`)
	if !duongDanOWindows.MatchString(p) {
		return "", errors.New(`phải là đường dẫn dạng D:\Games`)
	}
	var parts []string
	for _, c := range strings.Split(p[2:], `\`) {
		if c == "" { // gạch lặp ("D:\\Games") hoặc gạch cuối
			continue
		}
		if strings.IndexFunc(c, func(r rune) bool { return r < 0x20 || strings.ContainsRune(`<>:"|?*`, r) }) >= 0 {
			return "", fmt.Errorf("thành phần %q có ký tự không hợp lệ", c)
		}
		// Windows bỏ dấu chấm và khoảng trắng ở cuối thành phần, nên ".", ".." hay
		// "Games." đều mơ hồ: từ chối hết thay vì đoán hệ điều hành hiểu ra sao.
		if strings.TrimRight(c, ". ") != c {
			return "", fmt.Errorf("thành phần %q kết thúc bằng dấu chấm hoặc khoảng trắng", c)
		}
		parts = append(parts, c)
	}
	if len(parts) == 0 {
		return p[:2], nil
	}
	return p[:2] + `\` + strings.Join(parts, `\`), nil
}

// kiemTraCaiDatGame từ chối gốc game không dùng được ngay lúc lưu, để quản trị
// viên thấy lỗi thay vì thấy menu lặng lẽ quay về mặc định. Rỗng là hợp lệ (dùng
// mặc định). Giá trị cũ đã nằm sẵn trong DB thì vẫn được normalizeGameClientRoot đỡ.
func kiemTraCaiDatGame(in map[string]interface{}) error {
	v, ok := in[GameClientRootKey]
	if !ok {
		return nil
	}
	raw, isStr := v.(string)
	if !isStr {
		return errors.New("thư mục game trên máy trạm phải là chuỗi")
	}
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	if _, err := parseGameClientRoot(raw); err != nil {
		return fmt.Errorf("thư mục game trên máy trạm %q: %w", raw, err)
	}
	return nil
}

// ---------------------------------------------------------------- CRUD

func (s *GameService) List() ([]model.Game, error) {
	var games []model.Game
	if err := s.db.Order("priority DESC, display_name ASC").Find(&games).Error; err != nil {
		return nil, err
	}
	// Phần trăm đang tải lấy thẳng từ engine: ghi DB mỗi giây là thừa.
	if e := s.getEngine(); e != nil {
		for i := range games {
			if games[i].Status != model.GameStatusDownloading {
				continue
			}
			if p, ok := e.Progress(games[i].Name); ok && p.Length > 0 {
				games[i].Progress = int(p.BytesCompleted * 100 / p.Length)
			}
		}
	}
	return games, nil
}

type CreateGameRequest struct {
	Name        string `json:"name" binding:"required"`
	DisplayName string `json:"display_name"`
	Category    string `json:"category"`
	Launcher    string `json:"launcher"`
	LaunchArgs  string `json:"launch_args"`
	Priority    int    `json:"priority"`
}

// Create thêm một game ở master. Thư mục GAME_ROOT/<name> phải có sẵn.
func (s *GameService) Create(req *CreateGameRequest) (*model.Game, error) {
	if s.cfg.Role != config.GameRoleMaster {
		return nil, ErrGameMasterOnly
	}
	name := strings.TrimSpace(req.Name)
	if !tenGameDungDuoc(name) {
		return nil, errors.New("tên game chỉ gồm chữ, số, khoảng trắng và . _ ( ) + -, không quá 100 ký tự, không kết thúc bằng dấu chấm và không trùng tên thiết bị Windows (CON, NUL, COM1…)")
	}
	if fi, err := os.Stat(filepath.Join(s.cfg.Root, name)); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("không thấy thư mục %s", filepath.Join(s.cfg.Root, name))
	}
	if err := checkLauncher(req.Launcher); err != nil {
		return nil, err
	}
	g := model.Game{
		Name:        name,
		DisplayName: orDefault(strings.TrimSpace(req.DisplayName), name),
		Category:    strings.TrimSpace(req.Category),
		Launcher:    strings.TrimSpace(req.Launcher),
		LaunchArgs:  strings.TrimSpace(req.LaunchArgs),
		Priority:    req.Priority,
		Enabled:     true,
		AutoUpdate:  true,
		Status:      model.GameStatusNotInstalled,
	}
	if err := s.db.Create(&g).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return nil, errors.New("game này đã có")
		}
		return nil, err
	}
	s.Kick()
	return &g, nil
}

type UpdateGameRequest struct {
	DisplayName *string `json:"display_name"`
	Category    *string `json:"category"`
	Launcher    *string `json:"launcher"`
	LaunchArgs  *string `json:"launch_args"`
	Enabled     *bool   `json:"enabled"`
	AutoUpdate  *bool   `json:"auto_update"`
	Priority    *int    `json:"priority"`
}

func (s *GameService) Update(id string, req *UpdateGameRequest) (*model.Game, error) {
	if !laUUID(id) { // "abc" làm PostgreSQL ném lỗi thô về kiểu uuid ra tận màn hình
		return nil, gorm.ErrRecordNotFound
	}
	var g model.Game
	if err := s.db.First(&g, "id = ?", id).Error; err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
	// Ở quán, tên hiển thị, nhóm và launcher đến từ catalog của master: sửa tay
	// sẽ bị lần đồng bộ sau ghi đè, nên chỉ cho sửa ở master.
	if s.cfg.Role == config.GameRoleMaster {
		if req.DisplayName != nil && strings.TrimSpace(*req.DisplayName) != "" {
			updates["display_name"] = strings.TrimSpace(*req.DisplayName)
		}
		if req.Category != nil {
			updates["category"] = strings.TrimSpace(*req.Category)
		}
		if req.Launcher != nil {
			if err := checkLauncher(*req.Launcher); err != nil {
				return nil, err
			}
			updates["launcher"] = strings.TrimSpace(*req.Launcher)
		}
		if req.LaunchArgs != nil {
			updates["launch_args"] = strings.TrimSpace(*req.LaunchArgs)
		}
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.AutoUpdate != nil {
		updates["auto_update"] = *req.AutoUpdate
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}
	if len(updates) > 0 {
		if err := s.db.Model(&g).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	if err := s.db.First(&g, "id = ?", id).Error; err != nil {
		return nil, err
	}
	s.Kick()
	return &g, nil
}

// Delete bỏ một game khỏi danh mục. Tệp trên ổ đĩa giữ nguyên.
func (s *GameService) Delete(id string) error {
	if s.cfg.Role != config.GameRoleMaster {
		return ErrGameMasterOnly
	}
	if !laUUID(id) {
		return gorm.ErrRecordNotFound
	}
	var g model.Game
	if err := s.db.First(&g, "id = ?", id).Error; err != nil {
		return err
	}
	if e := s.getEngine(); e != nil {
		e.Remove(g.Name)
	}
	s.mu.Lock()
	delete(s.stabilizers, g.Name)
	s.mu.Unlock()
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("game_id = ?", g.ID).Delete(&model.GamePublish{}).Error; err != nil {
			return err
		}
		return tx.Delete(&g).Error
	})
}

// ---------------------------------------------------------------- menu game (máy trạm)

// GameMenuItem là một game trên menu của máy trạm. Máy trạm chỉ chạy được game
// khi Status == ready; đường dẫn đầy đủ là Root + `\` + Name + `\` + Launcher.
type GameMenuItem struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Category    string `json:"category"`
	Launcher    string `json:"launcher"`
	LaunchArgs  string `json:"launch_args"`
	Status      string `json:"status"`
	Progress    int    `json:"progress"`
	Version     int    `json:"version"`
}

// GameMenu: Root là GAME_ROOT như máy trạm thấy (xem GameClientRootKey).
type GameMenu struct {
	Root  string         `json:"root"`
	Items []GameMenuItem `json:"items"`
}

// Menu trả các game đang bật và có tệp chạy, mọi GAME_ROLE đều dùng được. Game
// đang tải vẫn hiện (kèm tiến độ) để khách biết, nhưng chưa chạy được.
func (s *GameService) Menu() (*GameMenu, error) {
	games, err := s.List()
	if err != nil {
		return nil, err
	}
	menu := &GameMenu{
		Root:  normalizeGameClientRoot(settingsGroup(s.db, GamesGroup)[GameClientRootKey]),
		Items: make([]GameMenuItem, 0, len(games)),
	}
	for _, g := range games {
		if !g.Enabled || g.Launcher == "" {
			continue
		}
		menu.Items = append(menu.Items, GameMenuItem{
			Name: g.Name, DisplayName: g.DisplayName, Category: g.Category,
			Launcher: g.Launcher, LaunchArgs: g.LaunchArgs,
			Status: g.Status, Progress: g.Progress, Version: g.Version,
		})
	}
	return menu, nil
}

func checkLauncher(l string) error {
	l = strings.TrimSpace(l)
	if l == "" {
		return nil
	}
	// Bắt đầu bằng "\" hay "/" là UNC hoặc gốc ổ đĩa, không phải đường dẫn trong thư mục game.
	if filepath.IsAbs(l) || strings.HasPrefix(l, `\`) || strings.HasPrefix(l, "/") ||
		strings.Contains(l, "..") || strings.Contains(l, ":") {
		return errors.New("launcher là đường dẫn tương đối trong thư mục game, ví dụ Bin/Game.exe")
	}
	for _, c := range strings.FieldsFunc(l, func(r rune) bool { return r == '/' || r == '\\' }) {
		if c != "." && thanhPhanWindowsXau(c) {
			return fmt.Errorf("launcher: thành phần %q kết thúc bằng dấu chấm hoặc khoảng trắng, hoặc trùng tên thiết bị Windows (CON, NUL, COM1…)", c)
		}
	}
	return nil
}

// ---------------------------------------------------------------- catalog (master)

// CatalogItem là một game master công bố cho các quán.
type CatalogItem struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Category    string `json:"category"`
	Launcher    string `json:"launcher"`
	LaunchArgs  string `json:"launch_args"`
	Priority    int    `json:"priority"`
	Version     int    `json:"version"`
	Infohash    string `json:"infohash"`
	SizeBytes   int64  `json:"size_bytes"`
}

// SeedToken là phần bí mật trong URL webseed: /seed/<token>/<game>/<tệp>.
// Engine BitTorrent không gửi được header khi gọi webseed, nên khoá phải nằm
// trong đường dẫn. Dùng băm của khoá catalog chứ không dùng chính nó, để log
// truy cập của master không làm lộ khoá đọc catalog.
func (s *GameService) SeedToken() string {
	if s.cfg.CatalogKey == "" {
		return ""
	}
	sum := sha256.Sum256([]byte("vnet-seed:" + s.cfg.CatalogKey))
	return hex.EncodeToString(sum[:16])
}

// CheckSeedToken so token webseed bằng thời gian cố định.
func (s *GameService) CheckSeedToken(tok string) bool {
	want := s.SeedToken()
	return want != "" && subtle.ConstantTimeCompare([]byte(tok), []byte(want)) == 1
}

// SeedServable: chỉ phát tệp của game đang bật và đã publish. Thư mục khác
// trong GAME_ROOT (game chưa publish, trạng thái engine) không ra ngoài.
func (s *GameService) SeedServable(name string) bool {
	if !tenGameDungDuoc(name) {
		return false
	}
	var n int64
	s.db.Model(&model.Game{}).Where("name = ? AND enabled = ? AND version > 0", name, true).Count(&n)
	return n > 0
}

// CheckCatalogKey so khoá bằng thời gian cố định.
func (s *GameService) CheckCatalogKey(key string) bool {
	return s.cfg.CatalogKey != "" && subtle.ConstantTimeCompare([]byte(key), []byte(s.cfg.CatalogKey)) == 1
}

func (s *GameService) Catalog() ([]CatalogItem, error) {
	if s.cfg.Role != config.GameRoleMaster {
		return nil, ErrGameMasterOnly
	}
	var games []model.Game
	if err := s.db.Where("enabled = ? AND version > 0", true).Order("priority DESC, name ASC").Find(&games).Error; err != nil {
		return nil, err
	}
	out := make([]CatalogItem, 0, len(games))
	for _, g := range games {
		out = append(out, CatalogItem{
			Name: g.Name, DisplayName: g.DisplayName, Category: g.Category,
			Launcher: g.Launcher, LaunchArgs: g.LaunchArgs, Priority: g.Priority,
			Version: g.Version, Infohash: g.Infohash, SizeBytes: g.SizeBytes,
		})
	}
	return out, nil
}

// Torrent trả tệp .torrent của bản mới nhất.
func (s *GameService) Torrent(name string) ([]byte, error) {
	if s.cfg.Role != config.GameRoleMaster {
		return nil, ErrGameMasterOnly
	}
	var g model.Game
	if err := s.db.Where("name = ? AND enabled = ?", name, true).First(&g).Error; err != nil {
		return nil, err
	}
	var p model.GamePublish
	if err := s.db.Where("game_id = ? AND version = ?", g.ID, g.Version).First(&p).Error; err != nil {
		return nil, err
	}
	return p.Torrent, nil
}

// ---------------------------------------------------------------- vòng chạy nền

// Kick yêu cầu vòng nền chạy ngay một lượt thay vì đợi tới kỳ sau.
func (s *GameService) Kick() {
	select {
	case s.kick <- struct{}{}:
	default:
	}
}

// Run chạy cho tới khi ctx kết thúc. Không làm gì khi GAME_ROLE trống.
func (s *GameService) Run(ctx context.Context) {
	if s.cfg.Role == config.GameRoleOff {
		return
	}
	dataDir := s.cfg.DataDir
	if dataDir == "" || !filepath.IsAbs(dataDir) {
		dataDir = filepath.Join(s.cfg.Root, ".vnet-gameupdate")
	}
	engine, err := gameupdate.NewEngine(gameupdate.EngineConfig{
		DataDir:    dataDir,
		ListenPort: s.cfg.ListenPort,
	})
	if err != nil {
		log.Printf("[game] không khởi động được engine: %v", err)
		return
	}
	s.mu.Lock()
	s.engine = engine
	s.runCtx = ctx
	s.mu.Unlock()
	var worker sync.WaitGroup
	defer func() {
		worker.Wait() // luồng tải (quán) dừng hẳn rồi mới đóng engine
		s.mu.Lock()
		s.engine = nil
		s.runCtx = nil
		s.mu.Unlock()
		engine.Close()
	}()
	log.Printf("[game] bật vai trò %s, thư mục %s", s.cfg.Role, s.cfg.Root)

	s.recoverDownloading()
	s.recoverPublishing()

	if s.cfg.Role == config.GameRoleMaster {
		s.seedPublished(ctx)
	} else {
		worker.Add(1)
		go func() {
			defer worker.Done()
			s.installLoop(ctx)
		}()
	}
	for {
		if s.cfg.Role == config.GameRoleMaster {
			s.publishTick(ctx, time.Now())
		} else {
			s.syncTick(ctx)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(s.cfg.PollInterval):
		case <-s.kick:
		}
	}
}

// recoverDownloading xử lý các game mà lần tải bị cắt giữa chừng (mất điện, khởi động lại).
//
// Không đưa về "queued": trạng thái đó nghĩa là người quản trị vừa bấm "tải ngay" và được bỏ qua
// khung giờ cập nhật lẫn tự cập nhật, nên một lần khởi động lại sẽ ghi vào ổ game ngoài khung giờ.
// Để đường tự cập nhật (có khung giờ) quyết định. Thư mục có thể đã bị ghi đè dở dang nên chưa
// được coi là sẵn sàng; riêng game đã có đúng bản của master (chỉ là lần kiểm tra lại bị ngắt)
// thì dữ liệu vẫn trọn vẹn và giữ "ready".
func (s *GameService) recoverDownloading() {
	res := s.db.Model(&model.Game{}).
		Where("status = ? AND infohash <> '' AND infohash = remote_infohash", model.GameStatusDownloading).
		Update("status", model.GameStatusReady)
	n := res.RowsAffected
	res = s.db.Model(&model.Game{}).Where("status = ?", model.GameStatusDownloading).
		Updates(map[string]interface{}{"status": model.GameStatusNotInstalled, "progress": 0})
	if n += res.RowsAffected; n > 0 {
		log.Printf("[game] %d game đang tải dở khi máy chủ dừng; sẽ tải lại theo khung giờ cập nhật", n)
	}
}

// recoverPublishing xử lý game kẹt ở "publishing" vì master dừng giữa lúc băm (PublishNow). Trạng
// thái này không còn ai gỡ, và PublishNow từ chối game đang publish. Giao dịch publish là nguyên tử —
// chưa commit thì version, vân tay và .torrent vẫn là của bản trước — nên chỉ cần trả game về đúng
// trạng thái của bản đó. Thư mục đã đổi thì publishTick tự publish lại khi nó đứng yên đủ lâu, nên
// không cần so vân tay ở đây. Không bao giờ để "error" mà không có lời giải thích.
func (s *GameService) recoverPublishing() {
	var games []model.Game
	if err := s.db.Where("status = ?", model.GameStatusPublishing).Find(&games).Error; err != nil {
		return
	}
	for _, g := range games {
		if g.Version == 0 {
			s.db.Model(&model.Game{}).Where("id = ?", g.ID).
				Updates(map[string]interface{}{"status": model.GameStatusNotInstalled, "error": ""})
			continue
		}
		if _, err := s.loadPublished(g); err != nil {
			s.setError(g.ID, fmt.Sprintf("thiếu hoặc hỏng tệp .torrent của bản %d (%v): bấm Publish để tạo lại", g.Version, err))
			continue
		}
		s.db.Model(&model.Game{}).Where("id = ?", g.ID).
			Updates(map[string]interface{}{"status": model.GameStatusReady, "error": ""})
		log.Printf("[game] %s: lần publish dở bị bỏ, giữ bản %d đã publish", g.Name, g.Version)
	}
}

// seedPublished phát lại các bản đã publish sau khi master khởi động.
func (s *GameService) seedPublished(ctx context.Context) {
	e := s.getEngine()
	if e == nil {
		return
	}
	var games []model.Game
	s.db.Where("enabled = ? AND version > 0", true).Find(&games)
	for _, g := range games {
		mi, err := s.loadPublished(g)
		if err != nil {
			continue
		}
		job := gameupdate.Job{Name: g.Name, MetaInfo: mi, Dest: filepath.Join(s.cfg.Root, g.Name)}
		if err := e.Seed(ctx, job); err != nil {
			// Thư mục đã đổi kể từ lần publish: lượt publish tới sẽ làm bản mới.
			log.Printf("[game] %s: chưa phát được bản %d: %v", g.Name, g.Version, err)
		}
	}
}

func (s *GameService) loadPublished(g model.Game) (*metainfo.MetaInfo, error) {
	var p model.GamePublish
	if err := s.db.Where("game_id = ? AND version = ?", g.ID, g.Version).First(&p).Error; err != nil {
		return nil, err
	}
	mi, _, err := gameupdate.LoadTorrent(p.Torrent)
	return mi, err
}

// publishTick: với mỗi game, nếu thư mục đã đổi và đứng yên đủ lâu thì publish bản mới.
func (s *GameService) publishTick(ctx context.Context, now time.Time) {
	var games []model.Game
	if err := s.db.Where("enabled = ?", true).Find(&games).Error; err != nil {
		return
	}
	for _, g := range games {
		if ctx.Err() != nil {
			return
		}
		dir := filepath.Join(s.cfg.Root, g.Name)
		fp, err := gameupdate.Fingerprint(dir)
		if err != nil {
			s.setError(g.ID, fmt.Sprintf("không đọc được thư mục: %v", err))
			continue
		}
		// Thư mục đã đọc được và trùng bản đã publish: lỗi trước đó (không đọc được thư mục, hoặc một
		// lần publish hỏng rồi thư mục trở lại như cũ) chỉ là tạm thời. Không gỡ thì /api/game-menu
		// báo mãi một game đang tốt là chưa sẵn sàng, vì không còn gì đưa nó về "ready".
		if g.Status == model.GameStatusError && g.Version > 0 && fp == g.Fingerprint {
			s.db.Model(&model.Game{}).Where("id = ? AND status = ?", g.ID, model.GameStatusError).
				Updates(map[string]interface{}{"status": model.GameStatusReady, "error": ""})
		}
		s.mu.Lock()
		st := s.stabilizers[g.Name]
		if st == nil {
			st = gameupdate.NewStabilizer(s.cfg.StableFor, g.Fingerprint)
			s.stabilizers[g.Name] = st
		}
		settled := st.Observe(fp, now)
		s.mu.Unlock()
		if !settled {
			continue
		}
		if err := s.publish(ctx, g, dir, fp); err != nil {
			log.Printf("[game] publish %s: %v", g.Name, err)
			s.setError(g.ID, err.Error())
			continue
		}
		s.mu.Lock()
		st.MarkPublished(fp)
		s.mu.Unlock()
	}
}

// PublishNow publish ngay một game, bỏ qua thời gian chờ đứng yên. Dùng khi
// người quản trị biết chắc game đã cập nhật xong.
//
// Băm chạy NỀN: một game 40 GB mất nhiều phút, vượt xa hạn ghi 30 giây của
// HTTP server. Trạng thái chuyển "publishing" ngay, trang quản trị tự làm mới
// cho tới khi thành "ready" (hoặc "error").
func (s *GameService) PublishNow(_ context.Context, id string) (*model.Game, error) {
	if s.cfg.Role != config.GameRoleMaster {
		return nil, ErrGameMasterOnly
	}
	if !laUUID(id) {
		return nil, gorm.ErrRecordNotFound
	}
	ctx := s.getRunCtx()
	if ctx == nil || s.getEngine() == nil {
		return nil, ErrGameRoleOff
	}
	var g model.Game
	if err := s.db.First(&g, "id = ?", id).Error; err != nil {
		return nil, err
	}
	if g.Status == model.GameStatusPublishing {
		return &g, nil
	}
	dir := filepath.Join(s.cfg.Root, g.Name)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("không thấy thư mục %s", dir)
	}
	s.db.Model(&model.Game{}).Where("id = ?", g.ID).Updates(map[string]interface{}{
		"status": model.GameStatusPublishing, "error": "",
	})
	go func() {
		fp, err := gameupdate.Fingerprint(dir)
		if err == nil && (fp != g.Fingerprint || g.Version == 0) {
			err = s.publish(ctx, g, dir, fp)
		}
		if err != nil {
			log.Printf("[game] publish %s: %v", g.Name, err)
			s.setError(g.ID, err.Error())
			return
		}
		// Thư mục không đổi kể từ lần publish trước: chỉ trả lại trạng thái.
		s.db.Model(&model.Game{}).Where("id = ? AND status = ?", g.ID, model.GameStatusPublishing).
			Update("status", model.GameStatusReady)
		s.mu.Lock()
		if st := s.stabilizers[g.Name]; st != nil {
			st.MarkPublished(fp)
		}
		s.mu.Unlock()
	}()
	if err := s.db.First(&g, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &g, nil
}

func (s *GameService) publish(ctx context.Context, g model.Game, dir, fp string) error {
	s.pubMu.Lock()
	defer s.pubMu.Unlock()
	// Đọc lại sau khi giữ khoá: lượt kia có thể vừa publish xong chính bản này.
	if err := s.db.First(&g, "id = ?", g.ID).Error; err != nil {
		return err
	}
	if g.Fingerprint == fp && g.Version > 0 {
		return nil
	}
	log.Printf("[game] băm %s để publish bản %d…", g.Name, g.Version+1)
	mi, err := gameupdate.BuildTorrent(dir, g.Name)
	if err != nil {
		return err
	}
	info, err := mi.UnmarshalInfo()
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := mi.Write(&buf); err != nil {
		return err
	}
	raw := buf.Bytes()
	hash := mi.HashInfoBytes().HexString()
	now := time.Now()
	version := g.Version + 1
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model.GamePublish{
			GameID: g.ID, Version: version, Infohash: hash, Torrent: raw,
			FileCount: len(info.Files), SizeBytes: info.TotalLength(),
		}).Error; err != nil {
			return err
		}
		return tx.Model(&model.Game{}).Where("id = ?", g.ID).Updates(map[string]interface{}{
			"version": version, "infohash": hash, "size_bytes": info.TotalLength(),
			"fingerprint": fp, "status": model.GameStatusReady, "progress": 100,
			"error": "", "last_updated_at": now,
		}).Error
	})
	if err != nil {
		return err
	}
	log.Printf("[game] đã publish %s bản %d (%d tệp, %d byte)", g.Name, version, len(info.Files), info.TotalLength())
	// Băm 40 GB có thể kéo dài hơn cả vòng nền: lúc xong, Run có thể đã dừng và engine đã đóng.
	e := s.getEngine()
	if e == nil {
		return nil
	}
	if err := e.Seed(ctx, gameupdate.Job{Name: g.Name, MetaInfo: mi, Dest: dir}); err != nil {
		log.Printf("[game] %s: publish xong nhưng chưa phát được: %v", g.Name, err)
	}
	return nil
}

func (s *GameService) setError(id, msg string) {
	if strings.TrimSpace(msg) == "" {
		msg = "lỗi không rõ nguyên nhân, xem nhật ký máy chủ"
	}
	s.db.Model(&model.Game{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status": model.GameStatusError, "error": truncateRunes(msg, 2000),
	})
}

// ---------------------------------------------------------------- đồng bộ (quán)

// SyncNow tải ngay một game ở quán, bỏ qua tự cập nhật, khung giờ và thời gian chờ thử lại.
func (s *GameService) SyncNow(id string) error {
	if s.cfg.Role != config.GameRoleCafe {
		return ErrGameCafeOnly
	}
	if !laUUID(id) {
		return gorm.ErrRecordNotFound
	}
	var g model.Game
	if err := s.db.First(&g, "id = ?", id).Error; err != nil {
		return err
	}
	if err := s.db.Model(&model.Game{}).Where("id = ? AND status <> ?", g.ID, model.GameStatusDownloading).
		Update("status", model.GameStatusQueued).Error; err != nil {
		return err
	}
	s.Kick()
	return nil
}

// Tải game ở quán gồm hai việc chạy tách rời:
//
//   - vòng nền (Run -> syncTick) đọc catalog của master MỖI lượt, không bao giờ chờ việc tải;
//   - MỘT luồng tải (installLoop) lần lượt tải từng game, mỗi game có đồng hồ canh tiến độ.
//
// Trước đây syncTick tải ngay trên vòng nền nên một lượt tải treo (webseed không trả lời, hoặc master
// đổi tệp sau khi publish nên piece nào cũng sai hash) chặn luôn cả việc đọc catalog: quán không
// bao giờ thấy bản mới hay game khác cho tới khi khởi động lại.
//
// Chỉ một luồng, không phải mỗi game một luồng: đường truyền WAN và ổ đĩa là chỗ nghẽn chung, hai
// game hàng chục GB tải song song chỉ chậm đi một nửa mỗi bên (và băm lại gấp đôi), không nhanh hơn.
// Một game lỗi hay treo không chặn các game sau vì đồng hồ canh tiến độ (GAME_STALL_TIMEOUT) bỏ nó,
// và nó chỉ được thử lại sau khoảng chờ tăng dần (retryDelay).

// syncTick đọc catalog rồi đánh thức luồng tải.
func (s *GameService) syncTick(ctx context.Context) {
	if err := s.pullCatalog(ctx); err != nil {
		log.Printf("[game] đọc catalog của master: %v", err)
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// installLoop chạy trong luồng tải riêng cho tới khi ctx kết thúc.
func (s *GameService) installLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
		}
		for ctx.Err() == nil {
			g := s.nextInstall(time.Now())
			if g == nil {
				break
			}
			s.installOne(ctx, *g)
		}
	}
}

// nextInstall chọn game cần tải tiếp theo (ưu tiên cao trước), hoặc nil nếu không còn.
//
// "queued" là người quản trị bấm "tải ngay" (hoặc lần tải đó bị master đổi bản giữa chừng): chạy luôn,
// bỏ qua tự cập nhật, khung giờ và thời gian chờ thử lại. Game chờ bản mới của master phải bật tự cập
// nhật, nằm trong khung giờ và không đang chờ thử lại. Game đã đúng bản master thì không có gì để cập
// nhật; nó chỉ quay lại đây khi lần "tải ngay" kiểm tra lại của nó bị lỗi (status "error"), và được thử
// lại sau thời gian chờ như failInstall đã hứa — không cần tự cập nhật hay khung giờ vì chẳng có gì mới để
// tải, và vì chính người quản trị đã yêu cầu nó. now được truyền vào để kiểm thử đặt giờ tuỳ ý.
//
// Việc lọc nằm ở đây chứ không ở câu SQL, để các test dựng được từng trường hợp mà không cần Postgres.
func (s *GameService) nextInstall(now time.Time) *model.Game {
	var games []model.Game
	s.db.Where("enabled = ? AND remote_infohash <> ''", true).Find(&games)
	sort.SliceStable(games, func(i, j int) bool { return games[i].Priority > games[j].Priority })

	inWindow := withinWindow(settingsGroup(s.db, GamesGroup)[GameUpdateWindowKey], now)
	for i := range games {
		g := &games[i]
		switch {
		case g.Status == model.GameStatusQueued:
			return g
		case g.Infohash == g.RemoteInfohash:
			if g.Status == model.GameStatusError && !s.inBackoff(g, now) {
				return g
			}
		case g.AutoUpdate && inWindow && !s.inBackoff(g, now):
			return g
		}
	}
	return nil
}

func (s *GameService) inBackoff(g *model.Game, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.retry[g.Name]
	return r != nil && r.infohash == g.RemoteInfohash && now.Before(r.notBefore)
}

// retryDelay: 5 phút sau lần lỗi đầu, gấp đôi mỗi lần, tối đa 1 giờ.
func retryDelay(fails int) time.Duration {
	d := gameRetryBase
	for i := 1; i < fails && d < gameRetryMax; i++ {
		d *= 2
	}
	return min(d, gameRetryMax)
}

// installOne tải một game và xử lý kết quả. Lỗi chỉ ghi vào game đó rồi trả về để luồng tải sang game khác.
func (s *GameService) installOne(ctx context.Context, g model.Game) {
	manual := g.Status == model.GameStatusQueued
	jobCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	s.mu.Lock()
	s.current = &installJob{name: g.Name, infohash: g.RemoteInfohash, cancel: cancel}
	s.mu.Unlock()

	err := s.install(jobCtx, g)

	s.mu.Lock()
	s.current = nil
	if err == nil {
		delete(s.retry, g.Name)
	}
	s.mu.Unlock()
	switch {
	case err == nil, ctx.Err() != nil:
		// Xong, hoặc máy chủ đang tắt: lần khởi động sau sẽ dọn trạng thái dở (recoverDownloading).
	case errors.Is(context.Cause(jobCtx), errSuperseded):
		log.Printf("[game] %s: master vừa publish bản mới, bỏ lượt tải bản cũ", g.Name)
		// Lượt tải được người quản trị bấm thì bản mới vẫn là yêu cầu của họ; còn lại để đường tự cập nhật quyết định.
		status := model.GameStatusNotInstalled
		if manual {
			status = model.GameStatusQueued
		}
		s.db.Model(&model.Game{}).Where("id = ?", g.ID).
			Updates(map[string]interface{}{"status": status, "progress": 0})
	default:
		log.Printf("[game] tải %s: %v", g.Name, err)
		s.failInstall(g, err, time.Now())
	}
}

// failInstall ghi lỗi vào game và đặt thời gian chờ trước lần thử lại.
func (s *GameService) failInstall(g model.Game, err error, now time.Time) {
	s.mu.Lock()
	r := s.retry[g.Name]
	if r == nil || r.infohash != g.RemoteInfohash { // bản mới của master: tính lại từ đầu
		r = &retryInfo{infohash: g.RemoteInfohash}
		s.retry[g.Name] = r
	}
	r.fails++
	delay := retryDelay(r.fails)
	r.notBefore = now.Add(delay)
	s.mu.Unlock()

	msg := err.Error()
	if errors.Is(err, gameupdate.ErrStalled) {
		msg = fmt.Sprintf("Tải bị treo: %s không có thêm dữ liệu nào đúng hash (master không phản hồi, hoặc master đã đổi tệp sau khi publish)", thoiLuong(s.cfg.StallTimeout))
	}
	// Chỉ hứa tự thử lại khi nextInstall sẽ thật sự nhặt game này lên: game đã đúng bản master thì luôn,
	// game chờ bản mới thì phải bật tự cập nhật (và còn phải chờ khung giờ, nên là "sớm nhất").
	hint := fmt.Sprintf("Tự thử lại sớm nhất sau %s.", thoiLuong(delay))
	if g.Infohash != g.RemoteInfohash && !g.AutoUpdate {
		hint = "Bấm Tải ngay để thử lại."
	}
	s.setError(g.ID, fmt.Sprintf("%s. %s", msg, hint))
}

// thoiLuong: "10 phút" thay vì "10m0s" trong thông báo cho người quản trị.
func thoiLuong(d time.Duration) string {
	if d >= time.Minute && d%time.Minute == 0 {
		return fmt.Sprintf("%d phút", d/time.Minute)
	}
	return d.Round(time.Second).String()
}

// supersede hủy lượt tải đang chạy của game name nếu nó đang tải một bản khác infohash mới nhất.
func (s *GameService) supersede(name, infohash string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c := s.current; c != nil && c.name == name && c.infohash != infohash {
		c.cancel(errSuperseded)
	}
}

// pullCatalog cập nhật danh sách game ở quán theo catalog của master.
func (s *GameService) pullCatalog(ctx context.Context) error {
	var items []CatalogItem
	if err := s.upstreamJSON(ctx, "/api/game-catalog", &items); err != nil {
		return err
	}
	for _, it := range items {
		if !tenGameDungDuoc(it.Name) {
			log.Printf("[game] bỏ qua game có tên không hợp lệ từ master: %q", it.Name)
			continue
		}
		var g model.Game
		err := s.db.Where("name = ?", it.Name).First(&g).Error
		fields := map[string]interface{}{
			"display_name": it.DisplayName, "category": it.Category,
			"launcher": it.Launcher, "launch_args": it.LaunchArgs,
			"remote_version": it.Version, "remote_infohash": it.Infohash,
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			g = model.Game{
				Name: it.Name, DisplayName: it.DisplayName, Category: it.Category,
				Launcher: it.Launcher, LaunchArgs: it.LaunchArgs, Priority: it.Priority,
				Enabled: true, AutoUpdate: true, Status: model.GameStatusNotInstalled,
				RemoteVersion: it.Version, RemoteInfohash: it.Infohash, SizeBytes: it.SizeBytes,
			}
			if err := s.db.Create(&g).Error; err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if err := s.db.Model(&g).Updates(fields).Error; err != nil {
			return err
		}
		s.supersede(it.Name, it.Infohash)
	}
	return nil
}

func (s *GameService) install(ctx context.Context, g model.Game) error {
	raw, err := s.upstreamBytes(ctx, "/api/game-catalog/"+url.PathEscape(g.Name)+"/torrent")
	if err != nil {
		return fmt.Errorf("tải tệp torrent: %w", err)
	}
	mi, info, err := gameupdate.LoadTorrent(raw)
	if err != nil {
		return fmt.Errorf("tệp torrent hỏng: %w", err)
	}
	hash := mi.HashInfoBytes().HexString()
	if hash != g.RemoteInfohash {
		// Master vừa publish bản mới giữa hai lần gọi: lượt sau sẽ lấy đúng.
		return fmt.Errorf("torrent (%s) không khớp catalog (%s)", hash, g.RemoteInfohash)
	}
	if info.Name != g.Name {
		return fmt.Errorf("torrent mang tên %q, không phải %q", info.Name, g.Name)
	}

	s.db.Model(&model.Game{}).Where("id = ?", g.ID).Updates(map[string]interface{}{
		"status": model.GameStatusDownloading, "progress": 0, "error": "",
		"size_bytes": info.TotalLength(),
	})
	log.Printf("[game] bắt đầu tải %s bản %d", g.Name, g.RemoteVersion)

	e := s.getEngine()
	if e == nil {
		return ErrGameRoleOff
	}
	job := gameupdate.Job{
		Name:         g.Name,
		MetaInfo:     mi,
		Dest:         filepath.Join(s.cfg.Root, g.Name),
		Webseeds:     []string{s.cfg.UpstreamURL + "/seed/" + s.SeedToken() + "/"},
		StallTimeout: s.cfg.StallTimeout,
	}
	if err := os.MkdirAll(job.Dest, 0o755); err != nil {
		return err
	}
	if err := e.Install(ctx, job); err != nil {
		return err
	}
	// Đóng torrent ngay khi xong: engine giữ tệp mở để ghi, và trên Windows thì
	// launcher, phần mềm diệt virus hay chính quản trị viên mở tệp game để đọc
	// đều bị "đang được dùng". Quán chưa có tracker nên seed tiếp cũng không ai nhận.
	e.Remove(g.Name)
	now := time.Now()
	log.Printf("[game] %s bản %d đã sẵn sàng", g.Name, g.RemoteVersion)
	return s.db.Model(&model.Game{}).Where("id = ?", g.ID).Updates(map[string]interface{}{
		"status": model.GameStatusReady, "progress": 100, "error": "",
		"version": g.RemoteVersion, "infohash": hash, "last_updated_at": now,
	}).Error
}

func (s *GameService) upstreamBytes(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.UpstreamURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(GameCatalogKeyHeader, s.cfg.CatalogKey)
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// .torrent của game lớn khoảng vài trăm KB; 32 MB là trần an toàn.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s trả %d: %s", path, resp.StatusCode, truncateRunes(string(body), 200))
	}
	return body, nil
}

func (s *GameService) upstreamJSON(ctx context.Context, path string, out interface{}) error {
	body, err := s.upstreamBytes(ctx, path)
	if err != nil {
		return err
	}
	var env struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return err
	}
	if env.Code != 0 {
		return errors.New(env.Message)
	}
	return json.Unmarshal(env.Data, out)
}

// GameCatalogKeyHeader mang khoá catalog giữa quán và master.
const GameCatalogKeyHeader = "X-Game-Key"

// withinWindow: "02:00-08:00" (có thể vắt qua nửa đêm, "22:00-06:00"). Rỗng
// hoặc sai cú pháp là không giới hạn — một khung giờ gõ sai không được làm quán
// đứng im không bao giờ cập nhật.
func withinWindow(window string, now time.Time) bool {
	window = strings.TrimSpace(window)
	if window == "" {
		return true
	}
	parts := strings.Split(window, "-")
	if len(parts) != 2 {
		return true
	}
	from, ok1 := parseHHMM(parts[0])
	to, ok2 := parseHHMM(parts[1])
	if !ok1 || !ok2 || from == to {
		return true
	}
	m := now.Hour()*60 + now.Minute()
	if from < to {
		return m >= from && m < to
	}
	return m >= from || m < to
}

func parseHHMM(s string) (int, bool) {
	hm := strings.Split(strings.TrimSpace(s), ":")
	if len(hm) != 2 {
		return 0, false
	}
	h, err1 := strconv.Atoi(hm[0])
	m, err2 := strconv.Atoi(hm[1])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
