package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/model"
)

// Bảng mốc giống hình nền của quán, thêm một mốc phần trăm.
var mocMau = []TopupBonusTier{
	{Min: 50000, Type: TopupBonusFixed, Value: 10000},
	{Min: 200000, Type: TopupBonusFixed, Value: 50000},
	{Min: 500000, Type: TopupBonusPercent, Value: 30},
	{Min: 1000000, Type: TopupBonusPercent, Value: 100},
}

func TestTinhTienTangNap(t *testing.T) {
	cases := []struct {
		ten    string
		amount int64
		muon   int64
		moc    int64
	}{
		{"dưới mốc thấp nhất", 49000, 0, 0},
		{"đúng mốc cố định", 50000, 10000, 50000},
		{"giữa hai mốc lấy mốc thấp hơn", 300000, 50000, 200000},
		{"mốc phần trăm tính trên số nạp", 600000, 180000, 500000},
		{"tặng 100%", 1000000, 1000000, 1000000},
		{"phần lẻ làm tròn xuống", 500001, 150000, 500000},
	}
	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			tang, moc := tinhTienTangNap(mocMau, c.amount)
			assert.Equal(t, c.muon, tang)
			if c.moc == 0 {
				assert.Nil(t, moc)
			} else {
				require.NotNil(t, moc)
				assert.Equal(t, c.moc, moc.Min)
			}
		})
	}

	tang, moc := tinhTienTangNap(nil, 1000000)
	assert.Zero(t, tang)
	assert.Nil(t, moc)
}

func TestChuanHoaMocNapTang(t *testing.T) {
	// Admin gửi lên dạng JSON đã giải mã: mảng các map, số là float64.
	raw := []interface{}{
		map[string]interface{}{"min": float64(200000), "type": "fixed", "value": float64(50000)},
		map[string]interface{}{"min": float64(50000), "type": "percent", "value": float64(20)},
	}
	tiers, err := chuanHoaMocNapTang(raw)
	require.NoError(t, err)
	assert.Equal(t, []TopupBonusTier{
		{Min: 50000, Type: TopupBonusPercent, Value: 20},
		{Min: 200000, Type: TopupBonusFixed, Value: 50000},
	}, tiers)

	tiers, err = chuanHoaMocNapTang(nil)
	require.NoError(t, err)
	assert.Equal(t, []TopupBonusTier{}, tiers, "bảng rỗng là tắt tính năng")

	loi := []struct {
		ten string
		moc []TopupBonusTier
		chu string
	}{
		{"mốc dưới 1.000đ", []TopupBonusTier{{Min: 500, Type: "fixed", Value: 100}}, "từ 1.000đ"},
		{"mốc trùng", []TopupBonusTier{{Min: 50000, Type: "fixed", Value: 1}, {Min: 50000, Type: "percent", Value: 1}}, "bị trùng"},
		{"tặng 0đ", []TopupBonusTier{{Min: 50000, Type: "fixed", Value: 0}}, "lớn hơn 0"},
		{"phần trăm quá lớn", []TopupBonusTier{{Min: 50000, Type: "percent", Value: 1001}}, "từ 1 đến 1000"},
		{"kiểu lạ", []TopupBonusTier{{Min: 50000, Type: "x2", Value: 1}}, "kiểu tặng"},
	}
	for _, c := range loi {
		t.Run(c.ten, func(t *testing.T) {
			_, err := chuanHoaMocNapTang(c.moc)
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.chu)
		})
	}
}

const mocJSON = `[{"min":50000,"type":"fixed","value":10000},{"min":200000,"type":"percent","value":100}]`

