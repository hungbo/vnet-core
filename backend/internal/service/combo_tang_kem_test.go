package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Món tặng kèm của combo: lúc bán ghi thành một đơn 0đ đã hoàn tất và trừ kho
// qua đúng đường của đơn bán.

func comboTangKemSvc(db *gorm.DB) *ComboService {
	audit := NewAuditService(db)
	return NewComboService(db, audit).WithOrders(NewOrderService(db, nil, audit, NewInventoryService(db, audit)))
}

var comboCoMonTangCols = []string{"id", "name", "type", "price", "total_minutes", "member_prefix", "member_count",
	"validity_days", "created_at", "gift_product_id", "gift_quantity"}

func expectComboCoMonTang(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE \(id = \$1 AND is_active = \$2\) AND "combos"\."deleted_at" IS NULL ORDER BY "combos"\."id" LIMIT \$3`).
		WithArgs("c1", true, 1).
		WillReturnRows(sqlmock.NewRows(comboCoMonTangCols).
			AddRow("c1", "Gói 3h", "prepaid", int64(50000), 180, "Goi3h", 5, 30, testNow, "prod-1", 2))
}

func expectMonTang(mock sqlmock.Sqlmock, id, ten string, hasStock bool, ton float64) {
	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND "products"\."deleted_at" IS NULL ORDER BY "products"\."id" LIMIT \$2`).
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "is_retail", "is_active", "has_stock", "current_stock"}).
			AddRow(id, ten, true, true, hasStock, ton))
}

// Phần thu tiền gói của hội viên có sẵn, trả tiền mặt — giống các test Purchase khác.
func expectThuTienGoi(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$2 FOR UPDATE`).
		WithArgs("mem-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "balance", "total_spent", "created_at"}).
			AddRow("mem-1", "khach1", int64(0), int64(0), testNow))
	mock.ExpectQuery(`INSERT INTO "combo_purchases"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("p1"))
	mock.ExpectQuery(`INSERT INTO "member_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("t1"))
	mock.ExpectExec(`UPDATE "members" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
}

