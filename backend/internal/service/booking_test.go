package service

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testRFC3339Time = "2026-06-25T10:00:00+07:00"

func TestBookingService_List(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "machine_bookings" WHERE "machine_bookings"\."deleted_at" IS NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT \* FROM "machine_bookings" WHERE "machine_bookings"\."deleted_at" IS NULL ORDER BY created_at desc LIMIT \$1`).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "customer_name", "status"}).
			AddRow("b1", "John", "pending"))

	result, err := svc.List(&BookingListRequest{Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.Len(t, result.Items, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBookingService_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	now := time.Now()
	mock.ExpectQuery(`SELECT \* FROM "machine_bookings" WHERE id = \$1 AND "machine_bookings"\."deleted_at" IS NULL ORDER BY "machine_bookings"."id" LIMIT \$2`).
		WithArgs("b1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "customer_name", "booked_from", "booked_to", "created_at", "updated_at"}).
			AddRow("b1", "John", now, now, now, now))

	result, err := svc.GetByID("b1")
	require.NoError(t, err)
	assert.Equal(t, "John", result.CustomerName)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBookingService_Create_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "machine_bookings"`).
		WithArgs("m1", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "machine_bookings"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	result, err := svc.Create(&CreateBookingRequest{
		MachineID:     "m1",
		CustomerName:  "John",
		CustomerPhone: "0123456789",
		BookedFrom:    testRFC3339Time,
		BookedTo:      "2026-06-25T12:00:00+07:00",
	}, "u1")
	require.NoError(t, err)
	assert.Equal(t, "John", result.CustomerName)
	assert.Equal(t, "pending", result.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBookingService_Create_InvalidTimeFormat(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	_, err := svc.Create(&CreateBookingRequest{
		MachineID:     "m1",
		CustomerName:  "John",
		CustomerPhone: "0123456789",
		BookedFrom:    "invalid-time",
		BookedTo:      "2026-06-25T12:00:00+07:00",
	}, "u1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid booked_from")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// A deposit has to leave the member's balance when the booking is made.
// It used to be recorded on the row only, while cancelling paid it out.
func TestBookingService_Create_ChargesDeposit(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "machine_bookings"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$2 FOR UPDATE`).
		WithArgs("mem-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance"}).AddRow("mem-1", int64(500000)))
	mock.ExpectQuery(`INSERT INTO "member_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("dep-1"))
	mock.ExpectExec(`UPDATE "members" SET "balance"=\$1,"updated_at"=\$2 WHERE "members"\."deleted_at" IS NULL AND "id" = \$3`).
		WithArgs(int64(400000), anyTime{}, "mem-1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`INSERT INTO "machine_bookings"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("bk-1"))
	mock.ExpectCommit()

	_, err := svc.Create(&CreateBookingRequest{
		MachineID:     "m1",
		MemberID:      "mem-1",
		BookedFrom:    "2026-09-01T10:00:00+07:00",
		BookedTo:      "2026-09-01T12:00:00+07:00",
		DepositAmount: 100000,
	}, "u1")

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBookingService_Create_RejectsDepositBeyondBalance(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "machine_bookings"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$2 FOR UPDATE`).
		WithArgs("mem-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance"}).AddRow("mem-1", int64(10)))
	mock.ExpectRollback()

	_, err := svc.Create(&CreateBookingRequest{
		MachineID:     "m1",
		MemberID:      "mem-1",
		BookedFrom:    "2026-09-01T10:00:00+07:00",
		BookedTo:      "2026-09-01T12:00:00+07:00",
		DepositAmount: 100000,
	}, "u1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient balance")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Cancelling a booking whose deposit was never charged must not pay anything out.
func TestBookingService_Cancel_NoRefundWithoutCharge(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machine_bookings" WHERE id = \$1 AND "machine_bookings"\."deleted_at" IS NULL ORDER BY "machine_bookings"\."id" LIMIT \$2`).
		WithArgs("bk-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "deposit_amount", "member_id", "deposit_transaction_id"}).
			AddRow("bk-1", "pending", int64(100000), "mem-1", nil))

	mock.ExpectBegin()
	// Cancel đọc lại lịch đặt dưới khoá trong chính transaction: bản đọc ở trên
	// nằm ngoài, nên hai lệnh huỷ song song đều hoàn cọc nếu không có bước này.
	mock.ExpectQuery(`SELECT \* FROM "machine_bookings" WHERE id = \$1 AND "machine_bookings"\."deleted_at" IS NULL ORDER BY "machine_bookings"\."id" LIMIT \$2 FOR UPDATE`).
		WithArgs("bk-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "deposit_amount", "member_id", "deposit_transaction_id"}).
			AddRow("bk-1", "pending", int64(100000), "mem-1", nil))
	mock.ExpectExec(`UPDATE "machine_bookings" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	_, err := svc.Cancel("bk-1")

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
