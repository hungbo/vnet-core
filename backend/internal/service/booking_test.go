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

	mock.ExpectQuery(`SELECT count\(\*\) FROM "machine_bookings" WHERE deleted_at IS NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT \* FROM "machine_bookings" WHERE deleted_at IS NULL ORDER BY created_at desc LIMIT \$1`).
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
	mock.ExpectQuery(`SELECT \* FROM "machine_bookings" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "machine_bookings"."id" LIMIT \$2`).
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
	}, "s1", "u1")
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
	}, "s1", "u1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid booked_from")
	assert.NoError(t, mock.ExpectationsWereMet())
}
