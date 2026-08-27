package service

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Mọi test ở đây theo cùng một khuôn: mock First trả về bản ghi, mock câu Count
// ĐẦU TIÊN trả về số > 0, rồi khẳng định hàm dừng lại — không có transaction
// nào mở ra, và lỗi khớp ErrRangBuoc để handler trả 409.

func TestMachineService_Delete_RefusesWhenSessionsRemain(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code"}).AddRow("m1", "M-001"))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "machine_sessions" WHERE machine_id = \$1`).
		WithArgs("m1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(142))

	err := svc.Delete("m1")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRangBuoc))
	assert.Contains(t, err.Error(), "142 phiên chơi")
	// Lối thoát phải nằm ngay trong thông điệp, nếu không người dùng bị chặn mà
	// không biết làm gì tiếp.
	assert.Contains(t, err.Error(), "tắt hoạt động máy")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_DeleteGroup_RefusesWhenMachinesRemain(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machine_groups" WHERE id = \$1`).
		WithArgs("g1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("g1", "VIP"))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "machines" WHERE group_id = \$1`).
		WithArgs("g1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(12))

	err := svc.DeleteGroup("g1")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRangBuoc))
	assert.Contains(t, err.Error(), "12 máy")
	assert.Contains(t, err.Error(), "chuyển chúng sang nhóm khác")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMemberService_Delete_RefusesWhenTransactionsRemain(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMemberService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1`).
		WithArgs("mem1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username"}).AddRow("mem1", "khach01"))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "member_transactions" WHERE member_id = \$1`).
		WithArgs("mem1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(7))

	err := svc.Delete("mem1")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRangBuoc))
	assert.Contains(t, err.Error(), "7 giao dịch số dư")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Nhóm mặc định kiểm TRƯỚC phần đếm: lý do này không sửa được bằng cách chuyển
// hội viên đi, nên không được để nó nấp sau một thông điệp sai.
func TestMemberService_DeleteGroup_RefusesDefaultGroupBeforeCounting(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMemberService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "member_groups" WHERE id = \$1`).
		WithArgs("g1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "is_default"}).AddRow("g1", "Thường", true))
	// Không khai Count nào: nếu hàm đếm trước, sqlmock sẽ báo lỗi.

	err := svc.DeleteGroup("g1")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRangBuoc))
	assert.Contains(t, err.Error(), "nhóm mặc định")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProductService_Delete_RefusesWhenOrderItemsRemain(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewProductService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("p1", "Cà phê"))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "order_items" WHERE product_id = \$1`).
		WithArgs("p1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(58))

	err := svc.Delete("p1")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRangBuoc))
	assert.Contains(t, err.Error(), "58 dòng đơn hàng")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_Delete_RefusesWhenPurchasesRemain(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("c1", "Gaming 3h"))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "combo_purchases" WHERE combo_id = \$1`).
		WithArgs("c1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(4))

	err := svc.Delete("c1")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRangBuoc))
	assert.Contains(t, err.Error(), "4 lượt mua combo")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInventoryService_DeleteSupplier_RefusesWhenProductsRemain(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewInventoryService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "suppliers" WHERE id = \$1`).
		WithArgs("s1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("s1", "NCC A"))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "products" WHERE supplier_id = \$1`).
		WithArgs("s1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(9))

	err := svc.DeleteSupplier("s1")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRangBuoc))
	assert.Contains(t, err.Error(), "9 sản phẩm")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPromotionService_Delete_RefusesWhenOrdersRemain(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPromotionService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "promotions" WHERE id = \$1`).
		WithArgs("pr1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("pr1", "Giảm 10%"))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "orders" WHERE promotion_id = \$1`).
		WithArgs("pr1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(31))

	err := svc.Delete("pr1")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRangBuoc))
	assert.Contains(t, err.Error(), "31 đơn hàng đã áp dụng")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Máy in mặc định chặn trước phần đếm — cùng lý do với nhóm hội viên mặc định.
