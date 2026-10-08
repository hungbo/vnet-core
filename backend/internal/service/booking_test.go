package service

import (
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/utils"
)

// Mã máy, hội viên, lịch phải là uuid thật: service kiểm định dạng trước khi
// đưa vào SQL, nên "m1" giờ bị từ chối ngay ở cửa.
const (
	tMayID  = "11111111-1111-1111-1111-111111111111"
	tHvID   = "22222222-2222-2222-2222-222222222222"
	tLichID = "33333333-3333-3333-3333-333333333333"
	tCocID  = "44444444-4444-4444-4444-444444444444"
)

// gioSau trả chuỗi RFC3339 cách bây giờ d — test phải dùng giờ tương đối vì
// Create từ chối giờ bắt đầu đã qua.
func gioSau(d time.Duration) string { return time.Now().Add(d).Format(time.RFC3339) }

// laMoc khớp một tham số time.Time đúng bằng want (bỏ qua múi giờ).
type laMoc struct{ want time.Time }

func (m laMoc) Match(v driver.Value) bool {
	t, ok := v.(time.Time)
	return ok && t.Equal(m.want)
}

// ghiNhan khớp mọi tham số và ghi lại giá trị để kiểm sau.
type ghiNhan struct{ seen *[]driver.Value }

func (g ghiNhan) Match(v driver.Value) bool {
	*g.seen = append(*g.seen, v)
	return true
}

var cotLich = []string{"id", "machine_id", "member_id", "status", "booked_from", "booked_to", "deposit_amount", "deposit_transaction_id", "notes"}

func dongLich(status string, from time.Time, deposit int64, cocTx interface{}, memberID interface{}) *sqlmock.Rows {
	return sqlmock.NewRows(cotLich).
		AddRow(tLichID, tMayID, memberID, status, from, from.Add(2*time.Hour), deposit, cocTx, "ghi chú cũ")
}

func expectDocLich(mock sqlmock.Sqlmock, rows *sqlmock.Rows) {
	mock.ExpectQuery(`SELECT \* FROM "machine_bookings" WHERE id = \$1 AND "machine_bookings"\."deleted_at" IS NULL ORDER BY "machine_bookings"\."id" LIMIT \$2$`).
		WithArgs(tLichID, 1).
		WillReturnRows(rows)
}

func expectKhoaLich(mock sqlmock.Sqlmock, rows *sqlmock.Rows) {
	mock.ExpectQuery(`SELECT \* FROM "machine_bookings" WHERE id = \$1 AND "machine_bookings"\."deleted_at" IS NULL ORDER BY "machine_bookings"\."id" LIMIT \$2 FOR UPDATE`).
		WithArgs(tLichID, 1).
		WillReturnRows(rows)
}

func expectKhoaMay(mock sqlmock.Sqlmock, active bool) {
	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 AND "machines"\."deleted_at" IS NULL ORDER BY "machines"\."id" LIMIT \$2 FOR UPDATE`).
		WithArgs(tMayID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code", "is_active"}).AddRow(tMayID, "PC-05", active))
}

func expectKhoaHoiVien(mock sqlmock.Sqlmock, balance int64) {
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$2 FOR UPDATE`).
		WithArgs(tHvID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance", "is_active"}).AddRow(tHvID, balance, true))
}

