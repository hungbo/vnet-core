package main

import (
	"testing"
	"time"
)

// Một quan sát "khoẻ mạnh": dịch vụ đăng ký rồi, đọc được, đang chạy, giao diện
// đã qua quãng im lặng.
func quanSatKhoe() guardObservation {
	return guardObservation{
		ServiceInstalled: true,
		ServiceQueryOK:   true,
		ServiceRunning:   true,
		UIUptime:         10 * time.Minute,
	}
}

func chay(t *testing.T, o guardObservation, luot int) (guardAction, int) {
	t.Helper()
	p := defaultGuardPolicy()
	st := &guardState{}
	act := guardNothing
	for i := 0; i < luot; i++ {
		act = st.step(o, p)
	}
	return act, st.strikes
}

// Đường đi bình thường: không bao giờ đụng tới máy.
func TestGuard_DichVuChayThiKhongLamGi(t *testing.T) {
	act, strikes := chay(t, quanSatKhoe(), 1000)
	if act != guardNothing || strikes != 0 {
		t.Fatalf("act=%v strikes=%d — dịch vụ đang chạy mà vẫn tính điểm xấu", act, strikes)
	}
}

// Đúng kịch bản cần chặn: dịch vụ bị giết và không bật lại được.
func TestGuard_DichVuChetHanThiTatMay(t *testing.T) {
	o := quanSatKhoe()
	o.ServiceRunning = false
	o.StartOK = false

	p := defaultGuardPolicy()
	st := &guardState{}
	for i := 1; i < p.Strike; i++ {
		if act := st.step(o, p); act != guardWarn {
			t.Fatalf("lượt %d: act=%v — chưa đủ số lượt đã đòi tắt máy", i, act)
		}
	}
	if act := st.step(o, p); act != guardShutdown {
		t.Fatalf("lượt %d: act=%v — đủ số lượt rồi mà không tắt máy", p.Strike, act)
	}
}

// Bật lại được là tha. Kẻ phá phải thắng LIÊN TỤC một phút mới bị tắt máy.
func TestGuard_BatLaiDuocThiXoaBoDem(t *testing.T) {
	p := defaultGuardPolicy()
	st := &guardState{}

	chet := quanSatKhoe()
	chet.ServiceRunning = false

	for i := 0; i < p.Strike-1; i++ {
		st.step(chet, p)
	}

	cuuDuoc := chet
	cuuDuoc.StartOK = true
	if act := st.step(cuuDuoc, p); act != guardNothing || st.strikes != 0 {
		t.Fatalf("act=%v strikes=%d — cứu được dịch vụ mà bộ đếm không xoá", act, st.strikes)
	}

	// Và sau khi xoá thì phải đếm lại từ đầu, không phải tắt máy ngay lượt sau.
	if act := st.step(chet, p); act != guardWarn {
		t.Fatalf("act=%v — bộ đếm không thật sự về 0", act)
	}
}

// Năm cách khiến ta KHÔNG phán được. Mỗi cách đều phải dừng lớp bảo vệ lại, vì
// nhầm một cái là tắt máy của khách đang chơi thật.
func TestGuard_KhongPhanDuocThiKhongBaoGioTatMay(t *testing.T) {
	p := defaultGuardPolicy()

	truongHop := map[string]func(*guardObservation){
		"Windows đang tắt":        func(o *guardObservation) { o.SystemShuttingDown = true },
		"đang bảo trì":            func(o *guardObservation) { o.MaintenanceMode = true },
		"giao diện vừa khởi động": func(o *guardObservation) { o.UIUptime = p.Grace - time.Second },
		"chưa cài dịch vụ":        func(o *guardObservation) { o.ServiceInstalled = false },
		"không đọc được dịch vụ":  func(o *guardObservation) { o.ServiceQueryOK = false },
	}

	for ten, doi := range truongHop {
		t.Run(ten, func(t *testing.T) {
			o := quanSatKhoe()
			o.ServiceRunning = false // dịch vụ thật sự không chạy
			doi(&o)

			act, strikes := chay(t, o, p.Strike*10)
			if act != guardNothing || strikes != 0 {
				t.Fatalf("act=%v strikes=%d — %s mà vẫn đi tới chỗ tắt máy", act, strikes, ten)
			}
		})
	}
}

// Bản chạy trên máy không phải Windows phải luôn im lặng: quan sát rỗng nghĩa
// là chưa cài dịch vụ, tức không phán được.
func TestGuard_NgoaiWindowsKhongBaoGioTatMay(t *testing.T) {
	act, strikes := chay(t, observeGuard(10*time.Hour), 1000)
	if act != guardNothing || strikes != 0 {
		t.Fatalf("act=%v strikes=%d", act, strikes)
	}
}

// Quãng im lặng phải dài hơn thời gian dịch vụ cần để khởi động cùng Windows.
// Đặt ngắn hơn là mỗi lần bật máy thành một lần tắt máy.
func TestGuard_QuangImLangDuDai(t *testing.T) {
	p := defaultGuardPolicy()
	if p.Grace < time.Minute {
		t.Errorf("Grace = %v — quá ngắn so với lúc Windows khởi động", p.Grace)
	}
	if tong := time.Duration(p.Strike) * p.Every; tong < time.Minute {
		t.Errorf("cần %v mới tắt máy — quá vội với một trục trặc thoáng qua", tong)
	}
}

// Nhân viên kỹ thuật mở máy bằng PIN thì việc đầu tiên họ làm thường là tắt
// dịch vụ. Lớp chống phá phải đứng yên trong lúc đó, nếu không thì máy tắt
// ngay giữa lúc đang sửa.
func TestGuard_BaoTriThiDungYen(t *testing.T) {
	p := defaultGuardPolicy()
	st := &guardState{}

	chet := quanSatKhoe()
	chet.ServiceRunning = false

	// Chưa bảo trì: bộ đếm chạy.
	for i := 0; i < 3; i++ {
		st.step(chet, p)
	}
	if st.strikes != 3 {
		t.Fatalf("strikes = %d, mong 3", st.strikes)
	}

	// Bật bảo trì: xoá bộ đếm và không bao giờ tắt máy.
	baoTri := chet
	baoTri.MaintenanceMode = true
	for i := 0; i < p.Strike*5; i++ {
		if act := st.step(baoTri, p); act != guardNothing {
			t.Fatalf("act=%v — đang bảo trì mà vẫn hành động", act)
		}
	}
	if st.strikes != 0 {
		t.Fatalf("strikes = %d — bảo trì phải xoá bộ đếm", st.strikes)
	}
}
