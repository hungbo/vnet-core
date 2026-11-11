package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/model"
	"gorm.io/gorm"
)

func TestRateMeter(t *testing.T) {
	var m rateMeter
	t0 := time.Unix(1000, 0)
	assert.Zero(t, m.sample(t0, 0), "lần đo đầu chưa có gì để so")
	// Chưa đủ một giây: giữ số cũ, không chia cho khoảng thời gian quá nhỏ.
	assert.Zero(t, m.sample(t0.Add(300*time.Millisecond), 1_000_000))
	got := m.sample(t0.Add(2*time.Second), 20_000_000)
	assert.Equal(t, int64(10_000_000), got, "20 MB trong 2 giây là 10 MB/s")
	// Bộ đếm về 0 (lượt tải mới): đo lại từ đầu, không ra số âm.
	assert.Zero(t, m.sample(t0.Add(3*time.Second), 100))
}

func TestGameLimits(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe})

	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1`).WithArgs("games").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}).
			AddRow("games", GameDownloadLimitKey, `"200"`).AddRow("games", GameUploadLimitKey, `"-5"`))
	l := svc.Limits()
	assert.Equal(t, int64(200), l.DownloadMbps)
	assert.Zero(t, l.UploadMbps, "số âm không phải trần")
	assert.NoError(t, mock.ExpectationsWereMet())

	_, err := svc.SetLimits(GameLimits{DownloadMbps: -1})
	assert.Error(t, err)
	_, err = svc.SetLimits(GameLimits{UploadMbps: maxLimitMbps + 1})
	assert.Error(t, err)
}

func TestGameQueue_TrangThai(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe})
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.Local)

	cols := []string{"id", "name", "display_name", "priority", "status", "auto_update", "infohash", "remote_infohash", "progress", "error", "size_bytes", "install_root"}
	mock.ExpectQuery(`SELECT \* FROM "games" WHERE enabled = \$1 AND origin = \$2 AND remote_infohash <> ''`).
		WillReturnRows(sqlmock.NewRows(cols).
			AddRow("1", "dang-tai", "Đang tải", 0, model.GameStatusDownloading, true, "a", "b", 40, "", 100, "").
			AddRow("2", "bam-tai-ngay", "Bấm tải ngay", 5, model.GameStatusQueued, false, "a", "b", 0, "", 100, "").
			AddRow("3", "cho-khung-gio", "Chờ khung giờ", 1, model.GameStatusNotInstalled, true, "", "b", 0, "", 100, `E:\Games`).
			AddRow("7", "chua-chon", "Quán chưa bấm Tải", 1, model.GameStatusNotInstalled, true, "", "b", 0, "", 100, "").
			AddRow("4", "tat-tu-cap-nhat", "Tắt tự cập nhật", 1, model.GameStatusReady, false, "a", "b", 100, "", 100, "").
			AddRow("5", "da-moi-nhat", "Đã mới nhất", 9, model.GameStatusReady, true, "b", "b", 100, "", 100, "").
			AddRow("6", "loi-cho-thu-lai", "Lỗi, chờ thử lại", 2, model.GameStatusError, true, "b", "b", 0, "treo", 100, ""))
	// Khung giờ 02:00-06:00: bây giờ là 12:00 nên đang ngoài khung.
	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1`).WithArgs("games").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}).AddRow("games", GameUpdateWindowKey, `"02:00-06:00"`))
	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1`).WithArgs("games").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}))

	retryAt := now.Add(10 * time.Minute)
	svc.retry["loi-cho-thu-lai"] = &retryInfo{infohash: "b", fails: 1, notBefore: retryAt}

	q, err := svc.Queue(now)
	require.NoError(t, err)
	require.NotNil(t, q.Current)
	assert.Equal(t, "dang-tai", q.Current.Name)
	assert.Equal(t, 40, q.Current.Progress)
	assert.False(t, q.InWindow)

	got := map[string]string{}
	var order []string
	for _, w := range q.Waiting {
		got[w.Name] = w.State
		order = append(order, w.Name)
	}
	assert.Equal(t, map[string]string{
		"bam-tai-ngay":    QueueNext,   // người quản trị bấm: chạy bất kể khung giờ
		"cho-khung-gio":   QueueWindow, // đã bấm Tải (có thư mục lưu) nhưng đang ngoài khung giờ
		"tat-tu-cap-nhat": QueueManual, // có bản mới, tắt tự cập nhật: phải bấm
		"loi-cho-thu-lai": QueueRetry,
	}, got)
	assert.NotContains(t, got, "da-moi-nhat", "game đã đúng bản master không nằm trong hàng đợi")
	assert.NotContains(t, got, "chua-chon", "game Hub quán chưa bấm Tải không chờ gì cả")
	assert.Equal(t, "bam-tai-ngay", order[0], "ưu tiên cao xếp trước")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// hubGiaChoDanhMuc dựng một Hub giả cho các test của trang Danh mục Hub.
func hubGiaChoDanhMuc(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *GameService {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(srv.Close)
	svc := NewGameService(nil, config.GameConfig{Role: config.GameRoleCafe, UpstreamURL: srv.URL, CatalogKey: "khoa-hub-khoa-hub-khoa-hub"})
	svc.http = http.DefaultClient // Hub giả nằm trên loopback, client thật của dịch vụ chặn loopback
	return svc
}

func TestHubCatalogError(t *testing.T) {
	assert.NoError(t, hubCatalogError(nil))
	assert.ErrorIs(t, hubCatalogError(assert.AnError), assert.AnError)
	assert.ErrorIs(t, hubCatalogError(errors.New("/api/game-catalog/all trả 404: not found")), ErrHubSelectionUnsupported)
	assert.ErrorIs(t, hubCatalogError(errors.New("invalid character '<' looking for beginning of value")), ErrHubSelectionUnsupported)
}

func TestHubCatalog_HubCuTraTrangWeb(t *testing.T) {
	svc := hubGiaChoDanhMuc(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<!doctype html><html></html>"))
	})
	_, err := svc.HubCatalog(context.Background())
	assert.ErrorIs(t, err, ErrHubSelectionUnsupported, "Hub cũ trả trang web thay vì JSON")
}

func TestHubCatalog_GhepVoiGameODQuan(t *testing.T) {
	db, mock := newMockDB(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/game-catalog/all", r.URL.Path)
		assert.Equal(t, "khoa-hub-khoa-hub-khoa-hub", r.Header.Get(GameCatalogKeyHeader))
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 0, "data": map[string]interface{}{
			"mode": "selected", "selected_count": 1,
			"items": []map[string]interface{}{
				{"name": "A", "display_name": "Game A", "available": true, "version": 3, "selected": true},
				{"name": "B", "display_name": "Game B", "available": false, "selected": false},
				{"name": "TuLam", "display_name": "Trùng tên", "available": true, "version": 1, "selected": true},
			},
		}})
	}))
	defer srv.Close()
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe, UpstreamURL: srv.URL, CatalogKey: "khoa-hub-khoa-hub-khoa-hub"})
	svc.http = http.DefaultClient

	mock.ExpectQuery(`SELECT \* FROM "games" ORDER BY priority DESC, display_name ASC`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "status", "progress", "version", "infohash", "remote_infohash", "auto_update", "origin", "install_root"}).
			AddRow("g1", "A", model.GameStatusReady, 100, 2, "old", "new", true, model.GameOriginHub, `E:\Games`).
			AddRow("g9", "TuLam", model.GameStatusReady, 100, 1, "x", "", true, model.GameOriginLocal, ""))

	v, err := svc.HubCatalog(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "selected", v.Mode)
	require.Len(t, v.Items, 3)
	require.NotNil(t, v.Items[0].Local)
	assert.True(t, v.Items[0].Local.NeedsUpdate, "bản trên ổ khác bản master")
	assert.Equal(t, 2, v.Items[0].Local.Version)
	assert.True(t, v.Items[0].Local.Chosen, "đã có bản trên ổ")
	assert.Equal(t, `E:\Games`, v.Items[0].Local.InstallRoot)
	assert.Nil(t, v.Items[1].Local, "game Hub chưa có bản ghi ở quán")
	assert.True(t, v.Items[2].LocalGame, "trùng tên game cục bộ")
	assert.Nil(t, v.Items[2].Local, "game cục bộ không phải tình trạng của game Hub")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Chọn game: lựa chọn đi qua Hub, game bỏ chọn thôi tự cập nhật, game chọn được xếp hàng, và catalog đọc lại.
func TestSelectHubGames(t *testing.T) {
	db, mock := newMockDB(t)
	var selection map[string][]string
	pulled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/api/game-catalog/selection":
			var body struct {
				Mode   string   `json:"mode"`
				Add    []string `json:"add"`
				Remove []string `json:"remove"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			selection = map[string][]string{"add": body.Add, "remove": body.Remove}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 0, "data": map[string]interface{}{"mode": "selected", "selected_count": 2, "unknown": []string{"khong-co"}}})
		case r.URL.Path == "/api/game-catalog":
			pulled = true
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 0, "data": []CatalogItem{}})
		default:
			t.Errorf("gọi lạ: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe, UpstreamURL: srv.URL, CatalogKey: "khoa-hub-khoa-hub-khoa-hub"})
	svc.http = http.DefaultClient

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "games" SET "auto_update"=\$1,"updated_at"=\$2 WHERE name IN \(\$3\) AND origin = \$4`).
		WithArgs(false, anyTime{}, "Cu", model.GameOriginHub).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "games" SET "auto_update"=\$1,"updated_at"=\$2 WHERE name IN \(\$3,\$4\) AND origin = \$5`).
		WithArgs(true, anyTime{}, "Moi", "khong-co", model.GameOriginHub).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "games" SET "status"=\$1,"updated_at"=\$2 WHERE name IN \(\$3,\$4\) AND origin = \$5 AND remote_infohash <> '' AND status <> \$6`).
		WithArgs(model.GameStatusQueued, anyTime{}, "Moi", "khong-co", model.GameOriginHub, model.GameStatusDownloading).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	res, err := svc.SelectHubGames(context.Background(), HubSelectionRequest{
		Mode: "selected", Add: []string{"Moi", "khong-co"}, Remove: []string{"Cu"}, DownloadNow: true,
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"Moi", "khong-co"}, selection["add"])
	assert.Equal(t, []string{"Cu"}, selection["remove"])
	assert.True(t, pulled, "đọc lại catalog để game vừa chọn có bản ghi")
	assert.Equal(t, 2, res.SelectedCount)
	assert.Equal(t, 1, res.Queued)
	assert.Equal(t, []string{"khong-co"}, res.Unknown)
	assert.NoError(t, mock.ExpectationsWereMet())

	_, err = svc.SelectHubGames(context.Background(), HubSelectionRequest{Add: make([]string, 1001)})
	assert.ErrorIs(t, err, ErrHubTooMany)
}

// Nút Tải trong hộp "Tải game": chọn trên Hub khi cần, rồi xếp tải vào thư mục quán chọn.
func TestDownloadHubGame(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	const byName = `SELECT \* FROM "games" WHERE name = \$1`
	const syncSQL = `UPDATE "games" SET "install_root"=\$1,"status"=\$2,"updated_at"=\$3 WHERE id = \$4 AND status <> \$5`
	gameRow := func(origin string) *sqlmock.Rows {
		return sqlmock.NewRows([]string{"id", "name", "origin", "infohash", "install_root"}).AddRow(id, "Dota 2", origin, "", "")
	}
	// hub dựng Hub giả có một game "Dota 2"; selected ghi lại các lần quán chọn thêm game.
	hub := func(t *testing.T, available, chosen bool) (*GameService, sqlmock.Sqlmock, *[]string) {
		db, mock := newMockDB(t)
		var selected []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == "/api/game-catalog/all":
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 0, "data": map[string]interface{}{
					"mode":  "selected",
					"items": []map[string]interface{}{{"name": "Dota 2", "display_name": "Dota 2", "available": available, "version": 2, "selected": chosen}},
				}})
			case r.Method == http.MethodPut && r.URL.Path == "/api/game-catalog/selection":
				var body struct {
					Mode string   `json:"mode"`
					Add  []string `json:"add"`
				}
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.Equal(t, "selected", body.Mode)
				selected = append(selected, body.Add...)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 0, "data": map[string]interface{}{"mode": "selected"}})
			case r.URL.Path == "/api/game-catalog":
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 0, "data": []CatalogItem{}})
			default:
				t.Errorf("gọi lạ: %s %s", r.Method, r.URL.Path)
			}
		}))
		t.Cleanup(srv.Close)
		svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe, UpstreamURL: srv.URL, CatalogKey: "khoa-hub-khoa-hub-khoa-hub"})
		svc.http = http.DefaultClient
		return svc, mock, &selected
	}
	thuMuc := func(t *testing.T) string {
		p, err := filepath.EvalSymlinks(t.TempDir())
		require.NoError(t, err)
		return p
	}

	t.Run("đã chọn trên Hub: xếp tải vào thư mục quán chọn", func(t *testing.T) {
		svc, mock, selected := hub(t, true, true)
		root := thuMuc(t)
		mock.ExpectQuery(byName).WithArgs("Dota 2", 1).WillReturnRows(gameRow(model.GameOriginHub))
		mock.ExpectQuery(`SELECT \* FROM "games" WHERE id = \$1`).WillReturnRows(gameRow(model.GameOriginHub))
		expectGameUpdate(mock, syncSQL, root, model.GameStatusQueued, anyTime{}, id, model.GameStatusDownloading)
		require.NoError(t, svc.DownloadHubGame(context.Background(), "Dota 2", root))
		assert.Empty(t, *selected, "không chọn lại game đã chọn")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("quán chưa chọn: chọn trên Hub trước", func(t *testing.T) {
		svc, mock, selected := hub(t, true, false)
		root := thuMuc(t)
		expectGameUpdate(mock, `UPDATE "games" SET "auto_update"=\$1,"updated_at"=\$2 WHERE name IN \(\$3\) AND origin = \$4`,
			true, anyTime{}, "Dota 2", model.GameOriginHub)
		mock.ExpectQuery(byName).WillReturnRows(gameRow(model.GameOriginHub))
		mock.ExpectQuery(`SELECT \* FROM "games" WHERE id = \$1`).WillReturnRows(gameRow(model.GameOriginHub))
		expectGameUpdate(mock, syncSQL, root, model.GameStatusQueued, anyTime{}, id, model.GameStatusDownloading)
		require.NoError(t, svc.DownloadHubGame(context.Background(), "Dota 2", root))
		assert.Equal(t, []string{"Dota 2"}, *selected)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("catalog chưa có game: báo thử lại sau", func(t *testing.T) {
		svc, mock, _ := hub(t, true, true)
		mock.ExpectQuery(byName).WillReturnError(gorm.ErrRecordNotFound)
		mock.ExpectQuery(byName).WillReturnError(gorm.ErrRecordNotFound)
		assert.ErrorContains(t, svc.DownloadHubGame(context.Background(), "Dota 2", thuMuc(t)), "thử lại sau")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("trùng tên game cục bộ: từ chối", func(t *testing.T) {
		svc, mock, _ := hub(t, true, true)
		mock.ExpectQuery(byName).WillReturnRows(gameRow(model.GameOriginLocal))
		assert.ErrorContains(t, svc.DownloadHubGame(context.Background(), "Dota 2", thuMuc(t)), "game cục bộ")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("Hub chưa phát được hay không có game: không đụng DB", func(t *testing.T) {
		svc, mock, selected := hub(t, false, false)
		assert.ErrorContains(t, svc.DownloadHubGame(context.Background(), "Dota 2", thuMuc(t)), "chưa phát được")
		assert.ErrorContains(t, svc.DownloadHubGame(context.Background(), "Khong Co", thuMuc(t)), "Hub không có game")
		assert.Empty(t, *selected)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestSelectHubGames_OnlyCafe(t *testing.T) {
	svc := NewGameService(nil, config.GameConfig{Role: config.GameRoleMaster})
	_, err := svc.SelectHubGames(context.Background(), HubSelectionRequest{})
	assert.ErrorIs(t, err, ErrGameCafeOnly)
	_, err = svc.BulkSync([]string{"a"})
	assert.ErrorIs(t, err, ErrGameCafeOnly)
	_, err = svc.HubCatalog(context.Background())
	assert.ErrorIs(t, err, ErrGameCafeOnly)
	assert.ErrorIs(t, svc.DownloadHubGame(context.Background(), "a", ""), ErrGameCafeOnly)
	_, err = svc.Queue(time.Now())
	assert.ErrorIs(t, err, ErrGameCafeOnly)
}

func TestBulkSync(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe})
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "games" SET "status"=\$1,"updated_at"=\$2 WHERE name IN \(\$3,\$4\) AND origin = \$5 AND remote_infohash <> '' AND status <> \$6`).
		WithArgs(model.GameStatusQueued, anyTime{}, "A", "B", model.GameOriginHub, model.GameStatusDownloading).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()
	n, err := svc.BulkSync([]string{"A", "B"})
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.NoError(t, mock.ExpectationsWereMet())
}
