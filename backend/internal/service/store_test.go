package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
)

func TestStoreService_List(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewStoreService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "stores"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	mock.ExpectQuery(`SELECT \* FROM "stores" ORDER BY id desc LIMIT \$1`).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "code"}).
			AddRow("s1", "Store A", "SA").
			AddRow("s2", "Store B", "SB"))

	stores, total, page, pageSize, err := svc.List(pagination.Params{Page: 1, PageSize: 20, Sort: "id", Order: "desc"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, stores, 2)
	assert.Equal(t, 1, page)
	assert.Equal(t, 20, pageSize)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStoreService_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewStoreService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "stores" WHERE id = \$1 ORDER BY "stores"."id" LIMIT \$2`).
		WithArgs("s1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("s1", "Store A"))

	store, err := svc.GetByID("s1")
	require.NoError(t, err)
	assert.Equal(t, "Store A", store.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStoreService_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewStoreService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "stores" WHERE id = \$1 ORDER BY "stores"."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.GetByID("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, "store not found", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStoreService_Create(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewStoreService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "stores"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	result, err := svc.Create(&CreateStoreRequest{
		Name: "New Store",
		Code: "NS",
	})
	require.NoError(t, err)
	assert.Equal(t, "New Store", result.Name)
	assert.Equal(t, "NS", result.Code)
	assert.True(t, result.IsActive)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStoreService_Update_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewStoreService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "stores" WHERE id = \$1 ORDER BY "stores"."id" LIMIT \$2`).
		WithArgs("s1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("s1", "Old Name"))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "stores" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	name := "Updated Name"
	result, err := svc.Update("s1", &UpdateStoreRequest{Name: &name})
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", result.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStoreService_Delete_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewStoreService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "stores" WHERE id = \$1 ORDER BY "stores"."id" LIMIT \$2`).
		WithArgs("s1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("s1", "Test"))

	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "stores" WHERE "stores"."id" = \$1`).
		WithArgs("s1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.Delete("s1")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
