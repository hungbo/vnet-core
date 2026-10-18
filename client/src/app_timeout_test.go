package main

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// Máy chủ "nhận kết nối mà không trả lời": đúng thứ làm giao diện treo — nó
// không từ chối (nên không có lỗi để rơi sang bản lưu offline) mà cũng không
// trả lời (nên lời gọi không bao giờ kết thúc).
//
// Dựng bằng socket thô chứ không phải httptest: giữ kết nối mở mà không đọc
// một byte, tệ hơn mọi handler HTTP viết tay.
func mayChuNhanKetNoiRoiIm(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var conns []net.Conn
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, c)
			mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		ln.Close()
		mu.Lock()
		defer mu.Unlock()
		for _, c := range conns {
			c.Close()
		}
	})
	return "http://" + ln.Addr().String()
}

// Máy chủ trả header 200 rồi treo giữa chừng thân trả lời: hạn chót phải bao cả
// phần đọc thân, không chỉ phần chờ header.
func mayChuTreoGiuaThan(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"code":0,`))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	// Cắt kết nối trước khi Close: nếu lời gọi bị test không có hạn chót thì nó
	// còn treo ở đây, và Close sẽ chờ handler mãi — test hỏng phải hỏng gọn.
	t.Cleanup(func() {
		srv.CloseClientConnections()
		srv.Close()
	})
	return srv.URL
}

var cacMayChuTreo = []struct {
	ten  string
	dung func(*testing.T) string
}{
	{"nhận kết nối rồi im", mayChuNhanKetNoiRoiIm},
	{"treo giữa thân trả lời", mayChuTreoGiuaThan},
}

// rutHanChot rút hạn chót xuống mức test chờ được; trả lại giá trị thật khi xong.
func rutHanChot(t *testing.T, thuong, traMay time.Duration) {
	t.Helper()
	cuThuong, cuTraMay := uiTimeout, logoutTimeout
	uiTimeout, logoutTimeout = thuong, traMay
	t.Cleanup(func() { uiTimeout, logoutTimeout = cuThuong, cuTraMay })
}

// chayCoCanh chạy fn và đo thời gian; quá `tran` mà chưa xong thì test hỏng ngay
// thay vì treo theo — lời gọi treo mãi chính là lỗi cần bắt, nên không thể để
// test của nó cũng treo mãi.
func chayCoCanh(t *testing.T, tran time.Duration, fn func()) time.Duration {
	t.Helper()
	xong := make(chan struct{})
	bat := time.Now()
	go func() {
		defer close(xong)
		fn()
	}()
	select {
	case <-xong:
		return time.Since(bat)
	case <-time.After(tran):
		t.Fatalf("lời gọi vẫn chưa trả về sau %v — lời gọi từ giao diện không có hạn chót", tran)
		return 0
	}
}

// Lời gọi bình thường: máy chủ im thì phải TRẢ LỖI trong hạn chót, và lỗi đó phải
// là "không với tới máy chủ" — đó là điều kiện để Login rơi được sang bản lưu
// nhân viên (xem credcache.go: sai mật khẩu KHÔNG được rơi sang đó).
func TestDoRequest_MayChuTreoThiHetHanVaBaoKhongVoiToi(t *testing.T) {
	const han = 300 * time.Millisecond
	rutHanChot(t, han, han)

	for _, c := range cacMayChuTreo {
		t.Run(c.ten, func(t *testing.T) {
			a := &App{cfg: &Config{ServerURL: c.dung(t)}}

			var err error
			mat := chayCoCanh(t, 5*time.Second, func() {
				_, err = a.doRequest("GET", "/api/auth/me", nil)
			})

			if err == nil {
				t.Fatal("máy chủ không trả lời mà doRequest vẫn báo thành công")
			}
			if !errors.Is(err, errServerUnreachable) {
				t.Errorf("lỗi = %v — phải bọc errServerUnreachable để Login còn rơi sang bản lưu offline", err)
			}
			if mat < han*8/10 {
				t.Errorf("trả về sau %v, ngắn hơn hạn chót %v — test không còn đo đúng chỗ treo", mat, han)
			}
		})
	}
}

// Đăng nhập khi máy chủ treo: phải trả lỗi tử tế cho khách, không quay mãi.
func TestLogin_MayChuTreoThiBaoLoi(t *testing.T) {
	dungThuMucTam(t) // bản lưu nhân viên nằm trong thư mục cấu hình của người chạy test
	const han = 300 * time.Millisecond
	rutHanChot(t, han, han)

	for _, c := range cacMayChuTreo {
		t.Run(c.ten, func(t *testing.T) {
			a := &App{cfg: &Config{ServerURL: c.dung(t)}, machineCode: "KT-01"}

			var err error
			// Cộng thêm một giây chờ cố định sau lần thử offline thất bại (chống dò mật khẩu).
			chayCoCanh(t, 6*time.Second, func() {
				_, err = a.Login("khach", "matkhau-bat-ky")
			})

			if err == nil || !strings.Contains(err.Error(), "Không kết nối được máy chủ") {
				t.Fatalf("Login = %v, mong lỗi không kết nối được máy chủ", err)
			}
			if a.loggedIn() {
				t.Error("Login thất bại mà vẫn giữ token")
			}
		})
	}
}

// Đăng xuất khi máy chủ treo: hết hạn chót RIÊNG của đăng xuất (ngắn hơn lời gọi
// thường) rồi vẫn dọn trạng thái cục bộ — không để khách kế tiếp ngồi trước một
// máy còn token của người trước.
func TestLogout_MayChuTreoVanDonTrangThai(t *testing.T) {
	const han = 300 * time.Millisecond
	// Hạn chót thường để dài: nếu Logout vô tình dùng nó thì test này quá giờ.
	rutHanChot(t, 10*time.Second, han)
	// Máy chủ treo cũng là "không với tới": Logout hẹn thử lại ở nền. Hạn 0 để không còn vòng nào chạy
	// sau test này (nó sẽ gõ vào cổng mà test khác có thể đã dùng lại).
	rutLichThuLai(t, time.Millisecond, 0)

	for _, c := range cacMayChuTreo {
		t.Run(c.ten, func(t *testing.T) {
			a := &App{
				cfg:    &Config{ServerURL: c.dung(t)},
				locker: NewScreenLocker(),
				token:  "token-khach", userID: "u1", username: "khach", fullName: "Khách", role: "member",
			}
			a.verified.Store(true)

			var err error
			mat := chayCoCanh(t, 3*time.Second, func() { err = a.Logout() })

			if err != nil {
				t.Errorf("Logout trả lỗi %v — máy chủ treo không được chặn việc đăng xuất", err)
			}
			if mat < han*8/10 {
				t.Errorf("trả về sau %v, ngắn hơn hạn chót %v — test không còn đo đúng chỗ treo", mat, han)
			}
			if a.token != "" || a.userID != "" || a.role != "" || a.verified.Load() {
				t.Errorf("trạng thái đăng nhập chưa được dọn: token=%q userID=%q role=%q verified=%v",
					a.token, a.userID, a.role, a.verified.Load())
			}
		})
	}
}

// rutLichThuLai rút lịch thử lại việc trả máy của Logout xuống mức test chờ được; trả lại giá trị thật khi xong.
func rutLichThuLai(t *testing.T, buoc, tong time.Duration) {
	t.Helper()
	cuBuoc, cuTong := endRetryBackoff, endRetryFor
	endRetryBackoff, endRetryFor = []time.Duration{buoc}, tong
	t.Cleanup(func() { endRetryBackoff, endRetryFor = cuBuoc, cuTong })
}

// lichNhanh là lịch thử lại cho test gọi thẳng traMayONen.
func lichNhanh() lichTraMay {
	return lichTraMay{buoc: []time.Duration{time.Millisecond}, hetHan: time.Now().Add(10 * time.Second), hanChot: 2 * time.Second}
}

// dungMayChuTraMay dựng máy chủ giả cho việc trả máy: GET /sessions/me trả getBody; POST
// /sessions/me/end cắt kết nối (như mạng chớp) ruotKetNoi lần đầu, sau đó trả lời bình thường.
// dem trả số lần đã hỏi phiên và số lần đã gọi kết thúc; daTra đóng khi một lần kết thúc thành công.
func dungMayChuTraMay(t *testing.T, getBody string, ruotKetNoi int) (url string, dem func() (hoi, ket int), daTra <-chan struct{}) {
	t.Helper()
	var mu sync.Mutex
	var nHoi, nKet int
	xong := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/api/sessions/me":
			nHoi++
			w.Write([]byte(getBody))
		case "/api/sessions/me/end":
			nKet++
			if nKet <= ruotKetNoi {
				conn, _, _ := w.(http.Hijacker).Hijack()
				conn.Close()
				return
			}
			if nKet == ruotKetNoi+1 {
				close(xong)
			}
			w.Write([]byte(`{"code":0,"data":null}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL, func() (int, int) {
		mu.Lock()
		defer mu.Unlock()
		return nHoi, nKet
	}, xong
}

