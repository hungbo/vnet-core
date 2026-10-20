package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var jpegHead = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0x10, 'J', 'F', 'I', 'F'}

func TestHasImageMagic(t *testing.T) {
	if !hasImageMagic(jpegHead) || !hasImageMagic([]byte("\x89PNG\r\n\x1a\nxxxx")) {
		t.Error("JPEG và PNG phải nhận ra")
	}
	for _, bad := range [][]byte{nil, []byte("<!doctype html>"), []byte("GIF89a"), []byte("RIFF....WEBP")} {
		if hasImageMagic(bad) {
			t.Errorf("%q không phải JPEG/PNG", bad)
		}
	}
}

func TestWallpaperFileName(t *testing.T) {
	a, b := wallpaperFileName("/uploads/2026/a.JPG"), wallpaperFileName("/uploads/2026/b.jpg")
	if a == b || !strings.HasSuffix(a, ".jpg") {
		t.Errorf("tên %q và %q: phải khác nhau và giữ đuôi chữ thường", a, b)
	}
	if a != wallpaperFileName("/uploads/2026/a.JPG") {
		t.Error("cùng đường dẫn phải cùng tên")
	}
}

// runner giả: không tải mạng, không đặt hình nền thật.
func newTestRunner(t *testing.T) (*wallpaperRunner, *[]string, *int) {
	t.Helper()
	var set []string
	fetched := 0
	return &wallpaperRunner{
		dir: t.TempDir(),
		fetch: func(u, dest string) error {
			fetched++
			return os.WriteFile(dest, jpegHead, 0o644)
		},
		set: func(f string) error { set = append(set, filepath.Base(f)); return nil },
	}, &set, &fetched
}

func TestWallpaperRunner_TinhNangTat(t *testing.T) {
	w, set, fetched := newTestRunner(t)
	w.tick(clientPolicy{}, time.Now())
	if len(*set) != 0 || *fetched != 0 {
		t.Errorf("chính sách rỗng là tắt: không tải, không đặt (đặt %v, tải %d)", *set, *fetched)
	}
}

func TestWallpaperRunner_MotAnhDatMotLan(t *testing.T) {
	w, set, fetched := newTestRunner(t)
	p := clientPolicy{Wallpapers: "/uploads/a.jpg"}
	now := time.Now()
	w.tick(p, now)
	w.tick(p, now.Add(time.Hour))
	if len(*set) != 1 || *fetched != 1 {
		t.Errorf("một ảnh: đặt một lần, tải một lần (đặt %v, tải %d)", *set, *fetched)
	}
}

func TestWallpaperRunner_TrinhChieu(t *testing.T) {
	w, set, _ := newTestRunner(t)
	p := clientPolicy{Wallpapers: "/uploads/a.jpg\n/uploads/b.jpg\n/uploads/c.png", WallpaperIntervalSeconds: 60}
	t0 := time.Now()
	w.tick(p, t0)
	w.tick(p, t0.Add(30*time.Second)) // chưa tới hạn
	if len(*set) != 1 {
		t.Fatalf("30 giây chưa đổi ảnh, đã đặt %v", *set)
	}
	w.tick(p, t0.Add(61*time.Second))
	w.tick(p, t0.Add(122*time.Second))
	w.tick(p, t0.Add(183*time.Second)) // quay vòng về ảnh đầu
	want := []string{wallpaperFileName("/uploads/a.jpg"), wallpaperFileName("/uploads/b.jpg"), wallpaperFileName("/uploads/c.png"), wallpaperFileName("/uploads/a.jpg")}
	if !sameStrings(*set, want) {
		t.Errorf("thứ tự ảnh %v, mong %v", *set, want)
	}
}

func TestWallpaperRunner_DoiDanhSachLamLaiTuDau(t *testing.T) {
	w, set, _ := newTestRunner(t)
	now := time.Now()
	w.tick(clientPolicy{Wallpapers: "/uploads/a.jpg"}, now)
	w.tick(clientPolicy{Wallpapers: "/uploads/b.jpg"}, now.Add(time.Second))
	if len(*set) != 2 || (*set)[1] != wallpaperFileName("/uploads/b.jpg") {
		t.Errorf("danh sách mới phải được đặt ngay: %v", *set)
	}
}

func TestWallpaperRunner_AnhLoiThiBoQuaVaThuLai(t *testing.T) {
	w, set, _ := newTestRunner(t)
	fail := true
	w.fetch = func(u, dest string) error {
		if strings.HasSuffix(u, "b.jpg") && fail {
			return errors.New("mất mạng")
		}
		return os.WriteFile(dest, jpegHead, 0o644)
	}
	p := clientPolicy{Wallpapers: "/uploads/a.jpg\n/uploads/b.jpg"}
	now := time.Now()
	w.tick(p, now)
	if len(*set) != 1 || (*set)[0] != wallpaperFileName("/uploads/a.jpg") {
		t.Fatalf("ảnh a vẫn phải chạy dù b lỗi: %v", *set)
	}
	fail = false
	w.tick(p, now.Add(10*time.Second)) // chưa hết một phút chờ
	if len(w.files) != 1 {
		t.Errorf("chưa tới lúc thử lại, files=%d", len(w.files))
	}
	w.tick(p, now.Add(70*time.Second))
	if len(w.files) != 2 {
		t.Errorf("sau một phút phải tải được ảnh b, files=%d", len(w.files))
	}
}

func TestFetchWallpaper(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/uploads/ok.jpg":
			_, _ = w.Write(jpegHead)
		case "/uploads/html.jpg":
			_, _ = w.Write([]byte("<html>đăng nhập</html>"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	dir := t.TempDir()
	dest := filepath.Join(dir, "x.jpg")
	if err := fetchWallpaper(srv.URL, "/uploads/ok.jpg", dest); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dest); string(b) != string(jpegHead) {
		t.Error("nội dung ảnh sai")
	}
	if _, err := os.Stat(dest + ".part"); err == nil {
		t.Error("tệp tạm phải được đổi tên đi")
	}
	for _, p := range []string{"/uploads/html.jpg", "/uploads/khong-co.jpg"} {
		d := filepath.Join(dir, "bad.jpg")
		if err := fetchWallpaper(srv.URL, p, d); err == nil {
			t.Errorf("%s phải lỗi", p)
		}
		if _, err := os.Stat(d); err == nil {
			t.Errorf("%s: không được để lại tệp hỏng trên đĩa", p)
		}
	}
}
