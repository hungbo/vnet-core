package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
)

func TestHwDiff(t *testing.T) {
	base := func() *model.Machine {
		return &model.Machine{
			CPUName: "Intel Core i5-12400F", GPUName: "RTX 3060", RAMGB: 16, StorageGB: 512,
			BootMode: "uefi", SecureBoot: "on", TPM: "2.0", HVCI: "off", Virtualization: "on", BIOSVersion: "F.42",
			Peripherals: model.StringArray{"Màn hình: DELL S2421HN", "Chuột: Logitech G102"},
		}
	}
	same := func() HeartbeatRequest {
		return HeartbeatRequest{
			CPUName: "Intel Core i5-12400F", GPUName: "RTX 3060", RAMGB: 16, StorageGB: 512,
			BootMode: "uefi", SecureBoot: "on", TPM: "2.0", HVCI: "off", Virtualization: "on", BIOSVersion: "F.42",
		}
	}
	periph := []string{"Màn hình: DELL S2421HN", "Chuột: Logitech G102"}

	t.Run("không đổi gì thì không ghi", func(t *testing.T) {
		assert.Empty(t, hwDiff(base(), same(), periph))
	})

	t.Run("nâng RAM và đổi card đồ hoạ", func(t *testing.T) {
		req := same()
		req.RAMGB = 32
		req.GPUName = "RTX 4070"
		got := hwDiff(base(), req, periph)
		assert.Equal(t, []hwChange{
			{Component: "gpu", Action: "changed", Old: "RTX 3060", New: "RTX 4070"},
			{Component: "ram", Action: "changed", Old: "16 GB", New: "32 GB"},
		}, got)
	})

	t.Run("tắt Secure Boot trong BIOS", func(t *testing.T) {
		req := same()
		req.SecureBoot = "off"
		assert.Equal(t, []hwChange{{Component: "secure_boot", Action: "changed", Old: "on", New: "off"}},
			hwDiff(base(), req, periph))
	})

	t.Run("giá trị cũ rỗng là lần đầu khai, không phải thay đổi", func(t *testing.T) {
		m := base()
		m.SecureBoot, m.TPM, m.BIOSVersion, m.RAMGB = "", "", "", 0
		assert.Empty(t, hwDiff(m, same(), periph))
	})

	t.Run("máy trạm không đọc được lần này thì không tính là đổi", func(t *testing.T) {
		assert.Empty(t, hwDiff(base(), HeartbeatRequest{}, nil))
	})

	t.Run("cắm thêm màn hình, rút chuột", func(t *testing.T) {
		got := hwDiff(base(), same(), []string{"Màn hình: DELL S2421HN", "Màn hình: LG 24GN600"})
		assert.Equal(t, []hwChange{
			{Component: "monitor", Action: "added", New: "LG 24GN600"},
			{Component: "mouse", Action: "removed", Old: "Logitech G102"},
		}, got)
	})

	t.Run("hai màn hình trùng tên: rút một cái vẫn thấy", func(t *testing.T) {
		m := base()
		m.Peripherals = model.StringArray{"Màn hình: DELL S2421HN", "Màn hình: DELL S2421HN"}
		got := hwDiff(m, same(), []string{"Màn hình: DELL S2421HN"})
		assert.Equal(t, []hwChange{{Component: "monitor", Action: "removed", Old: "DELL S2421HN"}}, got)
	})

	t.Run("danh sách ngoại vi cũ rỗng thì không ghi", func(t *testing.T) {
		m := base()
		m.Peripherals = nil
		assert.Empty(t, hwDiff(m, same(), periph))
	})

	t.Run("trần số dòng mỗi nhịp tim", func(t *testing.T) {
		var many []string
		for i := 0; i < 50; i++ {
			many = append(many, "Âm thanh: Thiết bị "+strings.Repeat("x", i+1))
		}
		assert.Len(t, hwDiff(base(), same(), many), hwChangeMax)
	})
}

