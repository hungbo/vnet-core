package service

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/utils"
	"gorm.io/gorm"
)

func TestInventoryService_ListSuppliers(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewInventoryService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "suppliers" WHERE "suppliers"\."deleted_at" IS NULL ORDER BY name asc`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("s1", "Supplier A"))

	result, err := svc.ListSuppliers()
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Bấm tiêu đề cột "Số lượng" ở trang Phiếu kho: sắp trên toàn bộ phiếu,
// created_at làm tiêu chí phụ.
func TestInventoryService_ListStockTransactions_SapXepTheoCot(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewInventoryService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "stock_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT \* FROM "stock_transactions" ORDER BY quantity asc, created_at desc LIMIT \$1`).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, err := svc.ListStockTransactions(StockTransactionFilter{}, &pagination.Params{Page: 1, PageSize: 20, Sort: "quantity", Order: "asc", Sorted: true})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Thanh lọc trang Giao dịch tồn kho: nguồn phiếu (ở đây: hệ thống xuất theo đơn), khoảng ngày theo giờ Việt Nam
// và ô tìm kiếm (sản phẩm, ghi chú có mã đơn, người thực hiện), không dấu. Đếm
// và lấy trang dùng cùng điều kiện, để tổng ở phân trang khớp với kết quả lọc.
func TestInventoryService_ListStockTransactions_Loc(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewInventoryService(db, NewAuditService(db))

	where := `WHERE \(transaction_type = \$1 AND reference_id IN \(SELECT id FROM orders\)\) AND created_at >= \$2 AND created_at <= \$3 AND \(unaccent\(description\) ILIKE unaccent\(\$4\)\s+OR product_id IN \(SELECT id FROM products WHERE unaccent\(name\) ILIKE unaccent\(\$5\)\)\s+OR created_by IN \(SELECT id FROM users WHERE unaccent\(full_name\) ILIKE unaccent\(\$6\) OR unaccent\(username\) ILIKE unaccent\(\$7\)\)\)`
	tu := time.Date(2026, 11, 1, 0, 0, 0, 0, utils.VietnamLocation())
	den := time.Date(2026, 11, 8, 23, 59, 59, 0, utils.VietnamLocation())
	mock.ExpectQuery(`SELECT count\(\*\) FROM "stock_transactions" `+where).
		WithArgs("outbound", tu, den, "%sting%", "%sting%", "%sting%", "%sting%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT \* FROM "stock_transactions" `+where+` ORDER BY created_at desc LIMIT \$8`).
		WithArgs("outbound", tu, den, "%sting%", "%sting%", "%sting%", "%sting%", 20).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, err := svc.ListStockTransactions(
		StockTransactionFilter{Kind: PhieuKhoXuatDon, DateFrom: "2026-11-01", DateTo: "2026-11-08"},
		&pagination.Params{Page: 1, PageSize: 20, Search: "sting"})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Lọc "Nhập tay": phiếu nhập không trỏ vào đơn nào (trang Sản phẩm trỏ vào chính
// sản phẩm, nên reference_id có thể khác rỗng mà vẫn là phiếu tay).
func TestInventoryService_ListStockTransactions_LocNhapTay(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewInventoryService(db, NewAuditService(db))

	// Một điều kiện duy nhất thì GORM không bọc ngoặc; có thêm ngày/tìm kiếm thì
	// bọc (xem TestInventoryService_ListStockTransactions_Loc).
	where := `WHERE transaction_type = \$1 AND \(reference_id IS NULL OR NOT reference_id IN \(SELECT id FROM orders\)\)`
	mock.ExpectQuery(`SELECT count\(\*\) FROM "stock_transactions" ` + where + `$`).
		WithArgs("inbound").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT \* FROM "stock_transactions" `+where+` ORDER BY created_at desc LIMIT \$2`).
		WithArgs("inbound", 20).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, err := svc.ListStockTransactions(StockTransactionFilter{Kind: PhieuKhoNhapTay}, &pagination.Params{Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Cột "Loại" hiện nguồn phiếu: nhập/xuất tay, xuất/hoàn theo đơn, kiểm kê. Tra
// một câu cho cả trang xem reference_id nào là đơn hàng.
func TestInventoryService_ListStockTransactions_NguonPhieu(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewInventoryService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "stock_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
	mock.ExpectQuery(`SELECT \* FROM "stock_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "transaction_type", "reference_id"}).
			AddRow("t1", "inbound", nil).      // nhập ở trang Giao dịch tồn kho
			AddRow("t2", "outbound", "sp1").   // xuất ở trang Sản phẩm: trỏ vào sản phẩm
			AddRow("t3", "outbound", "don1").  // bán theo đơn
			AddRow("t4", "inbound", "don1").   // huỷ đơn, trả kho
			AddRow("t5", "adjustment", "kk1")) // chốt kiểm kê
	mock.ExpectQuery(`SELECT "id" FROM "orders" WHERE id IN \(\$1,\$2,\$3,\$4\)$`).
		WithArgs("sp1", "don1", "don1", "kk1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("don1"))

	res, err := svc.ListStockTransactions(StockTransactionFilter{}, &pagination.Params{Page: 1, PageSize: 20})
	require.NoError(t, err)
	var kinds []string
	for _, it := range res.Items.([]StockTransactionResponse) {
		kinds = append(kinds, it.Kind)
	}
	assert.Equal(t, []string{PhieuKhoNhapTay, PhieuKhoXuatTay, PhieuKhoXuatDon, PhieuKhoHoanDon, PhieuKhoKiemKe}, kinds)
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

	mock.ExpectQuery(`SELECT \* FROM "suppliers" WHERE id = \$1 AND "suppliers"\."deleted_at" IS NULL ORDER BY "suppliers"."id" LIMIT \$2`).
		WithArgs("s1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("s1", "Old"))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "suppliers" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT \* FROM "suppliers" WHERE id = \$1 AND "suppliers"\."deleted_at" IS NULL AND "suppliers"\."id" = \$2 ORDER BY "suppliers"\."id" LIMIT \$3`).
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

	mock.ExpectQuery(`SELECT \* FROM "suppliers" WHERE id = \$1 AND "suppliers"\."deleted_at" IS NULL ORDER BY "suppliers"."id" LIMIT \$2`).
		WithArgs("s1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("s1"))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "products"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "stock_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "suppliers" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.DeleteSupplier("s1")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInventoryService_ListUnits(t *testing.T) {
	result := ListUnits()
	assert.Len(t, result, len(ValidUnits))
	assert.Equal(t, "bich", result[0].ID)
}

func TestInventoryService_DeductItemsStock_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewInventoryService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND "products"\."deleted_at" IS NULL ORDER BY "products"."id" LIMIT \$2`).
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
	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND "products"\."deleted_at" IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("ing-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "current_stock", "has_stock"}).
			AddRow("ing-1", "Coffee Beans", float64(100), true))

	mock.ExpectQuery(`INSERT INTO "stock_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("tx-1"))

	mock.ExpectExec(`UPDATE "products" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND "products"\."deleted_at" IS NULL ORDER BY "products"."id" LIMIT \$2`).
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
	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND "products"\."deleted_at" IS NULL ORDER BY "products"."id" LIMIT \$2`).
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
	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND "products"\."deleted_at" IS NULL ORDER BY "products"."id" LIMIT \$2`).
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
	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND "products"\."deleted_at" IS NULL ORDER BY "products"."id" LIMIT \$2`).
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

// Nhập -4 từng làm tồn GIẢM mà sổ vẫn ghi "nhập kho"; xuất âm thì ngược lại làm
// tồn TĂNG. Số lượng không hợp lệ phải bị chặn trước khi đụng tới CSDL.
func TestInventoryService_CreateStockTransaction_RejectsNonPositiveQuantity(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewInventoryService(db, NewAuditService(db))
	pid := "p-1"

	for _, c := range []struct {
		typ string
		qty float64
	}{
		{"inbound", -4}, {"inbound", 0}, {"outbound", -3}, {"outbound", 0}, {"adjustment", -1},
	} {
		_, err := svc.CreateStockTransaction(&CreateStockTransactionRequest{
			ProductID: &pid, TransactionType: c.typ, Quantity: c.qty,
		}, "u-1")
		// Khẳng định đúng câu báo lỗi: thiếu chốt chặn thì sqlmock cũng trả lỗi
		// (câu SELECT không được khai), test sẽ đỗ nhầm nếu chỉ kiểm có lỗi.
		require.EqualError(t, err, "số lượng phải lớn hơn 0", "%s %v", c.typ, c.qty)
	}
	// Không có kỳ vọng SQL nào: không câu lệnh nào được chạy.
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Điều chỉnh về 0 (kiểm thấy hết hàng) vẫn phải làm được.
func TestInventoryService_CreateStockTransaction_AdjustmentToZeroAllowed(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewInventoryService(db, NewAuditService(db))
	pid := "p-1"

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "current_stock"}).AddRow("p-1", "Nuoc", 5.0))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "products" SET "current_stock"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`INSERT INTO "stock_transactions"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("st-1"))
	mock.ExpectCommit()

	_, err := svc.CreateStockTransaction(&CreateStockTransactionRequest{
		ProductID: &pid, TransactionType: "adjustment", Quantity: 0,
	}, "u-1")
	// Sau commit service còn đọc lại phiếu/ghi audit — không kỳ vọng ở đây,
	// chỉ cần lệnh điều chỉnh về 0 đi qua được phần kiểm số lượng.
	if err != nil {
		require.NotContains(t, err.Error(), "số lượng")
	}
}
