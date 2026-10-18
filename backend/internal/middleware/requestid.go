package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vnet/core/pkg/response"
)

// RequestID gắn cho mỗi yêu cầu một mã ngắn: trả trong header X-Request-ID, in
// vào dòng log truy cập, và nối vào câu báo lỗi 500 (response.InternalError).
// Người dùng đọc lại mã đó là dev tìm ra đúng dòng log của lần bấm bị lỗi.
//
// Mã do server sinh, không nhận từ header của client: giá trị client gửi sẽ đi
// thẳng vào nhật ký mà không ai kiểm.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		id := hex.EncodeToString(b)
		c.Set(response.RequestIDKey, id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

// Recovery thay gin.Recovery.
//
// gin.Recovery trả 500 với thân rỗng (giao diện chỉ hiện "Request failed with
// status code 500") và in cả dòng request kèm query vào log — token ?token=
// của WebSocket lọt theo. Bản này in mã yêu cầu + đường dẫn + stack, rồi trả
// câu báo lỗi JSON có mã đó.
//
// onError (nil = chỉ ghi log) nhận mọi lỗi 500 để lưu vào nhật ký lỗi: panic
// kèm stack, và lỗi gốc handler gắn qua response.InternalError. 4xx là lỗi của
// người dùng, không phải việc của dev, nên không gửi.
func Recovery(onError func(c *gin.Context, kind, message, detail string)) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				stack := debug.Stack()
				_, _ = fmt.Fprintf(gin.DefaultErrorWriter, "[%s] PANIC id=%s %s %s: %v\n%s\n",
					time.Now().Format("2006-01-02 15:04:05"),
					c.GetString(response.RequestIDKey), c.Request.Method, c.Request.URL.Path,
					rec, stack)
				response.InternalError(c, "Lỗi hệ thống", fmt.Errorf("panic: %v", rec))
				c.Abort()
				if onError != nil {
					onError(c, "panic", fmt.Sprint(rec), string(stack))
				}
			}
		}()
		c.Next()
		if onError != nil && c.Writer.Status() >= 500 && len(c.Errors) > 0 {
			onError(c, "error", strings.Join(c.Errors.Errors(), "; "), "")
		}
	}
}
