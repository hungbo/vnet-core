package main

import (
	"testing"
	"time"
)

func TestNormalizePolicy(t *testing.T) {
	if got := normalizePolicy(clientPolicy{}); got != (clientPolicy{TamperAction: "restart", OfflineLockSeconds: 30}) {
		t.Errorf("chính sách rỗng = %+v", got)
	}
	got := normalizePolicy(clientPolicy{TamperAction: "shutdown", OfflineLockSeconds: 60, OfflineRebootSeconds: 70})
	if got.TamperAction != "shutdown" || got.OfflineRebootSeconds != 120 {
		t.Errorf("khởi động lại phải cách bậc khoá ít nhất 60 giây, nhận %+v", got)
	}
	if normalizePolicy(clientPolicy{TamperAction: "gì đó"}).TamperAction != "restart" {
		t.Error("hành động lạ phải về restart")
	}
}

var t0 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func sec(n int) time.Time { return t0.Add(time.Duration(n) * time.Second) }

// Giao diện báo cáo đều thì không làm gì cả.
func TestSupervisor_GiaoDienBaoDeuThiYen(t *testing.T) {
	s := newUISupervisor(t0)
	pol := defaultClientPolicy()
	s.noteUIRunning(t0, true)
	for i := 2; i <= 120; i += 2 {
		s.noteStatus(sec(i), uiStatus{Member: true})
		s.noteServerOK(sec(i))
		if a := s.decide(sec(i), pol, false); a != actNone {
			t.Fatalf("giây %d: hành động %v, mong không làm gì", i, a)
		}
	}
}

// Giao diện im quá 15 giây thì bật lại; lần thứ ba trong năm phút thì khởi động lại.
func TestSupervisor_GiaoDienTreoThiBatLaiRoiKhoiDongLai(t *testing.T) {
	s := newUISupervisor(t0)
	pol := defaultClientPolicy()
	now := 0
	for lan := 1; lan <= 3; lan++ {
		s.noteUIRunning(sec(now), true)
		s.noteStatus(sec(now), uiStatus{Member: true})
		s.noteServerOK(sec(now))
		if a := s.decide(sec(now+10), pol, false); a != actNone {
			t.Fatalf("lần %d: 10 giây im lặng chưa đủ để kết luận, nhận %v", lan, a)
		}
		a := s.decide(sec(now+20), pol, false)
		want := actRestartUI
		if lan == 3 {
			want = actReboot
		}
		if a != want {
			t.Fatalf("lần %d: nhận %v, mong %v", lan, a, want)
		}
		now += 40
	}
}

// Giao diện vừa bật chưa kịp báo cáo: có 30 giây để dựng cửa sổ.
func TestSupervisor_VuaBatThiChoDungCuaSo(t *testing.T) {
	s := newUISupervisor(t0)
	s.noteUIRunning(t0, true)
	if a := s.decide(sec(25), defaultClientPolicy(), false); a != actNone {
		t.Fatalf("25 giây đầu: nhận %v", a)
	}
	if a := s.decide(sec(35), defaultClientPolicy(), false); a != actRestartUI {
		t.Fatalf("35 giây không một báo cáo: nhận %v, mong bật lại", a)
	}
}

// Không có giao diện (chưa ai đăng nhập Windows) thì không có gì để chấm.
func TestSupervisor_KhongCoGiaoDienThiYen(t *testing.T) {
	s := newUISupervisor(t0)
	s.noteUIRunning(t0, false)
	if a := s.decide(sec(3600), defaultClientPolicy(), false); a != actNone {
		t.Fatalf("nhận %v", a)
	}
}

// Phiên kết thúc: giao diện về màn hình khoá trong 10 giây thì yên, không thì bật lại.
func TestSupervisor_PhienKetThucPhaiVeManHinhKhoa(t *testing.T) {
	pol := defaultClientPolicy()

	ngoan := newUISupervisor(t0)
	ngoan.noteUIRunning(t0, true)
	ngoan.noteStatus(sec(1), uiStatus{Member: true})
	ngoan.noteSessionEnded(sec(2))
	ngoan.noteStatus(sec(4), uiStatus{Locked: true})
	// Khách đăng nhập lại ngay: giao diện mở khoá hợp lệ, không được phạt.
	ngoan.noteStatus(sec(8), uiStatus{Member: true})
	ngoan.noteStatus(sec(13), uiStatus{Member: true})
	if a := ngoan.decide(sec(13), pol, false); a != actNone {
		t.Fatalf("đã khoá rồi đăng nhập lại: nhận %v", a)
	}

	li := newUISupervisor(t0)
	li.noteUIRunning(t0, true)
	li.noteStatus(sec(1), uiStatus{Member: true})
	li.noteSessionEnded(sec(2))
	for i := 3; i <= 13; i += 2 {
		li.noteStatus(sec(i), uiStatus{Member: true})
	}
	if a := li.decide(sec(13), pol, false); a != actRestartUI {
		t.Fatalf("không về màn hình khoá: nhận %v, mong bật lại giao diện", a)
	}
}

