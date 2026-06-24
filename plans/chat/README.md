## Tính năng Chat giữa Client & Admin

### Mô tả

Hệ thống chat real-time giữa admin web và desktop client (máy trạm):

- **Admin chat 1-1 với từng máy trạm**: Hỗ trợ kỹ thuật, hướng dẫn
- **Admin broadcast**: Gửi thông báo đến tất cả máy trạm
- **Máy trạm gửi yêu cầu hỗ trợ (SOS)**: Gamer gọi admin trợ giúp
- **Feedback / đánh giá dịch vụ** từ máy trạm

### Flow

```
Admin Web (Vue)                        Desktop Client (Wails)
      │                                        │
      │  ┌──────────────────────┐              │
      │  │ Chat Panel Component │ ◄── WebSocket ──│
      │  └──────────────────────┘              │
      │         │                              │
      │         ▼  REST API                    │
      │  ┌──────────────────────┐              │
      │  │  Chat Handler (Go)   │              │
      │  └────────┬─────────────┘              │
      │           │                            │
      │           ▼                            │
      │  ┌──────────────────────┐              │
      │  │  WebSocket Hub       │ ◄── broadcast │
      │  │  (gorilla/websocket) │              │
      │  └──────────────────────┘              │
```

### API Endpoints

```
GET    /api/chat/conversations                → Danh sách hội thoại
GET    /api/chat/conversations/:id            → Chi tiết hội thoại + messages
POST   /api/chat/conversations                → Tạo hội thoại mới
POST   /api/chat/conversations/:id/messages   → Gửi tin nhắn
GET    /api/chat/conversations/:id/messages   → Lịch sử tin nhắn (phân trang cursor)
POST   /api/chat/conversations/:id/read       → Đánh dấu đã đọc
DELETE /api/chat/conversations/:id            → Xóa hội thoại (soft delete)
```

### WebSocket Events

```
event: chat.message         → Tin nhắn mới
event: chat.typing          → Đang gõ
event: chat.read            → Đã đọc
event: chat.support_request → Yêu cầu hỗ trợ từ máy trạm (kèm machine_id)
```

### WebSocket Scaling Strategy

Khi chạy multi-instance backend, chat real-time cần cơ chế broadcast xuyên instance:

```
┌───────────────────────┐      ┌───────────────────┐
│  Backend Instance 1   │─────►│  Redis Pub/Sub    │
│  WebSocket Hub        │◄─────│  (broadcast)      │
└───────────────────────┘      └───────────────────┘
┌───────────────────────┐           │
│  Backend Instance 2   │───────────┘
│  WebSocket Hub        │
└───────────────────────┘

Client ───► Nginx (sticky session) ───► Instance 1 or 2
```

**Cơ chế:**
- Mỗi instance có WS Hub riêng, publish message lên Redis channel
- Các instance khác subscribe Redis channel → broadcast tới client của mình
- Nginx/HAProxy dùng sticky session (cookie) để client luôn về đúng instance
- Fallback: nếu Redis down → REST polling (client query mỗi 5s)

### Cấu trúc backend

```
backend/internal/chat/
├── handler.go        # HTTP handlers
├── service.go        # Business logic
├── repository.go     # Database queries
├── websocket.go      # WebSocket Hub + broadcast
└── model.go          # Chat structs
```

### Admin UI (Vue 3)
- Chat box góc phải màn hình (giống Messenger/Zalo)
- Danh sách hội thoại: tên máy + tin nhắn cuối + badge chưa đọc
- Chat popup real-time
- Gửi text, emoji
- Broadcast gửi tin đến tất cả máy online

### Desktop Client UI (Wails)
- Nút chat trên thanh taskbar / system tray
- Popup chat: danh sách hội thoại với admin
- Nút gửi yêu cầu hỗ trợ (SOS) — gửi kèm ảnh màn hình
- Popup thông báo khi admin nhắn tin

