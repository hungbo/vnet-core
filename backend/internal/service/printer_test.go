package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestPrinterService_List(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPrinterService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "printer_configs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "printer_type"}).
			AddRow("p1", "Kitchen Printer", "escpos").
			AddRow("p2", "Bar Printer", "escpos"))

	result, err := svc.List()
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPrinterService_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPrinterService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "printer_configs" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "printer_configs"."id" LIMIT \$2`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("p1", "Kitchen"))

	result, err := svc.GetByID("p1")
	require.NoError(t, err)
	assert.Equal(t, "Kitchen", result.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPrinterService_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPrinterService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "printer_configs" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "printer_configs"."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.GetByID("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, "printer not found", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPrinterService_Create(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPrinterService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "printer_configs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	result, err := svc.Create(&CreatePrinterRequest{
		Name:        "New Printer",
		PrinterType: "escpos",
		IPAddress:   "192.168.1.100",
		Port:        9100,
	})
	require.NoError(t, err)
	assert.Equal(t, "New Printer", result.Name)
	assert.Equal(t, 9100, result.Port)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPrinterService_Create_DefaultPort(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPrinterService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "printer_configs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	result, err := svc.Create(&CreatePrinterRequest{
		Name:        "Default Port Printer",
		PrinterType: "escpos",
	})
	require.NoError(t, err)
	assert.Equal(t, 9100, result.Port)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPrinterService_Create_IsDefault(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPrinterService(db, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "printer_configs" SET "is_default"=\$1 WHERE is_default = \$2`).
		WithArgs(false, true).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "printer_configs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	result, err := svc.Create(&CreatePrinterRequest{
		Name:        "Default Printer",
		PrinterType: "escpos",
		IsDefault:   true,
	})
	require.NoError(t, err)
	assert.True(t, result.IsDefault)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPrinterService_Update_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPrinterService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "printer_configs" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "printer_configs"."id" LIMIT \$2`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "ip_address"}).AddRow("p1", "Old", "192.168.1.1"))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "printer_configs" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT \* FROM "printer_configs" WHERE id = \$1 AND "printer_configs"."id" = \$2 ORDER BY "printer_configs"."id" LIMIT \$3`).
		WithArgs("p1", "p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "ip_address"}).AddRow("p1", "Updated", "192.168.1.2"))

	name := "Updated"
	ip := "192.168.1.2"
	result, err := svc.Update("p1", &UpdatePrinterRequest{Name: &name, IPAddress: &ip})
	require.NoError(t, err)
	assert.Equal(t, "Updated", result.Name)
	assert.Equal(t, "192.168.1.2", result.IPAddress)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPrinterService_Delete_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewPrinterService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "printer_configs" WHERE id = \$1 AND deleted_at IS NULL ORDER BY "printer_configs"."id" LIMIT \$2`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("p1", "Test"))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "printer_configs" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.Delete("p1")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