func TestHwDiff_TanSoQuetTut(t *testing.T) {
	m := &model.Machine{DisplayMode: "1920x1080 280Hz"}
	got := hwDiff(m, HeartbeatRequest{DisplayMode: "1920x1080 64Hz"}, nil)
	assert.Equal(t, []hwChange{{Component: "display", Action: "changed", Old: "1920x1080 280Hz", New: "1920x1080 64Hz"}}, got)

	// Tăng tần số, hoặc chỉ đổi độ phân giải (game vào toàn màn hình), thì không ghi.
	assert.Empty(t, hwDiff(m, HeartbeatRequest{DisplayMode: "1920x1080 280Hz"}, nil))
	assert.Empty(t, hwDiff(&model.Machine{DisplayMode: "1920x1080 60Hz"}, HeartbeatRequest{DisplayMode: "1920x1080 144Hz"}, nil))
	assert.Empty(t, hwDiff(m, HeartbeatRequest{DisplayMode: "1280x720 280Hz"}, nil))
	// Chưa có mốc cũ, hoặc máy trạm không báo lần này: không phải thay đổi.
	assert.Empty(t, hwDiff(&model.Machine{}, HeartbeatRequest{DisplayMode: "1920x1080 60Hz"}, nil))
	assert.Empty(t, hwDiff(m, HeartbeatRequest{}, nil))
}

func TestDisplayHz(t *testing.T) {
	assert.Equal(t, 144, displayHz("1920x1080 144Hz"))
	assert.Equal(t, 60, displayHz("2560x1440 60Hz"))
	assert.Equal(t, 0, displayHz(""))
	assert.Equal(t, 0, displayHz("1920x1080"))
}

