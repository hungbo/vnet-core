package service

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/model"
)

const gameByIDSQL = `SELECT \* FROM "games" WHERE id = \$1`

// LocalFolders đọc tên, nguồn và phần tuỳ chỉnh (custom.folder) của mọi game để bỏ thư mục đã dùng.
const gameNamesSQL = `SELECT "name","origin","custom" FROM "games"`

var gameNameCols = []string{"name", "origin", "custom"}

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
	for _, d := range []string{"Zebra", "alpha", "Taken", "Liên Quân", "_riot_metadata", ".vnet-gameupdate", "bad.", "CON"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, d), 0o755))
	}
	writeEmpty(t, root, "tep-le.txt") // tệp ở gốc không phải thư mục game
	svc, mock := cafeWithRoot(t, root)
	mock.ExpectQuery(gameNamesSQL).WillReturnRows(sqlmock.NewRows(gameNameCols).AddRow("taken", model.GameOriginHub, nil))

	got, err := svc.LocalFolders("")
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "Liên Quân", "Zebra"}, got,
		"bỏ thư mục ẩn, tên máy trạm không mở được, đã là game (không phân biệt hoa thường); tên có dấu vẫn hiện (đặt tên game riêng); sắp theo tên")
	assert.NoError(t, mock.ExpectationsWereMet())

	master := NewGameService(nil, config.GameConfig{Role: config.GameRoleMaster, Root: root})
	_, err = master.LocalFolders("")
	assert.ErrorIs(t, err, ErrGameLocalOnMaster)
	noRoot := NewGameService(nil, config.GameConfig{Role: config.GameRoleCafe})
	_, err = noRoot.LocalFolders("")
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

// lienKet tạo symlink; máy Windows không có quyền tạo symlink thì bỏ qua test.
func lienKet(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("không tạo được symlink: %v", err)
	}
}

func TestLocalFoldersVaLaunchers_ThuMucGameLaLienKet(t *testing.T) {
	root, kho := t.TempDir(), t.TempDir() // kho: thư mục game của phần mềm khác (Kzone…), nằm ngoài gốc game
	writeEmpty(t, kho, "CSGO/KZ_CSGO.exe", "CSGO/bin/cs2.exe", "CSGO/_kz/x.exe", "Khac/inner.exe", "tep.exe")
	lienKet(t, filepath.Join(kho, "CSGO"), filepath.Join(root, "CSGO"))
	lienKet(t, filepath.Join(kho, "tep.exe"), filepath.Join(root, "TepLe"))         // trỏ tới tệp
	lienKet(t, filepath.Join(kho, "KhongCo"), filepath.Join(root, "Hong"))          // trỏ tới chỗ không có
	lienKet(t, filepath.Join(kho, "Khac"), filepath.Join(kho, "CSGO", "LongTrong")) // liên kết bên trong thư mục game
	svc, mock := cafeWithRoot(t, root)
	mock.ExpectQuery(gameNamesSQL).WillReturnRows(sqlmock.NewRows(gameNameCols))

	got, err := svc.LocalFolders("")
	require.NoError(t, err)
	assert.Equal(t, []string{"CSGO"}, got, "liên kết tới thư mục là thư mục game; trỏ tới tệp hay chỗ không có thì không")
	assert.NoError(t, mock.ExpectationsWereMet())

	got, err = svc.LocalLaunchers("CSGO")
	require.NoError(t, err)
	assert.Equal(t, []string{"KZ_CSGO.exe", "bin/cs2.exe"}, got, "dò bên trong thư mục game là liên kết, vẫn bỏ _kz và không đi theo liên kết lồng bên trong")
}

func TestLocalFoldersVaLaunchers_JunctionWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("junction chỉ có trên Windows")
	}
	root, kho := t.TempDir(), t.TempDir()
	writeEmpty(t, kho, "Game/KZ_Game.exe")
	out, err := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(root, "Game"), filepath.Join(kho, "Game")).CombinedOutput()
	require.NoError(t, err, string(out))
	svc, mock := cafeWithRoot(t, root)
	mock.ExpectQuery(gameNamesSQL).WillReturnRows(sqlmock.NewRows(gameNameCols))

	got, err := svc.LocalFolders("")
	require.NoError(t, err)
	assert.Equal(t, []string{"Game"}, got)
	got, err = svc.LocalLaunchers("Game")
	require.NoError(t, err)
	assert.Equal(t, []string{"KZ_Game.exe"}, got)
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

