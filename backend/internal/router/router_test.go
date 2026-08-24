package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/pkg/jwt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// newTestRouter builds the real route table over a mock database. Authorization
// runs before any handler touches the database, so the middleware decision is
// observable without a live PostgreSQL.
func newTestRouter(t *testing.T) (*gin.Engine, *jwt.Manager) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mockDB, _, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { mockDB.Close() })

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: mockDB}), &gorm.Config{})
	require.NoError(t, err)

	jwtManager := jwt.New("test-secret", time.Hour, 7*24*time.Hour, "test")

	cfg := &config.Config{}
	cfg.Server.Mode = "test"
	cfg.Server.UploadDir = t.TempDir()

	r := gin.New()
	Register(r, db, jwtManager, hub.New(nil), cfg)

	return r, jwtManager
}

func memberToken(t *testing.T, m *jwt.Manager, memberID string) string {
	t.Helper()
	token, err := m.GenerateAccessToken(memberID, "player", "member", "", jwt.KindMember, []string{"member.access"})
	require.NoError(t, err)
	return token
}

func do(r *gin.Engine, method, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// The headline regression: a member account used to reach every admin endpoint.
func TestMemberTokenIsRejectedByAdminRoutes(t *testing.T) {
	r, jwtManager := newTestRouter(t)
	token := memberToken(t, jwtManager, "member-1")

	adminRoutes := []struct{ method, path string }{
		{"GET", "/api/systemManage/getUserList"},
		{"POST", "/api/systemManage/addUser"},
		{"DELETE", "/api/systemManage/deleteUser"},
		{"GET", "/api/members"},
		{"POST", "/api/members"},
		{"DELETE", "/api/members/member-2"},
		{"POST", "/api/members/member-1/topup"},
		{"GET", "/api/backups"},
		{"POST", "/api/backups"},
		{"GET", "/api/audit-logs"},
		{"GET", "/api/reports/daily-revenue"},
		{"GET", "/api/reports/by-member"},
		{"GET", "/api/orders"},
		{"DELETE", "/api/orders/order-1"},
		{"GET", "/api/machines"},
		{"POST", "/api/machines/machine-1/remote/shutdown"},
		{"GET", "/api/sessions/active"},
		{"POST", "/api/sessions/start"},
		{"GET", "/api/shifts"},
		{"GET", "/api/admin/notifications"},
		{"GET", "/api/transactions"},
		{"POST", "/api/products"},
		{"DELETE", "/api/categories/cat-1"},
		{"PUT", "/api/settings/general"},
		{"GET", "/api/settings"},
		{"POST", "/api/upload"},
		{"GET", "/api/route/getUserRoutes"},
		{"GET", "/api/auth/permissions"},
		{"DELETE", "/api/chat/rooms"},
		{"GET", "/api/suppliers"},
		{"GET", "/api/promotions"},
		{"GET", "/api/bookings"},
		{"GET", "/api/combos"},
		{"GET", "/api/curfew"},
		{"GET", "/api/printers"},
		{"GET", "/api/units"},
		{"GET", "/api/stock-transactions"},
		{"GET", "/api/member-groups"},
		{"GET", "/api/machine-groups"},
		{"GET", "/api/machine-assets"},
		{"GET", "/api/lucky-spin/rewards"},
	}

	for _, tc := range adminRoutes {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			w := do(r, tc.method, tc.path, token)
			assert.Equal(t, http.StatusForbidden, w.Code)
		})
	}
}

// The desktop client authenticates with a member token, so these must stay
// reachable. They fail later on the mock database; what matters is that
// authorization does not reject them.
func TestMemberTokenReachesClientRoutes(t *testing.T) {
	r, jwtManager := newTestRouter(t)
	token := memberToken(t, jwtManager, "member-1")

	clientRoutes := []struct{ method, path string }{
		{"GET", "/api/auth/me"},
		{"GET", "/api/categories"},
		{"GET", "/api/products"},
		{"GET", "/api/notifications"},
		{"GET", "/api/notifications/unread-count"},
		{"PUT", "/api/notifications/read-all"},
		{"GET", "/api/sessions/me"},
		{"GET", "/api/settings/general"},
		{"GET", "/api/machines/by-code/PC-01"},
		{"GET", "/api/chat/rooms"},
		{"GET", "/api/members/member-1"},
		{"GET", "/api/members/member-1/transactions"},
		{"GET", "/api/members/member-1/sessions"},
		{"GET", "/api/members/member-1/combos"},
	}

	for _, tc := range clientRoutes {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			require.True(t, routeExists(r, tc.method, tc.path), "route is not registered")

			w := do(r, tc.method, tc.path, token)
			// The handler runs and then fails on the mock database; the only
			// thing asserted here is that authorization let it through.
			assert.NotEqual(t, http.StatusForbidden, w.Code)

			if tc.path == "/api/auth/me" {
				// Ngoại lệ: /auth/me tra hồ sơ trong database, mà database ở đây
				// là giả nên tra hỏng và handler trả 401 mã 8888 (buộc đăng nhập
				// lại). Đó là 401 của HANDLER, không phải của middleware — phân
				// biệt bằng mã 8888 để phép kiểm vẫn bắt được lỗi phân quyền.
				if w.Code == http.StatusUnauthorized {
					assert.Contains(t, w.Body.String(), "8888",
						"401 ở đây phải là 8888 từ handler, không phải middleware chặn")
				}
				return
			}
			assert.NotEqual(t, http.StatusUnauthorized, w.Code)
		})
	}
}

