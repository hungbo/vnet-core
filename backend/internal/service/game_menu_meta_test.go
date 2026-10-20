package service

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/model"
)

func TestMenuVersion(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe})
	t1 := time.UnixMilli(1_700_000_000_123)
	t2 := time.UnixMilli(1_700_000_005_000)
	query := func(n int64, game, setting *time.Time) {
		mock.ExpectQuery(`SELECT COUNT\(\*\) AS n, MAX\(updated_at\) AS max FROM "games"`).
			WillReturnRows(sqlmock.NewRows([]string{"n", "max"}).AddRow(n, game))
		mock.ExpectQuery(`SELECT MAX\(updated_at\) AS max FROM "system_settings" WHERE group_name IN \(\$1,\$2\)`).WithArgs("games", "general").
			WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(setting))
	}
	query(3, &t1, &t2)
	v1, err := svc.MenuVersion()
	require.NoError(t, err)
	assert.Equal(t, "3.1700000000123.1700000005000", v1)

	query(3, &t1, &t2)
	v2, _ := svc.MenuVersion()
	assert.Equal(t, v1, v2, "không đổi gì thì version giữ nguyên")

	query(3, &t2, &t2)
	v3, _ := svc.MenuVersion()
	assert.NotEqual(t, v1, v3, "một game đổi")
	query(4, &t1, &t2)
	v4, _ := svc.MenuVersion()
	assert.NotEqual(t, v1, v4, "thêm game")
	query(3, &t1, &t1)
	v5, _ := svc.MenuVersion()
	assert.NotEqual(t, v1, v5, "cài đặt game đổi")

	query(0, nil, nil) // chưa có game, chưa có cài đặt: vẫn ra một chuỗi
	v6, err := svc.MenuVersion()
	require.NoError(t, err)
	assert.Equal(t, "0.0.0", v6)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHotGames(t *testing.T) {
	games := []model.Game{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}, {ID: "c", Name: "C"}, {ID: "d", Name: "D"},
		{ID: "e", Name: "E"}, {ID: "f", Name: "F"}, {ID: "g", Name: "G"}, {ID: "h", Name: "H"}}
	count := map[string]int64{"a": 50, "b": 30, "c": 30, "d": 4, "e": 9, "f": 8, "g": 7, "h": 6}
	hot := hotGames(games, count)
	assert.Len(t, hot, menuHotTop)
	for _, id := range []string{"a", "b", "c", "e", "f"} {
		assert.True(t, hot[id], id)
	}
	assert.False(t, hot["d"], "dưới ngưỡng tối thiểu")
	assert.False(t, hot["g"], "ngoài top")
	assert.Empty(t, hotGames(games, nil), "chưa có số liệu thì không nhãn nào")
	assert.Empty(t, hotGames(games, map[string]int64{"a": menuHotMin - 1}))
}

func TestMenuLabel(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	hot := map[string]bool{"h": true}
	assert.Equal(t, MenuLabelHot, menuLabel(&model.Game{ID: "h", CreatedAt: now}, hot, now), "Hot thắng New")
	assert.Equal(t, MenuLabelNew, menuLabel(&model.Game{ID: "n", CreatedAt: now.Add(-13 * 24 * time.Hour)}, hot, now))
	assert.Empty(t, menuLabel(&model.Game{ID: "o", CreatedAt: now.Add(-15 * 24 * time.Hour)}, hot, now))
	assert.Empty(t, menuLabel(&model.Game{ID: "z"}, hot, now), "game không có ngày tạo thì không gắn New")
}

func TestSortByLaunches(t *testing.T) {
	games := []model.Game{
		{ID: "1", Name: "A", Priority: 0}, {ID: "2", Name: "B", Priority: 0}, {ID: "3", Name: "C", Priority: 5},
		{ID: "4", Name: "D", Priority: 0}, {ID: "5", Name: "E", Priority: 0},
	}
	sortByLaunches(games, map[string]int64{"2": 10, "4": 10, "5": 3})
	var ids []string
	for _, g := range games {
		ids = append(ids, g.ID)
	}
	assert.Equal(t, []string{"3", "2", "4", "5", "1"}, ids, "ưu tiên cao trước, rồi nhiều lượt mở, hoà thì giữ thứ tự theo tên")

	same := []model.Game{{ID: "x"}, {ID: "y"}}
	sortByLaunches(same, nil)
	assert.Equal(t, "x", same[0].ID, "không có số liệu thì giữ nguyên")
}

func TestMenu_NhanHotNewVaThuTu(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe})
	now := time.Now()
	cols := []string{"id", "name", "display_name", "category", "launcher", "enabled", "status", "priority", "created_at"}
	mock.ExpectQuery(`SELECT \* FROM "games" ORDER BY priority DESC, display_name ASC`).WillReturnRows(sqlmock.NewRows(cols).
		AddRow("g1", "Alpha", "Alpha", "Online", "a.exe", true, model.GameStatusReady, 0, now.Add(-90*24*time.Hour)).
		AddRow("g2", "Beta", "Beta", "Online", "b.exe", true, model.GameStatusReady, 0, now.Add(-90*24*time.Hour)).
		AddRow("g3", "Gamma", "Gamma", "Online", "c.exe", true, model.GameStatusReady, 0, now.Add(-2*24*time.Hour)))
	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1`).WithArgs("games").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}))
	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1`).WithArgs("general").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}))
	mock.ExpectQuery(`SELECT game_id, COUNT\(\*\) AS n FROM "game_launches" WHERE created_at > \$1 GROUP BY "game_id"`).
		WillReturnRows(sqlmock.NewRows([]string{"game_id", "n"}).AddRow("g2", 40).AddRow("g1", 3))

	menu, err := svc.Menu()
	require.NoError(t, err)
	require.Len(t, menu.Items, 3)
	assert.Equal(t, []string{"Beta", "Alpha", "Gamma"}, []string{menu.Items[0].Name, menu.Items[1].Name, menu.Items[2].Name},
		"Beta nhiều lượt mở nhất lên đầu; Gamma không có lượt nào")
	assert.Equal(t, MenuLabelHot, menu.Items[0].Label)
	assert.Empty(t, menu.Items[1].Label, "Alpha có 3 lượt, dưới ngưỡng Hot và không mới")
	assert.Equal(t, MenuLabelNew, menu.Items[2].Label, "Gamma mới về quán 2 ngày")
	assert.NoError(t, mock.ExpectationsWereMet())

	// lần hỏi sau trong hạn: không đếm lại bảng game_launches
	mock.ExpectQuery(`SELECT \* FROM "games" ORDER BY priority DESC, display_name ASC`).WillReturnRows(sqlmock.NewRows(cols).
		AddRow("g1", "Alpha", "Alpha", "Online", "a.exe", true, model.GameStatusReady, 0, now))
	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1`).WithArgs("games").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}))
	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1`).WithArgs("general").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}))
	_, err = svc.Menu()
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet(), "dùng lại xếp hạng đã đếm")
}

func TestMenu_LoiDemLuotKhongLamHongMenu(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleCafe})
	mock.ExpectQuery(`SELECT \* FROM "games"`).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "display_name", "launcher", "enabled", "status"}).
		AddRow("g1", "A", "A", "a.exe", true, model.GameStatusReady))
	mock.ExpectQuery(`SELECT \* FROM "system_settings"`).WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}))
	mock.ExpectQuery(`SELECT game_id`).WillReturnError(assert.AnError)
	menu, err := svc.Menu()
	require.NoError(t, err)
	require.Len(t, menu.Items, 1)
	assert.Empty(t, menu.Items[0].Label)
}
