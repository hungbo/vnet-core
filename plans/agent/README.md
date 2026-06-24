# VNET Agent & Security

## Agent Architecture

```
┌──────────────────────────────────────────────────────┐
│  VNET Client (Wails - User Space)                    │
│  - UI (Vue 3)                                        │
│  - Go backend (local business logic, offline cache)  │
│  └── IPC (Named Pipes: \\.\pipe\vnet_agent) ──────┐ │
└──────────────────────────────────────────────────────┘│
                                                        │
┌──────────────────────────────────────────────────────┐│
│  VNET Agent (Windows Service - SYSTEM privileges)    ││
│  - Screen lock (OS-level, không thể tắt Task Mgr)    ││
│  - Block Ctrl+Alt+Del / Alt+Tab / Win Key            ││
│  - Process whitelist/blacklist                       ││
│  - Hardware monitor (temp, processes, uptime)        ││
│  - Remote command handler                            ││
│  - Heartbeat watchdog: mất 30s → safe unlock         ││
└──────────────────────────────────────────────────────┘│
                                                        │
┌──────────────────────────────────────────────────────┐│
│  VNET Backend (Go API - Server)                      ││
│  - REST + WebSocket                                  ││
│  - PostgreSQL + Redis                                ││
│  - Row-level locking cho balance/stock               ││
└──────────────────────────────────────────────────────┘
```

### IPC Message Format

```json
// Server → Agent command
{
  "command_id": "uuid",
  "action": "shutdown | restart | lock | message | kill_process | list_processes | block_app | execute | screenshot | stream_start | stream_stop | input",
  "payload": {}
}

// Agent → Server response
{
  "command_id": "uuid",
  "status": "success | error",
  "error": "optional",
  "data": {}
}

// Stream mode: Agent push frame liên tục
{
  "action": "stream_frame",
  "data": { "image_jpeg_base64": "...", "timestamp": "..." }
}
```

### Cấu trúc Agent

```
agent/
├── main.go
├── ipc/
│   └── namedpipe.go       # Named pipe server
├── locker/
│   └── screen.go          # OS-level screen lock
├── monitor/
│   ├── hardware.go        # Collect all metrics
│   ├── cpu.go
│   ├── gpu.go
│   ├── disk.go
│   ├── network.go
│   ├── process.go
│   ├── alert.go           # Local threshold check + push
│   └── remote.go          # Remote command handler
└── watchdog/
    └── heartbeat.go       # Heartbeat watchdog
```

## Screen Lock & Client Agent

1. Member login → Client gửi IPC → Agent lock screen + block system keys
2. Timer chạy ở cả Client (UI) và Agent (watchdog)
3. Nếu Client crash, Agent vẫn giữ lock. Mất heartbeat 30s → safe unlock
4. Agent chạy SYSTEM privileges → không thể kill bằng Task Manager

## Hardware Monitor

Agent push snapshot mỗi 60 giây; vượt threshold → push alert ngay.

### Metrics

| Nhóm | Thông số | Nguồn |
|---|---|---|
| **CPU** | temp, usage %, frequency, fan speed | WMI / OpenHardwareMonitor |
| **GPU** | temp, usage %, memory used, fan speed, clock | NVIDIA-SMI / ADLX |
| **RAM** | total, used % | GlobalMemoryStatusEx |
| **Disk** | health (SMART), used %, read/write rate | WMI / SMART |
| **Network** | latency to server, packet loss | WinSock |
| **Processes** | top 5 (name, CPU, RAM, PID) | EnumProcesses |
| **Uptime** | OS uptime, Agent uptime | GetTickCount64 |

### Database

- `hardware_snapshots` — raw data mỗi 60s, giữ 7 ngày, purge
- `hardware_snapshots_hourly` — aggregate lưu vĩnh viễn
- `hardware_alert_thresholds` — config ngưỡng
- `hardware_alerts` — lịch sử cảnh báo

### Admin UI Dashboard

