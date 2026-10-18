package service

import (
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mẫu chung với client/src/localadmin_test.go: máy trạm phải nhận đúng chuỗi
// này cho mật khẩu "Quantri@2026". Đổi tham số băm ở một bên là vỡ test bên kia.
const localAdminVector = "pbkdf2-sha256$210000$AAECAwQFBgcICQoLDA0ODw$MKWV8XVcE7Dxum7d8y+ShLzMKHmnfcZUwtxeP2Byz8A"

func TestHashLocalAdmin_KhopMauVoiMayTram(t *testing.T) {
	salt := make([]byte, 16)
	for i := range salt {
		salt[i] = byte(i)
	}
	got, err := hashLocalAdminWithSalt("  Quantri@2026 ", salt)
	require.NoError(t, err)
	assert.Equal(t, localAdminVector, got, "khoảng trắng hai đầu bị bỏ như máy trạm")
}

func TestPrepareClientSettings(t *testing.T) {
	coSan := map[string]string{LocalAdminUserKey: "kythuat", LocalAdminHashKey: localAdminVector}

	t.Run("đặt mới: băm, bỏ mật khẩu trần", func(t *testing.T) {
		out, err := prepareClientSettings(map[string]string{}, map[string]interface{}{
			LocalAdminUserKey: " kythuat ", LocalAdminPassKey: "Quantri@2026", "tamper_action": "restart",
		})
		require.NoError(t, err)
		assert.NotContains(t, out, LocalAdminPassKey)
		assert.Equal(t, "kythuat", out[LocalAdminUserKey])
		assert.True(t, strings.HasPrefix(out[LocalAdminHashKey].(string), "pbkdf2-sha256$210000$"))
		assert.Equal(t, "restart", out["tamper_action"])
	})
	t.Run("để trống mật khẩu khi đã có: giữ nguyên băm", func(t *testing.T) {
		out, err := prepareClientSettings(coSan, map[string]interface{}{LocalAdminUserKey: "kythuat2", LocalAdminPassKey: ""})
		require.NoError(t, err)
		assert.NotContains(t, out, LocalAdminHashKey)
		assert.Equal(t, "kythuat2", out[LocalAdminUserKey])
	})
	t.Run("xoá tên: xoá luôn băm", func(t *testing.T) {
		out, err := prepareClientSettings(coSan, map[string]interface{}{LocalAdminUserKey: ""})
		require.NoError(t, err)
		assert.Equal(t, "", out[LocalAdminHashKey])
	})
	t.Run("không đụng tới tài khoản: không ghi gì thêm", func(t *testing.T) {
		out, err := prepareClientSettings(coSan, map[string]interface{}{"tamper_action": "shutdown"})
		require.NoError(t, err)
		assert.Equal(t, map[string]interface{}{"tamper_action": "shutdown"}, out)
	})

	loi := []struct {
		ten     string
		current map[string]string
		in      map[string]interface{}
		chua    string
	}{
		{"có tên mà chưa từng có mật khẩu", map[string]string{}, map[string]interface{}{LocalAdminUserKey: "kythuat"}, "đặt mật khẩu"},
		{"mật khẩu ngắn", map[string]string{}, map[string]interface{}{LocalAdminUserKey: "kythuat", LocalAdminPassKey: "12345"}, "ít nhất 6"},
		{"mật khẩu mà không tên", map[string]string{}, map[string]interface{}{LocalAdminPassKey: "Quantri@2026"}, "tên tài khoản"},
		{"ghi thẳng băm", coSan, map[string]interface{}{LocalAdminHashKey: "x"}, "không ghi thẳng"},
		{"tên có khoảng trắng", map[string]string{}, map[string]interface{}{LocalAdminUserKey: "ky thuat", LocalAdminPassKey: "Quantri@2026"}, "khoảng trắng"},
	}
	for _, c := range loi {
		t.Run(c.ten, func(t *testing.T) {
			_, err := prepareClientSettings(c.current, c.in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.chua)
		})
	}
}

// GET /settings/:group mở cho cả hội viên: băm không bao giờ được lọt ra.
func TestSettingsService_GetByGroup_GiauBamQuanTriMayTram(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSettingsService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1 ORDER BY key`).
		WithArgs(ClientGroup).
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}).
			AddRow(ClientGroup, LocalAdminHashKey, `"`+localAdminVector+`"`).
			AddRow(ClientGroup, LocalAdminUserKey, `"kythuat"`))

	result, err := svc.GetByGroup(ClientGroup)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, LocalAdminUserKey, result[0].Key)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClientPolicyFrom_TaiKhoanQuanTri(t *testing.T) {
	p := clientPolicyFrom(map[string]string{LocalAdminUserKey: "kythuat", LocalAdminHashKey: localAdminVector})
	assert.Equal(t, "kythuat", p.LocalAdminUsername)
	assert.Equal(t, localAdminVector, p.LocalAdminHash)

	p = clientPolicyFrom(map[string]string{LocalAdminUserKey: "kythuat"})
	assert.Empty(t, p.LocalAdminUsername, "thiếu băm thì không gửi nửa tài khoản")
	assert.Empty(t, p.LocalAdminHash)
}
