package handler

import (
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/utils"
)

// Mật khẩu đúng mà còn bước mã 2 bước thì đăng nhập CHƯA xong: bộ chặn theo tên + IP
// phải giữ nguyên. Xoá ở đây thì sai mã vài lần rồi gõ lại mật khẩu là có lượt dò mới —
// đoán mã 6 số không giới hạn.
func TestAuthHandler_Login_MatKhauDungConBuocMaThiKhongXoaBoChan(t *testing.T) {
	db, mock := newTestDB(t)
	handler, _ := setupAuthHandler(db)
	router := newLoginRouter(handler)

	hash, _ := utils.HashPassword("matkhau-dung")
	mock.ExpectQuery(usersQuery).WithArgs("admin", 1).WillReturnRows(
		sqlmock.NewRows([]string{"id", "username", "password_hash", "is_active", "totp_secret", "totp_enabled_at"}).
			AddRow("u1", "admin", hash, true, "JBSWY3DPEHPK3PXP", time.Now()))
	mock.ExpectQuery(`SELECT \* FROM "user_roles"`).WithArgs("u1").WillReturnRows(sqlmock.NewRows([]string{"user_id", "role_id"}))
	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1`).WithArgs(service.FeaturesGroup).
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}).AddRow(service.FeaturesGroup, service.TwoFactorFeatureKey, `"true"`))

	// Các lượt sai mã trước đó đã đẩy bộ chặn tới sát ngưỡng.
	key := loginKey("admin", "192.0.2.10")
	for i := 0; i < loginMaxAttempts-1; i++ {
		handler.throttle.begin(key)
	}

	w := postAs(router, "/api/auth/login", "192.0.2.10:5555", `{"username":"admin","password":"matkhau-dung"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"need_totp":true`)

	_, ok := handler.throttle.begin(key)
	assert.False(t, ok, "lượt mật khẩu đúng nhưng chưa qua mã vẫn phải tính, không xoá bộ chặn")
}
