package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/gameupdate"
	"github.com/vnet/core/internal/model"
)

func TestWebseedsHopLe(t *testing.T) {
	good := []string{
		"https://master.example.com/seed/tok/",
		"http://192.168.1.10:19980/seed/tok/",
		"http://master.local:9000/seed/a.1.b/",
	}
	assert.Equal(t, good, webseedsHopLe("g", good))

	bad := []string{
		"", "ftp://x/seed/", "javascript:alert(1)", "file:///etc/passwd", "http://", "//x/seed/",
		"http://user:pass@x/seed/", "http://x/seed/?a=1", "http://x/seed/#f", "http://x/seed/tok", // thiếu "/" cuối
		"http://127.0.0.1:8080/seed/tok/", "http://[::1]:8080/seed/tok/", "http://169.254.169.254/seed/tok/", "http://0.0.0.0/seed/",
	}
	for _, u := range bad {
		assert.Empty(t, webseedsHopLe("g", []string{u}), u)
	}
	assert.Equal(t, []string{good[0]}, webseedsHopLe("g", []string{bad[2], good[0], bad[12]}), "giữ cái tốt, bỏ cái xấu")
	assert.Nil(t, webseedsHopLe("g", nil))
}

func TestWebseedsFor(t *testing.T) {
	svc := NewGameService(nil, config.GameConfig{Role: config.GameRoleCafe, UpstreamURL: "http://master:20800", CatalogKey: "khoa-catalog-khoa-catalog"})
	// catalog chỉ định: dùng đúng
	got := svc.webseedsFor(model.Game{RemoteWebseeds: model.StringArray{"https://m.example.com/seed/tok/"}})
	assert.Equal(t, []string{"https://m.example.com/seed/tok/"}, got)
	// master cũ không chỉ định: suy ra từ địa chỉ upstream như trước
	got = svc.webseedsFor(model.Game{})
	require.Len(t, got, 1)
	assert.Equal(t, "http://master:20800/seed/"+svc.SeedToken()+"/", got[0])
}

func TestWebseedsFor_HubKhongCoWebseed(t *testing.T) {
	svc := NewGameService(nil, config.GameConfig{Role: config.GameRoleCafe, UpstreamURL: "https://hub.example.com", CatalogKey: "khoa-catalog-khoa-catalog"})
	// Hub có tracker nhưng master không có địa chỉ công khai: chỉ P2P, tuyệt đối không suy webseed từ địa chỉ Hub
	// (Hub không phục vụ /seed/).
	got := svc.webseedsFor(model.Game{RemoteTrackers: model.StringArray{"wss://hub.example.com/announce/tok"}})
	assert.Empty(t, got)
	// có cả hai: webseed là đường dự phòng của P2P
	got = svc.webseedsFor(model.Game{
		RemoteTrackers: model.StringArray{"wss://hub.example.com/announce/tok"},
		RemoteWebseeds: model.StringArray{"https://m.example.com/seed/tok/"},
	})
	assert.Equal(t, []string{"https://m.example.com/seed/tok/"}, got)
}

func TestRelaysHopLe(t *testing.T) {
	good := []string{"https://vnet-hub.example.com/relay/tok/", "http://192.168.1.10:8090/relay/tok/"}
	assert.Equal(t, good, relaysHopLe("g", good))
	for _, u := range []string{"", "ftp://x/relay/", "wss://hub/relay/tok/", "http://127.0.0.1:8090/relay/tok/", "https://hub/relay/tok", "https://u:p@hub/relay/tok/", "https://hub/relay/tok/?a=1"} {
		assert.Empty(t, relaysHopLe("g", []string{u}), u)
	}
	assert.Nil(t, relaysHopLe("g", nil))
}

func TestWebseedsFor_HubCoRelayKhongSuyWebseedTuUpstream(t *testing.T) {
	svc := NewGameService(nil, config.GameConfig{Role: config.GameRoleCafe, UpstreamURL: "https://hub.example.com", CatalogKey: "khoa-catalog-khoa-catalog"})
	assert.Empty(t, svc.webseedsFor(model.Game{RemoteRelays: model.StringArray{"https://hub.example.com/relay/tok/"}}),
		"có relay là catalog của Hub: webseed không được suy từ địa chỉ Hub")
}

