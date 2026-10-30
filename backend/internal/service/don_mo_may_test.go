package service

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/model"
)

// Máy có đơn khách đang gọi hoặc chưa thanh toán thì đổi màu, và khách rời máy
// lúc còn đơn thì quầy nghe báo động.

// Đơn đi theo máy khách ngồi gần nhất (đang ngồi hoặc vừa bỏ về); khách vãng
// lai thì theo máy đã gọi món; đơn gọi ở quầy của hội viên chưa ngồi máy nào thì
// không tô máy nào.
func TestDonMoTheoMay_DonDiTheoKhach(t *testing.T) {
	db, mock := newMockDB(t)

	mock.ExpectQuery(`SELECT "id","status","member_id","machine_id","final_amount" FROM "orders" WHERE \(status IN \(\$1,\$2\) AND order_type <> \$3\) AND "orders"\."deleted_at" IS NULL`).
		WithArgs("pending", "confirmed", model.OrderTypeTopup).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "member_id", "machine_id", "final_amount"}).
			AddRow("o1", "pending", "mem-a", "may01", 20000).   // gọi ở MAY01 rồi đổi sang VIP01
			AddRow("o2", "confirmed", nil, "may02", 15000).     // khách vãng lai
			AddRow("o3", "confirmed", "mem-b", "may03", 30000). // gọi ở MAY03, đổi sang MAY04 rồi bỏ về
			AddRow("o4", "confirmed", "mem-c", nil, 10000).     // gọi ở quầy, chưa ngồi máy nào
			AddRow("o5", "confirmed", "mem-a", nil, 5000))      // gọi ở quầy, đang ngồi VIP01
	mock.ExpectQuery(`SELECT DISTINCT ON \(member_id\) member_id, machine_id FROM "machine_sessions" WHERE member_id IN \(\$1,\$2,\$3\) ORDER BY member_id, started_at DESC`).
		WithArgs("mem-a", "mem-b", "mem-c").
		WillReturnRows(sqlmock.NewRows([]string{"member_id", "machine_id"}).
			AddRow("mem-a", "vip01").
			AddRow("mem-b", "may04"))

	got, err := donMoTheoMay(db)
	require.NoError(t, err)
	assert.Equal(t, []DonMoMay{
		{MachineID: "vip01", PendingCount: 1, UnpaidCount: 1, Amount: 25000},
		{MachineID: "may02", UnpaidCount: 1, Amount: 15000},
		{MachineID: "may04", UnpaidCount: 1, Amount: 30000},
	}, got)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Hộp "Đơn đang gọi" của một máy lấy đúng những đơn làm máy đó đỏ — kể cả đơn
