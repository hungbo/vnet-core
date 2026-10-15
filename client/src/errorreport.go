package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Máy trạm gửi lỗi của nó về server (POST /api/machines/by-code/:code/errors),
// hiện trong trang "Nhật ký lỗi" của admin.
//
// Nhật ký trên máy (logfile.go) nằm ở ổ C:, mà máy chạy không ổ cứng thì C:
// làm mới mỗi lần boot — gửi về server là cách duy nhất để lỗi còn lại tới tay
// dev. Ba thứ được gửi: crash của lần chạy trước, lỗi JS của giao diện, và
// game không mở được.

type errorEvent struct {
	Kind     string `json:"kind"`
	Location string `json:"location"`
	Message  string `json:"message"`
	Detail   string `json:"detail,omitempty"`
	Version  string `json:"version"`
}

// Server nhận tối đa 20 lỗi mỗi lần (service.MachineErrorReportRequest).
const errorBatchMax = 20

var errorQueue = make(chan errorEvent, 50)

// reportError không bao giờ chặn: hàng đợi đầy thì bỏ. Lỗi dồn dập thì vài mẫu
// đầu là đủ — server gộp và đếm số lần.
func reportError(kind, location, message, detail string) {
	select {
	case errorQueue <- errorEvent{Kind: kind, Location: location, Message: message, Detail: detail, Version: version}:
	default:
	}
}

// ReportUIError: giao diện Vue gửi lỗi JS qua đây. Console của WebView2 không
// được lưu ở đâu cả, nên trước đây lỗi giao diện trên máy khách là vô hình.
func (a *App) ReportUIError(message, detail, location string) {
	reportError("error", "ui:"+location, message, detail)
}

// runErrorReporter gửi theo lô mỗi 10 giây. Mạng chưa lên (vừa boot) hay server
// đang khởi động lại thì giữ lô lại thử lần sau; server từ chối (4xx — mã máy
// chưa đăng ký) thì bỏ, thử lại cũng vô ích.
func runErrorReporter() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[error-report] dừng vì panic: %v", r)
		}
	}()
	var pending []errorEvent
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	for range tick.C {
	gom:
		for len(pending) < errorBatchMax {
			select {
			case e := <-errorQueue:
				pending = append(pending, e)
			default:
				break gom
			}
		}
		if len(pending) == 0 {
			continue
		}
		retry, err := sendErrors(LoadConfig(), pending)
		if err != nil {
			log.Printf("[error-report] không gửi được %d lỗi: %v", len(pending), err)
			if retry {
				continue
			}
		}
		pending = pending[:0]
	}
}

func sendErrors(cfg *Config, events []errorEvent) (retry bool, err error) {
	body, err := json.Marshal(map[string]any{"events": events})
	if err != nil {
		return false, err
	}
	endpoint := fmt.Sprintf("%s/api/machines/by-code/%s/errors",
		strings.TrimRight(cfg.ServerURL, "/"), url.PathEscape(cfg.MachineCode))
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return true, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 500 {
		return true, fmt.Errorf("server trả %d", resp.StatusCode)
	}
	if resp.StatusCode >= 300 {
		return false, fmt.Errorf("server từ chối: %d", resp.StatusCode)
	}
	return false, nil
}

// reportPreviousCrash: Go ghi stack của lần sập vào tệp crash (logfile.go, qua
// debug.SetCrashOutput). Tệp còn nội dung lúc khởi động nghĩa là lần chạy trước
// đã sập — dịch vụ tự bật lại sau 5 giây, giao diện do dịch vụ bật lại — nên đây
// là lúc gửi nó đi.
func reportPreviousCrash(crashPath, process string) {
	data, err := os.ReadFile(crashPath)
	if err != nil {
		return
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return
	}
	first := strings.TrimSpace(strings.SplitN(text, "\n", 2)[0])
	log.Printf("[crash] lần chạy trước của %s bị sập: %s", process, first)
	reportError("crash", process, first, text)
}
