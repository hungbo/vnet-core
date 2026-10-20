package service

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/model"
)

const gameByIDSQL = `SELECT \* FROM "games" WHERE id = \$1`

func writeEmpty(t *testing.T, root string, rel ...string) {
	t.Helper()
	for _, r := range rel {
		p := filepath.Join(root, filepath.FromSlash(r))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte("x"), 0o644))
	}
}

func cafeWithRoot(t *testing.T, root string) (*GameService, sqlmock.Sqlmock) {
	t.Helper()
	db, mock := newMockDB(t)
	return NewGameService(db, config.GameConfig{Role: config.GameRoleCafe, Root: root, UpstreamURL: "http://hub", CatalogKey: "khoa-catalog-khoa-catalog"}), mock
}

func TestLaTepChayVaThuMucAn(t *testing.T) {
	for _, ok := range []string{"a.exe", "A.EXE", "run.bat", "x.cmd", "Game.lnk"} {
		assert.True(t, laTepChay(ok), ok)
	}
	for _, no := range []string{"readme.txt", "a.dll", "exe", "a.exe.bak", ""} {
		assert.False(t, laTepChay(no), no)
	}
	assert.True(t, thuMucAn(".git"))
	assert.True(t, thuMucAn("_riot_metadata"))
	assert.False(t, thuMucAn("Riot Games"))
}

func TestLocalFolders(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"Zebra", "alpha", "Taken", "_riot_metadata", ".vnet-gameupdate", "bad.", "CON"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, d), 0o755))
	}
	writeEmpty(t, root, "tep-le.txt") // tệp ở gốc không phải thư mục game
	svc, mock := cafeWithRoot(t, root)
	mock.ExpectQuery(`SELECT "name" FROM "games"`).WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("taken"))

	got, err := svc.LocalFolders()
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "Zebra"}, got, "bỏ thư mục ẩn, tên không hợp lệ, đã là game (không phân biệt hoa thường); sắp theo tên")
	assert.NoError(t, mock.ExpectationsWereMet())

	master := NewGameService(nil, config.GameConfig{Role: config.GameRoleMaster, Root: root})
	_, err = master.LocalFolders()
	assert.ErrorIs(t, err, ErrGameLocalOnMaster)
	noRoot := NewGameService(nil, config.GameConfig{Role: config.GameRoleCafe})
	_, err = noRoot.LocalFolders()
	assert.ErrorIs(t, err, ErrGameNeedRoot)
}

func TestLocalLaunchers(t *testing.T) {
	root := t.TempDir()
	writeEmpty(t, root,
		"Game/Run.exe", "Game/Bin/game.exe", "Game/Bin/readme.txt", "Game/Bin/helper.dll", "Game/Setup.BAT",
		"Game/.hidden/x.exe", "Game/_vnet/y.exe", "Game/a/b/c/ok3.exe", "Game/a/b/c/d/deep.exe", "Game/a/ok.lnk")
	svc, _ := cafeWithRoot(t, root)

	got, err := svc.LocalLaunchers("Game")
	require.NoError(t, err)
	assert.Equal(t, []string{"Run.exe", "Setup.BAT", "a/ok.lnk", "Bin/game.exe", "a/b/c/ok3.exe"}, got,
		"tệp nông trước, bỏ thư mục ẩn và _vnet, bỏ tệp sâu quá 3 cấp thư mục, bỏ tệp không chạy được")

	for _, bad := range []string{"", "..", "../x", "a/b", `a\b`, "C:", "KhongCo"} {
		_, err := svc.LocalLaunchers(bad)
		assert.Error(t, err, bad)
	}
	master := NewGameService(nil, config.GameConfig{Role: config.GameRoleMaster, Root: root})
	_, err = master.LocalLaunchers("Game")
	assert.ErrorIs(t, err, ErrGameLocalOnMaster)
}