const phienTaiMayNay = `{"code":0,"data":{"machine_code":"KT-01"}}`

// Đăng xuất đúng lúc mạng chớp hay máy chủ đang khởi động lại: lời gọi kết thúc phiên thất bại,
// giao diện vẫn về màn hình khoá, nhưng phiên trên máy chủ PHẢI được trả khi máy chủ có lại — nếu
// không máy vẫn gửi nhịp tim nên máy chủ không bao giờ coi là mất máy, khách đã về vẫn bị trừ tiền
// và người sau gặp "máy đang có người dùng".
func TestLogout_KhongVoiToiMayChuThiVanTraMayKhiMayChuCoLai(t *testing.T) {
	rutHanChot(t, 5*time.Second, 2*time.Second)
	rutLichThuLai(t, 10*time.Millisecond, 10*time.Second)
	url, dem, daTra := dungMayChuTraMay(t, phienTaiMayNay, 2)

	a := &App{
		cfg:    &Config{ServerURL: url},
		locker: NewScreenLocker(), machineCode: "KT-01",
		token: "token-khach", userID: "u1", username: "khach", fullName: "Khách", role: "member",
	}
	a.verified.Store(true)

	if err := a.Logout(); err != nil {
		t.Fatalf("Logout trả lỗi %v", err)
	}
	if a.token != "" || a.userID != "" || a.role != "" || a.verified.Load() {
		t.Errorf("trạng thái đăng nhập chưa được dọn: token=%q userID=%q role=%q", a.token, a.userID, a.role)
	}

	select {
	case <-daTra:
	case <-time.After(5 * time.Second):
		_, ket := dem()
		t.Fatalf("máy chủ có lại mà phiên không bao giờ được trả (số lần gọi kết thúc: %d)", ket)
	}
}

