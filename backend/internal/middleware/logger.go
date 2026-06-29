package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

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

		if status >= 500 {
			fmt.Fprintln(gin.DefaultWriter, "ERROR "+line)
		} else if status >= 400 {
			fmt.Fprintln(gin.DefaultWriter, "WARN "+line)
		} else {
			fmt.Fprintln(gin.DefaultWriter, line)
		}
	}
}
