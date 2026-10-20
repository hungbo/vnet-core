package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/model"
)

// Danh mục Hub ở quán (roadmap H3, H4): quán xem MỌI game Hub hỗ trợ, chọn game cần tải, tải hàng loạt, sửa chữa, xem
// hàng đợi và đặt trần băng thông. Việc chọn nằm ở Hub (PUT /api/game-catalog/selection): Hub chỉ đưa vào catalog những
// game quán đã chọn, nên mọi quán khác nhau nhận đúng danh sách của mình.

// Giới hạn băng thông đặt bằng Mbps (cách quán nói về đường truyền); engine dùng byte/giây.
const (
	GameDownloadLimitKey = "download_limit_mbps"
	GameUploadLimitKey   = "upload_limit_mbps"
	mbpsBytes            = int64(125_000) // 1 Mbps = 125.000 byte/giây
	maxLimitMbps         = 100_000
)

var (
	// ErrHubSelectionUnsupported: Hub chưa có đường chọn game (bản Hub cũ).
	ErrHubSelectionUnsupported = errors.New("Hub chưa hỗ trợ chọn game: nhờ nhà cung cấp nâng cấp Hub")
	ErrHubTooMany              = errors.New("tối đa 1000 game mỗi lần")
)

// hubBrowseItem và hubBrowse là hình dạng GET /api/game-catalog/all của Hub.
type hubBrowseItem struct {
	Name         string `json:"name"`
	DisplayName  string `json:"display_name"`
	Category     string `json:"category"`
	SourceType   string `json:"source_type"`
	SizeBytes    int64  `json:"size_bytes"`
	ApproxSizeMB int    `json:"approx_size_mb"`
	Available    bool   `json:"available"`
	Version      int    `json:"version"`
	Mandatory    bool   `json:"mandatory"`
	Selected     bool   `json:"selected"`
}

type hubBrowse struct {
	Mode          string          `json:"mode"`
	SelectedCount int             `json:"selected_count"`
	Items         []hubBrowseItem `json:"items"`
}

// HubCatalogLocal là tình trạng của game ở quán; nil khi quán chưa có bản ghi game này.
type HubCatalogLocal struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	Progress   int    `json:"progress"`
	Version    int    `json:"version"` // bản đã nằm trọn trên ổ
	AutoUpdate bool   `json:"auto_update"`
	Priority   int    `json:"priority"`
	Error      string `json:"error"`
	// NeedsUpdate: master đã có bản khác bản trên ổ.
	NeedsUpdate bool `json:"needs_update"`
}

// HubCatalogGame là một dòng của trang Danh mục Hub.
type HubCatalogGame struct {
	hubBrowseItem
	Local *HubCatalogLocal `json:"local,omitempty"`
}

type HubCatalogView struct {
	Mode          string           `json:"mode"`
	SelectedCount int              `json:"selected_count"`
	Items         []HubCatalogGame `json:"items"`
}

// hubCatalogError đổi lỗi gọi Hub thành thông báo dễ hiểu: Hub cũ trả 404 hoặc trang web thay vì JSON.
func hubCatalogError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "trả 404") || strings.Contains(msg, "invalid character") {
		return ErrHubSelectionUnsupported
	}
	return err
}

func (s *GameService) requireCafe() error {
	if s.conf().Role != config.GameRoleCafe {
		return ErrGameCafeOnly
	}
	return nil
}

// HubCatalog đọc danh mục đầy đủ từ Hub rồi ghép với tình trạng game ở quán.
func (s *GameService) HubCatalog(ctx context.Context) (*HubCatalogView, error) {
	if err := s.requireCafe(); err != nil {
		return nil, err
	}
	var b hubBrowse
	if err := s.upstreamJSON(ctx, "/api/game-catalog/all", &b); err != nil {
		return nil, hubCatalogError(err)
	}
	games, err := s.listGames()
	if err != nil {
		return nil, err
	}
	local := make(map[string]model.Game, len(games))
	for _, g := range games {
		if g.Origin != model.GameOriginLocal {
			local[g.Name] = g
		}
	}
	view := &HubCatalogView{Mode: b.Mode, SelectedCount: b.SelectedCount, Items: make([]HubCatalogGame, 0, len(b.Items))}
	for _, it := range b.Items {
		row := HubCatalogGame{hubBrowseItem: it}
		if g, ok := local[it.Name]; ok {
			row.Local = &HubCatalogLocal{
				ID: g.ID, Status: g.Status, Progress: g.Progress, Version: g.Version, AutoUpdate: g.AutoUpdate,
				Priority: g.Priority, Error: g.Error,
				NeedsUpdate: g.RemoteInfohash != "" && g.Infohash != g.RemoteInfohash,
			}
		}
		view.Items = append(view.Items, row)
	}
	return view, nil
}