- Health overview (🟢🟡🔴) cho tất cả máy
- CPU/GPU temp chart (24h, 7d, 30d)
- RAM/Disk usage chart
- Lịch sử alerts + resolve button
- Notification push khi có alert mới

## Remote Control

Tất cả actions yêu cầu permission tương ứng (remote.*), không cần password admin.

| Action | API | Win32 API | Permission |
|---|---|---|---|
| Shutdown | `POST /remote/shutdown` | `ExitWindowsEx(EWX_SHUTDOWN)` | remote.shutdown |
| Restart | `POST /remote/restart` | `ExitWindowsEx(EWX_REBOOT)` | remote.restart |
| Lock | `POST /remote/lock` | `LockWorkStation()` | remote.lock |
| Message | `POST /remote/message` | `MessageBox` / custom toast | remote.message |
| Kill process | `POST /remote/kill` | `TerminateProcess()` | remote.kill |
| List processes | `GET /remote/processes` | `CreateToolhelp32Snapshot` | remote.processes |
| Block app | `POST /remote/block-app` | WMI auto-kill | remote.block |
| Execute | `POST /remote/execute` | `CreateProcess()` | remote.execute |
| Screenshot | `GET /remote/screenshot` | `BitBlt` → JPEG base64 | remote.screenshot |
| Stream screen | `WS /remote/stream` | BitBlt loop 1fps → WS push | remote.screenshot |
| Input remote | `POST /remote/input` | `SendInput()` | remote.input |

Admin UI route: `/machines/:id/remote`

## Client Machine UI (Wails Desktop)

### Kiến trúc file

```
client/
├── main.go              # Entry point
├── app.go               # Go backend + Wails Bindings
│   ├── Login(method, credential) → Session
│   ├── Logout() → void
│   ├── GetSession() → Session
│   ├── GetMenu(categoryId) → Product[]
│   ├── PlaceOrder(items) → Order
│   ├── GetTopupPresets() → number[]   (từ system_settings.topup_presets)
│   ├── RequestTopup(amount) → void    (gửi qua chat)
│   ├── SendMessage(text) → Message
│   ├── SendScreenshot() → Message
│   ├── GetHardware() → Snapshot
│   ├── ChangePin(old, new) → void
│   └── GetNotifications() → Notification[]
├── frontend/
│   ├── src/
│   │   ├── views/
│   │   │   ├── LockScreen.vue       # QR/PIN login
│   │   │   ├── Dashboard.vue        # Timer + topup + quick actions
│   │   │   ├── ServiceMenu.vue      # F&B ordering
│   │   │   ├── TopupDialog.vue      # Nạp tiền popup
│   │   │   ├── ChatBoard.vue        # Chat với admin
│   │   │   ├── SettingsPage.vue     # PIN, about
│   │   │   └── NotificationList.vue
│   │   ├── components/
│   │   │   ├── TimerWidget.vue      # Countdown
│   │   │   ├── BalanceDisplay.vue   # Số dư + KM
│   │   │   ├── ProductCard.vue      # Menu item
│   │   │   ├── CartPanel.vue        # Cart sidebar
│   │   │   └── SosButton.vue       # Emergency
│   │   ├── stores/
│   │   │   ├── session.store.ts
│   │   │   ├── order.store.ts
│   │   │   └── chat.store.ts
│   │   └── assets/
│   ├── package.json
│   └── vite.config.ts
└── wails.json
```

### Màn hình & Giao diện

**1. Lock Screen — Màn hình chờ / đăng nhập**

```
┌────────────────────────────────────┐
│         [LOGO VNET]                │
│                                     │
│  ┌──────────┬───────────┐          │
│  │ 📝 Tài khoản │ 📱 QR   │  ← Tab │
│  └──────────┴───────────┘          │
│                                     │
│  ┌── Tab: Tài khoản ──────────┐    │
│  │  Tài khoản: [____________]  │    │
│  │  Mật khẩu:  [____________]  │    │
│  │                             │    │
│  │  [Đăng nhập]               │    │
│  └─────────────────────────────┘    │
│                                     │
│  ┌── Tab: QR ─────────────────┐     │
│  │  ┌────────────────────┐    │     │
│  │  │   Camera view      │    │     │
│  │  │   (quét QR code)   │    │     │
│  │  └────────────────────┘    │     │
│  └─────────────────────────────┘    │
└────────────────────────────────────┘
```

