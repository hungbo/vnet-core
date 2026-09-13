package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

// Tên khoá trong constraintMessages phải trùng tên index PostgreSQL sinh ra.
//
// GORM đặt tên index của thẻ `uniqueIndex` là idx_<bảng>_<cột>, còn tiền tố
// "uni_" là của thẻ `unique`. Ba mục từng viết nhầm sang "uni_" nên không khớp
// gì cả, và nhân viên quầy nhận đúng câu "Dữ liệu 'idx_members_username' đã tồn
// tại" — tên index lọt thẳng ra giao diện thay vì lời giải thích.
func TestHandleCreateError_DichTenIndexTrung(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := map[string]string{
		"idx_members_username":  "Tên đăng nhập hội viên đã tồn tại",
		"idx_users_username":    "Tên đăng nhập đã tồn tại",
		"idx_orders_order_code": "Mã đơn hàng đã tồn tại",
	}

	for constraint, want := range cases {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

		handleCreateError(c, &pgconn.PgError{Code: "23505", ConstraintName: constraint})

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), want, "constraint %s", constraint)
		assert.NotContains(t, w.Body.String(), constraint, "tên index không được lọt ra giao diện")
	}
}

func TestHandleCreateError_LoiKhac(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	handleCreateError(c, errors.New("boom"))

	assert.Contains(t, w.Body.String(), "boom")
}
