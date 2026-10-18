package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func giay(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }

func soNguyen(v int64) *int64 { return &v }

// batMay là lúc nhân khởi động trong các bài kiểm; lucMay(s) là giờ hệ thống khi máy đã
// chạy s giây.
var batMay = time.Date(2026, 10, 14, 4, 39, 41, 300e6, time.UTC)

func lucMay(s float64) time.Time { return batMay.Add(giay(s)) }

func TestGiayMayChayLamTron(t *testing.T) {
	for _, c := range []struct {
		up   time.Duration
		want float64
	}{
		{37749 * time.Millisecond, 37.7},
		{37750 * time.Millisecond, 37.8},
		{166 * time.Second, 166},
		{30*time.Second + 249*time.Millisecond, 30.2},
		{0, 0},
	} {
		if got := giayMayChay(c.up); got != c.want {
			t.Errorf("giayMayChay(%v) = %v; mong %v", c.up, got, c.want)
		}
	}
}

// Mỗi mốc ghi một lần: lần gọi sau (giao diện bật lại, OnDomReady chạy lại) không đè.
func TestBootGhiMotLan(t *testing.T) {
	var b bootRecorder
	b.dichVuBatDau("", lucMay(30.2), giay(30.2))
	b.dichVuBatDau("", lucMay(200), giay(200))
	b.docTruocXong(giay(34.5), 374, 4253)
	b.docTruocXong(giay(600), 1, 1)
	b.coPhienWindows(giay(37.4))
	b.coPhienWindows(giay(700))
	b.giaoDienBatDau(-time.Second) // không biết: không chiếm chỗ của mốc thật
	b.giaoDienBatDau(giay(37.7))
	b.giaoDienBatDau(giay(800))
	b.khoaNativeHien(giay(37.8))
	b.webViewSan(giay(38.6))
	b.webViewSan(giay(900))
	b.domSan(giay(38.9)) // màn khoá native đã hiện: không thành "web"
	b.domSan(giay(950))
	b.moKhoa(giay(166))
	b.moKhoa(giay(1000))
	b.trang("html 52ms")
	b.trang("html 1ms")

	want := bootReport{
		BootAt: "2026-10-14T04:39:41Z", ClientVersion: version,
		ServiceStartS: 30.2, PrefetchDoneS: 34.5, PrefetchMB: soNguyen(374), PrefetchMS: soNguyen(4253), LogonS: 37.4,
		bootGUI: bootGUI{GUIStartS: 37.7, LockShownS: 37.8, LockKind: "native",
			WebviewReadyS: 38.6, DomReadyS: 38.9, UnlockS: 166, Page: "html 52ms"},
	}
	if !reflect.DeepEqual(b.r, want) {
		t.Errorf("báo cáo = %+v\nmong     %+v", b.r, want)
	}
}

// Không có màn khoá native nào hiện trước DOM sẵn sàng: màn hình khoá đầu tiên là web.
// Màn khoá native hiện muộn hơn thế (không xảy ra, Startup đã thôi chờ) cũng không đè.
func TestBootKhoaWeb(t *testing.T) {
	var b bootRecorder
	b.domSan(giay(50.3))
	b.khoaNativeHien(giay(51))
	if g := b.phanGiaoDien(); g.LockShownS != 50.3 || g.LockKind != "web" {
		t.Errorf("khoá = %v %q; mong 50.3 web", g.LockShownS, g.LockKind)
	}
}

// Dịch vụ nền gộp phần của giao diện: trường đã có thì giữ, trường còn thiếu thì nhận.
func TestBootGopGiaoDien(t *testing.T) {
	var b bootRecorder
	b.coPhienWindows(giay(37.4))
	// Loại màn khoá lạ không được vào nhịp tim, và không chiếm chỗ của mốc thật.
	b.gopGiaoDien(bootGUI{LockShownS: 37.5, LockKind: "curtain\n[boot] GIA"})
	// Giao diện đầu tiên chết trước DOM sẵn sàng.
	b.gopGiaoDien(bootGUI{GUIStartS: 37.7, LockShownS: 37.8, LockKind: "native", WebviewReadyS: 38.6})
	// Giao diện bật lại gửi mốc của NÓ.
	b.gopGiaoDien(bootGUI{GUIStartS: 60, LockShownS: 61.5, LockKind: "web", WebviewReadyS: 61,
		DomReadyS: 61.5, Page: "html 40ms"})
	// Lượt báo cáo kế của cùng giao diện đó, đã mở khoá.
	b.gopGiaoDien(bootGUI{GUIStartS: 60, LockShownS: 61.5, LockKind: "web", WebviewReadyS: 61,
		DomReadyS: 61.5, UnlockS: 90, Page: "html 40ms"})

	want := bootReport{LogonS: 37.4, bootGUI: bootGUI{
		GUIStartS: 37.7, LockShownS: 37.8, LockKind: "native", WebviewReadyS: 38.6,
		DomReadyS: 61.5, UnlockS: 90, Page: "html 40ms"}}
	if !reflect.DeepEqual(b.r, want) {
		t.Errorf("báo cáo = %+v\nmong     %+v", b.r, want)
	}
}

