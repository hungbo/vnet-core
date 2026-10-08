package service

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/gameupdate"
	"github.com/vnet/core/internal/model"
	"gorm.io/gorm"
)

func TestWithinWindow(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 10, 7, h, m, 0, 0, time.Local) }
	assert.True(t, withinWindow("", at(15, 0)), "rỗng là lúc nào cũng được")
	assert.True(t, withinWindow("02:00-08:00", at(2, 0)))
	assert.True(t, withinWindow("02:00-08:00", at(7, 59)))
	assert.False(t, withinWindow("02:00-08:00", at(8, 0)))
	assert.False(t, withinWindow("02:00-08:00", at(15, 0)))
	// Vắt qua nửa đêm.
	assert.True(t, withinWindow("22:00-06:00", at(23, 30)))
	assert.True(t, withinWindow("22:00-06:00", at(5, 0)))
	assert.False(t, withinWindow("22:00-06:00", at(12, 0)))
	// Gõ sai thì không được làm quán đứng im mãi.
	assert.True(t, withinWindow("2h-8h", at(15, 0)))
	assert.True(t, withinWindow("25:00-08:00", at(15, 0)))
}

func TestTenGameVaLauncher(t *testing.T) {
	for _, ok := range []string{"League of Legends", "CS2", "FC Online (VN)", "Audition_2.0", "Console", "COM10", "Aux Game"} {
		assert.True(t, tenGameDungDuoc(ok), ok)
	}
	for _, bad := range []string{
		"", "../x", "a/b", `a\b`, "C:", ".hidden", " leading", "a..b",
		"Game.", "Game ", // Windows bỏ dấu chấm/khoảng trắng cuối: "Game." và "Game" là một thư mục
		"CON", "con", "PRN", "AUX", "NUL", "COM1", "com9", "LPT1", "lpt9", "NUL.txt", "con.x.y", "AUX .txt",
	} {
		assert.False(t, tenGameDungDuoc(bad), bad)
	}
	assert.NoError(t, checkLauncher("Bin/Game.exe"))
	assert.NoError(t, checkLauncher(`Bin\Game.exe`))
	assert.NoError(t, checkLauncher("./Game.exe"))
	assert.NoError(t, checkLauncher(""))
	assert.NoError(t, checkLauncher("Riot Client/RiotClientServices.exe"))
	assert.NoError(t, checkLauncher("Console/Game.exe"), "Console không phải CON")
	assert.Error(t, checkLauncher("../evil.exe"))
	assert.Error(t, checkLauncher(`C:\Windows\cmd.exe`))
	assert.Error(t, checkLauncher("/bin/sh"))
	for _, bad := range []string{
		`\\server\share\x.exe`, `\Windows\x.exe`, "/etc/x", // UNC và gốc ổ đĩa
		"Bin./Game.exe", "Bin /Game.exe", "Bin/Game.exe.", // chấm/khoảng trắng cuối thành phần
		"CON", "Bin/NUL", "Bin/nul.exe", `Bin\COM1.exe`, "LPT3/x.exe", "aux.txt",
	} {
		assert.Error(t, checkLauncher(bad), bad)
	}
}

func TestNormalizeGameClientRoot(t *testing.T) {
	for in, want := range map[string]string{
		`D:\Games`:          `D:\Games`,
		`D:\Games\`:         `D:\Games`,
		`  e:\Game Disk\  `: `e:\Game Disk`,
		`D:/Games/`:         `D:\Games`,
		`D:\Games\Online`:   `D:\Games\Online`,
		`D:\`:               `D:`,       // ổ gốc: máy trạm ghép thêm `\` + tên game
		`D:`:                `D:`,       // chính là giá trị trang quản trị nạp lại từ ổ gốc
		`D:\\Games`:         `D:\Games`, // gõ nhầm gạch đôi: gộp lại thay vì để máy trạm từ chối
		`D://Games//Online`: `D:\Games\Online`,
	} {
		assert.Equal(t, want, normalizeGameClientRoot(in), in)
		// Trang quản trị nạp giá trị đã chuẩn hoá rồi Lưu lại: không được trượt về mặc định.
		assert.Equal(t, want, normalizeGameClientRoot(want), "chuẩn hoá hai lần: "+in)
	}
	// Rỗng, không phải đường dẫn tuyệt đối kiểu Windows, hoặc có thành phần mà máy
	// trạm sẽ từ chối (gameRootBase) thì dùng mặc định.
	for _, bad := range []string{
		"", "   ", "Games", `D:Games`, `\\server\share`, "/mnt/games", `1:\x`,
		`\\?\D:\Games`, `D:\Games\.`, `D:\Games\..`, `D:\Games\..\Windows`, `D:\Games.`, `D:\Games \x`,
		`D:\Games\a|b`, `D:\Games\a*`, `D:\Games\a:b`, "D:\\Games\x00",
	} {
		assert.Equal(t, GameClientRootDefault, normalizeGameClientRoot(bad), bad)
	}
}