// Đang có một dòng tụt Hz chưa xác nhận thì dòng tụt kế tiếp không được ghi thêm.
func TestMachineService_BoTutHzLapLai(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))
	ds := []hwChange{
		{Component: "ram", Action: "changed", Old: "16 GB", New: "32 GB"},
		{Component: "display", Action: "changed", Old: "1920x1080 144Hz", New: "1920x1080 60Hz"},
	}

	mock.ExpectQuery(`SELECT count\(\*\) FROM "machine_hardware_changes" WHERE machine_id = \$1 AND component = \$2 AND acknowledged_at IS NULL`).
		WithArgs("m1", "display").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	got := svc.boTutHzLapLai("m1", ds)
	assert.Equal(t, []hwChange{ds[0]}, got)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "machine_hardware_changes"`).
		WithArgs("m1", "display").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	assert.Len(t, svc.boTutHzLapLai("m1", ds), 2)

	// Không có dòng màn hình thì không truy vấn gì (sqlmock sẽ báo nếu có).
	assert.Len(t, svc.boTutHzLapLai("m1", ds[:1]), 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHwSummary(t *testing.T) {
	ds := []hwChange{
		{Component: "ram", Action: "changed", Old: "16 GB", New: "32 GB"},
		{Component: "monitor", Action: "added", New: "LG"},
		{Component: "mouse", Action: "removed", Old: "G102"},
		{Component: "gpu", Action: "changed", Old: "a", New: "b"},
	}
	assert.Equal(t, "ram: 16 GB → 32 GB; monitor: + LG; mouse: − G102 (+1)", hwSummary(ds))
}

func TestSanitizeSecurity(t *testing.T) {
	r := HeartbeatRequest{
		BootMode: "bios", SecureBoot: "maybe", TPM: "<b>", HVCI: "ON", Virtualization: "yes",
		BIOSVersion: "  " + strings.Repeat("v", 100), DisplayMode: "1920x1080; DROP TABLE",
	}
	r.sanitizeSecurity()
	assert.Empty(t, r.BootMode)
	assert.Empty(t, r.SecureBoot)
	assert.Empty(t, r.TPM)
	assert.Empty(t, r.HVCI)
	assert.Empty(t, r.Virtualization)
	assert.Empty(t, r.DisplayMode)
	assert.Len(t, []rune(r.BIOSVersion), 64)

	ok := HeartbeatRequest{BootMode: "uefi", SecureBoot: "unsupported", TPM: "2.0", HVCI: "on",
		Virtualization: "off", DisplayMode: "2560x1440 165Hz"}
	ok.sanitizeSecurity()
	assert.Equal(t, "uefi", ok.BootMode)
	assert.Equal(t, "unsupported", ok.SecureBoot)
	assert.Equal(t, "2.0", ok.TPM)
	assert.Equal(t, "2560x1440 165Hz", ok.DisplayMode)
	none := HeartbeatRequest{TPM: "none"}
	none.sanitizeSecurity()
	assert.Equal(t, "none", none.TPM)
}

// Máy đã có mốc: nâng RAM thì ghi đúng một dòng nhật ký, giữa cập nhật bảng máy và nhật ký nhịp tim.
func TestMachineService_Heartbeat_GhiNhatKyKhiDoiRAM(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "machine_code", "ram_gb", "secure_boot", "hw_baseline_at"}).
			AddRow("m1", "available", "M-001", 16, "on", time.Now().Add(-time.Hour)))

	mock.ExpectBegin()
	mock.ExpectExec(`^UPDATE "machines" SET "booted_at"=\$1,"cpu_temp"=\$2,"cpu_usage"=\$3,"disk_usage"=\$4,"gpu_temp"=\$5,"last_heartbeat"=\$6,"ram_gb"=\$7,"ram_usage"=\$8,"secure_boot"=\$9,"updated_at"=\$10 WHERE`).
		WithArgs(anyTime{}, 0.0, 0.0, 0.0, 0.0, anyTime{}, 32, 0.0, "off", anyTime{}, "m1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// Hai dòng: RAM 16→32 và Secure Boot on→off.
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "machine_hardware_changes"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID).AddRow(testUUID))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "audit_logs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow("a1", time.Now()))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "machine_hardware_snapshots"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	err := svc.Heartbeat("m1", HeartbeatRequest{Uptime: 100, RAMGB: 32, SecureBoot: "off"})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Số đo y hệt số đã lưu: không ghi dòng nhật ký nào (sqlmock báo lỗi nếu có INSERT thừa).
func TestMachineService_Heartbeat_KhongDoiThiKhongGhiNhatKy(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "ram_gb", "boot_mode", "hw_baseline_at"}).
			AddRow("m1", "available", 16, "uefi", time.Now().Add(-time.Hour)))
	mock.ExpectBegin()
	mock.ExpectExec(`^UPDATE "machines" SET`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "audit_logs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow("a1", time.Now()))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "machine_hardware_snapshots"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	assert.NoError(t, svc.Heartbeat("m1", HeartbeatRequest{RAMGB: 16, BootMode: "uefi"}))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_AckHardwareChanges(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectExec(`^UPDATE "machine_hardware_changes" SET "acknowledged_at"=\$1,"acknowledged_by"=\$2 WHERE acknowledged_at IS NULL AND id IN \(\$3,\$4\)`).
		WithArgs(anyTime{}, "staff", "c1", "c2").
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	n, err := svc.AckHardwareChanges([]string{"c1", "c2"}, false, "staff")
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)
	assert.NoError(t, mock.ExpectationsWereMet())

	_, err = svc.AckHardwareChanges(nil, false, "staff")
	assert.Error(t, err, "không chọn gì và không bấm 'tất cả' thì từ chối thay vì xác nhận hết")
}

func TestMachineService_ListHardwareChanges_ChuaXacNhanLenDau(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM machine_hardware_changes AS c LEFT JOIN machines AS m ON m.id = c.machine_id WHERE c.machine_id = \$1 AND c.acknowledged_at IS NULL`).
		WithArgs("m1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT c\.\*, m\.machine_code AS machine_code FROM machine_hardware_changes AS c LEFT JOIN machines AS m ON m\.id = c\.machine_id WHERE c\.machine_id = \$1 AND c\.acknowledged_at IS NULL ORDER BY \(c\.acknowledged_at IS NOT NULL\) asc, c\.detected_at desc LIMIT \$2`).
		WithArgs("m1", 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_id", "component", "machine_code"}).
			AddRow("c1", "m1", "ram", "M-001"))

	res, err := svc.ListHardwareChanges(pagination.Params{Page: 1, PageSize: 20}, HardwareChangeFilter{MachineID: "m1", Unacknowledged: true})
	require.NoError(t, err)
	assert.Equal(t, int64(1), res.Total)
	items := res.Items.([]model.MachineHardwareChange)
	require.Len(t, items, 1)
	assert.Equal(t, "M-001", items[0].MachineCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Bấm sắp xếp cột Máy: mã máy (bảng join) đứng trước, thứ tự mặc định làm tiêu chí phụ.
func TestMachineService_ListHardwareChanges_SapTheoCot(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM machine_hardware_changes AS c LEFT JOIN machines AS m ON m\.id = c\.machine_id`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT c\.\*, m\.machine_code AS machine_code FROM machine_hardware_changes AS c LEFT JOIN machines AS m ON m\.id = c\.machine_id ORDER BY m\.machine_code asc, \(c\.acknowledged_at IS NOT NULL\) asc, c\.detected_at desc LIMIT \$1`).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_id", "component", "machine_code"}).
			AddRow("c1", "m1", "ram", "M-001"))

	params := pagination.Params{Page: 1, PageSize: 20, Sort: "machine_code", Order: "asc", Sorted: true}
	_, err := svc.ListHardwareChanges(params, HardwareChangeFilter{})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHeartbeatRequest_AddActivity(t *testing.T) {
	up := map[string]interface{}{}
	(&HeartbeatRequest{}).addActivity(up)
	assert.Empty(t, up, "máy trạm không khai gì thì không ghi đè số đang có")

	app, down, upBps, ping := "  VALORANT-Win64-Shipping.exe ", int64(-5), int64(99_000_000_000), 700000
	(&HeartbeatRequest{ActiveApp: &app, NetDownBps: &down, NetUpBps: &upBps, PingMS: &ping}).addActivity(up)
	assert.Equal(t, "VALORANT-Win64-Shipping.exe", up["active_app"])
	assert.Equal(t, int64(0), up["net_down_bps"], "số âm kẹp về 0")
	assert.Equal(t, int64(10_000_000_000), up["net_up_bps"], "kẹp trần 10 GB/s")
	assert.Equal(t, 60000, up["ping_ms"])

	// Rỗng là giá trị có nghĩa ("chưa có cửa sổ nào"): vẫn ghi để xoá tên ứng dụng cũ.
	empty := ""
	up2 := map[string]interface{}{}
	(&HeartbeatRequest{ActiveApp: &empty}).addActivity(up2)
	assert.Equal(t, "", up2["active_app"])
}

func TestMachineService_MachineActivity(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT id, machine_code, status, last_heartbeat, active_app, cpu_temp, gpu_temp,\s+cpu_usage, ram_usage, disk_usage, net_down_bps, net_up_bps, ping_ms FROM "machines" WHERE deleted_at IS NULL ORDER BY machine_code asc`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code", "status", "active_app", "net_down_bps", "ping_ms"}).
			AddRow("m1", "M-001", "in_use", "chrome.exe", int64(1250000), 2))

	rows, err := svc.MachineActivity()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "chrome.exe", rows[0].ActiveApp)
	assert.Equal(t, int64(1250000), rows[0].NetDownBps)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Quán chưa có máy: hai bảng phải ra [] chứ không phải null, không thì trang Phần cứng trắng.
func TestMachineService_HardwareListsEmptyNotNull(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`FROM "machines" WHERE deleted_at IS NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code"}))
	mock.ExpectQuery(`FROM machines AS m WHERE m.deleted_at IS NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code"}))

	activity, err := svc.MachineActivity()
	require.NoError(t, err)
	overview, err := svc.HardwareOverview()
	require.NoError(t, err)

	for name, v := range map[string]interface{}{"activity": activity, "overview": overview} {
		b, err := json.Marshal(v)
		require.NoError(t, err)
		assert.Equal(t, "[]", string(b), name)
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}
