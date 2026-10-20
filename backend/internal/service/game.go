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
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/gorilla/websocket"
	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/gameupdate"
	"github.com/vnet/core/internal/hub"
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

// coKyTuLoi: chuỗi có U+FFFD (hoặc byte không phải UTF-8, range cũng ra U+FFFD). Đó là dấu vết
// của một lần đổi bảng mã làm mất chữ, ví dụ "Liên Minh" thành "Li�n Minh".
func coKyTuLoi(s string) bool {
	return !utf8.ValidString(s) || strings.ContainsRune(s, utf8.RuneError)
}

// kiemTraChuGame từ chối chuỗi người dùng nhập mà đã hỏng bảng mã hoặc có ký tự điều khiển. Tiếng
// Việt có dấu, emoji và các ký tự Unicode thường vẫn qua. nhan là tên trường để lỗi chỉ đúng chỗ.
// Kiểm sau khi cắt khoảng trắng — đúng giá trị sẽ được lưu — nên dấu xuống dòng thừa khi dán không bị từ chối.
func kiemTraChuGame(nhan, v string) error {
	v = strings.TrimSpace(v)
	if coKyTuLoi(v) {
		return fmt.Errorf("%s có ký tự lỗi (mất dấu tiếng Việt do sai bảng mã), vui lòng nhập lại", nhan)
	}
	if strings.IndexFunc(v, unicode.IsControl) >= 0 {
		return fmt.Errorf("%s có ký tự điều khiển không hợp lệ", nhan)
	}
	return nil
}

// kiemTraCacTruongChuGame kiểm các trường chữ của một game; con trỏ nil là trường không gửi lên (Update).
// Tên game (name) không cần ở đây: tenGameHopLe chỉ nhận ASCII nên U+FFFD không lọt qua được.
func kiemTraCacTruongChuGame(displayName, category, launcher, launchArgs *string) error {
	for _, f := range []struct {
		nhan string
		v    *string
	}{
		{"tên hiển thị", displayName}, {"nhóm", category}, {"launcher", launcher}, {"tham số chạy", launchArgs},
	} {
		if f.v == nil {
			continue
		}
		if err := kiemTraChuGame(f.nhan, *f.v); err != nil {
			return err
		}
	}
	return nil
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
	hub    *hub.Hub

	cfgMu      sync.RWMutex
	live       *config.GameConfig // cấu hình có hiệu lực khi quán nối Hub từ trang Game; nil là dùng cfg (môi trường)
	supervised bool               // Run đang chạy, nên có người nhận yêu cầu đổi cấu hình
	reconf     chan chan struct{} // yêu cầu áp lại cấu hình, kèm kênh để báo xong
	syncAt     time.Time          // lần đọc catalog gần nhất (quán); giữ mu
	syncErr    string             // lỗi của lần đó, rỗng là tốt; giữ mu
	keyInfo    *hubKeyInfo        // tình trạng key do Hub báo qua report; giữ mu
	eventsUp   bool               // đang giữ kênh WebSocket tới Hub; giữ mu
	noReport   bool               // upstream không có /report (master cũ): thôi gọi; giữ mu
	transfers  []reportTransfer   // các lần cài xong chưa báo Hub, mỗi mục gửi đúng một lần; giữ mu
	noEvents   bool               // upstream không có kênh WebSocket (master cũ): thôi nối; giữ mu
	eventsDial *websocket.Dialer  // test thay được; nil là dùng bộ nối có rào chặn địa chỉ nội bộ
	localClash map[string]bool    // tên game cục bộ trùng một game của catalog; giữ mu
	stats      *menuStats         // xếp hạng lượt mở đã đếm (nhãn Hot, thứ tự menu); giữ mu

	mu          sync.Mutex
	runCtx      context.Context // context của vòng nền; nil khi chưa chạy
	pubMu       sync.Mutex      // một lần publish một lúc: số phiên bản không được trùng
	stabilizers map[string]*gameupdate.Stabilizer
	kick        chan struct{}
	wake        chan struct{}         // vòng đọc catalog đánh thức luồng tải game (quán)
	current     *installJob           // game đang tải; nil khi luồng tải rảnh
	retry       map[string]*retryInfo // game tải lỗi: bao giờ được thử lại
	riotLogged  map[string]bool       // game Riot đã báo "Riot chạy từ thư mục khác" trong nhật ký; giữ pubMu
	riotBroken  map[string]bool       // thư mục game Riot có lần chụp gần nhất lỗi, bản chụp có thể dở; giữ pubMu
	speeds      map[string]*rateMeter // tốc độ tải đo giữa hai lần hỏi hàng đợi; giữ mu
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
		http:        newHubHTTPClient(60 * time.Second),
		stabilizers: map[string]*gameupdate.Stabilizer{},
		kick:        make(chan struct{}, 1),
		wake:        make(chan struct{}, 1),
		reconf:      make(chan chan struct{}, 4),
		retry:       map[string]*retryInfo{},
		riotLogged:  map[string]bool{},
		riotBroken:  map[string]bool{},
		speeds:      map[string]*rateMeter{},
	}
}

// WithHub nối hub WebSocket vào để vòng nền báo trang Game "games:changed" mỗi
// khi trạng thái một game đổi (publish xong, tải xong, lỗi, catalog có bản mới).
// Những việc đó không đi qua yêu cầu HTTP nên middleware không báo. Không nối
// thì vẫn chạy đúng, trang chỉ thấy khi tự tải lại.
func (s *GameService) WithHub(h *hub.Hub) *GameService {
	s.hub = h
	return s
}

// baoDoi báo trang quản trị rằng danh sách game vừa đổi. Hub nil thì bỏ qua.
func (s *GameService) baoDoi(id string) {
	s.hub.Changed("games", map[string]interface{}{"id": id})
}

