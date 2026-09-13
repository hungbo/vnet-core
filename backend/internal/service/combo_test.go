package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestComboService_List(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "combos" WHERE "combos"\."deleted_at" IS NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE "combos"\."deleted_at" IS NULL ORDER BY created_at desc LIMIT \$1`).
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

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1 AND "combos"\."deleted_at" IS NULL ORDER BY "combos"."id" LIMIT \$2`).
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
		SlotStart:    "08:00",
		SlotEnd:      "23:00",
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

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1 AND "combos"\."deleted_at" IS NULL ORDER BY "combos"."id" LIMIT \$2`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price", "created_at"}).
			AddRow("c1", "Old Name", int64(50000), testNow))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "combos" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1 AND "combos"\."deleted_at" IS NULL AND "combos"\."id" = \$2 ORDER BY "combos"\."id" LIMIT \$3`).
		WithArgs("c1", "c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price", "type", "created_at"}).
			AddRow("c1", "New Name", int64(50000), "fixed_slot", testNow))

	name := "New Name"
	result, err := svc.Update("c1", &UpdateComboRequest{Name: name})
	require.NoError(t, err)
	assert.Equal(t, "New Name", result.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_Update_FixedSlot_MissingSlot(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1 AND "combos"\."deleted_at" IS NULL ORDER BY "combos"."id" LIMIT \$2`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type", "slot_start", "slot_end", "created_at"}).
			AddRow("c1", "Gaming 3h", "fixed_slot", "", "", testNow))

	_, err := svc.Update("c1", &UpdateComboRequest{Type: "fixed_slot"})
	assert.EqualError(t, err, "slot_start and slot_end are required for fixed_slot type")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_Update_Prepaid_MissingMinutes(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1 AND "combos"\."deleted_at" IS NULL ORDER BY "combos"."id" LIMIT \$2`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type", "total_minutes", "created_at"}).
			AddRow("c1", "Gaming 3h", "prepaid", 0, testNow))

	_, err := svc.Update("c1", &UpdateComboRequest{Type: "prepaid", TotalMinutes: 0})
	assert.EqualError(t, err, "total_minutes must be > 0 for prepaid type")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_Delete_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1 AND "combos"\."deleted_at" IS NULL ORDER BY "combos"."id" LIMIT \$2`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("c1"))

	// Combo đã bán hay đã có phiên chơi dùng nó thì không xoá được nữa.
	mock.ExpectQuery(`SELECT count\(\*\) FROM "combo_purchases"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "machine_sessions"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
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

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE \(id = \$1 AND is_active = \$2\) AND "combos"\."deleted_at" IS NULL ORDER BY "combos"\."id" LIMIT \$3`).
		WithArgs("c1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type", "price", "total_minutes", "member_prefix", "member_count", "validity_days", "created_at"}).
			AddRow("c1", "Gaming 3h", "fixed_slot", int64(50000), 180, "GAMIN", 0, 30, testNow))

	// Atomic tx: lock combo → insert member → update member_count
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1 AND "combos"\."deleted_at" IS NULL ORDER BY "combos"\."id" LIMIT \$2 FOR UPDATE`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "member_prefix", "member_count"}).
			AddRow("c1", "Gaming 3h", "GAMIN", 0))
	mock.ExpectQuery(`INSERT INTO "members"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow("m1", testNow, testNow))
	mock.ExpectExec(`UPDATE "combos" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// Purchase creation tx
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$2 FOR UPDATE`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "balance", "total_spent", "created_at"}).
			AddRow("m1", "MEMBER-001", int64(0), int64(0), testNow))
	mock.ExpectQuery(`INSERT INTO "combo_purchases"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("p1"))
	mock.ExpectQuery(`INSERT INTO "member_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("t1"))
	mock.ExpectExec(`UPDATE "members" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result, err := svc.Purchase("c1", &PurchaseComboRequest{
		CustomerName:  "John",
		CustomerPhone: "0123456789",
		PaymentMethod: "cash",
	}, "u1")
	require.NoError(t, err)
	assert.Equal(t, "p1", result.ID)
	assert.Equal(t, int64(50000), result.Price)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_List_WithSearch(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "combos" WHERE unaccent\(name\) ILIKE unaccent\(\$1\) AND "combos"\."deleted_at" IS NULL`).
		WithArgs("%Gaming%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE unaccent\(name\) ILIKE unaccent\(\$1\) AND "combos"\."deleted_at" IS NULL ORDER BY created_at desc LIMIT \$2`).
		WithArgs("%Gaming%", 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price", "type", "created_at"}).
			AddRow("c1", "Gaming 3h", int64(50000), "fixed_slot", testNow))

	result, err := svc.List(&ComboListRequest{Page: 1, PageSize: 20, Search: "Gaming"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	items := result.Items.([]ComboResponse)
	assert.Len(t, items, 1)
	assert.Equal(t, "Gaming 3h", items[0].Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_List_WithPagination(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "combos" WHERE "combos"\."deleted_at" IS NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE "combos"\."deleted_at" IS NULL ORDER BY created_at desc LIMIT \$1 OFFSET \$2`).
		WithArgs(5, 5).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price", "type", "created_at"}).
			AddRow("c6", "Combo 6", int64(30000), "prepaid", testNow))

	result, err := svc.List(&ComboListRequest{Page: 2, PageSize: 5})
	require.NoError(t, err)
	assert.Equal(t, int64(10), result.Total)
	assert.Len(t, result.Items, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1 AND "combos"\."deleted_at" IS NULL ORDER BY "combos"."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.GetByID("nonexistent")
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_Create_Prepaid(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "combos"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	result, err := svc.Create(&CreateComboRequest{
		Name:         "Prepaid 2h",
		Type:         "prepaid",
		TotalMinutes: 120,
		Price:        30000,
	})
	require.NoError(t, err)
	assert.Equal(t, "Prepaid 2h", result.Name)
	assert.Equal(t, "prepaid", result.Type)
	assert.Equal(t, 120, result.TotalMinutes)
	assert.Equal(t, "PREPA", result.MemberPrefix)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_Create_FixedSlot_MissingSlot(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	_, err := svc.Create(&CreateComboRequest{
		Name:  "No Slot",
		Type:  "fixed_slot",
		Price: 10000,
	})
	assert.EqualError(t, err, "slot_start and slot_end are required for fixed_slot type")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_Create_Prepaid_MissingMinutes(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	_, err := svc.Create(&CreateComboRequest{
		Name:         "No Minutes",
		Type:         "prepaid",
		TotalMinutes: 0,
		Price:        10000,
	})
	assert.EqualError(t, err, "total_minutes must be > 0 for prepaid type")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_Update_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1 AND "combos"\."deleted_at" IS NULL ORDER BY "combos"."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.Update("nonexistent", &UpdateComboRequest{Name: "New"})
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_Update_Partial(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1 AND "combos"\."deleted_at" IS NULL ORDER BY "combos"."id" LIMIT \$2`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price", "created_at"}).
			AddRow("c1", "Gaming 3h", int64(50000), testNow))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "combos" SET .* WHERE .*`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1 AND "combos"\."deleted_at" IS NULL AND "combos"\."id" = \$2 ORDER BY "combos"\."id" LIMIT \$3`).
		WithArgs("c1", "c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price", "type", "created_at"}).
			AddRow("c1", "Gaming 3h", int64(60000), "fixed_slot", testNow))

	result, err := svc.Update("c1", &UpdateComboRequest{Price: 60000})
	require.NoError(t, err)
	assert.Equal(t, int64(60000), result.Price)
	assert.Equal(t, "Gaming 3h", result.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_Delete_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1 AND "combos"\."deleted_at" IS NULL ORDER BY "combos"."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	err := svc.Delete("nonexistent")
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_Purchase_ExistingMember(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE \(id = \$1 AND is_active = \$2\) AND "combos"\."deleted_at" IS NULL ORDER BY "combos"\."id" LIMIT \$3`).
		WithArgs("c1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type", "price", "total_minutes", "member_prefix", "member_count", "validity_days", "created_at"}).
			AddRow("c1", "Gaming 3h", "fixed_slot", int64(50000), 180, "GAMIN", 5, 30, testNow))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$2 FOR UPDATE`).
		WithArgs("existing-mem", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance", "total_spent", "created_at"}).
			AddRow("existing-mem", int64(100000), int64(200000), testNow))
	mock.ExpectQuery(`INSERT INTO "combo_purchases"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("p1"))
	mock.ExpectQuery(`INSERT INTO "member_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("t1"))
	mock.ExpectExec(`UPDATE "members" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result, err := svc.Purchase("c1", &PurchaseComboRequest{
		MemberID:      "existing-mem",
		PaymentMethod: "cash",
	}, "u1")
	require.NoError(t, err)
	assert.Equal(t, "p1", result.ID)
	assert.Equal(t, "Gaming 3h", result.ComboName)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_Purchase_InactiveCombo(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE \(id = \$1 AND is_active = \$2\) AND "combos"\."deleted_at" IS NULL ORDER BY "combos"\."id" LIMIT \$3`).
		WithArgs("c1", true, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.Purchase("c1", &PurchaseComboRequest{
		MemberID:      "existing-mem",
		PaymentMethod: "cash",
	}, "u1")
	assert.EqualError(t, err, "không tìm thấy gói cước hoặc gói đã ngừng bán")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_Purchase_BalancePayment(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE \(id = \$1 AND is_active = \$2\) AND "combos"\."deleted_at" IS NULL ORDER BY "combos"\."id" LIMIT \$3`).
		WithArgs("c1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type", "price", "total_minutes", "member_prefix", "member_count", "validity_days", "created_at"}).
			AddRow("c1", "Gaming 3h", "fixed_slot", int64(50000), 180, "GAMIN", 5, 30, testNow))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$2 FOR UPDATE`).
		WithArgs("mem-rich", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance", "total_spent", "created_at"}).
			AddRow("mem-rich", int64(200000), int64(100000), testNow))
	mock.ExpectQuery(`INSERT INTO "combo_purchases"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("p1"))
	mock.ExpectQuery(`INSERT INTO "member_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("t1"))
	mock.ExpectExec(`UPDATE "members" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result, err := svc.Purchase("c1", &PurchaseComboRequest{
		MemberID:      "mem-rich",
		PaymentMethod: "balance",
	}, "u1")
	require.NoError(t, err)
	assert.Equal(t, "p1", result.ID)
	assert.Equal(t, int64(50000), result.Price)
	assert.Equal(t, "Gaming 3h", result.ComboName)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_Activate_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	// Purchase lookup
	mock.ExpectQuery(`SELECT \* FROM "combo_purchases" WHERE id = \$1 ORDER BY "combo_purchases"."id" LIMIT \$2`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "combo_id", "member_id", "price", "activated", "remaining_minutes", "created_at"}).
			AddRow("p1", "c1", "mem1", int64(50000), false, 180, testNow))

	// Combo lookup
	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1 AND "combos"\."deleted_at" IS NULL ORDER BY "combos"\."id" LIMIT \$2`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type", "total_minutes", "slot_end", "created_at"}).
			AddRow("c1", "Gaming 3h", "fixed_slot", 180, "23:00", testNow))

	mock.ExpectBegin()
	// Activate khoá và đọc lại purchase trong transaction: kiểm tra Activated ở
	// trên chạy ngoài transaction nên hai lệnh kích hoạt song song đều lọt.
	mock.ExpectQuery(`SELECT \* FROM "combo_purchases" WHERE id = \$1 ORDER BY "combo_purchases"."id" LIMIT \$2 FOR UPDATE`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "combo_id", "member_id", "price", "activated", "remaining_minutes", "created_at"}).
			AddRow("p1", "c1", "mem1", int64(50000), false, 180, testNow))
	// Lock machine
	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE \(id = \$1 AND is_active = \$2\) AND "machines"\."deleted_at" IS NULL ORDER BY "machines"\."id" LIMIT \$3 FOR UPDATE`).
		WithArgs("m1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code", "status"}).
			AddRow("m1", "M-001", "available"))
	// Create session
	mock.ExpectQuery(`INSERT INTO "machine_sessions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("s1"))
	// Update machine status
	mock.ExpectExec(`UPDATE "machines" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	// Update purchase
	mock.ExpectExec(`UPDATE "combo_purchases" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result, err := svc.Activate("p1", &ActivateComboRequest{MachineID: "m1"})
	require.NoError(t, err)
	assert.True(t, result.Activated)
	assert.Equal(t, "Gaming 3h", result.ComboName)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_Activate_AlreadyActivated(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "combo_purchases" WHERE id = \$1 ORDER BY "combo_purchases"."id" LIMIT \$2`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "combo_id", "activated", "created_at"}).
			AddRow("p1", "c1", true, testNow))

	_, err := svc.Activate("p1", &ActivateComboRequest{MachineID: "m1"})
	assert.EqualError(t, err, "lượt mua này đã được kích hoạt")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_Activate_PurchaseNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "combo_purchases" WHERE id = \$1 ORDER BY "combo_purchases"."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.Activate("nonexistent", &ActivateComboRequest{MachineID: "m1"})
	assert.EqualError(t, err, "không tìm thấy lượt mua")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_Activate_MachineNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "combo_purchases" WHERE id = \$1 ORDER BY "combo_purchases"."id" LIMIT \$2`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "combo_id", "member_id", "price", "activated", "remaining_minutes", "created_at"}).
			AddRow("p1", "c1", "mem1", int64(50000), false, 180, testNow))

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1 AND "combos"\."deleted_at" IS NULL ORDER BY "combos"\."id" LIMIT \$2`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type", "total_minutes", "created_at"}).
			AddRow("c1", "Gaming 3h", "fixed_slot", 180, testNow))

	mock.ExpectBegin()
	// Activate khoá và đọc lại purchase trong transaction: kiểm tra Activated ở
	// trên chạy ngoài transaction nên hai lệnh kích hoạt song song đều lọt.
	mock.ExpectQuery(`SELECT \* FROM "combo_purchases" WHERE id = \$1 ORDER BY "combo_purchases"."id" LIMIT \$2 FOR UPDATE`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "combo_id", "member_id", "price", "activated", "remaining_minutes", "created_at"}).
			AddRow("p1", "c1", "mem1", int64(50000), false, 180, testNow))
	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE \(id = \$1 AND is_active = \$2\) AND "machines"\."deleted_at" IS NULL ORDER BY "machines"\."id" LIMIT \$3 FOR UPDATE`).
		WithArgs("nonexistent", true, 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectRollback()

	_, err := svc.Activate("p1", &ActivateComboRequest{MachineID: "nonexistent"})
	assert.EqualError(t, err, "không tìm thấy máy")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Buying with an empty wallet used to create the combo anyway and log a
// transaction claiming money moved when it had not.
func TestComboService_Purchase_RejectsInsufficientBalance(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE \(id = \$1 AND is_active = \$2\) AND "combos"\."deleted_at" IS NULL ORDER BY "combos"\."id" LIMIT \$3`).
		WithArgs("c1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type", "price", "total_minutes", "member_prefix", "member_count", "validity_days", "created_at"}).
			AddRow("c1", "Gaming 3h", "fixed_slot", int64(50000), 180, "GAMIN", 5, 30, testNow))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$2 FOR UPDATE`).
		WithArgs("mem-broke", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance", "total_spent", "created_at"}).
			AddRow("mem-broke", int64(100), int64(0), testNow))
	mock.ExpectRollback()

	_, err := svc.Purchase("c1", &PurchaseComboRequest{
		MemberID:      "mem-broke",
		PaymentMethod: "balance",
	}, "u1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "số dư không đủ")
	assert.NoError(t, mock.ExpectationsWereMet())
}
