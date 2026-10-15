package main

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// datDongHoThuc thay đồng hồ thức của supNow trong một test và lấy lại mốc từ đầu.
func datDongHoThuc(t *testing.T, f func() time.Duration) {
	t.Helper()
	cu := awakeClock
	reset := func() {
		supClockMu.Lock()
		supClockEpoch, supClockAwake, supClockLast = time.Time{}, false, 0
		supClockMu.Unlock()
	}
	awakeClock = f
	reset()
	t.Cleanup(func() { awakeClock = cu; reset() })
}

// Đồng hồ của bộ giám sát chỉ chạy khi máy thức: thời gian trôi mà đồng hồ thức
// đứng yên (máy ngủ) thì supNow cũng đứng yên.
func TestSupNow_KhongTinhLucNgu(t *testing.T) {
	var thuc atomic.Int64
	thuc.Store(int64(10 * time.Second))
	datDongHoThuc(t, func() time.Duration { return time.Duration(thuc.Load()) })

	truoc := supNow()
	time.Sleep(30 * time.Millisecond) // "ngủ": đồng hồ thường chạy, đồng hồ thức không
	if d := supNow().Sub(truoc); d != 0 {
		t.Fatalf("máy ngủ mà supNow vẫn chạy %v", d)
	}
	thuc.Add(int64(2 * time.Second))
	if d := supNow().Sub(truoc); d != 2*time.Second {
		t.Fatalf("thức 2 giây: supNow chạy %v", d)
	}
	thuc.Store(0) // đọc hỏng: đứng yên, không lùi
	if d := supNow().Sub(truoc); d != 2*time.Second {
		t.Fatalf("đọc hỏng: supNow lùi hoặc nhảy, %v", d)
	}
}

func TestSupNow_KhongCoDongHoThucThiLaDongHoGo(t *testing.T) {
	datDongHoThuc(t, func() time.Duration { return 0 })
	truoc := supNow()
	time.Sleep(20 * time.Millisecond)
	if d := supNow().Sub(truoc); d < 20*time.Millisecond {
		t.Fatalf("không có đồng hồ thức: supNow chỉ chạy %v", d)
	}
}

// Máy boot sẵn, màn hình khoá không ai đăng nhập, Lightning Boot cho ngủ cả đêm.
// Với đồng hồ thức, lúc khách bấm nút mới chỉ "2 giây" trôi qua.
func TestResume_QuaDemKhongRaLenhVaDemLaiTuLucThuc(t *testing.T) {
	pol := defaultClientPolicy()
	pol.IdleShutdownMinutes = 5
	s := newUISupervisor(t0)
	s.noteUIRunning(t0, true)
	s.noteServerOK(sec(1))
	s.noteStatus(sec(1), uiStatus{Locked: true}) // không ai đăng nhập từ sec(1)
	s.noteStatus(sec(240), uiStatus{Locked: true})
	if a := s.decide(sec(240), pol, false); a != actNone {
		t.Fatalf("trước khi ngủ: nhận %v", a)
	}

	// Ngủ. Thức ở sec(242) theo đồng hồ thức.
	thuc := sec(242)
	s.noteResume(thuc)
	if a := s.decide(thuc, pol, false); a != actNone {
		t.Fatalf("vừa thức: nhận %v, mong không làm gì", a)
	}

	// Khách có đủ 5 phút kể từ lúc thức, không phải 1 phút còn lại của trước lúc ngủ.
	for at := thuc.Add(2 * time.Second); ; at = at.Add(2 * time.Second) {
		s.noteServerOK(at)
		s.noteStatus(at, uiStatus{Locked: true})
		a := s.decide(at, pol, false)
		if at.Sub(thuc) < 5*time.Minute {
			if a != actNone {
				t.Fatalf("%v sau khi thức: nhận %v", at.Sub(thuc), a)
			}
			continue
		}
		if a != actShutdown {
			t.Fatalf("đủ 5 phút không ai đăng nhập sau khi thức: nhận %v, mong tắt máy", a)
		}
		break
	}
}

// Hội viên còn đăng nhập lúc máy ngủ: khoá cho tới khi máy chủ trả lời lại, để
// giao diện hỏi lại phiên trước khi người bấm nút dùng được desktop. Giao diện
// chỉ hỏi offlineLock mỗi 2 giây, nên khoá phải tới tay nó ít nhất một lần dù
// nhịp tim đã thành công trước lượt hỏi đầu tiên.
func TestResume_KhoaNgoaiTuyenToiKhiMayChuTraLoi(t *testing.T) {
	pol := defaultClientPolicy()
	s := newUISupervisor(t0)
	s.noteServerOK(sec(1))
	if s.offlineLock(sec(2), pol) {
		t.Fatal("chưa ngủ đã khoá")
	}

	s.noteResume(sec(3))
	s.noteServerOK(sec(3)) // nhịp tim giục lúc thức tới trước lượt hỏi đầu tiên
	if !s.offlineLock(sec(4), pol) {
		t.Fatal("lượt hỏi đầu tiên sau khi thức: phải khoá")
	}
	if !s.offlineLock(sec(6), pol) {
		t.Fatal("máy chủ chưa trả lời từ lúc báo khoá: phải còn khoá")
	}
	s.noteServerOK(sec(7))
	if s.offlineLock(sec(8), pol) {
		t.Fatal("máy chủ đã trả lời sau lúc báo khoá mà vẫn khoá")
	}
	if s.offlineLock(sec(10), pol) {
		t.Fatal("đã nhả rồi lại khoá")
	}
}

