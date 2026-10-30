package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/vnet/core/internal/model"
)

// Nhân viên quầy (casang, cachieu) không điền họ tên: cột "Người thực hiện" của
// phiếu kho từng hiện "-". Phải rơi về tên đăng nhập như trang Đơn hàng/Ca.
func TestStockTransactionToResponse_TenDangNhapKhiThieuHoTen(t *testing.T) {
	db, mock := newMockDB(t)
	uid := "u-casang"

	// Unscoped: nhân viên đã bị xoá vẫn phải hiện tên trên phiếu cũ.
	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1 ORDER BY "users"."id" LIMIT \$2`).
		WithArgs(uid, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "full_name"}).AddRow(uid, "casang", ""))

	res := stockTransactionToResponse(db, model.StockTransaction{ID: "t1", TransactionType: "outbound", CreatedBy: &uid})
	assert.Equal(t, "casang", res.CreatedByName)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStockTransactionToResponse_UuTienHoTen(t *testing.T) {
	db, mock := newMockDB(t)
	uid := "u-admin"
	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).
		WithArgs(uid, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "full_name"}).AddRow(uid, "admin", "Admin"))

	res := stockTransactionToResponse(db, model.StockTransaction{ID: "t2", TransactionType: "inbound", CreatedBy: &uid})
	assert.Equal(t, "Admin", res.CreatedByName)
	assert.NoError(t, mock.ExpectationsWereMet())
}
