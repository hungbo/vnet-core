package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComboService_List(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "combos" WHERE deleted_at IS NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE deleted_at IS NULL ORDER BY created_at desc LIMIT \$1`).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price", "type", "created_at"}).
			AddRow("c1", "Gaming 3h", int64(50000), "fixed_slot", testNow))

	result, err := svc.List(&ComboListRequest{Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.Len(t, result.Items, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "combos"."id" LIMIT \$2`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price", "type", "created_at"}).
			AddRow("c1", "Gaming 3h", int64(50000), "fixed_slot", testNow))

	result, err := svc.GetByID("c1")
	require.NoError(t, err)
	assert.Equal(t, "Gaming 3h", result.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_Create(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "combos"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	result, err := svc.Create(&CreateComboRequest{
		Name:         "Gaming 3h",
		Type:         "fixed_slot",
		TotalMinutes: 180,
		Price:        50000,
	})
	require.NoError(t, err)
	assert.Equal(t, "Gaming 3h", result.Name)
	assert.Equal(t, "GAMIN", result.MemberPrefix)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_Update_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "combos"."id" LIMIT \$2`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price", "created_at"}).
			AddRow("c1", "Old Name", int64(50000), testNow))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "combos" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1 AND "combos"."id" = \$2 ORDER BY "combos"."id" LIMIT \$3`).
		WithArgs("c1", "c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price", "type", "created_at"}).
			AddRow("c1", "New Name", int64(50000), "fixed_slot", testNow))

	name := "New Name"
	result, err := svc.Update("c1", &UpdateComboRequest{Name: name})
	require.NoError(t, err)
	assert.Equal(t, "New Name", result.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_Delete_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "combos"."id" LIMIT \$2`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("c1"))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "combos" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.Delete("c1")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_Purchase_CreatesMember(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1 AND deleted_at IS NULL AND is_active = \$2 ORDER BY "combos"."id" LIMIT \$3`).
		WithArgs("c1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type", "price", "total_minutes", "member_prefix", "member_count", "validity_days", "created_at"}).
			AddRow("c1", "Gaming 3h", "fixed_slot", int64(50000), 180, "GAMIN", 0, 30, testNow))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "members"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow("m1", testNow, testNow))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "combos" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "combo_purchases"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("p1"))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 ORDER BY "members"."id" LIMIT \$2`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "balance", "total_spent", "created_at"}).
			AddRow("m1", "MEMBER-001", int64(0), int64(0), testNow))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "member_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("t1"))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "members" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result, err := svc.Purchase("c1", &PurchaseComboRequest{
		CustomerName:  "John",
		CustomerPhone: "0123456789",
		PaymentMethod: "cash",
	}, "s1", "u1")
	require.NoError(t, err)
	assert.Equal(t, "p1", result.ID)
	assert.Equal(t, int64(50000), result.Price)
	assert.NoError(t, mock.ExpectationsWereMet())
}