func (s *GameService) Role() string { return s.conf().Role }

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
	// Source: "env" khi GAME_ROLE đặt vai (trang chỉ hiện), "settings" khi quán nối Hub từ trang Game, rỗng khi chưa có vai.
	Source string `json:"source"`
	// Giá trị đã lưu cho ô nối Hub. Key không bao giờ trả ra, chỉ báo đã có và vài ký tự cuối.
	HubURL   string `json:"hub_url"`
	HasKey   bool   `json:"has_key"`
	KeyHint  string `json:"key_hint"`
	GameRoot string `json:"game_root"`
	// LastSyncAt và SyncError: lần đọc catalog gần nhất của quán và lỗi của nó (key sai, hết hạn, mất mạng).
	LastSyncAt *time.Time `json:"last_sync_at,omitempty"`
	SyncError  string     `json:"sync_error"`
	// Tình trạng key do Hub báo (qua report): active, grace hoặc expired, kèm ngày hết hạn. Rỗng khi chưa có tin
	// (master cũ không có report). EventsConnected: đang giữ kênh WebSocket nhận "có bản mới" từ Hub.
	KeyStatus       string     `json:"key_status"`
	KeyExpiresAt    *time.Time `json:"key_expires_at,omitempty"`
	KeyGraceUntil   *time.Time `json:"key_grace_until,omitempty"`
	EventsConnected bool       `json:"events_connected"`
	// LocalClash: game cục bộ mà catalog cũng đang phát cùng tên. Quán giữ bản cục bộ; muốn dùng bản của Hub thì xoá
	// game cục bộ đó (tệp trên ổ giữ nguyên) rồi chờ lượt đọc catalog kế tiếp.
	LocalClash []string `json:"local_clash,omitempty"`
}

func (s *GameService) Info() GameInfo {
	st := settingsGroup(s.db, GamesGroup)
	c := s.conf()
	info := GameInfo{
		Role:         c.Role,
		Root:         c.Root,
		UpstreamURL:  c.UpstreamURL,
		UpdateWindow: st[GameUpdateWindowKey],
		ClientRoot:   normalizeGameClientRoot(st[GameClientRootKey]),
		HubURL:       st[GameHubURLKey],
		HasKey:       st[GameHubKeyKey] != "",
		KeyHint:      keyHint(st[GameHubKeyKey]),
		GameRoot:     st[GameRootKey],
	}
	switch {
	case s.cfg.Role != config.GameRoleOff:
		info.Source = "env"
	case c.Role != config.GameRoleOff:
		info.Source = "settings"
	}
	s.mu.Lock()
	if !s.syncAt.IsZero() {
		at := s.syncAt
		info.LastSyncAt = &at
	}
	info.SyncError = s.syncErr
	if k := s.keyInfo; k != nil {
		info.KeyStatus = k.Status
		exp, grace := k.ExpiresAt, k.GraceUntil
		info.KeyExpiresAt, info.KeyGraceUntil = &exp, &grace
	}
	info.EventsConnected = s.eventsUp
	for name := range s.localClash {
		info.LocalClash = append(info.LocalClash, name)
	}
	sort.Strings(info.LocalClash)
	s.mu.Unlock()
	return info
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
	// Kết nối Hub đi đường riêng PUT /api/games/hub: key là bí mật, và địa chỉ phải được thử trước khi lưu.
	for _, k := range []string{GameHubURLKey, GameHubKeyKey, GameRootKey} {
		if _, ok := in[k]; ok {
			return fmt.Errorf("không ghi %q ở đây: dùng PUT /api/games/hub", k)
		}
	}
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

// List là danh sách cho trang Game, kèm số lượt bấm "Chơi" của từng game.
func (s *GameService) List() ([]model.Game, error) {
	games, err := s.listGames()
	if err != nil {
		return nil, err
	}
	// Lượt bấm "Chơi" của từng game, đếm gộp một truy vấn.
	var luot []struct {
		GameID string
		N      int64
	}
	if err := s.db.Model(&model.GameLaunch{}).Select("game_id, COUNT(*) as n").
		Group("game_id").Find(&luot).Error; err != nil {
		return nil, err
	}
	theoGame := make(map[string]int64, len(luot))
	for _, l := range luot {
		theoGame[l.GameID] = l.N
	}
	for i := range games {
		games[i].LaunchCount = theoGame[games[i].ID]
	}
	return games, nil
}

// listGames không đếm lượt chơi: Menu gọi nó mỗi lần máy trạm mở menu game.
func (s *GameService) listGames() ([]model.Game, error) {
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

// GameLaunchRequest: máy trạm báo một lượt bấm "Chơi" (Key là GameMenuItem.Key).
type GameLaunchRequest struct {
	Key         string `json:"key" binding:"required,max=200"`
	MachineCode string `json:"machine_code" binding:"max=256"`
}

// ErrGameNotFound: khoá menu không ứng với game nào (game vừa bị xoá khỏi danh sách).
var ErrGameNotFound = errors.New("không có game này")

// RecordLaunch ghi một lượt bấm "Chơi". memberID rỗng khi nhân viên kỹ thuật mở game.
func (s *GameService) RecordLaunch(req *GameLaunchRequest, memberID string) error {
	// Khoá menu là "<tên thư mục>" hoặc "<tên thư mục>/<i>"; tên thư mục không
	// chứa "/", nên phần trước dấu "/" đầu tiên luôn là tên game.
	name, _, _ := strings.Cut(req.Key, "/")
	var g model.Game
	if err := s.db.Select("id").Where("name = ?", name).First(&g).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGameNotFound
		}
		return err
	}
	l := model.GameLaunch{GameID: g.ID, MenuKey: req.Key, MachineCode: req.MachineCode}
	if memberID != "" {
		l.MemberID = &memberID
	}
	return s.db.Create(&l).Error
}