func TestPrinterService_Delete_RefusesDefaultPrinter(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPrinterService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "printer_configs" WHERE id = \$1`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "is_default"}).AddRow("p1", "Bếp", true))

	err := svc.Delete("p1")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRangBuoc))
	assert.Contains(t, err.Error(), "máy in mặc định")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSystemManageService_DeleteUser_RefusesWhenShiftsRemain(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSystemManageService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).
		WithArgs("u1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username"}).AddRow("u1", "nhanvien01"))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "shifts" WHERE user_id = \$1`).
		WithArgs("u1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(23))

	err := svc.DeleteUser("u1")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRangBuoc))
	assert.Contains(t, err.Error(), "23 ca làm việc")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- Nhóm chặn theo TRẠNG THÁI -------------------------------------------

// Đơn đã hoàn tất dừng ngay ở guard trạng thái, không chạm tới câu Count nào.
func TestOrderService_Delete_RefusesCompletedOrder(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE id = \$1`).
		WithArgs("o1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_code", "status"}).
			AddRow("o1", "ORD-1", "completed"))

	err := svc.Delete("o1")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRangBuoc))
	assert.Contains(t, err.Error(), "ORD-1")
	assert.Contains(t, err.Error(), "huỷ đơn")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Đơn pending nhưng đã thu tiền: xoá là mất một dòng doanh thu.
func TestOrderService_Delete_RefusesWhenPaymentExists(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE id = \$1`).
		WithArgs("o1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_code", "status"}).
			AddRow("o1", "ORD-1", "pending"))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "payments" WHERE order_id = \$1`).
		WithArgs("o1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	err := svc.Delete("o1")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRangBuoc))
	assert.Contains(t, err.Error(), "1 phiếu thanh toán")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Đơn pending sạch chứng từ thì xoá được, và order_items phải đi cùng nó trong
// MỘT transaction. Chặn theo số bản ghi con sẽ làm test này không thể xanh —
// mọi đơn đều có order_items, tức là không đơn nào xoá được bao giờ.
func TestOrderService_Delete_CascadesOrderItems(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE id = \$1`).
		WithArgs("o1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_code", "status"}).
			AddRow("o1", "ORD-1", "pending"))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "payments"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "e_invoices"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "gift_card_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "order_items" WHERE order_id = \$1`).
		WithArgs("o1").
		WillReturnResult(sqlmock.NewResult(1, 3))
	mock.ExpectExec(`UPDATE "orders" SET "deleted_at"`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.Delete("o1")

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBookingService_Delete_RefusesWhenCheckedIn(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machine_bookings" WHERE id = \$1`).
		WithArgs("bk1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow("bk1", "checked_in"))

	err := svc.Delete("bk1")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRangBuoc))
	assert.Contains(t, err.Error(), "đã nhận máy")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBookingService_Delete_RefusesWhenDepositCharged(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machine_bookings" WHERE id = \$1`).
		WithArgs("bk1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "deposit_amount", "deposit_transaction_id"}).
			AddRow("bk1", "pending", int64(100000), "tx-1"))

	err := svc.Delete("bk1")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRangBuoc))
	assert.Contains(t, err.Error(), "100000")
	assert.Contains(t, err.Error(), "hoàn cọc")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- Bốn hàm từng xoá MÙ: ID sai phải trả lỗi, không còn báo thành công ----

