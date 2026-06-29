package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
)

func TestBackupService_List(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBackupService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "backup_logs"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT \* FROM "backup_logs" ORDER BY id desc LIMIT \$1`).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "file_name", "status"}).
			AddRow("b1", "backup_20260101.sql", "completed"))

	result, total, page, pageSize, err := svc.List(pagination.Params{Page: 1, PageSize: 20, Sort: "id", Order: "desc"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, result, 1)
	assert.Equal(t, 1, page)
	assert.Equal(t, 20, pageSize)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBackupService_Create(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBackupService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "backup_logs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	result, err := svc.Create(&CreateBackupRequest{Notes: "manual backup"})
	require.NoError(t, err)
	assert.Equal(t, "running", result.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBackupService_Restore_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBackupService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "backup_logs" WHERE id = \$1 ORDER BY "backup_logs"."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	err := svc.Restore("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, "backup not found", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBackupService_Restore_NoFile(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBackupService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "backup_logs" WHERE id = \$1 ORDER BY "backup_logs"."id" LIMIT \$2`).
		WithArgs("b1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "file_path"}).AddRow("b1", "completed", ""))

	err := svc.Restore("b1")
	assert.Error(t, err)
	assert.Equal(t, "backup file not found", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}
