package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"
)

type APIResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

// Hạn chót cho các lời gọi từ GIAO DIỆN lên máy chủ (App.doRequest). Trước đây
// chúng đi qua http.DefaultClient, vốn không có hạn chót nào: máy chủ nhận kết
// nối mà không trả lời thì lời gọi treo mãi — đăng nhập, khôi phục phiên, đăng
// xuất đều đứng im, và đường rơi sang bản lưu offline cũng không bao giờ được
// thử vì nó chỉ chạy khi lời gọi THẤT BẠI chứ không phải khi nó treo.
//
// Là biến để test rút ngắn được. Không lời gọi nào của giao diện cần lâu hơn:
// toàn JSON nhỏ, không có tải lên hay long-poll.
var (
	uiTimeout = 15 * time.Second
	// Trả máy: giao diện đã về màn hình khoá, còn cửa sổ phụ và trạng thái đăng
	// nhập bên Go chỉ dọn được SAU lời gọi này, nên không chờ lâu như lời gọi thường.
	logoutTimeout = 5 * time.Second

	// Trả máy mà không với tới máy chủ (mạng chớp, máy chủ đang khởi động lại) thì thử lại ở nền:
	// 5 giây, 15 giây, rồi mỗi phút. Nếu bỏ, phiên trên máy chủ chạy tiếp — máy vẫn gửi nhịp tim nên
	// máy chủ không coi là mất máy, khách đã về vẫn bị trừ tiền và máy kẹt "đang có người dùng".
	// Bỏ cuộc sau endRetryFor (tuổi thọ mặc định của access token; hết hạn thì máy chủ trả lời 9999
	// và vòng dừng sớm hơn).
	endRetryBackoff = []time.Duration{5 * time.Second, 15 * time.Second, time.Minute}
	endRetryFor     = 24 * time.Hour
)

func httpPost(url string, body interface{}) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(data)
	}

	log.Printf("[HTTP] POST %s body=%s", url, string(mustMarshal(body)))

	req, err := http.NewRequest("POST", url, reqBody)
	if err != nil {
		log.Printf("[HTTP] new request error: %v", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("[HTTP] do error: %v", err)
		return err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	log.Printf("[HTTP] POST %s → %d body=%s", url, resp.StatusCode, string(bodyBytes))
	return nil
}

func mustMarshal(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}