func expectMocNapTang(mock sqlmock.Sqlmock, value string) {
	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1`).
		WithArgs(TopupGroup).
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}).AddRow(TopupGroup, TopupBonusTiersKey, value))
}

// Nạp ở quầy đạt mốc: ghi thêm một dòng topup_bonus vào sổ (phương thức
// bonus_balance để két và doanh thu không đếm) và cộng vào số dư khuyến mãi.
func TestMemberService_Topup_NapTang(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMemberService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 .* FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance", "bonus_balance"}).AddRow("mem-1", int64(20000), int64(5000)))
	expectMocNapTang(mock, mocJSON)
	mock.ExpectQuery(`INSERT INTO "member_transactions"`).
		WithArgs("mem-1", "topup", int64(300000), int64(20000), int64(320000), int64(5000), int64(5000), "cash", nil, "", "user-1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("tx-1"))
	mock.ExpectQuery(`INSERT INTO "member_transactions"`).
		WithArgs("mem-1", "topup_bonus", int64(300000), int64(320000), int64(320000), int64(5000), int64(305000),
			"bonus_balance", nil, "Nạp tặng: nạp 300.000đ, mốc từ 200.000đ tặng 100%", "user-1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("tx-2"))
	mock.ExpectExec(`UPDATE "members" SET "balance"=\$1,"bonus_balance"=\$2,"updated_at"=\$3`).
		WithArgs(int64(320000), int64(305000), anyTime{}, "mem-1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("mem-1"))

	_, err := svc.Topup("mem-1", &TopupRequest{Amount: 300000, PaymentMethod: "cash"}, "user-1")

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// "Tặng tiền" đi đường bonus_balance: không đọc bảng mốc, không tặng thêm.
func TestMemberService_Topup_TangTienKhongNapTang(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMemberService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 .* FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance", "bonus_balance"}).AddRow("mem-1", int64(0), int64(0)))
	mock.ExpectQuery(`INSERT INTO "member_transactions"`).
		WithArgs("mem-1", "topup_bonus", int64(1000000), int64(0), int64(0), int64(0), int64(1000000), "bonus_balance", nil, "", "user-1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("tx-1"))
	mock.ExpectExec(`UPDATE "members" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("mem-1"))

	_, err := svc.Topup("mem-1", &TopupRequest{Amount: 1000000, PaymentMethod: "bonus_balance"}, "user-1")

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Đơn nạp từ máy trạm, nhân viên duyệt: cũng được tặng, và câu UPDATE hội viên
// cộng luôn bonus_balance.
func TestOrderService_processTopupOrder_NapTang(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewOrderService(db, nil, NewAuditService(db), NewInventoryService(db, NewAuditService(db)))
	const nhanVien = "11111111-1111-1111-1111-111111111111"

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "orders" WHERE id = \$1 .* FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_code", "status", "order_type", "member_id", "final_amount", "payment_method"}).
			AddRow("o1", "ORD-1", "pending", "topup", "mem-1", int64(50000), "cash"))
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 .* FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance", "bonus_balance"}).AddRow("mem-1", int64(0), int64(2000)))
	expectMocNapTang(mock, mocJSON)
	mock.ExpectQuery(`INSERT INTO "member_transactions"`).
		WithArgs("mem-1", "topup", int64(50000), int64(0), int64(50000), int64(0), int64(0), "cash", nil, "Nạp tiền qua đơn hàng ORD-1", nhanVien).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("tx-1"))
	mock.ExpectQuery(`INSERT INTO "member_transactions"`).
		WithArgs("mem-1", "topup_bonus", int64(10000), int64(50000), int64(50000), int64(2000), int64(12000),
			"bonus_balance", nil, "Nạp tặng: nạp 50.000đ, mốc từ 50.000đ tặng 10.000đ", nhanVien).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("tx-2"))
	mock.ExpectExec(`UPDATE "members" SET "balance"=\$1,"bonus_balance"=\$2,"updated_at"=\$3`).
		WithArgs(int64(50000), int64(12000), anyTime{}, "mem-1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE "orders" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.processTopupOrder(&model.Order{ID: "o1"}, nhanVien)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Lưu cài đặt nạp tiền: bảng mốc được kiểm trước khi ghi, sai thì không ghi gì.
func TestSettingsService_Update_TuChoiMocNapTangSai(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSettingsService(db, NewAuditService(db))

	_, err := svc.Update(TopupGroup, map[string]interface{}{
		TopupBonusTiersKey: []interface{}{map[string]interface{}{"min": float64(50000), "type": "percent", "value": float64(0)}},
	})

	require.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
