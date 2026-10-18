package scheduler

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()

	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: mockDB}), &gorm.Config{})
	require.NoError(t, err)

	return db, mock
}

// Máy đang có khách ngồi chơi thì không được đặt về "offline".
//
// Tác vụ này chỉ nhìn nhịp tim. Máy trạm treo hoặc rớt mạng giữa phiên là
// chuyện thường, và khi đó nó ghi đè status "in_use" thành "offline": quầy thấy
// máy trống trong khi khách vẫn đang chơi, và chốt chặn mở phiên trùng máy —
// vốn đọc chính cột status ấy — không còn nổ nữa. Thực tế đã dựng lại được cảnh
// hai phiên cùng chạy trên một máy vì đúng chuỗi này.
func TestMarkStaleMachinesOffline_BoQuaMayDangCoPhien(t *testing.T) {
	db, mock := newMockDB(t)

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE .*id NOT IN \(SELECT machine_id FROM machine_sessions WHERE is_active = true\)`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code"}))

	require.NoError(t, markStaleMachinesOffline(db, nil))
	assert.NoError(t, mock.ExpectationsWereMet())
}
