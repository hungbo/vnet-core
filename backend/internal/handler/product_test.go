package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/jwt"
)

// Thực đơn máy trạm gọi GET /products bằng token hội viên và không gửi
// is_active: món ngưng bán phải tự rơi khỏi danh sách, kể cả khi hội viên cố
// gửi is_active=false.
func TestProductHandler_List_HoiVienChiThayMonDangBan(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, mock := newTestDB(t)
	h := NewProductHandler(service.NewProductService(db, service.NewAuditService(db)))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "products" WHERE is_active = \$1 AND "products"\."deleted_at" IS NULL`).
		WithArgs(true).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT \* FROM "products" WHERE is_active = \$1`).
		WithArgs(true, 500).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/products?page_size=500&is_active=false", nil)
	c.Set(middleware.ContextKeyKind, jwt.KindMember)

	h.List(c)

	assert.Equal(t, 200, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Nhân viên không gửi is_active thì thấy cả món ngưng bán, để bật lại được.
func TestProductHandler_List_NhanVienThayTatCa(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, mock := newTestDB(t)
	h := NewProductHandler(service.NewProductService(db, service.NewAuditService(db)))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "products" WHERE "products"\."deleted_at" IS NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT \* FROM "products" WHERE "products"\."deleted_at" IS NULL`).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/products", nil)
	c.Set(middleware.ContextKeyKind, jwt.KindStaff)

	h.List(c)

	assert.Equal(t, 200, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}
