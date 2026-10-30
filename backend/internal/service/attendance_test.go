package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Bấm tiêu đề cột trên trang Điểm danh thì sắp theo cột đó trên toàn bộ lượt,
// lượt mới nhất làm tiêu chí phụ; không bấm thì vẫn mới nhất trước.
func TestAttendanceService_List_SortedByColumn(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewAttendanceService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "member_attendances"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(25))
	mock.ExpectQuery(`SELECT \* FROM "member_attendances" ORDER BY streak_days asc, checkin_at desc LIMIT \$1 OFFSET \$2`).
		WithArgs(20, 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "member_id", "streak_days"}))

	res, err := svc.List(&AttendanceListRequest{Page: 2, PageSize: 20, Sort: "streak_days", Order: "asc"})
	require.NoError(t, err)
	assert.Equal(t, int64(25), res.Total)
	assert.NoError(t, mock.ExpectationsWereMet())
}
