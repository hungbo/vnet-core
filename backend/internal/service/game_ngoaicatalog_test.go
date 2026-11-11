package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/model"
)

// Game Hub quán đang dùng mà catalog không còn gửi (quán chuyển sang "chỉ nhận game đã chọn" lúc PUBG còn đang tải, nên
// PUBG rơi khỏi lựa chọn): dòng ở quán đứng yên ở bản cũ. Trang Game phải được báo; game quán chưa bấm Tải thì không.
func TestPullCatalog_BaoGameHubKhongConTrongCatalog(t *testing.T) {
	items := []CatalogItem{{Name: "lol", DisplayName: "Liên Minh", Launcher: "a.exe", Version: 2, Infohash: "h2"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 0, "data": items})
	}))
	defer srv.Close()
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe, UpstreamURL: srv.URL, CatalogKey: "k"})
	svc.http = http.DefaultClient
	settings := func() {
		mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1`).WithArgs("games").
			WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}))
	}
	cols := []string{"id", "name", "origin", "enabled", "infohash", "install_root"}

	// "lol" là game cục bộ trùng tên: chỉ đọc, không ghi, để lượt đọc catalog không có UPDATE nào.
	mock.ExpectQuery(`SELECT \* FROM "games" WHERE name = \$1`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "origin"}).AddRow("g0", "lol", model.GameOriginLocal))
	mock.ExpectQuery(`SELECT \* FROM "games" WHERE \(origin = \$1 AND enabled = \$2\) AND name NOT IN \(\$3\) ORDER BY name`).
		WithArgs(model.GameOriginHub, true, "lol").
		WillReturnRows(sqlmock.NewRows(cols).
			AddRow("g1", "DangTai", model.GameOriginHub, true, "", `E:\Games`). // đã bấm Tải, chưa xong bản nào
			AddRow("g2", "MoiVao", model.GameOriginHub, true, "", "").          // chưa bấm Tải: không báo
			AddRow("g3", "PUBG", model.GameOriginHub, true, "h6", ""))          // đã tải ở GAME_ROOT
	require.NoError(t, svc.pullCatalog(context.Background()))
	settings()
	assert.Equal(t, []string{"DangTai", "PUBG"}, svc.Info().NotInCatalog)

	// Quán chọn lại, catalog gửi lại cả hai (catalog rỗng thì không có NOT IN): hết báo.
	items = nil
	mock.ExpectQuery(`SELECT \* FROM "games" WHERE origin = \$1 AND enabled = \$2 ORDER BY name`).
		WithArgs(model.GameOriginHub, true).
		WillReturnRows(sqlmock.NewRows(cols))
	require.NoError(t, svc.pullCatalog(context.Background()))
	settings()
	assert.Empty(t, svc.Info().NotInCatalog)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Đọc danh sách lỗi thì giữ danh sách cũ: catalog vẫn đã ghi xong, không được làm mất cảnh báo.
func TestPullCatalog_NgoaiCatalogLoiDBGiuDanhSachCu(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 0, "data": []CatalogItem{}})
	}))
	defer srv.Close()
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe, UpstreamURL: srv.URL, CatalogKey: "k"})
	svc.http = http.DefaultClient
	svc.notInCatalog = []string{"PUBG"}
	mock.ExpectQuery(`SELECT \* FROM "games" WHERE origin = \$1 AND enabled = \$2 ORDER BY name`).
		WillReturnError(assert.AnError)
	require.NoError(t, svc.pullCatalog(context.Background()))
	assert.Equal(t, []string{"PUBG"}, svc.notInCatalog)
	assert.NoError(t, mock.ExpectationsWereMet())
}
