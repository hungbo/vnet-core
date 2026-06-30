package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/hub"
	"gorm.io/gorm"
)

func TestSessionService_GetSession_Found(t *testing.T) {
	db, mock := newMockDB(t)
	wsHub := hub.New()
	svc := NewSessionService(db, wsHub, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machine_sessions" WHERE id = \$1 ORDER BY "machine_sessions"."id" LIMIT \$2`).
		WithArgs("s1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_id", "member_id", "is_active", "started_at", "created_at"}).
			AddRow("s1", "m1", "mem1", true, testNow, testNow))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 ORDER BY "machines"."id" LIMIT \$2`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code"}).AddRow("m1", "M-001"))

	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 ORDER BY "members"."id" LIMIT \$2`).
		WithArgs("mem1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "full_name"}).AddRow("mem1", "Test Member"))

	result, err := svc.GetSession("s1")
	require.NoError(t, err)
	assert.Equal(t, "m1", result.MachineID)
	assert.Equal(t, "M-001", result.MachineCode)
	assert.Equal(t, "Test Member", result.MemberName)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionService_GetSession_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	wsHub := hub.New()
	svc := NewSessionService(db, wsHub, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machine_sessions" WHERE id = \$1 ORDER BY "machine_sessions"."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.GetSession("nonexistent")
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionService_GetActiveSessions(t *testing.T) {
	db, mock := newMockDB(t)
	wsHub := hub.New()
	svc := NewSessionService(db, wsHub, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machine_sessions" WHERE is_active = \$1 AND store_id = \$2`).
		WithArgs(true, "s1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_id", "member_id", "is_active", "started_at", "created_at"}).
			AddRow("s1", "m1", "mem1", true, testNow, testNow))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 ORDER BY "machines"."id" LIMIT \$2`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code"}).AddRow("m1", "M-001"))

	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 ORDER BY "members"."id" LIMIT \$2`).
		WithArgs("mem1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "full_name"}).AddRow("mem1", "Test Member"))

	result, err := svc.GetActiveSessions()
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "M-001", result[0].MachineCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}