func TestCatalogItem_JSON_RelayWebseeds(t *testing.T) {
	raw, err := json.Marshal(CatalogItem{Name: "a"})
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "relay_webseeds")
	var back CatalogItem
	require.NoError(t, json.Unmarshal([]byte(`{"name":"a","relay_webseeds":["https://h/relay/t/"]}`), &back))
	assert.Equal(t, []string{"https://h/relay/t/"}, back.RelayWebseeds)
}

func TestTransfers_GomRoiGuiMotLanVaGiuKhiGuiHong(t *testing.T) {
	svc := NewGameService(nil, config.GameConfig{Role: config.GameRoleCafe, UpstreamURL: "http://x", CatalogKey: "k"})
	svc.noteTransfer("lol", 3, gameupdate.Transfer{})
	assert.Empty(t, svc.takeTransfers(), "lần cài không nhận byte nào không đáng báo")

	svc.noteTransfer("lol", 3, gameupdate.Transfer{P2P: 100, Webseed: 5, Relay: 7})
	svc.noteTransfer("cs2", 1, gameupdate.Transfer{Relay: 9})
	got := svc.takeTransfers()
	assert.Equal(t, []reportTransfer{
		{Game: "lol", Version: 3, P2PBytes: 100, WebseedBytes: 5, RelayBytes: 7},
		{Game: "cs2", Version: 1, RelayBytes: 9},
	}, got)
	assert.Empty(t, svc.takeTransfers(), "lấy rồi là hết")

	svc.noteTransfer("moi", 1, gameupdate.Transfer{P2P: 1})
	svc.putBackTransfers(got) // gửi hỏng: trả về, đứng trước mục mới
	again := svc.takeTransfers()
	require.Len(t, again, 3)
	assert.Equal(t, "lol", again[0].Game)
	assert.Equal(t, "moi", again[2].Game)

	for i := 0; i < maxPendingTransfers+50; i++ {
		svc.noteTransfer("g", i, gameupdate.Transfer{P2P: 1})
	}
	capped := svc.takeTransfers()
	require.Len(t, capped, maxPendingTransfers)
	assert.Equal(t, 50, capped[0].Version, "bỏ mục cũ nhất")

	svc.mu.Lock()
	svc.noReport = true
	svc.mu.Unlock()
	svc.noteTransfer("g", 1, gameupdate.Transfer{P2P: 1})
	assert.Empty(t, svc.takeTransfers(), "upstream không có report thì không dồn mục")
}

