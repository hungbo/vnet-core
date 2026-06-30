package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestOrderService_Create_NoStock(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE order_code LIKE \$1 ORDER BY order_code DESC,"orders"."id" LIMIT \$2`).
		WithArgs("ORD-%", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("prod-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price", "has_stock", "is_retail"}).
			AddRow("prod-1", "Coke", int64(10000), false, true))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "orders"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))

	mock.ExpectQuery(`INSERT INTO "order_items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow("item-1", testNow))

	mock.ExpectCommit()

	result, err := svc.Create(CreateOrderRequest{
		Items: []OrderItemRequest{
			{ProductID: "prod-1", Quantity: 2},
		},
	}, "user-1", "store-1")
	require.NoError(t, err)
	assert.Equal(t, int64(20000), result.TotalAmount)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderService_Create_WithBOMAndOptions(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE order_code LIKE \$1 ORDER BY order_code DESC,"orders"."id" LIMIT \$2`).
		WithArgs("ORD-%", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("prod-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price", "has_stock", "is_retail"}).
			AddRow("prod-1", "Coffee", int64(30000), true, true))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "orders"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))

	mock.ExpectQuery(`INSERT INTO "order_items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow("item-1", testNow))

	mock.ExpectCommit()

	result, err := svc.Create(CreateOrderRequest{
		Items: []OrderItemRequest{
			{ProductID: "prod-1", Quantity: 2},
		},
	}, "user-1", "store-1")
	require.NoError(t, err)
	assert.Equal(t, int64(60000), result.TotalAmount)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderService_Create_WithOptions(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	optionID := "opt-cheese-001"
	ingredientID := "ing-cheese-001"

	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE order_code LIKE \$1 ORDER BY order_code DESC,"orders"."id" LIMIT \$2`).
		WithArgs("ORD-%", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("prod-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price", "has_stock", "is_retail"}).
			AddRow("prod-1", "Sandwich", int64(25000), true, true))

	mock.ExpectQuery(`SELECT \* FROM "product_options" WHERE id = \$1 AND product_id = \$2 ORDER BY "product_options"."id" LIMIT \$3`).
		WithArgs(optionID, "prod-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "ingredient_id", "quantity"}).
			AddRow(optionID, ingredientID, float64(1)))

	mock.ExpectQuery(`SELECT "price" FROM "products" WHERE`).
		WithArgs(ingredientID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"price"}).AddRow(int64(0)))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "orders"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))

	mock.ExpectQuery(`INSERT INTO "order_items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow("item-1", testNow))

	mock.ExpectCommit()

	result, err := svc.Create(CreateOrderRequest{
		Items: []OrderItemRequest{
			{
				ProductID: "prod-1",
				Quantity:  1,
				Options:   `[{"option_id":"opt-cheese-001","quantity":1}]`,
			},
		},
	}, "user-1", "store-1")
	require.NoError(t, err)
	assert.Equal(t, int64(25000), result.TotalAmount)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderService_Create_ProductNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE order_code LIKE \$1 ORDER BY order_code DESC,"orders"."id" LIMIT \$2`).
		WithArgs("ORD-%", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.Create(CreateOrderRequest{
		Items: []OrderItemRequest{
			{ProductID: "nonexistent", Quantity: 1},
		},
	}, "user-1", "store-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "không tìm thấy sản phẩm")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderService_Create_WithOptionsAndBOM(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	optionID := "opt-extra-001"
	optionIngredientID := "ing-flavor-001"

	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE order_code LIKE \$1 ORDER BY order_code DESC,"orders"."id" LIMIT \$2`).
		WithArgs("ORD-%", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("prod-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price", "has_stock", "is_retail"}).
			AddRow("prod-1", "Coffee", int64(25000), true, true))

	mock.ExpectQuery(`SELECT \* FROM "product_options" WHERE id = \$1 AND product_id = \$2 ORDER BY "product_options"."id" LIMIT \$3`).
		WithArgs(optionID, "prod-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "ingredient_id", "quantity"}).
			AddRow(optionID, optionIngredientID, float64(1)))

	mock.ExpectQuery(`SELECT "price" FROM "products" WHERE`).
		WithArgs(optionIngredientID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"price"}).AddRow(int64(0)))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "orders"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))

	mock.ExpectQuery(`INSERT INTO "order_items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow("item-1", testNow))

	mock.ExpectCommit()

	result, err := svc.Create(CreateOrderRequest{
		Items: []OrderItemRequest{
			{
				ProductID: "prod-1",
				Quantity:  1,
				Options:   `[{"option_id":"opt-extra-001","quantity":1}]`,
			},
		},
	}, "user-1", "store-1")
	require.NoError(t, err)
	assert.Equal(t, int64(25000), result.TotalAmount)
	assert.NoError(t, mock.ExpectationsWereMet())
}
