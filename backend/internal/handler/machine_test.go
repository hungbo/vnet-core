package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/vnet/core/internal/service"
)

// GET /machines/by-code/:code mở cho mọi tài khoản đã đăng nhập — máy trạm của
// khách gọi nó để đọc cấu hình máy. Ghi chú là của nhân viên, không được lọt ra.
func TestMachineHandler_GetByCode_KhongTraGhiChu(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, mock := newTestDB(t)
	h := NewMachineHandler(service.NewMachineService(db, nil, service.NewAuditService(db)))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE machine_code = \$1`).
		WithArgs("M-001", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code", "note"}).
			AddRow("m1", "M-001", "hỏng phím, chờ linh kiện"))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/machines/by-code/M-001", nil)
	c.Params = gin.Params{{Key: "code", Value: "M-001"}}

	h.GetByCode(c)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"machine_code":"M-001"`)
	assert.NotContains(t, w.Body.String(), "linh kiện")
	assert.NoError(t, mock.ExpectationsWereMet())
}