// HubSelectionRequest: Mode "all" quay về nhận mọi game; còn lại áp Add và Remove lên danh sách đã chọn. DownloadNow xếp
// ngay các game vừa chọn vào hàng đợi, bỏ qua khung giờ cập nhật.
type HubSelectionRequest struct {
	Mode        string   `json:"mode"`
	Add         []string `json:"add"`
	Remove      []string `json:"remove"`
	DownloadNow bool     `json:"download_now"`
}

type HubSelectionResult struct {
	Mode          string   `json:"mode"`
	SelectedCount int      `json:"selected_count"`
	Unknown       []string `json:"unknown,omitempty"`
	// Queued: số game đã xếp vào hàng đợi tải ngay.
	Queued int `json:"queued"`
}

// SelectHubGames chuyển lựa chọn của quán cho Hub, đọc lại catalog để game mới có mặt ở quán, và (tuỳ chọn) xếp hàng tải.
func (s *GameService) SelectHubGames(ctx context.Context, req HubSelectionRequest) (*HubSelectionResult, error) {
	if err := s.requireCafe(); err != nil {
		return nil, err
	}
	if len(req.Add)+len(req.Remove) > 1000 {
		return nil, ErrHubTooMany
	}
	body, _ := json.Marshal(map[string]interface{}{"mode": req.Mode, "add": req.Add, "remove": req.Remove})
	var res HubSelectionResult
	if err := s.upstreamSend(ctx, http.MethodPut, "/api/game-catalog/selection", body, &res); err != nil {
		return nil, hubCatalogError(err)
	}

	// Game bỏ chọn thôi tự cập nhật (tệp và mục menu giữ nguyên: xoá game là việc riêng ở trang Game); game chọn lại thì
	// bật lại. Catalog của Hub sẽ không còn mang bản mới của game bỏ chọn, nên tắt tự cập nhật chỉ làm trạng thái trung thực.
	if len(req.Remove) > 0 {
		s.db.Model(&model.Game{}).Where("name IN ? AND origin = ?", req.Remove, model.GameOriginHub).Update("auto_update", false)
	}
	if len(req.Add) > 0 {
		s.db.Model(&model.Game{}).Where("name IN ? AND origin = ?", req.Add, model.GameOriginHub).Update("auto_update", true)
	}
	// Đọc catalog ngay: game vừa chọn có bản ghi ở quán trước khi trang tải lại danh sách.
	if err := s.pullCatalog(ctx); err != nil {
		return nil, fmt.Errorf("đã lưu lựa chọn nhưng chưa đọc lại được catalog: %w", err)
	}
	if req.DownloadNow && len(req.Add) > 0 {
		n, err := s.queueByNames(req.Add)
		if err != nil {
			return nil, err
		}
		res.Queued = n
	}
	s.Kick()
	return &res, nil
}

// queueByNames xếp các game Hub (đã có bản master) vào hàng đợi "tải ngay". Game đang tải không bị đụng; game chưa có
// bản master (chưa phát được) bị bỏ qua. Trả về số game đã xếp.
func (s *GameService) queueByNames(names []string) (int, error) {
	if len(names) == 0 {
		return 0, nil
	}
	res := s.db.Model(&model.Game{}).
		Where("name IN ? AND origin = ? AND remote_infohash <> '' AND status <> ?", names, model.GameOriginHub, model.GameStatusDownloading).
		Update("status", model.GameStatusQueued)
	return int(res.RowsAffected), res.Error
}

