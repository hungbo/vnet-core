package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vnet/core/internal/gameupdate"
	"github.com/vnet/core/internal/model"
)

// CoreVersion là phiên bản vnet-server, quán báo lên Hub cùng tình trạng game. main gán từ -ldflags -X main.version.
var CoreVersion = "dev"

// Quán và Hub nói chuyện thêm hai đường ngoài catalog:
//   - POST /api/game-catalog/report: quán báo bản và trạng thái từng game; Hub trả tình trạng key (còn hạn, ân hạn,
//     hết hạn). Hub dùng nó để vẽ ma trận đồng bộ và cảnh báo, quán dùng nó để hiện ngày hết hạn.
//   - GET /api/game-catalog/events (WebSocket): Hub đẩy "game.catalog.changed" để quán đọc catalog ngay thay vì đợi
//     chu kỳ GAME_POLL_INTERVAL.
// Cả hai là phần thêm: master cũ (vai master của core) không có, trả 404, và quán lặng lẽ thôi gọi.

// hubKeyInfo là tình trạng key do Hub báo.
type hubKeyInfo struct {
	Status     string    `json:"status"`
	ExpiresAt  time.Time `json:"expires_at"`
	GraceUntil time.Time `json:"grace_until"`
}

type reportGame struct {
	Name     string `json:"name"`
	Version  int    `json:"version"`
	Status   string `json:"status"`
	Progress int    `json:"progress"`
	Error    string `json:"error"`
}

// reportTransfer là một lần cài xong: số byte nhận được qua từng đường. Hub cộng dồn theo ngày để biết bao nhiêu thật sự
// đi P2P, qua webseed hay qua relay (và trả cho relay bao nhiêu băng thông).
type reportTransfer struct {
	Game         string `json:"game"`
	Version      int    `json:"version"`
	P2PBytes     int64  `json:"p2p_bytes"`
	WebseedBytes int64  `json:"webseed_bytes"`
	RelayBytes   int64  `json:"relay_bytes"`
}

type reportBody struct {
	CoreVersion  string           `json:"core_version"`
	MachineCount int              `json:"machine_count"`
	Games        []reportGame     `json:"games"`
	Transfers    []reportTransfer `json:"transfers,omitempty"`
}

// maxPendingTransfers: Hub không nhận hay quán tắt mạng lâu thì bỏ mục cũ nhất, không để danh sách phình mãi.
const maxPendingTransfers = 200

// noteTransfer ghi một lần cài xong để gửi trong report kế tiếp. Lần cài không nhận byte nào (dữ liệu đã đúng sẵn trên
// đĩa) không đáng báo. Mất khi quán khởi động lại trước khi report được gửi.
func (s *GameService) noteTransfer(game string, version int, t gameupdate.Transfer) {
	if t.Total() == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.noReport {
		return
	}
	s.transfers = append(s.transfers, reportTransfer{Game: game, Version: version, P2PBytes: t.P2P, WebseedBytes: t.Webseed, RelayBytes: t.Relay})
	if len(s.transfers) > maxPendingTransfers {
		s.transfers = s.transfers[len(s.transfers)-maxPendingTransfers:]
	}
}

// takeTransfers lấy danh sách chờ gửi; giữ lại bằng putBackTransfers nếu gửi không thành.
func (s *GameService) takeTransfers() []reportTransfer {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.transfers
	s.transfers = nil
	return out
}

func (s *GameService) putBackTransfers(ts []reportTransfer) {
	if len(ts) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transfers = append(ts, s.transfers...)
	if len(s.transfers) > maxPendingTransfers {
		s.transfers = s.transfers[len(s.transfers)-maxPendingTransfers:]
	}
}

// buildReport dựng nội dung report từ danh sách game của quán. Version là bản đã nằm trọn trên ổ (không phải bản
// master đang có), đúng điều Hub cần để biết quán tụt bản hay chưa.
func buildReport(games []model.Game, machineCount int) reportBody {
	out := reportBody{CoreVersion: CoreVersion, MachineCount: machineCount, Games: make([]reportGame, 0, len(games))}
	for _, g := range games {
		if g.Origin == model.GameOriginLocal {
			continue // game cục bộ là việc riêng của quán, Hub không cần biết
		}
		out.Games = append(out.Games, reportGame{
			Name: g.Name, Version: g.Version, Status: g.Status, Progress: g.Progress, Error: truncateRunes(g.Error, 500),
		})
	}
	return out
}

