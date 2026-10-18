package main

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	tokenTruyCap = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJraGFjaDEifQ.c2lnLWFjY2Vzcw"
	tokenLamMoi  = "eyJhbGciOiJIUzI1NiJ9.eyJ0eXAiOiJyZWZyZXNoIn0.c2lnLXJlZnJlc2g"
)

// ghiNhatKy chạy f với log trỏ vào bộ đệm và trả lại những gì đã ghi.
func ghiNhatKy(t *testing.T, f func()) string {
	t.Helper()
	var buf bytes.Buffer
	cu, co := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(cu); log.SetFlags(co) })
	f()
	return buf.String()
}

// ui.log nằm trong thư mục của tài khoản khách: trả lời đăng nhập ghi nguyên văn là
// khách sau lấy được token của khách trước (dùng được phiên và số dư của họ).
func TestDoRequest_KhongGhiTokenVaoNhatKy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":0,"message":"success","data":{"access_token":"` + tokenTruyCap +
			`","refresh_token":"` + tokenLamMoi + `","kind":"member","user":{"username":"khach1","phone":"0901234567"}}}`))
	}))
	defer srv.Close()
	a := &App{cfg: &Config{ServerURL: srv.URL}}

	out := ghiNhatKy(t, func() {
		if _, err := a.doRequest("POST", "/api/auth/client-login", LoginRequest{Username: "khach1", Password: "Bi-Mat-123"}); err != nil {
			t.Fatal(err)
		}
	})
	for _, bimat := range []string{tokenTruyCap, tokenLamMoi, "eyJ", "Bi-Mat-123", "0901234567"} {
		if strings.Contains(out, bimat) {
			t.Fatalf("nhật ký chứa %q:\n%s", bimat, out)
		}
	}
	if !strings.Contains(out, "/api/auth/client-login → 200") {
		t.Fatalf("thiếu dòng trạng thái:\n%s", out)
	}
}

// Tin chat và góp ý của khách đi trong thân YÊU CẦU: cũng không được nằm trong ui.log.
func TestDoRequest_KhongGhiTinChatVaGopY(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":0,"message":"success","data":{}}`))
	}))
	defer srv.Close()
	a := &App{cfg: &Config{ServerURL: srv.URL}}

	out := ghiNhatKy(t, func() {
		_, _ = a.doRequest("POST", "/api/chat/messages", map[string]interface{}{"room_id": "r1", "message": "mật khẩu wifi nhà tôi là 123", "message_type": "text"})
		_, _ = a.doRequest("POST", "/api/feedback", map[string]interface{}{"content": "nhân viên ca tối thái độ kém", "rating": 2})
	})
	for _, bimat := range []string{"wifi", "thái độ"} {
		if strings.Contains(out, bimat) {
			t.Fatalf("nhật ký chứa %q:\n%s", bimat, out)
		}
	}
	if !strings.Contains(out, `"message_type":"text"`) {
		t.Fatalf("trường không bí mật cũng bị che:\n%s", out)
	}
}

// Trả lời không phải của máy chủ VNET vẫn được ghi (để biết địa chỉ trỏ nhầm đi đâu),
// nhưng đã che.
func TestDoRequest_TraLoiLaGhiBanDaChe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>proxy login ` + tokenTruyCap + `</html>`))
	}))
	defer srv.Close()
	a := &App{cfg: &Config{ServerURL: srv.URL}}

	out := ghiNhatKy(t, func() { _, _ = a.doRequest("GET", "/api/sessions/me", nil) })
	if strings.Contains(out, "eyJ") {
		t.Fatalf("nhật ký chứa token:\n%s", out)
	}
	if !strings.Contains(out, "proxy login ***") {
		t.Fatalf("thiếu thân trả lời đã che:\n%s", out)
	}
}

func TestLogBody(t *testing.T) {
	cases := []struct{ ten, vao, mong string }{
		{"che trường bí mật ở mọi độ sâu, giữ code/message",
			`{"code":1001,"message":"Sai mật khẩu","data":{"refresh_token":"x","items":[{"pin":"1234","name":"a"}]}}`,
			`{"code":1001,"data":{"items":[{"name":"a","pin":"***"}],"refresh_token":"***"},"message":"Sai mật khẩu"}`},
		{"che JWT nằm trong chuỗi bất kỳ",
			`{"code":9999,"message":"hết hạn","data":{"detail":"bad ` + tokenTruyCap + `"}}`,
			`{"code":9999,"data":{"detail":"bad ***"},"message":"hết hạn"}`},
		{"giữ nguyên số lớn", `{"code":0,"data":{"amount":12345678901234567890}}`, `{"code":0,"data":{"amount":12345678901234567890}}`},
		{"không phải JSON", `oops ` + tokenLamMoi, `oops ***`},
	}
	for _, c := range cases {
		if got := logBody([]byte(c.vao)); got != c.mong {
			t.Errorf("%s: logBody = %s, mong %s", c.ten, got, c.mong)
		}
	}
	if got := logBody([]byte(strings.Repeat("a", 1000))); len(got) != maxLoggedBody+len("...") {
		t.Errorf("cắt ngắn: dài %d", len(got))
	}
}
