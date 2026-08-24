package hub

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/vnet/core/internal/middleware"
)

// originAllowed reports whether a websocket handshake from origin is
// acceptable. An empty Origin header comes from non-browser clients (the
// desktop agent), which are not subject to cross-site request forgery.
//
// host là Host của chính request. Trang quản trị được nhúng thẳng vào binary và
// phục vụ cùng cổng với API, nên đường bình thường nhất — mở
// http://localhost:8080 — là CÙNG ORIGIN. Bản cũ chỉ so chuỗi với
// ALLOWED_ORIGINS, nên khai "http://localhost" mà mở ở cổng 8080 là trượt: mọi
// handshake trả 403, WebSocket không bao giờ nối được, và toàn bộ phần thời gian
// thực chết lặng — không lỗi trên màn hình, chỉ là không có gì xảy ra nữa.
//
// Cùng origin thì không có nguy cơ CSRF để mà chặn, nên luôn cho qua.
func originAllowed(allowed []string, origin, host string) bool {
	if origin == "" {
		return true
	}
	if host != "" {
		if u, err := url.Parse(origin); err == nil && u.Host == host {
			return true
		}
	}
	for _, a := range allowed {
		if a == "*" || a == origin {
			return true
		}
	}
	return false
}

type ClientType string

const ClientTypeAdmin ClientType = "admin"
const ClientTypeClient ClientType = "client"

type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data,omitempty"`
}

type Client struct {
	conn        *websocket.Conn
	send        chan []byte
	ClientType  ClientType
	machineCode string
	UserID      string
	closeOnce   sync.Once
}

type Hub struct {
	mu       sync.RWMutex
	upgrader websocket.Upgrader
	clients  map[*Client]bool
	// Một mã máy có NHIỀU kết nối: tiến trình nền chạy như dịch vụ Windows giữ
	// một kết nối 24/7 bằng khoá máy, còn giao diện giữ một kết nối riêng bằng
	// token hội viên. Bản cũ là map[string]*Client nên bên nối sau đá bên nối
	// trước ra, và lệnh điều khiển chỉ tới được đúng một trong hai.
	machineClients map[string][]*Client
	rooms          map[string]map[*Client]bool
	onConnect      func(client *Client) []string
	onDisconnect   func(client *Client)
}