// Nhân viên đang đăng nhập thì phiên của máy kết thúc không liên quan tới họ.
func TestSupervisor_NhanVienThiKhongBatLai(t *testing.T) {
	s := newUISupervisor(t0)
	s.noteUIRunning(t0, true)
	s.noteStatus(sec(1), uiStatus{Admin: true})
	s.noteSessionEnded(sec(2))
	s.noteStatus(sec(13), uiStatus{Admin: true})
	if a := s.decide(sec(13), defaultClientPolicy(), false); a != actNone {
		t.Fatalf("nhận %v", a)
	}
}

// Mất kết nối: khoá ở giây 60, khởi động lại ở giây 300 nếu có hội viên.
func TestSupervisor_MatKetNoi(t *testing.T) {
	pol := defaultClientPolicy()
	s := newUISupervisor(t0)
	s.noteUIRunning(t0, true)

	baoCao := func(n int, st uiStatus) superviseAction {
		s.noteStatus(sec(n), st)
		return s.decide(sec(n), pol, false)
	}
	if s.offlineLock(sec(59), pol) {
		t.Error("59 giây: chưa được khoá")
	}
	if !s.offlineLock(sec(60), pol) {
		t.Error("60 giây: phải khoá")
	}
	if a := baoCao(298, uiStatus{Member: true, Locked: true}); a != actNone {
		t.Fatalf("298 giây: nhận %v", a)
	}
	if a := baoCao(300, uiStatus{Member: true, Locked: true}); a != actReboot {
		t.Fatalf("300 giây có hội viên: nhận %v, mong khởi động lại", a)
	}
	// Đã ra lệnh rồi thì không ra lệnh lần nữa.
	if a := baoCao(302, uiStatus{Member: true, Locked: true}); a != actNone {
		t.Fatalf("sau lệnh khởi động lại: nhận %v", a)
	}

	// Có mạng lại trước ngưỡng thì hết khoá.
	s2 := newUISupervisor(t0)
	s2.noteServerOK(sec(90))
	if s2.offlineLock(sec(100), pol) {
		t.Error("vừa nói chuyện được với máy chủ mà vẫn khoá")
	}
}

// Những trường hợp mất kết nối KHÔNG được khởi động lại.
func TestSupervisor_MatKetNoiKhongKhoiDongLaiOan(t *testing.T) {
	pol := defaultClientPolicy()
	cases := map[string]struct {
		st        uiStatus
		maintFlag bool
		pol       clientPolicy
	}{
		"không ai đăng nhập":    {st: uiStatus{Locked: true}, pol: pol},
		"nhân viên":             {st: uiStatus{Admin: true}, pol: pol},
		"bảo trì bằng PIN":      {st: uiStatus{Member: true, Maint: true}, pol: pol},
		"tệp maintenance.flag":  {st: uiStatus{Member: true}, maintFlag: true, pol: pol},
		"chính sách tắt reboot": {st: uiStatus{Member: true}, pol: clientPolicy{TamperAction: "restart", OfflineLockSeconds: 60}},
	}
	for ten, c := range cases {
		s := newUISupervisor(t0)
		s.noteUIRunning(t0, true)
		s.noteStatus(sec(3600), c.st)
		if a := s.decide(sec(3600), c.pol, c.maintFlag); a != actNone {
			t.Errorf("%s: nhận %v, mong không làm gì", ten, a)
		}
	}
}

