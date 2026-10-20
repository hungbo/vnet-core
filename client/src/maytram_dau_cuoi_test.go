package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Chuỗi C1–C4 từ lúc máy chủ trả nhịp tim/menu tới lúc tính năng chạy, với đúng dữ liệu của phòng test MAY06 (2026-10-20):
// ảnh /uploads/..., dòng WordPad, Alt+K bật. Mỗi mắt xích đã có test riêng; bài này nối chúng lại để một mắt xích gãy ở
// GIỮA (chính sách không qua được kênh dịch vụ ↔ giao diện, menu không ra logo) cũng bị bắt.

// nhipTimMAY06 là thân phản hồi nhịp tim như máy chủ 1.1.19 gửi.
const nhipTimMAY06 = `{"code":0,"message":"success","data":{"policy":{"tamper_action":"restart","offline_lock_seconds":60,` +
	`"offline_reboot_seconds":300,"blocked_apps":"","hidden_shortcuts":"","riot_metadata_source":"",` +
	`"idle_shutdown_minutes":5,"wallpapers":"/uploads/2026/10/20/1792465398056882300.png","wallpaper_interval_seconds":0,` +
	`"startup_apps":"C:\\Program Files\\Windows NT\\Accessories\\wordpad.exe","close_hotkey":true,"close_exclude":"",` +
	`"idle_action":"shutdown"},"vnc_password":""}}`

func datLaiChinhSach(t *testing.T) {
	t.Helper()
	policyMu.Lock()
	saved := policyNow
	policyNow = defaultClientPolicy()
	policyMu.Unlock()
	t.Cleanup(func() {
		policyMu.Lock()
		policyNow = saved
		policyMu.Unlock()
	})
}

func TestChinhSachTuNhipTimDenTinhNangC1C2C3(t *testing.T) {
	datLaiChinhSach(t)

	// Dịch vụ nền nhận nhịp tim, rồi gửi chính sách cho giao diện qua kênh (JSON); giao diện mới mở nên đang ở mặc định.
	onHeartbeatOK([]byte(nhipTimMAY06))
	wire, err := json.Marshal(svcReply{Policy: currentPolicy()})
	if err != nil {
		t.Fatal(err)
	}
	policyMu.Lock()
	policyNow = defaultClientPolicy()
	policyMu.Unlock()
	var reply svcReply
	if err := json.Unmarshal(wire, &reply); err != nil {
		t.Fatal(err)
	}
	if !setPolicy(reply.Policy) {
		t.Fatal("giao diện phải thấy chính sách đổi")
	}
	p := currentPolicy()
	if mo := moTaCaiDatMayTram(p); !strings.Contains(mo, "hình nền 1 ảnh") || !strings.Contains(mo, "ứng dụng tự chạy 1") || !strings.Contains(mo, "Alt+K bật") {
		t.Errorf("mô tả chính sách: %q", mo)
	}

	// C1: tải ảnh rồi đặt làm hình nền.
	var fetched, set []string
	w := &wallpaperRunner{
		dir: t.TempDir(),
		fetch: func(u, dest string) error {
			fetched = append(fetched, u)
			return os.WriteFile(dest, jpegHead, 0o644)
		},
		set: func(f string) error { set = append(set, filepath.Base(f)); return nil },
	}
	w.tick(p, time.Now())
	if len(fetched) != 1 || fetched[0] != "/uploads/2026/10/20/1792465398056882300.png" || len(set) != 1 {
		t.Errorf("hình nền: tải %v, đặt %v", fetched, set)
	}

	// C2: hội viên vừa đăng nhập thì mở WordPad (đường dẫn tuyệt đối trong Program Files của máy).
	var launched []string
	r := &startupRunner{
		root:         func() (string, error) { return `D:\Games`, nil },
		programFiles: func() []string { return []string{`C:\Program Files`} },
		exists:       func(string) bool { return true },
		running:      func() map[string]bool { return map[string]bool{} },
		launch:       func(exe, dir string) error { launched = append(launched, exe); return nil },
	}
	r.tick(p, false)
	if n := r.tick(p, true); n != 1 || len(launched) != 1 || launched[0] != `C:\Program Files\Windows NT\Accessories\wordpad.exe` {
		t.Errorf("ứng dụng tự chạy: chạy %d, %v", n, launched)
	}

	// C3: Alt+K bật.
	if !p.CloseHotkey {
		t.Error("Alt+K phải bật")
	}
}

// Chính sách lớn nhất có thể có phải lọt qua kênh trong MỘT thông điệp: bộ đệm 4096 byte cũ cắt nó, JSON hỏng, và giao diện
// không bao giờ nhận được chính sách (xem uiLinkMsgMax).
func TestChinhSachLonNhatLotQuaKenh(t *testing.T) {
	var anh, ungDung, loiTat []string
	for i := 0; i < maxWallpapers; i++ {
		anh = append(anh, "/uploads/2026/10/20/"+strings.Repeat("1", 150)+".png")
	}
	for i := 0; i < maxStartupApps; i++ {
		ungDung = append(ungDung, `C:\Program Files\`+strings.Repeat("a", 200)+`\x`+string(rune('a'+i))+`.exe`)
	}
	for i := 0; i < 50; i++ {
		loiTat = append(loiTat, strings.Repeat("L", 90)+string(rune('a'+i%26)))
	}
	p := normalizePolicy(clientPolicy{
		Wallpapers: strings.Join(anh, "\n"), StartupApps: strings.Join(ungDung, "\n"),
		HiddenShortcuts: strings.Join(loiTat, ","), CloseHotkey: true,
	})
	wire, err := json.Marshal(svcReply{Policy: p})
	if err != nil {
		t.Fatal(err)
	}
	if len(wire) <= 4096 {
		t.Fatalf("bài kiểm này phải dùng chính sách lớn hơn 4096 byte, được %d", len(wire))
	}
	if len(wire) > uiLinkMsgMax {
		t.Errorf("chính sách lớn nhất %d byte không lọt bộ đệm %d byte của kênh", len(wire), uiLinkMsgMax)
	}
}

func TestMenuBranding_TuMayChuDenLogo(t *testing.T) {
	var auth []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/game-menu/version":
			auth = append(auth, r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"version":"3.1700000000000.1700000000001"}}`))
		case "/api/game-menu":
			auth = append(auth, r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"root":"D:\\Games","items":[],` +
				`"cafe_name":"Net Mia","logo_url":"/uploads/2026/10/20/1792465398056882300.png"}}`))
		case "/uploads/2026/10/20/1792465398056882300.png":
			_, _ = w.Write(pngHead)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	gameMenuCache.reset()
	brandingLogo = logoCache{}
	t.Cleanup(func() {
		gameMenuCache.reset()
		brandingLogo = logoCache{}
	})
	a := &App{cfg: &Config{ServerURL: srv.URL}, token: "tok"}
	raw, err := a.MenuBranding()
	if err != nil {
		t.Fatal(err)
	}
	var b struct{ Name, Logo string }
	if err := json.Unmarshal([]byte(raw), &b); err != nil {
		t.Fatal(err)
	}
	if b.Name != "Net Mia" || !strings.HasPrefix(b.Logo, "data:image/png;base64,") {
		t.Errorf("tên %q, logo %.40q", b.Name, b.Logo)
	}
	for _, h := range auth {
		if h != "Bearer tok" {
			t.Errorf("menu phải gọi bằng token của hội viên, được %q", h)
		}
	}
}

func TestGhiPanicKhongLamSapTienTrinh(t *testing.T) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer ghiPanic("thử")
		panic("lỗi cố ý")
	}()
	<-done
}