func TestKiemTraCaiDatGame(t *testing.T) {
	assert.NoError(t, kiemTraCaiDatGame(map[string]interface{}{"update_window": "02:00-08:00"}), "không đụng tới client_root")
	for _, ok := range []string{"", "   ", `D:\Games`, `E:\`, `E:`, `D:\\Games`} {
		assert.NoError(t, kiemTraCaiDatGame(map[string]interface{}{GameClientRootKey: ok}), ok)
	}
	for _, bad := range []string{`Games`, `D:Games`, `\\server\share`, `D:\Games\..\x`, `D:\Games.`, `D:\Games\a|b`} {
		assert.Error(t, kiemTraCaiDatGame(map[string]interface{}{GameClientRootKey: bad}), bad)
	}
	assert.Error(t, kiemTraCaiDatGame(map[string]interface{}{GameClientRootKey: 5}))
	assert.Error(t, kiemTraCaiDatGame(map[string]interface{}{GameClientRootKey: nil}))
}

// PUT /settings/games với gốc game sai phải bị từ chối TRƯỚC khi chạm DB.
func TestSettingsUpdate_GameClientRootSai(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSettingsService(db, NewAuditService(db))

	_, err := svc.Update(GamesGroup, map[string]interface{}{GameClientRootKey: `D:\Games\..\x`})
	require.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet(), "không được có truy vấn nào")
}

func TestGameService_Menu(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe})

	mock.ExpectQuery(`SELECT \* FROM "games" ORDER BY priority DESC, display_name ASC`).
		WillReturnRows(sqlmock.NewRows([]string{"name", "display_name", "category", "launcher", "launch_args", "enabled", "version", "status", "progress"}).
			AddRow("lol", "League of Legends", "Online", "Riot Client/RiotClientServices.exe", "--launch-product=league_of_legends", true, 3, model.GameStatusReady, 100).
			AddRow("cs2", "CS2", "FPS", "game/bin/win64/cs2.exe", "", true, 0, model.GameStatusDownloading, 40).
			AddRow("off", "Tắt", "", "a.exe", "", false, 1, model.GameStatusReady, 100).
			AddRow("nolauncher", "Chưa có tệp chạy", "", "", "", true, 1, model.GameStatusReady, 100))
	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1`).WithArgs("games").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}).
			AddRow("games", "client_root", `"E:/Game Disk/"`))

	menu, err := svc.Menu()
	require.NoError(t, err)
	assert.Equal(t, `E:\Game Disk`, menu.Root)
	require.Len(t, menu.Items, 2, "game tắt và game chưa có launcher không lên menu")
	assert.Equal(t, GameMenuItem{
		Name: "lol", DisplayName: "League of Legends", Category: "Online",
		Launcher: "Riot Client/RiotClientServices.exe", LaunchArgs: "--launch-product=league_of_legends",
		Status: model.GameStatusReady, Progress: 100, Version: 3,
	}, menu.Items[0])
	assert.Equal(t, model.GameStatusDownloading, menu.Items[1].Status, "game đang tải vẫn hiện")
	assert.Equal(t, 40, menu.Items[1].Progress)
	assert.NoError(t, mock.ExpectationsWereMet())

	// Hợp đồng JSON mà máy trạm đọc.
	raw, err := json.Marshal(menu)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"root":"E:\\Game Disk"`)
	assert.Contains(t, string(raw), `"display_name":"League of Legends"`)
	assert.Contains(t, string(raw), `"launch_args":`)
}

func TestGameService_Menu_RongVaMacDinh(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{}) // GAME_ROLE trống: vẫn không báo lỗi

	mock.ExpectQuery(`SELECT \* FROM "games"`).WillReturnRows(sqlmock.NewRows([]string{"name"}))
	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1`).WithArgs("games").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}))

	menu, err := svc.Menu()
	require.NoError(t, err)
	assert.Equal(t, GameClientRootDefault, menu.Root)
	assert.NotNil(t, menu.Items, "JSON phải ra [] chứ không phải null")
	raw, _ := json.Marshal(menu)
	assert.JSONEq(t, `{"root":"D:\\Games","items":[]}`, string(raw))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------- vòng tải game ở quán