func TestCreateLocal_KiemTraTruocKhiChamDB(t *testing.T) {
	root := t.TempDir()
	writeEmpty(t, root, "Game/Bin/game.exe", "Game/readme.txt")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "Game", "Folder.exe"), 0o755)) // thư mục tên giống tệp chạy
	svc, mock := cafeWithRoot(t, root)

	cases := map[string]*CreateGameRequest{
		"tên hỏng":                   {Name: "..\\x", Launcher: "Bin/game.exe"},
		"thiếu thư mục":              {Name: "KhongCo", Launcher: "Bin/game.exe"},
		"thiếu launcher":             {Name: "Game"},
		"launcher tuyệt đối":         {Name: "Game", Launcher: "C:/Windows/notepad.exe"},
		"launcher lên thư mục trên":  {Name: "Game", Launcher: "../Game/Bin/game.exe"},
		"launcher không có":          {Name: "Game", Launcher: "Bin/khong-co.exe"},
		"launcher là thư mục":        {Name: "Game", Launcher: "Folder.exe"},
		"mục menu thiếu tệp":         {Name: "Game", MenuEntries: []model.GameMenuEntry{{DisplayName: "A", Launcher: "khong-co.exe"}}},
		"icon tuyệt đối":             {Name: "Game", Launcher: "Bin/game.exe", Icon: "C:/x.ico"},
		"tên hiển thị có điều khiển": {Name: "Game", Launcher: "Bin/game.exe", DisplayName: "a\x00b"},
	}
	for name, req := range cases {
		_, err := svc.CreateLocal(req)
		assert.Error(t, err, name)
	}
	assert.NoError(t, mock.ExpectationsWereMet(), "mọi lỗi phải dừng trước khi đụng DB")

	master := NewGameService(nil, config.GameConfig{Role: config.GameRoleMaster, Root: root})
	_, err := master.CreateLocal(&CreateGameRequest{Name: "Game", Launcher: "Bin/game.exe"})
	assert.ErrorIs(t, err, ErrGameLocalOnMaster)
	noRoot := NewGameService(nil, config.GameConfig{Role: config.GameRoleCafe})
	_, err = noRoot.CreateLocal(&CreateGameRequest{Name: "Game", Launcher: "Bin/game.exe"})
	assert.ErrorIs(t, err, ErrGameNeedRoot)
}

func TestCreateLocal_Luu(t *testing.T) {
	root := t.TempDir()
	writeEmpty(t, root, "Game/Bin/game.exe")
	svc, mock := cafeWithRoot(t, root)

	mock.ExpectQuery(`SELECT \* FROM "games" WHERE name = \$1`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectBegin()
	args := make([]driver.Value, 28) // số cột của bảng games; chỉ vài giá trị cần khớp
	for i := range args {
		args[i] = sqlmock.AnyArg()
	}
	var captured []driver.Value
	mock.ExpectQuery(`INSERT INTO "games"`).WithArgs(args...).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("g1"))
	mock.ExpectCommit()
	_ = captured

	g, err := svc.CreateLocal(&CreateGameRequest{Name: " Game ", Category: "Games Offline", Launcher: "Bin/game.exe", Priority: 3})
	require.NoError(t, err)
	assert.Equal(t, "Game", g.Name)
	assert.Equal(t, "Game", g.DisplayName, "tên hiển thị mặc định là tên thư mục")
	assert.Equal(t, model.GameOriginLocal, g.Origin)
	assert.Equal(t, model.GameStatusReady, g.Status)
	assert.Equal(t, 100, g.Progress)
	assert.True(t, g.Enabled)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateLocal_TrungTen(t *testing.T) {
	root := t.TempDir()
	writeEmpty(t, root, "Game/Bin/game.exe")
	req := &CreateGameRequest{Name: "Game", Launcher: "Bin/game.exe"}

	t.Run("trùng game của Hub", func(t *testing.T) {
		svc, mock := cafeWithRoot(t, root)
		mock.ExpectQuery(`SELECT \* FROM "games" WHERE name = \$1`).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "origin"}).AddRow("g1", "Game", model.GameOriginHub))
		_, err := svc.CreateLocal(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Hub")
		assert.NoError(t, mock.ExpectationsWereMet(), "không được ghi gì")
	})
	t.Run("đã thêm rồi", func(t *testing.T) {
		svc, mock := cafeWithRoot(t, root)
		mock.ExpectQuery(`SELECT \* FROM "games" WHERE name = \$1`).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "origin"}).AddRow("g1", "Game", model.GameOriginLocal))
		_, err := svc.CreateLocal(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "đã có")
	})
}

