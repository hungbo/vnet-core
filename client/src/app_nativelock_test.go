package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// CurrentLogin là thứ giao diện nhận lấy khi khách đã vào máy ở màn khoá native:
// nó phải cùng dạng với phản hồi của Login, vì session.store dựng trạng thái đăng
// nhập từ đúng các khoá đó (access_token, kind, user.*). Chưa có lần đăng nhập nào
// được máy chủ xác nhận thì trả rỗng — giao diện không được tự mở máy theo nó.
func TestCurrentLogin_CungDangVoiLogin(t *testing.T) {
	cases := []struct {
		ten      string
		token    string
		verified bool
		role     string
		mongKind string // rỗng: mong chuỗi rỗng
	}{
		{"chưa đăng nhập", "", false, "", ""},
		{"có token nhưng máy chủ chưa xác nhận", "tok", false, "member", ""},
		{"hội viên", "tok", true, "member", "member"},
		{"nhân viên", "tok", true, "admin", "staff"},
	}
	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			a := &App{token: c.token, userID: "u1", username: "khach", fullName: "Khách Thử", role: c.role}
			a.verified.Store(c.verified)

			out := a.CurrentLogin()
			if c.mongKind == "" {
				if out != "" {
					t.Fatalf("CurrentLogin = %s, mong rỗng", out)
				}
				return
			}
			var got struct {
				AccessToken string `json:"access_token"`
				Kind        string `json:"kind"`
				User        struct {
					ID       string `json:"id"`
					Username string `json:"username"`
					FullName string `json:"full_name"`
					Role     string `json:"role"`
				} `json:"user"`
			}
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatalf("không đọc được %q: %v", out, err)
			}
			if got.AccessToken != "tok" || got.Kind != c.mongKind || got.User.ID != "u1" ||
				got.User.Username != "khach" || got.User.FullName != "Khách Thử" || got.User.Role != c.role {
				t.Errorf("CurrentLogin = %s", out)
			}
		})
	}
}