// routeExists reports whether the engine has a handler registered for the given
// method and concrete path, so a handler-produced 404 is never mistaken for a
// missing route.
func routeExists(r *gin.Engine, method, path string) bool {
	for _, ri := range r.Routes() {
		if ri.Method != method {
			continue
		}
		if matchRoutePattern(ri.Path, path) {
			return true
		}
	}
	return false
}

func matchRoutePattern(pattern, path string) bool {
	pp := strings.Split(strings.Trim(pattern, "/"), "/")
	cp := strings.Split(strings.Trim(path, "/"), "/")
	if len(pp) != len(cp) {
		return false
	}
	for i := range pp {
		if strings.HasPrefix(pp[i], ":") || strings.HasPrefix(pp[i], "*") {
			continue
		}
		if pp[i] != cp[i] {
			return false
		}
	}
	return true
}

// A member may read its own record but not another member's.
func TestMemberCannotReadAnotherMemberRecord(t *testing.T) {
	r, jwtManager := newTestRouter(t)
	token := memberToken(t, jwtManager, "member-1")

	for _, path := range []string{
		"/api/members/member-2",
		"/api/members/member-2/transactions",
		"/api/members/member-2/sessions",
		"/api/members/member-2/combos",
	} {
		t.Run(path, func(t *testing.T) {
			assert.Equal(t, http.StatusForbidden, do(r, "GET", path, token).Code)
		})
	}
}

// A refresh token is signed with the same key as an access token.
func TestRefreshTokenIsNotAccepted(t *testing.T) {
	r, jwtManager := newTestRouter(t)

	refresh, err := jwtManager.GenerateRefreshToken("u1", jwt.KindStaff)
	require.NoError(t, err)

	w := do(r, "GET", "/api/members", refresh)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestStaffWithoutClientAdminIsRejectedByAdminOnlyRoutes(t *testing.T) {
	r, jwtManager := newTestRouter(t)

	// A staff token that carries no client.admin permission.
	token, err := jwtManager.GenerateAccessToken("u1", "cashier", "staff", "r1", jwt.KindStaff,
		[]string{"members.view", "orders.view"})
	require.NoError(t, err)

	for _, path := range []string{"/api/backups", "/api/audit-logs", "/api/admin/notifications", "/api/systemManage/getUserList"} {
		t.Run(path, func(t *testing.T) {
			assert.Equal(t, http.StatusForbidden, do(r, "GET", path, token).Code)
		})
	}
}

func TestUnauthenticatedRequestsAreRejected(t *testing.T) {
	r, _ := newTestRouter(t)

	w := do(r, "GET", "/api/members", "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// Swagger documents every endpoint and stays off in production.
func TestSwaggerNotRegisteredInReleaseMode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockDB, _, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { mockDB.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: mockDB}), &gorm.Config{})
	require.NoError(t, err)

	cfg := &config.Config{}
	cfg.Server.Mode = config.ModeRelease
	cfg.Server.UploadDir = t.TempDir()

	r := gin.New()
	Register(r, db, jwt.New("s", time.Hour, time.Hour, "t"), hub.New(nil), cfg)

	assert.Equal(t, http.StatusNotFound, do(r, "GET", "/swagger/index.html", "").Code)
}

// A member who learns another room's id must not be able to read it. Chat rooms
// are reachable by members by design, so the guard lives inside the handler.
func TestMemberCannotReachAnotherChatRoom(t *testing.T) {
	r, jwtManager := newTestRouter(t)
	token := memberToken(t, jwtManager, "member-1")

	// The mock database answers no rows, so the participant check fails closed
	// and the request is refused before any message is read.
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/chat/rooms/someone-elses-room/messages"},
		{"PUT", "/api/chat/rooms/someone-elses-room/read"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			require.True(t, routeExists(r, tc.method, tc.path))
			w := do(r, tc.method, tc.path, token)
			assert.NotEqual(t, http.StatusOK, w.Code, "must not succeed for a non-participant")
		})
	}
}