// Catalog có game cùng tên với một game cục bộ: giữ bản cục bộ, không ghi gì, và báo cho trang Game.
func TestPullCatalog_KhongDeGameCucBo(t *testing.T) {
	items := []CatalogItem{{Name: "lol", DisplayName: "Liên Minh", Category: "Online", Launcher: "a.exe", Version: 2, Infohash: "h2"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 0, "data": items})
	}))
	defer srv.Close()
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe, UpstreamURL: srv.URL, CatalogKey: "k"})
	svc.http = http.DefaultClient
	mock.ExpectQuery(`SELECT \* FROM "games" WHERE name = \$1`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "display_name", "origin"}).AddRow("g1", "lol", "Game của quán", model.GameOriginLocal))

	settings := func() {
		mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1`).WithArgs("games").
			WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}))
	}
	require.NoError(t, svc.pullCatalog(context.Background()))
	settings()
	assert.Equal(t, []string{"lol"}, svc.Info().LocalClash)
	assert.NoError(t, mock.ExpectationsWereMet(), "không được UPDATE hay INSERT")

	// lượt đọc sau mà catalog không còn game đó: hết trùng
	items = nil
	require.NoError(t, svc.pullCatalog(context.Background()))
	settings()
	assert.Empty(t, svc.Info().LocalClash)
}

func TestDelete_QuanChiXoaDuocGameCucBo(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	t.Run("game của Hub: không", func(t *testing.T) {
		svc, mock := cafeWithRoot(t, t.TempDir())
		mock.ExpectQuery(gameByIDSQL).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "origin"}).AddRow(id, "g", model.GameOriginHub))
		assert.ErrorIs(t, svc.Delete(id), ErrGameMasterOnly)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("id lạ: không đọc DB", func(t *testing.T) {
		svc, mock := cafeWithRoot(t, t.TempDir())
		assert.ErrorIs(t, svc.Delete("abc"), ErrGameMasterOnly)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("game cục bộ: xoá bản ghi", func(t *testing.T) {
		svc, mock := cafeWithRoot(t, t.TempDir())
		mock.ExpectQuery(gameByIDSQL).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "origin"}).AddRow(id, "g", model.GameOriginLocal))
		mock.ExpectBegin()
		mock.ExpectExec(`DELETE FROM "game_publishes"`).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec(`DELETE FROM "games"`).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		assert.NoError(t, svc.Delete(id))
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestUpdate_GameCucBoSuaDuocMoiTruong(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	svc, mock := cafeWithRoot(t, t.TempDir())
	row := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"id", "name", "display_name", "origin"}).AddRow(id, "g", "Cũ", model.GameOriginLocal)
	}
	mock.ExpectQuery(gameByIDSQL).WillReturnRows(row())
	expectGameUpdate(mock, `UPDATE "games" SET "category"=\$1,"display_name"=\$2,"updated_at"=\$3 WHERE "id" = \$4`,
		"Tools", "Mới", anyTime{}, id)
	mock.ExpectQuery(gameByIDSQL).WillReturnRows(row())

	name, cat := "Mới", "Tools"
	_, err := svc.Update(id, &UpdateGameRequest{DisplayName: &name, Category: &cat})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSyncNow_TuChoiGameCucBo(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	svc, mock := cafeWithRoot(t, t.TempDir())
	mock.ExpectQuery(gameByIDSQL).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "origin"}).AddRow(id, "g", model.GameOriginLocal))
	assert.ErrorIs(t, svc.SyncNow(id), ErrGameIsLocal)
	assert.NoError(t, mock.ExpectationsWereMet(), "không được đổi trạng thái sang queued")
}

func TestBuildReport_BoQuaGameCucBo(t *testing.T) {
	r := buildReport([]model.Game{
		{Name: "hub", Version: 2, Status: "ready", Progress: 100, Origin: model.GameOriginHub},
		{Name: "rieng", Status: "ready", Progress: 100, Origin: model.GameOriginLocal},
		{Name: "cu", Version: 1, Status: "ready", Progress: 100}, // dữ liệu từ trước, cột origin mặc định hub nhưng Go đọc ra rỗng
	}, 5)
	names := []string{}
	for _, g := range r.Games {
		names = append(names, g.Name)
	}
	assert.Equal(t, []string{"hub", "cu"}, names, "Hub không biết game cục bộ của quán")
}
