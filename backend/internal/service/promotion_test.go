package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromotionService_List(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPromotionService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "promotions" WHERE deleted_at IS NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT \* FROM "promotions" WHERE deleted_at IS NULL ORDER BY created_at desc LIMIT \$1`).
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

	mock.ExpectQuery(`SELECT \* FROM "promotions" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "promotions"."id" LIMIT \$2`).
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

	mock.ExpectQuery(`SELECT \* FROM "promotions" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "promotions"."id" LIMIT \$2`).
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

	mock.ExpectQuery(`SELECT \* FROM "lucky_spin_rewards" WHERE is_active = \$1`).
		WithArgs(true).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "reward_type", "reward_value", "probability", "max_per_day", "is_active"}).
			AddRow("r1", "100 Bonus", "bonus_points", `{"amount":100}`, 0.5, 3, true).
			AddRow("r2", "50 VND", "balance", `{"amount":50}`, 0.3, 3, true))

	result, err := svc.GetLuckySpinRewards()
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}
