package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSettingsService_List(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSettingsService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "system_settings" ORDER BY group_name, key`).
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}).
			AddRow("general", "site_name", "VNET").
			AddRow("general", "timezone", "Asia/HCM").
			AddRow("pricing", "vat_rate", "10"))

	result, err := svc.List()
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Len(t, result["general"], 2)
	assert.Len(t, result["pricing"], 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSettingsService_List_Empty(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSettingsService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "system_settings" ORDER BY group_name, key`).
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}))

	result, err := svc.List()
	require.NoError(t, err)
	assert.Len(t, result, 0)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSettingsService_GetByGroup_Found(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSettingsService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1 ORDER BY key`).
		WithArgs("general").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}).
			AddRow("general", "site_name", "VNET").
			AddRow("general", "timezone", "Asia/HCM"))

	result, err := svc.GetByGroup("general")
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Nhóm rỗng nay trả về danh sách rỗng, không còn là lỗi: đó là trạng thái lần
// đầu bình thường của một tab cài đặt chưa ai đụng tới.
func TestSettingsService_GetByGroup_NhomRongTraDanhSachRong(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSettingsService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1 ORDER BY key`).
		WithArgs("nonexistent").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}))

	result, err := svc.GetByGroup("nonexistent")
	require.NoError(t, err)
	assert.Empty(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSettingsService_Update_CreateNew(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSettingsService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1 AND key = \$2 ORDER BY "system_settings"."id" LIMIT \$3`).
		WithArgs("general", "new_key", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "system_settings"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1 ORDER BY key`).
		WithArgs("general").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}).
			AddRow("general", "new_key", "test_value"))

	result, err := svc.Update("general", map[string]interface{}{"new_key": "test_value"})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSettingsService_Update_Existing(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSettingsService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1 AND key = \$2 ORDER BY "system_settings"."id" LIMIT \$3`).
		WithArgs("general", "site_name", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "group_name", "key", "value"}).AddRow("s1", "general", "site_name", "VNET"))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "system_settings" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1 ORDER BY key`).
		WithArgs("general").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}).
			AddRow("general", "site_name", "VNET 2.0"))

	result, err := svc.Update("general", map[string]interface{}{"site_name": "VNET 2.0"})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// The value column is jsonb, so a bare string was rejected by PostgreSQL and
// saving any text setting always failed.
func TestSettingsService_Update_StoresValidJSON(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSettingsService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1 AND key = \$2 ORDER BY "system_settings"\."id" LIMIT \$3`).
		WithArgs("general", "store_name", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	// Quoted: "Tiem net ABC" is valid JSON, Tiem net ABC is not.
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "system_settings"`).
		WithArgs("general", "store_name", `"Tiem net ABC"`, "").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("s1"))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1 ORDER BY key`).
		WithArgs("general").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}).
			AddRow("general", "store_name", `"Tiem net ABC"`))

	res, err := svc.Update("general", map[string]interface{}{"store_name": "Tiem net ABC"})

	require.NoError(t, err)
	require.Len(t, res, 1)
	// Read back as plain text, so a form shows the value rather than its quotes.
	assert.Equal(t, "Tiem net ABC", res[0].Value)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDecodeSettingValue(t *testing.T) {
	assert.Equal(t, "abc", decodeSettingValue(`"abc"`))
	assert.Equal(t, `{"values":[1,2]}`, decodeSettingValue(`{"values":[1,2]}`))
	assert.Equal(t, "123", decodeSettingValue(`123`))
}

// Giá trị trong cột jsonb được decodeSettingValue trả về dạng CHUỖI. Kiểm bằng
// truthiness thì "false" — một chuỗi khác rỗng — vẫn ra "bật", và công tắc ở
// trang quản trị sẽ không bao giờ tắt được gì.
func TestSettingBool(t *testing.T) {
	cases := []struct {
		name string
		m    map[string]string
		def  bool
		want bool
	}{
		{"thiếu khoá thì lấy mặc định", map[string]string{}, true, true},
		{"rỗng thì lấy mặc định", map[string]string{"k": ""}, true, true},
		{"rác thì lấy mặc định", map[string]string{"k": "bật"}, true, true},
		{`chuỗi "false" phải TẮT`, map[string]string{"k": "false"}, true, false},
		{`chuỗi "true" phải BẬT`, map[string]string{"k": "true"}, false, true},
		{"có khoảng trắng vẫn đọc được", map[string]string{"k": " false "}, true, false},
		{"chấp nhận 0/1", map[string]string{"k": "0"}, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, settingBool(c.m, "k", c.def))
		})
	}
}
