package service

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/utils"
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

// Lọc "hôm nay → hôm nay" từng trả về rỗng dù nhật ký đầy ắp: mốc ngày tính
// theo nửa đêm UTC, tức 07:00 giờ ta, nên cắt mất cả ca đêm.
func TestAuditService_List_MocNgayTheoGioVietNam(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewAuditService(db)

	from := time.Date(2026, 9, 13, 0, 0, 0, 0, utils.VietnamLocation())
	to := time.Date(2026, 9, 13, 23, 59, 59, 0, utils.VietnamLocation())

	mock.ExpectQuery(`SELECT count\(\*\) FROM "audit_logs".*WHERE audit_logs\.created_at >= \$1 AND audit_logs\.created_at <= \$2`).
		WithArgs(from, to).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT audit_logs\.id.*WHERE audit_logs\.created_at >= \$1 AND audit_logs\.created_at <= \$2`).
		WithArgs(from, to, 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "action", "entity_type", "entity_id", "user_id", "user_name", "description", "metadata", "ip_address", "created_at"}).
			AddRow("a1", "create", "machine", "m1", nil, "", "Tạo máy", "", "", time.Now()))

	_, total, _, _, err := svc.List(
		pagination.Params{Page: 1, PageSize: 20, Sort: "id", Order: "desc"},
		AuditLogParams{DateFrom: "2026-09-13", DateTo: "2026-09-13"},
	)

	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Chỗ gọi không kèm metadata (đổi mật khẩu) từng ghi chuỗi rỗng vào cột jsonb,
// PostgreSQL từ chối cả dòng nhật ký — việc đổi mật khẩu không để lại dấu vết
// nào trong Nhật ký hoạt động.
func TestAuditService_Log_ThieuMetadataVanGhiJsonHopLe(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewAuditService(db)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "audit_logs"`).
		WithArgs("change_password", "user", "u1", "u1", "Đổi mật khẩu", "{}", "").
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow("a1", time.Now()))
	mock.ExpectCommit()

	userID := "u1"
	err := svc.Log(&LogAuditRequest{
		Action: "change_password", EntityType: "user", EntityID: "u1", UserID: &userID,
	})

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