// gọi ở quầy của khách đã rời máy.
func TestDonMoCuaMay(t *testing.T) {
	db, mock := newMockDB(t)

	mock.ExpectQuery(`SELECT "id","status","member_id","machine_id","final_amount" FROM "orders"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "member_id", "machine_id", "final_amount"}).
			AddRow("o1", "confirmed", "mem-a", nil, 20000).    // gọi ở quầy, khách đã rời MAY35
			AddRow("o2", "confirmed", "mem-a", "may35", 5000). // gọi trên MAY35
			AddRow("o3", "confirmed", nil, "may35", 7000).     // khách vãng lai MAY35
			AddRow("o4", "pending", "mem-b", "may35", 9000))   // gọi ở MAY35 rồi đổi sang MAY40
	mock.ExpectQuery(`SELECT DISTINCT ON \(member_id\) member_id, machine_id FROM "machine_sessions"`).
		WithArgs("mem-a", "mem-b").
		WillReturnRows(sqlmock.NewRows([]string{"member_id", "machine_id"}).
			AddRow("mem-a", "may35").
			AddRow("mem-b", "may40"))

	got, err := donMoCuaMay(db, "may35")
	require.NoError(t, err)
	assert.Equal(t, []string{"o1", "o2", "o3"}, got)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// expectDonTheoKhach: mở phiên (cả đổi máy) chuyển đơn chưa thu tiền của hội viên
// sang máy mới. Đơn nạp tiền, đơn đã xong hay đã huỷ thì giữ máy cũ.
func expectDonTheoKhach(mock sqlmock.Sqlmock, memberID, machineID string) {
	mock.ExpectExec(`UPDATE "orders" SET "machine_id"=\$1 WHERE \(member_id = \$2 AND status IN \(\$3,\$4\) AND order_type <> \$5\) AND \(machine_id IS NULL OR machine_id <> \$6\) AND "orders"\."deleted_at" IS NULL`).
		WithArgs(machineID, memberID, "pending", "confirmed", model.OrderTypeTopup, machineID).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestDonTheoKhach(t *testing.T) {
	db, mock := newMockDB(t)
	// Ngoài giao dịch thì GORM tự bọc lệnh UPDATE trong một giao dịch riêng.
	mock.ExpectBegin()
	expectDonTheoKhach(mock, "mem-a", "may40")
	mock.ExpectCommit()

	require.NoError(t, donTheoKhach(db, "mem-a", "may40"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDonMoTheoMay_KhongCoDon(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectQuery(`SELECT .* FROM "orders"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	got, err := donMoTheoMay(db)
	require.NoError(t, err)
	assert.Equal(t, []DonMoMay{}, got)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDanhDauBaoDonMo_MotLanMoiPhien(t *testing.T) {
	now := time.Now()
	assert.True(t, danhDauBaoDonMo("phien-danh-dau", now))
	assert.False(t, danhDauBaoDonMo("phien-danh-dau", now.Add(2*time.Minute)), "máy tắt rồi phiên đóng vì mất tín hiệu: chỉ báo một lần")
	assert.True(t, danhDauBaoDonMo("phien-khac", now), "phiên khác vẫn báo")
	assert.True(t, danhDauBaoDonMo("phien-danh-dau", now.Add(baoDonMoTrongVong)), "hết quãng chặn thì báo lại")
}

func expectDonMoCuaKhach(mock sqlmock.Sqlmock, soDon int) {
	mock.ExpectQuery(`SELECT COUNT\(\*\) AS so_don, COUNT\(\*\) FILTER \(WHERE status = 'pending'\) AS dang_goi, COALESCE\(SUM\(final_amount\), 0\) AS tong FROM "orders" WHERE \(status IN \(\$1,\$2\) AND order_type <> \$3\) AND \(member_id = \$4 OR \(member_id IS NULL AND machine_id = \$5\)\)`).
		WithArgs("pending", "confirmed", model.OrderTypeTopup, "mem-a", "may01").
		WillReturnRows(sqlmock.NewRows([]string{"so_don", "dang_goi", "tong"}).AddRow(soDon, 1, 45000))
}

// Còn đơn thì báo (đọc tên khách để hiện lên thông báo), và lần rớt kết nối thứ
// hai của cùng phiên không báo nữa.
func TestBaoDonMoKhiRoiMay_BaoMotLan(t *testing.T) {
	db, mock := newMockDB(t)
	h := hub.New(nil)
	may := &model.Machine{ID: "may01", MachineCode: "MAY01"}

	expectDonMoCuaKhach(mock, 2)
	mock.ExpectQuery(`SELECT "full_name","username" FROM "members" WHERE id = \$1`).
		WithArgs("mem-a", 1).
		WillReturnRows(sqlmock.NewRows([]string{"full_name", "username"}).AddRow("Nguyễn A", "nguyena"))
	baoDonMoKhiRoiMay(db, h, "phien-bao-mot-lan", "mem-a", may, "offline")

	expectDonMoCuaKhach(mock, 2)
	baoDonMoKhiRoiMay(db, h, "phien-bao-mot-lan", "mem-a", may, "ended")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Không còn đơn thì im lặng và KHÔNG đánh dấu phiên: khách gọi món sau đó rồi
// mới về thì vẫn phải báo.
func TestBaoDonMoKhiRoiMay_KhongConDon(t *testing.T) {
	db, mock := newMockDB(t)
	h := hub.New(nil)
	may := &model.Machine{ID: "may01", MachineCode: "MAY01"}

	expectDonMoCuaKhach(mock, 0)
	baoDonMoKhiRoiMay(db, h, "phien-khong-don", "mem-a", may, "offline")

	expectDonMoCuaKhach(mock, 1)
	mock.ExpectQuery(`SELECT "full_name","username" FROM "members"`).
		WillReturnRows(sqlmock.NewRows([]string{"full_name", "username"}).AddRow("", "nguyena"))
	baoDonMoKhiRoiMay(db, h, "phien-khong-don", "mem-a", may, "ended")
	assert.NoError(t, mock.ExpectationsWereMet())
}
