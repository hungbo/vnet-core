package service

import (
	"errors"
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

	// Đơn của hội viên nay cộng vào total_spent (cột quyết định hạng) bất kể
	// trả bằng gì — trước đây chỉ tiền mua gói cước được cộng.
	mock.ExpectExec(`UPDATE "members" SET "total_spent"=total_spent \+ \$1`).
		WithArgs(int64(50000), "mem-1").
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

	// Định lượng đọc trước khi rẽ nhánh: rỗng nghĩa là hàng bán thẳng, kho tự quản.
	mock.ExpectQuery(`SELECT \* FROM "product_ingredients" WHERE product_id = \$1`).
		WithArgs("prod-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "product_id", "ingredient_id", "quantity"}))

	mock.ExpectRollback()

	tx := db.Begin()
	_, err := svc.deductStockForOrder(tx, "o1", "ORD-00001", "user-1")
	tx.Rollback()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "không đủ tồn kho")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Tồn kho ĐÚNG BẰNG 0 phải chặn đơn.
//
// Đây là ca mà bản cũ để lọt: điều kiện `product.CurrentStock > 0` khiến cả
// nhánh kiểm tra bị bỏ qua khi hết sạch hàng, nên đơn vẫn confirm và thanh toán
// được. Test "thiếu hàng" ở trên dùng tồn 3 / cần 10 nên không bắt được.
func TestOrderService_DeductStock_RejectsExactlyZero(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "order_items" WHERE order_id = \$1`).
		WithArgs("o1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_id", "product_id", "quantity"}).
			AddRow("i1", "o1", "prod-1", 1))
	mock.ExpectQuery(`SELECT \* FROM "products".*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "has_stock", "current_stock"}).
			AddRow("prod-1", "Coke", true, 0.0))
	mock.ExpectQuery(`SELECT \* FROM "product_ingredients" WHERE product_id = \$1`).
		WithArgs("prod-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "product_id", "ingredient_id", "quantity"}))
	mock.ExpectRollback()

	tx := db.Begin()
	_, err := svc.deductStockForOrder(tx, "o1", "ORD-1", "user-1")
	tx.Rollback()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "không đủ tồn kho")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Tồn vừa đủ thì về 0 và KHÔNG lỗi — chống sửa quá tay thành `<= 0`.
func TestOrderService_DeductStock_ExactFitLeavesZero(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "order_items" WHERE order_id = \$1`).
		WithArgs("o1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_id", "product_id", "quantity"}).
			AddRow("i1", "o1", "prod-1", 3))
	mock.ExpectQuery(`SELECT \* FROM "products".*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "has_stock", "current_stock"}).
			AddRow("prod-1", "Coke", true, 3.0))
	mock.ExpectQuery(`SELECT \* FROM "product_ingredients" WHERE product_id = \$1`).
		WithArgs("prod-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "product_id", "ingredient_id", "quantity"}))
	mock.ExpectExec(`UPDATE "products" SET "current_stock"=\$1`).
		WithArgs(0.0, "prod-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	// Hàng bán thẳng cũng phải để lại bút toán kho, không chỉ đổi con số tồn:
	// thiếu nó thì trang "Giao dịch tồn kho" trống trơn trong khi kho vẫn chạy,
	// và kỳ kiểm kê không có gì để đối chiếu.
	mock.ExpectQuery(`INSERT INTO "stock_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	tx := db.Begin()
	ids, err := svc.deductStockForOrder(tx, "o1", "ORD-1", "user-1")
	require.NoError(t, err)
	require.NoError(t, tx.Commit().Error)

	assert.Equal(t, []string{"prod-1"}, ids)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Cùng một sản phẩm ở HAI dòng đơn phải được cộng dồn.
//
// Bản cũ tính mỗi dòng từ cùng một mốc tồn kho, nên tổng 4 trên tồn 3 vẫn lọt,
// và hai lệnh UPDATE ghi đè nhau nên chỉ trừ đúng một dòng.
func TestOrderService_DeductStock_SameProductTwoLines(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "order_items" WHERE order_id = \$1`).
		WithArgs("o1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_id", "product_id", "quantity"}).
			AddRow("i1", "o1", "prod-1", 2).
			AddRow("i2", "o1", "prod-1", 2))
	for i := 0; i < 2; i++ {
		mock.ExpectQuery(`SELECT \* FROM "products".*FOR UPDATE`).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "has_stock", "current_stock"}).
				AddRow("prod-1", "Coke", true, 3.0))
		mock.ExpectQuery(`SELECT \* FROM "product_ingredients" WHERE product_id = \$1`).
			WithArgs("prod-1").
			WillReturnRows(sqlmock.NewRows([]string{"id", "product_id", "ingredient_id", "quantity"}))
	}
	mock.ExpectRollback()

	tx := db.Begin()
	_, err := svc.deductStockForOrder(tx, "o1", "ORD-1", "user-1")
	tx.Rollback()

	require.Error(t, err, "tổng 4 trên tồn 3 phải bị chặn")
	assert.Contains(t, err.Error(), "không đủ tồn kho")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Món có công thức: cột current_stock thô KHÔNG được đụng tới.
