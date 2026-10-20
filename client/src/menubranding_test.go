package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var pngHead = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

func TestLogoHopLeMay(t *testing.T) {
	for _, ok := range []string{"/uploads/2026/10/20/1.png", "/uploads/a.WEBP", " /uploads/a.gif "} {
		if _, good := logoHopLeMay(ok); !good {
			t.Errorf("%q phải hợp lệ", ok)
		}
	}
	for _, bad := range []string{"", "/x/a.png", "/uploads/a.svg", "/uploads/../a.png", "http://evil/a.png", "/uploads/a.png?x=1", "/uploads/a b.png"} {
		if _, good := logoHopLeMay(bad); good {
			t.Errorf("%q phải bị từ chối", bad)
		}
	}
}

func TestFetchLogoDataURI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/uploads/ok.png":
			_, _ = w.Write(pngHead)
		case "/uploads/svg.png": // đuôi png nhưng nội dung là SVG: phải bị từ chối
			_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`))
		case "/uploads/big.png":
			_, _ = w.Write(append(pngHead, make([]byte, logoMaxBytes)...))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	uri, err := fetchLogoDataURI(srv.URL, "/uploads/ok.png")
	if err != nil || !strings.HasPrefix(uri, "data:image/png;base64,") {
		t.Fatalf("uri=%q err=%v", uri, err)
	}
	for _, p := range []string{"/uploads/svg.png", "/uploads/big.png", "/uploads/khong-co.png"} {
		if _, err := fetchLogoDataURI(srv.URL, p); err == nil {
			t.Errorf("%s phải lỗi", p)
		}
	}
}

func TestLogoCache(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path == "/uploads/loi.png" {
			http.Error(w, "hỏng", 500)
			return
		}
		_, _ = w.Write(pngHead)
	}))
	defer srv.Close()
	var c logoCache
	now := time.Now()
	for i := 0; i < 3; i++ {
		if c.get(srv.URL, "/uploads/ok.png", now) == "" {
			t.Fatal("logo phải có")
		}
	}
	if hits != 1 {
		t.Errorf("cùng đường dẫn chỉ tải một lần, tải %d", hits)
	}
	hits = 0
	for i := 0; i < 3; i++ {
		c.get(srv.URL, "/uploads/loi.png", now.Add(time.Duration(i)*time.Second))
	}
	if hits != 1 {
		t.Errorf("lỗi được nhớ một phút, không gọi lại mỗi lần (gọi %d)", hits)
	}
	c.get(srv.URL, "/uploads/loi.png", now.Add(2*time.Minute))
	if hits != 2 {
		t.Errorf("hết một phút phải thử lại (gọi %d)", hits)
	}
}