// Ô đăng nhập native không gọi Login trước khi Startup xong (Login cần a.ctx) và
// trước lượt trả lời đầu của dịch vụ nền (mang tài khoản quản trị máy trạm). Máy chủ
// từ chối thì câu lỗi về nguyên văn để màn khoá native hiện đúng như màn hình khoá web.
func TestNativeSubmit_ChoStartupVaChinhSachRoiMoiGoiLogin(t *testing.T) {
	var soLan atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		soLan.Add(1)
		w.Write([]byte(`{"code":1001,"message":"Sai tài khoản hoặc mật khẩu"}`))
	}))
	defer srv.Close()
	a := &App{
		cfg:          &Config{ServerURL: srv.URL},
		machineCode:  "KT-01",
		startupDone:  make(chan struct{}),
		chinhSachSan: make(chan struct{}),
	}

	xong := make(chan error, 1)
	go func() { xong <- a.nativeSubmit("khach", "sai-mat-khau") }()

	time.Sleep(150 * time.Millisecond)
	if n := soLan.Load(); n != 0 {
		t.Fatalf("đã gọi máy chủ %d lần trước khi Startup xong", n)
	}
	close(a.startupDone)
	time.Sleep(150 * time.Millisecond)
	if n := soLan.Load(); n != 0 {
		t.Fatalf("đã gọi máy chủ %d lần trước lượt trả lời đầu của dịch vụ nền", n)
	}
	a.coChinhSach()

	select {
	case err := <-xong:
		if err == nil || err.Error() != "Sai tài khoản hoặc mật khẩu" {
			t.Fatalf("nativeSubmit = %v, mong nguyên văn lỗi của máy chủ", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("nativeSubmit không trả về")
	}
	if n := soLan.Load(); n != 1 {
		t.Errorf("gọi máy chủ %d lần, mong 1", n)
	}
	if a.loggedIn() || a.verified.Load() {
		t.Error("bị từ chối mà vẫn giữ phiên")
	}
}

// Mỗi báo cáo hai giây một lần đều có trả lời: mở cửa cho ô đăng nhập native chỉ
// được đóng kênh đúng một lần, và cũng không được đụng tới màn hình khi chưa có Startup.
func TestApplyServiceReply_MoCuaChoONativeMotLan(t *testing.T) {
	cu := currentPolicy()
	t.Cleanup(func() {
		policyMu.Lock()
		policyNow = cu
		policyMu.Unlock()
	})
	a := &App{chinhSachSan: make(chan struct{})}

	a.applyServiceReply(svcReply{Policy: cu})
	a.applyServiceReply(svcReply{Policy: cu})

	select {
	case <-a.chinhSachSan:
	default:
		t.Fatal("lượt trả lời đầu của dịch vụ nền không mở cửa cho ô đăng nhập native")
	}
}

// Giao diện web đang khôi phục phiên cũ thì phiên đó thắng: Vào máy ở màn khoá native
// bị từ chối bằng câu tiếng Việt, và không hỏi máy chủ — Login không từ chối khi máy đã
// có token, nên cho chạy là hai lần đăng nhập giành một máy.
func TestNativeSubmit_NhuongPhienDangKhoiPhuc(t *testing.T) {
	t.Cleanup(func() { nativeLockRestoring(false) })
	var soLan atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		soLan.Add(1)
		w.Write([]byte(`{"code":1001,"message":"Sai tài khoản hoặc mật khẩu"}`))
	}))
	defer srv.Close()
	a := &App{
		cfg:          &Config{ServerURL: srv.URL},
		startupDone:  make(chan struct{}),
		chinhSachSan: make(chan struct{}),
	}
	close(a.startupDone)
	a.coChinhSach()

	nativeLockRestoring(true)
	err := a.nativeSubmit("khach", "matkhau")
	if err == nil || err.Error() != "Máy đang khôi phục phiên trước, vui lòng chờ giây lát" {
		t.Fatalf("nativeSubmit = %v, mong lời nhường phiên đang khôi phục", err)
	}
	if n := soLan.Load(); n != 0 {
		t.Errorf("đã gọi máy chủ %d lần trong lúc đang khôi phục phiên", n)
	}
	if a.nativeDangGui {
		t.Error("bị từ chối mà vẫn đánh dấu đang gửi")
	}
}

// Khách đã bấm Vào máy ở màn khoá native thì lần đó thắng: RestoreSession lui ngay,
// không hỏi máy chủ và không báo "đang khôi phục" lên màn khoá.
func TestRestoreSession_NhuongLanVaoMayNative(t *testing.T) {
	t.Cleanup(func() { nativeLockRestoring(false) })
	var soLan atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		soLan.Add(1)
	}))
	defer srv.Close()
	a := &App{cfg: &Config{ServerURL: srv.URL}, nativeDangGui: true}

	// Đúng lỗi này thì giao diện giữ phiên đã lưu để hỏi lại: lần Vào máy kia còn có
	// thể hỏng (sai mật khẩu).
	if _, err := a.RestoreSession("token-cu"); !errors.Is(err, errDangVaoMayNative) {
		t.Fatalf("RestoreSession = %v, mong errDangVaoMayNative", err)
	}
	if n := soLan.Load(); n != 0 {
		t.Errorf("đã hỏi máy chủ %d lần", n)
	}
	if nativeLockDangKhoiPhuc() || a.loggedIn() {
		t.Error("lui rồi mà vẫn báo đang khôi phục hoặc giữ token")
	}
}

// Trong lúc RestoreSession hỏi máy chủ, màn khoá native báo đang khôi phục; thất bại
// thì nút Vào máy mở lại ngay, không phải chờ hết nativeKhoiPhucToiDa. Đăng xuất (đường
// "phiên cũ đã hết" của restore) cũng mở lại.
func TestRestoreSession_BaoManKhoaNativeRoiTraLai(t *testing.T) {
	t.Cleanup(func() { nativeLockRestoring(false) })
	dangCho := make(chan struct{})
	tiepTuc := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	if !nativeLockDangKhoiPhuc() {
		t.Error("đang hỏi máy chủ mà màn khoá native không biết đang khôi phục")
	}
	close(tiepTuc)
	if err := <-xong; err == nil {
		t.Fatal("máy chủ từ chối token mà RestoreSession vẫn khôi phục")
	}
	if nativeLockDangKhoiPhuc() {
		t.Error("khôi phục thất bại mà nút Vào máy vẫn bị khoá")
	}

	nativeLockRestoring(true)
	if err := (&App{cfg: &Config{}}).Logout(); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if nativeLockDangKhoiPhuc() {
		t.Error("đã đăng xuất phiên khôi phục mà nút Vào máy vẫn bị khoá")
	}
}

