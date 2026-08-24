package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/model"
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
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "audit_logs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow("audit-1", testNow))
	mock.ExpectCommit()

	result, err := svc.Create(&CreatePromotionRequest{
		Name: "Happy Hour",
		Type: "discount",
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

	mock.ExpectBegin()
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