// expectGameUpdate: GORM bọc mỗi lệnh ghi trong một giao dịch.
func expectGameUpdate(mock sqlmock.Sqlmock, sql string, args ...driver.Value) {
	mock.ExpectBegin()
	e := mock.ExpectExec(sql)
	if len(args) > 0 {
		e = e.WithArgs(args...)
	}
	e.WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
}

const nextInstallSQL = `SELECT \* FROM "games" WHERE enabled = \$1 AND remote_infohash <> ''`

var gameCols = []string{"id", "name", "enabled", "auto_update", "priority", "infohash", "remote_infohash", "status"}

func gameRow(id, name string, autoUpdate bool, priority int, infohash, remote, status string) []driver.Value {
	return []driver.Value{id, name, true, autoUpdate, priority, infohash, remote, status}
}

// pickInstall chạy nextInstall với các dòng game và khung giờ cho trước.
func pickInstall(t *testing.T, svc *GameService, mock sqlmock.Sqlmock, window string, now time.Time, rows ...[]driver.Value) *model.Game {
	t.Helper()
	r := sqlmock.NewRows(gameCols)
	for _, row := range rows {
		r.AddRow(row...)
	}
	mock.ExpectQuery(nextInstallSQL).WithArgs(true).WillReturnRows(r)
	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1`).WithArgs("games").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}).AddRow("games", GameUpdateWindowKey, `"`+window+`"`))
	g := svc.nextInstall(now)
	require.NoError(t, mock.ExpectationsWereMet())
	return g
}

func TestNextInstall_KhungGioVaTuCapNhat(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 10, 7, h, m, 0, 0, time.Local) }
	outdated := func(auto bool, status string) []driver.Value {
		return gameRow("g1", "lol", auto, 0, "old", "new", status)
	}
	upToDate := func(auto bool, status string) []driver.Value {
		return gameRow("g1", "lol", auto, 0, "same", "same", status)
	}
	cases := []struct {
		name   string
		now    time.Time
		row    []driver.Value
		picked bool
	}{
		{"tự cập nhật, trong khung giờ", at(3, 0), outdated(true, model.GameStatusReady), true},
		{"tự cập nhật, NGOÀI khung giờ", at(15, 0), outdated(true, model.GameStatusReady), false},
		{"tắt tự cập nhật", at(3, 0), outdated(false, model.GameStatusReady), false},
		// Người quản trị bấm "tải ngay": bỏ qua cả hai.
		{"queued (bấm tay), ngoài khung giờ", at(15, 0), outdated(true, model.GameStatusQueued), true},
		{"queued (bấm tay), tắt tự cập nhật", at(15, 0), outdated(false, model.GameStatusQueued), true},
		// Lần tải dở bị cắt khi máy chủ dừng được recoverDownloading đưa về not_installed, không phải queued:
		// nó phải theo đúng khung giờ như mọi lần tự cập nhật.
		{"tải dở sau khi khởi động lại, ngoài khung giờ", at(15, 0), outdated(true, model.GameStatusNotInstalled), false},
		{"tải dở sau khi khởi động lại, trong khung giờ", at(3, 0), outdated(true, model.GameStatusNotInstalled), true},
		// Game đã đúng bản master: không có gì để cập nhật, dù tự cập nhật bật và đang trong khung giờ.
		{"đã đúng bản, ready", at(3, 0), upToDate(true, model.GameStatusReady), false},
		{"đã đúng bản, ready, bấm tay", at(15, 0), upToDate(false, model.GameStatusQueued), true},
		// Kiểm tra lại bị lỗi (master tắt, mạng đứt…): failInstall hứa tự thử lại nên phải thử — không lệ thuộc
		// tự cập nhật hay khung giờ vì chẳng có bản mới nào để tải, và lần đó do người quản trị bấm.
		{"đã đúng bản, kiểm tra lại lỗi", at(15, 0), upToDate(false, model.GameStatusError), true},
		// Còn game chờ bản mới mà lỗi thì vẫn theo luật tự cập nhật; tắt tự cập nhật thì chờ người bấm tay
		// (failInstall nói đúng điều đó).
		{"chờ bản mới, lỗi, tự cập nhật bật, trong khung giờ", at(3, 0), outdated(true, model.GameStatusError), true},
		{"chờ bản mới, lỗi, tắt tự cập nhật", at(3, 0), outdated(false, model.GameStatusError), false},
		{"chờ bản mới, lỗi, ngoài khung giờ", at(15, 0), outdated(true, model.GameStatusError), false},
	}
	for _, c := range cases {
		db, mock := newMockDB(t)
		svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe})
		g := pickInstall(t, svc, mock, "02:00-08:00", c.now, c.row)
		assert.Equal(t, c.picked, g != nil, c.name)
	}
}

func TestNextInstall_UuTienCaoTruoc(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe})
	g := pickInstall(t, svc, mock, "", time.Now(),
		gameRow("g1", "low", true, 1, "a", "b", model.GameStatusReady),
		gameRow("g2", "high", true, 9, "a", "b", model.GameStatusReady))
	require.NotNil(t, g)
	assert.Equal(t, "high", g.Name)
}

// Một game tải lỗi không được tải lại ngay (vòng nóng) mà chờ giãn dần; game khác vẫn được làm;
// bản mới của master hoặc lệnh bấm tay xoá thời gian chờ.
func TestInstallBackoff(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe, StallTimeout: 10 * time.Minute})
	now := time.Date(2026, 10, 7, 3, 0, 0, 0, time.Local)

	bad := model.Game{ID: "g1", Name: "bad", RemoteInfohash: "v4"}
	expectGameUpdate(mock, `UPDATE "games" SET`)
	svc.failInstall(bad, gameupdate.ErrStalled, now)
	require.NoError(t, mock.ExpectationsWereMet())

	rows := func() [][]driver.Value {
		return [][]driver.Value{
			gameRow("g1", "bad", true, 9, "v3", "v4", model.GameStatusError),
			gameRow("g2", "other", true, 1, "x", "y", model.GameStatusReady),
		}
	}
	g := pickInstall(t, svc, mock, "", now.Add(time.Minute), rows()...)
	require.NotNil(t, g)
	assert.Equal(t, "other", g.Name, "game lỗi đang chờ, game khác vẫn được tải dù ưu tiên thấp hơn")

	g = pickInstall(t, svc, mock, "", now.Add(retryDelay(1)+time.Second), rows()...)
	require.NotNil(t, g)
	assert.Equal(t, "bad", g.Name, "hết thời gian chờ thì thử lại")

	// Bản mới trên master: tính lại từ đầu.
	r := rows()
	r[0][6] = "v5"
	g = pickInstall(t, svc, mock, "", now.Add(time.Minute), r...)
	require.NotNil(t, g)
	assert.Equal(t, "bad", g.Name, "bản mới thay thế lượt lỗi")

	// Bấm tay bỏ qua thời gian chờ.
	r = rows()
	r[0][7] = model.GameStatusQueued
	g = pickInstall(t, svc, mock, "", now.Add(time.Minute), r...)
	require.NotNil(t, g)
	assert.Equal(t, "bad", g.Name)
}

// Game đã đúng bản master mà lần kiểm tra lại bị lỗi: nextInstall không còn bỏ rơi nó mãi mãi. Đo được trên
// hai quán thật: lỗi một lần là "error" vĩnh viễn dù thông báo hứa "Tự thử lại sau 5 phút".
func TestNextInstall_GameDungBanBiLoiThiThuLaiSauThoiGianCho(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe})
	now := time.Date(2026, 10, 7, 15, 0, 0, 0, time.Local) // ngoài khung giờ 02:00-08:00
	row := gameRow("g1", "big", false, 0, "v6", "v6", model.GameStatusError)

	expectGameUpdate(mock, `UPDATE "games" SET`)
	svc.failInstall(model.Game{ID: "g1", Name: "big", Infohash: "v6", RemoteInfohash: "v6"}, errors.New("tải tệp torrent: connection refused"), now)
	require.NoError(t, mock.ExpectationsWereMet())

	assert.Nil(t, pickInstall(t, svc, mock, "02:00-08:00", now.Add(time.Minute), row), "đang chờ thử lại")
	g := pickInstall(t, svc, mock, "02:00-08:00", now.Add(retryDelay(1)+time.Second), row)
	require.NotNil(t, g, "hết thời gian chờ thì phải thử lại, không để error mãi")
	assert.Equal(t, "big", g.Name)
}

// chuaChuoi khớp tham số chuỗi có (hoặc không có) một đoạn cho trước.
type chuaChuoi struct {
	doan string
	co   bool
}

func (c chuaChuoi) Match(v driver.Value) bool {
	s, ok := v.(string)
	return ok && strings.Contains(s, c.doan) == c.co
}

// Thông báo lỗi chỉ được hứa "tự thử lại" khi nextInstall sẽ thật sự nhặt game đó lên.
func TestFailInstall_ChiHuaTuThuLaiKhiThatSuSeThuLai(t *testing.T) {
	cases := []struct {
		ten string
		g   model.Game
		hua bool
	}{
		{"đã đúng bản", model.Game{ID: "g1", Name: "a", Infohash: "h", RemoteInfohash: "h"}, true},
		{"đã đúng bản, tắt tự cập nhật", model.Game{ID: "g1", Name: "a", Infohash: "h", RemoteInfohash: "h", AutoUpdate: false}, true},
		{"chờ bản mới, tự cập nhật bật", model.Game{ID: "g1", Name: "a", Infohash: "old", RemoteInfohash: "new", AutoUpdate: true}, true},
		{"chưa cài, tự cập nhật bật", model.Game{ID: "g1", Name: "a", RemoteInfohash: "new", AutoUpdate: true}, true},
		{"chờ bản mới, tắt tự cập nhật", model.Game{ID: "g1", Name: "a", Infohash: "old", RemoteInfohash: "new"}, false},
	}
	for _, c := range cases {
		db, mock := newMockDB(t)
		svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe})
		expectGameUpdate(mock, `UPDATE "games" SET "error"=\$1,"status"=\$2,"updated_at"=\$3 WHERE id = \$4`,
			chuaChuoi{"Tự thử lại", c.hua}, model.GameStatusError, anyTime{}, "g1")
		svc.failInstall(c.g, errors.New("lỗi bất kỳ"), time.Now())
		require.NoError(t, mock.ExpectationsWereMet(), c.ten)
		if !c.hua {
			// Không hứa thì phải chỉ đường: người quản trị chỉ còn cách bấm tay.
			db, mock = newMockDB(t)
			svc = NewGameService(db, config.GameConfig{Role: config.GameRoleCafe})
			expectGameUpdate(mock, `UPDATE "games" SET`, chuaChuoi{"Tải ngay", true}, model.GameStatusError, anyTime{}, "g1")
			svc.failInstall(c.g, errors.New("lỗi bất kỳ"), time.Now())
			require.NoError(t, mock.ExpectationsWereMet(), c.ten)
		}
	}
}

func TestRetryDelay(t *testing.T) {
	assert.Equal(t, 5*time.Minute, retryDelay(1))
	assert.Equal(t, 10*time.Minute, retryDelay(2))
	assert.Equal(t, 20*time.Minute, retryDelay(3))
	assert.Equal(t, time.Hour, retryDelay(6))
	assert.Equal(t, time.Hour, retryDelay(1000), "không tràn số")
}

// Lỗi treo phải ghi thông báo tiếng Việt rõ ràng, kèm bao lâu sẽ thử lại.
func TestFailInstall_ThongBaoTreo(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe, StallTimeout: 10 * time.Minute})
	expectGameUpdate(mock, `UPDATE "games" SET "error"=\$1,"status"=\$2,"updated_at"=\$3 WHERE id = \$4`,
		sqlmock.AnyArg(), model.GameStatusError, anyTime{}, "g1")
	svc.failInstall(model.Game{ID: "g1", Name: "lol", RemoteInfohash: "h"}, gameupdate.ErrStalled, time.Now())
	require.NoError(t, mock.ExpectationsWereMet())
}

// Master publish bản mới trong lúc đang tải bản cũ thì hủy lượt tải cũ; cùng bản hoặc game khác thì không.
func TestSupersede(t *testing.T) {
	db, _ := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe})
	ctx, cancel := context.WithCancelCause(context.Background())
	svc.current = &installJob{name: "lol", infohash: "v4", cancel: cancel}

	svc.supersede("lol", "v4")
	svc.supersede("cs2", "v9")
	assert.NoError(t, ctx.Err(), "cùng bản hoặc game khác không hủy")

	svc.supersede("lol", "v5")
	require.Error(t, ctx.Err())
	assert.ErrorIs(t, context.Cause(ctx), errSuperseded)
}

// ---------------------------------------------------------------- phục hồi sau khi dừng giữa chừng

// Lần tải dở bị cắt không được thành "queued": queued là lệnh bấm tay và bỏ qua khung giờ.
func TestRecoverDownloading(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe})

	// Game đã có đúng bản của master (chỉ kiểm tra lại bị ngắt): dữ liệu còn trọn, giữ ready.
	expectGameUpdate(mock, `UPDATE "games" SET "status"=\$1,"updated_at"=\$2 WHERE status = \$3 AND infohash <> '' AND infohash = remote_infohash`,
		model.GameStatusReady, anyTime{}, model.GameStatusDownloading)
	// Còn lại là đang cập nhật dở: thư mục có thể đã bị ghi đè một phần, chưa chạy được.
	expectGameUpdate(mock, `UPDATE "games" SET "progress"=\$1,"status"=\$2,"updated_at"=\$3 WHERE status = \$4`,
		0, model.GameStatusNotInstalled, anyTime{}, model.GameStatusDownloading)
	svc.recoverDownloading()
	assert.NoError(t, mock.ExpectationsWereMet())
}

// publishedTorrent dựng một .torrent thật để loadPublished đọc được.
func publishedTorrent(t *testing.T, dir string) []byte {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.bin"), []byte("hello game"), 0o644))
	mi, err := gameupdate.BuildTorrent(dir, filepath.Base(dir))
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, mi.Write(&buf))
	return buf.Bytes()
}

// master tắt giữa lúc băm để lại "publishing" và không còn gì gỡ: game đã publish phải về "ready".
func TestRecoverPublishing(t *testing.T) {
	raw := publishedTorrent(t, filepath.Join(t.TempDir(), "Game"))
	cols := []string{"id", "name", "version", "status"}
	const publishingSQL = `SELECT \* FROM "games" WHERE status = \$1`

	t.Run("đã có bản publish và .torrent: về ready, xoá lỗi", func(t *testing.T) {
		db, mock := newMockDB(t)
		svc := NewGameService(db, config.GameConfig{Role: config.GameRoleMaster})
		mock.ExpectQuery(publishingSQL).WithArgs(model.GameStatusPublishing).
			WillReturnRows(sqlmock.NewRows(cols).AddRow("g1", "Game", 3, model.GameStatusPublishing))
		mock.ExpectQuery(`SELECT \* FROM "game_publishes" WHERE game_id = \$1 AND version = \$2`).WithArgs("g1", 3, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "game_id", "version", "torrent"}).AddRow("p1", "g1", 3, raw))
		expectGameUpdate(mock, `UPDATE "games" SET "error"=\$1,"status"=\$2,"updated_at"=\$3 WHERE id = \$4`,
			"", model.GameStatusReady, anyTime{}, "g1")
		svc.recoverPublishing()
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("chưa từng publish: not_installed để publishTick làm như thường", func(t *testing.T) {
		db, mock := newMockDB(t)
		svc := NewGameService(db, config.GameConfig{Role: config.GameRoleMaster})
		mock.ExpectQuery(publishingSQL).WithArgs(model.GameStatusPublishing).
			WillReturnRows(sqlmock.NewRows(cols).AddRow("g1", "Game", 0, model.GameStatusPublishing))
		expectGameUpdate(mock, `UPDATE "games" SET "error"=\$1,"status"=\$2,"updated_at"=\$3 WHERE id = \$4`,
			"", model.GameStatusNotInstalled, anyTime{}, "g1")
		svc.recoverPublishing()
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("thiếu .torrent: lỗi CÓ lời giải thích, không để error rỗng", func(t *testing.T) {
		db, mock := newMockDB(t)
		svc := NewGameService(db, config.GameConfig{Role: config.GameRoleMaster})
		mock.ExpectQuery(publishingSQL).WithArgs(model.GameStatusPublishing).
			WillReturnRows(sqlmock.NewRows(cols).AddRow("g1", "Game", 3, model.GameStatusPublishing))
		mock.ExpectQuery(`SELECT \* FROM "game_publishes"`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
		expectGameUpdate(mock, `UPDATE "games" SET "error"=\$1,"status"=\$2`,
			sqlmock.AnyArg(), model.GameStatusError, anyTime{}, "g1")
		svc.recoverPublishing()
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// Lỗi đọc thư mục tạm thời: thư mục trở lại như bản đã publish thì game phải về "ready".
func TestPublishTick_GoLoiTamThoi(t *testing.T) {
	root := t.TempDir()
	publishedTorrent(t, filepath.Join(root, "Game"))
	fp, err := gameupdate.Fingerprint(filepath.Join(root, "Game"))
	require.NoError(t, err)
	cols := []string{"id", "name", "enabled", "version", "fingerprint", "status", "error"}
	const unsetSQL = `UPDATE "games" SET "error"=\$1,"status"=\$2,"updated_at"=\$3 WHERE id = \$4 AND status = \$5`

	run := func(t *testing.T, version int, fingerprint, status string) error {
		db, mock := newMockDB(t)
		svc := NewGameService(db, config.GameConfig{Role: config.GameRoleMaster, Root: root, StableFor: time.Hour})
		mock.ExpectQuery(`SELECT \* FROM "games" WHERE enabled = \$1`).WithArgs(true).
			WillReturnRows(sqlmock.NewRows(cols).AddRow("g1", "Game", true, version, fingerprint, status, "không đọc được thư mục: x"))
		expectGameUpdate(mock, unsetSQL, "", model.GameStatusReady, anyTime{}, "g1", model.GameStatusError)
		svc.publishTick(context.Background(), time.Now())
		return mock.ExpectationsWereMet()
	}

	assert.NoError(t, run(t, 2, fp, model.GameStatusError), "trùng bản đã publish: gỡ lỗi")
	assert.Error(t, run(t, 2, "khac", model.GameStatusError), "thư mục đã đổi: để publish xử lý, không gỡ")
	assert.Error(t, run(t, 0, fp, model.GameStatusError), "chưa từng publish thì không có gì để về")
	assert.Error(t, run(t, 2, fp, model.GameStatusReady), "đang tốt thì không cần chạm vào")
}

func TestSetError_KhongBaoGioRong(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{})
	expectGameUpdate(mock, `UPDATE "games" SET "error"=\$1`,
		"lỗi không rõ nguyên nhân, xem nhật ký máy chủ", model.GameStatusError, anyTime{}, "g1")
	svc.setError("g1", "  ")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------- id không hợp lệ

func TestGameService_IdKhongPhaiUUID(t *testing.T) {
	db, mock := newMockDB(t)
	master := NewGameService(db, config.GameConfig{Role: config.GameRoleMaster})
	cafe := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe})

	_, err := master.Update("abc", &UpdateGameRequest{})
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	assert.ErrorIs(t, master.Delete("abc"), gorm.ErrRecordNotFound)
	_, err = master.PublishNow(context.Background(), "abc")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	assert.ErrorIs(t, cafe.SyncNow("abc"), gorm.ErrRecordNotFound)
	assert.NoError(t, mock.ExpectationsWereMet(), "không được chạm DB")
}

// SyncNow trên id hợp lệ nhưng không có game: 404, không phải thành công im lặng.
func TestSyncNow_IdKhongTonTai(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe})
	mock.ExpectQuery(`SELECT \* FROM "games" WHERE id = \$1`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	err := svc.SyncNow("11111111-1111-1111-1111-111111111111")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
	assert.NoError(t, mock.ExpectationsWereMet())
}
