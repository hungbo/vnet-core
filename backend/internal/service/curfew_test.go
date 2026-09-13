package service

import (
	"github.com/vnet/core/internal/model"
	"testing"
	"time"

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

func TestIsMinor(t *testing.T) {
	at := time.Date(2026, 8, 21, 22, 0, 0, 0, time.UTC)

	born2010 := time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC)
	born2000 := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	// Turns 18 tomorrow, so still a minor tonight.
	almost := time.Date(2008, 8, 22, 0, 0, 0, 0, time.UTC)

	assert.True(t, IsMinor(&model.Member{DateOfBirth: &born2010}, at))
	assert.False(t, IsMinor(&model.Member{DateOfBirth: &born2000}, at))
	assert.True(t, IsMinor(&model.Member{DateOfBirth: &almost}, at))

	// An unknown birthday must not lock the account out of the system.
	assert.False(t, IsMinor(&model.Member{}, at))
	assert.False(t, IsMinor(nil, at))
}

func TestWithinCurfew(t *testing.T) {
	// Same-day window.
	assert.True(t, withinCurfew("14:00:00", "12:00:00", "18:00:00"))
	assert.False(t, withinCurfew("19:00:00", "12:00:00", "18:00:00"))

	// Window that wraps past midnight — the common case for a curfew.
	assert.True(t, withinCurfew("23:30:00", "22:00:00", "06:00:00"))
	assert.True(t, withinCurfew("02:00:00", "22:00:00", "06:00:00"))
	assert.False(t, withinCurfew("12:00:00", "22:00:00", "06:00:00"))

	// End is exclusive.
	assert.False(t, withinCurfew("06:00:00", "22:00:00", "06:00:00"))

	assert.False(t, withinCurfew("12:00:00", "", ""))
}

func TestCurfewService_CheckStart_BlocksMinorDuringCurfew(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewCurfewService(db, NewAuditService(db))

	at := time.Date(2026, 8, 21, 23, 0, 0, 0, time.UTC) // Friday 23:00
	born := time.Date(2012, 1, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`SELECT \* FROM "curfew_policies" WHERE is_active = \$1 AND day_of_week = \$2`).
		WithArgs(true, int(at.Weekday())).
		WillReturnRows(sqlmock.NewRows([]string{"id", "curfew_start", "curfew_end", "is_active"}).
			AddRow("c1", "22:00:00", "06:00:00", true))

	err := svc.CheckStart(&model.Member{DateOfBirth: &born}, at)

	require.Error(t, err)
	// Câu này hiện thẳng lên màn hình khoá của khách nên phải là tiếng Việt,
	// và chỉ nêu giờ-phút chứ không kèm giây như cột time lưu trong database.
	assert.Contains(t, err.Error(), "khung giờ cấm")
	assert.Contains(t, err.Error(), "vị thành niên")
	assert.NotContains(t, err.Error(), ":00:00", "không hiện phần giây cho người đọc")
}

func TestCurfewService_CheckStart_AllowsAdult(t *testing.T) {
	db, _ := newMockDB(t)
	svc := NewCurfewService(db, NewAuditService(db))

	at := time.Date(2026, 8, 21, 23, 0, 0, 0, time.UTC)
	born := time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)

	// No policy lookup happens at all for an adult.
	assert.NoError(t, svc.CheckStart(&model.Member{DateOfBirth: &born}, at))
}