// Giao diện (session.store.ts, restore) nhận ra lời lui của RestoreSession bằng đúng một
// câu chép tay: đổi một bên mà quên bên kia là phiên đã lưu lại bị xoá khi khách bấm
// Vào máy ở màn khoá native.
func TestErrDangVaoMayNative_KhopGiaoDien(t *testing.T) {
	ts, err := os.ReadFile("frontend/src/stores/session.store.ts")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`const NATIVE_DANG_VAO = '([^']+)'`).FindSubmatch(ts)
	if m == nil {
		t.Fatal("session.store.ts không còn NATIVE_DANG_VAO")
	}
	if !strings.Contains(errDangVaoMayNative.Error(), string(m[1])) {
		t.Errorf("errDangVaoMayNative = %q không chứa NATIVE_DANG_VAO = %q", errDangVaoMayNative, m[1])
	}
}

// DomReady (và đổi chế độ màn hình) không bao giờ hạ màn khoá native, và không khoá lại
// máy kỹ thuật viên vừa mở ở chế độ bảo trì.
func TestKhoaKhiApTrangThai(t *testing.T) {
	cases := []struct {
		nativeGiu, dangNhap, baoTri, mong bool
	}{
		{true, false, false, true},
		{true, true, false, true}, // RestoreSession đã gán token mà phiên chưa chắc còn
		{true, false, true, true},
		{false, false, false, true},
		{false, true, false, false},
		{false, false, true, false}, // bảo trì: DomReady không khoá lại
		{false, true, true, false},
	}
	for _, c := range cases {
		if got := khoaKhiApTrangThai(c.nativeGiu, c.dangNhap, c.baoTri); got != c.mong {
			t.Errorf("khoaKhiApTrangThai(native=%v, đăng nhập=%v, bảo trì=%v) = %v, mong %v",
				c.nativeGiu, c.dangNhap, c.baoTri, got, c.mong)
		}
	}
}

// Sau Login của màn khoá native: chỉ mở máy khi máy chủ đã xác nhận một lần đăng nhập
// còn nguyên; lần đó bị một lần đăng xuất chạy chồng dọn mất thì khoá lại và báo lỗi.
func TestSauVaoMayNative(t *testing.T) {
	loiMayChu := errors.New("Sai tài khoản hoặc mật khẩu")
	cases := []struct {
		ten                         string
		loi                         error
		baoTri, dangNhap, xacNhan   bool
		mongKhoa, mongMo, mongCoLoi bool
	}{
		{"máy chủ từ chối", loiMayChu, false, false, false, false, false, true},
		{"quản trị máy trạm / bản lưu nhân viên", nil, true, false, false, false, false, false},
		{"đăng nhập xong", nil, false, true, true, false, true, false},
		{"bị đăng xuất chạy chồng dọn mất", nil, false, false, false, true, false, true},
		{"token còn mà chưa xác nhận", nil, false, true, false, true, false, true},
	}
	for _, c := range cases {
		khoa, mo, err := sauVaoMayNative(c.loi, c.baoTri, c.dangNhap, c.xacNhan)
		if khoa != c.mongKhoa || mo != c.mongMo || (err != nil) != c.mongCoLoi {
			t.Errorf("%s: khoá=%v mở=%v lỗi=%v, mong khoá=%v mở=%v có lỗi=%v",
				c.ten, khoa, mo, err, c.mongKhoa, c.mongMo, c.mongCoLoi)
		}
		if c.loi != nil && err != c.loi {
			t.Errorf("%s: lỗi = %v, mong nguyên văn lỗi của Login", c.ten, err)
		}
	}
}