func expectDonTangVaDong(mock sqlmock.Sqlmock) {
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE order_code LIKE \$1`).
		WillReturnError(gorm.ErrRecordNotFound)
	// Đơn 0đ, hoàn tất ngay, gắn hội viên vừa mua gói.
	mock.ExpectQuery(`INSERT INTO "orders" .*VALUES \(\$1,\$2,\$3,\$4,\$5,\$6,\$7,\$8,\$9,\$10,\$11,\$12,\$13,\$14,\$15,\$16\)`).
		WithArgs("ORD-00001", "completed", "product", "mem-1", nil, "", int64(0), int64(0), nil, int64(0),
			"cash", "Tặng kèm combo Gói 3h", "u1", "u1", anyTime{}, nil).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow("o1", testNow))
}

func TestComboService_Purchase_MonTangMacDinh_TaoDon0dVaTruKho(t *testing.T) {
	db, mock := newMockDB(t)
	svc := comboTangKemSvc(db)

	expectComboCoMonTang(mock)
	expectMonTang(mock, "prod-1", "Sting", true, 10)
	// Kiểm tồn trước khi đụng tới tiền hay tài khoản.
	expectMonTang(mock, "prod-1", "Sting", true, 10)
	mock.ExpectQuery(`SELECT \* FROM "product_ingredients" WHERE product_id = \$1`).
		WithArgs("prod-1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	mock.ExpectBegin()
	expectThuTienGoi(mock)
	expectDonTangVaDong(mock)
	mock.ExpectQuery(`INSERT INTO "order_items"`).
		WithArgs("o1", "prod-1", "Sting", 2, int64(0), "null", int64(0), "served", "").
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow("i1", testNow))
	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND "products"\."deleted_at" IS NULL ORDER BY "products"\."id" LIMIT \$2 FOR UPDATE`).
		WithArgs("prod-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "has_stock", "current_stock"}).
			AddRow("prod-1", "Sting", true, 10.0))
	mock.ExpectQuery(`SELECT \* FROM "product_ingredients" WHERE product_id = \$1`).
		WithArgs("prod-1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec(`UPDATE "products" SET "current_stock"=\$1`).
		WithArgs(8.0, "prod-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`INSERT INTO "stock_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectQuery(`INSERT INTO "payments"`).
		WithArgs("o1", "cash", int64(0), "", "completed", anyTime{}, nil).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow("pay1", testNow))
	mock.ExpectCommit()

	result, err := svc.Purchase("c1", &PurchaseComboRequest{MemberID: "mem-1", PaymentMethod: "cash"}, "u1")
	require.NoError(t, err)
	assert.Equal(t, "Sting", result.GiftProductName)
	assert.Equal(t, 2, result.GiftQuantity)
	assert.Equal(t, "ORD-00001", result.GiftOrderCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Nhân viên đổi sang món khác lúc bán: món đó được ghi, số lượng vẫn theo gói.
// Món không theo dõi kho thì không có dòng nào bị trừ.
func TestComboService_Purchase_DoiMonTang(t *testing.T) {
	db, mock := newMockDB(t)
	svc := comboTangKemSvc(db)

	expectComboCoMonTang(mock)
	expectMonTang(mock, "prod-2", "Trà đá", false, 0)
	expectMonTang(mock, "prod-2", "Trà đá", false, 0)
	mock.ExpectQuery(`SELECT \* FROM "product_ingredients" WHERE product_id = \$1`).
		WithArgs("prod-2").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	mock.ExpectBegin()
	expectThuTienGoi(mock)
	expectDonTangVaDong(mock)
	mock.ExpectQuery(`INSERT INTO "order_items"`).
		WithArgs("o1", "prod-2", "Trà đá", 2, int64(0), "null", int64(0), "served", "").
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow("i1", testNow))
	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 .*FOR UPDATE`).
		WithArgs("prod-2", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "has_stock"}).AddRow("prod-2", "Trà đá", false))
	mock.ExpectQuery(`SELECT \* FROM "product_ingredients" WHERE product_id = \$1`).
		WithArgs("prod-2").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`INSERT INTO "payments"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow("pay1", testNow))
	mock.ExpectCommit()

	result, err := svc.Purchase("c1", &PurchaseComboRequest{
		MemberID: "mem-1", PaymentMethod: "cash", GiftProductID: strPtr("prod-2"),
	}, "u1")
	require.NoError(t, err)
	assert.Equal(t, "Trà đá", result.GiftProductName)
	assert.Equal(t, 2, result.GiftQuantity)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Khách không lấy món tặng: không đọc sản phẩm, không có đơn nào.
func TestComboService_Purchase_BoMonTang(t *testing.T) {
	db, mock := newMockDB(t)
	svc := comboTangKemSvc(db)

	expectComboCoMonTang(mock)
	mock.ExpectBegin()
	expectThuTienGoi(mock)
	mock.ExpectCommit()

	result, err := svc.Purchase("c1", &PurchaseComboRequest{
		MemberID: "mem-1", PaymentMethod: "cash", GiftProductID: strPtr(""),
	}, "u1")
	require.NoError(t, err)
	assert.Empty(t, result.GiftOrderCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Hết món tặng thì dừng TRƯỚC khi tạo tài khoản khách hay thu tiền gói.
func TestComboService_Purchase_HetMonTang_KhongTaoTaiKhoan(t *testing.T) {
	db, mock := newMockDB(t)
	svc := comboTangKemSvc(db)

	expectComboCoMonTang(mock)
	expectMonTang(mock, "prod-1", "Sting", true, 1)
	expectMonTang(mock, "prod-1", "Sting", true, 1)
	mock.ExpectQuery(`SELECT \* FROM "product_ingredients" WHERE product_id = \$1`).
		WithArgs("prod-1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, err := svc.Purchase("c1", &PurchaseComboRequest{PaymentMethod: "cash"}, "u1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "không đủ tồn kho")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_Create_MonTang(t *testing.T) {
	db, mock := newMockDB(t)
	svc := comboTangKemSvc(db)

	expectMonTang(mock, "prod-1", "Sting", true, 10)
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "combos" .*"gift_product_id","gift_quantity"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("c1"))
	mock.ExpectCommit()
	mock.ExpectQuery(`SELECT "id","name" FROM "products" WHERE id IN \(\$1\)`).
		WithArgs("prod-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("prod-1", "Sting"))

	// Số lượng 0 tính là 1: có món tặng thì ít nhất một phần.
	result, err := svc.Create(&CreateComboRequest{
		Name: "Gói 3h", Type: "prepaid", TotalMinutes: 180, Price: 50000, GiftProductID: "prod-1",
	})
	require.NoError(t, err)
	assert.Equal(t, "prod-1", result.GiftProductID)
	assert.Equal(t, "Sting", result.GiftProductName)
	assert.Equal(t, 1, result.GiftQuantity)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_Create_MonTangNgungBan(t *testing.T) {
	db, mock := newMockDB(t)
	svc := comboTangKemSvc(db)

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1`).
		WithArgs("prod-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "is_retail", "is_active"}).
			AddRow("prod-1", "Sting", true, false))

	_, err := svc.Create(&CreateComboRequest{
		Name: "Gói 3h", Type: "prepaid", TotalMinutes: 180, GiftProductID: "prod-1", GiftQuantity: 1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "đang ngưng bán")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Gửi gift_product_id rỗng là bỏ tặng: cả món lẫn số lượng về trống.
func TestComboService_Update_BoMonTang(t *testing.T) {
	db, mock := newMockDB(t)
	svc := comboTangKemSvc(db)

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "gift_product_id", "gift_quantity", "created_at"}).
			AddRow("c1", "Gói 3h", "prod-1", 2, testNow))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "combos" SET "gift_product_id"=\$1,"gift_quantity"=\$2 WHERE`).
		WithArgs(nil, 0, "c1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1`).
		WithArgs("c1", "c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "gift_product_id", "gift_quantity", "created_at"}).
			AddRow("c1", "Gói 3h", nil, 0, testNow))

	result, err := svc.Update("c1", &UpdateComboRequest{GiftProductID: strPtr("")})
	require.NoError(t, err)
	assert.Empty(t, result.GiftProductID)
	assert.Equal(t, 0, result.GiftQuantity)
	assert.NoError(t, mock.ExpectationsWereMet())
}
