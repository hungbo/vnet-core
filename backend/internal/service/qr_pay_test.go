package service

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vnet/core/internal/model"
)

func TestKiemTraCaiDatQR(t *testing.T) {
	ok := map[string]interface{}{"bank": "mb", "account_no": " 0123456789 ", "expire_minutes": "10", "account_name": "QUAN NET"}
	require.NoError(t, kiemTraCaiDatQR(ok))
	assert.Equal(t, "0123456789", ok["account_no"])
	assert.Equal(t, int64(10), ok["expire_minutes"])

	for name, in := range map[string]map[string]interface{}{
		"ngan hang la":      {"bank": "tcb"},
		"so tk co chu":      {"account_no": "12ab5678"},
		"so tk qua ngan":    {"account_no": "12345"},
		"han qua ngan":      {"expire_minutes": 1},
		"han qua dai":       {"expire_minutes": 120},
	} {
		assert.Error(t, kiemTraCaiDatQR(in), name)
	}
}

func TestQRConfigFrom_MacDinh(t *testing.T) {
	c := qrConfigFrom(map[string]string{"enabled": "true", "bank": "vcb", "account_no": "123456"})
	assert.True(t, c.Enabled)
	assert.True(t, c.configured())
	assert.Equal(t, int64(qrDefaultExpireMinutes), c.ExpireMinutes)
	// Mặc định chỉ gợi ý, nhân viên tự bấm duyệt.
	assert.False(t, c.AutoConfirm)
	assert.False(t, qrConfigFrom(map[string]string{"bank": "tcb", "account_no": "123456"}).configured())
}

func TestTaoMaNap(t *testing.T) {
	for i := 0; i < 200; i++ {
		c, err := taoMaNap()
		require.NoError(t, err)
		assert.Equal(t, []string{c}, qrCodesIn("ND: "+c))
	}
}

func TestQRPay_SourceOnline(t *testing.T) {
	now := time.Now()
	s := &QRPayService{}
	assert.False(t, s.sourceOnline(now), "chưa đọc được lần nào")
	s.lastSync = now.Add(-20 * time.Second)
	assert.True(t, s.sourceOnline(now))
	s.lastSync = now.Add(-51 * time.Second)
	assert.False(t, s.sourceOnline(now), "lỡ 3 lần đọc liên tiếp")
}

func TestQRPay_DeleteNotification(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	const chon = `SELECT \* FROM "bank_notifications" WHERE id = \$1 AND "bank_notifications"."deleted_at" IS NULL`
	const xoa = `UPDATE "bank_notifications" SET "deleted_at"=\$1 WHERE \(id = \$2 AND status NOT IN \(\$3,\$4\)\) AND "bank_notifications"."deleted_at" IS NULL`
	cols := []string{"id", "amount", "incoming", "status", "content"}

	t.Run("chua cong tien thi xoa mem", func(t *testing.T) {
		db, mock := newMockDB(t)
		s := NewQRPayService(db, nil, &AuditService{db: db}, nil, nil)
		mock.ExpectQuery(chon).WillReturnRows(sqlmock.NewRows(cols).AddRow(id, 50000, true, model.BankNotifUnmatched, "chuyen nham"))
		mock.ExpectBegin()
		mock.ExpectExec(xoa).WithArgs(anyTime{}, id, model.BankNotifMatched, model.BankNotifAssigned).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		mock.ExpectBegin()
		mock.ExpectQuery(`INSERT INTO "audit_logs"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("a"))
		mock.ExpectCommit()

		require.NoError(t, s.DeleteNotification(id, "22222222-2222-2222-2222-222222222222"))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("da cong tien thi khong xoa", func(t *testing.T) {
		db, mock := newMockDB(t)
		s := NewQRPayService(db, nil, &AuditService{db: db}, nil, nil)
		mock.ExpectQuery(chon).WillReturnRows(sqlmock.NewRows(cols).AddRow(id, 50000, true, model.BankNotifMatched, "VNET123"))
		mock.ExpectBegin()
		mock.ExpectExec(xoa).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectCommit()

		assert.ErrorContains(t, s.DeleteNotification(id, ""), "đã cộng tiền")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("ma khong hop le khong cham database", func(t *testing.T) {
		db, mock := newMockDB(t)
		s := NewQRPayService(db, nil, &AuditService{db: db}, nil, nil)
		assert.Error(t, s.DeleteNotification("abc", ""))
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
