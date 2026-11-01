package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		"ghi khoa dt thang": {"device_key": "abc"},
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

func TestQRPay_VerifyDevice(t *testing.T) {
	db, mock := newMockDB(t)
	now := time.Date(2026, 10, 31, 18, 0, 0, 0, time.UTC)
	s := NewQRPayService(db, nil, nil, nil, nil)
	s.now = func() time.Time { return now }
	const key = "khoa-bi-mat"
	body := []byte(`{"package":"com.mbmobile"}`)
	ts := strconv.FormatInt(now.UnixMilli(), 10)
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(ts + "\n"))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	expectKey := func() {
		mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1 AND key = \$2`).
			WithArgs(QRPaymentGroup, QRDeviceKeyKey, 1).
			WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}).AddRow(QRPaymentGroup, QRDeviceKeyKey, `"`+key+`"`))
	}

	expectKey()
	assert.NoError(t, s.VerifyDevice(ts, sig, body))

	expectKey()
	assert.ErrorIs(t, s.VerifyDevice(ts, sig, []byte(`{"package":"com.VCB"}`)), ErrQRBadSignature)

	expectKey()
	old := strconv.FormatInt(now.Add(-10*time.Minute).UnixMilli(), 10)
	assert.Error(t, s.VerifyDevice(old, sig, body))

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQRPay_PairHuyMaSauNamLanSai(t *testing.T) {
	db, mock := newMockDB(t)
	s := NewQRPayService(db, nil, &AuditService{db: db}, nil, nil)
	now := time.Date(2026, 10, 31, 18, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	s.pairCode, s.pairExpires = "123456", now.Add(qrPairCodeTTL)

	for i := 0; i < qrPairMaxFails; i++ {
		_, err := s.Pair(fmt.Sprintf("%06d", i))
		require.Error(t, err)
	}
	// Mã đúng cũng không dùng được nữa: không có truy vấn nào xuống database.
	_, err := s.Pair("123456")
	assert.Error(t, err)
	assert.Empty(t, s.pairCode)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQRPay_PhoneOnline(t *testing.T) {
	s := NewQRPayService(nil, nil, nil, nil, nil)
	now := time.Date(2026, 10, 31, 19, 0, 0, 0, time.UTC)

	// App cũ chỉ có nhịp HTTP: online nếu nhịp gần nhất chưa quá 3 phút.
	s.phone.LastSeen = now.Add(-2 * time.Minute)
	assert.True(t, s.phoneOnline(now))
	s.phone.LastSeen = now.Add(-4 * time.Minute)
	assert.False(t, s.phoneOnline(now))

	// Đã từng nối WebSocket: rớt là tính theo lúc rớt, không theo nhịp.
	s.wsCapable = true
	s.phone.LastSeen = now.Add(-10 * time.Second)
	s.wsLostAt = now.Add(-5 * time.Second)
	assert.True(t, s.phoneOnline(now), "còn trong thời gian chờ nối lại")
	s.wsLostAt = now.Add(-15 * time.Second)
	assert.False(t, s.phoneOnline(now), "rớt quá 10 giây là mất kết nối, dù nhịp HTTP mới 10 giây")
}