// Máy chủ đã trả lời thì dừng, kể cả khi trả lời là "không": thử lại mãi với token hết hạn hay phiên
// đã đóng chỉ là gõ cửa vô ích.
func TestTraMayONen_DungKhiMayChuDaTraLoi(t *testing.T) {
	cases := []struct {
		ten, getBody string
		hoi, ket     int
	}{
		{"hết phiên (null)", `{"code":0,"data":null}`, 1, 0},
		{"token hết hạn", `{"code":9999,"message":"token hết hạn"}`, 1, 0},
		// /sessions/me/end kết thúc phiên đang chạy của hội viên Ở BẤT KỲ MÁY NÀO. Hội viên đã sang máy
		// khác thì kết thúc ở đây là đuổi họ khỏi chỗ đang chơi.
		{"phiên đã sang máy khác", `{"code":0,"data":{"machine_code":"KT-02"}}`, 1, 0},
		{"phiên còn trên máy này", phienTaiMayNay, 1, 1},
	}
	for _, c := range cases {
		url, dem, _ := dungMayChuTraMay(t, c.getBody, 0)
		a := &App{cfg: &Config{ServerURL: url}, machineCode: "KT-01"}

		chayCoCanh(t, 5*time.Second, func() { a.traMayONen("token-khach", lichNhanh()) })

		if hoi, ket := dem(); hoi != c.hoi || ket != c.ket {
			t.Errorf("%s: hỏi phiên %d lần, kết thúc %d lần — mong %d và %d", c.ten, hoi, ket, c.hoi, c.ket)
		}
	}
}

// Khách đăng nhập lại ngay trên máy này: máy chủ nối lại ĐÚNG phiên cũ, nên kết thúc nó theo lệnh trả
// máy trễ là đuổi họ khỏi máy giữa chừng. Nhân viên vào xem máy thì không cản việc trả máy.
func TestTraMayONen_KhachDangNhapLaiThiKhongKetThucPhienDangChoi(t *testing.T) {
	url, dem, _ := dungMayChuTraMay(t, phienTaiMayNay, 0)
	a := &App{cfg: &Config{ServerURL: url}, machineCode: "KT-01", token: "token-moi", role: "member"}
	chayCoCanh(t, 5*time.Second, func() { a.traMayONen("token-cu", lichNhanh()) })
	if hoi, ket := dem(); hoi != 0 || ket != 0 {
		t.Errorf("khách đã đăng nhập lại mà vẫn gọi máy chủ: hỏi %d, kết thúc %d", hoi, ket)
	}

	url, dem, _ = dungMayChuTraMay(t, phienTaiMayNay, 0)
	a = &App{cfg: &Config{ServerURL: url}, machineCode: "KT-01", token: "token-nv", role: "admin"}
	chayCoCanh(t, 5*time.Second, func() { a.traMayONen("token-cu", lichNhanh()) })
	if _, ket := dem(); ket != 1 {
		t.Errorf("nhân viên đăng nhập không được cản việc trả máy: kết thúc %d lần", ket)
	}
}