// kho: thư mục game của phần mềm khác (Kzone…) nằm ngoài GAME_ROOT, thêm thẳng không qua junction.
func TestLocalFoldersVaLaunchers_GocTuyY(t *testing.T) {
	root, kho := t.TempDir(), t.TempDir()
	writeEmpty(t, kho, "VALORANT/Riot Client/RiotClientServices.exe", "VALORANT/_kz/x.exe", "Roblox/r.exe", "tep.exe")
	require.NoError(t, os.MkdirAll(filepath.Join(kho, "_kz"), 0o755))
	svc, mock := cafeWithRoot(t, root)
	mock.ExpectQuery(gameNamesSQL).WillReturnRows(sqlmock.NewRows(gameNameCols).AddRow("Roblox", model.GameOriginLocal, nil))

	got, err := svc.LocalFolders(kho + string(filepath.Separator)) // gạch cuối như người gõ
	require.NoError(t, err)
	assert.Equal(t, []string{"VALORANT"}, got, "cùng luật với GAME_ROOT: bỏ thư mục ẩn, tệp lẻ, tên đã là game")
	assert.NoError(t, mock.ExpectationsWereMet())

	got, err = svc.LocalLaunchers(filepath.Join(kho, "VALORANT"))
	require.NoError(t, err)
	assert.Equal(t, []string{"Riot Client/RiotClientServices.exe"}, got)

	for _, bad := range []string{"Online Games", "a/b", `\\may\share`, "//may/share", filepath.Join(kho, "KhongCo")} {
		_, err := svc.LocalFolders(bad)
		assert.Error(t, err, bad)
		_, err = svc.LocalLaunchers(bad)
		assert.Error(t, err, bad)
	}
	assert.NoError(t, mock.ExpectationsWereMet(), "đường dẫn sai không đọc DB")
}

func TestCreateLocal_TuThuMuc_KiemTraTruocKhiChamDB(t *testing.T) {
	root, kho := t.TempDir(), t.TempDir()
	writeEmpty(t, kho, "VALORANT/Riot Client/RiotClientServices.exe", "Liên Quân/game.exe")
	writeEmpty(t, root, "VALORANT/chi-o-goc.exe") // cùng tên trong GAME_ROOT: phải kiểm trong thư mục đã chọn, không phải ở đây
	svc, mock := cafeWithRoot(t, root)
	val := filepath.Join(kho, "VALORANT")
	const l, cr = "Riot Client/RiotClientServices.exe", `E:\Online Games`

	cases := map[string]struct {
		req  *CreateGameRequest
		want string // đoạn phải có trong lỗi: đúng chỗ chặn, không phải lỗi khác
	}{
		"đường dẫn tương đối":      {&CreateGameRequest{Folder: "VALORANT", Launcher: l, ClientRoot: cr}, "đường dẫn đầy đủ"},
		"thư mục chia sẻ":          {&CreateGameRequest{Folder: `\\may\share\VALORANT`, Launcher: l, ClientRoot: cr}, "đường dẫn đầy đủ"},
		"thư mục chia sẻ kiểu /":   {&CreateGameRequest{Folder: "//may/share/VALORANT", Launcher: l, ClientRoot: cr}, "đường dẫn đầy đủ"},
		"thư mục không có":         {&CreateGameRequest{Folder: filepath.Join(kho, "KhongCo"), Launcher: l, ClientRoot: cr}, "không thấy thư mục"},
		"tên game sai":             {&CreateGameRequest{Folder: val, Name: "../x", Launcher: l, ClientRoot: cr}, "tên game chỉ gồm"},
		"thư mục có dấu, chưa tên": {&CreateGameRequest{Folder: filepath.Join(kho, "Liên Quân"), Launcher: "game.exe", ClientRoot: cr}, "nhập tên game riêng"},
		"launcher chỉ ở GAME_ROOT": {&CreateGameRequest{Folder: val, Launcher: "chi-o-goc.exe", ClientRoot: cr}, "không thấy tệp chạy"},
		"mục menu thiếu tệp": {&CreateGameRequest{Folder: val, ClientRoot: cr,
			MenuEntries: []model.GameMenuEntry{{DisplayName: "A", Launcher: "khong-co.exe"}}}, "không thấy tệp chạy"},
		"gốc máy trạm sai": {&CreateGameRequest{Folder: val, Launcher: l, ClientRoot: "Online Games"}, "gốc game trên máy trạm"},
		"gốc máy trạm UNC": {&CreateGameRequest{Folder: val, Launcher: l, ClientRoot: `\\may\games`}, "gốc game trên máy trạm"},
	}
	if runtime.GOOS != "windows" {
		// Thư mục cha /tmp/… không phải ổ đĩa Windows: máy trạm không mở được, phải ghi gốc máy trạm.
		cases["thư mục cha không phải ổ đĩa"] = struct {
			req  *CreateGameRequest
			want string
		}{&CreateGameRequest{Folder: val, Launcher: l}, "gốc game trên máy trạm"}
	}
	for name, c := range cases {
		_, err := svc.CreateLocal(c.req)
		if assert.Error(t, err, name) {
			assert.Contains(t, err.Error(), c.want, name)
		}
	}
	assert.NoError(t, mock.ExpectationsWereMet(), "mọi lỗi phải dừng trước khi đụng DB")
}