Server check `members.role` sau khi login:
- `member` → Member Dashboard (timer, balance, topup, order, chat)
- `combo` → Combo Dashboard (timer combo, order, chat — ẩn topup)
- `admin` → Admin Dashboard ("Quản trị", chat — ẩn timer/topup/order)

**Member Dashboard — Sau khi đăng nhập (hội viên):**

```
┌────────────────────────────────────┐
│ 🟢 M03-VIP    👤 Nguyễn Văn A     │
│                                    │
│  ┌────────────────────────────┐    │
│  │  ⏱ 2:35:42 còn lại        │    │
│  │  💰 Số dư: 150k (KM 50k)  │    │
│  │  💠 Combo: Prepaid 5h      │    │
│  └────────────────────────────┘    │
│                                    │
│  ┌──────┐ ┌────────┐ ┌────────┐   │
│  │  💳  │ │  🍕   │ │  💬   │   │
│  │ Nạp  │ │ Đồ ăn │ │ Chat  │   │
│  │ tiền │ │        │ │       │   │
│  └──────┘ └────────┘ └────────┘   │
│                                    │
│  ┌────────────────────────────┐    │
│  │ 🔔 Giải đấu Valorant tối   │    │
│  └────────────────────────────┘    │
│                                    │
│  [Đăng xuất]            [Cài đặt] │
└────────────────────────────────────┘
```

**Combo Dashboard — Sau khi đăng nhập (combo role):**

```
┌────────────────────────────────────┐
│ 🟢 M03        👤 SANGCHIEU-001    │
│                                    │
│  ┌────────────────────────────┐    │
│  │  ⏱ 4:15:23 còn lại        │    │
│  │  💠 Combo: 5h Prepaid      │    │
│  └────────────────────────────┘    │
│                                    │
│  ┌──────┐ ┌────────┐ ┌────────┐   │
│  │  🚫  │ │  🍕   │ │  💬   │   │
│  │ Nạp  │ │ Đồ ăn │ │ Chat  │   │
│  │ tiền │ │        │ │       │   │
│  │(tắt) │ └────────┘ └────────┘   │
│  └──────┘                         │
│                                    │
│  [Đăng xuất]            [Cài đặt] │
└────────────────────────────────────┘
```

**Admin Dashboard — Sau khi đăng nhập (admin role):**

```
┌────────────────────────────────────┐
│ 🔧 Admin    M03-VIP  🟢 Online    │
│ 👤 Kane (Admin)                    │
│                                    │
│  ┌────────────────────────────┐    │
│  │  Không tính giờ            │    │
│  │  Chế độ quản trị           │    │
│  └────────────────────────────┘    │
│                                    │
│  ┌──────┐ ┌────────┐ ┌────────┐   │
│  │  🕐  │ │  🚫   │ │  💬   │   │
│  │ Giờ  │ │ Đồ ăn │ │ Chat  │   │
│  │ chơi │ │ (tắt) │ │       │   │
│  └──────┘ └────────┘ └────────┘   │
│                                    │
│  [Đăng xuất]            [Cài đặt] │
└────────────────────────────────────┘
```

Khác biệt với Member Dashboard:
- Timer hiển thị "Không tính giờ" thay vì đếm ngược
- Nút 💳 Nạp tiền ❌ ẩn
- Nút 🍕 Đồ ăn ❌ ẩn (hiển thị icon 🚫)
- Tất cả chức năng khác giống hệt: Chat, Cài đặt, Game Launcher, Notifications
- Backend: `POST /api/auth/admin-login` → session có `is_admin=true`, không tạo machine_sessions