// BulkSync là "Tải hàng loạt" và "Sửa chữa nhiều game": xếp các game vào hàng đợi, bỏ qua tự cập nhật và khung giờ. Với game
// đã có đúng bản master, lượt này băm lại mọi tệp, tải phần thiếu hoặc hỏng và xoá tệp thừa (Engine.Install luôn kiểm lại ổ).
func (s *GameService) BulkSync(names []string) (int, error) {
	if err := s.requireCafe(); err != nil {
		return 0, err
	}
	if len(names) > 1000 {
		return 0, ErrHubTooMany
	}
	n, err := s.queueByNames(names)
	if err != nil {
		return 0, err
	}
	s.Kick()
	return n, nil
}

// upstreamSend gọi Hub bằng phương thức tuỳ ý với thân JSON, đọc phản hồi {code, message, data}.
func (s *GameService) upstreamSend(ctx context.Context, method, path string, body []byte, out interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, s.conf().UpstreamURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set(GameCatalogKeyHeader, s.conf().CatalogKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s trả %d: %s", path, resp.StatusCode, truncateRunes(hubText(raw), 200))
	}
	var env struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return err
	}
	if env.Code != 0 {
		return errors.New(env.Message)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(env.Data, out)
}

// ---------------------------------------------------------------- băng thông

type GameLimits struct {
	DownloadMbps int64 `json:"download_mbps"`
	UploadMbps   int64 `json:"upload_mbps"`
}

// Limits đọc trần băng thông đã lưu (0 là không giới hạn).
func (s *GameService) Limits() GameLimits {
	st := settingsGroup(s.db, GamesGroup)
	return GameLimits{
		DownloadMbps: min(max(settingInt(st, GameDownloadLimitKey, 0), 0), maxLimitMbps),
		UploadMbps:   min(max(settingInt(st, GameUploadLimitKey, 0), 0), maxLimitMbps),
	}
}

// SetLimits lưu trần băng thông và áp ngay cho engine đang chạy, kể cả lượt tải dở.
func (s *GameService) SetLimits(l GameLimits) (*GameLimits, error) {
	if l.DownloadMbps < 0 || l.UploadMbps < 0 || l.DownloadMbps > maxLimitMbps || l.UploadMbps > maxLimitMbps {
		return nil, fmt.Errorf("trần băng thông từ 0 (không giới hạn) tới %d Mbps", maxLimitMbps)
	}
	if err := saveGameSettings(s.db, map[string]string{
		GameDownloadLimitKey: fmt.Sprint(l.DownloadMbps), GameUploadLimitKey: fmt.Sprint(l.UploadMbps),
	}); err != nil {
		return nil, err
	}
	if e := s.getEngine(); e != nil {
		e.SetLimits(l.DownloadMbps*mbpsBytes, l.UploadMbps*mbpsBytes)
	}
	return &l, nil
}

// ---------------------------------------------------------------- hàng đợi

// rateMeter đo tốc độ giữa hai lần hỏi: tốc độ tức thời trộn với giá trị cũ để số không nhảy lung tung.
type rateMeter struct {
	at    time.Time
	bytes int64
	bps   int64
}

func (m *rateMeter) sample(now time.Time, bytes int64) int64 {
	if m.at.IsZero() || bytes < m.bytes { // lượt tải mới (bộ đếm về 0)
		m.at, m.bytes, m.bps = now, bytes, 0
		return 0
	}
	dt := now.Sub(m.at)
	if dt < time.Second {
		return m.bps
	}
	inst := int64(float64(bytes-m.bytes) / dt.Seconds())
	if m.bps == 0 {
		m.bps = inst
	} else {
		m.bps = (m.bps + inst) / 2
	}
	m.at, m.bytes = now, bytes
	return m.bps
}

// Trạng thái của một game trong hàng đợi.
const (
	QueueDownloading = "downloading" // đang tải
	QueueNext        = "queued"      // tới lượt khi luồng tải rảnh
	QueueWindow      = "window"      // đợi khung giờ cập nhật
	QueueRetry       = "retry"       // lỗi, đang chờ thử lại
	QueueManual      = "manual"      // có bản mới nhưng tắt tự cập nhật: cần bấm Tải ngay
)