// Những gì đang được theo dõi lúc máy ngủ vẫn được theo dõi tiếp khi thức: khoá
// ngoại tuyến đang bật không tự nhả, phiên vừa kết thúc vẫn phải về màn hình khoá.
func TestResume_KhongXoaDauVetTruocKhiNgu(t *testing.T) {
	pol := defaultClientPolicy()
	s := newUISupervisor(t0)
	s.noteUIRunning(t0, true)
	s.noteStatus(sec(100), uiStatus{Member: true})
	if !s.offlineLock(sec(100), pol) {
		t.Fatal("mất máy chủ 100 giây mà chưa khoá")
	}
	s.noteSessionEnded(sec(101))
	s.noteResume(sec(102))
	s.noteStatus(sec(104), uiStatus{Member: true})
	if !s.offlineLock(sec(104), pol) {
		t.Fatal("ngủ một giấc là nhả khoá ngoại tuyến")
	}
	if a := s.decide(sec(112), pol, false); a != actRestartUI {
		t.Fatalf("phiên kết thúc trước lúc ngủ mà giao diện không về màn hình khoá: nhận %v", a)
	}
}

func TestDaNgu(t *testing.T) {
	cases := []struct {
		thuong, thuc, want time.Duration
	}{
		{8*time.Hour + 2*time.Second, 2 * time.Second, 8 * time.Hour}, // ngủ cả đêm
		{2 * time.Second, 2 * time.Second, 0},                         // lượt thường
		{2*time.Second + 3*time.Millisecond, 2 * time.Second, 0},      // lệch làm tròn
		{40 * time.Second, 40 * time.Second, 0},                       // tiến trình chậm: cả hai cùng chạy
	}
	for _, c := range cases {
		if got := daNgu(c.thuong, c.thuc); got != c.want {
			t.Errorf("daNgu(%v, %v) = %v, mong %v", c.thuong, c.thuc, got, c.want)
		}
	}
}

func datLaiOnResume(t *testing.T) {
	t.Helper()
	oldSup := agentSupervisor
	xoa := func() {
		resumeMu.Lock()
		lastResumeAt = time.Time{}
		resumeMu.Unlock()
		select {
		case <-nhipTimGiuc:
		default:
		}
	}
	xoa()
	t.Cleanup(func() { agentSupervisor = oldSup; xoa() })
}

func TestOnResume_KhoaGiucNhipTimVaBoQuaLanLap(t *testing.T) {
	datLaiOnResume(t)
	pol := defaultClientPolicy()
	agentSupervisor = newUISupervisor(supNow())

	onResume()
	if !agentSupervisor.offlineLock(supNow(), pol) {
		t.Fatal("onResume không giữ khoá chờ máy chủ")
	}
	select {
	case <-nhipTimGiuc:
	default:
		t.Fatal("onResume không giục nhịp tim")
	}
	if !vuaThucDay(time.Minute) {
		t.Fatal("vuaThucDay không biết máy vừa thức")
	}

	// Lần gọi thứ hai ngay sau (vòng giám sát tự phát hiện cùng lần thức) bị bỏ qua.
	time.Sleep(time.Millisecond)
	agentSupervisor.noteServerOK(supNow())
	onResume()
	if agentSupervisor.offlineLock(supNow(), pol) {
		t.Fatal("lần gọi thứ hai trong 10 giây không được bỏ qua")
	}
	select {
	case <-nhipTimGiuc:
		t.Fatal("lần gọi thứ hai lại giục nhịp tim")
	default:
	}
}

// Sau khi thức: gửi nhịp tim ngay, máy chủ chưa nhận thì thử lại tới khi được,
// được rồi thì thôi — không đợi lượt 15 giây.
func TestRunTelemetry_GiucThiGuiNgayVaThuLaiToiKhiDuoc(t *testing.T) {
	datLaiOnResume(t)
	agentSupervisor = nil
	cu := nhipTimThuLai
	nhipTimThuLai = 20 * time.Millisecond
	t.Cleanup(func() { nhipTimThuLai = cu })

	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable) // mạng/máy chủ chưa lên
			return
		}
		w.Write([]byte(`{"code":0,"data":{}}`))
	}))
	defer srv.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		runTelemetry(t.Context(), &Config{ServerURL: srv.URL, MachineCode: "MAY1", HeartbeatInterval: time.Hour})
	}()
	// Đợi vòng nhịp tim dừng hẳn (t.Context huỷ trước các Cleanup) rồi mới trả lại
	// các biến toàn cục nó đọc.
	t.Cleanup(func() { <-done })
	giucNhipTim()

	han := time.Now().Add(5 * time.Second)
	for n.Load() < 2 && time.Now().Before(han) {
		time.Sleep(5 * time.Millisecond)
	}
	if n.Load() < 2 {
		t.Fatalf("giục xong mà máy chủ chỉ nhận %d nhịp tim", n.Load())
	}
	time.Sleep(600 * time.Millisecond)
	if got := n.Load(); got != 2 {
		t.Fatalf("đã gửi được mà vẫn thử lại: %d nhịp tim", got)
	}
}

// Máy vừa thức thì chưa cập nhật: lượt kiểm quá hạn trong lúc ngủ chạy đúng lúc
// khách vừa bấm nút.
func TestThuTuCapNhat_HoanKhiVuaThucDay(t *testing.T) {
	datLaiOnResume(t)
	cfg := &Config{ServerURL: "http://127.0.0.1:1", MachineCode: "MAY1"}
	if err := thuTuCapNhat(cfg); err == nil {
		t.Fatal("chưa ngủ: lẽ ra phải hỏi máy chủ (và lỗi vì không có máy chủ)")
	}
	resumeMu.Lock()
	lastResumeAt = time.Now()
	resumeMu.Unlock()
	if err := thuTuCapNhat(cfg); err != nil {
		t.Fatalf("vừa thức: không được hỏi máy chủ, nhận %v", err)
	}
}