//
// Bắt hai lỗi cùng lúc — chặn oan (cột thô bằng 0 nhưng nguyên liệu còn đầy) và
// trừ hai lần (trừ cả cột thô lẫn nguyên liệu). ExpectationsWereMet bảo đảm
// không có lệnh UPDATE nào cho sản phẩm cha.
func TestOrderService_DeductStock_BOMIgnoresRawColumn(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "order_items" WHERE order_id = \$1`).
		WithArgs("o1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_id", "product_id", "quantity"}).
			AddRow("i1", "o1", "mon-1", 1))
	// Cột thô bằng 0 — nếu rơi vào nhánh kho tự quản thì đơn bị chặn oan.
	mock.ExpectQuery(`SELECT \* FROM "products".*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "has_stock", "current_stock"}).
			AddRow("mon-1", "Cà phê", true, 0.0))
	mock.ExpectQuery(`SELECT \* FROM "product_ingredients" WHERE product_id = \$1`).
		WithArgs("mon-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "product_id", "ingredient_id", "quantity"}).
			AddRow("pi1", "mon-1", "ng-lieu-1", 2.0))
	// Nguyên liệu đọc dưới khoá, còn đủ.
	mock.ExpectQuery(`SELECT \* FROM "products".*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "has_stock", "current_stock"}).
			AddRow("ng-lieu-1", "Cà phê bột", true, 50.0))
	mock.ExpectQuery(`INSERT INTO "stock_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("st1"))
	mock.ExpectExec(`UPDATE "products" SET "current_stock"=\$1`).
		WithArgs(48.0, "ng-lieu-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	tx := db.Begin()
	ids, err := svc.deductStockForOrder(tx, "o1", "ORD-1", "user-1")
	require.NoError(t, err, "món nấu từ nguyên liệu không được chặn vì cột thô bằng 0")
	require.NoError(t, tx.Commit().Error)

	assert.Equal(t, []string{"ng-lieu-1"}, ids, "chỉ nguyên liệu đổi, không phải món cha")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Huỷ đơn món có công thức: hoàn nguyên liệu, KHÔNG cộng vào cột thô của cha.
//
// Trước đây chiều trừ và chiều hoàn có hai phép phân nhánh khác nhau, nên món
// vừa có công thức vừa bật has_stock bị hoàn vào cột thô mỗi lần huỷ — tồn kho
// sinh ra từ hư không.
func TestOrderService_RestoreStock_BOMDoesNotInflateSelfStock(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "order_items" WHERE order_id = \$1`).
		WithArgs("o1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_id", "product_id", "quantity"}).
			AddRow("i1", "o1", "mon-1", 1))
	mock.ExpectQuery(`SELECT \* FROM "products".*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "has_stock", "current_stock"}).
			AddRow("mon-1", "Cà phê", true, 7.0))
	mock.ExpectQuery(`SELECT \* FROM "product_ingredients" WHERE product_id = \$1`).
		WithArgs("mon-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "product_id", "ingredient_id", "quantity"}).
			AddRow("pi1", "mon-1", "ng-lieu-1", 2.0))
	mock.ExpectQuery(`SELECT \* FROM "products".*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "has_stock", "current_stock"}).
			AddRow("ng-lieu-1", "Cà phê bột", true, 48.0))
	mock.ExpectQuery(`INSERT INTO "stock_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("st1"))
	mock.ExpectExec(`UPDATE "products" SET "current_stock"=\$1`).
		WithArgs(50.0, "ng-lieu-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	tx := db.Begin()
	ids, err := svc.restoreStockForOrder(tx, "o1", "ORD-1", "user-1")
	require.NoError(t, err)
	require.NoError(t, tx.Commit().Error)

	assert.Equal(t, []string{"ng-lieu-1"}, ids)
	assert.NoError(t, mock.ExpectationsWereMet(), "không được có UPDATE nào cho mon-1")
}

// Hàng bán thẳng: huỷ đơn cộng lại vào cột thô, và phải đọc DƯỚI KHOÁ.
func TestOrderService_RestoreStock_SelfStockAddsBackUnderLock(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "order_items" WHERE order_id = \$1`).
		WithArgs("o1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_id", "product_id", "quantity"}).
			AddRow("i1", "o1", "prod-1", 2))
	mock.ExpectQuery(`SELECT \* FROM "products".*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "has_stock", "current_stock"}).
			AddRow("prod-1", "Coke", true, 5.0))
	mock.ExpectQuery(`SELECT \* FROM "product_ingredients" WHERE product_id = \$1`).
		WithArgs("prod-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "product_id", "ingredient_id", "quantity"}))
	mock.ExpectExec(`UPDATE "products" SET "current_stock"=\$1`).
		WithArgs(7.0, "prod-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	// Hoàn kho cũng phải có bút toán "inbound" đối ứng với bút toán "outbound"
	// lúc xuất, nếu không sổ kho chỉ có một vế.
	mock.ExpectQuery(`INSERT INTO "stock_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	tx := db.Begin()
	ids, err := svc.restoreStockForOrder(tx, "o1", "ORD-1", "user-1")
	require.NoError(t, err)
	require.NoError(t, tx.Commit().Error)

	assert.Equal(t, []string{"prod-1"}, ids)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Đọc định lượng lỗi phải TRẢ LỖI, tuyệt đối không im lặng rơi sang nhánh cột
// thô — vì "không có công thức" giờ là một nhánh rẽ, nuốt lỗi thành bán tự do.
func TestOrderService_DeductStock_BOMReadFails(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "order_items" WHERE order_id = \$1`).
		WithArgs("o1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_id", "product_id", "quantity"}).
			AddRow("i1", "o1", "prod-1", 1))
	mock.ExpectQuery(`SELECT \* FROM "products".*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "has_stock", "current_stock"}).
			AddRow("prod-1", "Coke", true, 99.0))
	mock.ExpectQuery(`SELECT \* FROM "product_ingredients" WHERE product_id = \$1`).
		WithArgs("prod-1").
		WillReturnError(errors.New("kết nối đứt"))
	mock.ExpectRollback()

	tx := db.Begin()
	_, err := svc.deductStockForOrder(tx, "o1", "ORD-1", "user-1")
	tx.Rollback()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "không đọc được định lượng")
	assert.NoError(t, mock.ExpectationsWereMet())
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

// Duyệt đơn nạp tiền lần thứ hai phải bị chặn, và phải bị chặn TRƯỚC khi có
// đồng nào chuyển đi.
//
// Đây là lỗi mất tiền thật: UpdateStatus đọc đơn bằng một câu SELECT trần ở
// ngoài transaction rồi mới kiểm trạng thái, nên hai nhân viên bấm "Duyệt" cùng
// lúc — hoặc cùng một người bấm trên điện thoại và trên máy tính — đều thấy
// "pending" và đều cộng tiền. Guard duy nhất đáng tin là đọc lại dưới khoá bên
// trong transaction.
func TestOrderService_UpdateStatus_TopupRejectsSecondApproval(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	// Bản đọc ngoài transaction vẫn còn "pending" — đúng như thiết bị thứ hai
	// nhìn thấy khi trang chưa được tải lại.
	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE id = \$1 AND "orders"\."deleted_at" IS NULL ORDER BY "orders"\."id" LIMIT \$2`).
		WithArgs("o1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_code", "status", "order_type", "member_id", "final_amount"}).
			AddRow("o1", "ORD-1", "pending", "topup", "mem-1", int64(100000)))

	mock.ExpectBegin()
	// Dưới khoá thì sự thật lộ ra: lần duyệt trước đã chốt đơn.
	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE id = \$1 AND "orders"\."deleted_at" IS NULL ORDER BY "orders"\."id" LIMIT \$2 FOR UPDATE`).
		WithArgs("o1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_code", "status", "order_type", "member_id", "final_amount"}).
			AddRow("o1", "ORD-1", "completed", "topup", "mem-1", int64(100000)))
	mock.ExpectRollback()

	_, err := svc.UpdateStatus("o1", "user-1", UpdateStatusRequest{Status: "completed"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "đã được xử lý")
	// Không có INSERT member_transactions, không có UPDATE members: mọi kỳ vọng
	// đã khai đều khớp và không có kỳ vọng nào khác được dùng.
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Đơn nạp không gắn hội viên phải trả lỗi, không được báo thành công.
//
// CreateTopupOrderRequest không bắt buộc member_id. Bản cũ deref thẳng con trỏ
// nil; panic bị recover nuốt mất, mà giá trị trả về không đặt tên nên hàm trả
// nil — nhân viên thấy "nạp thành công" trong khi không đồng nào chuyển đi.
func TestOrderService_UpdateStatus_TopupWithoutMemberFails(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE id = \$1 AND "orders"\."deleted_at" IS NULL ORDER BY "orders"\."id" LIMIT \$2`).
		WithArgs("o1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_code", "status", "order_type", "member_id", "final_amount"}).
			AddRow("o1", "ORD-1", "pending", "topup", nil, int64(100000)))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE id = \$1 AND "orders"\."deleted_at" IS NULL ORDER BY "orders"\."id" LIMIT \$2 FOR UPDATE`).
		WithArgs("o1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_code", "status", "order_type", "member_id", "final_amount"}).
			AddRow("o1", "ORD-1", "pending", "topup", nil, int64(100000)))
	mock.ExpectRollback()

	_, err := svc.UpdateStatus("o1", "user-1", UpdateStatusRequest{Status: "completed"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "không gắn hội viên")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Huỷ một đơn đã huỷ không được hoàn kho lần nữa.
//
// restoreStockForOrder cộng tồn kho trả lại. Chạy hai lần là sinh hàng từ hư
// không, và bảng kiểm kê cuối ca sẽ lệch đúng bằng số hàng của đơn đó.
func TestOrderService_UpdateStatus_CancelRejectsAlreadyCancelled(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE id = \$1 AND "orders"\."deleted_at" IS NULL ORDER BY "orders"\."id" LIMIT \$2`).
		WithArgs("o1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_code", "status", "order_type"}).
			AddRow("o1", "ORD-1", "confirmed", "product"))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE id = \$1 AND "orders"\."deleted_at" IS NULL ORDER BY "orders"\."id" LIMIT \$2 FOR UPDATE`).
		WithArgs("o1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_code", "status", "order_type"}).
			AddRow("o1", "ORD-1", "cancelled", "product"))
	mock.ExpectRollback()

	_, err := svc.UpdateStatus("o1", "user-1", UpdateStatusRequest{Status: "cancelled"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "đã được xử lý")
	assert.NoError(t, mock.ExpectationsWereMet())
}