func TestReportToHub_GuiTransfersDungMotLan(t *testing.T) {
	var bodies []reportBody
	fail := true
	hub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b reportBody
		_ = json.NewDecoder(r.Body).Decode(&b)
		bodies = append(bodies, b)
		if fail {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"key":{"status":"active","expires_at":"2027-01-01T00:00:00Z","grace_until":"2027-01-04T00:00:00Z"}}}`))
	}))
	defer hub.Close()
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe, UpstreamURL: hub.URL, CatalogKey: "k"})
	svc.http = http.DefaultClient
	svc.noteTransfer("lol", 2, gameupdate.Transfer{P2P: 100, Relay: 50})

	expectReportQueries(mock)
	svc.reportToHub(context.Background()) // Hub 502: chưa tính là đã gửi
	fail = false
	expectReportQueries(mock)
	svc.reportToHub(context.Background())
	expectReportQueries(mock)
	svc.reportToHub(context.Background())

	require.Len(t, bodies, 3)
	want := []reportTransfer{{Game: "lol", Version: 2, P2PBytes: 100, RelayBytes: 50}}
	assert.Equal(t, want, bodies[0].Transfers)
	assert.Equal(t, want, bodies[1].Transfers, "gửi lại vì lần đầu hỏng")
	assert.Empty(t, bodies[2].Transfers, "đã gửi thành công thì không gửi nữa")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTrackersHopLe(t *testing.T) {
	good := []string{
		"wss://vnet-hub.example.com/announce/t0123.1790000000.abcd",
		"ws://192.168.1.10:8090/announce/a1.2.3",
		"ws://hub.local:8090/announce/tok",
	}
	assert.Equal(t, good, trackersHopLe("g", good))

	bad := []string{
		"", "http://hub/announce/x", "https://hub/announce/x", "udp://hub:80/announce", "ftp://x/", "javascript:alert(1)",
		"ws://", "//hub/announce/x", "ws://user:pass@hub/announce/x", "ws://hub/announce/x?a=1", "ws://hub/announce/x#f",
		"ws://127.0.0.1:8090/announce/x", "wss://[::1]/announce/x", "ws://169.254.169.254/announce/x", "ws://0.0.0.0/announce/x",
	}
	for _, u := range bad {
		assert.Empty(t, trackersHopLe("g", []string{u}), u)
	}
	assert.Equal(t, []string{good[0]}, trackersHopLe("g", []string{bad[1], good[0], bad[11]}), "giữ cái tốt, bỏ cái xấu")
	assert.Nil(t, trackersHopLe("g", nil))
}

func TestSplitList(t *testing.T) {
	assert.Equal(t, []string{"stun:a:1", "turn:b:2"}, splitList(" stun:a:1, ,turn:b:2 ,"))
	assert.Nil(t, splitList(""))
}

func TestCatalogItem_JSON_Trackers(t *testing.T) {
	raw, err := json.Marshal(CatalogItem{Name: "a"})
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "trackers", "catalog của master cũ giữ nguyên như trước")
	var back CatalogItem
	require.NoError(t, json.Unmarshal([]byte(`{"name":"a","trackers":["wss://h/announce/t"]}`), &back))
	assert.Equal(t, []string{"wss://h/announce/t"}, back.Trackers)
}

func TestCatalogItem_JSON_Webseeds(t *testing.T) {
	raw, err := json.Marshal(CatalogItem{Name: "a"})
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "webseeds", "catalog của master cũ giữ nguyên như trước")
	raw, err = json.Marshal(CatalogItem{Name: "a", Webseeds: []string{"http://x/seed/t/"}})
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"webseeds":["http://x/seed/t/"]`)
	var back CatalogItem
	require.NoError(t, json.Unmarshal([]byte(`{"name":"a","webseeds":["http://x/seed/t/"]}`), &back))
	assert.Equal(t, []string{"http://x/seed/t/"}, back.Webseeds)
}

func TestBuildReport(t *testing.T) {
	CoreVersion = "1.2.3"
	defer func() { CoreVersion = "dev" }()
	r := buildReport([]model.Game{
		{Name: "a", Version: 3, Status: "ready", Progress: 100},
		{Name: "b", Version: 1, Status: "error", Error: strings.Repeat("ế", 900)},
	}, 24)
	assert.Equal(t, "1.2.3", r.CoreVersion)
	assert.Equal(t, 24, r.MachineCount)
	require.Len(t, r.Games, 2)
	assert.Equal(t, reportGame{Name: "a", Version: 3, Status: "ready", Progress: 100}, r.Games[0])
	assert.Equal(t, 500, len([]rune(r.Games[1].Error)), "cắt theo ký tự, không theo byte")
	assert.NotNil(t, buildReport(nil, 0).Games, "danh sách rỗng là [] chứ không phải null")
}

