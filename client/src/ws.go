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
}

func NewWSClient(ctx context.Context, baseURL, token string, machineCode string) *WSClient {
	c := &WSClient{
		ctx:         ctx,
		baseURL:     baseURL,
		token:       token,
		machineCode: machineCode,
		handlers:    make(map[string]WSHandler),
	}

	c.On("session:updated", func(msg WSMessage) {
		runtime.EventsEmit(ctx, "vnet:session:updated", string(msg.Payload))
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

	c.On("rooms:cleared", func(msg WSMessage) {
		runtime.EventsEmit(ctx, "vnet:rooms:cleared")
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
	header.Set("Authorization", "Bearer "+c.token)
	log.Printf("[WS] token prefix: %s...", safePrefix(c.token, 20))

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
