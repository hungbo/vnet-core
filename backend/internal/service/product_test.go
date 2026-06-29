package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestProductService_List_All(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewProductService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "products" WHERE deleted_at IS NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE deleted_at IS NULL ORDER BY sort_order asc, name asc LIMIT \$1`).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "category_id", "price", "current_stock"}).
			AddRow("p1", "Coke", nil, int64(10000), float64(0)).
			AddRow("p2", "Pepsi", nil, int64(10000), float64(0)))

	mock.ExpectQuery(`SELECT \* FROM "product_options" WHERE product_id = \$1 ORDER BY sort_order asc`).
		WithArgs("p1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	mock.ExpectQuery(`SELECT \* FROM "product_options" WHERE product_id = \$1 ORDER BY sort_order asc`).
		WithArgs("p2").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	result, err := svc.List(nil, "", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(2), result.Total)
	assert.Len(t, result.Items, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProductService_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewProductService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price"}).AddRow("p1", "Coke", int64(10000)))

	mock.ExpectQuery(`SELECT \* FROM "product_options" WHERE product_id = \$1 ORDER BY sort_order asc`).
		WithArgs("p1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	result, err := svc.GetByID("p1")
	require.NoError(t, err)
	assert.Equal(t, "Coke", result.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProductService_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewProductService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.GetByID("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, "không tìm thấy sản phẩm", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProductService_Create(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewProductService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "products"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT \* FROM "product_options" WHERE product_id = \$1 ORDER BY sort_order asc`).
		WithArgs(testUUID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	catID := "cat1"
	result, err := svc.Create(&CreateProductRequest{
		CategoryID: &catID,
		Name:       "New Product",
		Price:      50000,
	})
	require.NoError(t, err)
	assert.Equal(t, "New Product", result.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProductService_Create_WithCurrentStock(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewProductService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "products"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT \* FROM "product_options" WHERE product_id = \$1 ORDER BY sort_order asc`).
		WithArgs(testUUID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	catID := "cat1"
	result, err := svc.Create(&CreateProductRequest{
		CategoryID:   &catID,
		Name:         "Bottled Water",
		Price:        10000,
		HasStock:     true,
		CurrentStock: 50,
	})
	require.NoError(t, err)
	assert.Equal(t, "Bottled Water", result.Name)
	assert.Equal(t, float64(50), result.CurrentStock)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProductService_GetByID_HasStock_WithCurrentStock(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewProductService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "has_stock", "current_stock"}).
			AddRow("p1", "Water", true, float64(50)))

	mock.ExpectQuery(`SELECT \* FROM "product_options" WHERE product_id = \$1 ORDER BY sort_order asc`).
		WithArgs("p1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	result, err := svc.GetByID("p1")
	require.NoError(t, err)
	assert.Equal(t, float64(50), result.CurrentStock)
	assert.Empty(t, result.Ingredients)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProductService_GetByID_HasStock_NoBOM(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewProductService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "has_stock"}).
			AddRow("p1", "Coffee", true))

	mock.ExpectQuery(`SELECT \* FROM "product_options" WHERE product_id = \$1 ORDER BY sort_order asc`).
		WithArgs("p1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	result, err := svc.GetByID("p1")
	require.NoError(t, err)
	assert.Equal(t, float64(0), result.CurrentStock)
	assert.Empty(t, result.Ingredients)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProductService_Update_CurrentStock(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewProductService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "has_stock"}).
			AddRow("p1", "Water", false))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "products" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND "products"."id" = \$2 ORDER BY "products"."id" LIMIT \$3`).
		WithArgs("p1", "p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "has_stock", "current_stock"}).
			AddRow("p1", "Water", true, float64(30)))

	mock.ExpectQuery(`SELECT \* FROM "product_options" WHERE product_id = \$1 ORDER BY sort_order asc`).
		WithArgs("p1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	newStock := 30.0
	result, err := svc.Update("p1", &UpdateProductRequest{
		HasStock:     boolPtr(true),
		CurrentStock: &newStock,
	})
	require.NoError(t, err)
	assert.Equal(t, float64(30), result.CurrentStock)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProductService_Delete_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewProductService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("p1", "Test"))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "products" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.Delete("p1")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