// reportToHub gửi report. Lỗi chỉ ghi log một lần mỗi loại và không bao giờ chặn việc tải game.
func (s *GameService) reportToHub(ctx context.Context) {
	s.mu.Lock()
	skip := s.noReport
	s.mu.Unlock()
	c := s.conf()
	if skip || c.Role != "cafe" || c.UpstreamURL == "" {
		return
	}
	games, err := s.listGames()
	if err != nil {
		return
	}
	var machines int64
	s.db.Model(&model.Machine{}).Count(&machines)
	rep := buildReport(games, int(machines))
	rep.Transfers = s.takeTransfers()
	sent := false
	defer func() {
		if !sent { // gửi không thành (mạng, Hub tạm lỗi, 401): giữ lại cho lần sau
			s.putBackTransfers(rep.Transfers)
		}
	}()
	raw, err := json.Marshal(rep)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.UpstreamURL+"/api/game-catalog/report", bytes.NewReader(raw))
	if err != nil {
		return
	}
	req.Header.Set(GameCatalogKeyHeader, c.CatalogKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed:
		s.mu.Lock()
		s.noReport = true
		s.mu.Unlock()
		log.Printf("[game] upstream không có /api/game-catalog/report (master cũ), quán thôi báo trạng thái")
	case resp.StatusCode == http.StatusOK:
		sent = true
		var env struct {
			Code int `json:"code"`
			Data struct {
				Key *hubKeyInfo `json:"key"`
			} `json:"data"`
		}
		if json.Unmarshal(body, &env) == nil && env.Code == 0 && env.Data.Key != nil {
			s.setKeyInfo(env.Data.Key)
		}
	default: // 401: key sai hoặc bị thu hồi; lỗi đó đã hiện ở "sync_error", tình trạng key cũ không còn đúng nữa
		s.setKeyInfo(nil)
	}
}

func (s *GameService) setKeyInfo(k *hubKeyInfo) {
	s.mu.Lock()
	changed := (s.keyInfo == nil) != (k == nil) || (k != nil && s.keyInfo != nil && *s.keyInfo != *k)
	s.keyInfo = k
	s.mu.Unlock()
	if changed {
		s.baoDoi("")
	}
}

// ---------------------------------------------------------------- webseed do catalog chỉ định

// webseedsFor là nơi tải dữ liệu của game: do catalog chỉ định (Hub: dữ liệu nằm ở máy master khác máy catalog), không
// có thì suy ra từ địa chỉ upstream như master cũ.
func (s *GameService) webseedsFor(g model.Game) []string {
	if len(g.RemoteWebseeds) > 0 {
		return g.RemoteWebseeds
	}
	if len(g.RemoteTrackers) > 0 || len(g.RemoteRelays) > 0 {
		return nil // Hub: master không có địa chỉ công khai, chỉ phát qua P2P; upstream (Hub) không có /seed/
	}
	return []string{s.conf().UpstreamURL + "/seed/" + s.SeedToken() + "/"}
}

// webseedsHopLe giữ lại các webseed dùng được. Catalog là dữ liệu từ xa: máy chủ quán sẽ gọi các địa chỉ này, nên chỉ
// nhận http(s) có tên máy, không tài khoản, và địa chỉ IP thẳng không được là địa chỉ nội bộ của chính máy này.
func webseedsHopLe(game string, in []string) []string {
	var out []string
	for _, raw := range in {
		u, err := url.Parse(raw)
		bad := err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil ||
			u.RawQuery != "" || u.Fragment != "" || !strings.HasSuffix(u.Path, "/")
		if !bad {
			if ip := net.ParseIP(u.Hostname()); ip != nil && blockedHubIP(ip) {
				bad = true
			}
		}
		if bad {
			log.Printf("[game] %s: bỏ webseed không hợp lệ từ catalog: %q", game, raw)
			continue
		}
		out = append(out, raw)
	}
	return out
}

// relaysHopLe giữ lại các địa chỉ relay dùng được: cùng luật với webseed (http(s), tên máy, không tài khoản, không IP nội bộ).
func relaysHopLe(game string, in []string) []string { return webseedsHopLe(game+" (relay)", in) }

// trackersHopLe giữ lại các tracker dùng được: máy chủ quán sẽ mở kết nối tới đây, nên cũng chỉ nhận ws(s) có tên máy,
// không tài khoản, và địa chỉ IP thẳng không được là địa chỉ nội bộ. Khác webseed: token nằm ở đường dẫn, không cần "/" cuối.
func trackersHopLe(game string, in []string) []string {
	var out []string
	for _, raw := range in {
		u, err := url.Parse(raw)
		bad := err != nil || (u.Scheme != "ws" && u.Scheme != "wss") || u.Host == "" || u.User != nil ||
			u.RawQuery != "" || u.Fragment != ""
		if !bad {
			if ip := net.ParseIP(u.Hostname()); ip != nil && blockedHubIP(ip) {
				bad = true
			}
		}
		if bad {
			log.Printf("[game] %s: bỏ tracker không hợp lệ từ catalog", game) // URL mang token: không ghi ra log
			continue
		}
		out = append(out, raw)
	}
	return out
}

// splitList tách danh sách ngăn bằng dấu phẩy, bỏ phần tử trống.
func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ---------------------------------------------------------------- kênh WebSocket

