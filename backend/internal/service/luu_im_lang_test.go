package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Các test dưới đây chốt loạt lỗi "báo lưu thành công mà không ghi gì": giá trị
// zero (false, 0, "") từng bị bỏ qua ở Create (do `default:` của cột) hoặc ở
// Update (do phép kiểm `!= ""` / `> 0`).

func TestProductService_Create_NgungBanGhiLaiFalse(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewProductService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "products"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "is_active"}).AddRow("p1", true))
	mock.ExpectExec(`UPDATE "products" SET "is_active"=\$1 WHERE`).
		WithArgs(false, "p1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(`SELECT \* FROM "product_options"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	dangBan := false
	res, err := svc.Create(&CreateProductRequest{Name: "Coca", Price: 10000, IsActive: &dangBan})
	require.NoError(t, err)
	assert.False(t, res.IsActive)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMemberService_UpdateGroup_DatVeKhong(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMemberService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "member_groups" WHERE id = \$1`).
		WithArgs("g1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "discount_percent", "min_spent"}).
			AddRow("g1", "VIP", float64(10), int64(50000)))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "member_groups" SET "discount_percent"=\$1,"min_spent"=\$2,"name"=\$3 WHERE`).
		WithArgs(float64(0), int64(0), "VIP", "g1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(`SELECT \* FROM "member_groups" WHERE id = \$1`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("g1", "VIP"))

	khong := 0.0
	var khongDong int64
	_, err := svc.UpdateGroup("g1", &UpdateGroupRequest{Name: "VIP", DiscountPercent: &khong, MinSpent: &khongDong})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMemberService_Update_XoaTrongSoDienThoai(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMemberService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "phone"}).AddRow("m1", "an", "0900000001"))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "members" SET "phone"=\$1,"updated_at"=\$2 WHERE`).
		WithArgs("", anyTime{}, "m1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	// GetByID sau khi sửa: lỗi đọc lại không phải điều test này quan tâm.
	mock.ExpectQuery(`SELECT \* FROM "members"`).WillReturnError(assert.AnError)

	_, _ = svc.Update("m1", &UpdateMemberRequest{Phone: strPtr("")})
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_Update_GiaKhong(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price"}).AddRow("c1", "Đêm", int64(50000)))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "combos" SET "description"=\$1,"price"=\$2,"validity_days"=\$3 WHERE`).
		WithArgs("", int64(0), 0, "c1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(`SELECT \* FROM "combos"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price"}).AddRow("c1", "Đêm", int64(0)))

	var gia int64
	ngay := 0
	_, err := svc.Update("c1", &UpdateComboRequest{Price: &gia, ValidityDays: &ngay, Description: strPtr("")})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCurfewService_Create_KhongGioVaTat(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewCurfewService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "curfew_policies"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "max_minor_hours", "is_active"}).AddRow("cf1", 2, true))
	mock.ExpectExec(`UPDATE "curfew_policies" SET "is_active"=\$1,"max_minor_hours"=\$2 WHERE`).
		WithArgs(false, 0, "cf1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	khong := 0
	tat := false
	res, err := svc.Create(&CreateCurfewRequest{DayOfWeek: 1, CurfewStart: "22:00", CurfewEnd: "06:00", MaxMinorHours: &khong, IsActive: &tat})
	require.NoError(t, err)
	assert.Equal(t, 0, res.MaxMinorHours)
	assert.False(t, res.IsActive)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderService_Update_XoaSoBanVaMay(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))

	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE id = \$1`).
		WithArgs("o1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "table_number"}).AddRow("o1", "pending", "B5"))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "orders" SET "machine_id"=\$1,"table_number"=\$2 WHERE`).
		WithArgs(nil, "", "o1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE id = \$1`).WillReturnError(assert.AnError)

	_, _ = svc.Update("o1", UpdateOrderRequest{TableNumber: strPtr(""), MachineID: strPtr("")}, "")
	assert.NoError(t, mock.ExpectationsWereMet())
}
