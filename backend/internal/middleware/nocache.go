package middleware

import "github.com/gin-gonic/gin"

// NoCache đánh dấu mọi phản hồi API là không được dùng lại.
//
// Trước đây API không gửi bất kỳ thông tin hạn dùng nào — không Cache-Control,
// không ETag, không Last-Modified — nên trình duyệt hoặc proxy đứng giữa được
// phép tự phục vụ lại bản cũ. Triệu chứng quan sát được trên trình duyệt: tạo
// hội viên xong, trang tự gọi lại GET /api/members nhưng nhận đúng danh sách
// trước khi tạo, nên dòng mới không hiện cho tới khi bấm "Làm mới". Dữ liệu
// danh sách thay đổi theo từng thao tác, không bao giờ được phép dùng lại.
func NoCache() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}