func expectCaiDatLimits(mock sqlmock.Sqlmock, kv ...string) {
	rows := sqlmock.NewRows([]string{"key", "value", "group_name"})
	for i := 0; i+1 < len(kv); i += 2 {
		rows.AddRow(kv[i], kv[i+1], "limits")
	}
	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1`).
		WithArgs("limits").
		WillReturnRows(rows)
}

const sqlTrungLich = `SELECT count\(\*\) FROM "machine_bookings" WHERE \(machine_id = \$1 AND status NOT IN \('cancelled', 'completed', 'no_show'\) AND booked_from < \$2 AND booked_to > \$3\)`

func TestBookingService_List(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "machine_bookings" WHERE "machine_bookings"\."deleted_at" IS NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT \* FROM "machine_bookings" WHERE "machine_bookings"\."deleted_at" IS NULL ORDER BY created_at desc LIMIT \$1`).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "customer_name", "status"}).
			AddRow("b1", "John", "pending"))

	result, err := svc.List(&BookingListRequest{Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.Len(t, result.Items, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Bộ lọc sai phải thành 400 có lý do: machine_id "abc" từng làm PostgreSQL ném
// lỗi (500), ngày sai định dạng bị bỏ qua (ra MỌI lịch), trạng thái lạ ra rỗng.
func TestBookingService_List_BoLocSaiBaoLoi(t *testing.T) {
	cases := []BookingListRequest{
		{Status: "noshow"},
		{MachineID: "abc"},
		{DateFrom: "2026-10-04"},
		{DateTo: "04/10/2026"},
	}
	for _, req := range cases {
		db, mock := newMockDB(t)
		svc := NewBookingService(db, NewAuditService(db))
		r := req
		_, err := svc.List(&r)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrDatChoKhongHopLe), err.Error())
		assert.NoError(t, mock.ExpectationsWereMet())
	}
}

func TestBookingService_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	now := time.Now()
	mock.ExpectQuery(`SELECT \* FROM "machine_bookings" WHERE id = \$1 AND "machine_bookings"\."deleted_at" IS NULL ORDER BY "machine_bookings"."id" LIMIT \$2`).
		WithArgs(tLichID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "customer_name", "booked_from", "booked_to", "created_at", "updated_at"}).
			AddRow(tLichID, "John", now, now, now, now))

	result, err := svc.GetByID(tLichID)
	require.NoError(t, err)
	assert.Equal(t, "John", result.CustomerName)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBookingService_GetByID_IDSaiKhongRaLoiSQL(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	_, err := svc.GetByID("abc")
	require.Error(t, err)
	assert.Equal(t, "không tìm thấy lịch đặt máy", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Khách vãng lai không cọc: khoá máy, kiểm trùng (đã loại no_show) và chèn —
// tất cả trong MỘT transaction để hai lệnh đặt song song không cùng lọt.
func TestBookingService_Create_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	expectCaiDatLimits(mock)
	mock.ExpectBegin()
	expectKhoaMay(mock, true)
	mock.ExpectQuery(sqlTrungLich).
		WithArgs(tMayID, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`INSERT INTO "machine_bookings"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(tLichID))
	mock.ExpectCommit()

	// Trang quản trị gửi giờ UTC (toISOString).
	from := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	result, err := svc.Create(&CreateBookingRequest{
		MachineID:     tMayID,
		CustomerName:  "John",
		CustomerPhone: "0123456789",
		BookedFrom:    from.Format(time.RFC3339),
		BookedTo:      from.Add(2 * time.Hour).Format(time.RFC3339),
	}, "u1")
	require.NoError(t, err)
	assert.Equal(t, "John", result.CustomerName)
	assert.Equal(t, "pending", result.Status)
	// Phản hồi của Create phải cùng múi +07:00 với GET.
	assert.True(t, strings.HasSuffix(result.BookedFrom, "+07:00"), result.BookedFrom)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBookingService_Create_InvalidTimeFormat(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	_, err := svc.Create(&CreateBookingRequest{
		MachineID:     tMayID,
		CustomerName:  "John",
		CustomerPhone: "0123456789",
		BookedFrom:    "invalid-time",
		BookedTo:      "2026-06-25T12:00:00+07:00",
	}, "u1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "giờ bắt đầu không đúng định dạng")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Những lỗi đầu vào bị chặn trước khi chạm database.
func TestBookingService_Create_TuChoiDauVaoSai(t *testing.T) {
	cases := []struct {
		name string
		req  CreateBookingRequest
		want string
	}{
		{"giờ đã qua", CreateBookingRequest{MachineID: tMayID, BookedFrom: gioSau(-time.Hour), BookedTo: gioSau(time.Hour)}, "đã qua"},
		{"kết thúc trước bắt đầu", CreateBookingRequest{MachineID: tMayID, BookedFrom: gioSau(2 * time.Hour), BookedTo: gioSau(time.Hour)}, "giờ kết thúc phải sau"},
		{"cọc âm", CreateBookingRequest{MachineID: tMayID, MemberID: tHvID, BookedFrom: gioSau(time.Hour), BookedTo: gioSau(2 * time.Hour), DepositAmount: -1}, "không được âm"},
		{"vãng lai đặt cọc", CreateBookingRequest{MachineID: tMayID, BookedFrom: gioSau(time.Hour), BookedTo: gioSau(2 * time.Hour), DepositAmount: 20000}, "khách vãng lai không đặt cọc"},
		{"machine_id rác", CreateBookingRequest{MachineID: "abc", BookedFrom: gioSau(time.Hour), BookedTo: gioSau(2 * time.Hour)}, "không tìm thấy máy"},
		{"member_id rác", CreateBookingRequest{MachineID: tMayID, MemberID: "abc", BookedFrom: gioSau(time.Hour), BookedTo: gioSau(2 * time.Hour)}, "không tìm thấy hội viên"},
	}
	for _, c := range cases {
		db, mock := newMockDB(t)
		svc := NewBookingService(db, NewAuditService(db))
		r := c.req
		_, err := svc.Create(&r, "u1")
		require.Error(t, err, c.name)
		assert.Contains(t, err.Error(), c.want, c.name)
		assert.NoError(t, mock.ExpectationsWereMet(), c.name)
	}
}

func TestBookingService_Create_MayKhongTonTaiHoacBiKhoa(t *testing.T) {
	// Máy không có.
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))
	expectCaiDatLimits(mock)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 .* FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectRollback()
	_, err := svc.Create(&CreateBookingRequest{MachineID: tMayID, BookedFrom: gioSau(time.Hour), BookedTo: gioSau(2 * time.Hour)}, "u1")
	require.Error(t, err)
	assert.Equal(t, "không tìm thấy máy", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())

	// Máy bị khoá.
	db, mock = newMockDB(t)
	svc = NewBookingService(db, NewAuditService(db))
	expectCaiDatLimits(mock)
	mock.ExpectBegin()
	expectKhoaMay(mock, false)
	mock.ExpectRollback()
	_, err = svc.Create(&CreateBookingRequest{MachineID: tMayID, BookedFrom: gioSau(time.Hour), BookedTo: gioSau(2 * time.Hour)}, "u1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PC-05 đang bị khoá")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBookingService_Create_HoiVienKhongTonTai(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))
	expectCaiDatLimits(mock)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 .* FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectRollback()

	_, err := svc.Create(&CreateBookingRequest{MachineID: tMayID, MemberID: tHvID, BookedFrom: gioSau(time.Hour), BookedTo: gioSau(2 * time.Hour)}, "u1")
	require.Error(t, err)
	assert.Equal(t, "không tìm thấy hội viên", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

// A deposit has to leave the member's balance when the booking is made, and
// the ledger row must point back at the booking (reference_id).
func TestBookingService_Create_ChargesDeposit(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	expectCaiDatLimits(mock)
	mock.ExpectBegin()
	expectKhoaHoiVien(mock, 500000)
	expectKhoaMay(mock, true)
	mock.ExpectQuery(sqlTrungLich).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	// Lịch chèn TRƯỚC để giao dịch cọc có reference_id trỏ về nó.
	mock.ExpectQuery(`INSERT INTO "machine_bookings"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(tLichID))
	var thamSo []driver.Value
	ghi := ghiNhan{seen: &thamSo}
	mock.ExpectQuery(`INSERT INTO "member_transactions" .*"reference_id"`).
		WithArgs(ghi, ghi, ghi, ghi, ghi, ghi, ghi, ghi, ghi, ghi, ghi, ghi).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(tCocID))
	mock.ExpectExec(`UPDATE "members" SET "balance"=\$1,"updated_at"=\$2 WHERE "members"\."deleted_at" IS NULL AND "id" = \$3`).
		WithArgs(int64(400000), anyTime{}, tHvID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE "machine_bookings" SET "deposit_transaction_id"=\$1`).
		WithArgs(tCocID, anyTime{}, tLichID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	res, err := svc.Create(&CreateBookingRequest{
		MachineID:     tMayID,
		MemberID:      tHvID,
		BookedFrom:    gioSau(time.Hour),
		BookedTo:      gioSau(3 * time.Hour),
		DepositAmount: 100000,
	}, "u1")

	require.NoError(t, err)
	require.NotNil(t, res.DepositTransactionID)
	assert.Equal(t, tCocID, *res.DepositTransactionID)
	assert.Contains(t, thamSo, driver.Value(tLichID), "giao dịch cọc phải mang reference_id = id lịch")
	coMoTaViet := false
	for _, v := range thamSo {
		if s, ok := v.(string); ok && strings.HasPrefix(s, "Đặt cọc giữ máy") {
			coMoTaViet = true
		}
	}
	assert.True(t, coMoTaViet, "mô tả giao dịch cọc phải bằng tiếng Việt")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBookingService_Create_RejectsDepositBeyondBalance(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	expectCaiDatLimits(mock)
	mock.ExpectBegin()
	expectKhoaHoiVien(mock, 10)
	expectKhoaMay(mock, true)
	mock.ExpectQuery(sqlTrungLich).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`INSERT INTO "machine_bookings"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(tLichID))
	mock.ExpectRollback()

	_, err := svc.Create(&CreateBookingRequest{
		MachineID:     tMayID,
		MemberID:      tHvID,
		BookedFrom:    gioSau(time.Hour),
		BookedTo:      gioSau(3 * time.Hour),
		DepositAmount: 100000,
	}, "u1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "số dư không đủ")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Trần mỗi ngày tính theo NGÀY VIỆT NAM. Lịch 00:30 sáng giờ ta là 17:30 UTC
// hôm trước; bản cũ lấy ngày theo múi chuỗi gửi lên (UTC từ trang quản trị)
// nên đếm nó vào ngày hôm trước.
func TestBookingService_Create_TranNgayTheoGioVietNam(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	ngayVN := utils.StartOfDay(time.Now().Add(72 * time.Hour))
	from := ngayVN.Add(30 * time.Minute).UTC()

	expectCaiDatLimits(mock, "max_bookings_per_day", "1")
	mock.ExpectBegin()
	expectKhoaMay(mock, true)
	mock.ExpectQuery(sqlTrungLich).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "machine_bookings" WHERE \(status NOT IN \('cancelled', 'no_show'\) AND booked_from >= \$1 AND booked_from < \$2\)`).
		WithArgs(laMoc{ngayVN}, laMoc{ngayVN.AddDate(0, 0, 1)}).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectRollback()

	_, err := svc.Create(&CreateBookingRequest{
		MachineID:  tMayID,
		BookedFrom: from.Format(time.RFC3339),
		BookedTo:   from.Add(time.Hour).Format(time.RFC3339),
	}, "u1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), ngayVN.Format("02/01/2006"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Cọc không sửa được sau khi tạo: sửa con số trên lịch không đụng ví, mà huỷ
// lại hoàn theo con số đã sửa.
func TestBookingService_Update_TuChoiDoiCoc(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	expectDocLich(mock, dongLich("pending", time.Now().Add(3*time.Hour), 25000, tCocID, tHvID))

	coc := int64(500000)
	_, err := svc.Update(tLichID, &UpdateBookingRequest{DepositAmount: &coc}, "u1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "không sửa được tiền cọc")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBookingService_Update_ChiSuaLichDangCho(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	from := time.Now().Add(3 * time.Hour)
	expectDocLich(mock, dongLich("pending", from, 0, nil, nil))
	mock.ExpectBegin()
	expectKhoaMay(mock, true)
	expectKhoaLich(mock, dongLich("no_show", from, 0, nil, nil))
	mock.ExpectRollback()

	_, err := svc.Update(tLichID, &UpdateBookingRequest{CustomerName: "B"}, "u1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chỉ sửa được lịch đang chờ")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Đổi giờ phải kiểm trùng lịch (trừ chính nó) như lúc tạo.
func TestBookingService_Update_DoiGioKiemTrungLich(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	from := time.Now().Add(3 * time.Hour).Truncate(time.Second)
	expectDocLich(mock, dongLich("pending", from, 0, nil, nil))
	expectCaiDatLimits(mock) // hạn huỷ: không đặt
	expectCaiDatLimits(mock) // trần ngày
	mock.ExpectBegin()
	expectKhoaMay(mock, true)
	expectKhoaLich(mock, dongLich("pending", from, 0, nil, nil))
	mock.ExpectQuery(sqlTrungLich+` AND id <> \$4`).
		WithArgs(tMayID, sqlmock.AnyArg(), sqlmock.AnyArg(), tLichID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectRollback()

	_, err := svc.Update(tLichID, &UpdateBookingRequest{
		BookedFrom: from.Add(time.Hour).Format(time.RFC3339),
		BookedTo:   from.Add(3 * time.Hour).Format(time.RFC3339),
	}, "u1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "đã có người đặt")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBookingService_Update_DoiGioSaiThuTuHoacQuaKhu(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	from := time.Now().Add(3 * time.Hour).Truncate(time.Second)
	expectDocLich(mock, dongLich("pending", from, 0, nil, nil))
	expectCaiDatLimits(mock)

	_, err := svc.Update(tLichID, &UpdateBookingRequest{
		BookedFrom: from.Format(time.RFC3339),
		BookedTo:   from.Add(-time.Hour).Format(time.RFC3339),
	}, "u1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "giờ kết thúc phải sau")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Đổi giờ khi đã quá hạn huỷ cũng bị từ chối — dời lịch sát giờ là nhả chỗ
// y như huỷ.
func TestBookingService_Update_DoiGioQuaHanHuy(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	from := time.Now().Add(30 * time.Minute).Truncate(time.Second)
	expectDocLich(mock, dongLich("pending", from, 0, nil, nil))
	expectCaiDatLimits(mock, "cancel_before_minutes", "60")

	_, err := svc.Update(tLichID, &UpdateBookingRequest{
		BookedFrom: from.Add(24 * time.Hour).Format(time.RFC3339),
		BookedTo:   from.Add(26 * time.Hour).Format(time.RFC3339),
	}, "u1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chỉ đổi giờ được trước giờ giữ máy ít nhất 60 phút")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// PUT không gửi notes thì giữ nguyên ghi chú — bản cũ ghi đè bằng "".
func TestBookingService_Update_KhongGuiNotesThiGiuNguyen(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	from := time.Now().Add(3 * time.Hour)
	expectDocLich(mock, dongLich("pending", from, 0, nil, nil))
	mock.ExpectBegin()
	expectKhoaMay(mock, true)
	expectKhoaLich(mock, dongLich("pending", from, 0, nil, nil))
	mock.ExpectExec(`UPDATE "machine_bookings" SET "customer_name"=\$1,"updated_at"=\$2 WHERE`).
		WithArgs("Tên mới", anyTime{}, tLichID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	_, err := svc.Update(tLichID, &UpdateBookingRequest{CustomerName: "Tên mới"}, "u1")
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Cancelling a booking whose deposit was never charged must not pay anything out.
func TestBookingService_Cancel_NoRefundWithoutCharge(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	from := time.Now().Add(3 * time.Hour)
	expectDocLich(mock, dongLich("pending", from, 100000, nil, tHvID))
	expectCaiDatLimits(mock)
	mock.ExpectBegin()
	// Cancel đọc lại lịch đặt dưới khoá trong chính transaction: bản đọc ở trên
	// nằm ngoài, nên hai lệnh huỷ song song đều hoàn cọc nếu không có bước này.
	expectKhoaLich(mock, dongLich("pending", from, 100000, nil, tHvID))
	mock.ExpectExec(`UPDATE "machine_bookings" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	_, err := svc.Cancel(tLichID, "u1")

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Huỷ hoàn đúng số đã TRỪ (đọc từ giao dịch cọc), không phải deposit_amount
// trên lịch — con số đó trước đây sửa được qua PUT.
func TestBookingService_Cancel_HoanTheoSoDaThu(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	from := time.Now().Add(3 * time.Hour)
	// Lịch ghi 500.000đ (đã bị sửa) nhưng giao dịch cọc thật chỉ -25.000đ.
	expectDocLich(mock, dongLich("pending", from, 500000, tCocID, tHvID))
	expectCaiDatLimits(mock)
	mock.ExpectBegin()
	expectKhoaHoiVien(mock, 75000)
	expectKhoaLich(mock, dongLich("pending", from, 500000, tCocID, tHvID))
	mock.ExpectQuery(`SELECT \* FROM "member_transactions" WHERE id = \$1`).
		WithArgs(tCocID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "amount"}).AddRow(tCocID, int64(-25000)))
	mock.ExpectQuery(`INSERT INTO "member_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("rf-1"))
	mock.ExpectExec(`UPDATE "members" SET "balance"=\$1`).
		WithArgs(int64(100000), anyTime{}, tHvID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE "machine_bookings" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	_, err := svc.Cancel(tLichID, "u1")
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Nhận máy trả cọc vào ví — cọc thành tiền chơi. Bản cũ chỉ đổi trạng thái,
// cọc biến mất.
func TestBookingService_CheckIn_HoanCocVaoVi(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	from := time.Now().Add(10 * time.Minute)
	expectDocLich(mock, dongLich("pending", from, 25000, tCocID, tHvID))
	mock.ExpectBegin()
	expectKhoaHoiVien(mock, 5000)
	expectKhoaLich(mock, dongLich("pending", from, 25000, tCocID, tHvID))
	mock.ExpectQuery(`SELECT \* FROM "member_transactions" WHERE id = \$1`).
		WithArgs(tCocID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "amount"}).AddRow(tCocID, int64(-25000)))
	mock.ExpectQuery(`INSERT INTO "member_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("rf-1"))
	mock.ExpectExec(`UPDATE "members" SET "balance"=\$1`).
		WithArgs(int64(30000), anyTime{}, tHvID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE "machine_bookings" SET "status"=\$1,"updated_at"=\$2 WHERE`).
		WithArgs("checked_in", anyTime{}, tLichID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	res, err := svc.CheckIn(tLichID, "u1")
	require.NoError(t, err)
	assert.Equal(t, "checked_in", res.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBookingService_CheckIn_QuaSom(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	from := time.Now().Add(2 * time.Hour)
	expectDocLich(mock, dongLich("pending", from, 0, nil, nil))
	mock.ExpectBegin()
	expectKhoaLich(mock, dongLich("pending", from, 0, nil, nil))
	mock.ExpectRollback()

	_, err := svc.CheckIn(tLichID, "u1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chưa tới giờ nhận máy")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Đánh vắng tay chỉ được sau mốc của tác vụ nền (bắt đầu + 15 phút).
func TestBookingService_NoShow_TruocMocBiTuChoi(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	from := time.Now().Add(-5 * time.Minute)
	expectDocLich(mock, dongLich("pending", from, 0, nil, nil))
	mock.ExpectBegin()
	expectKhoaLich(mock, dongLich("pending", from, 0, nil, nil))
	mock.ExpectRollback()

	_, err := svc.NoShow(tLichID, "u1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chỉ đánh vắng được từ")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Không đến = mất cọc: KHÔNG có dòng hoàn cọc nào và ví không bị đụng lần nữa
// (cọc đã rời ví lúc đặt). Không có UPDATE "members" nào trong chuỗi dưới đây.
//
// Doanh thu: báo cáo tính theo tiền VÀO QUÁN, đồng cọc này đã nằm trong doanh
// thu từ lần nạp ví sinh ra nó, nên không cộng thêm (xem
// TestBookingService_TienMatCocKhongDemHaiLanTrongDoanhThu).
func TestBookingService_NoShow_MatCocKhongDungVi(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	from := time.Now().Add(-20 * time.Minute)
	expectDocLich(mock, dongLich("pending", from, 25000, tCocID, tHvID))
	mock.ExpectBegin()
	expectKhoaLich(mock, dongLich("pending", from, 25000, tCocID, tHvID))
	mock.ExpectQuery(`SELECT \* FROM "member_transactions" WHERE id = \$1`).
		WithArgs(tCocID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "amount"}).AddRow(tCocID, int64(-25000)))
	mock.ExpectExec(`UPDATE "machine_bookings" SET "status"=\$1,"updated_at"=\$2 WHERE`).
		WithArgs("no_show", anyTime{}, tLichID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	res, err := svc.NoShow(tLichID, "u1")
	require.NoError(t, err)
	assert.Equal(t, "no_show", res.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Doanh thu ngày chỉ đếm tiền vào quầy (topup, refund, combo tiền mặt): dòng
// booking_deposit / deposit_refund không nằm trong bộ lọc, nên cọc mất do
// không đến được tính đúng MỘT lần — lúc khách nạp tiền vào ví.
func TestBookingService_TienMatCocKhongDemHaiLanTrongDoanhThu(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewReportService(db)

	mock.ExpectQuery(`SELECT .* FROM "orders"`).
		WillReturnRows(sqlmock.NewRows([]string{"date", "total_orders", "revenue", "discount"}))
	mock.ExpectQuery(`SELECT .* FROM "member_transactions" WHERE \(\(transaction_type IN \('topup', 'refund'\) OR \(transaction_type = 'combo_purchase' AND payment_method = 'cash'\)\)`).
		WillReturnRows(sqlmock.NewRows([]string{"date", "amount", "count"}).AddRow("2026-10-04", int64(100000), 1))

	rows, err := svc.DailyRevenue("2026-10-04", "2026-10-04")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(100000), rows[0].Revenue)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Lịch không đến / đã huỷ đã xong tiền cọc (mất hoặc hoàn) nên xoá được — bản
// cũ chặn mọi lịch có cọc nên chúng nằm vĩnh viễn trên trang.
func TestBookingService_Delete_ChoXoaLichDaXongCoc(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	expectDocLich(mock, dongLich("no_show", time.Now().Add(-time.Hour), 25000, tCocID, tHvID))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "machine_bookings" SET "deleted_at"=\$1`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	require.NoError(t, svc.Delete(tLichID, "u1"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Từ giờ bắt đầu tới hạn đánh vắng, máy chỉ dành cho chủ lịch: hội viên khác
// mở máy (ở quầy hay tự đăng nhập) đều bị từ chối.
func TestSessionService_StartSession_MayDaDatTruocChanNguoiKhac(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSessionService(db, hub.New(nil), NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE \(id = \$1 AND is_active = \$2\)`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code", "status", "is_active"}).
			AddRow(tMayID, "PC-05", "available", true))
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE \(id = \$1 AND is_active = \$2\)`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "full_name", "is_active", "balance"}).
			AddRow("mem-khac", "Người khác", true, 100000))
	mock.ExpectQuery(`SELECT \* FROM "machine_sessions" WHERE member_id = \$1 AND is_active = \$2`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 .* FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "is_active", "balance"}).AddRow("mem-khac", true, 100000))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "machine_sessions" WHERE member_id = \$1 AND is_active = \$2`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 .* FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code", "status", "is_active"}).
			AddRow(tMayID, "PC-05", "available", true))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "machine_sessions" WHERE machine_id = \$1 AND is_active = \$2`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	from := time.Now().Add(-5 * time.Minute)
	mock.ExpectQuery(`SELECT \* FROM "machine_bookings" WHERE \(machine_id = \$1 AND status = \$2 AND booked_from <= \$3 AND booked_from > \$4\) AND "machine_bookings"\."deleted_at" IS NULL ORDER BY booked_from LIMIT \$5 FOR UPDATE`).
		WithArgs(tMayID, "pending", anyTime{}, anyTime{}, 1).
		WillReturnRows(dongLich("pending", from, 25000, tCocID, tHvID))
	mock.ExpectRollback()

	_, err := svc.StartSession(&StartRequest{MachineID: tMayID, MemberID: "mem-khac"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "máy PC-05 đã được đặt trước đến "+from.Add(BookingNoShowGrace).In(utils.VietnamLocation()).Format("15:04"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Lịch khách vãng lai (không có hội viên) chặn mọi hội viên trong giờ giữ.
func TestGiuMayTheoLichDat_LichVangLaiChanMoiNguoi(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectQuery(`SELECT \* FROM "machine_bookings" WHERE .* FOR UPDATE`).
		WillReturnRows(dongLich("pending", time.Now().Add(-time.Minute), 0, nil, nil))

	_, err := giuMayTheoLichDat(db, &model.Machine{ID: tMayID, MachineCode: "PC-05"}, &model.Member{ID: tHvID}, time.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "đã được đặt trước")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Trước giờ bắt đầu thì người khác vẫn ngồi được — máy chưa tới giờ giữ.
func TestGiuMayTheoLichDat_TruocGioBatDauChoNguoiKhac(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectQuery(`SELECT \* FROM "machine_bookings" WHERE .* FOR UPDATE`).
		WillReturnRows(dongLich("pending", time.Now().Add(20*time.Minute), 0, nil, tHvID))

	n, err := giuMayTheoLichDat(db, &model.Machine{ID: tMayID, MachineCode: "PC-05"}, &model.Member{ID: "mem-khac"}, time.Now())
	require.NoError(t, err)
	assert.Nil(t, n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Ô tìm trên trang Đặt chỗ ghi "Tìm theo tên / mã máy" nên phải tra cả hai,
// và phải bỏ dấu.
//
// Bản cũ chỉ tra customer_name và customer_phone bằng ILIKE trần: gõ đúng mã
// máy ra rỗng (dù ô nhập hứa tìm được), còn gõ "Le Thi" không ra "Lê Thị Hồng
// Nhung" vì tên khách luôn có dấu còn nhân viên quầy gõ không dấu.
func TestBookingService_List_TimTheoTenBoDauVaMaMay(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewBookingService(db, NewAuditService(db))

	dieuKien := `WHERE \(unaccent\(customer_name\) ILIKE unaccent\(\$1\) OR customer_phone ILIKE \$2 OR machine_id IN \(SELECT id FROM machines WHERE machine_code ILIKE \$3 AND deleted_at IS NULL\)\)`

	mock.ExpectQuery(`SELECT count\(\*\) FROM "machine_bookings" `+dieuKien).
		WithArgs("%PC-05%", "%PC-05%", "%PC-05%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT \* FROM "machine_bookings" ` + dieuKien).
		WillReturnRows(sqlmock.NewRows([]string{"id", "customer_name"}).
			AddRow(testUUID, "Lê Thị Hồng Nhung"))

	res, err := svc.List(&BookingListRequest{Page: 1, PageSize: 20, Search: "PC-05"})

	require.NoError(t, err)
	assert.Equal(t, int64(1), res.Total)
}
