package hub

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient registers a client with a send buffer of the given capacity so
// tests can force the "receiver cannot keep up" path.
func newTestClient(h *Hub, ct ClientType, machineCode string, buffer int) *Client {
	c := &Client{
		send:        make(chan []byte, buffer),
		ClientType:  ct,
		machineCode: machineCode,
	}

	h.mu.Lock()
	h.clients[c] = true
	if machineCode != "" {
		h.machineClients[machineCode] = append(h.machineClients[machineCode], c)
	}
	h.mu.Unlock()

	return c
}

// Services are wired with a nil hub in unit tests, and a nil receiver used to
// crash the whole package.
func TestNilHub_MethodsAreNoOps(t *testing.T) {
	var h *Hub

	assert.NotPanics(t, func() { h.Broadcast(Event{Type: "order:new"}) })
	assert.NotPanics(t, func() { h.BroadcastToType(Event{Type: "order:new"}, ClientTypeAdmin) })
	assert.NotPanics(t, func() {
		// Hub nil nghĩa là không có máy nào kết nối được — báo offline, không
		// được báo thành công.
		assert.ErrorIs(t, h.SendToMachine("PC-01", Event{Type: "remote:shutdown"}), ErrMachineOffline)
	})
}

func TestBroadcast_DeliversToEveryClient(t *testing.T) {
	h := New(nil)
	a := newTestClient(h, ClientTypeAdmin, "", 4)
	b := newTestClient(h, ClientTypeClient, "PC-01", 4)

	h.Broadcast(Event{Type: "order:new"})

	assert.Len(t, a.send, 1)
	assert.Len(t, b.send, 1)
}

func TestBroadcastToType_OnlyTargetsMatchingType(t *testing.T) {
	h := New(nil)
	admin := newTestClient(h, ClientTypeAdmin, "", 4)
	client := newTestClient(h, ClientTypeClient, "PC-01", 4)

	h.BroadcastToType(Event{Type: "chat:message"}, ClientTypeAdmin)

	assert.Len(t, admin.send, 1)
	assert.Len(t, client.send, 0)
}

// A client whose buffer is full is evicted. The eviction used to happen under
// the read lock, racing with readPump's cleanup.
func TestBroadcast_EvictsClientThatCannotKeepUp(t *testing.T) {
	h := New(nil)
	stalled := newTestClient(h, ClientTypeClient, "PC-01", 0)

	h.Broadcast(Event{Type: "order:new"})

	h.mu.RLock()
	_, stillRegistered := h.clients[stalled]
	machineStillRegistered := len(h.machineClients["PC-01"]) > 0
	h.mu.RUnlock()

	assert.False(t, stillRegistered)
	assert.False(t, machineStillRegistered)

	_, open := <-stalled.send
	assert.False(t, open, "send channel should be closed after eviction")
}

// Two broadcasts racing on the same stalled client used to close its channel
// twice and panic.
func TestBroadcast_ConcurrentEvictionClosesChannelOnce(t *testing.T) {
	h := New(nil)
	newTestClient(h, ClientTypeClient, "PC-01", 0)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.Broadcast(Event{Type: "order:new"})
		}()
	}
	wg.Wait()

	h.mu.RLock()
	remaining := len(h.clients)
	h.mu.RUnlock()
	assert.Equal(t, 0, remaining)
}

// removeClient is reached from both broadcast and readPump teardown.
func TestRemoveClient_IsIdempotent(t *testing.T) {
	h := New(nil)
	c := newTestClient(h, ClientTypeClient, "PC-01", 1)

	assert.NotPanics(t, func() {
		h.removeClient(c)
		h.removeClient(c)
	})
}

// A reconnect registers a new client under the same machine code; tearing down
// the stale one must not unregister the live one.
func TestRemoveClient_KeepsReconnectedMachineEntry(t *testing.T) {
	h := New(nil)
	stale := newTestClient(h, ClientTypeClient, "PC-01", 1)
	fresh := newTestClient(h, ClientTypeClient, "PC-01", 1)

	h.removeClient(stale)

	h.mu.RLock()
	registered := append([]*Client(nil), h.machineClients["PC-01"]...)
	h.mu.RUnlock()
	require.Len(t, registered, 1)
	assert.Same(t, fresh, registered[0])
}

// Dịch vụ nền và giao diện cùng khai một mã máy. Bản cũ giữ đúng một client mỗi
// mã nên bên nối sau đá bên nối trước ra, và lệnh điều khiển chỉ tới được một
// trong hai — máy trống thì không ai nhận lệnh tắt.
func TestSendToMachine_ReachesEveryConnectionOfTheMachine(t *testing.T) {
	h := New(nil)
	service := newTestClient(h, ClientTypeClient, "PC-01", 4)
	ui := newTestClient(h, ClientTypeClient, "PC-01", 4)
	other := newTestClient(h, ClientTypeClient, "PC-02", 4)

	require.NoError(t, h.SendToMachine("PC-01", Event{Type: "remote:shutdown"}))

	assert.Len(t, service.send, 1, "dịch vụ nền phải nhận được lệnh")
	assert.Len(t, ui.send, 1, "giao diện phải nhận được lệnh")
	assert.Len(t, other.send, 0, "máy khác không được nhận")

	// Giao diện tắt: dịch vụ nền vẫn phải nhận lệnh, đây là cả điểm của việc
	// tách tiến trình.
	h.removeClient(ui)
	require.NoError(t, h.SendToMachine("PC-01", Event{Type: "remote:shutdown"}))
	assert.Len(t, service.send, 2)
}

