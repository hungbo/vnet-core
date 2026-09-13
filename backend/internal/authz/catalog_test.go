package authz

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mã quyền gõ sai trong router = API đó không ai cấp quyền được, tức tính năng
// bị khoá cứng mà không có thông báo nào. Test này đọc thẳng file router và đối
// chiếu với danh mục.
func TestRouterPermissionsCoNamTrongDanhMuc(t *testing.T) {
	src, err := os.ReadFile("../router/router.go")
	require.NoError(t, err)

	codes := regexp.MustCompile(`PermissionRequired\("([^"]+)"\)`).FindAllStringSubmatch(string(src), -1)
	require.NotEmpty(t, codes, "không đọc được mã quyền nào từ router")

	for _, m := range codes {
		assert.True(t, Has(m[1]), "router dùng mã %q nhưng danh mục không có", m[1])
	}
}

// Mọi API của nhân viên phải có quyền riêng. Hai ngoại lệ nằm trong danh sách
// dưới đây: gác chúng thì không ai đăng nhập vào được.
func TestMoiRouteNhanVienDeuCoQuyen(t *testing.T) {
	src, err := os.ReadFile("../router/router.go")
	require.NoError(t, err)
	lines := strings.Split(string(src), "\n")

	// Nhóm có StaffOnly / PermissionRequired đặt ở cấp group.
	groupStaff := map[string]bool{}
	groupPerm := map[string]bool{}
	groupDef := regexp.MustCompile(`^\s*(\w+)\s*:=\s*\w+\.Group\("[^"]*",(.*)$`)
	routeDef := regexp.MustCompile(`^\s*(\w+)\.(GET|POST|PUT|DELETE|PATCH)\("([^"]*)",(.*)$`)

	ngoaiLe := map[string]bool{
		"/getUserRoutes": true, // menu của chính người đang đăng nhập
		"/permissions":   true, // danh sách quyền của chính người đang đăng nhập
	}

	for _, l := range lines {
		if m := groupDef.FindStringSubmatch(l); m != nil {
			if strings.Contains(m[2], "StaffOnly") {
				groupStaff[m[1]] = true
			}
			if strings.Contains(m[2], "PermissionRequired") {
				groupPerm[m[1]] = true
			}
		}
	}

	var thieu []string
	for _, l := range lines {
		m := routeDef.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		group, path, rest := m[1], m[3], m[4]
		staff := strings.Contains(rest, "StaffOnly") || groupStaff[group]
		perm := strings.Contains(rest, "PermissionRequired") || groupPerm[group]
		if staff && !perm && !ngoaiLe[path] {
			thieu = append(thieu, group+path)
		}
	}

	assert.Empty(t, thieu, "route dành cho nhân viên nhưng chưa gác quyền: %v", thieu)
}

// Quyền mặc định của hai vai trò phải nằm trong danh mục — sai một mã là vai trò
// đó thiếu quyền mà không ai biết.
func TestQuyenMacDinhNamTrongDanhMuc(t *testing.T) {
	for _, code := range append(ManagerCodes(), StaffCodes()...) {
		assert.True(t, Has(code), "quyền mặc định %q không có trong danh mục", code)
	}
}

// Quản lý không được chạm back-office; nhân viên quầy không được cấu hình.
func TestVaiTroMacDinhKhongVuotRanh(t *testing.T) {
	for _, code := range ManagerCodes() {
		assert.False(t, strings.HasPrefix(code, "system."), "manager không được có %q", code)
		assert.NotEqual(t, Wildcard, code)
	}
	camVoiStaff := []string{"products.update", "products.delete", "promotions.create", "pricing.update",
		"system.users.create", "backups.restore", "notifications.dispatch", "machines.delete"}
	staff := map[string]bool{}
	for _, c := range StaffCodes() {
		staff[c] = true
	}
	for _, c := range camVoiStaff {
		assert.False(t, staff[c], "nhân viên quầy không được có %q", c)
	}
}
