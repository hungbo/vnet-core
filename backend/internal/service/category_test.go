package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCategoryService_List(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewCategoryService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "categories"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "sort_order"}).
			AddRow("c1", "Food", 1).
			AddRow("c2", "Drinks", 2))

	result, err := svc.List()
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCategoryService_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewCategoryService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "categories" WHERE id = \$1 AND "categories"\."deleted_at" IS NULL ORDER BY "categories"."id" LIMIT \$2`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("c1", "Food"))

	mock.ExpectQuery(`SELECT \* FROM "categories" WHERE parent_id = \$1 AND "categories"\."deleted_at" IS NULL ORDER BY sort_order asc`).
		WithArgs("c1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

	result, err := svc.GetByID("c1")
	require.NoError(t, err)
	assert.Equal(t, "Food", result.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCategoryService_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewCategoryService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "categories" WHERE id = \$1 AND "categories"\."deleted_at" IS NULL ORDER BY "categories"."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.GetByID("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, "không tìm thấy danh mục", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCategoryService_Create(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewCategoryService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "categories"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	result, err := svc.Create(&CreateCategoryRequest{Name: "New Category"})
	require.NoError(t, err)
	assert.Equal(t, "New Category", result.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCategoryService_Update_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewCategoryService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "categories" WHERE id = \$1 AND "categories"\."deleted_at" IS NULL ORDER BY "categories"."id" LIMIT \$2`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("c1", "Old Name"))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "categories" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT \* FROM "categories" WHERE id = \$1 AND "categories"\."deleted_at" IS NULL AND "categories"\."id" = \$2 ORDER BY "categories"\."id" LIMIT \$3`).
		WithArgs("c1", "c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("c1", "New Name"))

	name := "New Name"
	result, err := svc.Update("c1", &UpdateCategoryRequest{Name: &name})
	require.NoError(t, err)
	assert.Equal(t, "New Name", result.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCategoryService_Delete_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewCategoryService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "categories" WHERE id = \$1 AND "categories"\."deleted_at" IS NULL ORDER BY "categories"."id" LIMIT \$2`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("c1", "Test"))

	// Xoá danh mục còn sản phẩm hoặc còn danh mục con sẽ để lại bản ghi mồ côi,
	// nên Delete đếm hai thứ đó trước khi động vào deleted_at.
	mock.ExpectQuery(`SELECT count\(\*\) FROM "products"`).
		WithArgs("c1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "categories"`).
		WithArgs("c1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "categories" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.Delete("c1")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Danh mục còn sản phẩm phải bị từ chối: nếu cho xoá, trang Sản phẩm tra tên
// danh mục trong danh sách đang hoạt động, không thấy, và hiện UUID thô.
func TestCategoryService_Delete_RefusesWhenProductsRemain(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewCategoryService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "categories" WHERE id = \$1`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("c1", "Nước"))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "products"`).
		WithArgs("c1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

	err := svc.Delete("c1")
	assert.EqualError(t, err, "không xoá được danh mục còn sản phẩm")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Danh mục không được là cha của chính nó, và chuỗi cha không được tạo vòng lặp.
//
// Database không có ràng buộc nào, nên hai thao tác bình thường trên giao diện
// đủ để làm hỏng: đặt A làm con của B rồi đặt B làm con của A. Khi đó không
// danh mục nào còn parent_id rỗng, hàm dựng cây không tìm ra gốc, và
// GET /api/categories trả về null — mất sạch cây danh mục kéo theo trang Sản
// phẩm và thực đơn máy trạm, mà không có lỗi nào hiện ra. Dựng lại được trên
// hệ thống thật bằng đúng hai lệnh PUT.
func TestCategoryService_Update_ChanTuLamChaCuaChinhNo(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewCategoryService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "categories" WHERE id = \$1`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("c1", "Đồ ăn nhanh"))

	chinhNo := "c1"
	_, err := svc.Update("c1", &UpdateCategoryRequest{ParentID: &chinhNo})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cha của chính nó")
}

func TestCategoryService_Update_ChanVongLapChaCon(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewCategoryService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "categories" WHERE id = \$1`).
		WithArgs("cha", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("cha", "Đồ ăn nhanh"))

	// Đi ngược từ "con" lên: cha của "con" chính là "cha" đang sửa → vòng lặp.
	mock.ExpectQuery(`SELECT "parent_id" FROM "categories" WHERE id = \$1`).
		WithArgs("con", 1).
		WillReturnRows(sqlmock.NewRows([]string{"parent_id"}).AddRow("cha"))

	con := "con"
	_, err := svc.Update("cha", &UpdateCategoryRequest{ParentID: &con})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "vòng lặp")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// parent_id rỗng phải thành NULL, không được để PostgreSQL nổ lỗi uuid thô.
func TestCategoryService_chaHopLe_RongThanhNil(t *testing.T) {
	db, _ := newMockDB(t)
	svc := NewCategoryService(db, NewAuditService(db))

	for _, v := range []string{"", "   "} {
		cha, err := svc.chaHopLe("c1", &v)
		require.NoError(t, err)
		assert.Nil(t, cha)
	}

	cha, err := svc.chaHopLe("c1", nil)
	require.NoError(t, err)
	assert.Nil(t, cha)
}
