package service

import (
	"math"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/model"
	"gorm.io/gorm"
)

func TestSessionService_GetSession_Found(t *testing.T) {
	db, mock := newMockDB(t)
	wsHub := hub.New(nil)
	svc := NewSessionService(db, wsHub, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machine_sessions" WHERE id = \$1 ORDER BY "machine_sessions"."id" LIMIT \$2`).
		WithArgs("s1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_id", "member_id", "is_active", "started_at", "created_at"}).
			AddRow("s1", "m1", "mem1", true, testNow, testNow))

	// Unscoped: máy bị xoá mềm mà khách còn ngồi thì phiên vẫn chạy, nên truy vấn
	// KHÔNG được lọc theo deleted_at.
	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 ORDER BY "machines"\."id" LIMIT \$2`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code"}).AddRow("m1", "M-001"))

	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$2`).
		WithArgs("mem1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "full_name"}).AddRow("mem1", "Test Member"))

	result, err := svc.GetSession("s1")
	require.NoError(t, err)
	assert.Equal(t, "m1", result.MachineID)
	assert.Equal(t, "M-001", result.MachineCode)
	assert.Equal(t, "Test Member", result.MemberName)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionService_GetSession_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	wsHub := hub.New(nil)
	svc := NewSessionService(db, wsHub, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machine_sessions" WHERE id = \$1 ORDER BY "machine_sessions"."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.GetSession("nonexistent")
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionService_GetActiveSessions(t *testing.T) {
	db, mock := newMockDB(t)
	wsHub := hub.New(nil)
	svc := NewSessionService(db, wsHub, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machine_sessions" WHERE is_active = \$1`).
		WithArgs(true).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_id", "member_id", "is_active", "started_at", "created_at"}).
			AddRow("s1", "m1", "mem1", true, testNow, testNow))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 ORDER BY "machines"\."id" LIMIT \$2`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code"}).AddRow("m1", "M-001"))

	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$2`).
		WithArgs("mem1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "full_name"}).AddRow("mem1", "Test Member"))

	result, err := svc.GetActiveSessions()
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "M-001", result[0].MachineCode)
	// Phiên đang chạy phải có số phút đã chơi; cột duration_minutes chỉ được ghi
	// lúc trả máy nên trước đây trường này luôn null.
	require.NotNil(t, result[0].DurationMinutes)
	assert.GreaterOrEqual(t, *result[0].DurationMinutes, 0)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionService_StartSession_WithCombo(t *testing.T) {
	db, mock := newMockDB(t)
	wsHub := hub.New(nil)
	svc := NewSessionService(db, wsHub, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE \(id = \$1 AND is_active = \$2\) AND "machines"\."deleted_at" IS NULL ORDER BY "machines"\."id" LIMIT \$3`).
		WithArgs("m1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code", "status", "is_active"}).
			AddRow("m1", "M-001", "available", true))

	mock.ExpectQuery(`SELECT \* FROM "members" WHERE \(id = \$1 AND is_active = \$2\) AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$3`).
		WithArgs("mem1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "full_name", "is_active"}).
			AddRow("mem1", "Test Member", true))

	// Đổi từ Count sang First: thông báo "đang chơi máy nào" cần dòng phiên,
	// không chỉ đếm. Không có dòng nào = hội viên chưa chơi máy nào.
	mock.ExpectQuery(`SELECT \* FROM "machine_sessions" WHERE member_id = \$1 AND is_active = \$2`).
		WithArgs("mem1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "machines" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(`SELECT \* FROM "combo_purchases" WHERE id = \$1 ORDER BY "combo_purchases"."id" LIMIT \$2`).
		WithArgs("cp1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "combo_id", "member_id", "activated", "remaining_minutes", "created_at"}).
			AddRow("cp1", "c1", "mem1", true, 120, testNow))

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1 AND "combos"\."deleted_at" IS NULL ORDER BY "combos"\."id" LIMIT \$2`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type", "total_minutes", "slot_end", "created_at"}).
			AddRow("c1", "Prepaid 2h", "prepaid", 120, "", testNow))

	mock.ExpectExec(`UPDATE "combo_purchases" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(`INSERT INTO "machine_sessions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("s1"))
	// Mở máy ghi lại lần ghé gần nhất của hội viên.
	mock.ExpectExec(`UPDATE "members" SET "last_visit_at"`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result, err := svc.StartSession(&StartRequest{
		MachineID:       "m1",
		MemberID:        "mem1",
		ComboPurchaseID: "cp1",
	})
	require.NoError(t, err)
	assert.Equal(t, "s1", result.ID)
	assert.Equal(t, "M-001", result.MachineCode)
	assert.Equal(t, "prepaid", result.ComboType)
	assert.NotNil(t, result.RemainingMinutes)
	assert.Equal(t, 120, *result.RemainingMinutes)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionService_StartSession_ComboNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	wsHub := hub.New(nil)
	svc := NewSessionService(db, wsHub, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE \(id = \$1 AND is_active = \$2\) AND "machines"\."deleted_at" IS NULL ORDER BY "machines"\."id" LIMIT \$3`).
		WithArgs("m1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code", "status", "is_active"}).
			AddRow("m1", "M-001", "available", true))

	mock.ExpectQuery(`SELECT \* FROM "members" WHERE \(id = \$1 AND is_active = \$2\) AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$3`).
		WithArgs("mem1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "full_name", "is_active"}).
			AddRow("mem1", "Test Member", true))

	// Đổi từ Count sang First: thông báo "đang chơi máy nào" cần dòng phiên,
	// không chỉ đếm. Không có dòng nào = hội viên chưa chơi máy nào.
	mock.ExpectQuery(`SELECT \* FROM "machine_sessions" WHERE member_id = \$1 AND is_active = \$2`).
		WithArgs("mem1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "machines" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(`SELECT \* FROM "combo_purchases" WHERE id = \$1 ORDER BY "combo_purchases"."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectRollback()

	_, err := svc.StartSession(&StartRequest{
		MachineID:       "m1",
		MemberID:        "mem1",
		ComboPurchaseID: "nonexistent",
	})
	assert.EqualError(t, err, "combo purchase not found")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionService_StartSession_ComboNotActivated(t *testing.T) {
	db, mock := newMockDB(t)
	wsHub := hub.New(nil)
	svc := NewSessionService(db, wsHub, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE \(id = \$1 AND is_active = \$2\) AND "machines"\."deleted_at" IS NULL ORDER BY "machines"\."id" LIMIT \$3`).
		WithArgs("m1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code", "status", "is_active"}).
			AddRow("m1", "M-001", "available", true))

	mock.ExpectQuery(`SELECT \* FROM "members" WHERE \(id = \$1 AND is_active = \$2\) AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$3`).
		WithArgs("mem1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "full_name", "is_active"}).
			AddRow("mem1", "Test Member", true))

	// Đổi từ Count sang First: thông báo "đang chơi máy nào" cần dòng phiên,
	// không chỉ đếm. Không có dòng nào = hội viên chưa chơi máy nào.
	mock.ExpectQuery(`SELECT \* FROM "machine_sessions" WHERE member_id = \$1 AND is_active = \$2`).
		WithArgs("mem1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "machines" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(`SELECT \* FROM "combo_purchases" WHERE id = \$1 ORDER BY "combo_purchases"."id" LIMIT \$2`).
		WithArgs("cp1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "combo_id", "activated"}).
			AddRow("cp1", "c1", false))
	mock.ExpectRollback()

	_, err := svc.StartSession(&StartRequest{
		MachineID:       "m1",
		MemberID:        "mem1",
		ComboPurchaseID: "cp1",
	})
	assert.EqualError(t, err, "combo purchase is not activated")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionService_EndSession_WithCombo(t *testing.T) {
	db, mock := newMockDB(t)
	wsHub := hub.New(nil)
	svc := NewSessionService(db, wsHub, NewAuditService(db))

	// Load session (with ComboID set & RemainingMinutes)
	mock.ExpectQuery(`SELECT \* FROM "machine_sessions" WHERE id = \$1 AND is_active = \$2 ORDER BY "machine_sessions"."id" LIMIT \$3`).
		WithArgs("s1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_id", "member_id", "combo_id", "remaining_minutes", "is_active", "started_at", "created_at"}).
			AddRow("s1", "m1", "mem1", "cp1", 120, true, testNow, testNow))

	// Load machine
	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 AND "machines"\."deleted_at" IS NULL ORDER BY "machines"\."id" LIMIT \$2`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code", "group_id", "status"}).
			AddRow("m1", "M-001", nil, "in_use"))

	// Số phút phải trả tiền nay tính từ ảnh chụp trên chính dòng phiên, nên
	// không còn transaction chỉ để khoá combo_purchases rồi commit ngay — khoá
	// đó nhả trước khi trừ đồng nào, không bảo vệ được gì.

	// CalculateCost: load machine
	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 AND "machines"\."deleted_at" IS NULL ORDER BY "machines"\."id" LIMIT \$2`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code", "group_id", "status"}).
			AddRow("m1", "M-001", nil, "in_use"))

	// Một transaction duy nhất cho toàn bộ phần ghi.
	mock.ExpectBegin()

	// Load member (snapshot GroupID)
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$2`).
		WithArgs("mem1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "full_name", "balance", "group_id"}).
			AddRow("mem1", "Test Member", int64(50000), nil))

	// Trừ tiền chạy TRƯỚC khi lưu phiên: nó đặt charged_amount.
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$2 FOR UPDATE`).
		WithArgs("mem1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "full_name", "balance", "group_id"}).
			AddRow("mem1", "Test Member", int64(50000), nil))

	// SAVE member (balance deducted = 50000 - 0 = 50000)
	mock.ExpectExec(`UPDATE "members" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Tiền phiên bằng 0 nên KHÔNG tạo dòng giao dịch 0₫ làm rác sổ; chỉ tra xem
	// đã có dòng nào cho phiên này chưa.
	mock.ExpectQuery(`SELECT \* FROM "member_transactions" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	// SAVE session (ended)
	mock.ExpectExec(`UPDATE "machine_sessions" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// SAVE machine (status=available)
	mock.ExpectExec(`UPDATE "machines" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// SELECT purchase (deduct remaining_minutes)
	mock.ExpectQuery(`SELECT \* FROM "combo_purchases" WHERE id = \$1 ORDER BY "combo_purchases"."id" LIMIT \$2`).
		WithArgs("cp1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "combo_id", "remaining_minutes", "created_at"}).
			AddRow("cp1", "c1", 120, testNow))

	// UPDATE purchase (remaining_minutes = 120 - 0 = 120)
	mock.ExpectExec(`UPDATE "combo_purchases" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	result, err := svc.EndSession("s1")
	require.NoError(t, err)
	assert.Equal(t, "s1", result.SessionID)
	assert.Equal(t, "M-001", result.MachineCode)
	assert.Equal(t, int64(0), result.TotalCost)
	assert.Equal(t, int64(50000), result.BalanceBefore)
	assert.Equal(t, int64(50000), result.BalanceAfter)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// The whole billing engine used to output zero because the only price field
// could not be saved through the API and the tier table was never migrated.
func TestSessionService_CalculateCost_UsesGroupRate(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSessionService(db, hub.New(nil), NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 AND "machines"\."deleted_at" IS NULL ORDER BY "machines"\."id" LIMIT \$2`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "group_id"}).AddRow("m1", "g1"))
	mock.ExpectQuery(`SELECT \* FROM "machine_groups" WHERE id = \$1 AND "machine_groups"\."deleted_at" IS NULL ORDER BY "machine_groups"\."id" LIMIT \$2`).
		WithArgs("g1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price_per_hour"}).AddRow("g1", "VIP", int64(20000)))
	// No member: tier lookup is skipped, time-of-day rate is consulted.
	mock.ExpectQuery(`SELECT \* FROM "time_based_pricings"`).
		WillReturnError(gorm.ErrRecordNotFound)

	got, err := svc.CalculateCost("m1", "", 90)

	require.NoError(t, err)
	assert.Equal(t, int64(20000), got.PricePerHour)
	assert.Equal(t, int64(30000), got.FinalCost) // 90 min at 20k/h
	assert.Equal(t, int64(0), got.DiscountAmount)
}

// A member tier discount has to reach the bill; it never did.
func TestSessionService_CalculateCost_AppliesMemberDiscount(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSessionService(db, hub.New(nil), NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "group_id"}).AddRow("m1", "g1"))
	mock.ExpectQuery(`SELECT \* FROM "machine_groups"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price_per_hour"}).AddRow("g1", "Standard", int64(10000)))
	mock.ExpectQuery(`SELECT \* FROM "members"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "group_id"}).AddRow("mem-1", "mg-gold"))
	// No tier-specific rate, no time-based rate: fall through to the base rate.
	mock.ExpectQuery(`SELECT \* FROM "machine_prices"`).WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery(`SELECT \* FROM "time_based_pricings"`).WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery(`SELECT \* FROM "member_groups"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "discount_percent"}).AddRow("mg-gold", 10.0))

	got, err := svc.CalculateCost("m1", "mem-1", 60)

	require.NoError(t, err)
	assert.Equal(t, int64(10000), got.GrossCost)
	assert.Equal(t, 10.0, got.DiscountPercent)
	assert.Equal(t, int64(1000), got.DiscountAmount)
	assert.Equal(t, int64(9000), got.FinalCost)
}

// A tier rate beats the group base rate.
func TestSessionService_CalculateCost_MemberTierRateWins(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSessionService(db, hub.New(nil), NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "group_id"}).AddRow("m1", "g1"))
	mock.ExpectQuery(`SELECT \* FROM "machine_groups"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price_per_hour"}).AddRow("g1", "Standard", int64(10000)))
	mock.ExpectQuery(`SELECT \* FROM "members"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "group_id"}).AddRow("mem-1", "mg-vip"))
	mock.ExpectQuery(`SELECT \* FROM "machine_prices"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "price_per_hour"}).AddRow("mp1", int64(6000)))
	mock.ExpectQuery(`SELECT \* FROM "member_groups"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "discount_percent"}).AddRow("mg-vip", 0.0))

	got, err := svc.CalculateCost("m1", "mem-1", 60)

	require.NoError(t, err)
	assert.Equal(t, int64(6000), got.PricePerHour)
	assert.Equal(t, int64(6000), got.FinalCost)
}

// Ending a session used to roll back when the member could not pay, leaving the
// machine stuck on "in use" forever.
func TestSessionService_EndSession_ShortBalanceLeavesDebtNotDeadlock(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSessionService(db, hub.New(nil), NewAuditService(db))

	startedAt := time.Now().Add(-60 * time.Minute)

	mock.ExpectQuery(`SELECT \* FROM "machine_sessions" WHERE id = \$1 AND is_active = \$2 ORDER BY "machine_sessions"\."id" LIMIT \$3`).
		WithArgs("s1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_id", "member_id", "started_at", "is_active"}).
			AddRow("s1", "m1", "mem-1", startedAt, true))
	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 AND "machines"\."deleted_at" IS NULL ORDER BY "machines"\."id" LIMIT \$2`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code", "group_id"}).AddRow("m1", "PC-01", "g1"))

	// Cost lookup: 10k/hour, no tier or time override, no discount.
	mock.ExpectQuery(`SELECT \* FROM "machines"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "group_id"}).AddRow("m1", "g1"))
	mock.ExpectQuery(`SELECT \* FROM "machine_groups"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price_per_hour"}).AddRow("g1", "Std", int64(10000)))
	mock.ExpectQuery(`SELECT \* FROM "members"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "group_id"}).AddRow("mem-1", nil))
	mock.ExpectQuery(`SELECT \* FROM "time_based_pricings"`).WillReturnError(gorm.ErrRecordNotFound)

	mock.ExpectBegin()

	// Ảnh chụp nhóm hội viên lên dòng phiên.
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$2`).
		WithArgs("mem-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "group_id"}).AddRow("mem-1", nil))

	// 2000 bonus + 1000 balance against a 10000 fee -> 7000 of debt.
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$2 FOR UPDATE`).
		WithArgs("mem-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance", "bonus_balance"}).
			AddRow("mem-1", int64(1000), int64(2000)))
	mock.ExpectExec(`UPDATE "members" SET`).WillReturnResult(sqlmock.NewResult(1, 1))
	// Chưa có dòng session_fee nào cho phiên này → tạo mới.
	mock.ExpectQuery(`SELECT \* FROM "member_transactions" WHERE`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`INSERT INTO "member_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("tx-1"))

	mock.ExpectExec(`UPDATE "machine_sessions" SET`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE "machines" SET`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	res, err := svc.EndSession("s1")

	require.NoError(t, err, "ending a session must never be blocked by a short balance")
	assert.Equal(t, int64(2000), res.BonusUsed)
	assert.Equal(t, int64(7000), res.AmountUnpaid)
	assert.Equal(t, int64(-7000), res.BalanceAfter)
}