// Màn hình khoá không ai đăng nhập đủ số phút thì tắt máy, đúng một lần.
func TestSupervisor_KhongAiDangNhapThiTatMay(t *testing.T) {
	pol := defaultClientPolicy()
	pol.IdleShutdownMinutes = 5
	s := newUISupervisor(t0)
	s.noteUIRunning(t0, true)
	for i := 2; i < 300; i += 2 {
		s.noteStatus(sec(i), uiStatus{Locked: true})
		s.noteServerOK(sec(i))
		if a := s.decide(sec(i), pol, false); a != actNone {
			t.Fatalf("giây %d: chưa đủ 5 phút mà đã %v", i, a)
		}
	}
	s.noteStatus(sec(302), uiStatus{Locked: true})
	s.noteServerOK(sec(302))
	if a := s.decide(sec(302), pol, false); a != actShutdown {
		t.Fatalf("đủ 5 phút không ai đăng nhập: nhận %v, mong tắt máy", a)
	}
	if a := s.decide(sec(304), pol, false); a != actNone {
		t.Fatalf("đã ra lệnh tắt rồi thì không ra lần nữa, nhận %v", a)
	}
}

// Có người đăng nhập rồi thoát thì đồng hồ đếm lại từ lúc thoát, không cộng dồn.
func TestSupervisor_DangNhapThiDemLai(t *testing.T) {
	pol := defaultClientPolicy()
	pol.IdleShutdownMinutes = 5
	s := newUISupervisor(t0)
	s.noteUIRunning(t0, true)
	s.noteStatus(sec(0), uiStatus{Locked: true})
	s.noteStatus(sec(240), uiStatus{Member: true}) // 4 phút thì có khách vào
	s.noteStatus(sec(600), uiStatus{Locked: true}) // khách thoát
	s.noteServerOK(sec(800))
	s.noteStatus(sec(800), uiStatus{Locked: true})
	if a := s.decide(sec(800), pol, false); a != actNone {
		t.Fatalf("mới thoát hơn 3 phút mà đã %v", a)
	}
	s.noteServerOK(sec(900))
	s.noteStatus(sec(900), uiStatus{Locked: true})
	if a := s.decide(sec(900), pol, false); a != actShutdown {
		t.Fatalf("thoát được 5 phút: nhận %v, mong tắt máy", a)
	}
}

// Những trường hợp KHÔNG được tự tắt máy dù đã quá giờ.
func TestSupervisor_KhongTatMayOan(t *testing.T) {
	pol := defaultClientPolicy()
	pol.IdleShutdownMinutes = 5
	cases := map[string]struct {
		st        uiStatus
		maintFlag bool
		pol       clientPolicy
	}{
		"hội viên đang chơi":      {st: uiStatus{Member: true}, pol: pol},
		"khoá từ quầy, còn khách": {st: uiStatus{Locked: true, Member: true}, pol: pol},
		"nhân viên":               {st: uiStatus{Locked: true, Admin: true}, pol: pol},
		"bảo trì bằng PIN":        {st: uiStatus{Locked: true, Maint: true}, pol: pol},
		"tệp maintenance.flag":    {st: uiStatus{Locked: true}, maintFlag: true, pol: pol},
		"chính sách tắt (0)":      {st: uiStatus{Locked: true}, pol: defaultClientPolicy()},
	}
	for ten, c := range cases {
		s := newUISupervisor(t0)
		s.noteUIRunning(t0, true)
		for i := 2; i <= 3600; i += 2 {
			s.noteStatus(sec(i), c.st)
			s.noteServerOK(sec(i))
			if a := s.decide(sec(i), c.pol, c.maintFlag); a != actNone {
				t.Errorf("%s: giây %d nhận %v, mong không làm gì", ten, i, a)
				break
			}
		}
	}
}

// Giao diện chết thì không biết có ai ngồi máy hay không: quên đồng hồ đi.
func TestSupervisor_GiaoDienChetThiQuenDongHo(t *testing.T) {
	pol := defaultClientPolicy()
	pol.IdleShutdownMinutes = 5
	s := newUISupervisor(t0)
	s.noteUIRunning(t0, true)
	s.noteStatus(sec(0), uiStatus{Locked: true})
	s.noteUIRunning(sec(200), false)
	s.noteUIRunning(sec(210), true)
	s.noteServerOK(sec(320))
	s.noteStatus(sec(320), uiStatus{Locked: true})
	if a := s.decide(sec(320), pol, false); a != actNone {
		t.Fatalf("đồng hồ phải tính lại từ lần báo cáo đầu sau khi giao diện lên lại, nhận %v", a)
	}
}
