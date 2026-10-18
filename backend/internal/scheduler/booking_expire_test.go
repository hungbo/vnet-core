package scheduler

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Đánh vắng / hoàn tất hàng loạt phải lấy lại id (RETURNING) để báo
// booking:updated cho từng lịch — bản cũ đổi trạng thái mà không báo gì.
func TestExpireBookings_TraVeIDDeBaoSuKien(t *testing.T) {
	db, mock := newMockDB(t)

	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE "machine_bookings" SET "status"=\$1,"updated_at"=\$2 WHERE \(status = \$3 AND booked_from < \$4\) AND "machine_bookings"\."deleted_at" IS NULL RETURNING "id"`).
		WithArgs("no_show", sqlmock.AnyArg(), "pending", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("b1").AddRow("b2"))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE "machine_bookings" SET "status"=\$1,"updated_at"=\$2 WHERE \(status = \$3 AND booked_to < \$4\) AND "machine_bookings"\."deleted_at" IS NULL RETURNING "id"`).
		WithArgs("completed", sqlmock.AnyArg(), "checked_in", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectCommit()

	// Hub nil: PhatLichDatDoi bỏ qua, chỉ kiểm câu SQL và không lỗi.
	require.NoError(t, expireBookings(db, nil))
	assert.NoError(t, mock.ExpectationsWereMet())
}
