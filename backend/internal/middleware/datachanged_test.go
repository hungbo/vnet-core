package middleware

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/pkg/response"
)

func TestChangeTopic(t *testing.T) {
	cases := []struct{ method, route, want string }{
		{"POST", "/api/members", "members"},
		{"PUT", "/api/members/:id", "members"},
		{"POST", "/api/members/:id/topup", "members"},
		{"DELETE", "/api/orders/batch-delete", "orders"},
		{"POST", "/api/systemManage/addUser", "systemManage"},
		{"PUT", "/api/settings/:group", "settings"},
		{"POST", "/api/sessions/me/end", "sessions"},
		{"POST", "/api/machines/:id/remote/:action", "machines"},
		{"POST", "/api/admin/notifications/:id/dispatch", "admin-notifications"},
		{"PUT", "/api/notifications/:id/read", "notifications"},
		// máy trạm báo lỗi / vi phạm: thuộc trang khác với đoạn đầu đường dẫn
		{"POST", "/api/machines/by-code/:code/errors", "error-reports"},
		{"POST", "/api/machines/by-code/:code/blocklist/violations", "website-rules"},
		// đọc, đăng nhập, chat, nhịp tim, in, tải tệp: không phát
		{"GET", "/api/members", ""},
		{"POST", "/api/auth/login", ""},
		{"PUT", "/api/auth/change-password", ""},
		{"POST", "/api/chat/messages", ""},
		{"POST", "/api/machines/by-code/:code/heartbeat", ""},
		{"POST", "/api/machines/:id/heartbeat", ""},
		{"POST", "/api/orders/:id/print", ""},
		{"POST", "/api/upload", ""},
		// route không khớp (404) có FullPath rỗng
		{"POST", "", ""},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, ChangeTopic(c.method, c.route), "%s %s", c.method, c.route)
	}
}

func TestDataChanged_PhatSauKhiGhiThanhCong(t *testing.T) {
	gin.SetMode(gin.TestMode)
	type phat struct {
		topic string
		ev    ChangeEvent
	}
	var got []phat
	r := gin.New()
	r.Use(DataChanged(func(topic string, ev ChangeEvent) { got = append(got, phat{topic, ev}) }))
	r.PUT("/api/members/:id", func(c *gin.Context) { response.Success(c, nil) })
	r.DELETE("/api/products/:id", func(c *gin.Context) { response.Forbidden(c, "không có quyền") })
	r.GET("/api/members", func(c *gin.Context) { response.Success(c, nil) })
	r.POST("/api/machines/by-code/:code/heartbeat", func(c *gin.Context) { response.Success(c, nil) })

	doJSON(r, "PUT", "/api/members/m-1", `{"full_name":"Lan"}`)
	doJSON(r, "DELETE", "/api/products/p-1", "")
	doJSON(r, "GET", "/api/members", "")
	doJSON(r, "POST", "/api/machines/by-code/MAY1/heartbeat", `{}`)
	doJSON(r, "POST", "/api/khong-co", `{}`)

	require.Len(t, got, 1, "chỉ yêu cầu ghi THÀNH CÔNG mới phát")
	assert.Equal(t, "members", got[0].topic)
	assert.Equal(t, ChangeEvent{Method: http.MethodPut, Route: "/api/members/:id", ID: "m-1"}, got[0].ev)
}
