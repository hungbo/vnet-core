package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// serviceName là tên dịch vụ trong services.msc. Bộ cài và lệnh gỡ đều dùng nó.
const (
	serviceName = "VNETClient"
	// Tên hiển thị: bộ lọc sự kiện của tác vụ canh chừng so theo chuỗi này, nên
	// đổi nó ở một chỗ mà quên chỗ kia là tác vụ im lặng không bao giờ kích.
	serviceDisplayName = "VNET Client Agent"
)

// runAgent là phần việc nền, KHÔNG phụ thuộc Windows.
//
// Trước đây toàn bộ những goroutine này khởi động trong App.Startup, tức là sống
// chết theo cửa sổ giao diện: khách tắt cửa sổ là máy im lặng hoàn toàn với máy
// chủ, không nhịp tim, không nhận lệnh nào. Tách ra đây để chúng chạy từ lúc bật
// máy, kể cả khi chưa ai đăng nhập.
//
// Trả về khi ctx bị huỷ.
func runAgent(ctx context.Context) {
	cfg := LoadConfig()
	log.Printf("[agent] máy %s → %s", cfg.MachineCode, cfg.ServerURL)

	go runTelemetry(ctx, cfg)
	go newWebBlocker(cfg).run(ctx)
	go runAgentWS(ctx, cfg)
	go superviseUI(ctx)

	<-ctx.Done()
}

// runAgentWS giữ kết nối WebSocket bằng KHOÁ MÁY, không phải token người dùng.
//
// Đây là phần vá lỗ hổng lớn nhất: bản cũ chỉ nối sau khi khách đăng nhập, bằng
// token của khách, nên máy không có ai ngồi thì không có kết nối nào và nhân
// viên KHÔNG tắt hay khoá được máy trống — đúng lúc cần nhất.
//
// Tiến trình nền chỉ xử lý những lệnh làm được ở session 0. Khoá màn hình và
// hiện thông báo là việc của giao diện: hook bàn phím và cửa sổ không tồn tại ở
// session 0. Nhờ hub gửi lệnh tới MỌI kết nối của một mã máy, hai bên cùng nhận
// và mỗi bên bỏ qua phần không thuộc mình.
func runAgentWS(ctx context.Context, cfg *Config) {
	if cfg.AgentToken == "" {
		log.Printf("[agent] chưa có khoá máy — bỏ qua WebSocket, chỉ gửi nhịp tim")
		return
	}

	ws := NewAgentWSClient(ctx, cfg)
	ws.On("remote:shutdown", func(WSMessage) { shutdownMachine() })
	ws.On("remote:restart", func(WSMessage) { restartMachine() })
	ws.On("remote:block-app", func(msg WSMessage) { blockAppByName(remoteField(msg, "app")) })
	ws.On("remote:unblock-app", func(msg WSMessage) { unblockAppByName(remoteField(msg, "app")) })
	ws.Run()
}

// remoteField bóc một trường chuỗi khỏi payload lồng hai lớp
// {"data":{"payload":{...}}} mà máy chủ gửi xuống.
func remoteField(msg WSMessage, key string) string {
	var outer struct {
		Payload map[string]interface{} `json:"payload"`
	}
	if err := json.Unmarshal(msg.Payload, &outer); err != nil {
		return ""
	}
	if v, ok := outer.Payload[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// superviseUI giữ cho giao diện luôn chạy trong phiên của người dùng.
//
// Đây là lý do tiến trình nền phải là dịch vụ chứ không phải một mục tự khởi
// động thường: khách tắt giao diện trong Task Manager thì phải có ai đó bật lại,
// và cái "ai đó" không được nằm cùng thuyền với nó.
func superviseUI(ctx context.Context) {
	const checkEvery = 5 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(checkEvery):
		}

		if uiIsRunning() {
			continue
		}
		if err := launchUIInUserSession(); err != nil {
			// Chưa ai đăng nhập vào Windows là trạng thái bình thường, không phải
			// lỗi — đừng làm ngập nhật ký.
			log.Printf("[agent] chưa bật được giao diện: %v", err)
		}
	}
}

// runService là điểm vào của chế độ --service.
//
// Trên Windows nó chạy dưới Service Control Manager; ở nơi khác (chỉ dùng khi
// phát triển) nó chạy như một tiến trình thường để còn thử được logic nền.
func runService() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if runAsWindowsService(ctx, cancel) {
		return
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	runAgent(ctx)
}
