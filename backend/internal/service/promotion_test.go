package service

import (
	"database/sql/driver"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/utils"
)

func TestPromotionService_List(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPromotionService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "promotions" WHERE "promotions"\."deleted_at" IS NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT \* FROM "promotions" WHERE "promotions"\."deleted_at" IS NULL ORDER BY created_at desc LIMIT \$1`).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "created_at"}).
			AddRow("p1", "Happy Hour", testNow))

	mock.ExpectQuery(`SELECT \* FROM "promotion_conditions" WHERE promotion_id = \$1`).
		WithArgs("p1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "promotion_id", "condition_key", "condition_value"}))

	mock.ExpectQuery(`SELECT \* FROM "promotion_rewards" WHERE promotion_id = \$1`).
		WithArgs("p1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "promotion_id", "reward_type", "reward_value"}))

	result, err := svc.List(&PromotionListRequest{Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPromotionService_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPromotionService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "promotions" WHERE id = \$1 AND "promotions"\."deleted_at" IS NULL ORDER BY "promotions"."id" LIMIT \$2`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "created_at"}).
			AddRow("p1", "Happy Hour", testNow))

	mock.ExpectQuery(`SELECT \* FROM "promotion_conditions" WHERE promotion_id = \$1`).
		WithArgs("p1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "promotion_id", "condition_key", "condition_value"}))

	mock.ExpectQuery(`SELECT \* FROM "promotion_rewards" WHERE promotion_id = \$1`).
		WithArgs("p1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "promotion_id", "reward_type", "reward_value"}))

	result, err := svc.GetByID("p1")
	require.NoError(t, err)
	assert.Equal(t, "Happy Hour", result.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPromotionService_Create(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPromotionService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "promotions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectQuery(`INSERT INTO "promotion_rewards"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("rw-1"))
	mock.ExpectCommit()

	result, err := svc.Create(&CreatePromotionRequest{
		Name:     "Happy Hour",
		Type:     "percentage",
		IsActive: true,
		Rewards:  []CreatePromotionReward{{RewardType: RewardDiscountPercent, RewardValue: json.RawMessage(`{"percent":10}`)}},
	})
	require.NoError(t, err)
	assert.Equal(t, "Happy Hour", result.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPromotionService_Delete_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPromotionService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "promotions" WHERE id = \$1 AND "promotions"\."deleted_at" IS NULL ORDER BY "promotions"."id" LIMIT \$2`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("p1"))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "orders"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectBegin()
	// Điều kiện và phần thưởng là con sở hữu, xoá cứng theo.
	mock.ExpectExec(`DELETE FROM "promotion_conditions"`).
		WillReturnResult(sqlmock.NewResult(1, 0))
	mock.ExpectExec(`DELETE FROM "promotion_rewards"`).
		WillReturnResult(sqlmock.NewResult(1, 0))
	mock.ExpectExec(`UPDATE "promotions" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.Delete("p1")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPromotionService_GetLuckySpinRewards(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPromotionService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "lucky_spin_rewards" WHERE is_active = \$1 ORDER BY created_at`).
		WithArgs(true).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "reward_type", "reward_value", "probability", "max_per_day", "is_active"}).
			AddRow("r1", "100 Bonus", "bonus_points", `{"amount":100}`, 0.5, 3, true).
			AddRow("r2", "50 VND", "balance", `{"amount":50}`, 0.3, 3, true))

	result, err := svc.GetLuckySpinRewards(false)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	// Amount phải được bóc sẵn khỏi jsonb — giao diện đọc thẳng reward_value là
	// cách chắc chắn cho ra ô trống.
	assert.Equal(t, int64(100), result[0].Amount)
	assert.Equal(t, int64(50), result[1].Amount)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// weightedSelect KHÔNG được chuẩn hoá theo tổng xác suất: phần thiếu so với 1
// là tỉ lệ quay trượt mà quán cố ý để lại.
func TestWeightedSelect_LeavesRoomToLose(t *testing.T) {
	rewards := []model.LuckySpinReward{
		{ID: "r1", Name: "iPhone", Probability: 0.01},
	}
	wins := 0
	for i := 0; i < 2000; i++ {
		if weightedSelect(rewards) != nil {
			wins++
		}
	}
	// 1% trên 2000 lượt ⇒ kỳ vọng 20. Ngưỡng 200 rộng rãi nhưng vẫn bắt được
	// lỗi cũ, vốn cho ra đúng 2000.
	assert.Less(t, wins, 200, "chuẩn hoá xác suất khiến lượt nào cũng trúng")

	// Tổng bằng 1 thì không bao giờ trượt.
	full := []model.LuckySpinReward{
		{ID: "a", Probability: 0.5},
		{ID: "b", Probability: 0.5},
	}
	for i := 0; i < 200; i++ {
		assert.NotNil(t, weightedSelect(full))
	}
}

// H1: tạo khuyến mãi ở trạng thái tắt phải lưu đúng false. GORM thay false bằng
// `default:true` khi INSERT, nên Create phải ghi lại false trong cùng giao dịch.
func TestPromotionService_Create_InactiveStaysInactive(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPromotionService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "promotions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectExec(`UPDATE "promotions" SET "is_active"=\$1 WHERE "promotions"\."deleted_at" IS NULL AND "id" = \$2`).
		WithArgs(false, testUUID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`INSERT INTO "promotion_rewards"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("rw-1"))
	mock.ExpectCommit()

	result, err := svc.Create(&CreatePromotionRequest{
		Name:     "Tạm tắt",
		Type:     "fixed",
		IsActive: false,
		Rewards:  []CreatePromotionReward{{RewardType: RewardDiscountAmount, RewardValue: json.RawMessage(`{"amount":20000}`)}},
	})
	require.NoError(t, err)
	assert.False(t, result.IsActive)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// M5: dữ liệu vô nghĩa bị chặn trước khi chạm DB, kèm câu báo lỗi tiếng Việt.
func TestPromotionService_Create_Validation(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPromotionService(db, NewAuditService(db))

	pct := func(v string) []CreatePromotionReward {
		return []CreatePromotionReward{{RewardType: RewardDiscountPercent, RewardValue: json.RawMessage(v)}}
	}
	ok := pct(`{"percent":10}`)
	tests := []struct {
		name string
		req  CreatePromotionRequest
		want string
	}{
		{"không có phần thưởng", CreatePromotionRequest{Name: "a", Type: "percentage"}, "ít nhất một phần thưởng"},
		{"loại lạ", CreatePromotionRequest{Name: "a", Type: "giam_gia", Rewards: ok}, "loại khuyến mãi"},
		{"ưu tiên âm", CreatePromotionRequest{Name: "a", Type: "percentage", Priority: -1, Rewards: ok}, "ưu tiên"},
		{"phần trăm 0", CreatePromotionRequest{Name: "a", Type: "percentage", Rewards: pct(`{"percent":0}`)}, "phần trăm giảm"},
		{"phần trăm quá 100", CreatePromotionRequest{Name: "a", Type: "percentage", Rewards: pct(`{"percent":150}`)}, "phần trăm giảm"},
		{"phần trăm gõ số trần", CreatePromotionRequest{Name: "a", Type: "percentage", Rewards: pct(`10`)}, "percent"},
		{"giảm tối đa âm", CreatePromotionRequest{Name: "a", Type: "percentage", Rewards: pct(`{"percent":10,"max_discount":-5}`)}, "max_discount"},
		{"giảm tiền bằng 0", CreatePromotionRequest{Name: "a", Type: "fixed", Rewards: []CreatePromotionReward{
			{RewardType: RewardDiscountAmount, RewardValue: json.RawMessage(`{"amount":0}`)}}}, "số tiền giảm"},
		{"tiền tối thiểu âm", CreatePromotionRequest{Name: "a", Type: "percentage", Rewards: ok, Conditions: []CreatePromotionCondition{
			{ConditionKey: CondMinAmount, ConditionValue: json.RawMessage(`-1`)}}}, "min_amount"},
		{"thứ ngoài 0-6", CreatePromotionRequest{Name: "a", Type: "percentage", Rewards: ok, Conditions: []CreatePromotionCondition{
			{ConditionKey: CondDayOfWeek, ConditionValue: json.RawMessage(`[7]`)}}}, "day_of_week"},
		{"khung giờ sai dạng", CreatePromotionRequest{Name: "a", Type: "percentage", Rewards: ok, Conditions: []CreatePromotionCondition{
			{ConditionKey: CondTimeRange, ConditionValue: json.RawMessage(`{"from":"18h","to":"22:00"}`)}}}, "time_range"},
		{"từ ngày sau đến ngày", CreatePromotionRequest{Name: "a", Type: "percentage", Rewards: ok,
			ValidFrom: strPtr("2026-10-10T00:00:00+07:00"), ValidTo: strPtr("2026-10-01T00:00:00+07:00")}, "ngày bắt đầu"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Create(&tc.req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}

// L1: null phải xoá được mốc ngày; bỏ trống trường thì giữ nguyên.
func TestPromotionService_Update_ClearsValidTo(t *testing.T) {
	var req UpdatePromotionRequest
	require.NoError(t, json.Unmarshal([]byte(`{"valid_to": null}`), &req))
	assert.Nil(t, req.ValidFrom, "trường không gửi phải phân biệt được với null")
	assert.Equal(t, "null", string(req.ValidTo))

	db, mock := newMockDB(t)
	svc := NewPromotionService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "promotions" WHERE id = \$1`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type", "valid_to"}).
			AddRow("p1", "Hè", "percentage", testNow))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "promotions" SET "valid_to"=\$1 WHERE "promotions"\."deleted_at" IS NULL AND "id" = \$2`).
		WithArgs(nil, "p1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	_, err := svc.Update("p1", &req)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Sửa một đầu của khoảng ngày vẫn phải so với đầu còn lại đang lưu.
func TestPromotionService_Update_RejectsInvertedRange(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPromotionService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "promotions" WHERE id = \$1`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type", "valid_to"}).
			AddRow("p1", "Hè", "percentage", testNow))

	_, err := svc.Update("p1", &UpdatePromotionRequest{ValidFrom: json.RawMessage(`"2030-01-01T00:00:00Z"`)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ngày bắt đầu")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// H2: ô thưởng tạo ở trạng thái tắt phải lưu là tắt.
func TestPromotionService_CreateLuckySpinReward_InactiveStaysInactive(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPromotionService(db, NewAuditService(db))

	off := false
	// Ô tắt không cần kiểm tổng xác suất, nên không có SELECT nào trước INSERT.
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "lucky_spin_rewards"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("r1"))
	mock.ExpectExec(`UPDATE "lucky_spin_rewards" SET "is_active"=\$1 WHERE "id" = \$2`).
		WithArgs(false, "r1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	res, err := svc.CreateLuckySpinReward(&LuckySpinRewardRequest{
		Name: "Tạm tắt", RewardType: "balance", Amount: 5000, Probability: 0.9, IsActive: &off,
	})
	require.NoError(t, err)
	assert.False(t, res.IsActive)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// H2: bật lại một ô đang tắt phải kiểm lại tổng xác suất.
func TestPromotionService_UpdateLuckySpinReward_ActivatingRechecksSum(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPromotionService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "lucky_spin_rewards" WHERE id = \$1`).
		WithArgs("r1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "probability", "is_active"}).AddRow("r1", 0.5, false))
	mock.ExpectQuery(`SELECT \* FROM "lucky_spin_rewards" WHERE is_active = \$1 AND id <> \$2`).
		WithArgs(true, "r1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "probability", "is_active"}).AddRow("r2", 0.6, true))

	on := true
	_, err := svc.UpdateLuckySpinReward("r1", &LuckySpinRewardRequest{
		Name: "x", RewardType: "balance", Amount: 1000, Probability: 0.5, IsActive: &on,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vượt quá 100%")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// L4: decimal(5,4) làm tròn dưới 0.0001 về 0 — phải chặn từ đầu.
func TestPromotionService_LuckySpinReward_ProbabilityTooSmall(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPromotionService(db, NewAuditService(db))

	_, err := svc.CreateLuckySpinReward(&LuckySpinRewardRequest{
		Name: "x", RewardType: "balance", Amount: 1000, Probability: 0.00005,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "0,01%")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// vnMidnight khớp đúng 0 giờ hôm nay theo giờ Việt Nam (M3).
type vnMidnight struct{}

func (vnMidnight) Match(v driver.Value) bool {
	tm, ok := v.(time.Time)
	return ok && tm.Equal(utils.StartOfDay(time.Now()))
}

const spinMemberID = "11111111-2222-3333-4444-555555555555"

// H3 + M3 + L2: lượt quay chạy trong một giao dịch, khoá dòng hội viên, đếm
// lượt theo ngày giờ Việt Nam ngay dưới khoá, và cộng tiền bằng biểu thức
// (balance + x) với số dư trước/sau lấy từ dòng đã khoá.
func TestPromotionService_Spin_LocksMemberAndCreditsAtomically(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPromotionService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$2 FOR UPDATE`).
		WithArgs(spinMemberID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance", "bonus_balance", "is_active"}).
			AddRow(spinMemberID, int64(30000), int64(0), true))
	mock.ExpectQuery(`SELECT \* FROM "lucky_spin_rewards" WHERE is_active = \$1`).
		WithArgs(true).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "reward_type", "reward_value", "probability", "max_per_day", "is_active"}).
			AddRow("r1", "10k", "balance", `{"amount": 10000}`, 1.0, 2, true))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "lucky_spin_logs" WHERE member_id = \$1 AND spun_at >= \$2`).
		WithArgs(spinMemberID, vnMidnight{}).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`INSERT INTO "lucky_spin_logs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("log-1"))
	mock.ExpectQuery(`INSERT INTO "member_transactions" \("member_id","transaction_type","amount","balance_before","balance_after"`).
		WithArgs(spinMemberID, "lucky_spin_balance", int64(10000), int64(30000), int64(40000),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("tx-1"))
	mock.ExpectExec(`UPDATE "members" SET "balance"=balance \+ \$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs(int64(10000), anyTime{}, spinMemberID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	res, err := svc.Spin(&SpinRequest{MemberID: spinMemberID})
	require.NoError(t, err)
	assert.True(t, res.IsWin)
	require.NotNil(t, res.Reward)
	assert.Equal(t, int64(10000), res.Reward.Amount, "L2: amount phải được điền")
	assert.Equal(t, 2, res.DailySpins)
	assert.Equal(t, 2, res.MaxPerDay)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Hết lượt: đếm dưới khoá thấy đủ lượt thì rollback, không ghi gì.
func TestPromotionService_Spin_DailyLimitReached(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPromotionService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "members" .* FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "is_active"}).AddRow(spinMemberID, true))
	mock.ExpectQuery(`SELECT \* FROM "lucky_spin_rewards"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "reward_type", "reward_value", "probability", "max_per_day", "is_active"}).
			AddRow("r1", "balance", `{"amount": 1}`, 1.0, 0, true))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "lucky_spin_logs"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectRollback()

	_, err := svc.Spin(&SpinRequest{MemberID: spinMemberID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hết lượt")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// M2: member_id không phải UUID bị chặn trước DB; hội viên không tồn tại trả
// lỗi riêng (handler đổi thành 404) và không để lại dòng log mồ côi.
func TestPromotionService_Spin_RejectsBadMember(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPromotionService(db, NewAuditService(db))

	_, err := svc.Spin(&SpinRequest{MemberID: "khong-phai-uuid"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mã hội viên không hợp lệ")

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "members" .* FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectRollback()

	_, err = svc.Spin(&SpinRequest{MemberID: spinMemberID})
	assert.ErrorIs(t, err, ErrSpinMemberNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// M4: quay thử chỉ đọc danh sách ô, không ghi gì.
func TestPromotionService_SimulateSpin_WritesNothing(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPromotionService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "lucky_spin_rewards" WHERE is_active = \$1`).
		WithArgs(true).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "reward_type", "reward_value", "probability", "max_per_day", "is_active"}).
			AddRow("r1", "10k", "balance", `{"amount": 10000}`, 1.0, 3, true))

	res, err := svc.SimulateSpin()
	require.NoError(t, err)
	assert.True(t, res.IsWin)
	assert.Equal(t, int64(10000), res.Reward.Amount)
	assert.Equal(t, 3, res.MaxPerDay)
	assert.NoError(t, mock.ExpectationsWereMet())
}
