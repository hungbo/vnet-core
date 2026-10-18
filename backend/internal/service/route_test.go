package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// tenMenu gom tên mọi mục menu người dùng thấy: con của nhóm vnet và các nhóm
// cấp trên.
func tenMenu(perms []string) map[string]bool {
	out := map[string]bool{}
	for _, r := range (&RouteService{}).GetUserRoutes(perms).Routes {
		out[r.Name] = true
		for _, c := range r.Children {
			out[c.Name] = true
		}
	}
	return out
}

// Router đòi client.admin cộng mã riêng cho Sao lưu, Nhật ký hoạt động, Thông
// báo và /systemManage; menu phải đòi y như vậy.
func TestMenuBackOfficeDoiCaClientAdminLanMaRieng(t *testing.T) {
	own := []string{"backups.view", "audit.view", "notifications.view", "system.users.view"}
	trang := []string{"vnet_backups", "vnet_audit", "vnet_notifications", "system", "system_user"}

	chiMaRieng := tenMenu(own)
	for _, ten := range trang {
		assert.False(t, chiMaRieng[ten], "%s hiện khi thiếu client.admin", ten)
	}

	chiCong := tenMenu([]string{"client.admin"})
	for _, ten := range trang {
		assert.False(t, chiCong[ten], "%s hiện khi thiếu mã riêng", ten)
	}

	du := tenMenu(append([]string{"client.admin"}, own...))
	for _, ten := range trang {
		assert.True(t, du[ten], "%s phải hiện khi có cả hai mã", ten)
	}

	for _, ten := range trang {
		assert.True(t, tenMenu([]string{"*"})[ten], "chủ (*) phải thấy %s", ten)
	}
}

// Nhật ký lỗi không nằm sau client.admin (API /error-reports chỉ đòi errors.view).
func TestMenuNhatKyLoiKhongDoiClientAdmin(t *testing.T) {
	assert.True(t, tenMenu([]string{"errors.view"})["vnet_error-logs"])
}
