package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Chỉ được mở MỘT phiên kiểm kê tại một thời điểm.
//
// Lúc chốt, hệ thống áp CHÊNH LỆCH đã ghi lên tồn kho hiện tại — cố ý như vậy
// để hàng bán ra giữa lúc đếm và lúc chốt không bị xoá mất. Nhưng nếu hai phiên
// cùng mở, hai người cùng đếm ra 30 trong khi sổ ghi 35 thì mỗi phiên ghi lệch
// -5, và chốt cả hai trừ 5 hai lần: 35 → 30 → 25 trong khi kho thật có 30.
// Dựng lại được trên hệ thống thật bằng đúng chuỗi đó.
func TestInventoryCountService_Open_ChiMotPhienMoTaiMotThoiDiem(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewInventoryCountService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT \* FROM "inventory_count_sessions" WHERE status = \$1`).
		WithArgs(CountStatusOpen, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "status"}).
			AddRow("s1", "KK-00002", CountStatusOpen))
	mock.ExpectRollback()

	_, err := svc.Open(&OpenCountRequest{Note: "phiên 3"}, "user-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "KK-00002")
	assert.Contains(t, err.Error(), "đang mở")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Không còn phiên nào mở thì vẫn mở được bình thường.
func TestInventoryCountService_Open_KhongConPhienMoThiChoQua(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewInventoryCountService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT \* FROM "inventory_count_sessions" WHERE status = \$1`).
		WithArgs(CountStatusOpen, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "status"}))
	mock.ExpectQuery(`SELECT \* FROM "inventory_count_sessions" WHERE code LIKE \$1`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code"}).AddRow("s1", "KK-00007"))
	mock.ExpectQuery(`INSERT INTO "inventory_count_sessions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	phien, err := svc.Open(&OpenCountRequest{Note: "phiên mới"}, "user-1")

	require.NoError(t, err)
	assert.Equal(t, "KK-00008", phien.Code, "mã phiên phải tăng tiếp theo mã lớn nhất")
	assert.NoError(t, mock.ExpectationsWereMet())
}
