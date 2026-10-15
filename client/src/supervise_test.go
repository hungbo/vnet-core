package main

import (
	"testing"
	"time"
)

func TestNormalizePolicy(t *testing.T) {
	if got := normalizePolicy(clientPolicy{}); got != (clientPolicy{TamperAction: "restart", OfflineLockSeconds: 30, IdleAction: "shutdown"}) {
		t.Errorf("chính sách rỗng = %+v", got)
	}
	got := normalizePolicy(clientPolicy{TamperAction: "shutdown", OfflineLockSeconds: 60, OfflineRebootSeconds: 70})
	if got.TamperAction != "shutdown" || got.OfflineRebootSeconds != 120 {
		t.Errorf("khởi động lại phải cách bậc khoá ít nhất 60 giây, nhận %+v", got)
	}
	if normalizePolicy(clientPolicy{TamperAction: "gì đó"}).TamperAction != "restart" {
		t.Error("hành động lạ phải về restart")
	}
	if normalizePolicy(clientPolicy{IdleAction: "hibernate"}).IdleAction != "shutdown" {
		t.Error("hành động khi rảnh lạ phải về tắt máy")
	}
	if normalizePolicy(clientPolicy{IdleAction: "standby"}).IdleAction != "standby" {
		t.Error("standby phải giữ nguyên")
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

// Ngủ chờ: máy chưa ai đăng nhập từ lúc bật thì cho ngủ, không khởi động lại; thức
// dậy mà vẫn không ai đăng nhập thì đủ thời gian nữa mới ngủ lại.
func TestSupervisor_NguChoMayChuaAiDung(t *testing.T) {
	pol := defaultClientPolicy()
	pol.IdleShutdownMinutes = 5
	pol.IdleAction = "standby"
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
	if a := s.decide(sec(302), pol, false); a != actSleep {
		t.Fatalf("đủ 5 phút, máy chưa ai dùng: nhận %v, mong ngủ chờ", a)
	}
	// Ngủ không làm máy "đang khởi động lại": thức dậy vẫn giám sát như thường,
	// và không ra lệnh ngủ dồn dập mỗi hai giây.
	for i := 304; i < 600; i += 2 {
		s.noteStatus(sec(i), uiStatus{Locked: true})
		s.noteServerOK(sec(i))
		if a := s.decide(sec(i), pol, false); a != actNone {
			t.Fatalf("giây %d: vừa cho ngủ ở giây 302 mà đã %v", i, a)
		}
	}
	s.noteStatus(sec(604), uiStatus{Locked: true})
	s.noteServerOK(sec(604))
	if a := s.decide(sec(604), pol, false); a != actSleep {
		t.Fatalf("thêm 5 phút nữa vẫn không ai đăng nhập: nhận %v, mong ngủ lại", a)
	}
}

// Ngủ chờ: máy đã có người dùng thì KHÔNG được cho ngủ — người bấm nút tiếp theo
// sẽ thấy tài khoản và game của khách trước. Phải khởi động lại cho sạch, đúng một lần.
func TestSupervisor_NguChoMayDaDungThiKhoiDongLai(t *testing.T) {
	pol := defaultClientPolicy()
	pol.IdleShutdownMinutes = 5
	pol.IdleAction = "standby"
	for ten, st := range map[string]uiStatus{
		"hội viên":  {Member: true},
		"nhân viên": {Locked: true, Admin: true},
		"bảo trì":   {Locked: true, Maint: true},
	} {
		s := newUISupervisor(t0)
		s.noteUIRunning(t0, true)
		s.noteStatus(sec(0), uiStatus{Locked: true})
		s.noteStatus(sec(60), st)
		s.noteStatus(sec(120), uiStatus{Locked: true}) // người đó thoát
		s.noteServerOK(sec(422))
		s.noteStatus(sec(422), uiStatus{Locked: true})
		if a := s.decide(sec(422), pol, false); a != actIdleRestart {
			t.Errorf("%s: máy đã có người dùng, nhận %v, mong khởi động lại", ten, a)
			continue
		}
		if a := s.decide(sec(424), pol, false); a != actNone {
			t.Errorf("%s: đã ra lệnh khởi động lại rồi thì thôi, nhận %v", ten, a)
		}
	}
}

// noteStatus báo "vừa có người dùng" đúng một lần để dịch vụ ghi dấu ra đĩa; dấu đọc
// lại sau khi dịch vụ khởi động lại (danhDauDaDung) cũng chặn việc cho ngủ.
func TestSupervisor_DauDaDung(t *testing.T) {
	s := newUISupervisor(t0)
	s.noteUIRunning(t0, true)
	if s.noteStatus(sec(2), uiStatus{Locked: true}) {
		t.Fatal("màn khoá không ai đăng nhập không phải là đã dùng")
	}
	if !s.noteStatus(sec(4), uiStatus{Member: true}) {
		t.Fatal("lần đầu có hội viên phải báo")
	}
	if s.noteStatus(sec(6), uiStatus{Member: true}) || s.noteStatus(sec(8), uiStatus{Admin: true}) {
		t.Fatal("chỉ báo lần đầu")
	}

	pol := defaultClientPolicy()
	pol.IdleShutdownMinutes = 5
	pol.IdleAction = "standby"
	s2 := newUISupervisor(t0)
	s2.danhDauDaDung()
	s2.noteUIRunning(t0, true)
	s2.noteStatus(sec(0), uiStatus{Locked: true})
	s2.noteServerOK(sec(302))
	s2.noteStatus(sec(302), uiStatus{Locked: true})
	if a := s2.decide(sec(302), pol, false); a != actIdleRestart {
		t.Fatalf("dấu đã dùng từ trước khi dịch vụ khởi động lại: nhận %v, mong khởi động lại", a)
	}
}

// Ngủ chờ: giao diện mất đi mà bộ giám sát không bảo nó tắt là desktop đã lộ ra —
// coi như đã dùng. Còn bộ giám sát tự bật lại giao diện (treo) thì máy vẫn sạch.
func TestSupervisor_NguChoGiaoDienMat(t *testing.T) {
	pol := defaultClientPolicy()
	pol.IdleShutdownMinutes = 5
	pol.IdleAction = "standby"

	s := newUISupervisor(t0)
	s.noteUIRunning(t0, true)
	s.noteStatus(sec(2), uiStatus{Locked: true})
	s.noteUIRunning(sec(10), false) // bị tắt từ bên ngoài
	s.noteUIRunning(sec(12), true)
	s.noteStatus(sec(14), uiStatus{Locked: true})
	s.noteServerOK(sec(320))
	s.noteStatus(sec(320), uiStatus{Locked: true})
	if a := s.decide(sec(320), pol, false); a != actIdleRestart {
		t.Fatalf("giao diện bị tắt từ bên ngoài: nhận %v, mong khởi động lại", a)
	}

	s2 := newUISupervisor(t0)
	s2.noteUIRunning(t0, true)
	s2.noteStatus(sec(2), uiStatus{Locked: true})
	if a := s2.decide(sec(20), pol, false); a != actRestartUI { // im quá 15 giây
		t.Fatalf("giao diện im lặng: nhận %v, mong bật lại", a)
	}
	s2.noteUIRunning(sec(21), false) // chính bộ giám sát tắt nó
	s2.noteUIRunning(sec(22), true)
	s2.noteStatus(sec(24), uiStatus{Locked: true})
	s2.noteServerOK(sec(330))
	s2.noteStatus(sec(330), uiStatus{Locked: true})
	if a := s2.decide(sec(330), pol, false); a != actSleep {
		t.Fatalf("bộ giám sát tự bật lại giao diện, máy vẫn sạch: nhận %v, mong ngủ chờ", a)
	}
}

// Ngủ chờ: tệp maintenance.flag từng có mặt là máy đã có người dùng, kể cả khi tệp
// bị xoá trước lúc hết giờ rảnh.
func TestSupervisor_NguChoTepBaoTri(t *testing.T) {
	pol := defaultClientPolicy()
	pol.IdleShutdownMinutes = 5
	pol.IdleAction = "standby"
	s := newUISupervisor(t0)
	s.noteUIRunning(t0, true)
	s.noteStatus(sec(2), uiStatus{Locked: true})
	s.noteServerOK(sec(2))
	if a := s.decide(sec(2), pol, true); a != actNone {
		t.Fatalf("đang bảo trì: nhận %v", a)
	}
	s.noteServerOK(sec(400))
	s.noteStatus(sec(400), uiStatus{Locked: true})
	if a := s.decide(sec(400), pol, false); a != actIdleRestart {
		t.Fatalf("tệp bảo trì đã bị xoá: nhận %v, mong khởi động lại", a)
	}
}

// Ngủ chờ rồi thức dậy (khách bấm nút): đủ thời gian rảnh TÍNH TỪ LÚC THỨC mới ngủ lại.
func TestSupervisor_NguChoThucDayDemLai(t *testing.T) {
	pol := defaultClientPolicy()
	pol.IdleShutdownMinutes = 5
	pol.IdleAction = "standby"
	s := newUISupervisor(t0)
	s.noteUIRunning(t0, true)
	s.noteStatus(sec(2), uiStatus{Locked: true})
	s.noteServerOK(sec(302))
	s.noteStatus(sec(302), uiStatus{Locked: true})
	if a := s.decide(sec(302), pol, false); a != actSleep {
		t.Fatalf("nhận %v, mong ngủ chờ", a)
	}
	// Thức dậy ở giây 500 (đồng hồ thức; khoảng ngủ không tính).
	s.noteResume(sec(500))
	s.noteStatus(sec(500), uiStatus{Locked: true})
	s.noteServerOK(sec(798))
	s.noteStatus(sec(798), uiStatus{Locked: true})
	if a := s.decide(sec(798), pol, false); a != actNone {
		t.Fatalf("mới thức chưa đủ 5 phút mà đã %v", a)
	}
	s.noteServerOK(sec(802))
	s.noteStatus(sec(802), uiStatus{Locked: true})
	if a := s.decide(sec(802), pol, false); a != actSleep {
		t.Fatalf("thức đủ 5 phút vẫn không ai đăng nhập: nhận %v, mong ngủ lại", a)
	}
}