type QueueEntry struct {
	Name        string     `json:"name"`
	DisplayName string     `json:"display_name"`
	Priority    int        `json:"priority"`
	State       string     `json:"state"`
	Progress    int        `json:"progress"`
	BytesPerSec int64      `json:"bytes_per_sec"`
	Peers       int        `json:"peers"`
	SizeBytes   int64      `json:"size_bytes"`
	RetryAt     *time.Time `json:"retry_at,omitempty"`
	Error       string     `json:"error,omitempty"`
}

type QueueView struct {
	Current      *QueueEntry  `json:"current"`
	Waiting      []QueueEntry `json:"waiting"`
	InWindow     bool         `json:"in_window"`
	UpdateWindow string       `json:"update_window"`
	Limits       GameLimits   `json:"limits"`
}

// retryAt: thời điểm game được thử lại, nil nếu không đang chờ.
func (s *GameService) retryAt(g *model.Game, now time.Time) *time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.retry[g.Name]
	if r != nil && r.infohash == g.RemoteInfohash && now.Before(r.notBefore) {
		t := r.notBefore
		return &t
	}
	return nil
}

// Queue cho thấy luồng tải đang làm gì và những game nào đang chờ, theo đúng luật nextInstall (ưu tiên cao trước).
func (s *GameService) Queue(now time.Time) (*QueueView, error) {
	if err := s.requireCafe(); err != nil {
		return nil, err
	}
	var games []model.Game
	if err := s.db.Where("enabled = ? AND origin = ? AND remote_infohash <> ''", true, model.GameOriginHub).Find(&games).Error; err != nil {
		return nil, err
	}
	sort.SliceStable(games, func(i, j int) bool { return games[i].Priority > games[j].Priority })

	st := settingsGroup(s.db, GamesGroup)
	view := &QueueView{
		InWindow: withinWindow(st[GameUpdateWindowKey], now), UpdateWindow: st[GameUpdateWindowKey],
		Limits: s.Limits(), Waiting: []QueueEntry{},
	}
	e := s.getEngine()
	for i := range games {
		g := &games[i]
		entry := QueueEntry{Name: g.Name, DisplayName: g.DisplayName, Priority: g.Priority, SizeBytes: g.SizeBytes, Error: g.Error}
		upToDate := g.Infohash == g.RemoteInfohash
		switch {
		case g.Status == model.GameStatusDownloading:
			entry.State, entry.Progress = QueueDownloading, g.Progress
			if e != nil {
				if p, ok := e.Progress(g.Name); ok {
					if p.Length > 0 {
						entry.Progress = int(p.BytesCompleted * 100 / p.Length)
						entry.SizeBytes = p.Length
					}
					entry.Peers = p.Peers
					s.mu.Lock()
					m := s.speeds[g.Name]
					if m == nil {
						m = &rateMeter{}
						s.speeds[g.Name] = m
					}
					entry.BytesPerSec = m.sample(now, p.Received)
					s.mu.Unlock()
				}
			}
			view.Current = &entry
			continue
		case g.Status == model.GameStatusQueued:
			entry.State = QueueNext
		case upToDate && g.Status == model.GameStatusError:
			if at := s.retryAt(g, now); at != nil {
				entry.State, entry.RetryAt = QueueRetry, at
			} else {
				entry.State = QueueNext
			}
		case upToDate:
			continue // đã đúng bản master, không có gì để tải
		case !g.AutoUpdate:
			entry.State = QueueManual
		case !view.InWindow:
			entry.State = QueueWindow
		default:
			if at := s.retryAt(g, now); at != nil {
				entry.State, entry.RetryAt = QueueRetry, at
			} else {
				entry.State = QueueNext
			}
		}
		view.Waiting = append(view.Waiting, entry)
	}
	// Game thôi không tải nữa thì bỏ bộ đo của nó.
	s.mu.Lock()
	for name := range s.speeds {
		if view.Current == nil || view.Current.Name != name {
			delete(s.speeds, name)
		}
	}
	s.mu.Unlock()
	return view, nil
}