func expectReportQueries(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`SELECT \* FROM "games" ORDER BY priority DESC, display_name ASC`).
		WillReturnRows(sqlmock.NewRows([]string{"name", "version", "status", "progress"}).AddRow("lol", 2, "ready", 100))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "machines"`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(18))
}

func TestReportToHub_NhanTinhTrangKey(t *testing.T) {
	var gotKey string
	var gotBody reportBody
	hub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/game-catalog/report", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		gotKey = r.Header.Get(GameCatalogKeyHeader)
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"key":{"status":"grace","expires_at":"2026-10-20T00:00:00Z","grace_until":"2026-10-23T00:00:00Z"}}}`))
	}))
	defer hub.Close()

	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe, UpstreamURL: hub.URL, CatalogKey: "VGK-AAAAA-BBBBB-CCCCC-DDDDD"})
	svc.http = http.DefaultClient
	expectReportQueries(mock)

	svc.reportToHub(context.Background())

	assert.Equal(t, "VGK-AAAAA-BBBBB-CCCCC-DDDDD", gotKey)
	assert.Equal(t, 18, gotBody.MachineCount)
	require.Len(t, gotBody.Games, 1)
	assert.Equal(t, "lol", gotBody.Games[0].Name)
	assert.Equal(t, 2, gotBody.Games[0].Version)
	assert.NoError(t, mock.ExpectationsWereMet())

	svc.mu.Lock()
	k := svc.keyInfo
	svc.mu.Unlock()
	require.NotNil(t, k)
	assert.Equal(t, "grace", k.Status)
	assert.Equal(t, time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), k.ExpiresAt.UTC())
}

func TestReportToHub_MasterCuKhongCoReportThiThoiGoi(t *testing.T) {
	calls := 0
	master := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":404,"message":"endpoint not found"}`))
	}))
	defer master.Close()
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe, UpstreamURL: master.URL, CatalogKey: "k"})
	svc.http = http.DefaultClient
	expectReportQueries(mock)

	svc.reportToHub(context.Background())
	svc.reportToHub(context.Background()) // lần hai không còn gọi, cũng không chạm DB
	assert.Equal(t, 1, calls, "thôi gọi sau khi biết upstream không có report")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReportToHub_KeyBiTuChoiXoaTinhTrangCu(t *testing.T) {
	hub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":401,"message":"key đã bị thu hồi","data":{"reason":"key_revoked"}}`))
	}))
	defer hub.Close()
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe, UpstreamURL: hub.URL, CatalogKey: "k"})
	svc.http = http.DefaultClient
	svc.keyInfo = &hubKeyInfo{Status: "active"}
	expectReportQueries(mock)

	svc.reportToHub(context.Background())
	svc.mu.Lock()
	defer svc.mu.Unlock()
	assert.Nil(t, svc.keyInfo, "tình trạng key cũ không còn đúng")
	assert.False(t, svc.noReport, "401 không phải 'upstream không có report'")
}

func TestReportToHub_KhongPhaiQuanThiKhongGoi(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleMaster})
	svc.reportToHub(context.Background())
	svc = NewGameService(db, config.GameConfig{})
	svc.reportToHub(context.Background())
	assert.NoError(t, mock.ExpectationsWereMet(), "master và máy chưa có vai không chạm DB hay mạng")
}

func TestWsURL(t *testing.T) {
	assert.Equal(t, "ws://hub:8090", wsURL("http://hub:8090"))
	assert.Equal(t, "wss://hub.example.com", wsURL("https://hub.example.com"))
	assert.Equal(t, "ftp://x", wsURL("ftp://x"))
}

// wsHub dựng một Hub giả có kênh /api/game-catalog/events; send là tin sẽ đẩy xuống ngay khi quán nối.
func wsHub(t *testing.T, send ...string) (*httptest.Server, *int) {
	t.Helper()
	conns := new(int)
	up := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/game-catalog/events" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get(GameCatalogKeyHeader) != "VGK-AAAAA-BBBBB-CCCCC-DDDDD" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		*conns++
		defer c.Close()
		for _, m := range send {
			_ = c.WriteMessage(websocket.TextMessage, []byte(m))
		}
		for { // giữ kết nối tới khi quán đóng
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}))
	t.Cleanup(srv.Close)
	return srv, conns
}

func newEventsSvc(t *testing.T, upstream string) *GameService {
	svc := NewGameService(nil, config.GameConfig{Role: config.GameRoleCafe, UpstreamURL: upstream, CatalogKey: "VGK-AAAAA-BBBBB-CCCCC-DDDDD"})
	svc.eventsDial = websocket.DefaultDialer // Hub giả nằm trên loopback, bộ nối thật của dịch vụ chặn loopback
	return svc
}