func TestPricingService_DeleteMachinePrice_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPricingService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machine_prices" WHERE id = \$1`).
		WillReturnError(gorm.ErrRecordNotFound)

	err := svc.DeleteMachinePrice("khong-co-that", "u1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "không tìm thấy dòng giá")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPricingService_DeleteTimePricing_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPricingService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "time_based_pricings" WHERE id = \$1`).
		WillReturnError(gorm.ErrRecordNotFound)

	err := svc.DeleteTimePricing("khong-co-that", "u1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "không tìm thấy khung giá")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWebsiteBlockService_DeleteRule_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewWebsiteBlockService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "website_blocking_rules" WHERE id = \$1`).
		WillReturnError(gorm.ErrRecordNotFound)

	err := svc.DeleteRule("khong-co-that", "u1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "không tìm thấy luật")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Luật xoá được thì lịch áp dụng và ánh xạ nhóm máy phải đi cùng nó — cả hai
// xoá cứng nên luật xoá mềm không kéo theo được gì.
func TestWebsiteBlockService_DeleteRule_CascadesSchedulesAndMappings(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewWebsiteBlockService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "website_blocking_rules" WHERE id = \$1`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("r1", "Chặn game lậu"))

	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "website_blocking_schedules" WHERE rule_id = \$1`).
		WithArgs("r1").
		WillReturnResult(sqlmock.NewResult(1, 2))
	mock.ExpectExec(`DELETE FROM "website_rule_mappings" WHERE rule_id = \$1`).
		WithArgs("r1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE "website_blocking_rules" SET "deleted_at"`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.DeleteRule("r1", "u1")

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAppUpdateService_Delete_RefusesActiveRelease(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewAppUpdateService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "app_updates" WHERE id = \$1`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "version", "is_active"}).AddRow("a1", "1.2.0", true))

	err := svc.Delete("a1", "u1")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRangBuoc))
	assert.Contains(t, err.Error(), "đang phát hành")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- Hai hàm batch: phải đi qua hàm xoá đơn lẻ, không được chạy một câu
//     "WHERE id IN (...)" bỏ qua toàn bộ guard --------------------------------

// Chọn cả trang rồi bấm xoá không được phép quét sạch cả tài khoản còn chứng từ.
// Bản cũ chạy một câu IN duy nhất nên chuyện đó xảy ra được.
func TestSystemManageService_BatchDeleteUsers_StopsAtGuard(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSystemManageService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).
		WithArgs("u1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username"}).AddRow("u1", "nhanvien01"))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "shifts" WHERE user_id = \$1`).
		WithArgs("u1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
	// Không khai gì cho "u2": hàm phải dừng ngay, không đụng tới tài khoản sau.

	err := svc.BatchDeleteUsers([]string{"u1", "u2"})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRangBuoc))
	assert.Contains(t, err.Error(), "5 ca làm việc")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderService_BatchDelete_StopsAtGuard(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE id = \$1`).
		WithArgs("o1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_code", "status"}).
			AddRow("o1", "ORD-1", "completed"))
	// Không khai gì cho "o2".

	err := svc.BatchDelete([]string{"o1", "o2"})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRangBuoc))
	assert.Contains(t, err.Error(), "ORD-1")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- Vai trò: chỗ DUY NHẤT có khoá ngoại thật ------------------------------

// Xoá vai trò phải dọn hai bảng nối trước, trong cùng một transaction.
//
// user_roles và role_permissions là hai khoá ngoại thật duy nhất của hệ thống
// (many2many trong model/user.go), cả hai NO ACTION, mà Role thì xoá CỨNG. Bỏ
// bước dọn này thì PostgreSQL chặn bằng lỗi 23503 thô — và vì gần như vai trò
// nào cũng có quyền, chức năng xoá vai trò coi như hỏng.
func TestSystemManageService_DeleteRole_CleansJoinTables(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSystemManageService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "roles" WHERE id = \$1`).
		WithArgs("r1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("r1", "Thu ngân"))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "user_roles" JOIN users`).
		WithArgs("r1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "role_permissions" WHERE role_id = \$1`).
		WithArgs("r1").
		WillReturnResult(sqlmock.NewResult(1, 12))
	// Tài khoản xoá mềm vẫn giữ dòng user_roles của nó, phần đếm ở trên không
	// thấy nhưng khoá ngoại thì thấy.
	mock.ExpectExec(`DELETE FROM "user_roles" WHERE role_id = \$1`).
		WithArgs("r1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`DELETE FROM "roles" WHERE "roles"\."id" = \$1`).
		WithArgs("r1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.DeleteRole("r1")

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Vai trò còn tài khoản đang dùng thì chặn trước, không đụng tới bảng nối.
func TestSystemManageService_DeleteRole_RefusesWhenAssigned(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSystemManageService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "roles" WHERE id = \$1`).
		WithArgs("r1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("r1", "Thu ngân"))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "user_roles" JOIN users`).
		WithArgs("r1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

	err := svc.DeleteRole("r1")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRangBuoc))
	assert.Contains(t, err.Error(), "3 tài khoản")
	assert.NoError(t, mock.ExpectationsWereMet())
}
