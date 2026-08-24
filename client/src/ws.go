package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type WSMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"data"`
}

type WSHandler func(msg WSMessage)

type WSClient struct {
	ctx         context.Context
	baseURL     string
	token       string
	machineCode string
	conn        *websocket.Conn
	handlers    map[string]WSHandler

	// agentToken khác rỗng nghĩa là nối bằng KHOÁ MÁY thay vì token người dùng.
	// Tiến trình nền dùng đường này để giữ kết nối kể cả khi chưa ai đăng nhập —
	// bản cũ chỉ nối sau khi khách đăng nhập, nên máy trống không nhận lệnh nào.
	agentToken string
}

// NewAgentWSClient dựng một kết nối cho tiến trình nền: xác thực bằng khoá máy,
// KHÔNG đăng ký handler nào cho giao diện (không có cửa sổ để mà phát sự kiện).
func NewAgentWSClient(ctx context.Context, cfg *Config) *WSClient {
	return &WSClient{
		ctx:         ctx,
		baseURL:     cfg.ServerURL,
		machineCode: cfg.MachineCode,
		agentToken:  cfg.AgentToken,
		handlers:    make(map[string]WSHandler),
	}
}

// Run nối và chạy tới khi context bị huỷ, tự nối lại khi rớt.
func (c *WSClient) Run() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}
		if err := c.Connect(c.ctx); err != nil {
			return
		}
	}
}

func NewWSClient(ctx context.Context, baseURL, token string, machineCode string) *WSClient {
	c := &WSClient{
		ctx:         ctx,
		baseURL:     baseURL,
		token:       token,
		machineCode: machineCode,
		handlers:    make(map[string]WSHandler),
	}

	c.On("session:started", func(msg WSMessage) {
		runtime.EventsEmit(ctx, "vnet:session:updated", string(msg.Payload))
	})

	c.On("session:ended", func(msg WSMessage) {
		runtime.EventsEmit(ctx, "vnet:session:updated", string(msg.Payload))
	})

	// Máy chủ tự đóng phiên: hết tiền, hết khung giờ, hoặc giới nghiêm. Không có
	// handler này thì đồng hồ trên máy chạy tiếp như chưa có gì xảy ra, cho tới
	// khi khách tự tải lại.
	c.On("session:auto-ended", func(msg WSMessage) {
		runtime.EventsEmit(ctx, "vnet:session:ended", string(msg.Payload))
	})

	c.On("chat:message", func(msg WSMessage) {
		runtime.EventsEmit(ctx, "vnet:chat:message", string(msg.Payload))
	})

	c.On("message:status:updated", func(msg WSMessage) {
		runtime.EventsEmit(ctx, "vnet:message:status:updated", string(msg.Payload))
	})

	c.On("balance:updated", func(msg WSMessage) {
		var data map[string]interface{}
		if json.Unmarshal(msg.Payload, &data) == nil {
			runtime.EventsEmit(ctx, "vnet:balance:updated", data)
		}
	})

	c.On("notification:new", func(msg WSMessage) {
		var data map[string]interface{}
		if json.Unmarshal(msg.Payload, &data) == nil {
			runtime.EventsEmit(ctx, "vnet:notification:new", data)
		}
	})

	c.On("topup:confirmed", func(msg WSMessage) {
		var data map[string]interface{}
		if json.Unmarshal(msg.Payload, &data) == nil {
			runtime.EventsEmit(ctx, "vnet:topup:confirmed", data)
		}
	})

	c.On("room:read", func(msg WSMessage) {
		runtime.EventsEmit(ctx, "vnet:room:read", string(msg.Payload))
	})

	c.On("rooms:cleared", func(msg WSMessage) {
		runtime.EventsEmit(ctx, "vnet:rooms:cleared")
	})

	// The backend has always sent these two; without handlers the client kept
	// showing a room staff had just deleted, and missed rooms staff opened.
	c.On("room:new", func(msg WSMessage) {
		runtime.EventsEmit(ctx, "vnet:room:new", string(msg.Payload))
	})

	c.On("room:deleted", func(msg WSMessage) {
		runtime.EventsEmit(ctx, "vnet:room:deleted", string(msg.Payload))
	})

	return c
}

func (c *WSClient) On(eventType string, handler WSHandler) {
	c.handlers[eventType] = handler
}

func (c *WSClient) Connect(ctx context.Context) error {
	wsHost := strings.TrimPrefix(c.baseURL, "http://")
	wsScheme := "ws://"
	if strings.HasPrefix(c.baseURL, "https://") {
		wsHost = strings.TrimPrefix(c.baseURL, "https://")
		wsScheme = "wss://"
	}
	u := fmt.Sprintf("%s%s/api/ws/client?machine_code=%s", wsScheme, wsHost, c.machineCode)
	log.Printf("[WS] connecting to %s", u)

	if wsHost == "" {
		log.Printf("[WS] empty host, retrying in 5s")
		time.Sleep(5 * time.Second)
		return c.Connect(ctx)
	}

	header := http.Header{}
	switch {
	case c.agentToken != "":
		header.Set("X-Agent-Token", c.agentToken)
		log.Printf("[WS] nối bằng khoá máy %s", c.machineCode)
	case c.token != "":
		header.Set("Authorization", "Bearer "+c.token)
		log.Printf("[WS] token prefix: %s...", safePrefix(c.token, 20))
	default:
		// Không gửi header nào cả. Khoá máy là tuỳ chọn, nên tiến trình nền của
		// một máy chưa cấp khoá chỉ cần mã máy. Gửi "Bearer " rỗng thì máy chủ
		// vẫn cho qua, nhưng nó nói dối về ý định và người đọc nhật ký sau này
		// sẽ đi tìm xem token biến đâu mất.
		log.Printf("[WS] nối bằng mã máy %s, không kèm khoá", c.machineCode)
	}

	for {
		select {
		case <-ctx.Done():
			log.Printf("[WS] context done: %v", ctx.Err())
			return ctx.Err()
		default:
		}

		var err error
		c.conn, _, err = websocket.DefaultDialer.Dial(u, header)
		if err != nil {
			log.Printf("[WS] dial error: %v, retry in 5s", err)
			time.Sleep(5 * time.Second)
			continue
		}
		log.Printf("[WS] connected: %s", u)

		if err := c.readLoop(ctx); err != nil {
			log.Printf("[WS] read loop error: %v, reconnecting", err)
			c.close()
			time.Sleep(5 * time.Second)
		}
	}
}

func safePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func (c *WSClient) readLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return err
		}

		var msg WSMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("[WS] unmarshal error: %v raw=%s", err, string(data))
			continue
		}

		log.Printf("[WS] received: type=%s data=%s", msg.Type, string(msg.Payload))

		if handler, ok := c.handlers[msg.Type]; ok {
			go handler(msg)
		} else {
			log.Printf("[WS] no handler for type=%s", msg.Type)
		}
	}
}

func (c *WSClient) close() {
	if c.conn != nil {
		c.conn.Close()
	}
}
