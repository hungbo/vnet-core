package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/pkg/jwt"
	"github.com/vnet/core/pkg/response"
)

// newStaffActionRouter giả AuthRequired bằng cách đặt sẵn người gọi vào context.
func newStaffActionRouter(kind string) (*gin.Engine, *[]StaffAction) {
	gin.SetMode(gin.TestMode)
	var got []StaffAction
	r := gin.New()
	r.Use(RequestID(), func(c *gin.Context) {
		c.Set(ContextKeyUserID, "u-1")
		c.Set(ContextKeyKind, kind)
		c.Next()
	}, StaffActionLog(func(a StaffAction) { got = append(got, a) }))
	return r, &got
}

func doJSON(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestStaffActionLog_RecordsStaffWriteWithMaskedBody(t *testing.T) {
	r, got := newStaffActionRouter(jwt.KindStaff)
	var handlerSaw string
	r.PUT("/api/systemManage/users/:id", func(c *gin.Context) {
		raw, _ := io.ReadAll(c.Request.Body)
		handlerSaw = string(raw)
		c.Status(http.StatusOK)
	})

	w := doJSON(r, "PUT", "/api/systemManage/users/3f9a1c2e-0000-4000-8000-000000000001",
		`{"full_name":"Lan","password":"bi-mat-123","profile":{"pin_code":"1234","note":"ok"}}`)

	assert.Contains(t, handlerSaw, "bi-mat-123", "handler phải nhận nguyên thân yêu cầu")
	require.Len(t, *got, 1)
	a := (*got)[0]
	assert.Equal(t, "u-1", a.UserID)
	assert.Equal(t, "PUT", a.Method)
	assert.Equal(t, "/api/systemManage/users/:id", a.Route)
	assert.Equal(t, "3f9a1c2e-0000-4000-8000-000000000001", a.EntityID)
	assert.Equal(t, http.StatusOK, a.Status)
	assert.Equal(t, w.Header().Get("X-Request-ID"), a.RequestID)

	var body map[string]any
	require.NoError(t, json.Unmarshal(a.Body, &body))
	assert.Equal(t, "Lan", body["full_name"])
	assert.Equal(t, "***", body["password"])
	assert.Equal(t, "***", body["profile"].(map[string]any)["pin_code"])
	assert.NotContains(t, string(a.Body), "bi-mat-123")
	assert.NotContains(t, string(a.Body), "1234")
}

// Lần thử bị từ chối cũng là dấu vết: ai đó bấm vào việc không được phép.
func TestStaffActionLog_RecordsRejectedAttempts(t *testing.T) {
	r, got := newStaffActionRouter(jwt.KindStaff)
	r.DELETE("/api/members/:id", func(c *gin.Context) { response.Forbidden(c, "không có quyền") })

	doJSON(r, "DELETE", "/api/members/abc", "")

	require.Len(t, *got, 1)
	assert.Equal(t, http.StatusForbidden, (*got)[0].Status)
	assert.Nil(t, (*got)[0].Body)
}

func TestStaffActionLog_SkipsReadsMembersAndChatter(t *testing.T) {
	r, got := newStaffActionRouter(jwt.KindStaff)
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	r.GET("/api/products", ok)
	r.POST("/api/chat/messages", ok)
	r.POST("/api/error-reports", ok)
	doJSON(r, "GET", "/api/products", "")
	doJSON(r, "POST", "/api/chat/messages", `{"content":"chào"}`)
	doJSON(r, "POST", "/api/error-reports", `{"message":"x"}`)
	assert.Empty(t, *got)

	rm, gotMember := newStaffActionRouter(jwt.KindMember)
	rm.POST("/api/orders", ok)
	doJSON(rm, "POST", "/api/orders", `{"items":[]}`)
	assert.Empty(t, *gotMember, "thao tác của hội viên không phải nhật ký nhân viên")
}

// Thân quá dài vẫn phải là JSON hợp lệ cho cột jsonb.
func TestStaffActionLog_TruncatesLargeBodyAsValidJSON(t *testing.T) {
	r, got := newStaffActionRouter(jwt.KindStaff)
	r.POST("/api/machines/batch", func(c *gin.Context) { c.Status(http.StatusOK) })

	doJSON(r, "POST", "/api/machines/batch", `{"note":"`+strings.Repeat("a", 10000)+`"}`)

	require.Len(t, *got, 1)
	assert.True(t, json.Valid((*got)[0].Body))
	assert.LessOrEqual(t, len((*got)[0].Body), staffActionBodyMax+16)
}

// Trang Cài đặt gửi thẳng {khoá: giá trị}: khoá API phải bị che, còn khoá chỉ
// tình cờ chứa chữ ngắn ("spin" chứa "pin") thì giữ để nhật ký còn đọc được.
func TestCheKhoaNhayCam_SettingsLikeKeys(t *testing.T) {
	got := cheKhoaNhayCam(map[string]any{
		"api_key": "k1", "api_secret": "s1", "access_token": "t1", "bot_token": "t2",
		"smtp_password": "p1", "pin": "0000", "client_cert": "c1", "authorization": "Bearer x",
		"daily_spins": float64(3), "shop_name": "Mia Gaming",
	}).(map[string]any)

	for _, k := range []string{"api_key", "api_secret", "access_token", "bot_token", "smtp_password", "pin", "client_cert", "authorization"} {
		assert.Equal(t, "***", got[k], k)
	}
	assert.Equal(t, float64(3), got["daily_spins"])
	assert.Equal(t, "Mia Gaming", got["shop_name"])
}
