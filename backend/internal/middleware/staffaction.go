package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vnet/core/pkg/jwt"
	"github.com/vnet/core/pkg/response"
)

// StaffAction là một thao tác thay đổi dữ liệu của nhân viên, ở mức HTTP.
type StaffAction struct {
	UserID    string
	Method    string
	Route     string // mẫu route: /api/products/:id
	Path      string // đường dẫn thật
	EntityID  string // tham số :id nếu có
	IP        string
	RequestID string
	Status    int
	Duration  time.Duration
	// Thân yêu cầu JSON, đã che mật khẩu/token, tối đa staffActionBodyMax byte.
	Body json.RawMessage
}

const staffActionBodyMax = 4 << 10

// Hội thoại chat, "đã đọc", và gửi lỗi JS: là trò chuyện hoặc tiếng ồn, không
// phải thao tác trên dữ liệu của quán — ghi chúng chỉ làm loãng nhật ký.
var staffActionSkip = map[string]bool{
	"POST /api/error-reports":            true,
	"POST /api/chat/rooms":               true,
	"POST /api/chat/messages":            true,
	"PUT /api/chat/rooms/:id/read":       true,
	"PUT /api/chat/messages/:id/deliver": true,
	"PUT /api/chat/messages/:id/read":    true,
}

// StaffActionLog gọi record cho MỌI yêu cầu thay đổi dữ liệu (POST/PUT/PATCH/
// DELETE) của một nhân viên đã đăng nhập — kể cả yêu cầu bị từ chối (403) hay
// lỗi, vì một lần thử cũng là dấu vết.
//
// Trước đây việc ghi nhật ký nằm rải trong từng service: 73 trên 127 thao tác
// ghi mà không kèm người làm, 14 thao tác không ghi gì, và không ghi đăng nhập
// hay IP. Đặt ở một chỗ thì route viết sau cũng tự có nhật ký.
//
// Phải chạy SAU AuthRequired (cần biết ai đang gọi).
func StaffActionLog(record func(StaffAction)) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		default:
			c.Next()
			return
		}
		body := docThanJSON(c)
		start := time.Now()

		c.Next()

		if GetKind(c) != jwt.KindStaff {
			return
		}
		route := c.FullPath()
		if staffActionSkip[c.Request.Method+" "+route] {
			return
		}
		record(StaffAction{
			UserID:    GetUserID(c),
			Method:    c.Request.Method,
			Route:     route,
			Path:      c.Request.URL.Path,
			EntityID:  c.Param("id"),
			IP:        c.ClientIP(),
			RequestID: c.GetString(response.RequestIDKey),
			Status:    c.Writer.Status(),
			Duration:  time.Since(start),
			Body:      body,
		})
	}
}

// docThanJSON đọc thân yêu cầu JSON rồi TRẢ LẠI nguyên vẹn cho handler, và đưa
// về bản đã che khoá nhạy cảm, cắt ngắn. Thân không phải JSON (tải tệp lên) hay
// quá lớn thì bỏ qua — handler vẫn nhận đủ.
func docThanJSON(c *gin.Context) json.RawMessage {
	if c.Request.Body == nil || !strings.Contains(c.GetHeader("Content-Type"), "application/json") ||
		c.Request.ContentLength > 1<<20 {
		return nil
	}
	raw, err := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewReader(raw))
	if err != nil || len(raw) == 0 {
		return nil
	}
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return nil
	}
	out, err := json.Marshal(cheKhoaNhayCam(v))
	if err != nil {
		return nil
	}
	if len(out) > staffActionBodyMax {
		// Cắt ngang thì không còn là JSON hợp lệ cho cột jsonb: gói thành chuỗi.
		out, _ = json.Marshal(string(out[:staffActionBodyMax]) + "…")
	}
	return out
}

var (
	// Từ đủ dài để không trùng nghĩa khác: có mặt ở đâu trong tên khoá là che.
	khoaNhayCam = regexp.MustCompile(`(?i)pass|secret|token|otp|hash|credential|cookie|signature|private|auth`)
	// Từ ngắn dễ trùng ("spin" chứa "pin"): chỉ che khi đứng thành một đoạn của
	// tên khoá — api_key, pin_code, client_cert.
	doanNhayCam = regexp.MustCompile(`(?i)(^|[_.-])(pin|key|apikey|cert)($|[_.-])`)
)

// cheKhoaNhayCam thay giá trị của mọi khoá kiểu mật khẩu/token/khoá API bằng
// "***", ở mọi độ sâu: đổi mật khẩu, tạo tài khoản, đặt lại mật khẩu hội viên
// đều gửi mật khẩu trong thân, trang Cài đặt gửi thẳng api_key/api_secret, và
// nhật ký thì ai có quyền xem cũng đọc được. Che thừa còn hơn lộ.
func cheKhoaNhayCam(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if khoaNhayCam.MatchString(k) || doanNhayCam.MatchString(k) {
				t[k] = "***"
			} else {
				t[k] = cheKhoaNhayCam(val)
			}
		}
	case []any:
		for i := range t {
			t[i] = cheKhoaNhayCam(t[i])
		}
	}
	return v
}
