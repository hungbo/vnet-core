package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSendErrors_PostsBatchToMachineEndpoint(t *testing.T) {
	var path string
	var got struct{ Events []errorEvent }
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	retry, err := sendErrors(&Config{ServerURL: srv.URL + "/", MachineCode: "MAY 01"},
		[]errorEvent{{Kind: "crash", Location: "service", Message: "panic: boom"}})
	if err != nil || retry {
		t.Fatalf("sendErrors = (%v, %v), muốn (false, nil)", retry, err)
	}
	if path != "/api/machines/by-code/MAY 01/errors" {
		t.Errorf("path = %q", path)
	}
	if len(got.Events) != 1 || got.Events[0].Message != "panic: boom" {
		t.Errorf("events = %+v", got.Events)
	}
}

// Mạng chưa lên / server đang khởi động lại: giữ lô lại thử sau. Server từ chối
// (mã máy chưa đăng ký): bỏ, thử lại mãi cũng vô ích và chặn mọi lỗi sau đó.
func TestSendErrors_RetryOnlyWhenWorthIt(t *testing.T) {
	cases := []struct {
		status int
		retry  bool
	}{{http.StatusBadGateway, true}, {http.StatusBadRequest, false}}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
		}))
		retry, err := sendErrors(&Config{ServerURL: srv.URL, MachineCode: "MAY01"}, []errorEvent{{Message: "x"}})
		srv.Close()
		if err == nil || retry != tc.retry {
			t.Errorf("status %d: (%v, %v), muốn retry=%v kèm lỗi", tc.status, retry, err, tc.retry)
		}
	}

	retry, err := sendErrors(&Config{ServerURL: "http://127.0.0.1:1", MachineCode: "MAY01"}, []errorEvent{{Message: "x"}})
	if err == nil || !retry {
		t.Errorf("không kết nối được: (%v, %v), muốn thử lại", retry, err)
	}
}

func TestReportPreviousCrash_QueuesFirstLineAndStack(t *testing.T) {
	for len(errorQueue) > 0 {
		<-errorQueue
	}
	crash := filepath.Join(t.TempDir(), "service.log.crash")
	stack := "panic: assignment to entry in nil map\n\ngoroutine 19 [running]:\nmain.runTelemetry()\n"
	if err := os.WriteFile(crash, []byte(stack), 0o644); err != nil {
		t.Fatal(err)
	}

	reportPreviousCrash(crash, "service")

	select {
	case e := <-errorQueue:
		if e.Kind != "crash" || e.Location != "service" || e.Message != "panic: assignment to entry in nil map" {
			t.Errorf("event = %+v", e)
		}
		if e.Detail != stack[:len(stack)-1] {
			t.Errorf("detail = %q", e.Detail)
		}
	default:
		t.Fatal("không có lỗi nào được xếp hàng")
	}

	// Tệp rỗng (lần trước thoát êm) thì không báo gì.
	_ = os.WriteFile(crash, nil, 0o644)
	reportPreviousCrash(crash, "service")
	if len(errorQueue) != 0 {
		t.Error("tệp crash rỗng mà vẫn báo")
	}
}
