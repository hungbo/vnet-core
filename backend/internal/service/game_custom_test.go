package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/model"
	"gorm.io/gorm"
)

func steps(l ...string) []model.GameRunStep {
	var out []model.GameRunStep
	for _, x := range l {
		out = append(out, model.GameRunStep{Launcher: x})
	}
	return out
}

func TestChuanHoaRunFirst(t *testing.T) {
	got, err := chuanHoaRunFirst([]model.GameRunStep{{Launcher: "  tools/setup.bat ", LaunchArgs: " /silent "}, {Launcher: "svc.exe"}})
	require.NoError(t, err)
	assert.Equal(t, model.GameRunSteps{{Launcher: "tools/setup.bat", LaunchArgs: "/silent"}, {Launcher: "svc.exe"}}, got)

	got, err = chuanHoaRunFirst(nil)
	require.NoError(t, err)
	assert.Nil(t, got, "rỗng là không có lệnh nào")

	for name, in := range map[string][]model.GameRunStep{
		"thiếu tệp chạy":      {{LaunchArgs: "/x"}},
		"đường dẫn tuyệt đối": steps(`C:\Windows\notepad.exe`),
		"lên thư mục trên":    steps("../x.exe"),
		"ký tự điều khiển":    {{Launcher: "a.exe", LaunchArgs: "a\x00b"}},
		"tham số quá dài":     {{Launcher: "a.exe", LaunchArgs: strings.Repeat("a", 501)}},
		"quá nhiều bước":      steps("1.exe", "2.exe", "3.exe", "4.exe", "5.exe", "6.exe"),
	} {
		_, err := chuanHoaRunFirst(in)
		assert.Error(t, err, name)
	}
	_, err = chuanHoaRunFirst(steps("1.exe", "2.exe", "3.exe", "4.exe", "5.exe"))
	assert.NoError(t, err, "đúng 5 bước là được")
}

func TestRunFirstHopLe(t *testing.T) {
	in := append(steps("ok1.exe", `C:\bad.exe`, "../bad2.exe", "ok2.exe"), model.GameRunStep{})
	got := runFirstHopLe("g", in)
	assert.Equal(t, model.GameRunSteps{{Launcher: "ok1.exe"}, {Launcher: "ok2.exe"}}, got, "bước hỏng bị bỏ riêng, không bỏ cả game")
	assert.Len(t, runFirstHopLe("g", steps("1.exe", "2.exe", "3.exe", "4.exe", "5.exe", "6.exe", "7.exe")), 5)
	assert.Nil(t, runFirstHopLe("g", nil))
}

func TestRunFirstOf(t *testing.T) {
	base := model.GameRunSteps{{Launcher: "a.exe"}}
	own := model.GameRunSteps{{Launcher: "b.exe"}}
	none := model.GameRunSteps{}
	assert.Equal(t, base, runFirstOf(&model.Game{RunFirst: base}), "không tuỳ chỉnh: theo game gốc")
	assert.Equal(t, own, runFirstOf(&model.Game{RunFirst: base, Custom: model.GameCustom{RunFirst: &own}}))
	assert.Empty(t, runFirstOf(&model.Game{RunFirst: base, Custom: model.GameCustom{RunFirst: &none}}), "có mặt nhưng rỗng: bỏ hết lệnh của game gốc")
}

func TestGameCustom_JSONVaCot(t *testing.T) {
	var zero model.GameCustom
	v, err := zero.Value()
	require.NoError(t, err)
	assert.Nil(t, v, "chưa tuỳ chỉnh gì thì lưu NULL")

	empty := model.GameRunSteps{}
	c := model.GameCustom{DisplayName: "Tên", ClientRoot: `E:\G`, RunFirst: &empty}
	v, err = c.Value()
	require.NoError(t, err)
	var back model.GameCustom
	require.NoError(t, back.Scan(v))
	require.NotNil(t, back.RunFirst, "RunFirst rỗng nhưng có mặt phải sống sót qua cột")
	assert.Empty(t, *back.RunFirst)
	assert.Equal(t, "Tên", back.DisplayName)

	require.NoError(t, back.Scan(nil))
	assert.True(t, back.IsZero())
	require.NoError(t, back.Scan([]byte("null")))
	assert.True(t, back.IsZero())
	assert.Error(t, back.Scan(42))
}

