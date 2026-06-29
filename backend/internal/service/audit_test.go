package service

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/pkg/pagination"
)

func TestAuditService_List(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewAuditService(db)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "audit_logs" LEFT JOIN users ON users.id = audit_logs.user_id::uuid`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	mock.ExpectQuery(`SELECT audit_logs.id, audit_logs.action, audit_logs.entity_type, audit_logs.entity_id, audit_logs.user_id, COALESCE\(users.full_name, ''\) as user_name, audit_logs.description, audit_logs.metadata, audit_logs.ip_address, audit_logs.created_at FROM "audit_logs" LEFT JOIN users ON users.id = audit_logs.user_id::uuid ORDER BY audit_logs.created_at desc LIMIT \$1`).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "action", "entity_type", "entity_id", "user_id", "user_name", "description", "metadata", "ip_address", "created_at"}).
			AddRow("a1", "create", "machine", "m1", nil, "", "Tạo máy", "", "", time.Now()).
			AddRow("a2", "update", "member", "m2", nil, "", "Cập nhật hội viên", "", "", time.Now()))

	logs, total, page, pageSize, err := svc.List(pagination.Params{Page: 1, PageSize: 20, Sort: "id", Order: "desc"}, AuditLogParams{})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, logs, 2)
	assert.Equal(t, 1, page)
	assert.Equal(t, 20, pageSize)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuditService_List_WithFilters(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewAuditService(db)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "audit_logs" LEFT JOIN users ON users.id = audit_logs.user_id::uuid WHERE audit_logs.action = \$1 AND audit_logs.entity_type = \$2`).
		WithArgs("create", "machine").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT audit_logs.id, audit_logs.action, audit_logs.entity_type, audit_logs.entity_id, audit_logs.user_id, COALESCE\(users.full_name, ''\) as user_name, audit_logs.description, audit_logs.metadata, audit_logs.ip_address, audit_logs.created_at FROM "audit_logs" LEFT JOIN users ON users.id = audit_logs.user_id::uuid WHERE audit_logs.action = \$1 AND audit_logs.entity_type = \$2 ORDER BY audit_logs.created_at desc LIMIT \$3`).
		WithArgs("create", "machine", 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "action", "entity_type", "entity_id", "user_id", "user_name", "description", "metadata", "ip_address", "created_at"}).
			AddRow("a1", "create", "machine", "m1", nil, "", "Tạo máy", "", "", time.Now()))

	logs, total, _, _, err := svc.List(
		pagination.Params{Page: 1, PageSize: 20, Sort: "id", Order: "desc"},
		AuditLogParams{Action: "create", EntityType: "machine"},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, logs, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuditService_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewAuditService(db)

	mock.ExpectQuery(`SELECT \* FROM "audit_logs" WHERE id = \$1 ORDER BY "audit_logs"."id" LIMIT \$2`).
		WithArgs("a1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "action"}).AddRow("a1", "create"))

	result, err := svc.GetByID("a1")
	require.NoError(t, err)
	assert.Equal(t, "create", result.Action)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuditService_Log(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewAuditService(db)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "audit_logs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	err := svc.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "machine",
		EntityID:   "m1",
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