**3. Topup Dialog — Nạp tiền**

Khi click nút 💳 Nạp tiền:

```
┌────────────────────────────────────┐
│  💳 Nạp tiền                    × │
│  Số dư: 150k  (KM 50k)           │
│                                    │
│  Chọn số tiền nạp:                │
│  ┌──────┐ ┌──────┐ ┌──────┐       │
│  │ 5k   │ │ 10k  │ │ 20k  │       │
│  └──────┘ └──────┘ └──────┘       │
│  ┌──────┐ ┌──────┐ ┌──────┐       │
│  │ 50k  │ │ 100k │ │ 200k │       │
│  └──────┘ └──────┘ └──────┘       │
│  ┌──────┐ ┌──────┐ ┌──────────┐   │
│  │ 500k │ │ 1tr  │ │ Nhập số… │   │
│  └──────┘ └──────┘ └──────────┘   │
│                                    │
│  ┌────────────────────────────┐    │
│  │  Số tiền: 100,000₫        │    │
│  └────────────────────────────┘    │
│                                    │
│  [💬 Gửi yêu cầu qua Chat]        │
│  📌 Admin phản hồi sau khi         │
│     nhận tiền mặt / QR             │
└────────────────────────────────────┘
```

Các mệnh giá lấy từ `system_settings.topup_presets` (cấu hình trên admin web). Mặc định: `[5000, 10000, 20000, 50000, 100000, 200000, 500000, 1000000]`

**4. Service Menu — Gọi đồ ăn/uống**

```
┌────────────────────────────────────┐
│ ← Quay lại       🍕 Đồ ăn & Nước  │
│                                    │
│ [Đồ uống] [Đồ ăn] [Snack]         │
│                                    │
│ ┌──────┐ ┌──────┐ ┌──────┐        │
│ │Coca  │ │Pepsi │ │Sting │        │
│ │15k   │ │15k   │ │12k   │        │
│ │[+]   │ │[+]   │ │[+]   │        │
│ └──────┘ └──────┘ └──────┘        │
│                                    │
│ ┌──── Giỏ hàng ────────────────┐   │
│ │ Coca x 2 = 30k   [−] 2 [+]  │   │
│ │ Tổng: 30k                    │   │
│ │ Số dư: 150k                  │   │
│ │ [Đặt hàng]                   │   │
│ └──────────────────────────────┘   │
└────────────────────────────────────┘
```

**5. Chat Board**

```
┌────────────────────────────────────┐
│ 💬 Hỗ trợ                    [×]   │
│                                    │
│ ┌────────────────────────────────┐ │
│ │ Admin: Chào bạn, cần giúp gì? │ │
│ │ 🕐 14:30                      │ │
│ │                                │ │
│ │ Bạn: 💳 Nạp 100k              │ │
│ │ 🕐 14:31                      │ │
│ │                                │ │
│ │ Admin: ✅ Đã nạp 100k          │ │
│ │ Số dư: 250k (KM 50k) 🕐 14:32 │ │
│ └────────────────────────────────┘ │
│                                    │
│ [________________________] [📷] ▶ │
│                                    │
│ [🆘 Yêu cầu hỗ trợ gấp]          │
└────────────────────────────────────┘
```

## Website Blocking (Khống chế Website)

Agent thực thi chặn website qua **Hosts file** (`C:\Windows\System32\drivers\etc\hosts`), cần admin privilege.

### Agent implementation

**Thêm file:** `agent/monitor/website.go`

```go
// Sync rules từ server mỗi 60s
func (w *WebsiteBlocker) Sync() {
    rules := GET /api/website-rules/active
    w.UpdateHostsFile(rules)
}

// Ghi hosts file
func (w *WebsiteBlocker) UpdateHostsFile(rules []Rule) {
    // Thêm 127.0.0.1 <domain> cho mỗi rule type='block'
    // Xóa entry khi rule bị deactivate hoặc hết schedule
    // Format: "127.0.0.1 facebook.com"
}

// Kiểm tra schedule
func (w *WebsiteBlocker) IsRuleActive(rule Rule) bool {
    // Check day_of_week  có match hôm nay?
    // Check start_time <= now < end_time?
    // Nếu không có schedule → luôn active
}
```

