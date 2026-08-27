package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/model"
)

// Lỗi phải khớp errors.Is(ErrRangBuoc) để handler trả 409, NHƯNG Error() phải
// là câu tiếng Việt sạch — nó hiện thẳng lên toast của trang quản trị.
func TestKiemTraPhuThuoc_LoiKhopSentinelNhungKhongCoTienTo(t *testing.T) {
	db, mock := newMockDB(t)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "machine_sessions" WHERE machine_id = \$1`).
		WithArgs("m1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(142))

	err := kiemTraPhuThuoc(db, "m1", []phuThuoc{
		{Bang: &model.MachineSession{}, Cot: "machine_id", Nhan: "phiên chơi"},
	}, "hãy tắt hoạt động máy thay vì xoá")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRangBuoc), "handler dựa vào errors.Is để trả 409")
	assert.Equal(t, "không xoá được: còn 142 phiên chơi — hãy tắt hoạt động máy thay vì xoá", err.Error())
	assert.False(t, strings.HasPrefix(err.Error(), "ràng buộc dữ liệu"),
		"tiền tố của sentinel không được lọt vào câu hiện cho người dùng")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Count hỏng phải trả lỗi, không được coi như "không có bản ghi nào".
//
// Bỏ qua lỗi của Count nghĩa là database sập thì n = 0 và bản ghi bị xoá — đúng
// cái mà hàm này sinh ra để chặn. MemberService.DeleteGroup từng mắc lỗi đó.
func TestKiemTraPhuThuoc_LoiCuaCountKhongBiNuot(t *testing.T) {
	db, mock := newMockDB(t)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "machine_sessions"`).
		WillReturnError(errors.New("connection refused"))

	err := kiemTraPhuThuoc(db, "m1", []phuThuoc{
		{Bang: &model.MachineSession{}, Cot: "machine_id", Nhan: "phiên chơi"},
	}, "gợi ý")

	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrRangBuoc), "lỗi hạ tầng không phải lỗi ràng buộc, không được trả 409")
	assert.Contains(t, err.Error(), "connection refused")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Dừng ở phụ thuộc ĐẦU TIÊN còn dữ liệu: các câu Count sau không được chạy.
func TestKiemTraPhuThuoc_DungOPhuThuocDauTien(t *testing.T) {
	db, mock := newMockDB(t)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "machine_sessions"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	// Không khai ExpectQuery cho "orders": nếu hàm chạy tiếp, sqlmock sẽ báo lỗi.

	err := kiemTraPhuThuoc(db, "m1", []phuThuoc{
		{Bang: &model.MachineSession{}, Cot: "machine_id", Nhan: "phiên chơi"},
		{Bang: &model.Order{}, Cot: "machine_id", Nhan: "đơn hàng"},
	}, "gợi ý")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "phiên chơi")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Không còn phụ thuộc nào thì cho qua.
func TestKiemTraPhuThuoc_KhongConPhuThuocThiChoQua(t *testing.T) {
	db, mock := newMockDB(t)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "machine_sessions"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "orders"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	err := kiemTraPhuThuoc(db, "m1", []phuThuoc{
		{Bang: &model.MachineSession{}, Cot: "machine_id", Nhan: "phiên chơi"},
		{Bang: &model.Order{}, Cot: "machine_id", Nhan: "đơn hàng"},
	}, "gợi ý")

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// chanVi dùng chung sentinel với kiemTraPhuThuoc để handler chỉ có một đường xử lý.
func TestChanVi_DungChungSentinel(t *testing.T) {
	err := chanVi("đơn đã có %d phiếu thanh toán", 2)
	assert.True(t, errors.Is(err, ErrRangBuoc))
	assert.Equal(t, "đơn đã có 2 phiếu thanh toán", err.Error())
}
