package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreparePaymentSettings_SinhMaVaGiuTienMat(t *testing.T) {
	out, err := preparePaymentSettings(map[string]interface{}{
		"methods": []interface{}{
			map[string]interface{}{"code": "transfer", "name": "Chuyển khoản", "enabled": false},
			map[string]interface{}{"name": " Quẹt thẻ POS ", "enabled": true},
		},
	})
	require.NoError(t, err)
	list := out["methods"].([]PaymentMethodOption)
	require.Len(t, list, 3)
	// Lưu thiếu tiền mặt thì tự thêm lại, luôn bật.
	assert.Equal(t, PaymentMethodOption{Code: "cash", Name: "Tiền mặt", Enabled: true}, list[0])
	assert.Equal(t, "quet_the_pos", list[2].Code)
	assert.Equal(t, "Quẹt thẻ POS", list[2].Name)
}

func TestPreparePaymentSettings_TuChoi(t *testing.T) {
	cases := map[string][]interface{}{
		"ma danh rieng": {map[string]interface{}{"code": "balance", "name": "Số dư"}},
		"trung ma":      {map[string]interface{}{"code": "momo", "name": "MoMo"}, map[string]interface{}{"code": "momo", "name": "MoMo 2"}},
		"thieu ten":     {map[string]interface{}{"code": "momo", "name": "  "}},
	}
	for name, methods := range cases {
		_, err := preparePaymentSettings(map[string]interface{}{"methods": methods})
		assert.Error(t, err, name)
	}
}

func TestPreparePaymentSettings_TienMatKhongTatDuoc(t *testing.T) {
	out, err := preparePaymentSettings(map[string]interface{}{
		"methods": []interface{}{map[string]interface{}{"code": "cash", "name": "Tiền mặt", "enabled": false}},
	})
	require.NoError(t, err)
	assert.True(t, out["methods"].([]PaymentMethodOption)[0].Enabled)
}

func TestKiemTraPhuongThucThanhToan(t *testing.T) {
	db, mock := newMockDB(t)
	rows := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"group_name", "key", "value"}).
			AddRow("payment", "methods", `[{"code":"cash","name":"Tiền mặt","enabled":true},{"code":"transfer","name":"Chuyển khoản","enabled":false},{"code":"momo","name":"MoMo","enabled":true}]`)
	}
	for i := 0; i < 3; i++ {
		mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1`).WithArgs("payment").WillReturnRows(rows())
	}

	assert.NoError(t, kiemTraPhuongThucThanhToan(db, "cash"))
	assert.NoError(t, kiemTraPhuongThucThanhToan(db, "momo"))
	assert.ErrorContains(t, kiemTraPhuongThucThanhToan(db, "transfer"), "đang tắt")
	assert.ErrorContains(t, kiemTraPhuongThucThanhToan(db, "ewallet"), "không có")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentMethods_MacDinhKhiChuaCai(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1`).WithArgs("payment").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}))
	assert.Equal(t, defaultPaymentMethods, PaymentMethods(db))
}
