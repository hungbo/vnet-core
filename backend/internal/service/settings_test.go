package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSettingsService_List(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSettingsService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "system_settings" ORDER BY group_name, key`).
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}).
			AddRow("general", "site_name", "VNET").
			AddRow("general", "timezone", "Asia/HCM").
			AddRow("pricing", "vat_rate", "10"))

	result, err := svc.List()
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Len(t, result["general"], 2)
	assert.Len(t, result["pricing"], 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSettingsService_List_Empty(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSettingsService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "system_settings" ORDER BY group_name, key`).
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}))

	result, err := svc.List()
	require.NoError(t, err)
	assert.Len(t, result, 0)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSettingsService_GetByGroup_Found(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSettingsService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1 ORDER BY key`).
		WithArgs("general").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}).
			AddRow("general", "site_name", "VNET").
			AddRow("general", "timezone", "Asia/HCM"))

	result, err := svc.GetByGroup("general")
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSettingsService_GetByGroup_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSettingsService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1 ORDER BY key`).
		WithArgs("nonexistent").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}))

	_, err := svc.GetByGroup("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, "settings group not found", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSettingsService_Update_CreateNew(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSettingsService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1 AND key = \$2 ORDER BY "system_settings"."id" LIMIT \$3`).
		WithArgs("general", "new_key", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "system_settings"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1 ORDER BY key`).
		WithArgs("general").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}).
			AddRow("general", "new_key", "test_value"))

	result, err := svc.Update("general", map[string]interface{}{"new_key": "test_value"})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSettingsService_Update_Existing(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSettingsService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1 AND key = \$2 ORDER BY "system_settings"."id" LIMIT \$3`).
		WithArgs("general", "site_name", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "group_name", "key", "value"}).AddRow("s1", "general", "site_name", "VNET"))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "system_settings" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1 ORDER BY key`).
		WithArgs("general").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}).
			AddRow("general", "site_name", "VNET 2.0"))

	result, err := svc.Update("general", map[string]interface{}{"site_name": "VNET 2.0"})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}
