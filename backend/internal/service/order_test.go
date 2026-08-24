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

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND "products"\."deleted_at" IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("prod-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price", "has_stock", "is_retail"}).
			AddRow("prod-1", "Coke", int64(10000), false, true))

	mock.ExpectBegin()
	// Mã đơn nay được sinh trong giao dịch, dưới khoá tư vấn.
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE order_code LIKE \$1`).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery(`INSERT INTO "orders"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))

	mock.ExpectQuery(`INSERT INTO "order_items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow("item-1", testNow))

	mock.ExpectCommit()

	result, err := svc.Create(CreateOrderRequest{
		Items: []OrderItemRequest{
			{ProductID: "prod-1", Quantity: 2},
		},
	}, "user-1")
	require.NoError(t, err)
	assert.Equal(t, int64(20000), result.TotalAmount)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderService_Create_WithBOMAndOptions(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND "products"\."deleted_at" IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("prod-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price", "has_stock", "is_retail"}).
			AddRow("prod-1", "Coffee", int64(30000), true, true))

	mock.ExpectBegin()
	// Mã đơn nay được sinh trong giao dịch, dưới khoá tư vấn.
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE order_code LIKE \$1`).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery(`INSERT INTO "orders"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))

	mock.ExpectQuery(`INSERT INTO "order_items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow("item-1", testNow))

	mock.ExpectCommit()

	result, err := svc.Create(CreateOrderRequest{
		Items: []OrderItemRequest{
			{ProductID: "prod-1", Quantity: 2},
		},
	}, "user-1")
	require.NoError(t, err)
	assert.Equal(t, int64(60000), result.TotalAmount)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderService_Create_WithOptions(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	optionID := "opt-cheese-001"
	ingredientID := "ing-cheese-001"

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND "products"\."deleted_at" IS NULL ORDER BY "products"."id" LIMIT \$2`).
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
	// Mã đơn nay được sinh trong giao dịch, dưới khoá tư vấn.
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE order_code LIKE \$1`).
		WillReturnError(gorm.ErrRecordNotFound)
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
	}, "user-1")
	require.NoError(t, err)
	assert.Equal(t, int64(25000), result.TotalAmount)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderService_Create_ProductNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND "products"\."deleted_at" IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.Create(CreateOrderRequest{
		Items: []OrderItemRequest{
			{ProductID: "nonexistent", Quantity: 1},
		},
	}, "user-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "không tìm thấy sản phẩm")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderService_Create_WithOptionsAndBOM(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	optionID := "opt-extra-001"
	optionIngredientID := "ing-flavor-001"

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND "products"\."deleted_at" IS NULL ORDER BY "products"."id" LIMIT \$2`).
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
	// Mã đơn nay được sinh trong giao dịch, dưới khoá tư vấn.
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE order_code LIKE \$1`).
		WillReturnError(gorm.ErrRecordNotFound)
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
	}, "user-1")
	require.NoError(t, err)
	assert.Equal(t, int64(25000), result.TotalAmount)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Pay used to accept whatever amount the caller sent and mark the order paid.
func TestOrderService_Pay_RejectsAmountMismatch(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE id = \$1 AND "orders"\."deleted_at" IS NULL ORDER BY "orders"\."id" LIMIT \$2 FOR UPDATE`).
		WithArgs("o1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_code", "status", "final_amount"}).
			AddRow("o1", "ORD-1", "confirmed", int64(500000)))
	mock.ExpectRollback()

	_, err := svc.Pay("o1", PayRequest{PaymentMethod: "cash", Amount: 1})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "không khớp")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Paying from balance has to actually move the member's money.
func TestOrderService_Pay_FromBalanceDeductsMember(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE id = \$1 AND "orders"\."deleted_at" IS NULL ORDER BY "orders"\."id" LIMIT \$2 FOR UPDATE`).
		WithArgs("o1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_code", "status", "final_amount", "member_id"}).
			AddRow("o1", "ORD-1", "confirmed", int64(50000), "mem-1"))

	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$2 FOR UPDATE`).
		WithArgs("mem-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance"}).AddRow("mem-1", int64(80000)))

	mock.ExpectQuery(`INSERT INTO "member_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("tx-1"))
	mock.ExpectExec(`UPDATE "members" SET "balance"=\$1,"updated_at"=\$2 WHERE "members"\."deleted_at" IS NULL AND "id" = \$3`).
		WithArgs(int64(30000), anyTime{}, "mem-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(`INSERT INTO "payments"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("pay-1"))
	mock.ExpectExec(`UPDATE "orders" SET`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT \* FROM "order_items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`SELECT \* FROM "payments"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, err := svc.Pay("o1", PayRequest{PaymentMethod: "balance", Amount: 50000})

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// A balance payment must fail rather than complete for free.
func TestOrderService_Pay_FromBalanceRejectsInsufficientFunds(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE id = \$1 AND "orders"\."deleted_at" IS NULL ORDER BY "orders"\."id" LIMIT \$2 FOR UPDATE`).
		WithArgs("o1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_code", "status", "final_amount", "member_id"}).
			AddRow("o1", "ORD-1", "confirmed", int64(50000), "mem-1"))
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$2 FOR UPDATE`).
		WithArgs("mem-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance"}).AddRow("mem-1", int64(100)))
	mock.ExpectRollback()

	_, err := svc.Pay("o1", PayRequest{PaymentMethod: "balance", Amount: 50000})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "số dư không đủ")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Thiếu hàng phải chặn đơn. Bản cũ cắt tồn kho về 0 rồi cho đơn đi tiếp, nên
// sổ sách vẫn khớp trong khi kho thì không còn hàng.
func TestOrderService_DeductStock_RejectsInsufficient(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "order_items" WHERE order_id = \$1`).
		WithArgs("o1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_id", "product_id", "quantity"}).
			AddRow("i1", "o1", "prod-1", 10))

	// Đọc dưới khoá dòng trong chính giao dịch sẽ ghi.
	mock.ExpectQuery(`SELECT \* FROM "products".*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "has_stock", "current_stock"}).
			AddRow("prod-1", "Coke", true, 3.0))

	mock.ExpectRollback()

	tx := db.Begin()
	err := svc.deductStockForOrder(tx, "o1", "ORD-00001")
	tx.Rollback()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "không đủ tồn kho")
}

// Sinh mã đơn phải chạy dưới khoá, nếu không hai đơn cùng lúc sẽ trùng mã và
// đơn thứ hai vỡ ở ràng buộc duy nhất.
func TestOrderService_GenerateOrderCode_TakesLock(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE order_code LIKE \$1`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_code"}).AddRow("o9", "ORD-00041"))
	mock.ExpectRollback()

	tx := db.Begin()
	code := svc.GenerateOrderCode(tx)
	tx.Rollback()

	assert.Equal(t, "ORD-00042", code)
	assert.NoError(t, mock.ExpectationsWereMet())
}
