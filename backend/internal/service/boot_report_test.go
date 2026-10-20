package service

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin/binding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func soThuc(v float64) *float64 { return &v }

// batLog chuyển log chuẩn vào bộ đệm cho tới hết test.
func batLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	cu, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(cu)
		log.SetFlags(flags)
	})
	return &buf
}

func bootReportMau() *BootReport {
	return &BootReport{
		BootAt:        "2026-10-14T04:39:41Z",
		ClientVersion: "1.0.12",
		ServiceStartS: soThuc(30.2),
		PrefetchDoneS: soThuc(34.5),
		PrefetchMB:    soThuc(374),
		PrefetchMS:    soThuc(4253),
		LogonS:        soThuc(37.4),
		GUIStartS:     soThuc(37.7),
		LockShownS:    soThuc(37.8),
		LockKind:      "native",
		WebviewReadyS: soThuc(38.6),
		DOMReadyS:     soThuc(38.9),
		UnlockS:       soThuc(166),
		Page:          "html 52ms, js 87ms",
	}
}

var bootNow = time.Date(2026, 10, 14, 5, 0, 0, 0, time.UTC)

// Route nhịp tim mở cho cả LAN: chuỗi từ máy trạm không được chèn dòng giả
// vào log máy chủ, và phải cắt theo KÝ TỰ.
func TestBootReport_LamSach_BoKyTuDieuKhienVaCatDoDai(t *testing.T) {
	r := (&BootReport{
		ClientVersion: " 1.0.12\r\n[boot] GIA v9 \x00\u202e",
		Page:          "html 52ms,\n js 87ms\u2028\t" + strings.Repeat("ộ", 300),
	}).lamSach(bootNow)
	require.NotNil(t, r)

	// Phiên bản đứng trần trong dòng log: lạ là bỏ hẳn, không cố làm sạch.
	assert.Equal(t, "", r.ClientVersion)
	for in, want := range map[string]string{
		" 1.0.12 ":              "1.0.12",
		"dev":                   "dev",
		"1.0.12-rc.1+g3f2a":     "1.0.12-rc.1+g3f2a",
		"1.0.12 [boot] MAY2":    "",
		"1.0.12=x":              "",
		strings.Repeat("1", 33): "",
		"1.0.\u0661":            "",
	} {
		r := (&BootReport{ClientVersion: in, ServiceStartS: soThuc(1)}).lamSach(bootNow)
		require.NotNil(t, r, in)
		assert.Equal(t, want, r.ClientVersion, "client_version %q", in)
	}

	assert.True(t, strings.HasPrefix(r.Page, "html 52ms, js 87ms"))
	assert.NotContains(t, r.Page, "\u2028")
	assert.NotContains(t, r.Page, "\t")
	assert.Equal(t, 200, utf8.RuneCountInString(r.Page))
	assert.True(t, utf8.ValidString(r.Page))
}

func TestBootReport_LamSach_LockKindChiNhanNativeHoacWeb(t *testing.T) {
	cases := map[string]string{
		"native":  "native",
		"web":     "web",
		" web ":   "web",
		"Native":  "",
		"curtain": "",
		"":        "",
	}
	for in, want := range cases {
		r := (&BootReport{LockKind: in, LockShownS: soThuc(1)}).lamSach(bootNow)
		require.NotNil(t, r, in)
		assert.Equal(t, want, r.LockKind, "lock_kind %q", in)
	}
}

// Ngoài khoảng là BỎ (coi như chưa biết), không ép về biên; trong khoảng thì
// làm tròn: giây 0,1 — MB và ms về số nguyên.
func TestBootReport_LamSach_KhoangGiaTri(t *testing.T) {
	r := (&BootReport{
		ServiceStartS: soThuc(-0.1),
		PrefetchDoneS: soThuc(86400.5),
		LogonS:        soThuc(37.44),
		GUIStartS:     soThuc(0),
		UnlockS:       soThuc(86400),
		PrefetchMB:    soThuc(374.6),
		PrefetchMS:    soThuc(-1),
	}).lamSach(bootNow)
	require.NotNil(t, r)

	assert.Nil(t, r.ServiceStartS)
	assert.Nil(t, r.PrefetchDoneS)
	assert.Nil(t, r.PrefetchMS)
	require.NotNil(t, r.LogonS)
	assert.Equal(t, 37.4, *r.LogonS)
	require.NotNil(t, r.GUIStartS)
	assert.Equal(t, 0.0, *r.GUIStartS)
	require.NotNil(t, r.UnlockS)
	assert.Equal(t, 86400.0, *r.UnlockS)
	require.NotNil(t, r.PrefetchMB)
	assert.Equal(t, 375.0, *r.PrefetchMB)

	r = (&BootReport{PrefetchMB: soThuc(2 << 20), PrefetchMS: soThuc(86400*1000 + 1)}).lamSach(bootNow)
	assert.Nil(t, r, "chỉ có giá trị ngoài khoảng thì không còn gì để ghi")
}