func TestSetCustom(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	cafe := func() (*GameService, sqlmock.Sqlmock) { return cafeWithRoot(t, t.TempDir()) }
	row := func(custom interface{}) *sqlmock.Rows {
		return sqlmock.NewRows([]string{"id", "name", "display_name", "origin", "custom"}).AddRow(id, "g", "Gốc", model.GameOriginHub, custom)
	}
	str := func(s string) *string { return &s }

	t.Run("master không có tuỳ chỉnh riêng", func(t *testing.T) {
		svc := NewGameService(nil, config.GameConfig{Role: config.GameRoleMaster})
		_, err := svc.SetCustom(id, &GameCustomRequest{})
		assert.ErrorIs(t, err, ErrGameCafeOnly)
	})
	t.Run("id lạ không chạm DB", func(t *testing.T) {
		svc, mock := cafe()
		_, err := svc.SetCustom("abc", &GameCustomRequest{})
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("đặt tên, icon, gốc và lệnh chạy trước", func(t *testing.T) {
		svc, mock := cafe()
		mock.ExpectQuery(gameByIDSQL).WillReturnRows(row(nil))
		var saved string
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "games" SET "custom"=\$1,"updated_at"=\$2 WHERE "id" = \$3`).
			WithArgs(sqlmock.AnyArg(), anyTime{}, id).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		mock.ExpectQuery(gameByIDSQL).WillReturnRows(row(`{"display_name":"Tên quán"}`))
		_ = saved

		g, err := svc.SetCustom(id, &GameCustomRequest{
			DisplayName: str(" Tên quán "), Icon: str("Bin/x.exe"), ClientRoot: str("e:/Games2/"),
			RunFirst: &[]model.GameRunStep{{Launcher: "tools/setup.bat", LaunchArgs: "/s"}},
		})
		require.NoError(t, err)
		assert.Equal(t, "Tên quán", g.Custom.DisplayName)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("lỗi dừng trước khi ghi", func(t *testing.T) {
		for name, req := range map[string]*GameCustomRequest{
			"gốc sai":             {ClientRoot: str(`Games`)},
			"gốc UNC":             {ClientRoot: str(`\\may\chia-se`)},
			"icon tuyệt đối":      {Icon: str(`C:\x.ico`)},
			"tên có điều khiển":   {DisplayName: str("a\x00b")},
			"lệnh chạy trước sai": {RunFirst: &[]model.GameRunStep{{Launcher: `C:\x.exe`}}},
		} {
			svc, mock := cafe()
			mock.ExpectQuery(gameByIDSQL).WillReturnRows(row(nil))
			_, err := svc.SetCustom(id, req)
			assert.Error(t, err, name)
			assert.NoError(t, mock.ExpectationsWereMet(), name+": không được UPDATE")
		}
	})
	t.Run("reset lệnh chạy trước và xoá tên", func(t *testing.T) {
		svc, mock := cafe()
		mock.ExpectQuery(gameByIDSQL).WillReturnRows(row(`{"display_name":"Cũ","run_first":[]}`))
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "games" SET "custom"=\$1`).WithArgs(nil, anyTime{}, id).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		mock.ExpectQuery(gameByIDSQL).WillReturnRows(row(nil))
		_, err := svc.SetCustom(id, &GameCustomRequest{DisplayName: str(""), ResetRunFirst: true})
		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet(), "không còn tuỳ chỉnh gì thì cột về NULL")
	})
}