// Phiên kết thúc trong lúc giao diện chưa nhận lần đăng nhập ở màn khoá native (trang
// còn nạp, hoặc WebView2 đã chết): Go phải tự đăng xuất, vì trang chưa đăng nhập thì bỏ
// qua sự kiện. Trang đã nhận rồi thì để trang lo, như trước. Trang vẫn luôn được báo.
func TestKhoaKhiHetPhienChuaNhan(t *testing.T) {
	var traMay atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/sessions/me/end" {
			traMay.Add(1)
		}
		w.Write([]byte(`{"code":0,"data":null}`))
	}))
	defer srv.Close()

	t.Cleanup(func() { nativeState.Store(nlChoHien) })

	cases := []struct {
		ten        string
		loai       string
		payload    string
		dockHidden bool
		nativeGiu  bool
		role       string
		mongKhoa   bool
	}{
		{"chưa nhận, phiên của hội viên này", "session:ended", `{"member_id":"u1"}`, true, false, "member", true},
		{"chưa nhận, máy chủ tự đóng phiên (không ghi hội viên)", "session:auto-ended", `{"reason":"out_of_balance"}`, true, false, "member", true},
		{"chưa nhận, giới nghiêm", "curfew:enforced", `{"member_id":"u1"}`, true, false, "member", true},
		{"trang đã nhận", "session:ended", `{"member_id":"u1"}`, false, false, "member", false},
		// RestoreSession xong, trang đang hỏi phiên còn không: trang tự lo.
		{"đang khôi phục, màn khoá native còn giữ", "session:ended", `{"member_id":"u1"}`, true, true, "member", false},
		{"phiên của hội viên khác", "session:ended", `{"member_id":"u2"}`, true, false, "member", false},
		{"nhân viên", "session:ended", `{}`, true, false, "admin", false},
	}
	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			traMay.Store(0)
			nativeState.Store(nlDaTra)
			if c.nativeGiu {
				nativeState.Store(nlGiuKhoa)
			}
			a := &App{cfg: &Config{ServerURL: srv.URL}, token: "tok", userID: "u1", role: c.role, dockHidden: c.dockHidden}
			a.verified.Store(true)
			ws := &WSClient{handlers: map[string]WSHandler{}}
			var baoTrang int
			ws.On(c.loai, func(WSMessage) { baoTrang++ })

			a.khoaKhiHetPhienChuaNhan(ws)
			ws.handlers[c.loai](WSMessage{Type: c.loai, Payload: json.RawMessage(c.payload)})

			if baoTrang != 1 {
				t.Errorf("trang được báo %d lần, mong 1", baoTrang)
			}
			if daKhoa := !a.loggedIn() && !a.verified.Load(); daKhoa != c.mongKhoa {
				t.Errorf("đã đăng xuất = %v, mong %v", daKhoa, c.mongKhoa)
			}
			if c.mongKhoa && traMay.Load() != 1 {
				t.Errorf("gọi trả máy %d lần, mong 1 (đăng xuất như trang)", traMay.Load())
			}
		})
	}
}

// Thứ đang trả lời ở địa chỉ máy chủ không phải máy chủ VNET (trang HTML của router,
// IIS, cổng đăng nhập wifi): câu lỗi phải là tiếng Việt — màn khoá native hiện nguyên
// văn — và tính như không với tới máy chủ, để kỹ thuật viên còn vào bằng bản lưu nhân viên.
func TestDoRequest_TraLoiKhongPhaiJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><body>Router login</body></html>`))
	}))
	defer srv.Close()
	a := &App{cfg: &Config{ServerURL: srv.URL}}

	_, err := a.doRequest("POST", "/api/auth/client-login", LoginRequest{Username: "nv", Password: "x"})
	if !errors.Is(err, errServerUnreachable) {
		t.Errorf("lỗi = %v, mong bọc errServerUnreachable", err)
	}
	if err == nil || !strings.Contains(err.Error(), loiTraLoiLa) {
		t.Errorf("lỗi = %v, mong câu tiếng Việt %q", err, loiTraLoiLa)
	}
}
