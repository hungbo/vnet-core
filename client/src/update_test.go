package main

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// Tải một tệp thực thi rồi chạy mà không kiểm băm nghĩa là ai đứng giữa đường
// truyền cũng chạy được mã tuỳ ý trên toàn bộ máy trạm. Các phép kiểm dưới đây
// khoá lại hành vi đó.

func serveBytes(t *testing.T, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func sha256hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestDownloadVerifiesChecksum(t *testing.T) {
	payload := []byte("day la ban cai dat gia")
	srv := serveBytes(t, payload)

	path, err := downloadAndVerify(&UpdateInfo{
		HasUpdate: true, Version: "1.2.0",
		FileURL: srv.URL, Checksum: sha256hex(payload), FileSize: int64(len(payload)),
	})
	if err != nil {
		t.Fatalf("băm đúng mà vẫn từ chối: %v", err)
	}
	defer os.Remove(path)

	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(payload) {
		t.Errorf("nội dung tệp sai: %v", err)
	}
}

// Tệp bị tráo giữa đường: băm lệch, phải từ chối VÀ xoá tệp.
func TestDownloadRejectsTamperedFile(t *testing.T) {
	srv := serveBytes(t, []byte("tep da bi trao"))

	path, err := downloadAndVerify(&UpdateInfo{
		HasUpdate: true, Version: "1.2.0",
		FileURL: srv.URL, Checksum: sha256hex([]byte("tep goc")),
	})
	if err == nil {
		os.Remove(path)
		t.Fatal("chấp nhận tệp có băm không khớp")
	}
	if !strings.Contains(err.Error(), "băm không khớp") {
		t.Errorf("thông báo lỗi không nói rõ băm lệch: %v", err)
	}
	if path != "" {
		if _, statErr := os.Stat(path); statErr == nil {
			t.Error("tệp hỏng vẫn còn trên đĩa")
		}
	}
}

// Máy chủ khai thiếu băm hoặc băm rác: dừng ngay, không tải về.
func TestDownloadRefusesWithoutValidChecksum(t *testing.T) {
	srv := serveBytes(t, []byte("bat ky"))
	for _, bad := range []string{"", "khong-phai-bam", strings.Repeat("z", 64)} {
		_, err := downloadAndVerify(&UpdateInfo{
			HasUpdate: true, Version: "1.2.0", FileURL: srv.URL, Checksum: bad,
		})
		if err == nil {
			t.Errorf("chấp nhận băm %q", bad)
		}
	}
}

// Kích thước lệch cũng là dấu hiệu tệp không phải thứ máy chủ công bố.
func TestDownloadRejectsWrongSize(t *testing.T) {
	payload := []byte("ngan hon khai bao")
	srv := serveBytes(t, payload)
	_, err := downloadAndVerify(&UpdateInfo{
		HasUpdate: true, Version: "1.2.0",
		FileURL: srv.URL, Checksum: sha256hex(payload), FileSize: 999999,
	})
	if err == nil {
		t.Fatal("chấp nhận tệp có kích thước khác khai báo")
	}
}

// Cờ -X main.version phải bám được vào một biến có thật.
func TestVersionVariableExists(t *testing.T) {
	if version == "" {
		t.Error("biến version rỗng — cờ -X lúc biên dịch không có chỗ bám")
	}
}