func TestSendToMachine_RoutesByMachineCode(t *testing.T) {
	h := New(nil)
	target := newTestClient(h, ClientTypeClient, "PC-01", 4)
	other := newTestClient(h, ClientTypeClient, "PC-02", 4)

	assert.NoError(t, h.SendToMachine("PC-01", Event{Type: "machine:shutdown"}))

	assert.Len(t, target.send, 1)
	assert.Len(t, other.send, 0)
}

// Gửi tới máy chưa kết nối PHẢI báo lỗi. Trước đây trả nil, nên lệnh điều
// khiển từ xa báo thành công dù không máy nào nhận được.
func TestSendToMachine_UnknownMachineIsAnError(t *testing.T) {
	h := New(nil)

	assert.ErrorIs(t, h.SendToMachine("PC-404", Event{Type: "remote:shutdown"}), ErrMachineOffline)
}

func TestSendToMachine_DeliversToConnectedMachine(t *testing.T) {
	h := New(nil)
	c := newTestClient(h, ClientTypeClient, "PC-01", 4)

	assert.NoError(t, h.SendToMachine("PC-01", Event{Type: "remote:lock"}))
	assert.Len(t, c.send, 1)
}

func TestSendToMachine_FullQueueIsAnError(t *testing.T) {
	h := New(nil)
	newTestClient(h, ClientTypeClient, "PC-01", 1)

	assert.NoError(t, h.SendToMachine("PC-01", Event{Type: "remote:lock"}))
	assert.ErrorIs(t, h.SendToMachine("PC-01", Event{Type: "remote:lock"}), ErrMachineBusy)
}

func TestIsMachineOnline(t *testing.T) {
	h := New(nil)
	newTestClient(h, ClientTypeClient, "PC-01", 4)

	assert.True(t, h.IsMachineOnline("PC-01"))
	assert.False(t, h.IsMachineOnline("PC-404"))

	var nilHub *Hub
	assert.False(t, nilHub.IsMachineOnline("PC-01"))
}

// Một tài khoản có thể có nhiều kết nối cùng lúc: máy trạm mở cửa sổ hỗ trợ
// thành tiến trình riêng, mỗi tiến trình một WebSocket. Nối thiếu một cái là
// tin nhắn rơi vào kết nối không hiển thị gì.
func TestJoinRoomByUserID_ReachesEveryConnectionOfTheUser(t *testing.T) {
	h := New(nil)

	thanhDieuKhien := newTestClient(h, ClientTypeClient, "PC-01", 1)
	cuaSoChat := newTestClient(h, ClientTypeClient, "PC-01", 1)
	nguoiKhac := newTestClient(h, ClientTypeClient, "PC-02", 1)

	h.mu.Lock()
	thanhDieuKhien.UserID = "member-1"
	cuaSoChat.UserID = "member-1"
	nguoiKhac.UserID = "member-2"
	h.mu.Unlock()

	h.JoinRoomByUserID("member-1", "room-1")
	h.PublishToRoom("room-1", Event{Type: "chat:message"}, "")

	assert.Len(t, thanhDieuKhien.send, 1, "thanh điều khiển phải nhận được")
	assert.Len(t, cuaSoChat.send, 1, "cửa sổ chat phải nhận được")
	assert.Len(t, nguoiKhac.send, 0, "người khác phòng không được nhận")
}

// userID rỗng là kết nối của tiến trình nền. So sánh với chuỗi rỗng mà không
// chặn thì mọi kết nối như thế bị kéo vào phòng chat của người lạ.
func TestJoinRoomByUserID_BoQuaUserIDRong(t *testing.T) {
	h := New(nil)
	nen := newTestClient(h, ClientTypeClient, "PC-01", 1)

	h.JoinRoomByUserID("", "room-1")
	h.PublishToRoom("room-1", Event{Type: "chat:message"}, "")

	assert.Len(t, nen.send, 0)
}

func TestOriginAllowed(t *testing.T) {
	tests := []struct {
		name    string
		allowed []string
		origin  string
		host    string
		want    bool
	}{
		{"non-browser client sends no Origin", []string{"https://admin.example.com"}, "", "", true},
		{"listed origin", []string{"https://admin.example.com"}, "https://admin.example.com", "", true},
		{"unlisted origin", []string{"https://admin.example.com"}, "https://evil.example.com", "", false},
		{"wildcard", []string{"*"}, "https://evil.example.com", "", true},
		{"empty allowlist", nil, "https://evil.example.com", "", false},

		// Trang quản trị nhúng trong binary chạy cùng cổng với API, nên đường
		// dùng bình thường nhất là cùng origin. Bản cũ đòi khai đúng scheme +
		// host + CỔNG trong ALLOWED_ORIGINS; khai "http://localhost" rồi mở ở
		// cổng 8080 là trượt, và cả phần thời gian thực chết lặng.
		{"cùng origin dù không khai", nil, "http://localhost:8080", "localhost:8080", true},
		{"cùng origin, allowlist khai thiếu cổng", []string{"http://localhost"}, "http://localhost:8080", "localhost:8080", true},
		{"khác cổng thì vẫn là khác origin", nil, "http://localhost:9999", "localhost:8080", false},
		{"trang lạ mạo Host cũng không qua", nil, "https://evil.example.com", "localhost:8080", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, originAllowed(tc.allowed, tc.origin, tc.host))
		})
	}
}
