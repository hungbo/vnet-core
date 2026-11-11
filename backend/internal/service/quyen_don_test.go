package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/model"
)

// Xác nhận, huỷ, thu tiền: ba quyền riêng trên cùng một API đổi trạng thái.
func TestQuyenDoiTrangThai(t *testing.T) {
	cases := []struct{ loai, trangThai, quyen string }{
		{model.OrderTypeProduct, "confirmed", "orders.confirm"},
		{model.OrderTypeProduct, "cancelled", "orders.cancel"},
		{model.OrderTypeProduct, "completed", "orders.pay"},   // hoàn thành đơn món = thu tiền
		{model.OrderTypeTopup, "completed", "orders.confirm"}, // duyệt đơn nạp tiền mặt
		{model.OrderTypeTopup, "confirmed", "orders.confirm"},
		{model.OrderTypeTopup, "cancelled", "orders.cancel"}, // từ chối đơn nạp
	}
	for _, c := range cases {
		assert.Equal(t, c.quyen, quyenDoiTrangThai(c.loai, c.trangThai), "%s → %s", c.loai, c.trangThai)
	}
}

func TestOrderService_QuyenDoiTrangThai(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), nil)

	mock.ExpectQuery(`SELECT "id","order_type" FROM "orders" WHERE id = \$1 AND "orders"\."deleted_at" IS NULL ORDER BY "orders"\."id" LIMIT \$2`).
		WithArgs("o1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_type"}).AddRow("o1", model.OrderTypeProduct))

	code, err := svc.QuyenDoiTrangThai("o1", "completed")
	require.NoError(t, err)
	assert.Equal(t, "orders.pay", code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Khởi động: thêm hai mã mới nếu thiếu, rồi vai trò nào đang có orders.status
// nhận cả hai. Không gỡ mã cũ (quay về bản trước vẫn chạy).
func TestChuyenQuyenTrangThaiDon(t *testing.T) {
	db, mock := newMockDB(t)

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO permissions \(code, name, module\) VALUES \(\$1, \$2, \$3\)\s+ON CONFLICT \(code\) DO NOTHING`).
		WithArgs("orders.confirm", sqlmock.AnyArg(), "orders").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO permissions \(code, name, module\) VALUES \(\$1, \$2, \$3\)\s+ON CONFLICT \(code\) DO NOTHING`).
		WithArgs("orders.cancel", sqlmock.AnyArg(), "orders").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO role_permissions \(role_id, permission_id\)\s+SELECT rp\.role_id, moi\.id\s+FROM role_permissions rp\s+JOIN permissions cu ON cu\.id = rp\.permission_id AND cu\.code = 'orders\.status'\s+JOIN permissions moi ON moi\.code IN \('orders\.confirm', 'orders\.cancel'\)\s+ON CONFLICT DO NOTHING`).
		WillReturnResult(sqlmock.NewResult(0, 4))
	mock.ExpectCommit()

	require.NoError(t, ChuyenQuyenTrangThaiDon(db))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Trang Phân quyền không hiện mã đã bỏ.
func TestGetAllPermissions_AnMaDaBo(t *testing.T) {
	db, mock := newMockDB(t)
	svc := &SystemManageService{db: db}

	mock.ExpectQuery(`SELECT \* FROM "permissions" WHERE code NOT IN \(\$1\) ORDER BY module, code`).
		WithArgs("orders.status").
		WillReturnRows(sqlmock.NewRows([]string{"id", "code"}).AddRow("p1", "orders.confirm"))

	got, err := svc.GetAllPermissions()
	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}
