package service

import (
	"testing"

	"bytes"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
)

func TestProductService_List_All(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewProductService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "products" WHERE "products"\."deleted_at" IS NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE "products"\."deleted_at" IS NULL ORDER BY sort_order asc, name asc LIMIT \$1`).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "category_id", "price", "current_stock"}).
			AddRow("p1", "Coke", nil, int64(10000), float64(0)).
			AddRow("p2", "Pepsi", nil, int64(10000), float64(0)))

	mock.ExpectQuery(`SELECT \* FROM "product_ingredients" WHERE product_id = \$1`).
		WithArgs("p1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	mock.ExpectQuery(`SELECT \* FROM "product_ingredients" WHERE product_id = \$1`).
		WithArgs("p2").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	result, err := svc.List(nil, "", "", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(2), result.Total)
	assert.Len(t, result.Items, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Thẻ danh mục trong thực đơn máy trạm gửi category_id lên và trước đây không ai
// đọc, nên thẻ nào cũng ra nguyên cả thực đơn. Chốt lại điều kiện WHERE.
func TestProductService_List_LocTheoDanhMuc(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewProductService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "products" WHERE category_id = \$1 AND "products"\."deleted_at" IS NULL`).
		WithArgs("c1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE category_id = \$1 AND "products"\."deleted_at" IS NULL ORDER BY sort_order asc, name asc LIMIT \$2`).
		WithArgs("c1", 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "category_id", "price", "current_stock"}).
			AddRow("p1", "Mì xào bò", "c1", int64(45000), float64(0)))

	mock.ExpectQuery(`SELECT \* FROM "product_ingredients" WHERE product_id = \$1`).
		WithArgs("p1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	result, err := svc.List(nil, "c1", "", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.Len(t, result.Items, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProductService_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewProductService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND "products"\."deleted_at" IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price"}).AddRow("p1", "Coke", int64(10000)))

	mock.ExpectQuery(`SELECT \* FROM "product_ingredients" WHERE product_id = \$1`).
		WithArgs("p1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	result, err := svc.GetByID("p1")
	require.NoError(t, err)
	assert.Equal(t, "Coke", result.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProductService_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewProductService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND "products"\."deleted_at" IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.GetByID("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, "không tìm thấy sản phẩm", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProductService_Create(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewProductService(db, NewAuditService(db))

	// Danh mục nay được kiểm tồn tại trước khi ghi (cột category_id không có
	// khoá ngoại nên mã sai vẫn lưu được).
	mock.ExpectQuery(`SELECT count\(\*\) FROM "categories" WHERE id = \$1`).
		WithArgs("cat1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "products"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT \* FROM "product_ingredients" WHERE product_id = \$1`).
		WithArgs(testUUID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	catID := "cat1"
	result, err := svc.Create(&CreateProductRequest{
		CategoryID: &catID,
		Name:       "New Product",
		Price:      50000,
	})
	require.NoError(t, err)
	assert.Equal(t, "New Product", result.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProductService_Create_WithCurrentStock(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewProductService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "categories" WHERE id = \$1`).
		WithArgs("cat1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "products"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT \* FROM "product_ingredients" WHERE product_id = \$1`).
		WithArgs(testUUID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	catID := "cat1"
	result, err := svc.Create(&CreateProductRequest{
		CategoryID:   &catID,
		Name:         "Bottled Water",
		Price:        10000,
		HasStock:     true,
		CurrentStock: 50,
	})
	require.NoError(t, err)
	assert.Equal(t, "Bottled Water", result.Name)
	assert.Equal(t, float64(50), result.CurrentStock)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProductService_GetByID_HasStock_WithCurrentStock(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewProductService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND "products"\."deleted_at" IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "has_stock", "current_stock"}).
			AddRow("p1", "Water", true, float64(50)))

	mock.ExpectQuery(`SELECT \* FROM "product_ingredients" WHERE product_id = \$1`).
		WithArgs("p1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	result, err := svc.GetByID("p1")
	require.NoError(t, err)
	assert.Equal(t, float64(50), result.CurrentStock)
	assert.Empty(t, result.Ingredients)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProductService_GetByID_HasStock_NoBOM(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewProductService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND "products"\."deleted_at" IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "has_stock"}).
			AddRow("p1", "Coffee", true))

	mock.ExpectQuery(`SELECT \* FROM "product_ingredients" WHERE product_id = \$1`).
		WithArgs("p1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	result, err := svc.GetByID("p1")
	require.NoError(t, err)
	assert.Equal(t, float64(0), result.CurrentStock)
	assert.Empty(t, result.Ingredients)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProductService_Update_CurrentStock(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewProductService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND "products"\."deleted_at" IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "has_stock"}).
			AddRow("p1", "Water", false))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "products" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND "products"\."deleted_at" IS NULL AND "products"\."id" = \$2 ORDER BY "products"\."id" LIMIT \$3`).
		WithArgs("p1", "p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "has_stock", "current_stock"}).
			AddRow("p1", "Water", true, float64(30)))

	mock.ExpectQuery(`SELECT \* FROM "product_ingredients" WHERE product_id = \$1`).
		WithArgs("p1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	newStock := 30.0
	result, err := svc.Update("p1", &UpdateProductRequest{
		HasStock:     boolPtr(true),
		CurrentStock: &newStock,
	})
	require.NoError(t, err)
	assert.Equal(t, float64(30), result.CurrentStock)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProductService_Delete_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewProductService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "products" WHERE id = \$1 AND "products"\."deleted_at" IS NULL ORDER BY "products"."id" LIMIT \$2`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("p1", "Test"))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "order_items"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "stock_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "inventory_counts"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "product_ingredients"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectBegin()
	// Công thức, tuỳ chọn và ánh xạ máy in của CHÍNH sản phẩm này là con sở hữu.
	mock.ExpectExec(`DELETE FROM "product_ingredients"`).
		WillReturnResult(sqlmock.NewResult(1, 0))
	mock.ExpectExec(`DELETE FROM "product_options"`).
		WillReturnResult(sqlmock.NewResult(1, 0))
	mock.ExpectExec(`DELETE FROM "product_printer_mappings"`).
		WillReturnResult(sqlmock.NewResult(1, 0))
	mock.ExpectExec(`UPDATE "products" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.Delete("p1")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Bỏ trống ô danh mục phải ghi NULL, không phải chuỗi rỗng.
//
// Ô chọn trên trang quản trị gửi "" khi người dùng không chọn hoặc xoá lựa
// chọn. category_id là cột uuid cho phép rỗng và PostgreSQL từ chối "" — lỗi
// thô `invalid input syntax for type uuid: ""` từng lọt thẳng ra người dùng ở
// cả đường tạo lẫn đường sửa sản phẩm. Đây là lần thứ ba gặp cùng bẫy này
// (nhóm của máy, danh mục cha, rồi danh mục của sản phẩm) nên dùng chung
// uuidRongThanhNil.
func TestUuidRongThanhNil(t *testing.T) {
	rong := ""
	trang := "   "
	that := "550e8400-e29b-41d4-a716-446655440000"

	assert.Nil(t, uuidRongThanhNil(nil))
	assert.Nil(t, uuidRongThanhNil(&rong), "chuỗi rỗng phải thành NULL")
	assert.Nil(t, uuidRongThanhNil(&trang), "chuỗi toàn khoảng trắng phải thành NULL")
	assert.Equal(t, &that, uuidRongThanhNil(&that), "uuid thật phải giữ nguyên")
}

// Cập nhật MỘT phần sản phẩm không được đòi phải gửi kèm giá.
//
// Price là *int64 với binding "min=0" nhưng thiếu omitempty: validator coi con
// trỏ nil là vi phạm, nên gọi PUT chỉ để đổi nhà cung cấp cũng bị chặn bằng câu
// "Giá tối thiểu là 0" — thông báo nói về một trường mà người gọi không hề đụng
// tới. Cả repo dùng khuôn `omitempty,min=...`; đây từng là chỗ duy nhất lệch.
func TestUpdateProductRequest_ChoPhepCapNhatMotPhan(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for ten, body := range map[string]string{
		"chỉ đổi nhà cung cấp": `{"supplier_id":"550e8400-e29b-41d4-a716-446655440000"}`,
		"chỉ đổi tên":          `{"name":"Tên mới"}`,
		"có kèm giá hợp lệ":    `{"name":"Tên mới","price":15000}`,
		"giá 0":                `{"price":0}`,
	} {
		t.Run(ten, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPut, "/products/x", bytes.NewBufferString(body))
			c.Request.Header.Set("Content-Type", "application/json")

			var req UpdateProductRequest
			assert.NoError(t, c.ShouldBindJSON(&req))
		})
	}
}

// Giá âm vẫn phải bị chặn.
func TestUpdateProductRequest_ChanGiaAm(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPut, "/products/x", bytes.NewBufferString(`{"price":-1}`))
	c.Request.Header.Set("Content-Type", "application/json")

	var req UpdateProductRequest
	assert.Error(t, c.ShouldBindJSON(&req))
}

// Mã danh mục không có thật vẫn tạo được sản phẩm vì cột category_id không có
// khoá ngoại; sản phẩm sau đó hiện "Đã xoá" ở cột danh mục mà không ai biết gõ
// sai ở đâu.
func TestProductService_Create_ChanDanhMucKhongTonTai(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewProductService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "categories" WHERE id = \$1`).
		WithArgs("khong-co-that").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	catID := "khong-co-that"
	_, err := svc.Create(&CreateProductRequest{CategoryID: &catID, Name: "Mì tôm", Price: 15000})

	require.Error(t, err)
	assert.Equal(t, "không tìm thấy danh mục", err.Error())
	// Không có lệnh INSERT nào được gửi đi.
	assert.NoError(t, mock.ExpectationsWereMet())
}