func TestMenu_TuyChinhCuaQuan(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe})
	cols := []string{"name", "display_name", "category", "launcher", "launch_args", "icon", "menu_entries", "enabled", "version", "status", "run_first", "custom"}
	mock.ExpectQuery(`SELECT \* FROM "games" ORDER BY priority DESC, display_name ASC`).
		WillReturnRows(sqlmock.NewRows(cols).
			// game của Hub, quán không tuỳ chỉnh: lệnh chạy trước theo game gốc, không có gốc riêng
			AddRow("A", "Game A", "Online", "a.exe", "", "a.ico", nil, true, 1, model.GameStatusReady, `[{"launcher":"pre/a.bat","launch_args":"/x"}]`, nil).
			// quán đặt tên, icon, gốc và thay lệnh chạy trước
			AddRow("B", "Game B", "Online", "b.exe", "", "b.ico", nil, true, 1, model.GameStatusReady, `[{"launcher":"pre/b.bat"}]`,
				`{"display_name":"B của quán","icon":"my.ico","client_root":"E:\\Games2","run_first":[{"launcher":"own.exe"}]}`).
			// quán bỏ hết lệnh chạy trước của game gốc
			AddRow("C", "Game C", "Online", "c.exe", "", "", nil, true, 1, model.GameStatusReady, `[{"launcher":"pre/c.bat"}]`, `{"run_first":[]}`).
			// game nhiều mục: tên và icon của từng mục giữ nguyên, gốc và lệnh chạy trước theo game
			AddRow("D", "Game D", "Ứng dụng", "", "", "", `[{"display_name":"Word","launcher":"w.exe","icon":"w.ico"}]`, true, 1, model.GameStatusReady, nil,
				`{"display_name":"Bỏ qua","icon":"bo-qua.ico","client_root":"F:\\Apps"}`))
	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1`).WithArgs("games").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}))

	menu, err := svc.Menu()
	require.NoError(t, err)
	by := map[string]GameMenuItem{}
	for _, it := range menu.Items {
		by[it.Key] = it
	}
	require.Len(t, menu.Items, 4)

	a := by["A"]
	assert.Equal(t, "Game A", a.DisplayName)
	assert.Equal(t, "a.ico", a.Icon)
	assert.Empty(t, a.Root, "không có gốc riêng thì dùng gốc chung của menu")
	assert.Equal(t, model.GameRunSteps{{Launcher: "pre/a.bat", LaunchArgs: "/x"}}, a.RunFirst)

	b := by["B"]
	assert.Equal(t, "B của quán", b.DisplayName)
	assert.Equal(t, "my.ico", b.Icon)
	assert.Equal(t, `E:\Games2`, b.Root)
	assert.Equal(t, model.GameRunSteps{{Launcher: "own.exe"}}, b.RunFirst)

	assert.Empty(t, by["C"].RunFirst, "quán bỏ hết lệnh chạy trước")

	d := by["D/1"]
	assert.Equal(t, "Word", d.DisplayName, "tên của mục menu không bị tên tuỳ chỉnh của game đè")
	assert.Equal(t, "w.ico", d.Icon)
	assert.Equal(t, `F:\Apps`, d.Root)
	assert.NoError(t, mock.ExpectationsWereMet())

	raw, err := json.Marshal(a)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), `"root"`, "omitempty: máy trạm cũ thấy đúng hình dạng cũ")
}

func TestCatalog_CoRunFirst(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleMaster})
	mock.ExpectQuery(`SELECT \* FROM "games" WHERE enabled = \$1 AND version > 0`).
		WillReturnRows(sqlmock.NewRows([]string{"name", "display_name", "run_first"}).
			AddRow("A", "A", `[{"launcher":"pre.bat"}]`).AddRow("B", "B", nil))
	items, err := svc.Catalog()
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, model.GameRunSteps{{Launcher: "pre.bat"}}, items[0].RunFirst)
	raw, _ := json.Marshal(items[1])
	assert.NotContains(t, string(raw), "run_first", "catalog của game thường giữ nguyên như trước")
}

func TestCreateLocal_VoiRunFirst(t *testing.T) {
	root := t.TempDir()
	writeEmpty(t, root, "Game/Bin/game.exe", "Game/pre/setup.bat")
	svc, _ := cafeWithRoot(t, root)
	_, err := svc.CreateLocal(&CreateGameRequest{Name: "Game", Launcher: "Bin/game.exe", RunFirst: steps(`C:\x.exe`)})
	assert.Error(t, err, "bước chạy trước sai thì từ chối trước khi chạm DB")
}