const (
	eventsBackoffMin    = 5 * time.Second
	eventsBackoffMax    = 10 * time.Minute
	eventsReadTimeout   = 90 * time.Second
	eventsUnsupportedBO = 30 * time.Minute
)

// wsURL đổi http(s) thành ws(s).
func wsURL(base string) string {
	switch {
	case strings.HasPrefix(base, "https://"):
		return "wss://" + strings.TrimPrefix(base, "https://")
	case strings.HasPrefix(base, "http://"):
		return "ws://" + strings.TrimPrefix(base, "http://")
	}
	return base
}

func (s *GameService) eventsDialer() *websocket.Dialer {
	if s.eventsDial != nil {
		return s.eventsDial
	}
	// Cùng rào với mọi lời gọi tới Hub: không nối tới địa chỉ nội bộ của chính máy chủ.
	d := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second, Control: hubDialControl}
	return &websocket.Dialer{NetDialContext: d.DialContext, HandshakeTimeout: 15 * time.Second}
}

// eventsLoop giữ kênh WebSocket tới Hub cho tới khi ctx kết thúc, nối lại với thời gian chờ tăng dần. Chỉ là đường tắt
// để biết sớm: mất kênh thì quán vẫn đọc catalog định kỳ.
func (s *GameService) eventsLoop(ctx context.Context) {
	backoff := eventsBackoffMin
	for ctx.Err() == nil {
		s.mu.Lock()
		skip := s.noEvents
		s.mu.Unlock()
		if skip {
			return
		}
		held, err := s.eventsOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		if held {
			backoff = eventsBackoffMin // đã nối được rồi mới rớt: thử lại sớm
		}
		wait := backoff
		var unsupported *eventsUnsupported
		if errors.As(err, &unsupported) {
			s.mu.Lock()
			s.noEvents = true
			s.mu.Unlock()
			log.Printf("[game] upstream không có kênh WebSocket (master cũ), quán đọc catalog theo chu kỳ")
			return
		}
		var rejected *eventsRejected
		if errors.As(err, &rejected) {
			// Hub từ chối key (thu hồi, hết hạn): đọc catalog ngay để trang Game hiện lý do, không đợi chu kỳ.
			s.Kick()
		}
		if err != nil && backoff == eventsBackoffMin {
			log.Printf("[game] kênh WebSocket tới Hub: %v (thử lại sau %s)", err, wait)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		backoff = min(backoff*2, eventsBackoffMax)
	}
}

type eventsUnsupported struct{ status int }

func (e *eventsUnsupported) Error() string { return "upstream không có kênh sự kiện" }

type eventsRejected struct{ status int }

func (e *eventsRejected) Error() string { return "Hub từ chối key của kênh sự kiện" }

type eventMessage struct {
	Event string `json:"event"`
	Data  struct {
		Name    string      `json:"name"`
		Version int         `json:"version"`
		Key     *hubKeyInfo `json:"key"`
	} `json:"data"`
}

// eventsOnce nối một lần và đọc cho tới khi kênh đóng. held cho biết đã nối được hay chưa.
func (s *GameService) eventsOnce(ctx context.Context) (held bool, err error) {
	c := s.conf()
	hdr := http.Header{}
	hdr.Set(GameCatalogKeyHeader, c.CatalogKey)
	conn, resp, err := s.eventsDialer().DialContext(ctx, wsURL(c.UpstreamURL)+"/api/game-catalog/events", hdr)
	if err != nil {
		if resp != nil {
			switch resp.StatusCode {
			case http.StatusNotFound, http.StatusMethodNotAllowed:
				return false, &eventsUnsupported{status: resp.StatusCode}
			case http.StatusUnauthorized, http.StatusForbidden:
				return false, &eventsRejected{status: resp.StatusCode}
			}
		}
		return false, err
	}
	defer conn.Close()
	s.setEventsUp(true)
	defer s.setEventsUp(false)

	go func() { // ctx kết thúc thì đóng kết nối để vòng đọc thoát
		<-ctx.Done()
		_ = conn.Close()
	}()
	_ = conn.SetReadDeadline(time.Now().Add(eventsReadTimeout))
	conn.SetPingHandler(func(data string) error { // Hub ping mỗi 30 giây: còn ping là còn sống
		_ = conn.SetReadDeadline(time.Now().Add(eventsReadTimeout))
		return conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(5*time.Second))
	})
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return true, err
		}
		_ = conn.SetReadDeadline(time.Now().Add(eventsReadTimeout))
		var m eventMessage
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		switch m.Event {
		case "game.catalog.changed":
			s.Kick()
		case "hello":
			if m.Data.Key != nil {
				s.setKeyInfo(m.Data.Key)
			}
		}
	}
}

func (s *GameService) setEventsUp(up bool) {
	s.mu.Lock()
	changed := s.eventsUp != up
	s.eventsUp = up
	s.mu.Unlock()
	if changed {
		s.baoDoi("")
	}
}
