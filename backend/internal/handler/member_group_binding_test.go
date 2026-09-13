package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/vnet/core/internal/service"
)

// Mức giảm giá và mức chi tiêu tối thiểu của nhóm hội viên phải bị chặn ngay ở
// cửa vào, không chỉ ở giao diện.
//
// Ô nhập trên trang quản trị có kẹp giá trị (gõ 150 thành 100, gõ -1000 thành
// 0), nhưng gọi thẳng endpoint thì tạo được nhóm "giảm 150%" hoặc "giảm -20%".
// Tiền phiên chơi có tự kẹp về 100 nên không âm, nhưng bảng nhóm hội viên vẫn
// hiện "150%" cho người vận hành đọc và tin, và mọi chỗ dùng con số này về sau
// đều phải tự nhớ kẹp lại.
func TestCreateGroup_ChanGiamGiaNgoaiKhoang(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := map[string]string{
		"giảm giá trên 100":     `{"name":"Thử","discount_percent":150}`,
		"giảm giá âm":           `{"name":"Thử","discount_percent":-20}`,
		"chi tiêu tối thiểu âm": `{"name":"Thử","min_spent":-500000}`,
	}

	for ten, body := range cases {
		t.Run(ten, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/member-groups", bytes.NewBufferString(body))
			c.Request.Header.Set("Content-Type", "application/json")

			// Chỉ kiểm lớp binding: nếu nó chặn thì thân handler không bao giờ chạy,
			// nên service nil ở đây là đủ và cũng chính là bằng chứng đã chặn.
			var req service.CreateGroupRequest
			err := c.ShouldBindJSON(&req)

			assert.Error(t, err, "giá trị ngoài khoảng phải bị từ chối")
		})
	}
}

func TestCreateGroup_ChapNhanGiaTriHopLe(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for ten, body := range map[string]string{
		"giảm 0%":             `{"name":"Đồng","discount_percent":0,"min_spent":0}`,
		"giảm 100%":           `{"name":"Kim Cương","discount_percent":100}`,
		"giảm 10% có mức chi": `{"name":"Vàng","discount_percent":10,"min_spent":2000000}`,
		"không khai giảm giá": `{"name":"Bạc"}`,
	} {
		t.Run(ten, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/member-groups", bytes.NewBufferString(body))
			c.Request.Header.Set("Content-Type", "application/json")

			var req service.CreateGroupRequest
			assert.NoError(t, c.ShouldBindJSON(&req))
		})
	}
}
