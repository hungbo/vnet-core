package hub

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/vnet/core/internal/middleware"
)

var upgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type ClientType string

const ClientTypeAdmin  ClientType = "admin"
const ClientTypeClient ClientType = "client"

type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data,omitempty"`
}

type Client struct {
	conn        *websocket.Conn
	send        chan []byte
	ClientType  ClientType
	storeID     string
	machineCode string
	UserID      string
}

type Hub struct {
	mu             sync.RWMutex
	clients        map[*Client]bool
	machineClients map[string]*Client
	rooms          map[string]map[*Client]bool
	onConnect      func(client *Client) []string
	onDisconnect   func(client *Client)
}

func New() *Hub {
	return &Hub{
		clients:        make(map[*Client]bool),
		machineClients: make(map[string]*Client),
		rooms:          make(map[string]map[*Client]bool),
	}
}

func (h *Hub) OnConnect(cb func(client *Client) []string) {
	h.onConnect = cb
}

func (h *Hub) OnDisconnect(cb func(client *Client)) {
	h.onDisconnect = cb
}

func (h *Hub) JoinRoom(client *Client, roomID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[roomID] == nil {
		h.rooms[roomID] = make(map[*Client]bool)
	}
	h.rooms[roomID][client] = true
}

func (h *Hub) LeaveRoom(client *Client, roomID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[roomID] != nil {
		delete(h.rooms[roomID], client)
		if len(h.rooms[roomID]) == 0 {
			delete(h.rooms, roomID)
		}
	}
}

func (h *Hub) LeaveAllRooms(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for roomID, members := range h.rooms {
		delete(members, client)
		if len(members) == 0 {
			delete(h.rooms, roomID)
		}
	}
}

func (h *Hub) RemoveRoom(roomID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.rooms, roomID)
}

func (h *Hub) RemoveAllRooms() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.rooms = make(map[string]map[*Client]bool)
}

func (h *Hub) JoinRoomByUserID(userID, roomID string) {
	h.mu.RLock()
	var target *Client
	for client := range h.clients {
		if client.UserID == userID {
			target = client
			break
		}
	}
	h.mu.RUnlock()
	if target != nil {
		h.JoinRoom(target, roomID)
	}
}

func (h *Hub) JoinAllAdminsToRoom(roomID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[roomID] == nil {
		h.rooms[roomID] = make(map[*Client]bool)
	}
	for client := range h.clients {
		if client.ClientType == ClientTypeAdmin {
			h.rooms[roomID][client] = true
		}
	}
}

func (h *Hub) PublishToRoom(roomID string, event Event, skipUserID string) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("[WS] PublishToRoom marshal error: %v", err)
		return
	}

	h.mu.RLock()
	members := h.rooms[roomID]
	if len(members) == 0 {
		h.mu.RUnlock()
		return
	}

	for client := range members {
		if client.UserID == skipUserID {
			continue
		}
		select {
		case client.send <- data:
		default:
		}
	}
	h.mu.RUnlock()
}

func (h *Hub) HandleWS(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WS] upgrade error: %v", err)
		return
	}

	roleID := c.GetString(middleware.ContextKeyRoleID)
	userID := middleware.GetUserID(c)
	clientType := ClientTypeClient
	if roleID != "" {
		clientType = ClientTypeAdmin
	}

	machineCode := c.Request.URL.Query().Get("machine_code")

	log.Printf("[WS] New connection: roleID=%q userID=%q clientType=%s machineCode=%s", roleID, userID, clientType, machineCode)

	client := &Client{
		conn:        conn,
		send:        make(chan []byte, 64),
		ClientType:  clientType,
		machineCode: machineCode,
		UserID:      userID,
	}

	h.mu.Lock()
	h.clients[client] = true
	if machineCode != "" {
		h.machineClients[machineCode] = client
	}
	h.mu.Unlock()

	if h.onConnect != nil {
		rooms := h.onConnect(client)
		for _, roomID := range rooms {
			h.JoinRoom(client, roomID)
		}
	}

	go h.writePump(client)
	go h.readPump(client)
}

func (h *Hub) Broadcast(event Event) {
	log.Printf("[WS] Broadcast: type=%s", event.Type)
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("[WS] broadcast marshal error: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	log.Printf("[WS] Broadcast to %d clients", len(h.clients))

	for client := range h.clients {
		select {
		case client.send <- data:
		default:
			close(client.send)
			delete(h.clients, client)
			if client.machineCode != "" {
				delete(h.machineClients, client.machineCode)
			}
		}
	}
}

func (h *Hub) BroadcastToType(event Event, ct ClientType) {
	log.Printf("[WS] BroadcastToType: type=%s targetType=%s", event.Type, ct)
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("[WS] broadcast marshal error: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	count := 0
	for client := range h.clients {
		if client.ClientType != ct {
			continue
		}
		count++
		select {
		case client.send <- data:
		default:
			close(client.send)
			delete(h.clients, client)
			if client.machineCode != "" {
				delete(h.machineClients, client.machineCode)
			}
		}
	}
	log.Printf("[WS] BroadcastToType sent to %d/%d clients", count, len(h.clients))
}

func (h *Hub) SendToMachine(machineCode string, event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	h.mu.RLock()
	client, ok := h.machineClients[machineCode]
	h.mu.RUnlock()

	if !ok {
		return nil
	}

	select {
	case client.send <- data:
	default:
	}

	return nil
}

func (h *Hub) writePump(client *Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		client.conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.send:
			if !ok {
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *Hub) readPump(client *Client) {
	defer func() {
		if h.onDisconnect != nil {
			h.onDisconnect(client)
		}
		h.LeaveAllRooms(client)
		h.mu.Lock()
		delete(h.clients, client)
		if client.machineCode != "" {
			delete(h.machineClients, client.machineCode)
		}
		h.mu.Unlock()
		client.conn.Close()
	}()

	client.conn.SetReadLimit(4096)
	client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.conn.SetPongHandler(func(string) error {
		client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := client.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}