type CreateGameRequest struct {
	Name        string `json:"name" binding:"required"`
	DisplayName string `json:"display_name"`
	Category    string `json:"category"`
	Launcher    string `json:"launcher"`
	LaunchArgs  string `json:"launch_args"`
	Icon        string `json:"icon"`
	// MenuEntries: các mục riêng trên menu máy trạm (xem model.Game.MenuEntries).
	MenuEntries []model.GameMenuEntry `json:"menu_entries"`
	// RunFirst: lệnh chạy trước khi mở game.
	RunFirst []model.GameRunStep `json:"run_first"`
	Priority int                 `json:"priority"`
}

// Create thêm một game ở master. Thư mục GAME_ROOT/<name> phải có sẵn.
func (s *GameService) Create(req *CreateGameRequest) (*model.Game, error) {
	if s.conf().Role != config.GameRoleMaster {
		return nil, ErrGameMasterOnly
	}
	name := strings.TrimSpace(req.Name)
	if !tenGameDungDuoc(name) {
		return nil, errors.New("tên game chỉ gồm chữ, số, khoảng trắng và . _ ( ) + -, không quá 100 ký tự, không kết thúc bằng dấu chấm và không trùng tên thiết bị Windows (CON, NUL, COM1…)")
	}
	if err := kiemTraCacTruongChuGame(&req.DisplayName, &req.Category, &req.Launcher, &req.LaunchArgs); err != nil {
		return nil, err
	}
	if fi, err := os.Stat(filepath.Join(s.conf().Root, name)); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("không thấy thư mục %s", filepath.Join(s.conf().Root, name))
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
	g := model.Game{
		Name:        name,
		DisplayName: orDefault(strings.TrimSpace(req.DisplayName), name),
		Category:    strings.TrimSpace(req.Category),
		Launcher:    strings.TrimSpace(req.Launcher),
		LaunchArgs:  strings.TrimSpace(req.LaunchArgs),
		Icon:        strings.TrimSpace(req.Icon),
		MenuEntries: entries,
		RunFirst:    runFirst,
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
	Icon        *string `json:"icon"`
	// MenuEntries: gửi [] để bỏ các mục riêng, không gửi thì giữ nguyên.
	MenuEntries *[]model.GameMenuEntry `json:"menu_entries"`
	// RunFirst: gửi [] để bỏ lệnh chạy trước, không gửi thì giữ nguyên (master và game cục bộ).
	RunFirst *[]model.GameRunStep `json:"run_first"`
	// ClientRoot: gốc game trên máy trạm riêng cho game cục bộ ("" để theo ổ chung). Game của Hub đặt qua SetCustom.
	ClientRoot *string `json:"client_root"`
	Enabled    *bool   `json:"enabled"`
	AutoUpdate *bool   `json:"auto_update"`
	Priority   *int    `json:"priority"`
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
	// Game cục bộ là của quán: sửa được mọi trường, không có catalog nào ghi đè.
	if s.conf().Role == config.GameRoleMaster || g.Origin == model.GameOriginLocal {
		if err := kiemTraCacTruongChuGame(req.DisplayName, req.Category, req.Launcher, req.LaunchArgs); err != nil {
			return nil, err
		}
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
		if req.Icon != nil {
			if err := kiemTraIcon(*req.Icon); err != nil {
				return nil, err
			}
			updates["icon"] = strings.TrimSpace(*req.Icon)
		}
		if req.MenuEntries != nil {
			entries, err := chuanHoaMucMenu(*req.MenuEntries)
			if err != nil {
				return nil, err
			}
			updates["menu_entries"] = entries
		}
		if req.RunFirst != nil {
			steps, err := chuanHoaRunFirst(*req.RunFirst)
			if err != nil {
				return nil, err
			}
			updates["run_first"] = steps
		}
	}
	if req.ClientRoot != nil && g.Origin == model.GameOriginLocal {
		root := strings.TrimSpace(*req.ClientRoot)
		if root != "" {
			p, err := parseGameClientRoot(root)
			if err != nil {
				return nil, fmt.Errorf("gốc game trên máy trạm %q: %w", root, err)
			}
			root = p
		}
		c := g.Custom
		c.ClientRoot = root
		updates["custom"] = c
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
	master := s.conf().Role == config.GameRoleMaster
	if !master && !laUUID(id) {
		return ErrGameMasterOnly // quán chỉ xoá được game cục bộ; id lạ thì khỏi đọc DB
	}
	if !laUUID(id) {
		return gorm.ErrRecordNotFound
	}
	var g model.Game
	if err := s.db.First(&g, "id = ?", id).Error; err != nil {
		return err
	}
	if !master && g.Origin != model.GameOriginLocal {
		return ErrGameMasterOnly
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

// GameMenuItem là một mục trên menu của máy trạm. Máy trạm chỉ chạy được game
// khi Status == ready; đường dẫn đầy đủ là Root + `\` + Name + `\` + Launcher.
type GameMenuItem struct {
	// Key: khoá của mục, máy trạm gửi lại khi chạy hay lấy icon. Bằng Name khi game là
	// một mục; "<Name>/<thứ tự từ 1>" cho từng mục riêng (tên game không chứa "/").
	Key         string `json:"key"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Category    string `json:"category"`
	Launcher    string `json:"launcher"`
	LaunchArgs  string `json:"launch_args"`
	// Icon: tệp lấy icon, tương đối trong thư mục game; rỗng là lấy từ Launcher.
	Icon     string `json:"icon"`
	Status   string `json:"status"`
	Progress int    `json:"progress"`
	Version  int    `json:"version"`
	// Root: gốc game riêng của game này (khi quán đặt khác ổ chung). Rỗng là dùng GameMenu.Root.
	Root string `json:"root,omitempty"`
	// RunFirst: các bước chạy lần lượt (và chờ) trước khi mở game; máy trạm cũ không biết trường này thì bỏ qua.
	RunFirst model.GameRunSteps `json:"run_first,omitempty"`
	// Label: nhãn trên thẻ game, "hot" (nhiều lượt mở gần đây) hoặc "new" (mới về quán); rỗng là không nhãn.
	Label string `json:"label,omitempty"`
}

// GameMenu: Root là GAME_ROOT như máy trạm thấy (xem GameClientRootKey).
type GameMenu struct {
	Root  string         `json:"root"`
	Items []GameMenuItem `json:"items"`
	// CafeName và LogoURL: tên quán và đường dẫn logo (/uploads/...) hiện ở đầu menu máy trạm; rỗng là không có.
	CafeName string `json:"cafe_name,omitempty"`
	LogoURL  string `json:"logo_url,omitempty"`
}

// Menu trả các game đang bật và có tệp chạy, mọi GAME_ROLE đều dùng được. Game
// đang tải vẫn hiện (kèm tiến độ) để khách biết, nhưng chưa chạy được. Game có mục
// menu riêng ra từng mục đó, cùng thư mục và trạng thái.
func (s *GameService) Menu() (*GameMenu, error) {
	games, err := s.listGames()
	if err != nil {
		return nil, err
	}
	menu := &GameMenu{
		Root:  normalizeGameClientRoot(settingsGroup(s.db, GamesGroup)[GameClientRootKey]),
		Items: make([]GameMenuItem, 0, len(games)),
	}
	general := settingsGroup(s.db, GeneralGroup)
	menu.CafeName = truncateRunes(strings.TrimSpace(general["store_name"]), 100)
	if logo, ok := logoMenuHopLe(general[MenuLogoKey]); ok {
		menu.LogoURL = logo
	}
	now := time.Now()
	counts := s.launchCounts(now)
	sortByLaunches(games, counts)
	hot := hotGames(games, counts)
	for _, g := range games {
		if !g.Enabled {
			continue
		}
		label := menuLabel(&g, hot, now)
		item := func(key, display, launcher, args, icon string) GameMenuItem {
			// Tên đã lưu mà hỏng bảng mã (dữ liệu cũ, tạo qua đường mã hoá sai) thì hiện slug: một tên có
			// ô vuông còn tệ hơn tên không dấu. Không sửa DB — chỉ là cách hiển thị cho tới khi quản trị viên nhập lại.
			if display == "" || coKyTuLoi(display) {
				display = g.Name
			}
			return GameMenuItem{
				Key: key, Name: g.Name, DisplayName: display, Category: g.Category,
				Launcher: launcher, LaunchArgs: args, Icon: icon,
				Status: g.Status, Progress: g.Progress, Version: g.Version,
				Root: g.Custom.ClientRoot, RunFirst: runFirstOf(&g), Label: label,
			}
		}
		if len(g.MenuEntries) > 0 {
			for i, e := range g.MenuEntries {
				if e.Launcher != "" {
					menu.Items = append(menu.Items, item(g.Name+"/"+strconv.Itoa(i+1), e.DisplayName, e.Launcher, e.LaunchArgs, e.Icon))
				}
			}
			continue
		}
		if g.Launcher != "" {
			// Tên và icon quán đặt riêng thắng của catalog (game một mục).
			display, icon := orDefault(g.Custom.DisplayName, g.DisplayName), orDefault(g.Custom.Icon, g.Icon)
			menu.Items = append(menu.Items, item(g.Name, display, g.Launcher, g.LaunchArgs, icon))
		}
	}
	return menu, nil
}

// RiotMetadataSource: giá trị mặc định của riot_metadata_source khi admin để trống:
// <client_root>\<game>\_vnet\ProgramData\Riot Games của game bật đầu tiên (theo tên) mà thư mục trên
// GAME_ROOT CỦA QUÁN NÀY có bản chụp. "" nếu không phải quán hoặc chưa có.
//
// Không lọc theo trạng thái: giá trị đổi qua lại suốt mấy giờ tải lại game thì mỗi lần đổi máy trạm
// lại ghi policy.json và chép lại metadata. Bản chụp là liên kết (Lstat) thì bỏ, như máy trạm từ chối.
func (s *GameService) RiotMetadataSource() string {
	if s.conf().Role != config.GameRoleCafe || s.conf().Root == "" {
		return ""
	}
	var names []string
	if s.db.Model(&model.Game{}).Where("enabled = ?", true).Order("name").Pluck("name", &names).Error != nil {
		return ""
	}
	for _, n := range names {
		if !tenGameDungDuoc(n) {
			continue
		}
		if fi, err := os.Lstat(filepath.Join(s.conf().Root, n, "_vnet", "ProgramData", "Riot Games")); err != nil || !fi.IsDir() {
			continue
		}
		root := normalizeGameClientRoot(settingsGroup(s.db, GamesGroup)[GameClientRootKey])
		return chuanHoaNguonRiot(root + `\` + n + `\_vnet\ProgramData\Riot Games`)
	}
	return ""
}

func checkLauncher(l string) error { return checkGamePath("launcher", l) }

// checkGamePath kiểm một đường dẫn tương đối trong thư mục game (launcher, icon); nhan là tên
// trường cho lỗi. Rỗng là hợp lệ.
func checkGamePath(nhan, l string) error {
	l = strings.TrimSpace(l)
	if l == "" {
		return nil
	}
	// Bắt đầu bằng "\" hay "/" là UNC hoặc gốc ổ đĩa, không phải đường dẫn trong thư mục game.
	if filepath.IsAbs(l) || strings.HasPrefix(l, `\`) || strings.HasPrefix(l, "/") ||
		strings.Contains(l, "..") || strings.Contains(l, ":") {
		return fmt.Errorf("%s là đường dẫn tương đối trong thư mục game, ví dụ Bin/Game.exe", nhan)
	}
	for _, c := range strings.FieldsFunc(l, func(r rune) bool { return r == '/' || r == '\\' }) {
		if c != "." && thanhPhanWindowsXau(c) {
			return fmt.Errorf("%s: thành phần %q kết thúc bằng dấu chấm hoặc khoảng trắng, hoặc trùng tên thiết bị Windows (CON, NUL, COM1…)", nhan, c)
		}
	}
	return nil
}

// kiemTraIcon kiểm trường icon của game hoặc của một mục menu.
func kiemTraIcon(icon string) error {
	if err := kiemTraChuGame("icon", icon); err != nil {
		return err
	}
	if len(strings.TrimSpace(icon)) > 500 {
		return errors.New("icon dài quá 500 ký tự")
	}
	return checkGamePath("icon", icon)
}

// maxGameMenuEntries: một thư mục game hiếm khi có hơn vài chương trình đáng đưa lên menu.
const maxGameMenuEntries = 10

// chuanHoaMucMenu cắt khoảng trắng rồi kiểm các mục menu riêng của một game: mỗi mục cần tên
// hiển thị và launcher, icon tuỳ chọn. Danh sách rỗng trả nil (game là một mục như cũ).
func chuanHoaMucMenu(in []model.GameMenuEntry) (model.GameMenuEntries, error) {
	if len(in) > maxGameMenuEntries {
		return nil, fmt.Errorf("một game có tối đa %d mục trên menu", maxGameMenuEntries)
	}
	var out model.GameMenuEntries
	for i, e := range in {
		e = model.GameMenuEntry{
			DisplayName: strings.TrimSpace(e.DisplayName), Launcher: strings.TrimSpace(e.Launcher),
			LaunchArgs: strings.TrimSpace(e.LaunchArgs), Icon: strings.TrimSpace(e.Icon),
		}
		nhan := fmt.Sprintf("mục menu %d", i+1)
		if e.DisplayName == "" || e.Launcher == "" {
			return nil, fmt.Errorf("%s cần tên hiển thị và launcher", nhan)
		}
		// Cùng giới hạn với các cột của bảng games.
		if utf8.RuneCountInString(e.DisplayName) > 200 || len(e.Launcher) > 500 || len(e.LaunchArgs) > 500 {
			return nil, fmt.Errorf("%s: tên hiển thị tối đa 200 ký tự, launcher và tham số tối đa 500", nhan)
		}
		if err := kiemTraCacTruongChuGame(&e.DisplayName, nil, &e.Launcher, &e.LaunchArgs); err != nil {
			return nil, fmt.Errorf("%s: %w", nhan, err)
		}
		if err := checkLauncher(e.Launcher); err != nil {
			return nil, fmt.Errorf("%s: %w", nhan, err)
		}
		if err := kiemTraIcon(e.Icon); err != nil {
			return nil, fmt.Errorf("%s: %w", nhan, err)
		}
		out = append(out, e)
	}
	return out, nil
}

// cungMucMenu: hai danh sách mục menu giống hệt nhau (nil và rỗng là một).
func cungMucMenu(a, b model.GameMenuEntries) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------- catalog (master)

// CatalogItem là một game master công bố cho các quán.
type CatalogItem struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Category    string `json:"category"`
	Launcher    string `json:"launcher"`
	LaunchArgs  string `json:"launch_args"`
	Icon        string `json:"icon"`
	// MenuEntries: omitempty để catalog của game thường giữ nguyên như trước.
	MenuEntries model.GameMenuEntries `json:"menu_entries,omitempty"`
	Priority    int                   `json:"priority"`
	Version     int                   `json:"version"`
	Infohash    string                `json:"infohash"`
	SizeBytes   int64                 `json:"size_bytes"`
	// Webseeds: omitempty để catalog của master cũ giữ nguyên như trước. Hub điền khi dữ liệu nằm ở máy master khác.
	Webseeds []string `json:"webseeds,omitempty"`
	// Trackers: tracker WebRTC để quán gặp master qua P2P, URL đã ký cho quán này. Hub điền khi bật tracker; master
	// cũ không có. Có Trackers mà không có Webseeds nghĩa là master chỉ phát qua P2P.
	Trackers []string `json:"trackers,omitempty"`
	// RelayWebseeds: relay của Hub (HTTP qua đường hầm của master), đã ký cho quán này. Chỉ dùng khi P2P và webseed đều
	// bế tắc. Master cũ và Hub chưa bật relay không có trường này.
	RelayWebseeds []string `json:"relay_webseeds,omitempty"`
	// RunFirst: lệnh chạy trước khi mở game. omitempty để catalog của game thường giữ nguyên như trước.
	RunFirst model.GameRunSteps `json:"run_first,omitempty"`
}

// SeedToken là phần bí mật trong URL webseed: /seed/<token>/<game>/<tệp>.
// Engine BitTorrent không gửi được header khi gọi webseed, nên khoá phải nằm
// trong đường dẫn. Dùng băm của khoá catalog chứ không dùng chính nó, để log
// truy cập của master không làm lộ khoá đọc catalog.
func (s *GameService) SeedToken() string {
	if s.conf().CatalogKey == "" {
		return ""
	}
	sum := sha256.Sum256([]byte("vnet-seed:" + s.conf().CatalogKey))
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
	return s.conf().CatalogKey != "" && subtle.ConstantTimeCompare([]byte(key), []byte(s.conf().CatalogKey)) == 1
}

func (s *GameService) Catalog() ([]CatalogItem, error) {
	if s.conf().Role != config.GameRoleMaster {
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
			Launcher: g.Launcher, LaunchArgs: g.LaunchArgs, Icon: g.Icon, MenuEntries: g.MenuEntries,
			Priority: g.Priority, Version: g.Version, Infohash: g.Infohash, SizeBytes: g.SizeBytes,
			RunFirst: g.RunFirst,
		})
	}
	return out, nil
}

// Torrent trả tệp .torrent của bản mới nhất.
func (s *GameService) Torrent(name string) ([]byte, error) {
	if s.conf().Role != config.GameRoleMaster {
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

// torrentFileIOEnv: cách anacrolix đọc tệp đang phát, do bộ cài đặt vào môi trường của dịch vụ
// Windows (=classic). Kiểm lại khi nâng anacrolix (đang v1.61.0).
const torrentFileIOEnv = "TORRENT_STORAGE_DEFAULT_FILE_IO"

// torrentFileIOClassic đọc biến lúc nạp gói như anacrolix, trước khi loadServiceConfig chép
// config.env vào môi trường: biến đặt trong config.env không đổi được cách anacrolix đọc tệp.
var torrentFileIOClassic = os.Getenv(torrentFileIOEnv) == "classic"

// runRole chạy vai đang có hiệu lực (master hoặc quán) cho tới khi ctx kết thúc. Run (game_hub.go) gọi nó và
// dừng rồi bật lại khi quán nối hay ngắt Hub. Không làm gì khi chưa có vai.
func (s *GameService) runRole(ctx context.Context) {
	if s.conf().Role == config.GameRoleOff {
		return
	}
	dataDir := s.conf().DataDir
	if dataDir == "" || !filepath.IsAbs(dataDir) {
		dataDir = filepath.Join(s.conf().Root, ".vnet-gameupdate")
	}
	c := s.conf()
	engine, err := gameupdate.NewEngine(gameupdate.EngineConfig{
		DataDir:    dataDir,
		ListenPort: c.ListenPort,
		// Quán chỉ cần P2P qua tracker của Hub: engine không mở cổng BitTorrent nào. Master của core vẫn dùng cổng cũ.
		WebRTC:     c.Role == config.GameRoleCafe && c.WebRTC,
		ICEServers: splitList(c.ICEServers),
		// Trần băng thông quán đặt ở trang Danh mục Hub; không đặt thì không giới hạn.
		DownloadBps: s.Limits().DownloadMbps * mbpsBytes, UploadBps: s.Limits().UploadMbps * mbpsBytes,
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
	log.Printf("[game] bật vai trò %s, thư mục %s", s.conf().Role, s.conf().Root)
	if runtime.GOOS == "windows" && !torrentFileIOClassic {
		log.Printf("[game] CẢNH BÁO: dịch vụ thiếu %s=classic: tệp đã phát bị khoá tới khi dịch vụ dừng "+
			"(quán không xoá được tệp thừa, master không chụp được ProgramData). Chạy lại bộ cài vnet-server.", torrentFileIOEnv)
	}

	s.recoverDownloading()
	s.recoverPublishing()

	if s.conf().Role == config.GameRoleMaster {
		s.seedPublished(ctx)
	} else {
		worker.Add(2)
		go func() {
			defer worker.Done()
			s.installLoop(ctx)
		}()
		go func() {
			defer worker.Done()
			s.eventsLoop(ctx)
		}()
	}
	for {
		if s.conf().Role == config.GameRoleMaster {
			s.publishTick(ctx, time.Now())
		} else {
			s.syncTick(ctx)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(s.conf().PollInterval):
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
		s.baoDoi("")
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
	if len(games) > 0 {
		s.baoDoi("")
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
		job := gameupdate.Job{Name: g.Name, MetaInfo: mi, Dest: filepath.Join(s.conf().Root, g.Name)}
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
		dir := filepath.Join(s.conf().Root, g.Name)
		if err := s.captureLauncherState(dir); err != nil {
			// Chỉ ở master, để người quản trị đọc: Catalog không xét trạng thái nên quán vẫn nhận bản cũ.
			// Lượt này không fingerprint, không Observe, không publish.
			s.setError(g.ID, err.Error())
			continue
		}
		fp, err := gameupdate.Fingerprint(dir)
		if err != nil {
			s.setError(g.ID, fmt.Sprintf("không đọc được thư mục: %v", err))
			continue
		}
		// Thư mục đã đọc được và trùng bản đã publish: lỗi trước đó (không đọc được thư mục, hoặc một
		// lần publish hỏng rồi thư mục trở lại như cũ) chỉ là tạm thời. Không gỡ thì /api/game-menu
		// báo mãi một game đang tốt là chưa sẵn sàng, vì không còn gì đưa nó về "ready".
		if g.Status == model.GameStatusError && g.Version > 0 && fp == g.Fingerprint {
			if s.db.Model(&model.Game{}).Where("id = ? AND status = ?", g.ID, model.GameStatusError).
				Updates(map[string]interface{}{"status": model.GameStatusReady, "error": ""}).RowsAffected > 0 {
				s.baoDoi(g.ID)
			}
		}
		s.mu.Lock()
		st := s.stabilizers[g.Name]
		if st == nil {
			st = gameupdate.NewStabilizer(s.conf().StableFor, g.Fingerprint)
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
	if s.conf().Role != config.GameRoleMaster {
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
	dir := filepath.Join(s.conf().Root, g.Name)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("không thấy thư mục %s", dir)
	}
	s.db.Model(&model.Game{}).Where("id = ?", g.ID).Updates(map[string]interface{}{
		"status": model.GameStatusPublishing, "error": "",
	})
	go func(g model.Game) { // bản sao: lệnh First cuối hàm ghi lại g trong lúc goroutine còn đọc
		err := s.captureLauncherState(dir)
		var fp string
		if err == nil {
			fp, err = gameupdate.Fingerprint(dir)
		}
		if err == nil && (fp != g.Fingerprint || g.Version == 0) {
			err = s.publish(ctx, g, dir, fp)
		}
		if err != nil {
			log.Printf("[game] publish %s: %v", g.Name, err)
			s.setError(g.ID, err.Error())
			return
		}
		// Thư mục không đổi kể từ lần publish trước: chỉ trả lại trạng thái.
		if s.db.Model(&model.Game{}).Where("id = ? AND status = ?", g.ID, model.GameStatusPublishing).
			Update("status", model.GameStatusReady).RowsAffected > 0 {
			s.baoDoi(g.ID)
		}
		s.mu.Lock()
		if st := s.stabilizers[g.Name]; st != nil {
			st.MarkPublished(fp)
		}
		s.mu.Unlock()
	}(g)
	if err := s.db.First(&g, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &g, nil
}

// captureLauncherState: chỉ game Riot. Chép %ProgramData%\Riot Games của master vào thư mục game
// (gameupdate.CaptureRiot) trước khi fingerprint, để máy trạm nhận trạng thái cài cùng bản game.
// Giữ pubMu để không bao giờ ghi đè tệp mà publish() (BuildTorrent) đang băm, và hai lượt chụp
// không ghi cùng một tệp. Không bao giờ gọi khi đang giữ pubMu.
func (s *GameService) captureLauncherState(dir string) error {
	if !gameupdate.IsRiotGame(dir) {
		return nil // không khoá: một lần băm dài ở game khác không được làm kẹt lượt này
	}
	pd := os.Getenv("ProgramData")
	if pd == "" && runtime.GOOS == "windows" {
		pd = `C:\ProgramData`
	}
	if pd == "" {
		return nil
	}
	src := filepath.Join(pd, "Riot Games")
	s.pubMu.Lock()
	defer s.pubMu.Unlock()
	// Riot chạy từ thư mục khác thì đây là game thường, publish không cần classic: xét trước.
	err := gameupdate.RiotElsewhere(src, dir)
	if err == nil {
		if runtime.GOOS == "windows" && !torrentFileIOClassic {
			return fmt.Errorf("dịch vụ thiếu %s=classic nên không ghi được vào thư mục đang phát: chạy lại bộ cài vnet-server rồi khởi động lại dịch vụ", torrentFileIOEnv)
		}
		var n int
		n, err = gameupdate.CaptureRiot(src, dir)
		if n > 0 {
			log.Printf("[game] %s: chụp ProgramData\\Riot Games vào _vnet, %d thay đổi", filepath.Base(dir), n)
		}
	}
	if errors.Is(err, gameupdate.ErrRiotElsewhere) {
		// Không phải lỗi: publish thư mục như game thường. Nhưng quán sẽ không có bản chụp, nên
		// báo một lần trong nhật ký, không thì không ai biết tính năng đang tắt.
		if !s.riotLogged[dir] {
			s.riotLogged[dir] = true
			log.Printf("[game] %s: %v", filepath.Base(dir), err)
		}
		err = nil // chưa có bản chụp nào, nên không có gì dở
	}
	// Chụp lỗi có thể đã ghi/xoá dở bản chụp, mà lượt publish kia có thể đã chụp xong và fingerprint
	// trước lỗi này: publish() đợi lần chụp thành công sau, không băm bản dở.
	if err != nil {
		s.riotBroken[dir] = true
	} else {
		delete(s.riotBroken, dir)
	}
	return err
}

func (s *GameService) publish(ctx context.Context, g model.Game, dir, fp string) error {
	s.pubMu.Lock()
	defer s.pubMu.Unlock()
	if s.riotBroken[dir] && gameupdate.IsRiotGame(dir) {
		return errors.New(`bản chụp ProgramData\Riot Games trong thư mục game chưa trọn (lần chụp gần nhất lỗi): chưa publish, sẽ thử lại sau lần chụp thành công`)
	}
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
	s.baoDoi(g.ID)
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
	s.baoDoi(id)
}

// ---------------------------------------------------------------- đồng bộ (quán)

// SyncNow tải ngay một game ở quán, bỏ qua tự cập nhật, khung giờ và thời gian chờ thử lại.
func (s *GameService) SyncNow(id string) error {
	if s.conf().Role != config.GameRoleCafe {
		return ErrGameCafeOnly
	}
	if !laUUID(id) {
		return gorm.ErrRecordNotFound
	}
	var g model.Game
	if err := s.db.First(&g, "id = ?", id).Error; err != nil {
		return err
	}
	if g.Origin == model.GameOriginLocal {
		return ErrGameIsLocal
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
	err := s.pullCatalog(ctx)
	s.noteSync(err)
	if err != nil {
		log.Printf("[game] đọc catalog của master: %v", err)
	}
	// Báo cả khi đọc catalog lỗi: key hết hạn bị catalog từ chối nhưng report vẫn nhận, và chính phản hồi của nó cho
	// quán biết key hết hạn ngày nào.
	s.reportToHub(ctx)
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
			s.reportToHub(ctx) // Hub thấy bản mới của quán ngay, không đợi lượt đọc catalog sau
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
		s.baoDoi(g.ID)
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
		msg = fmt.Sprintf("Tải bị treo: %s không có thêm dữ liệu nào đúng hash (master không phản hồi, hoặc master đã đổi tệp sau khi publish)", thoiLuong(s.conf().StallTimeout))
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
	s.mu.Lock()
	s.localClash = nil // dựng lại từ lượt đọc này: xoá game cục bộ hay catalog gỡ game thì hết trùng
	s.mu.Unlock()
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
			"icon": it.Icon, "menu_entries": it.MenuEntries,
			"remote_version": it.Version, "remote_infohash": it.Infohash,
			"remote_webseeds": model.StringArray(webseedsHopLe(it.Name, it.Webseeds)),
			"remote_trackers": model.StringArray(trackersHopLe(it.Name, it.Trackers)),
			"remote_relays":   model.StringArray(relaysHopLe(it.Name, it.RelayWebseeds)),
			"run_first":       runFirstHopLe(it.Name, it.RunFirst),
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			g = model.Game{
				Name: it.Name, DisplayName: it.DisplayName, Category: it.Category,
				Launcher: it.Launcher, LaunchArgs: it.LaunchArgs, Icon: it.Icon, MenuEntries: it.MenuEntries,
				Priority: it.Priority, Enabled: true, AutoUpdate: true, Status: model.GameStatusNotInstalled,
				RemoteVersion: it.Version, RemoteInfohash: it.Infohash, SizeBytes: it.SizeBytes,
				RemoteWebseeds: webseedsHopLe(it.Name, it.Webseeds), RemoteTrackers: trackersHopLe(it.Name, it.Trackers),
				RemoteRelays: relaysHopLe(it.Name, it.RelayWebseeds), RunFirst: runFirstHopLe(it.Name, it.RunFirst),
			}
			if err := s.db.Create(&g).Error; err != nil {
				return err
			}
			s.baoDoi(g.ID)
			continue
		}
		if err != nil {
			return err
		}
		if g.Origin == model.GameOriginLocal {
			// Game cục bộ trùng tên với một game của catalog: quán giữ bản của mình, catalog không ghi đè.
			s.noteLocalClash(it.Name)
			continue
		}
		// Vòng nền ghi lại catalog mỗi lượt dù không có gì mới: chỉ báo trang
		// quản trị khi có cột thật sự đổi.
		doi := g.Category != it.Category || g.Launcher != it.Launcher || g.LaunchArgs != it.LaunchArgs ||
			g.Icon != it.Icon || !cungMucMenu(g.MenuEntries, it.MenuEntries) ||
			g.RemoteVersion != it.Version || g.RemoteInfohash != it.Infohash
		// Tên của master đã hỏng bảng mã thì không ghi đè tên đang tốt ở quán; quán chưa có tên tốt thì
		// nhận như thường (Menu lo phần hiển thị).
		if coKyTuLoi(it.DisplayName) && g.DisplayName != "" && !coKyTuLoi(g.DisplayName) {
			log.Printf("[game] %s: master gửi tên hiển thị hỏng bảng mã, giữ tên cũ %q", it.Name, g.DisplayName)
			delete(fields, "display_name")
		}
		if _, ok := fields["display_name"]; ok && g.DisplayName != it.DisplayName {
			doi = true
		}
		if err := s.db.Model(&g).Updates(fields).Error; err != nil {
			return err
		}
		if doi {
			s.baoDoi(g.ID)
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
	// Trang Game thấy "đang tải" rồi tự hỏi phần trăm mỗi 3 giây (lấy từ engine,
	// không ghi DB); lúc xong hay lỗi thì báo lại lần nữa.
	s.baoDoi(g.ID)
	log.Printf("[game] bắt đầu tải %s bản %d", g.Name, g.RemoteVersion)

	e := s.getEngine()
	if e == nil {
		return ErrGameRoleOff
	}
	job := gameupdate.Job{
		Name:         g.Name,
		MetaInfo:     mi,
		Dest:         filepath.Join(s.conf().Root, g.Name),
		Webseeds:     s.webseedsFor(g),
		Trackers:     g.RemoteTrackers,
		Relays:       g.RemoteRelays,
		RelayDelay:   s.conf().RelayDelay,
		WebseedDelay: s.conf().WebseedDelay,
		StallTimeout: s.conf().StallTimeout,
		OnTransfer:   func(t gameupdate.Transfer) { s.noteTransfer(g.Name, g.RemoteVersion, t) },
		OnRelayFallback: func() {
			log.Printf("[game] %s: P2P và webseed đều không có thêm dữ liệu nào sau %s, dùng relay của Hub", g.Name, s.conf().RelayDelay)
		},
		OnWebseedFallback: func() {
			log.Printf("[game] %s: P2P không có thêm dữ liệu nào sau %s, dùng webseed", g.Name, s.conf().WebseedDelay)
		},
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
	if err := s.db.Model(&model.Game{}).Where("id = ?", g.ID).Updates(map[string]interface{}{
		"status": model.GameStatusReady, "progress": 100, "error": "",
		"version": g.RemoteVersion, "infohash": hash, "last_updated_at": now,
	}).Error; err != nil {
		return err
	}
	s.baoDoi(g.ID)
	return nil
}

func (s *GameService) upstreamBytes(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.conf().UpstreamURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(GameCatalogKeyHeader, s.conf().CatalogKey)
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
		return nil, fmt.Errorf("%s trả %d: %s", path, resp.StatusCode, truncateRunes(hubText(body), 200))
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

// hubText: lời nhắn trong phản hồi lỗi {code, message} của Hub để quản trị viên đọc được ("sai khoá catalog
// game"), thay vì cả khối JSON. Không đúng dạng đó thì giữ nguyên nội dung.
func hubText(body []byte) string {
	var env struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &env) == nil && env.Message != "" {
		return env.Message
	}
	return string(body)
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
