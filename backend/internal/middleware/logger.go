package middleware

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vnet/core/pkg/response"
)

// tokenTrongQuery: WebSocket xác thực bằng ?token=<JWT> (auth.go), nên in nguyên
// query là ghi token đăng nhập vào server.log.
var tokenTrongQuery = regexp.MustCompile(`(^|&)token=[^&]*`)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := tokenTrongQuery.ReplaceAllString(c.Request.URL.RawQuery, "${1}token=***")

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		clientIP := c.ClientIP()

		line := fmt.Sprintf("[%s] %s %3d %13v | %s %s",
			time.Now().Format("2006-01-02 15:04:05"),
			method, status, latency, path, clientIP,
		)
		if query != "" {
			line += fmt.Sprintf(" ?%s", query)
		}
		if id := c.GetString(response.RequestIDKey); id != "" {
			line += " id=" + id
		}
		// Lỗi gốc do handler gắn qua c.Error (response.InternalError) — thứ duy
		// nhất cho biết VÌ SAO yêu cầu này hỏng. Giữ trên một dòng để grep được.
		if len(c.Errors) > 0 {
			line += " | err: " + strings.ReplaceAll(strings.Join(c.Errors.Errors(), "; "), "\n", " ")
		}

		if status >= 500 {
			fmt.Fprintln(gin.DefaultWriter, "ERROR "+line)
		} else if status >= 400 {
			fmt.Fprintln(gin.DefaultWriter, "WARN "+line)
		} else {
			fmt.Fprintln(gin.DefaultWriter, line)
		}
	}
}
