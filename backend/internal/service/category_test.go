package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCategoryService_List(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewCategoryService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "categories"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "sort_order"}).
			AddRow("c1", "Food", 1).
			AddRow("c2", "Drinks", 2))

	result, err := svc.List()
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCategoryService_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewCategoryService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "categories" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "categories"."id" LIMIT \$2`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("c1", "Food"))

	mock.ExpectQuery(`SELECT \* FROM "categories" WHERE parent_id = \$1 AND deleted_at IS NULL ORDER BY sort_order asc`).
		WithArgs("c1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

	result, err := svc.GetByID("c1")
	require.NoError(t, err)
	assert.Equal(t, "Food", result.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCategoryService_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewCategoryService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "categories" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "categories"."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.GetByID("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, "category not found", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCategoryService_Create(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewCategoryService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "categories"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	result, err := svc.Create(&CreateCategoryRequest{Name: "New Category"})
	require.NoError(t, err)
	assert.Equal(t, "New Category", result.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCategoryService_Update_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewCategoryService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "categories" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "categories"."id" LIMIT \$2`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("c1", "Old Name"))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "categories" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT \* FROM "categories" WHERE id = \$1 AND "categories"."id" = \$2 ORDER BY "categories"."id" LIMIT \$3`).
		WithArgs("c1", "c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("c1", "New Name"))

	name := "New Name"
	result, err := svc.Update("c1", &UpdateCategoryRequest{Name: &name})
	require.NoError(t, err)
	assert.Equal(t, "New Name", result.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCategoryService_Delete_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewCategoryService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "categories" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "categories"."id" LIMIT \$2`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("c1", "Test"))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "categories" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.Delete("c1")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
