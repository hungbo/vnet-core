package service

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
)

// ---------- Machine ----------

func TestMachineService_List(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "machines" WHERE store_id = \$1`).
		WithArgs(testStoreID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE store_id = \$1 ORDER BY id desc LIMIT \$2`).
		WithArgs(testStoreID, 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code", "store_id", "status"}).
			AddRow("m1", "M-001", testStoreID, "available").
			AddRow("m2", "M-002", testStoreID, "offline"))

	result, err := svc.List(pagination.Params{Page: 1, PageSize: 20, Sort: "id", Order: "desc"}, testStoreID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), result.Total)
	assert.NotNil(t, result.Items)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 ORDER BY "machines"."id" LIMIT \$2`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code", "status"}).
			AddRow("m1", "M-001", "available"))

	machine, err := svc.GetByID("m1")
	require.NoError(t, err)
	assert.Equal(t, "M-001", machine.MachineCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 ORDER BY "machines"."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.GetByID("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, "machine not found", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_Create(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "machines"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	result, err := svc.Create(&CreateMachineRequest{
		MachineCode: "M-003",
		CPUName:     "Intel i7",
		RAMGB:       16,
		GPUName:     "RTX 3060",
		StorageGB:   512,
		OSInfo:      "Windows 11",
	}, testStoreID)
	require.NoError(t, err)
	assert.Equal(t, "M-003", result.MachineCode)
	assert.Equal(t, "offline", result.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_Update_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 ORDER BY "machines"."id" LIMIT \$2`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code", "status"}).AddRow("m1", "M-001", "offline"))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "machines" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	status := "maintenance"
	result, err := svc.Update("m1", &UpdateMachineRequest{Status: &status})
	require.NoError(t, err)
	assert.Equal(t, "maintenance", result.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_Update_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 ORDER BY "machines"."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.Update("nonexistent", &UpdateMachineRequest{})
	assert.Error(t, err)
	assert.Equal(t, "machine not found", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_Delete_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 ORDER BY "machines"."id" LIMIT \$2`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code"}).AddRow("m1", "M-001"))

	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "machines" WHERE "machines"."id" = \$1`).
		WithArgs("m1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.Delete("m1")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_Delete_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 ORDER BY "machines"."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	err := svc.Delete("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, "machine not found", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_Heartbeat_OfflineToAvailable(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 ORDER BY "machines"."id" LIMIT \$2`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow("m1", "offline"))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "machines" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "audit_logs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow("audit-1", time.Now()))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "machine_hardware_snapshots"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	err := svc.Heartbeat("m1", 45.0, 60.0, "192.168.1.1", "AA:BB:CC:DD:EE:FF")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_RemoteAction(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, hub.New(), NewAuditService(db))

	mock.ExpectQuery(`SELECT (.+) FROM "machines" WHERE (.+)`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"machine_code", "status"}).AddRow("M-001", "available"))

	err := svc.RemoteAction("m1", "shutdown", nil)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_List_Empty(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "machines" WHERE store_id = \$1`).
		WithArgs(testStoreID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE store_id = \$1 ORDER BY id desc LIMIT \$2`).
		WithArgs(testStoreID, 20).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	result, err := svc.List(pagination.Params{Page: 1, PageSize: 20, Sort: "id", Order: "desc"}, testStoreID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Total)
	assert.NotNil(t, result.Items)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------- Machine Group ----------

func TestMachineService_ListGroups(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machine_groups" WHERE store_id = \$1 ORDER BY sort_order asc`).
		WithArgs(testStoreID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "sort_order"}).
			AddRow("g1", "VIP", 1).
			AddRow("g2", "Regular", 2))

	groups, err := svc.ListGroups(testStoreID)
	require.NoError(t, err)
	assert.Len(t, groups, 2)
	assert.Equal(t, "VIP", groups[0].Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_CreateGroup(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "machine_groups"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	result, err := svc.CreateGroup(&CreateMachineGroupRequest{Name: "VIP", Color: "#FF0000", SortOrder: 1}, testStoreID)
	require.NoError(t, err)
	assert.Equal(t, "VIP", result.Name)
	assert.Equal(t, testStoreID, *result.StoreID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_DeleteGroup_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machine_groups" WHERE id = \$1 ORDER BY "machine_groups"."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	err := svc.DeleteGroup("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, "machine group not found", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------- Machine Price ----------

func TestMachineService_ListPrices_All(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machine_prices"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "price_per_hour"}).AddRow("p1", int64(10000)))

	prices, err := svc.ListPrices("")
	require.NoError(t, err)
	assert.Len(t, prices, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_ListPrices_FilterByGroup(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machine_prices" WHERE machine_group_id = \$1`).
		WithArgs("g1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_group_id", "price_per_hour"}).AddRow("p1", "g1", int64(15000)))

	prices, err := svc.ListPrices("g1")
	require.NoError(t, err)
	assert.Len(t, prices, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_CreatePrice(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "machine_prices"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	result, err := svc.CreatePrice(&CreateMachinePriceRequest{
		PricePerHour:  20000,
		EffectiveFrom: "2025-06-01",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(20000), result.PricePerHour)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_DeletePrice_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machine_prices" WHERE id = \$1 ORDER BY "machine_prices"."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	err := svc.DeletePrice("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_UpdatePrice_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machine_prices" WHERE id = \$1 ORDER BY "machine_prices"."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.UpdatePrice("nonexistent", &UpdateMachinePriceRequest{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------- Machine Asset ----------

func TestMachineService_CreateAsset_DefaultStatus(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "machine_assets"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	result, err := svc.CreateAsset(&CreateMachineAssetRequest{
		MachineID: "m1",
		AssetType: "monitor",
		Brand:     "Dell",
	})
	require.NoError(t, err)
	assert.Equal(t, "monitor", result.AssetType)
	assert.Equal(t, "good", result.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_DeleteAsset_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machine_assets" WHERE id = \$1 ORDER BY "machine_assets"."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	err := svc.DeleteAsset("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, "machine asset not found", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 ORDER BY "machines"."id" LIMIT \$2`).
		WithArgs("m1", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.GetByID("m1")
	assert.Error(t, err)
	assert.Equal(t, "machine not found", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}