func TestBootReport_LamSach_BootAt(t *testing.T) {
	cases := map[string]string{
		"2026-10-14T04:39:41Z":           "2026-10-14T04:39:41Z",
		"2026-10-14T11:39:41+07:00":      "2026-10-14T04:39:41Z", // về UTC
		"2026-10-14T04:39:41.987Z":       "2026-10-14T04:39:41Z", // cắt về giây
		"2026-10-07T05:00:00Z":           "2026-10-07T05:00:00Z", // đúng 7 ngày
		"2026-10-14T05:05:00Z":           "2026-10-14T05:05:00Z", // đồng hồ máy trạm nhanh vài phút
		"2026-10-14T06:00:00Z":           "",                     // tương lai xa
		"2026-10-14 04:39:41":            "",
		"hôm qua":                        "",
		"2026-10-14T04:39:41Z\n[boot] X": "",
	}
	for in, want := range cases {
		r := (&BootReport{BootAt: in, ServiceStartS: soThuc(1)}).lamSach(bootNow)
		require.NotNil(t, r, in)
		assert.Equal(t, want, r.BootAt, "boot_at %q", in)
	}

	// Máy chạy quá 7 ngày vẫn gửi boot_at cũ: bỏ cả gói, không ghi một dòng boot=- mới.
	assert.Nil(t, (&BootReport{BootAt: "2026-10-07T04:59:59Z", ServiceStartS: soThuc(1)}).lamSach(bootNow))
}

func TestBootReport_LamSach_RongThiNil(t *testing.T) {
	var r *BootReport
	assert.Nil(t, r.lamSach(bootNow))
	assert.Nil(t, (&BootReport{}).lamSach(bootNow))
	assert.Nil(t, (&BootReport{LockKind: "curtain", Page: " \r\n ", BootAt: "x"}).lamSach(bootNow))
}

func TestBootReport_DongLog_DayDu(t *testing.T) {
	r := bootReportMau().lamSach(bootNow)
	require.NotNil(t, r)
	assert.Equal(t,
		`[boot] MAY1 v1.0.12 boot=2026-10-14T04:39:41Z svc=30.2 prefetch=34.5(374MB/4253ms) logon=37.4 gui=37.7 `+
			`lock=37.8(native) webview=38.6 dom=38.9 unlock=166.0 page="html 52ms, js 87ms"`,
		r.dongLog("MAY1"))
}

func TestBootReport_DongLog_GiaTriThieuLaGach(t *testing.T) {
	r := (&BootReport{ServiceStartS: soThuc(30.2)}).lamSach(bootNow)
	require.NotNil(t, r)
	assert.Equal(t,
		"[boot] MAY1 - boot=- svc=30.2 prefetch=- logon=- gui=- lock=- webview=- dom=- unlock=- page=-",
		r.dongLog("MAY1"))

	// Thiếu một phần trong nhóm thì gạch đúng phần đó.
	r = (&BootReport{PrefetchDoneS: soThuc(34.5), LockKind: "web"}).lamSach(bootNow)
	require.NotNil(t, r)
	assert.Equal(t,
		"[boot] MAY1 - boot=- svc=- prefetch=34.5(-MB/-ms) logon=- gui=- lock=-(web) webview=- dom=- unlock=- page=-",
		r.dongLog("MAY1"))

	// Mã máy đến từ URL — không được mang xuống dòng vào log, cũng không được dựng
	// một đoạn giả của máy khác: ngoài bộ ký tự an toàn thì in trong ngoặc kép.
	assert.True(t, strings.HasPrefix(r.dongLog("MAY1\n[boot] GIA"), `[boot] "MAY1\n[boot] GIA" - `))
	assert.True(t, strings.HasPrefix(r.dongLog("MAY 1 v9 [boot] MAY2"), `[boot] "MAY 1 v9 [boot] MAY2" - `))
	assert.True(t, strings.HasPrefix(r.dongLog("Máy-01"), `[boot] "Máy-01" - `))
	assert.True(t, strings.HasPrefix(r.dongLog("PC_01.lan-2"), `[boot] PC_01.lan-2 - `))
}

// Chuỗi vài MB từ LAN: cắt theo byte trước, kết quả vẫn đúng 200 ký tự UTF-8 hợp lệ
// dù chỗ cắt rơi giữa một ký tự nhiều byte.
func TestBootReport_LamSach_ChuoiRatDai(t *testing.T) {
	for _, dau := range []string{"", "a", "ab", "abc"} {
		r := (&BootReport{Page: dau + strings.Repeat("ộ", 2<<20)}).lamSach(bootNow)
		require.NotNil(t, r)
		assert.Equal(t, 200, utf8.RuneCountInString(r.Page))
		assert.True(t, utf8.ValidString(r.Page))
		assert.True(t, strings.HasPrefix(r.Page, dau+"ộ"))
	}
}

