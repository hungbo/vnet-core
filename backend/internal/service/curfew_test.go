package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCurfewService_List(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewCurfewService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "curfew_policies"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT \* FROM "curfew_policies" ORDER BY created_at desc LIMIT \$1`).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "day_of_week", "curfew_start", "curfew_end"}).
			AddRow("c1", 6, "22:00", "06:00"))

	result, err := svc.List(&CurfewListRequest{Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCurfewService_List_FilterByDay(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewCurfewService(db, NewAuditService(db))

	day := 6
	mock.ExpectQuery(`SELECT count\(\*\) FROM "curfew_policies" WHERE day_of_week = \$1`).
		WithArgs(6).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT \* FROM "curfew_policies" WHERE day_of_week = \$1 ORDER BY created_at desc LIMIT \$2`).
		WithArgs(6, 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "day_of_week", "curfew_start", "curfew_end"}).
			AddRow("c1", 6, "22:00", "06:00"))

	result, err := svc.List(&CurfewListRequest{DayOfWeek: &day, Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCurfewService_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewCurfewService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "curfew_policies" WHERE id = \$1 ORDER BY "curfew_policies"."id" LIMIT \$2`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "day_of_week"}).AddRow("c1", 6))

	result, err := svc.GetByID("c1")
	require.NoError(t, err)
	assert.Equal(t, 6, result.DayOfWeek)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCurfewService_Create(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewCurfewService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "curfew_policies"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	result, err := svc.Create(&CreateCurfewRequest{
		DayOfWeek:   6,
		CurfewStart: "22:00",
		CurfewEnd:   "06:00",
	})
	require.NoError(t, err)
	assert.Equal(t, "22:00", result.CurfewStart)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCurfewService_Update_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewCurfewService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "curfew_policies" WHERE id = \$1 ORDER BY "curfew_policies"."id" LIMIT \$2`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "curfew_start"}).AddRow("c1", "22:00"))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "curfew_policies" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT \* FROM "curfew_policies" WHERE id = \$1 AND "curfew_policies"."id" = \$2 ORDER BY "curfew_policies"."id" LIMIT \$3`).
		WithArgs("c1", "c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "curfew_start"}).AddRow("c1", "23:00"))

	_, err := svc.Update("c1", &UpdateCurfewRequest{CurfewStart: "23:00"})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCurfewService_Delete_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewCurfewService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "curfew_policies" WHERE id = \$1 ORDER BY "curfew_policies"."id" LIMIT \$2`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("c1"))

	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "curfew_policies" WHERE "curfew_policies"."id" = \$1`).
		WithArgs("c1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.Delete("c1")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCurfewService_Override_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewCurfewService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "curfew_policies" WHERE id = \$1 ORDER BY "curfew_policies"."id" LIMIT \$2`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "day_of_week"}).AddRow("c1", 6))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "curfew_policies" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result, err := svc.Override(&OverrideCurfewRequest{
		PolicyID:       "c1",
		OverrideReason: "Special event",
	}, testUserID)
	require.NoError(t, err)
	assert.Equal(t, "Special event", result.OverrideReason)
	assert.Equal(t, testUserID, *result.OverrideByAdmin)
	assert.NoError(t, mock.ExpectationsWereMet())
}
