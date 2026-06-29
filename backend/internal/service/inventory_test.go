package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestInventoryService_ListSuppliers(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewInventoryService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "suppliers" WHERE deleted_at IS NULL ORDER BY name asc`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("s1", "Supplier A"))

	result, err := svc.ListSuppliers()
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInventoryService_CreateSupplier(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewInventoryService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "suppliers"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	result, err := svc.CreateSupplier(&CreateSupplierRequest{Name: "Supplier A"})
	require.NoError(t, err)
	assert.Equal(t, "Supplier A", result.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInventoryService_UpdateSupplier_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewInventoryService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "suppliers" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "suppliers"."id" LIMIT \$2`).
		WithArgs("s1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("s1", "Old"))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "suppliers" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT \* FROM "suppliers" WHERE id = \$1 AND "suppliers"."id" = \$2 ORDER BY "suppliers"."id" LIMIT \$3`).
		WithArgs("s1", "s1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("s1", "Updated"))

	name := "Updated"
	result, err := svc.UpdateSupplier("s1", &UpdateSupplierRequest{Name: &name})
	require.NoError(t, err)
	assert.Equal(t, "Updated", result.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInventoryService_DeleteSupplier_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewInventoryService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "suppliers" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "suppliers"."id" LIMIT \$2`).
		WithArgs("s1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("s1"))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "suppliers" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.DeleteSupplier("s1")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInventoryService_ListWarehouses(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewInventoryService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "warehouses" WHERE deleted_at IS NULL ORDER BY name asc`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("w1", "Main WH"))

	result, err := svc.ListWarehouses()
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInventoryService_CreateWarehouse(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewInventoryService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "warehouses"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	result, err := svc.CreateWarehouse(&CreateWarehouseRequest{Name: "Main WH"})
	require.NoError(t, err)
	assert.Equal(t, "Main WH", result.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInventoryService_ListUnits(t *testing.T) {
	result := ListUnits()
	assert.Len(t, result, 9)
	assert.Equal(t, "bich", result[0].ID)
}

func TestInventoryService_DeductItemsStock_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewInventoryService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("ing-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "current_stock", "has_stock"}).
			AddRow("ing-1", "Coffee Beans", float64(100), true))

	mock.ExpectQuery(`INSERT INTO "stock_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("tx-1"))

	mock.ExpectExec(`UPDATE "products" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	tx := db.Begin()
	err := svc.DeductItemsStock(tx, []StockDeductionItem{
		{IngredientID: "ing-1", Quantity: 20},
	}, "prod-1", "ref-uuid-001", "ORD-001")
	tx.Commit()
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInventoryService_DeductItemsStock_MultipleIngredients(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewInventoryService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("ing-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "current_stock", "has_stock"}).
			AddRow("ing-1", "Coffee Beans", float64(100), true))

	mock.ExpectQuery(`INSERT INTO "stock_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("tx-1"))

	mock.ExpectExec(`UPDATE "products" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("ing-2", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "current_stock", "has_stock"}).
			AddRow("ing-2", "Milk", float64(200), true))

	mock.ExpectQuery(`INSERT INTO "stock_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("tx-2"))

	mock.ExpectExec(`UPDATE "products" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	tx := db.Begin()
	err := svc.DeductItemsStock(tx, []StockDeductionItem{
		{IngredientID: "ing-1", Quantity: 20},
		{IngredientID: "ing-2", Quantity: 30},
	}, "prod-1", "ref-uuid-001", "ORD-001")
	tx.Commit()
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInventoryService_DeductItemsStock_GroupsSameIngredient(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewInventoryService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("ing-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "current_stock", "has_stock"}).
			AddRow("ing-1", "Coffee Beans", float64(100), true))

	mock.ExpectQuery(`INSERT INTO "stock_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("tx-1"))

	mock.ExpectExec(`UPDATE "products" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	tx := db.Begin()
	err := svc.DeductItemsStock(tx, []StockDeductionItem{
		{IngredientID: "ing-1", Quantity: 10},
		{IngredientID: "ing-1", Quantity: 15},
	}, "prod-1", "ref-uuid-001", "ORD-001")
	tx.Commit()
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInventoryService_DeductItemsStock_InsufficientStock(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewInventoryService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("ing-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "current_stock", "has_stock"}).
			AddRow("ing-1", "Coffee Beans", float64(5), true))
	mock.ExpectRollback()

	tx := db.Begin()
	err := svc.DeductItemsStock(tx, []StockDeductionItem{
		{IngredientID: "ing-1", Quantity: 10},
	}, "prod-1", "ref-uuid-001", "ORD-001")
	tx.Rollback()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "không đủ tồn kho")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInventoryService_DeductItemsStock_IngredientNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewInventoryService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectRollback()

	tx := db.Begin()
	err := svc.DeductItemsStock(tx, []StockDeductionItem{
		{IngredientID: "nonexistent", Quantity: 1},
	}, "prod-1", "ref-uuid-001", "ORD-001")
	tx.Rollback()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "không tìm thấy")
	assert.NoError(t, mock.ExpectationsWereMet())
}