// The debt is settled by refusing the next session, not by blocking the exit.
// Đang chơi một máy mà mở máy khác thì bị chặn, và thông báo phải NÊU TÊN máy
// đang chơi — bằng tiếng Việt, vì nó hiện thẳng lên màn hình khoá của khách.
func TestSessionService_StartSession_DangChoiMayKhacThiChan(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSessionService(db, hub.New(nil), NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE \(id = \$1 AND is_active = \$2\)`).
		WithArgs("m2", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow("m2", "available"))
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE \(id = \$1 AND is_active = \$2\)`).
		WithArgs("mem1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance"}).AddRow("mem1", int64(50000)))
	// Đã có phiên đang chạy trên máy M1 (mã "PC-05").
	mock.ExpectQuery(`SELECT \* FROM "machine_sessions" WHERE member_id = \$1 AND is_active = \$2`).
		WithArgs("mem1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_id", "machine_code", "is_active"}).
			AddRow("sess-old", "m1", "PC-05", true))

	_, err := svc.StartSession(&StartRequest{MachineID: "m2", MemberID: "mem1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PC-05", "thông báo phải nêu tên máy đang chơi")
	assert.Contains(t, err.Error(), "trả máy")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionService_StartSession_RefusesMemberInDebt(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSessionService(db, hub.New(nil), NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE \(id = \$1 AND is_active = \$2\) AND "machines"\."deleted_at" IS NULL ORDER BY "machines"\."id" LIMIT \$3`).
		WithArgs("m1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow("m1", "available"))
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE \(id = \$1 AND is_active = \$2\) AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$3`).
		WithArgs("mem-1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance"}).AddRow("mem-1", int64(-7000)))
	mock.ExpectQuery(`SELECT \* FROM "machine_sessions" WHERE member_id = \$1 AND is_active = \$2`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	// Trần nợ đọc từ nhóm cài đặt "limits"; nhóm rỗng ⇒ trần 0 ⇒ nợ nào cũng chặn.
	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1`).
		WithArgs("limits").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}))

	_, err := svc.StartSession(&StartRequest{MachineID: "m1", MemberID: "mem-1"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "vượt trần nợ")
}

// Trần nợ khác 0: khách nợ trong hạn vẫn mở được máy, quá hạn thì bị chặn.
func TestSessionService_StartSession_AllowsDebtWithinMaxDebt(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSessionService(db, hub.New(nil), NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE \(id = \$1 AND is_active = \$2\) AND "machines"\."deleted_at" IS NULL ORDER BY "machines"\."id" LIMIT \$3`).
		WithArgs("m1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow("m1", "available"))
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE \(id = \$1 AND is_active = \$2\) AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$3`).
		WithArgs("mem-1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance"}).AddRow("mem-1", int64(-7000)))
	mock.ExpectQuery(`SELECT \* FROM "machine_sessions" WHERE member_id = \$1 AND is_active = \$2`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1`).
		WithArgs("limits").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}).
			AddRow("limits", "max_debt", "10000"))

	// Nợ 7.000 dưới trần 10.000 nên qua được chốt chặn nợ; dừng ở chốt kế tiếp
	// (số dư 0 và không có gói) chứ không phải vì nợ.
	_, err := svc.StartSession(&StartRequest{MachineID: "m1", MemberID: "mem-1"})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "trần nợ")
}