func TestBootCatTrang(t *testing.T) {
	dai := strings.Repeat("vẽ đầu ", 40) // 280 ký tự, nhiều byte mỗi ký tự
	var gui bootRecorder
	gui.trang(dai)
	g := gui.phanGiaoDien()
	if n := utf8.RuneCountInString(g.Page); n != bootPageMax || !utf8.ValidString(g.Page) {
		t.Errorf("trang dài %d ký tự (UTF-8 hợp lệ: %v); mong %d", n, utf8.ValidString(g.Page), bootPageMax)
	}
	// Dịch vụ cũng cắt phần nhận qua kênh, không tin độ dài của bên gửi.
	var svc bootRecorder
	svc.gopGiaoDien(bootGUI{Page: dai})
	if n := utf8.RuneCountInString(svc.r.Page); n != bootPageMax {
		t.Errorf("dịch vụ giữ trang dài %d ký tự; mong %d", n, bootPageMax)
	}
}

// Hình dạng JSON phải khớp đúng hợp đồng với máy chủ (service.BootReport).
func TestBootReportJSON(t *testing.T) {
	r := bootReport{
		BootAt: "2026-10-14T04:39:41Z", ClientVersion: "1.0.12",
		ServiceStartS: 30.2, PrefetchDoneS: 34.5, PrefetchMB: soNguyen(374), PrefetchMS: soNguyen(4253), LogonS: 37.4,
		bootGUI: bootGUI{GUIStartS: 37.7, LockShownS: 37.8, LockKind: "native",
			WebviewReadyS: 38.6, DomReadyS: 38.9, UnlockS: 166, Page: "html 52ms, js 87ms"},
	}
	out, _ := json.Marshal(TelemetryPayload{BootReport: &r})
	var got struct {
		BootReport map[string]any `json:"boot_report"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"boot_at": "2026-10-14T04:39:41Z", "client_version": "1.0.12",
		"service_start_s": 30.2, "prefetch_done_s": 34.5, "prefetch_mb": 374.0, "prefetch_ms": 4253.0,
		"logon_s": 37.4, "gui_start_s": 37.7, "lock_shown_s": 37.8, "lock_kind": "native",
		"webview_ready_s": 38.6, "dom_ready_s": 38.9, "unlock_s": 166.0, "page": "html 52ms, js 87ms",
	}
	if !reflect.DeepEqual(got.BootReport, want) {
		t.Errorf("boot_report = %v\nmong          %v", got.BootReport, want)
	}

	// Mốc chưa biết thì không có trong JSON; không có báo cáo thì không có boot_report.
	out, _ = json.Marshal(bootReport{BootAt: "2026-10-14T04:39:41Z", ClientVersion: "dev", ServiceStartS: 30.2})
	if s := string(out); s != `{"boot_at":"2026-10-14T04:39:41Z","client_version":"dev","service_start_s":30.2}` {
		t.Errorf("báo cáo thiếu mốc = %s", s)
	}
	out, _ = json.Marshal(TelemetryPayload{})
	if strings.Contains(string(out), "boot_report") {
		t.Errorf("nhịp tim không báo cáo vẫn có boot_report: %s", out)
	}

	// Đọc trước chạy mà không tới 1 MB / 1 ms: số 0 là số đo, vẫn gửi.
	var b bootRecorder
	b.docTruocXong(giay(31), 0, 0)
	out, _ = json.Marshal(b.r)
	if s := string(out); s != `{"prefetch_done_s":31,"prefetch_mb":0,"prefetch_ms":0}` {
		t.Errorf("đọc trước 0 MB = %s", s)
	}
}

// Báo cáo trên kênh giao diện → dịch vụ: các trường cũ giữ nguyên chỗ, "boot" là một
// object lồng bên trong, và dịch vụ đọc lại được.
func TestUIStatusBootJSON(t *testing.T) {
	st := uiStatus{Locked: true, Boot: &bootGUI{GUIStartS: 37.7, LockShownS: 37.8, LockKind: "native"}}
	out, _ := json.Marshal(st)
	want := `{"locked":true,"member":false,"admin":false,"maint":false,` +
		`"boot":{"gui_start_s":37.7,"lock_shown_s":37.8,"lock_kind":"native"}}`
	if string(out) != want {
		t.Errorf("uiStatus = %s\nmong       %s", out, want)
	}
	var lai uiStatus
	if err := json.Unmarshal(out, &lai); err != nil || lai.Boot == nil || *lai.Boot != *st.Boot || !lai.Locked {
		t.Errorf("đọc lại = %+v, %v", lai, err)
	}
	// Giao diện bản cũ không gửi "boot".
	var cu uiStatus
	if err := json.Unmarshal([]byte(`{"locked":true}`), &cu); err != nil || cu.Boot != nil {
		t.Errorf("báo cáo không có boot: %+v, %v", cu, err)
	}
}

func TestBootBaoCao(t *testing.T) {
	var b bootRecorder
	b.dichVuBatDau("", lucMay(30.2), giay(30.2))

	// Trang chưa nạp xong và máy mới chạy chưa tới curtainMaxUptime: chưa gửi.
	if r := b.baoCao(lucMay(60), giay(60)); r != nil {
		t.Fatalf("gửi sớm: %+v", r)
	}
	b.gopGiaoDien(bootGUI{GUIStartS: 37.7, DomReadyS: 38.9, Page: "html 52ms"})
	r := b.baoCao(lucMay(60), giay(60))
	if r == nil {
		t.Fatal("trang đã nạp xong mà chưa gửi")
	}
	if r.BootAt != "2026-10-14T04:39:41Z" || r.ClientVersion != version || r.ServiceStartS != 30.2 || r.Page != "html 52ms" {
		t.Errorf("báo cáo = %+v", r)
	}

	// boot_at giữ nguyên qua dao động dưới giây (đồng hồ hệ thống và uptime lệch nhau
	// vài mili giây, cắt tới giây thì nhảy qua lại) và qua độ trôi vài giây mỗi ngày.
	if r := b.baoCao(lucMay(75.4).Add(800*time.Millisecond), giay(75.4)); r.BootAt != "2026-10-14T04:39:41Z" {
		t.Errorf("boot_at nhảy theo dao động: %s", r.BootAt)
	}
	if r := b.baoCao(lucMay(43200).Add(-4*time.Second), giay(43200)); r.BootAt != "2026-10-14T04:39:41Z" {
		t.Errorf("boot_at nhảy theo độ trôi đồng hồ: %s", r.BootAt)
	}
	// Đồng hồ máy bị chỉnh hẳn: tính lại.
	if r := b.baoCao(lucMay(90).Add(time.Minute), giay(90)); r.BootAt != "2026-10-14T04:40:41Z" {
		t.Errorf("boot_at không theo đồng hồ đã chỉnh: %s", r.BootAt)
	}

	// Giao diện không lên được: quá curtainMaxUptime thì gửi những gì đã có.
	var hong bootRecorder
	hong.dichVuBatDau("", lucMay(30.2), giay(30.2))
	if r := hong.baoCao(lucMay(300), curtainMaxUptime); r == nil || r.ServiceStartS != 30.2 {
		t.Errorf("máy chạy quá %v mà chưa gửi: %+v", curtainMaxUptime, r)
	}
}

// bootLanDau: dịch vụ đầu tiên của lần bật máy ghi đủ mốc và ghi tệp ở nhịp tim.
func bootLanDau(t *testing.T, tep string) *bootReport {
	t.Helper()
	var a bootRecorder
	a.dichVuBatDau(tep, lucMay(30.2), giay(30.2))
	a.docTruocXong(giay(34.5), 374, 4253)
	a.coPhienWindows(giay(37.4))
	a.gopGiaoDien(bootGUI{GUIStartS: 37.7, LockShownS: 37.8, LockKind: "native",
		WebviewReadyS: 38.6, DomReadyS: 38.9, Page: "html 52ms"})
	r := a.baoCao(lucMay(45), giay(45))
	if r == nil {
		t.Fatal("dịch vụ đầu tiên không báo cáo")
	}
	return r
}

// Dịch vụ khởi động lại giữa lần bật máy (tự cập nhật ở giây thứ 90): tiến trình mới nạp
// báo cáo của lần bật máy này từ tệp, mọi mốc nó tự đo lại đều bị bỏ, và nhịp tim gửi
// đúng báo cáo cũ — máy chủ không có thêm dòng nào.
func TestBootQuaLanDichVuKhoiDongLai(t *testing.T) {
	cu := version
	t.Cleanup(func() { version = cu })
	tep := filepath.Join(t.TempDir(), "boot.json")

	version = "1.0.12"
	truoc := bootLanDau(t, tep)

	version = "1.0.13"
	var b bootRecorder
	b.dichVuBatDau(tep, lucMay(95.6).Add(700*time.Millisecond), giay(95.6)) // lệch giây khi cắt
	b.docTruocXong(giay(99), 370, 3000)
	b.coPhienWindows(giay(101))
	b.gopGiaoDien(bootGUI{GUIStartS: 101.2, LockShownS: 101.3, LockKind: "native",
		WebviewReadyS: 102, DomReadyS: 102.3, Page: "html 60ms"})
	b.gopGiaoDien(bootGUI{UnlockS: 166}) // mốc chưa có thì vẫn nhận
	sau := b.baoCao(lucMay(166.4), giay(166.4))

	want := *truoc
	want.UnlockS = 166
	if sau == nil || !reflect.DeepEqual(*sau, want) {
		t.Errorf("sau khi dịch vụ khởi động lại = %+v\nmong                         %+v", sau, &want)
	}

	// Tệp đã có cả mốc mở khoá: một lần khởi động lại nữa vẫn ra đúng báo cáo đó.
	var c bootRecorder
	c.dichVuBatDau(tep, lucMay(700), giay(700))
	if r := c.baoCao(lucMay(715), giay(715)); r == nil || !reflect.DeepEqual(*r, want) {
		t.Errorf("lần khởi động lại thứ hai = %+v", r)
	}
}

// Tệp của lần bật máy TRƯỚC (máy có ổ cứng giữ tệp qua lần khởi động lại) thì bỏ qua.
func TestBootTepLanBatTruoc(t *testing.T) {
	tep := filepath.Join(t.TempDir(), "boot.json")
	bootLanDau(t, tep)

	var b bootRecorder
	sau := batMay.Add(10 * time.Minute) // lần bật máy sau, 10 phút sau lần trước
	b.dichVuBatDau(tep, sau.Add(giay(28.1)), giay(28.1))
	if b.r.ServiceStartS != 28.1 || b.r.PrefetchDoneS != 0 || b.r.GUIStartS != 0 || b.r.BootAt != "2026-10-14T04:49:41Z" {
		t.Errorf("nạp nhầm tệp của lần bật trước: %+v", b.r)
	}

	// Tệp hỏng cũng như không có.
	if err := os.WriteFile(tep, []byte("{hỏng"), 0o644); err != nil {
		t.Fatal(err)
	}
	var c bootRecorder
	c.dichVuBatDau(tep, lucMay(30.2), giay(30.2))
	if c.r.ServiceStartS != 30.2 || c.tat {
		t.Errorf("tệp hỏng: %+v tat=%v", c.r, c.tat)
	}
}

// Dịch vụ bật quá curtainMaxUptime mà không có báo cáo nào của lần bật máy này: những gì
// nó đo được không phải một lần khởi động, nên không gửi gì và không ghi tệp.
func TestBootBatMuonKhongCoTep(t *testing.T) {
	tep := filepath.Join(t.TempDir(), "boot.json")
	var b bootRecorder
	b.dichVuBatDau(tep, lucMay(3600), giay(3600))
	b.gopGiaoDien(bootGUI{GUIStartS: 3601, DomReadyS: 3602, Page: "html 50ms"})
	if r := b.baoCao(lucMay(3615), giay(3615)); r != nil {
		t.Errorf("dịch vụ bật muộn vẫn báo cáo: %+v", r)
	}
	if _, err := os.Stat(tep); !os.IsNotExist(err) {
		t.Errorf("dịch vụ bật muộn vẫn ghi tệp: %v", err)
	}

	// Có tệp của chính lần bật máy này thì bật muộn mấy cũng vẫn báo cáo.
	truoc := bootLanDau(t, tep)
	var c bootRecorder
	c.dichVuBatDau(tep, lucMay(3600), giay(3600))
	if r := c.baoCao(lucMay(3615), giay(3615)); r == nil || !reflect.DeepEqual(*r, *truoc) {
		t.Errorf("bật muộn có tệp = %+v", r)
	}
}

// Tệp chỉ ghi lại khi báo cáo đổi, không phải mỗi nhịp tim.
func TestBootGhiTepKhiDoi(t *testing.T) {
	tep := filepath.Join(t.TempDir(), "boot.json")
	bootLanDau(t, tep)
	var b bootRecorder
	b.dichVuBatDau(tep, lucMay(95), giay(95))
	if b.doi {
		t.Error("nạp lại tệp mà không đổi gì vẫn đánh dấu cần ghi")
	}
	if err := os.Remove(tep); err != nil {
		t.Fatal(err)
	}
	b.baoCao(lucMay(110), giay(110))
	if _, err := os.Stat(tep); !os.IsNotExist(err) {
		t.Errorf("báo cáo không đổi mà vẫn ghi tệp: %v", err)
	}
	b.moKhoa(giay(120))
	b.baoCao(lucMay(125), giay(125))
	if _, err := os.Stat(tep); err != nil {
		t.Errorf("báo cáo đổi mà không ghi tệp: %v", err)
	}
}
