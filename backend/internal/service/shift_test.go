package service

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
)

const (
	shiftSelectByID     = `SELECT \* FROM "shifts" WHERE id = \$1 ORDER BY "shifts"."id" LIMIT \$2`
	shiftLockByID       = shiftSelectByID + ` FOR UPDATE`
	shiftSelectOpen     = `SELECT \* FROM "shifts" WHERE status = \$1 ORDER BY "shifts"."id" LIMIT \$2`
	shiftUserNameLookup = `SELECT id, username, full_name FROM "users" WHERE id IN \(\$1\)`
	shiftOtherUserID    = "880e8400-e29b-41d4-a716-446655440003"
)

func expectShiftUserName(mock sqlmock.Sqlmock, userID, fullName string) {
	mock.ExpectQuery(shiftUserNameLookup).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "full_name"}).AddRow(userID, "u", fullName))
}

// expectShiftCash khai báo sáu truy vấn của expectedCash theo đúng thứ tự.
func expectShiftCash(mock sqlmock.Sqlmock, payments, topups, combos, in, out, refunds int64) {
	one := func(v int64) *sqlmock.Rows { return sqlmock.NewRows([]string{"coalesce"}).AddRow(v) }
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(amount\), 0\) FROM "payments"`).WillReturnRows(one(payments))
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(amount\), 0\) FROM "member_transactions" WHERE payment_method`).WillReturnRows(one(topups))
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(-amount\), 0\) FROM "member_transactions" WHERE payment_method`).WillReturnRows(one(combos))
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(amount\), 0\) FROM "cash_handovers"`).WillReturnRows(one(in))
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(amount\), 0\) FROM "cash_handovers"`).WillReturnRows(one(out))
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(-amount\), 0\) FROM "member_transactions" WHERE transaction_type = \$1 AND created_at >= \$2 AND created_at <= \$3`).
		WithArgs("refund", anyTime{}, anyTime{}).
		WillReturnRows(one(refunds))
}

func openShiftRow(userID string) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "user_id", "status", "opening_balance", "started_at", "notes"}).
		AddRow(testUUID, userID, "open", int64(500000), testNow, "")
}

func i64(v int64) *int64 { return &v }

