package service

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/utils"
)

// Tài khoản combo sinh ra lúc bán gói có số dư 0. Đăng nhập ở máy trạm phải mở
// phiên bằng gói đó, không thì khách bị chặn "số dư 0₫ chưa đủ mức tối thiểu".

func luc(gio, phut int) time.Time {
	return time.Date(2026, 11, 2, gio, phut, 0, 0, utils.VietnamLocation())
}

func moGoiDangNhap(mock sqlmock.Sqlmock, goi *sqlmock.Rows) {
	mock.ExpectQuery(`SELECT \* FROM "combo_purchases" WHERE member_id = \$1 AND current_session_id IS NULL AND \(expires_at IS NULL OR expires_at > \$2\) ORDER BY activated DESC, created_at`).
		WithArgs("mem1", anyTime{}).
		WillReturnRows(goi)
}

func moCombo(mock sqlmock.Sqlmock, id, ten, loai, bd, kt string) {
	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1 ORDER BY "combos"\."id" LIMIT \$2`).
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type", "slot_start", "slot_end"}).
			AddRow(id, ten, loai, bd, kt))
}

var mayDangNhap = &model.Machine{ID: "m1", MachineCode: "MAY01"}

func TestGoiDungKhiDangNhap_TrongKhungGio(t *testing.T) {
	db, mock := newMockDB(t)
	moGoiDangNhap(mock, sqlmock.NewRows([]string{"id", "combo_id", "activated"}).AddRow("cp1", "c1", false))
	moCombo(mock, "c1", "sang", "fixed_slot", "07:00:00", "12:00:00")

	id, lyDo, err := goiDungKhiDangNhap(db, "mem1", mayDangNhap, luc(9, 30))
	require.NoError(t, err)
	assert.Equal(t, "cp1", id)
	assert.Empty(t, lyDo)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGoiDungKhiDangNhap_NgoaiKhungGioThiBaoLyDo(t *testing.T) {
	db, mock := newMockDB(t)
	moGoiDangNhap(mock, sqlmock.NewRows([]string{"id", "combo_id", "activated"}).AddRow("cp1", "c1", false))
	moCombo(mock, "c1", "sang", "fixed_slot", "07:00:00", "12:00:00")

	id, lyDo, err := goiDungKhiDangNhap(db, "mem1", mayDangNhap, luc(12, 0))
	require.NoError(t, err)
	assert.Empty(t, id, "12:00 là hết khung 07:00–12:00")
	assert.Equal(t, "gói sang chỉ dùng trong khung 07:00–12:00", lyDo)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGoiDungKhiDangNhap_KhungQuaNuaDem(t *testing.T) {
	db, mock := newMockDB(t)
	moGoiDangNhap(mock, sqlmock.NewRows([]string{"id", "combo_id", "activated"}).AddRow("cp1", "c1", false))
	moCombo(mock, "c1", "quadem", "fixed_slot", "22:00:00", "06:00:00")

	id, _, err := goiDungKhiDangNhap(db, "mem1", mayDangNhap, luc(23, 15))
	require.NoError(t, err)
	assert.Equal(t, "cp1", id)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGoiDungKhiDangNhap_GoiHetPhutThiDungGoiKeTiep(t *testing.T) {
	db, mock := newMockDB(t)
	moGoiDangNhap(mock, sqlmock.NewRows([]string{"id", "combo_id", "activated", "remaining_minutes"}).
		AddRow("cp1", "c1", true, 0).
		AddRow("cp2", "c1", false, 120))
	moCombo(mock, "c1", "5gio", "hours", "", "")
	moCombo(mock, "c1", "5gio", "hours", "", "")

	id, lyDo, err := goiDungKhiDangNhap(db, "mem1", mayDangNhap, luc(9, 0))
	require.NoError(t, err)
	assert.Equal(t, "cp2", id)
	assert.Empty(t, lyDo)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGoiDungKhiDangNhap_KhongCoGoi(t *testing.T) {
	db, mock := newMockDB(t)
	moGoiDangNhap(mock, sqlmock.NewRows([]string{"id"}))

	id, lyDo, err := goiDungKhiDangNhap(db, "mem1", mayDangNhap, luc(9, 0))
	require.NoError(t, err)
	assert.Empty(t, id)
	assert.Empty(t, lyDo, "hội viên thường không có gói: để lỗi số dư tự nói")
	assert.NoError(t, mock.ExpectationsWereMet())
}