func New(allowedOrigins []string) *Hub {
	return &Hub{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return originAllowed(allowedOrigins, r.Header.Get("Origin"), r.Host)
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		clients:        make(map[*Client]bool),
		machineClients: make(map[string][]*Client),
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

// JoinRoomByUserID nối MỌI kết nối của một người vào phòng.
//
// Mọi, chứ không phải kết nối đầu tiên tìm thấy: máy trạm mở cửa sổ hỗ trợ
// thành TIẾN TRÌNH RIÊNG, mỗi tiến trình một WebSocket, nên cùng một tài khoản
// thường có hai kết nối sống song song — thanh điều khiển và cửa sổ chat. Chọn
// một cái thì tin nhắn hay rơi đúng vào cái KHÔNG hiển thị gì, và triệu chứng
// là chat lúc được lúc không mà log phía máy chủ vẫn báo gửi thành công.
//
// userID rỗng thì thoát: kết nối của tiến trình nền không có tài khoản, và so
// sánh với chuỗi rỗng sẽ kéo tất cả chúng vào phòng.
func (h *Hub) JoinRoomByUserID(userID, roomID string) {
	if userID == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[roomID] == nil {
		h.rooms[roomID] = make(map[*Client]bool)
	}
	for client := range h.clients {
		if client.UserID == userID {
			h.rooms[roomID][client] = true
		}
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

	// defer chứ không RUnlock thủ công ở cuối: hàm này gửi vào channel của người
	// khác, và bất kỳ panic nào ở giữa cũng không được phép bỏ lại khoá đọc —
	// mất khoá là cả hub đứng chứ không chỉ hỏng một tin nhắn.
	h.mu.RLock()
	defer h.mu.RUnlock()

	members := h.rooms[roomID]
	if len(members) == 0 {
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
}

func (h *Hub) HandleWS(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
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
		h.machineClients[machineCode] = append(h.machineClients[machineCode], client)
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

// removeClient drops a client from every registry and closes its send channel
// exactly once. Both broadcast paths and readPump funnel through here, so the
// close has to survive a concurrent second call.
func (h *Hub) removeClient(client *Client) {
	h.mu.Lock()
	delete(h.clients, client)

	// Gỡ khỏi mọi phòng chat NGAY TẠI ĐÂY, không để người gọi tự nhớ.
	//
	// Trước đây chỉ nhánh readPump gọi LeaveAllRooms; hai nhánh loại client
	// nghẽn trong Broadcast và BroadcastToType thì không. Client bị loại vẫn
	// nằm lại trong h.rooms với channel ĐÃ ĐÓNG, và lần PublishToRoom sau đó
	// panic "send on closed channel" — select/default không đỡ được, gửi vào
	// channel đã đóng luôn panic.
	//
	// Hậu quả nặng hơn một cú panic: PublishToRoom mở RLock mà không defer, nên
	// panic bỏ lại khoá đọc đang giữ và toàn bộ hub đứng — mọi WebSocket mới
	// treo ở h.mu.Lock(). Vòng lặp gửi cũng đứt giữa chừng, nên vài client kịp
	// nhận còn số còn lại thì không.
	for roomID, members := range h.rooms {
		delete(members, client)
		if len(members) == 0 {
			delete(h.rooms, roomID)
		}
	}

	// Chỉ gỡ ĐÚNG client này khỏi danh sách của mã máy. Các kết nối khác cùng mã
	// (dịch vụ nền, hoặc một lần nối lại vừa đăng ký) phải được giữ nguyên.
	if client.machineCode != "" {
		list := h.machineClients[client.machineCode]
		for i, c := range list {
			if c == client {
				h.machineClients[client.machineCode] = append(list[:i], list[i+1:]...)
				break
			}
		}
		if len(h.machineClients[client.machineCode]) == 0 {
			delete(h.machineClients, client.machineCode)
		}
	}
	h.mu.Unlock()

	client.closeOnce.Do(func() { close(client.send) })
}

// Broadcast is a no-op on a nil Hub so services wired without a websocket hub
// (and the tests that construct them that way) stay usable.
func (h *Hub) Broadcast(event Event) {
	if h == nil {
		return
	}

	log.Printf("[WS] Broadcast: type=%s", event.Type)
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("[WS] broadcast marshal error: %v", err)
		return
	}

	// Mutating the registries under the read lock would be a data race, so
	// stalled clients are collected here and evicted after the lock is gone.
	h.mu.RLock()
	log.Printf("[WS] Broadcast to %d clients", len(h.clients))
	var stale []*Client
	for client := range h.clients {
		select {
		case client.send <- data:
		default:
			stale = append(stale, client)
		}
	}
	h.mu.RUnlock()

	for _, client := range stale {
		h.removeClient(client)
	}
}

func (h *Hub) BroadcastToType(event Event, ct ClientType) {
	if h == nil {
		return
	}

	log.Printf("[WS] BroadcastToType: type=%s targetType=%s", event.Type, ct)
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("[WS] broadcast marshal error: %v", err)
		return
	}

	h.mu.RLock()
	count := 0
	var stale []*Client
	for client := range h.clients {
		if client.ClientType != ct {
			continue
		}
		count++
		select {
		case client.send <- data:
		default:
			stale = append(stale, client)
		}
	}
	log.Printf("[WS] BroadcastToType sent to %d/%d clients", count, len(h.clients))
	h.mu.RUnlock()

	for _, client := range stale {
		h.removeClient(client)
	}
}

// ErrMachineOffline và ErrMachineBusy cho phép người gọi phân biệt "đã gửi"
// với "không gửi được". Trước đây cả hai trường hợp đều trả nil, nên lệnh điều
// khiển từ xa báo thành công dù không máy nào nhận được.
var (
	ErrMachineOffline = errors.New("máy trạm chưa kết nối")
	ErrMachineBusy    = errors.New("hàng đợi gửi tới máy trạm đã đầy")
)

// IsMachineOnline cho biết máy trạm có đang giữ kết nối WebSocket hay không.
func (h *Hub) IsMachineOnline(machineCode string) bool {
	if h == nil {
		return false
	}
	h.mu.RLock()
	n := len(h.machineClients[machineCode])
	h.mu.RUnlock()
	return n > 0
}

// SendToMachine đẩy một sự kiện tới đúng máy trạm.
//
// Trả về ErrMachineOffline nếu máy chưa kết nối, ErrMachineBusy nếu hàng đợi
// đầy. Người gọi nào chỉ muốn thông báo "gửi được thì tốt" (ví dụ báo nạp tiền
// thành công) cứ bỏ qua giá trị trả về như cũ.
// SendToAdminsAndMachine gửi một sự kiện cho toàn bộ máy quản trị VÀ đúng một
// máy trạm.
//
// Đây là đích đúng cho những sự kiện mang dữ liệu của MỘT hội viên mà máy trạm
// của chính họ cũng cần: phiên chơi, số dư. Broadcast thì tiện nhưng gửi tới mọi
// client kể cả máy trạm của khách khác — khách nào mở DevTools trên máy trạm là
// đọc được tên và số tiền của người ngồi máy bên cạnh.
//
// Máy trạm không kết nối KHÔNG phải lỗi: khách có thể vừa tắt máy, và phía quản
// trị vẫn phải nhận được sự kiện.
// SendToUser gửi tới MỌI kết nối của một tài khoản, bất kể tài khoản đó đang
// ngồi máy nào — hoặc không ngồi máy nào cả.
//
// Sinh ra cho sự kiện số dư. SendToMachine cần mã máy, mà những đường đổi số dư
// như nạp tiền hay hoàn tiền chỉ biết member_id: nhân viên nạp cho khách đang
// đứng ở quầy, chưa vào phiên nào, thì không có mã máy để mà gửi.
//
// "Mọi kết nối" là chủ ý: máy trạm mở cửa sổ phụ thành tiến trình riêng nên một
// tài khoản thường có vài WebSocket cùng lúc, và cái nào cũng đang vẽ số dư lên
// màn hình.
func (h *Hub) SendToUser(userID string, event Event) {
	if h == nil || userID == "" {
		return
	}

	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("[WS] SendToUser marshal error: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		if client.UserID != userID {
			continue
		}
		select {
		case client.send <- data:
		default:
		}
	}
}

func (h *Hub) SendToAdminsAndMachine(machineCode string, event Event) {
	if h == nil {
		return
	}
	h.BroadcastToType(event, ClientTypeAdmin)
	if machineCode != "" {
		_ = h.SendToMachine(machineCode, event)
	}
}

func (h *Hub) SendToMachine(machineCode string, event Event) error {
	if h == nil {
		return ErrMachineOffline
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	h.mu.RLock()
	list := append([]*Client(nil), h.machineClients[machineCode]...)
	h.mu.RUnlock()

	if len(list) == 0 {
		return ErrMachineOffline
	}

	// Gửi tới MỌI kết nối của máy đó. Dịch vụ nền lo tắt/khởi động lại/chặn ứng
	// dụng; giao diện lo khoá màn hình và hiện thông báo — mỗi bên bỏ qua lệnh
	// không thuộc phần mình, nên không lệnh nào bị làm hai lần.
	//
	// Chỉ báo bận khi KHÔNG kết nối nào nhận được: một bên nghẽn không được làm
	// hỏng lệnh đã tới được bên kia.
	delivered := 0
	for _, client := range list {
		select {
		case client.send <- data:
			delivered++
		default:
		}
	}
	if delivered == 0 {
		return ErrMachineBusy
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
		h.removeClient(client)
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