func TestShiftService_List(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "shifts"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT \* FROM "shifts" ORDER BY started_at desc LIMIT \$1`).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status", "started_at"}).
			AddRow("s1", testUserID, "open", testNow))

	result, err := svc.List(&pagination.Params{
		Page: 1, PageSize: 20, Sort: "started_at", Order: "desc",
	}, "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Ô tìm kiếm gửi tên nhân viên: phải tìm qua bảng users, không so với cột uuid.
func TestShiftService_List_SearchAndStatus(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))

	where := `WHERE \(user_id IN \(SELECT id FROM users WHERE unaccent\(username\) ILIKE unaccent\(\$1\) OR unaccent\(full_name\) ILIKE unaccent\(\$2\)\)\) AND status = \$3`
	mock.ExpectQuery(`SELECT count\(\*\) FROM "shifts" `+where).
		WithArgs("%Hùng%", "%Hùng%", "closed").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT \* FROM "shifts" `+where+` ORDER BY started_at desc LIMIT \$4`).
		WithArgs("%Hùng%", "%Hùng%", "closed", 20).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	result, err := svc.List(&pagination.Params{Page: 1, PageSize: 20, Search: "Hùng"}, "closed")
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestShiftService_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))

	mock.ExpectQuery(shiftSelectByID).
		WithArgs(testUUID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status"}).AddRow(testUUID, testUserID, "open"))
	expectShiftUserName(mock, testUserID, "Nguyễn Văn A")

	result, err := svc.GetByID(testUUID)
	require.NoError(t, err)
	assert.Equal(t, "open", result.Status)
	assert.Equal(t, "Nguyễn Văn A", result.UserName)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestShiftService_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))

	mock.ExpectQuery(shiftSelectByID).
		WithArgs(testUUID, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.GetByID(testUUID)
	assert.ErrorIs(t, err, ErrShiftNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// "current" hay id gõ sai không được chạm DB — lỗi uuid thô từng lọt ra ngoài.
func TestShiftService_NonUUIDIsNotFoundWithoutQuery(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))
	owner := ShiftActor{UserID: testUserID}

	_, err := svc.GetByID("current")
	assert.ErrorIs(t, err, ErrShiftNotFound)

	_, err = svc.CloseShift("abc", &CloseShiftRequest{ClosingBalance: i64(0)}, owner)
	assert.ErrorIs(t, err, ErrShiftNotFound)

	_, err = svc.Handover("abc", &HandoverRequest{Amount: 1, HandoverType: "cash_in", Reason: "x"}, owner)
	assert.ErrorIs(t, err, ErrShiftNotFound)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestShiftService_OpenShift_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))

	mock.ExpectQuery(shiftSelectOpen).
		WithArgs("open", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "shifts"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()
	expectShiftUserName(mock, testUserID, "Nguyễn Văn A")

	result, err := svc.OpenShift(&OpenShiftRequest{OpeningBalance: 500000}, testUserID)
	require.NoError(t, err)
	assert.Equal(t, "open", result.Status)
	assert.Equal(t, int64(500000), result.OpeningBalance)
	assert.Equal(t, "Nguyễn Văn A", result.UserName)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Một két tiền: ca mở của NGƯỜI KHÁC cũng chặn việc mở ca, và lỗi nêu tên họ.
func TestShiftService_OpenShift_AnotherUsersShiftOpen(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))

	mock.ExpectQuery(shiftSelectOpen).
		WithArgs("open", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status"}).AddRow(testUUID, shiftOtherUserID, "open"))
	expectShiftUserName(mock, shiftOtherUserID, "Trần Thị B")

	_, err := svc.OpenShift(&OpenShiftRequest{}, testUserID)
	require.Error(t, err)
	assert.Equal(t, "đang có ca mở của Trần Thị B — đóng ca đó trước khi mở ca mới", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Hai lần mở ca đồng thời: lần thua cuộc đụng chỉ mục uniq_shift_open.
func TestShiftService_OpenShift_UniqueViolationRace(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))

	mock.ExpectQuery(shiftSelectOpen).
		WithArgs("open", 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "shifts"`).
		WillReturnError(errors.New(`ERROR: duplicate key value violates unique constraint "uniq_shift_open" (SQLSTATE 23505)`))
	mock.ExpectRollback()
	mock.ExpectQuery(shiftSelectOpen).
		WithArgs("open", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status"}).AddRow(testUUID, shiftOtherUserID, "open"))
	expectShiftUserName(mock, shiftOtherUserID, "Trần Thị B")

	_, err := svc.OpenShift(&OpenShiftRequest{}, testUserID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "đang có ca mở của Trần Thị B")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestShiftService_OpenShift_NegativeBalance(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))

	_, err := svc.OpenShift(&OpenShiftRequest{OpeningBalance: -1}, testUserID)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestShiftService_CloseShift_Validation(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))
	owner := ShiftActor{UserID: testUserID}

	_, err := svc.CloseShift(testUUID, &CloseShiftRequest{}, owner)
	assert.EqualError(t, err, "nhập tiền cuối ca")

	_, err = svc.CloseShift(testUUID, &CloseShiftRequest{ClosingBalance: i64(-5)}, owner)
	assert.EqualError(t, err, "tiền cuối ca không được âm")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestShiftService_CloseShift_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(shiftLockByID).
		WithArgs(testUUID, 1).
		WillReturnRows(openShiftRow(testUserID))

	// Chỉ tiền mặt vào/ra két: 200k + 80k + 50k + 0 − 30k − 20k hoàn tiền = 280k.
	expectShiftCash(mock, 200000, 80000, 50000, 0, 30000, 20000)

	mock.ExpectExec(`UPDATE "shifts" SET "closing_balance"=\$1,"discrepancy"=\$2,"ended_at"=\$3,"expected_total"=\$4,"notes"=\$5,"status"=\$6 WHERE id = \$7 AND status = \$8`).
		WithArgs(int64(780000), int64(0), anyTime{}, int64(280000), "", "closed", testUUID, "open").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	expectShiftUserName(mock, testUserID, "Nguyễn Văn A")

	result, err := svc.CloseShift(testUUID, &CloseShiftRequest{ClosingBalance: i64(780000)}, ShiftActor{UserID: testUserID})
	require.NoError(t, err)
	assert.Equal(t, "closed", result.Status)
	assert.Equal(t, "Nguyễn Văn A", result.UserName)
	require.NotNil(t, result.ClosingBalance)
	require.NotNil(t, result.ExpectedTotal)
	require.NotNil(t, result.Discrepancy)
	// Phản hồi dựng từ đúng các giá trị đã ghi.
	assert.Equal(t, int64(780000), *result.ClosingBalance)
	assert.Equal(t, int64(280000), *result.ExpectedTotal)
	assert.Equal(t, int64(0), *result.Discrepancy)
	assert.NotNil(t, result.EndedAt)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Lần đóng thứ hai thua cuộc ở câu UPDATE có điều kiện status = 'open'.
func TestShiftService_CloseShift_ConcurrentSecondClose(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(shiftLockByID).
		WithArgs(testUUID, 1).
		WillReturnRows(openShiftRow(testUserID))
	expectShiftCash(mock, 0, 0, 0, 0, 0, 0)
	mock.ExpectExec(`UPDATE "shifts" SET .* WHERE id = \$7 AND status = \$8`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	_, err := svc.CloseShift(testUUID, &CloseShiftRequest{ClosingBalance: i64(500000)}, ShiftActor{UserID: testUserID})
	assert.EqualError(t, err, "ca đã đóng")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestShiftService_CloseShift_AlreadyClosed(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(shiftLockByID).
		WithArgs(testUUID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status"}).AddRow(testUUID, testUserID, "closed"))
	mock.ExpectRollback()

	_, err := svc.CloseShift(testUUID, &CloseShiftRequest{ClosingBalance: i64(0)}, ShiftActor{UserID: testUserID})
	assert.EqualError(t, err, "ca đã đóng")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestShiftService_CloseShift_NotOwner(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(shiftLockByID).
		WithArgs(testUUID, 1).
		WillReturnRows(openShiftRow(testUserID))
	mock.ExpectRollback()

	_, err := svc.CloseShift(testUUID, &CloseShiftRequest{ClosingBalance: i64(0)}, ShiftActor{UserID: shiftOtherUserID})
	assert.ErrorIs(t, err, ErrShiftForbidden)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Chủ quán (quyền "*") đóng được ca của nhân viên, và ghi chú chỉ được nối khi có.
func TestShiftService_CloseShift_ShopOwnerAppendsNotes(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(shiftLockByID).
		WithArgs(testUUID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status", "opening_balance", "started_at", "notes"}).
			AddRow(testUUID, testUserID, "open", int64(0), testNow, "đầu ca"))
	expectShiftCash(mock, 0, 0, 0, 0, 0, 0)
	mock.ExpectExec(`UPDATE "shifts" SET`).
		WithArgs(int64(0), int64(0), anyTime{}, int64(0), "đầu ca\nquên đóng ca", "closed", testUUID, "open").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	expectShiftUserName(mock, testUserID, "Nguyễn Văn A")

	result, err := svc.CloseShift(testUUID, &CloseShiftRequest{ClosingBalance: i64(0), Notes: "quên đóng ca"},
		ShiftActor{UserID: shiftOtherUserID, IsOwner: true})
	require.NoError(t, err)
	assert.Equal(t, "đầu ca\nquên đóng ca", result.Notes)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestShiftService_Handover_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(shiftLockByID).
		WithArgs(testUUID, 1).
		WillReturnRows(openShiftRow(testUserID))
	// Két: 500k đầu ca + 100k thu − 50k hoàn tiền = 550k, đủ để chi 200k.
	expectShiftCash(mock, 100000, 0, 0, 0, 0, 50000)
	mock.ExpectQuery(`INSERT INTO "cash_handovers"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("h1"))
	mock.ExpectCommit()

	result, err := svc.Handover(testUUID, &HandoverRequest{
		Amount:       200000,
		HandoverType: "cash_out",
		Reason:       "Nộp tiền về két sắt",
	}, ShiftActor{UserID: testUserID})
	require.NoError(t, err)
	assert.Equal(t, "h1", result.ID)
	assert.Equal(t, "cash_out", result.HandoverType)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestShiftService_Handover_CashOutExceedsDrawer(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(shiftLockByID).
		WithArgs(testUUID, 1).
		WillReturnRows(openShiftRow(testUserID))
	// 500k + 100k − 50k hoàn tiền = 550k trong két.
	expectShiftCash(mock, 100000, 0, 0, 0, 0, 50000)
	mock.ExpectRollback()

	_, err := svc.Handover(testUUID, &HandoverRequest{
		Amount: 600000, HandoverType: "cash_out", Reason: "rút",
	}, ShiftActor{UserID: testUserID})
	assert.EqualError(t, err, "không đủ tiền trong két: hiện chỉ có 550000 đ, không chi được 600000 đ")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestShiftService_Handover_ClosedShift(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(shiftLockByID).
		WithArgs(testUUID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status"}).AddRow(testUUID, testUserID, "closed"))
	mock.ExpectRollback()

	_, err := svc.Handover(testUUID, &HandoverRequest{
		Amount: 1000, HandoverType: "cash_in", Reason: "tiền lẻ",
	}, ShiftActor{UserID: testUserID})
	assert.EqualError(t, err, "ca đã đóng — không ghi thu/chi được nữa")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestShiftService_Handover_NotOwner(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(shiftLockByID).
		WithArgs(testUUID, 1).
		WillReturnRows(openShiftRow(testUserID))
	mock.ExpectRollback()

	_, err := svc.Handover(testUUID, &HandoverRequest{
		Amount: 1000, HandoverType: "cash_in", Reason: "tiền lẻ",
	}, ShiftActor{UserID: shiftOtherUserID})
	assert.ErrorIs(t, err, ErrShiftForbidden)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestShiftService_Handover_ReasonRequired(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewShiftService(db, NewAuditService(db))

	_, err := svc.Handover(testUUID, &HandoverRequest{
		Amount: 1000, HandoverType: "cash_in", Reason: "   ",
	}, ShiftActor{UserID: testUserID})
	assert.EqualError(t, err, "nhập lý do thu/chi")
	assert.NoError(t, mock.ExpectationsWereMet())
}
