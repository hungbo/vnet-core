package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
)

func TestShiftService_List(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "shifts"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT \* FROM "shifts" ORDER BY started_at desc LIMIT \$1`).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status", "started_at"}).
			AddRow("s1", testUserID, "open", testNow))

	result, err := svc.List(&pagination.Params{
		Page: 1, PageSize: 20, Sort: "started_at", Order: "desc",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestShiftService_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "shifts" WHERE id = \$1 ORDER BY "shifts"."id" LIMIT \$2`).
		WithArgs("s1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status"}).AddRow("s1", testUserID, "open"))

	result, err := svc.GetByID("s1")
	require.NoError(t, err)
	assert.Equal(t, "open", result.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestShiftService_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "shifts" WHERE id = \$1 ORDER BY "shifts"."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.GetByID("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, "shift not found", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestShiftService_OpenShift_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "shifts" WHERE user_id = \$1 AND status = \$2 ORDER BY "shifts"."id" LIMIT \$3`).
		WithArgs(testUserID, "open", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "shifts"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	result, err := svc.OpenShift(&OpenShiftRequest{OpeningBalance: 500000}, testUserID, testStoreID)
	require.NoError(t, err)
	assert.Equal(t, "open", result.Status)
	assert.Equal(t, int64(500000), result.OpeningBalance)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestShiftService_OpenShift_AlreadyOpen(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "shifts" WHERE user_id = \$1 AND status = \$2 ORDER BY "shifts"."id" LIMIT \$3`).
		WithArgs(testUserID, "open", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status"}).AddRow("s1", testUserID, "open"))

	_, err := svc.OpenShift(&OpenShiftRequest{}, testUserID, testStoreID)
	assert.Error(t, err)
	assert.Equal(t, "user already has an open shift", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestShiftService_CloseShift_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "shifts" WHERE id = \$1 ORDER BY "shifts"."id" LIMIT \$2`).
		WithArgs("s1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status", "opening_balance", "store_id", "started_at", "notes"}).
			AddRow("s1", testUserID, "open", int64(500000), &testStoreID, testNow, ""))

	mock.ExpectQuery(`SELECT COALESCE\(SUM\(final_amount\), 0\) FROM "orders"`).
		WillReturnRows(sqlmock.NewRows([]string{"coalesce"}).AddRow(int64(200000)))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "shifts" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT \* FROM "shifts" WHERE id = \$1 AND "shifts"."id" = \$2 ORDER BY "shifts"."id" LIMIT \$3`).
		WithArgs("s1", "s1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status", "opening_balance"}).
			AddRow("s1", testUserID, "closed", int64(500000)))

	result, err := svc.CloseShift("s1", &CloseShiftRequest{ClosingBalance: 800000})
	require.NoError(t, err)
	assert.Equal(t, "closed", result.Status)
	assert.NotNil(t, result.Discrepancy)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestShiftService_CloseShift_AlreadyClosed(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "shifts" WHERE id = \$1 ORDER BY "shifts"."id" LIMIT \$2`).
		WithArgs("s1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow("s1", "closed"))

	_, err := svc.CloseShift("s1", &CloseShiftRequest{ClosingBalance: 0})
	assert.Error(t, err)
	assert.Equal(t, "shift is already closed", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestShiftService_Handover_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "shifts" WHERE id = \$1 ORDER BY "shifts"."id" LIMIT \$2`).
		WithArgs("s1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id"}).AddRow("s1", testUserID))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "cash_handovers"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("h1"))
	mock.ExpectCommit()

	result, err := svc.Handover("s1", &HandoverRequest{
		Amount:       200000,
		HandoverType: "cash_out",
		Reason:       "Safe deposit",
	}, testUserID)
	require.NoError(t, err)

	m := result.(map[string]interface{})
	assert.Equal(t, "h1", m["id"])
	assert.Equal(t, "cash_out", m["handover_type"])
	assert.NoError(t, mock.ExpectationsWereMet())
}