### Sync flow

```
Admin web → Server (CRUD) → Push WS "website-rules:updated"
                                                      ↓
Agent nhận notification → Sync() → UpdateHostsFile() → reload
```

### Default block categories

Category mặc định (admin có thể thêm/sửa/xóa):
- `social`: facebook.com, youtube.com, tiktok.com...
- `gaming`: trochoiviet.com, gamevui.vn...
- `streaming`: netflix.com, hulu.com...
- `adult`: * (wildcard)
- `p2p`: torrent sites
- `custom`: admin tự nhập pattern

### Violation detection

- Agent monitor DNS resolution log
- Nếu process truy cập domain bị chặn → ghi `website_violations`
- Admin xem violations log trong `/settings/website-blocking`

### Flow Topup (Client → Admin)

```
Máy trạm                     Server                    Admin Web
   │                           │                          │
   ├─ Click 💳 Nạp tiền        │                          │
   ├─ Chọn 100k               │                          │
   ├─ [Gửi yêu cầu] ────────► │                          │
   │                           ├─ Tạo chat message:       │
   │                           │  "M03 yêu cầu nạp 100k"  │
   │                           ├─ Push notification ────► │
   │                           │                  ┌──────┴──────┐
   │                           │                  │ Admin chọn  │
   │                           │                  │ [Xác nhận]  │
   │                           │◄─────────────────│ hoặc nhập số│
   │                           │                  │ tiền thực tế│
   │                           ├─ Topup transaction        │
   │                           ├─ balance += 100k          │
   │◄──── chat message ────────┤                           │
   │ "✅ Đã nạp 100k thành công!"                         │
   │ Số dư: 250k                                           │
```

## Security Threat Model

| # | Vector | Mức |
|---|---|---|
| 1 | Client timer pause | 🔴 Cao |
| 2 | Heartbeat spoofing | 🔴 Cao |
| 3 | Offline mode replay | 🔴 Cao |
| 4 | Race condition balance | 🔴 Cao |
| 5 | Combo sharing | 🟡 TB |
| 6 | Local SQLite cache edit | 🟡 TB |
| 7 | System clock tampering | 🟡 TB |
| 8 | Agent kill | 🟡 TB |
| 9 | IP/MAC spoofing | 🟡 TB |
| 10 | Brute force PIN | 🔵 Thấp |

### Biện pháp

1. **Server-side timer** — session timer trên server, client chỉ UI
2. **Double-spend protection** — `SELECT FOR UPDATE` + idempotency key
3. **Offline mode an toàn** — max 15 phút, HMAC cache, validate khi reconnect
4. **Agent integrity** — hash binary, heartbeat, watchdog
5. **Rate limiting** — request/s/IP cho endpoint nhạy cảm
6. **Combo anti-sharing** — 1 combo = 1 current_session_id

## Offline Mode & Fallback

```
Client (Wails)                    Server (Go API)
     │                                  │
     │──── WebSocket heartbeat ────────►│
     │◄──── ack ────────────────────────│
     │                                  │
     │  [Mất kết nối]                  │
     │  ├─ Pause timer (đóng băng giờ) │
     │  ├─ Lưu local SQLite cache      │
     │  └─ Retry reconnect every 3s   │
     │                                  │
     │  [Kết nối lại]                  │
     │  ├─ Sync cached data lên server │
     │  ├─ Resume timer từ điểm dừng   │
     │  └─ Continue normal flow        │
```

- Heartbeat mỗi 5s, mất 3 liên tiếp → offline mode
- Local SQLite có HMAC signature chống edit
- Offline tối đa 15 phút, sau đó auto lock
