package service

import (
	"fmt"
	"testing"
	"time"
	"unicode/utf8"

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

	mock.ExpectQuery(`SELECT count\(\*\) FROM "machines"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE "machines"\."deleted_at" IS NULL ORDER BY id desc LIMIT \$1`).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code", "status"}).
			AddRow("m1", "M-001", "available").
			AddRow("m2", "M-002", "offline"))

	result, err := svc.List(pagination.Params{Page: 1, PageSize: 20, Sort: "id", Order: "desc"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), result.Total)
	assert.NotNil(t, result.Items)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 AND "machines"\."deleted_at" IS NULL ORDER BY "machines"\."id" LIMIT \$2`).
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

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 AND "machines"\."deleted_at" IS NULL ORDER BY "machines"\."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.GetByID("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, "không tìm thấy máy", err.Error())
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
	})
	require.NoError(t, err)
	assert.Equal(t, "M-003", result.MachineCode)
	assert.Equal(t, "offline", result.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_Update_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 AND "machines"\."deleted_at" IS NULL ORDER BY "machines"\."id" LIMIT \$2`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code", "status"}).AddRow("m1", "M-001", "offline"))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "machines" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	status := "available"
	result, err := svc.Update("m1", &UpdateMachineRequest{Status: &status})
	require.NoError(t, err)
	assert.Equal(t, "available", result.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_Update_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 AND "machines"\."deleted_at" IS NULL ORDER BY "machines"\."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.Update("nonexistent", &UpdateMachineRequest{})
	assert.Error(t, err)
	assert.Equal(t, "không tìm thấy máy", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_Delete_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 AND "machines"\."deleted_at" IS NULL ORDER BY "machines"\."id" LIMIT \$2`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code"}).AddRow("m1", "M-001"))

	// Máy còn bất kỳ chứng từ nào thì chỉ tắt được, không xoá.
	mock.ExpectQuery(`SELECT count\(\*\) FROM "machine_sessions"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "orders"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "machine_bookings"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "service_feedbacks"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectBegin()
	// Tài sản gắn máy là con sở hữu, xoá theo trong cùng transaction.
	mock.ExpectExec(`UPDATE "machine_assets" SET`).
		WillReturnResult(sqlmock.NewResult(1, 0))
	// Soft delete: the row is stamped, not removed.
	mock.ExpectExec(`UPDATE "machines" SET "deleted_at"=\$1 WHERE "machines"."id" = \$2 AND "machines"."deleted_at" IS NULL`).
		WithArgs(anyTime{}, "m1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.Delete("m1")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_Delete_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 AND "machines"\."deleted_at" IS NULL ORDER BY "machines"\."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	err := svc.Delete("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, "không tìm thấy máy", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_Heartbeat_OfflineToAvailable(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 AND "machines"\."deleted_at" IS NULL ORDER BY "machines"\."id" LIMIT \$2`).
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

	err := svc.Heartbeat("m1", HeartbeatRequest{CPUTemp: 45.0, GPUTemp: 60.0, IP: "192.168.1.1", MAC: "AA:BB:CC:DD:EE:FF"})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Gói tin KHÔNG khai ip/mac thì hai cột đó phải giữ nguyên.
//
// Bản cũ ghi đè vô điều kiện, mà vòng giám sát 60 giây gửi gói không có ip/mac,
// nên mỗi phút địa chỉ IP và MAC của máy lại bị xoá trắng rồi nhịp tim sau mới
// ghi lại — trên trang Máy nó hiện thành hai cột nhấp nháy lúc có lúc không.
// Biểu thức dưới đây neo cả hai đầu: thừa một cột là không khớp.
func TestMachineService_Heartbeat_GoiTinThieuIPThiKhongXoaTrang(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow("m1", "available"))

	mock.ExpectBegin()
	mock.ExpectExec(`^UPDATE "machines" SET "booted_at"=\$1,"cpu_temp"=\$2,"gpu_temp"=\$3,"last_heartbeat"=\$4,"updated_at"=\$5 WHERE`).
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

	err := svc.Heartbeat("m1", HeartbeatRequest{CPUTemp: 45, GPUTemp: 60, CPUUsage: 12, Uptime: 3600})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Cấu hình máy do máy trạm tự khai được ghi vào bảng machines, và uptime rơi
// vào dòng lịch sử phần cứng thay vì rơi vào hư không như trước.
func TestMachineService_Heartbeat_GhiCauHinhMayTuKhai(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow("m1", "available"))

	mock.ExpectBegin()
	mock.ExpectExec(`^UPDATE "machines" SET "booted_at"=\$1,"cpu_name"=\$2,"cpu_temp"=\$3,"gpu_name"=\$4,"gpu_temp"=\$5,`+
		`"ip_address"=\$6,"last_heartbeat"=\$7,"mac_address"=\$8,"ram_gb"=\$9,"storage_gb"=\$10,"updated_at"=\$11 WHERE`).
		WithArgs(anyTime{}, "Intel Core i5-12400F", 45.0, "NVIDIA GeForce RTX 3060", 60.0,
			"192.168.1.5", anyTime{}, "AA:BB:CC:DD:EE:FF", 16, 512, anyTime{}, "m1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "audit_logs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow("audit-1", time.Now()))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "machine_hardware_snapshots"`).
		WithArgs("m1", 45.0, 60.0, 12.5, 40.0, 55.0, int64(86400)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	err := svc.Heartbeat("m1", HeartbeatRequest{
		CPUTemp: 45, GPUTemp: 60, IP: "192.168.1.5", MAC: "AA:BB:CC:DD:EE:FF",
		CPUUsage: 12.5, RAMUsage: 40, DiskUsage: 55, Uptime: 86400,
		CPUName: "Intel Core i5-12400F", GPUName: "NVIDIA GeForce RTX 3060",
		RAMGB: 16, StorageGB: 512,
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Tên CPU dài hơn bề rộng cột thì phải cắt theo KÝ TỰ. PostgreSQL từ chối cả
// câu lệnh khi chuỗi quá dài chứ không cắt hộ, nên một nhịp tim là hỏng cả
// nhịp tim lẫn dòng lịch sử.
func TestTruncateRunes(t *testing.T) {
	assert.Equal(t, "abc", truncateRunes("abc", 5))
	assert.Equal(t, "abcde", truncateRunes("abcdefgh", 5))

	// Cắt theo byte sẽ tạo chuỗi UTF-8 hỏng ở đây.
	got := truncateRunes("Bộ xử lý Trung tâm", 5)
	assert.Equal(t, "Bộ xử", got)
	assert.True(t, utf8.ValidString(got))
}

// Nhật ký phần cứng phải sắp theo THỜI GIAN.
//
// Mặc định của pagination là "id desc", mà id là UUID ngẫu nhiên — nghĩa là bảng
// "50 số đo gần nhất" trên màn hình thật ra là 50 dòng bất kỳ, xếp lộn xộn. Với
// vài dòng dữ liệu thì trông vẫn hợp lý, nên lỗi này không tự lộ ra bao giờ.
func TestMachineService_GetHardwareHistory_SapTheoThoiGian(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "machine_hardware_snapshots" WHERE machine_id = \$1`).
		WithArgs("m1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	mock.ExpectQuery(`SELECT \* FROM "machine_hardware_snapshots" WHERE machine_id = \$1 ORDER BY created_at desc LIMIT \$2`).
		WithArgs("m1", 50).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_id", "uptime"}).
			AddRow("s1", "m1", int64(120)).
			AddRow("s2", "m1", int64(60)))

	// Cố ý truyền vào thứ tự mặc định của pagination: service phải ghi đè nó.
	res, err := svc.GetHardwareHistory("m1", pagination.Params{
		Page: 1, PageSize: 50, Sort: "id", Order: "desc",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), res.Total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Không có máy nào kết nối thì lệnh phải báo offline. Bản cũ trả nil ở đây,
// nên giao diện hiện "đã gửi lệnh" trong khi chẳng có gì xảy ra.
func TestMachineService_RemoteAction_OfflineMachineReports(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, hub.New(nil), NewAuditService(db))

	mock.ExpectQuery(`SELECT (.+) FROM "machines" WHERE (.+)`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code", "status"}).AddRow("m1", "M-001", "available"))
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result, err := svc.RemoteAction("m1", "shutdown", nil, "u1")
	assert.ErrorIs(t, err, ErrMachineOffline)
	// Lệnh không tới được máy thì KHÔNG được chốt phiên: không bao giờ tính tiền
	// của khách khi không chắc họ đã thật sự rời máy.
	assert.Nil(t, result)
}

// Lệnh lạ phải bị chặn trước khi chạm database: :action vốn là chuỗi tự do.
func TestMachineService_RemoteAction_RejectsUnknownAction(t *testing.T) {
	db, _ := newMockDB(t)
	svc := NewMachineService(db, hub.New(nil), NewAuditService(db))

	_, err := svc.RemoteAction("m1", "format_c", nil, "u1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "không được hỗ trợ")
	// Không truy vấn nào được chạy — không cần ExpectQuery nào cả.
}

// Ba lệnh giám sát phải nằm trong whitelist và phải có cờ báo-dữ-liệu-về.
func TestMachineService_RemoteAction_MonitoringActionsAllowed(t *testing.T) {
	for _, a := range []string{"screenshot", "process-list", "process-kill"} {
		_, ok := remoteActions[a]
		assert.True(t, ok, "lệnh giám sát %q phải được cho phép", a)
		assert.True(t, wantsReport[a], "lệnh %q phải cần request_id để khớp câu trả lời", a)
	}
}

// Cố ý không có lệnh chạy câu lệnh tuỳ ý.
func TestMachineService_RemoteAction_HasNoArbitraryExec(t *testing.T) {
	for _, forbidden := range []string{"exec", "execute", "command", "cmd", "run", "shell"} {
		_, ok := remoteActions[forbidden]
		assert.False(t, ok, "lệnh %q không được nằm trong danh sách cho phép", forbidden)
	}
}

func TestMachineService_List_Empty(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "machines"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE "machines"\."deleted_at" IS NULL ORDER BY id desc LIMIT \$1`).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	result, err := svc.List(pagination.Params{Page: 1, PageSize: 20, Sort: "id", Order: "desc"})
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Total)
	assert.NotNil(t, result.Items)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------- Machine Group ----------

func TestMachineService_ListGroups(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machine_groups" WHERE "machine_groups"\."deleted_at" IS NULL ORDER BY sort_order asc`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "sort_order"}).
			AddRow("g1", "VIP", 1).
			AddRow("g2", "Regular", 2))

	groups, err := svc.ListGroups("")
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

	result, err := svc.CreateGroup(&CreateMachineGroupRequest{Name: "VIP", Color: "#FF0000", SortOrder: 1})
	require.NoError(t, err)
	assert.Equal(t, "VIP", result.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_DeleteGroup_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machine_groups" WHERE id = \$1 AND "machine_groups"\."deleted_at" IS NULL ORDER BY "machine_groups"\."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	err := svc.DeleteGroup("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, "không tìm thấy nhóm máy", err.Error())
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

	mock.ExpectQuery(`SELECT \* FROM "machine_assets" WHERE id = \$1 AND "machine_assets"\."deleted_at" IS NULL ORDER BY "machine_assets"\."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	err := svc.DeleteAsset("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, "không tìm thấy thiết bị của máy", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1 AND "machines"\."deleted_at" IS NULL ORDER BY "machines"\."id" LIMIT \$2`).
		WithArgs("m1", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.GetByID("m1")
	assert.Error(t, err)
	assert.Equal(t, "không tìm thấy máy", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Bảng endsSession quyết định lệnh nào chấm dứt lượt chơi. Khoá màn hình hay gửi
// thông báo mà cũng chốt tiền thì nhân viên không dám bấm nút nào nữa.
func TestMachineService_OnlyShutdownAndRestartEndTheSession(t *testing.T) {
	for action := range remoteActions {
		want := action == "shutdown" || action == "restart"
		assert.Equal(t, want, endsSession[action],
			"lệnh %q: endsSession phải là %v", action, want)
	}
	// Mọi lệnh trong endsSession phải là lệnh có thật, nếu không nó là điều kiện
	// không bao giờ đúng và cả nhánh chốt phiên thành code chết.
	for action := range endsSession {
		_, ok := remoteActions[action]
		assert.True(t, ok, "lệnh %q trong endsSession không có trong remoteActions", action)
	}
}

// Máy trống thì tắt máy không được coi là lỗi — quán tắt máy không có khách suốt.
func TestMachineService_EndActiveSession_NoSessionIsNotAnError(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, hub.New(nil), NewAuditService(db)).
		WithSessions(NewSessionService(db, nil, NewAuditService(db)))

	mock.ExpectQuery(`SELECT (.+) FROM "machine_sessions" WHERE (.+)`).
		WithArgs("m1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	ended, err := svc.endActiveSession("m1")
	assert.NoError(t, err)
	assert.Nil(t, ended)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------- Machine: tạo hàng loạt ----------

const truyVanKiemMaLo = `SELECT "machine_code","deleted_at" FROM "machines" WHERE machine_code IN \(`

func TestMachineService_BatchCreateMachines_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(truyVanKiemMaLo).
		WithArgs("PC-01", "PC-02", "PC-03").
		WillReturnRows(sqlmock.NewRows([]string{"machine_code", "deleted_at"}))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "machines"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID).AddRow("m2").AddRow("m3"))
	mock.ExpectCommit()

	res, err := svc.BatchCreateMachines(&BatchCreateMachinesRequest{
		Prefix: "PC-", From: 1, To: 3, Digits: 2,
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"PC-01", "PC-02", "PC-03"}, res.Codes)
	assert.Equal(t, 3, res.Created)
	assert.True(t, res.OK())
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Lần kiểm thử không được ghi gì. Không có ExpectBegin nào, nên một INSERT lọt
// ra sẽ làm test đỏ.
func TestMachineService_BatchCreateMachines_DryRunKhongGhi(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(truyVanKiemMaLo).
		WithArgs("PC-1", "PC-2").
		WillReturnRows(sqlmock.NewRows([]string{"machine_code", "deleted_at"}))

	res, err := svc.BatchCreateMachines(&BatchCreateMachinesRequest{
		Prefix: "PC-", From: 1, To: 2, DryRun: true,
	})
	require.NoError(t, err)
	assert.True(t, res.DryRun)
	assert.Equal(t, 0, res.Created)
	assert.True(t, res.OK())
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Toàn bộ hoặc không có gì: một mã vướng thì không máy nào được tạo.
func TestMachineService_BatchCreateMachines_TrungThiKhongTaoGi(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(truyVanKiemMaLo).
		WithArgs("PC-01", "PC-02", "PC-03").
		WillReturnRows(sqlmock.NewRows([]string{"machine_code", "deleted_at"}).
			AddRow("PC-02", nil))

	res, err := svc.BatchCreateMachines(&BatchCreateMachinesRequest{
		Prefix: "PC-", From: 1, To: 3, Digits: 2,
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"PC-02"}, res.Conflicts)
	assert.Empty(t, res.Deleted)
	assert.Equal(t, 0, res.Created)
	assert.False(t, res.OK())
	assert.Contains(t, res.MoTaVuong(), "PC-02")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Máy xoá mềm vẫn giữ mã vì machine_code có unique index THƯỜNG. Phải báo riêng,
// nếu không người dùng đi tìm một máy không hiện ở đâu cả.
func TestMachineService_BatchCreateMachines_MayDaXoaTachRieng(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(truyVanKiemMaLo).
		WithArgs("PC-01", "PC-02").
		WillReturnRows(sqlmock.NewRows([]string{"machine_code", "deleted_at"}).
			AddRow("PC-01", time.Now()))

	res, err := svc.BatchCreateMachines(&BatchCreateMachinesRequest{
		Prefix: "PC-", From: 1, To: 2, Digits: 2,
	})
	require.NoError(t, err)
	assert.Empty(t, res.Conflicts)
	assert.Equal(t, []string{"PC-01"}, res.Deleted)
	assert.Equal(t, 0, res.Created)
	assert.Contains(t, res.MoTaVuong(), "đã xoá")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Các phép kiểm dưới đây phải chặn TRƯỚC khi chạm database — không mock truy vấn
// nào, nên một lần gọi DB lọt ra sẽ làm test đỏ.
func TestMachineService_BatchCreateMachines_TuChoiTruocKhiChamDB(t *testing.T) {
	cases := []struct {
		ten string
		req BatchCreateMachinesRequest
		loi string
	}{
		{"thiếu tiền tố", BatchCreateMachinesRequest{Prefix: "  ", From: 1, To: 2}, "thiếu tiền tố"},
		{"khoảng ngược", BatchCreateMachinesRequest{Prefix: "PC-", From: 9, To: 2}, "lớn hơn hoặc bằng"},
		{"vượt trần", BatchCreateMachinesRequest{Prefix: "PC-", From: 1, To: maxBatchMachines + 1}, "tối đa"},
		{"mã quá dài", BatchCreateMachinesRequest{Prefix: "MAY-TRAM-PHONG-VIP-", From: 1, To: 2, Digits: 4}, "vượt trần"},
	}
	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			db, mock := newMockDB(t)
			svc := NewMachineService(db, nil, NewAuditService(db))

			req := c.req
			res, err := svc.BatchCreateMachines(&req)
			require.Error(t, err)
			assert.Nil(t, res)
			assert.Contains(t, err.Error(), c.loi)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// Digits=0 thì không đệm số 0, và số vượt quá số chữ số đã khai cũng không bị cắt.
func TestMachineService_BatchCreateMachines_DemSoKhong(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(truyVanKiemMaLo).
		WithArgs("PC-9", "PC-10", "PC-11").
		WillReturnRows(sqlmock.NewRows([]string{"machine_code", "deleted_at"}))

	res, err := svc.BatchCreateMachines(&BatchCreateMachinesRequest{
		Prefix: "PC-", From: 9, To: 11, DryRun: true,
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"PC-9", "PC-10", "PC-11"}, res.Codes)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_BatchCreateMachines_MoTaVuongCatBot(t *testing.T) {
	res := &BatchCreateResult{}
	for i := 1; i <= 14; i++ {
		res.Conflicts = append(res.Conflicts, fmt.Sprintf("PC-%02d", i))
	}
	mo := res.MoTaVuong()
	assert.Contains(t, mo, "14 mã đã có máy")
	assert.Contains(t, mo, "PC-10")
	assert.Contains(t, mo, "và 4 mã khác")
	assert.NotContains(t, mo, "PC-14")
}

// Bỏ chọn nhóm phải ghi NULL, không phải chuỗi rỗng.
//
// Giao diện gửi group_id:"" khi người dùng không chọn nhóm hoặc xoá nhóm đang
// chọn. Cột group_id là uuid cho phép rỗng, và PostgreSQL từ chối "" cho kiểu
// uuid — lỗi thô `invalid input syntax for type uuid: ""` từng lọt thẳng ra
// người dùng ở cả ba đường: tạo một máy, sửa máy, tạo máy hàng loạt.
func TestMachineService_Update_NhomRongGhiNull(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code", "status"}).AddRow("m1", "M-001", "offline"))

	mock.ExpectBegin()
	// NULL, không phải "": đối số đầu của câu UPDATE là group_id.
	mock.ExpectExec(`UPDATE "machines" SET "group_id"=\$1`).
		WithArgs(nil, anyTime{}, "m1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	rong := ""
	_, err := svc.Update("m1", &UpdateMachineRequest{GroupID: &rong})

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_nhomRongThanhNil(t *testing.T) {
	rong := ""
	trang := "   "
	that := "550e8400-e29b-41d4-a716-446655440000"

	assert.Nil(t, nhomRongThanhNil(nil))
	assert.Nil(t, nhomRongThanhNil(&rong), `chuỗi rỗng phải thành NULL`)
	assert.Nil(t, nhomRongThanhNil(&trang), `chuỗi toàn khoảng trắng phải thành NULL`)
	assert.Equal(t, &that, nhomRongThanhNil(&that), "uuid thật phải giữ nguyên")
}

// Ô tìm kiếm trên trang Nhóm máy phải thật sự lọc.
//
// Giống hệt trang Nhóm hội viên: giao diện vẫn gửi ?search= nhưng hàm lấy danh
// sách bỏ qua tham số đó, nên gõ gì cũng ra đủ nhóm — một nút bấm vào không làm gì.
func TestMachineService_ListGroups_LocTheoTuKhoa(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machine_groups" WHERE unaccent\(name\) ILIKE unaccent\(\$1\)`).
		WithArgs("%may lanh%").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("g3", "Phòng máy lạnh"))

	groups, err := svc.ListGroups("may lanh")

	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Equal(t, "Phòng máy lạnh", groups[0].Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Ô tìm kiếm trên trang Máy phải lọc theo cả mã máy lẫn tên nhóm.
//
// Ô nhập ghi "Tìm mã máy / nhóm" nhưng hàm List bỏ qua hẳn tham số search —
// đây là lần thứ ba cùng một lỗi (hội viên, nhóm máy, rồi máy). Quán vài trăm
// máy thì đó là ô duy nhất để tìm một máy cụ thể.
func TestMachineService_List_LocTheoMaMayVaTenNhom(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	dieuKien := `WHERE \(unaccent\(machine_code\) ILIKE unaccent\(\$1\) OR group_id IN \(SELECT id FROM machine_groups WHERE unaccent\(name\) ILIKE unaccent\(\$2\) AND deleted_at IS NULL\)\)`

	mock.ExpectQuery(`SELECT count\(\*\) FROM "machines" `+dieuKien).
		WithArgs("%PC-0%", "%PC-0%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	mock.ExpectQuery(`SELECT \* FROM "machines" ` + dieuKien).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code"}).
			AddRow("m1", "PC-01").
			AddRow("m2", "PC-02"))

	res, err := svc.List(pagination.Params{Page: 1, PageSize: 20, Sort: "id", Order: "desc", Search: "PC-0"})

	require.NoError(t, err)
	assert.Equal(t, int64(2), res.Total)
	assert.NoError(t, mock.ExpectationsWereMet())
}