// boot_report sai kiểu không được làm hỏng cả nhịp tim (400): bỏ cả gói, phần còn lại
// của nhịp tim vẫn nhận. Giải mã qua đúng binding của gin mà handler dùng.
func TestBootReport_SaiKieuKhongLamHongNhipTim(t *testing.T) {
	for _, bao := range []string{
		`"x"`, `123`, `[]`, `null`,
		`{"boot_at":123,"service_start_s":30.2}`,
		`{"service_start_s":"30","logon_s":37.4}`,
		`{"prefetch_mb":1e400,"service_start_s":30.2}`,
		`{"lock_kind":["native"],"service_start_s":30.2}`,
	} {
		var req HeartbeatRequest
		body := fmt.Sprintf(`{"cpu_temp":45,"uptime":180,"boot_report":%s}`, bao)
		require.NoError(t, binding.JSON.BindBody([]byte(body), &req), bao)
		assert.Equal(t, 45.0, req.CPUTemp, bao)
		assert.Nil(t, req.BootReport.lamSach(bootNow), bao)
	}

	var req HeartbeatRequest
	require.NoError(t, binding.JSON.BindBody([]byte(`{"boot_report":{"service_start_s":30.2,"lock_kind":"native"}}`), &req))
	require.NotNil(t, req.BootReport)
	require.NotNil(t, req.BootReport.ServiceStartS)
	assert.Equal(t, 30.2, *req.BootReport.ServiceStartS)
	assert.Equal(t, "native", req.BootReport.LockKind)
}

// Máy trạm gửi lại cả gói mỗi nhịp tim: chỉ ghi khi dòng đổi, và theo từng máy.
func TestMachineService_GhiBootReport_ChiGhiKhiDoi(t *testing.T) {
	buf := batLog(t)
	svc := NewMachineService(nil, nil, nil)
	soDong := func() int { return strings.Count(buf.String(), "[boot] ") }

	r := bootReportMau()
	r.UnlockS = nil
	svc.ghiBootReport("MAY1", r)
	svc.ghiBootReport("MAY1", r)
	// Khác ở dạng thô nhưng y hệt sau khi làm sạch.
	r2 := bootReportMau()
	r2.UnlockS = nil
	r2.LogonS = soThuc(37.41)
	r2.Page = " html 52ms, js 87ms\r\n"
	svc.ghiBootReport("MAY1", r2)
	assert.Equal(t, 1, soDong(), "gói y hệt sau khi làm sạch không ghi lại")

	svc.ghiBootReport("MAY1", bootReportMau())
	assert.Equal(t, 2, soDong(), "mở khoá xong là gói đổi, ghi một dòng mới")
	assert.Contains(t, buf.String(), "unlock=166.0")

	svc.ghiBootReport("MAY2", bootReportMau())
	assert.Equal(t, 3, soDong(), "máy khác có dòng riêng")
	svc.ghiBootReport("MAY1", bootReportMau())
	assert.Equal(t, 3, soDong())

	svc.ghiBootReport("MAY1", nil)
	svc.ghiBootReport("MAY1", &BootReport{LockKind: "x"})
	assert.Equal(t, 3, soDong(), "gói rỗng không ghi gì")
	assert.Len(t, svc.bootDaGhi, 2)
}

// Bản đồ dòng đã ghi có trần: mã máy tự khai được qua route mở và xoá máy không xoá
// dòng của nó. Đầy thì xoá trắng; máy cũ chỉ ghi lại một dòng.
func TestMachineService_GhiBootReport_BanDoCoTran(t *testing.T) {
	buf := batLog(t)
	svc := NewMachineService(nil, nil, nil)
	svc.bootDaGhi = make(map[string]string, bootMayToiDa)
	for i := 0; i < bootMayToiDa; i++ {
		svc.bootDaGhi[fmt.Sprintf("RAC%d", i)] = "x"
	}
	svc.ghiBootReport("MAY1", bootReportMau())
	assert.Len(t, svc.bootDaGhi, 1)
	assert.Equal(t, 1, strings.Count(buf.String(), "[boot] MAY1 "))

	svc.ghiBootReport("MAY1", bootReportMau())
	assert.Equal(t, 1, strings.Count(buf.String(), "[boot] MAY1 "))
}

// boot_report chỉ ghi log: câu UPDATE của nhịp tim phải y nguyên như khi không có nó.
func TestMachineService_Heartbeat_BootReportKhongDoiSQL(t *testing.T) {
	buf := batLog(t)
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code", "status"}).AddRow("m1", "MAY1", "available"))

	mock.ExpectBegin()
	mock.ExpectExec(`^UPDATE "machines" SET "booted_at"=\$1,"cpu_temp"=\$2,"cpu_usage"=\$3,"disk_usage"=\$4,"gpu_temp"=\$5,"last_heartbeat"=\$6,"ram_usage"=\$7,"updated_at"=\$8 WHERE`).
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

	r := bootReportMau()
	r.BootAt = time.Now().Add(-3 * time.Minute).UTC().Format(time.RFC3339)
	err := svc.Heartbeat("m1", HeartbeatRequest{CPUTemp: 45, GPUTemp: 60, Uptime: 180, BootReport: r})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())

	assert.Equal(t, 1, strings.Count(buf.String(), "[boot] "))
	assert.Contains(t, buf.String(), "[boot] MAY1 v1.0.12 boot="+r.BootAt+" svc=30.2 ")
}