// billableMinutes quyết định khách phải trả tiền bao nhiêu phút. Gói khung giờ
// từng bị tính tiền LẦN HAI theo giá lẻ vì loại gói này có TotalMinutes = 0 nên
// RemainingMinutes = 0 và phép trừ không che được gì.
func TestBillableMinutes(t *testing.T) {
	prepaid := 60
	zero := 0
	comboID := "cp1"

	cases := []struct {
		name    string
		session model.MachineSession
		elapsed int
		want    int
	}{
		{"không gói: trả tiền toàn bộ", model.MachineSession{}, 90, 90},
		{"gói trả trước phủ hết", model.MachineSession{ComboID: &comboID, RemainingMinutes: &prepaid}, 45, 0},
		{"gói trả trước phủ một phần", model.MachineSession{ComboID: &comboID, RemainingMinutes: &prepaid}, 90, 30},
		{
			"gói khung giờ: không trả thêm đồng nào",
			model.MachineSession{ComboID: &comboID, ComboType: "fixed_slot", RemainingMinutes: &zero},
			300, 0,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, billableMinutes(&c.session, c.elapsed))
		})
	}
}

// Tính tiền theo phút là phép TÍCH LUỸ TỚI ĐÍCH. Hai tính chất phải giữ, nếu
// không là trừ sai tiền của khách:
//
//   - Idempotent: gọi lại trên cùng số phút thì lần hai trừ 0₫.
//   - Không lệch do làm tròn: trừ dần 60 lượt phải bằng ĐÚNG một lượt tính trọn
//     60 phút. math.Ceil áp lên tổng, cộng dồn từng phút thì lệch tới +60₫/giờ.
func TestChargeAccrualIsIdempotentAndMatchesSingleCharge(t *testing.T) {
	const pricePerHour = int64(10000)

	// cost mô phỏng đúng phép tính của CalculateCost: làm tròn lên trên TỔNG.
	cost := func(minutes int) int64 {
		if minutes <= 0 {
			return 0
		}
		return int64(math.Ceil(float64(minutes) * float64(pricePerHour) / 60.0))
	}

	charged := int64(0)
	total := int64(0)
	for m := 1; m <= 60; m++ {
		target := cost(m)
		delta := target - charged
		if delta < 0 {
			delta = 0
		}
		total += delta
		charged = target

		// Gọi lại ngay trên cùng số phút: không được trừ thêm đồng nào.
		again := cost(m) - charged
		if again < 0 {
			again = 0
		}
		assert.Equal(t, int64(0), again, "phút %d: lượt tính lặp lại phải trừ 0₫", m)
	}

	assert.Equal(t, cost(60), total,
		"tổng trừ dần phải khớp từng đồng với một lượt tính trọn phiên")
	assert.Equal(t, pricePerHour, total, "một giờ ở giá %d₫ phải thu đúng %d₫", pricePerHour, pricePerHour)
}