func TestEvents_BanMoiThiDanhThucVongNen(t *testing.T) {
	hub, _ := wsHub(t,
		`{"event":"hello","data":{"key":{"status":"active","expires_at":"2027-01-01T00:00:00Z","grace_until":"2027-01-04T00:00:00Z"}}}`,
		`{"event":"game.catalog.changed","data":{"name":"lol","version":4}}`)
	svc := newEventsSvc(t, hub.URL)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); _, _ = svc.eventsOnce(ctx) }()

	select {
	case <-svc.kick:
	case <-time.After(3 * time.Second):
		t.Fatal("Hub báo có bản mới mà vòng nền không được đánh thức")
	}
	assert.Eventually(t, func() bool {
		svc.mu.Lock()
		defer svc.mu.Unlock()
		return svc.keyInfo != nil && svc.keyInfo.Status == "active" && svc.eventsUp
	}, 3*time.Second, 10*time.Millisecond, "hello mang tình trạng key, và kênh được ghi nhận là đang nối")

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("ctx kết thúc mà vòng đọc không thoát")
	}
	svc.mu.Lock()
	assert.False(t, svc.eventsUp, "đóng kênh thì không còn báo đang nối")
	svc.mu.Unlock()
}

func TestEvents_BoQuaTinLa(t *testing.T) {
	hub, _ := wsHub(t, `không phải JSON`, `{"event":"gi-do-la"}`, `{"event":"game.catalog.changed","data":{"name":"x","version":1}}`)
	svc := newEventsSvc(t, hub.URL)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _, _ = svc.eventsOnce(ctx) }()
	select {
	case <-svc.kick:
	case <-time.After(3 * time.Second):
		t.Fatal("tin rác làm vòng đọc dừng")
	}
}

func TestEvents_MasterCuKhongCoKenhThiThoiNoi(t *testing.T) {
	old := httptest.NewServer(http.NotFoundHandler())
	defer old.Close()
	svc := newEventsSvc(t, old.URL)
	done := make(chan struct{})
	go func() { defer close(done); svc.eventsLoop(context.Background()) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("upstream trả 404 mà vòng nối lại vẫn quay")
	}
	svc.mu.Lock()
	assert.True(t, svc.noEvents)
	svc.mu.Unlock()
}

func TestEvents_SaiKeyKhongPhaiMasterCu(t *testing.T) {
	hub, _ := wsHub(t)
	svc := newEventsSvc(t, hub.URL)
	svc.cfg.CatalogKey = "VGK-SAISA-SAISA-SAISA-SAISA"
	held, err := svc.eventsOnce(context.Background())
	assert.False(t, held)
	require.Error(t, err)
	var unsupported *eventsUnsupported
	assert.NotErrorAs(t, err, &unsupported, "401 là key sai, không phải 'upstream không có kênh'")
	var rejected *eventsRejected
	assert.ErrorAs(t, err, &rejected)
}

// Hub đá kênh vì key bị thu hồi: nối lại bị từ chối thì quán phải đọc catalog ngay để trang Game hiện lý do.
func TestEvents_BiTuChoiThiDanhThucDocCatalog(t *testing.T) {
	hub, _ := wsHub(t)
	svc := newEventsSvc(t, hub.URL)
	svc.cfg.CatalogKey = "VGK-SAISA-SAISA-SAISA-SAISA"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go svc.eventsLoop(ctx)
	select {
	case <-svc.kick:
	case <-time.After(3 * time.Second):
		t.Fatal("bị Hub từ chối key mà quán không đọc lại catalog")
	}
}

func TestEvents_BoNoiChanDiaChiNoiBo(t *testing.T) {
	hub, _ := wsHub(t)
	svc := NewGameService(nil, config.GameConfig{Role: config.GameRoleCafe, UpstreamURL: hub.URL, CatalogKey: "VGK-AAAAA-BBBBB-CCCCC-DDDDD"})
	// không gán eventsDial: dùng bộ nối thật, phải từ chối loopback
	_, err := svc.eventsOnce(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "địa chỉ nội bộ")
}
