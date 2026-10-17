package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Khoá menu "<thư mục>/<i>" (mục riêng như Excel của LibreOffice) vẫn tính cho
// game theo tên thư mục, và giữ nguyên khoá để sau đếm được từng mục.
func TestGameService_RecordLaunch(t *testing.T) {
	db, mock := newMockDB(t)
	svc := &GameService{db: db}

	mock.ExpectQuery(`SELECT "id" FROM "games" WHERE name = \$1 ORDER BY "games"\."id" LIMIT \$2`).
		WithArgs("LibreOffice", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("g1"))
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "game_launches" \("game_id","menu_key","member_id","machine_code"\)`).
		WithArgs("g1", "LibreOffice/1", "m1", "MAY06").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("l1"))
	mock.ExpectCommit()

	err := svc.RecordLaunch(&GameLaunchRequest{Key: "LibreOffice/1", MachineCode: "MAY06"}, "m1")

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGameService_RecordLaunch_GameKhongCon(t *testing.T) {
	db, mock := newMockDB(t)
	svc := &GameService{db: db}

	mock.ExpectQuery(`SELECT "id" FROM "games" WHERE name = \$1`).
		WithArgs("DaXoa", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	err := svc.RecordLaunch(&GameLaunchRequest{Key: "DaXoa"}, "")

	assert.ErrorIs(t, err, ErrGameNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Trang Game nhận số lượt chơi của từng game; game chưa ai chơi là 0.
func TestGameService_List_DemLuotChoi(t *testing.T) {
	db, mock := newMockDB(t)
	svc := &GameService{db: db}

	mock.ExpectQuery(`SELECT \* FROM "games" ORDER BY priority DESC, display_name ASC`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "status"}).
			AddRow("g1", "LOL", "ready").
			AddRow("g2", "CS2", "ready"))
	mock.ExpectQuery(`SELECT game_id, COUNT\(\*\) as n FROM "game_launches" GROUP BY "game_id"`).
		WillReturnRows(sqlmock.NewRows([]string{"game_id", "n"}).AddRow("g1", int64(42)))

	games, err := svc.List()

	require.NoError(t, err)
	require.Len(t, games, 2)
	assert.Equal(t, int64(42), games[0].LaunchCount)
	assert.Equal(t, int64(0), games[1].LaunchCount)
	assert.NoError(t, mock.ExpectationsWereMet())
}