// expectInsertGame chờ lượt INSERT game và ghi lại mọi giá trị đã gửi.
func expectInsertGame(mock sqlmock.Sqlmock) *[]driver.Value {
	var seen []driver.Value
	mock.ExpectQuery(`SELECT \* FROM "games" WHERE name = \$1`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectBegin()
	args := make([]driver.Value, 28) // số cột của bảng games
	for i := range args {
		args[i] = ghiNhan{&seen}
	}
	mock.ExpectQuery(`INSERT INTO "games"`).WithArgs(args...).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("g1"))
	mock.ExpectCommit()
	return &seen
}

func TestCreateLocal_TuThuMucNgoaiGameRoot(t *testing.T) {
	root, kho := t.TempDir(), t.TempDir()
	writeEmpty(t, kho, "VALORANT/Riot Client/RiotClientServices.exe", "VALORANT/tools/pre.bat")
	val := filepath.Join(kho, "VALORANT")

	t.Run("gốc máy trạm ghi tay", func(t *testing.T) {
		svc, mock := cafeWithRoot(t, root)
		seen := expectInsertGame(mock)
		g, err := svc.CreateLocal(&CreateGameRequest{
			Folder: " " + val + string(filepath.Separator) + " ", ClientRoot: "E:/Online Games/",
			Launcher: "Riot Client/RiotClientServices.exe", RunFirst: steps("tools/pre.bat"),
		})
		require.NoError(t, err)
		assert.Equal(t, "VALORANT", g.Name, "tên game mặc định là tên thư mục")
		assert.Equal(t, "VALORANT", g.DisplayName)
		assert.Equal(t, `E:\Online Games`, g.Custom.ClientRoot, "gốc máy trạm được chuẩn hoá như ô ở trang Game")
		assert.Equal(t, model.GameOriginLocal, g.Origin)
		assert.Equal(t, model.GameStatusReady, g.Status)
		assert.Contains(t, *seen, driver.Value(`{"client_root":"E:\\Online Games"}`), "custom.client_root được ghi vào DB")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("gốc máy trạm mặc định là thư mục cha", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("thư mục cha phải là ổ đĩa Windows (E:\\…)")
		}
		svc, mock := cafeWithRoot(t, root)
		expectInsertGame(mock)
		g, err := svc.CreateLocal(&CreateGameRequest{Folder: val, Name: "VALORANT", Launcher: "Riot Client/RiotClientServices.exe"})
		require.NoError(t, err)
		want, err := parseGameClientRoot(kho)
		require.NoError(t, err)
		assert.Equal(t, want, g.Custom.ClientRoot)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("thư mục ngay trong GAME_ROOT: theo ổ chung", func(t *testing.T) {
		writeEmpty(t, root, "Game/Bin/game.exe")
		svc, mock := cafeWithRoot(t, root)
		seen := expectInsertGame(mock)
		g, err := svc.CreateLocal(&CreateGameRequest{Folder: filepath.Join(root, "Game"), Launcher: "Bin/game.exe"})
		require.NoError(t, err)
		assert.Equal(t, "Game", g.Name)
		assert.Empty(t, g.Custom.ClientRoot, "máy trạm có thể thấy GAME_ROOT dưới chữ ổ khác, nên không ghi đường dẫn máy chủ")
		for _, v := range *seen {
			if s, ok := v.(string); ok {
				assert.NotContains(t, s, "client_root")
			}
		}
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// Thư mục Roblox của Kzone khi Roblox đã là game của Hub: ngoài GAME_ROOT vẫn chọn được (thêm dưới tên khác), trong
// GAME_ROOT thì không (đó là thư mục Hub tải về). Thư mục game cục bộ đã dùng thì ẩn ở mọi nơi.
func TestLocalFolders_TrungTenGameHub(t *testing.T) {
	root, kho := t.TempDir(), t.TempDir()
	for _, d := range []string{"Roblox", "Khac"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, d), 0o755))
	}
	for _, d := range []string{"Roblox", "VALORANT", "Khac"} {
		require.NoError(t, os.MkdirAll(filepath.Join(kho, d), 0o755))
	}
	rows := func() *sqlmock.Rows {
		return sqlmock.NewRows(gameNameCols).
			AddRow("Roblox", model.GameOriginHub, nil).
			AddRow("Valorant KZ", model.GameOriginLocal, `{"client_root":"E:\\Online Games","folder":"VALORANT"}`)
	}
	svc, mock := cafeWithRoot(t, root)
	mock.ExpectQuery(gameNamesSQL).WillReturnRows(rows())
	mock.ExpectQuery(gameNamesSQL).WillReturnRows(rows())

	got, err := svc.LocalFolders("")
	require.NoError(t, err)
	assert.Equal(t, []string{"Khac"}, got, "trong GAME_ROOT, thư mục trùng tên game Hub là của Hub")
	got, err = svc.LocalFolders(kho)
	require.NoError(t, err)
	assert.Equal(t, []string{"Khac", "Roblox"}, got, "ngoài GAME_ROOT: Roblox của Kzone chọn được, VALORANT đã là game cục bộ thì ẩn")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateLocal_TenKhacThuMuc(t *testing.T) {
	root, kho := t.TempDir(), t.TempDir()
	writeEmpty(t, kho, "Roblox/RobloxPlayerLauncher.exe", "Liên Quân/LQ.exe")
	const cr = `E:\Online Games`

	t.Run("tên trùng game Hub: đặt tên khác", func(t *testing.T) {
		svc, mock := cafeWithRoot(t, root)
		mock.ExpectQuery(`SELECT \* FROM "games" WHERE name = \$1`).WithArgs("Roblox", 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "origin"}).AddRow("g1", "Roblox", model.GameOriginHub))
		_, err := svc.CreateLocal(&CreateGameRequest{Folder: filepath.Join(kho, "Roblox"), Launcher: "RobloxPlayerLauncher.exe", ClientRoot: cr})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "đặt tên game khác")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("tên riêng, thư mục giữ ở custom.folder", func(t *testing.T) {
		svc, mock := cafeWithRoot(t, root)
		seen := expectInsertGame(mock)
		g, err := svc.CreateLocal(&CreateGameRequest{
			Folder: filepath.Join(kho, "Roblox"), Name: " Roblox KZ ", DisplayName: "Roblox", Launcher: "RobloxPlayerLauncher.exe", ClientRoot: cr,
		})
		require.NoError(t, err)
		assert.Equal(t, "Roblox KZ", g.Name)
		assert.Equal(t, "Roblox", g.DisplayName)
		assert.Equal(t, model.GameCustom{ClientRoot: cr, Folder: "Roblox"}, g.Custom)
		assert.Contains(t, *seen, driver.Value(`{"client_root":"E:\\Online Games","folder":"Roblox"}`))
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("thư mục có dấu, tên game không dấu", func(t *testing.T) {
		svc, mock := cafeWithRoot(t, root)
		expectInsertGame(mock)
		g, err := svc.CreateLocal(&CreateGameRequest{Folder: filepath.Join(kho, "Liên Quân"), Name: "Lien Quan", Launcher: "LQ.exe", ClientRoot: cr})
		require.NoError(t, err)
		assert.Equal(t, "Liên Quân", g.Custom.Folder)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// Game cục bộ cùng tên với một game Hub quán chưa tải: thay được sau khi người dùng xác nhận (replace_hub).
func TestCreateLocal_ThayGameHubChuaTai(t *testing.T) {
	root, kho := t.TempDir(), t.TempDir()
	writeEmpty(t, kho, "Roblox/RobloxPlayerLauncher.exe")
	const hubID = "22222222-2222-2222-2222-222222222222"
	req := func(replace bool) *CreateGameRequest {
		return &CreateGameRequest{Folder: filepath.Join(kho, "Roblox"), Launcher: "RobloxPlayerLauncher.exe", ClientRoot: `E:\Online Games`, ReplaceHub: replace}
	}
	hubRow := func(mock sqlmock.Sqlmock, status string, version int) {
		mock.ExpectQuery(`SELECT \* FROM "games" WHERE name = \$1`).WithArgs("Roblox", 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "origin", "status", "version"}).
				AddRow(hubID, "Roblox", model.GameOriginHub, status, version))
	}

	t.Run("chưa xác nhận: hỏi lại, không ghi gì", func(t *testing.T) {
		svc, mock := cafeWithRoot(t, root)
		hubRow(mock, model.GameStatusNotInstalled, 0)
		_, err := svc.CreateLocal(req(false))
		assert.ErrorIs(t, err, ErrGameHubChuaTaiCanXacNhan)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	for _, c := range []struct {
		ten     string
		status  string
		version int
	}{
		{"đã tải xong", model.GameStatusReady, 3},
		{"đang tải", model.GameStatusDownloading, 0},
		{"đang chờ tải", model.GameStatusQueued, 0},
		{"có bản cũ trên ổ", model.GameStatusNotInstalled, 2},
	} {
		t.Run(c.ten+": không thay dù đã xác nhận", func(t *testing.T) {
			svc, mock := cafeWithRoot(t, root)
			hubRow(mock, c.status, c.version)
			_, err := svc.CreateLocal(req(true))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "đã tải hoặc đang tải")
			assert.NoError(t, mock.ExpectationsWereMet(), "không được xoá hay thêm gì")
		})
	}

	t.Run("đã xác nhận: xoá dòng Hub và thêm game cục bộ trong một transaction", func(t *testing.T) {
		svc, mock := cafeWithRoot(t, root)
		hubRow(mock, model.GameStatusNotInstalled, 0)
		mock.ExpectBegin()
		mock.ExpectExec(`DELETE FROM "games" WHERE id = \$1 AND status = \$2 AND version = 0`).
			WithArgs(hubID, model.GameStatusNotInstalled).WillReturnResult(sqlmock.NewResult(0, 1))
		args := make([]driver.Value, 28)
		for i := range args {
			args[i] = sqlmock.AnyArg()
		}
		mock.ExpectQuery(`INSERT INTO "games"`).WithArgs(args...).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("g-moi"))
		mock.ExpectCommit()

		g, err := svc.CreateLocal(req(true))
		require.NoError(t, err)
		assert.Equal(t, "Roblox", g.Name)
		assert.Equal(t, model.GameOriginLocal, g.Origin)
		assert.Empty(t, g.Custom.Folder, "tên trùng tên thư mục")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("vòng nền vừa xếp game Hub vào hàng đợi: không xoá được thì không thêm", func(t *testing.T) {
		svc, mock := cafeWithRoot(t, root)
		hubRow(mock, model.GameStatusNotInstalled, 0)
		mock.ExpectBegin()
		mock.ExpectExec(`DELETE FROM "games"`).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectRollback()

		_, err := svc.CreateLocal(req(true))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "vừa bắt đầu tải")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestTenThuMucDungDuoc(t *testing.T) {
	for _, ok := range []string{"Roblox", "Liên Quân Mobile", "Game (2024)", "a.b"} {
		assert.True(t, tenThuMucDungDuoc(ok), ok)
	}
	for _, no := range []string{"", ".", "..", "Game.", "Game ", "CON", "a:b", `a\b`, "a/b", "a|b", "a\x01b", "Li\ufffdn", strings.Repeat("a", 256)} {
		assert.False(t, tenThuMucDungDuoc(no), no)
	}
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
