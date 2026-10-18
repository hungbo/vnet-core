package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// Địa chỉ trả về phải dựng từ route tĩnh /uploads, không phải từ đường dẫn trên
// đĩa. Bản Docker đặt UPLOAD_DIR=/app/uploads, và khi ghép "/" + đường dẫn đĩa
// thì ra "/app/uploads/..." — không route nào khớp, request rơi vào trang quản
// trị nhúng và trả về HTML kèm mã 200. Thẻ <img> không báo lỗi, chỉ hiện ô
// trắng, nên mọi ảnh tải lên bằng bản Docker im lặng hỏng.
func TestUpload_URLKhongDinhDuongDanTrenDia(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// t.TempDir() là đường TUYỆT ĐỐI, đúng hình dạng của UPLOAD_DIR=/app/uploads
	// trong Docker — tức là đã phủ chính trường hợp gây lỗi. Trường hợp tương
	// đối là cấu hình mặc định khi chạy trực tiếp.
	for name, uploadDir := range map[string]string{
		"tuyệt đối": t.TempDir(),
		"tương đối": filepath.Join("testdata-tmp", t.Name()),
	} {
		t.Run(name, func(t *testing.T) {
			h := NewUploadHandler(uploadDir, 5<<20)
			if !filepath.IsAbs(uploadDir) {
				t.Cleanup(func() { os.RemoveAll("testdata-tmp") })
			}

			var body bytes.Buffer
			w := multipart.NewWriter(&body)
			part, err := w.CreateFormFile("file", "anh-mon.png")
			if err != nil {
				t.Fatal(err)
			}
			part.Write([]byte("khong-phai-png-that-nhung-du-de-ghi"))
			w.Close()

			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/upload", &body)
			c.Request.Header.Set("Content-Type", w.FormDataContentType())

			h.Upload(c)

			if rec.Code != http.StatusOK {
				t.Fatalf("HTTP %d: %s", rec.Code, rec.Body.String())
			}

			var res struct {
				Data struct {
					URL string `json:"url"`
				} `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
				t.Fatal(err)
			}

			if !strings.HasPrefix(res.Data.URL, "/uploads/") {
				t.Errorf("url = %q — phải bắt đầu bằng /uploads/ để khớp route tĩnh", res.Data.URL)
			}
			if strings.Contains(res.Data.URL, uploadDir) || strings.Contains(res.Data.URL, "//") {
				t.Errorf("url = %q — dính đường dẫn trên đĩa (%s)", res.Data.URL, uploadDir)
			}
			if !strings.HasSuffix(res.Data.URL, ".png") {
				t.Errorf("url = %q — mất phần mở rộng", res.Data.URL)
			}
		})
	}
}

// Định dạng lạ bị chặn trước khi ghi đĩa.
func TestUpload_ChanDinhDangLa(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewUploadHandler(t.TempDir(), 5<<20)

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, _ := w.CreateFormFile("file", "script.exe")
	part.Write([]byte("MZ"))
	w.Close()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/upload", &body)
	c.Request.Header.Set("Content-Type", w.FormDataContentType())

	h.Upload(c)

	if rec.Code == http.StatusOK {
		t.Errorf("nhận .exe: %s", rec.Body.String())
	}
}