// Khôi phục phiên khi máy chủ treo: trả lỗi trong hạn chót, và KHÔNG để lại token
// của khách trước ở trạng thái "đã đăng nhập".
func TestRestoreSession_MayChuTreoThiBoVaKhongGiuToken(t *testing.T) {
	const han = 300 * time.Millisecond
	rutHanChot(t, han, han)

	for _, c := range cacMayChuTreo {
		t.Run(c.ten, func(t *testing.T) {
			a := &App{cfg: &Config{ServerURL: c.dung(t)}}

			var err error
			chayCoCanh(t, 5*time.Second, func() {
				_, err = a.RestoreSession("token-cua-khach-truoc")
			})

			if err == nil {
				t.Fatal("máy chủ không trả lời mà RestoreSession vẫn khôi phục")
			}
			if a.loggedIn() || a.verified.Load() {
				t.Errorf("còn giữ phiên sau khi khôi phục thất bại: token=%q verified=%v", a.token, a.verified.Load())
			}
		})
	}
}

// Trong lúc RestoreSession chờ máy chủ, token CHƯA được tin thì chưa được gán vào
// a.token: gán sớm làm loggedIn() báo true (OnDomReady mở khoá cửa sổ theo nó) và
// thất bại sau đó lại xoá a.token — kể cả của người vừa đăng nhập tay.
func TestRestoreSession_ChuaGanTokenKhiChuaXacNhan(t *testing.T) {
	dangCho := make(chan struct{})
	tiepTuc := make(chan struct{})
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		close(dangCho)
		<-tiepTuc
		w.Write([]byte(`{"code":401,"message":"token hết hạn"}`))
	}))
	defer srv.Close()
	a := &App{cfg: &Config{ServerURL: srv.URL}}

	xong := make(chan error, 1)
	go func() {
		_, err := a.RestoreSession("token-cu")
		xong <- err
	}()

	select {
	case <-dangCho:
	case <-time.After(5 * time.Second):
		t.Fatal("RestoreSession không gọi máy chủ")
	}
	if a.loggedIn() {
		t.Error("RestoreSession đã gán a.token trước khi máy chủ xác nhận")
	}
	close(tiepTuc)

	if err := <-xong; err == nil {
		t.Fatal("máy chủ từ chối token mà RestoreSession vẫn khôi phục")
	}
	if auth != "Bearer token-cu" {
		t.Errorf("Authorization = %q — lời hỏi phải mang chính token đang xin khôi phục", auth)
	}
	if a.loggedIn() {
		t.Error("bị từ chối mà vẫn giữ token")
	}
}

// Máy chủ xác nhận phiên cũ NHƯNG người ở máy đã tự vào trong lúc chờ (đăng nhập
// tay, hoặc mở khoá bảo trì): phiên của họ thắng, phiên cũ bị bỏ — không ghi đè
// danh tính, không mở thêm WebSocket cho người khác.
func TestRestoreSession_NguoiDangNhapTayTrongLucChoThiThang(t *testing.T) {
	cases := []struct {
		ten string
		vao func(a *App)
	}{
		{"đăng nhập tay", func(a *App) { a.token, a.userID, a.role = "token-moi", "u-moi", "member" }},
		{"mở khoá bảo trì", func(a *App) { a.baoTri.Store(true) }},
	}
	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			var a *App
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				c.vao(a) // người ở máy vào đúng lúc máy chủ đang nghĩ
				w.Write([]byte(`{"code":0,"data":{"id":"u-cu","username":"khach-cu","full_name":"Khách Cũ","role":"member","kind":"member"}}`))
			}))
			defer srv.Close()
			a = &App{cfg: &Config{ServerURL: srv.URL}}

			if _, err := a.RestoreSession("token-cu"); err == nil {
				t.Fatal("phiên cũ vẫn được khôi phục đè lên người vừa vào")
			}
			if a.token == "token-cu" || a.userID == "u-cu" || a.wsClient != nil {
				t.Errorf("phiên cũ đã ghi đè: token=%q userID=%q ws=%v", a.token, a.userID, a.wsClient != nil)
			}
		})
	}
}
