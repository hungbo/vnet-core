package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ChangeEvent đi kèm sự kiện "<chủ đề>:changed" gửi tới trang quản trị: đủ để
// trang biết dữ liệu của nó vừa đổi, và nếu cần thì bản ghi nào.
type ChangeEvent struct {
	Method string `json:"method"`
	Route  string `json:"route"` // mẫu route: /api/members/:id/topup
	ID     string `json:"id,omitempty"`
}

// Route ghi mà trang quản trị không cần nghe: đăng nhập, tải tệp, chat (đã có sự
// kiện riêng), in, và việc máy trạm báo cáo đều đặn (nhịp tim 15 giây mỗi máy —
// phát theo nó là cả phòng máy làm trang Máy trạm tải lại liên tục; chuyển trạng
// thái thật thì service tự phát machine:status).
var changeSkip = map[string]bool{
	"POST /api/upload":                            true,
	"POST /api/machines/by-code/:code/heartbeat":  true,
	"POST /api/machines/:id/heartbeat":            true,
	"POST /api/machines/by-code/:code/screenshot": true,
	"POST /api/machines/by-code/:code/processes":  true,
	"POST /api/lucky-spin/simulate":               true,
	// Lệnh điều khiển từ xa và đánh thức máy chỉ ghi nhật ký; tắt / khởi động lại
	// máy thì đóng phiên qua SessionService, nơi tự phát "sessions:changed".
	"POST /api/machines/:id/remote/:action": true,
	"POST /api/machines/:id/wake":           true,
	"POST /api/orders/:id/print":            true,
	"POST /api/orders/:id/print-stations":   true,
	"POST /api/printers/:id/test":           true,
	"POST /api/combos/:id/print":            true,
}

var changeSkipPrefix = []string{"/api/auth/", "/api/chat/"}

// Route mà đoạn đầu đường dẫn không phải chủ đề của dữ liệu nó đổi.
var changeTopicOverride = map[string]string{
	"POST /api/machines/by-code/:code/errors":               "error-reports",
	"POST /api/machines/by-code/:code/blocklist/violations": "website-rules",
}

// changeSkipKey: handler đặt cờ này khi yêu cầu ghi lần này không đổi gì (xem SkipChange).
const changeSkipKey = "vnet.change.skip"

// SkipChange báo DataChanged đừng phát cho yêu cầu này: route ghi nhưng lần này
// không ghi gì — vd. kiểm thử (dry_run) tạo máy hàng loạt, giao diện gọi mỗi lần
// sửa ô nhập, phát theo nó là mọi trang Máy trạm đang mở tải lại liên tục.
func SkipChange(c *gin.Context) {
	c.Set(changeSkipKey, true)
}

// ChangeTopic trả về chủ đề của một yêu cầu ghi: đoạn đầu của đường dẫn sau
// /api/ ("members", "orders", "systemManage"...), hoặc "" nếu không phát.
func ChangeTopic(method, route string) string {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return ""
	}
	key := method + " " + route
	if t, ok := changeTopicOverride[key]; ok {
		return t
	}
	if changeSkip[key] {
		return ""
	}
	for _, p := range changeSkipPrefix {
		if strings.HasPrefix(route, p) {
			return ""
		}
	}
	rest, ok := strings.CutPrefix(route, "/api/")
	if !ok || rest == "" {
		return ""
	}
	topic, rest, _ := strings.Cut(rest, "/")
	// /api/admin/notifications: phần quản trị của một tài nguyên hội viên cũng có.
	if topic == "admin" {
		sub, _, _ := strings.Cut(rest, "/")
		if sub == "" || strings.HasPrefix(sub, ":") {
			return ""
		}
		topic = "admin-" + sub
	}
	if topic == "" || strings.HasPrefix(topic, ":") {
		return ""
	}
	return topic
}

// DataChanged gọi notify sau MỌI yêu cầu ghi thành công (POST/PUT/PATCH/DELETE,
// mã HTTP dưới 400), với chủ đề lấy từ mẫu route.
//
// Trang quản trị trước đây chỉ tự cập nhật ở 7/31 trang, mỗi trang trông vào
// một sự kiện riêng mà service phải nhớ phát; thao tác ở trang khác, ở tab khác
// hay từ máy trạm thì trang đang mở không biết gì cho tới khi nhân viên bấm F5.
// Phát ở MỘT chỗ — như StaffActionLog — thì route viết sau cũng tự có sự kiện.
// Thay đổi không đi qua yêu cầu HTTP (tác vụ hẹn giờ, chuyển trạng thái theo
// nhịp tim) vẫn phải tự phát: xem hub.Changed.
//
// Chạy sau handler, tức sau khi service đã commit: trang nhận sự kiện rồi tải
// lại là thấy dữ liệu mới.
func DataChanged(notify func(topic string, ev ChangeEvent)) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		route := c.FullPath()
		topic := ChangeTopic(c.Request.Method, route)
		if topic == "" || c.Writer.Status() >= http.StatusBadRequest || c.GetBool(changeSkipKey) {
			return
		}
		notify(topic, ChangeEvent{Method: c.Request.Method, Route: route, ID: c.Param("id")})
	}
}
