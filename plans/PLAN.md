# VNET Core — Hệ thống Quản lý Phòng Game (Open Source)

## Tổng quan

VNET Core là hệ thống quản lý phòng game mã nguồn mở.  
Cafe tự host local backend, admin web, desktop client, và windows agent.  
**VNET Cloud Hub** (central SaaS sync) và **VNET Mobile** (app cho gamer) là sản phẩm riêng — liên hệ author để biết thêm.

| Module | Chức năng | Phase |
|---|---|---|
| **VNET Core** | Quản lý tính tiền, hội viên, máy trạm | Phase 1 |
| **VNET F&B** | Quản lý F&B, POS, kho hàng | Phase 1 |
| **VNET Up** | Cập nhật & đồng bộ game | Phase 2 |

## Công nghệ

| Layer | Technology |
|---|---|
| **Backend API** | Go (Gin/Echo/Fiber) |
| **Desktop Client UI** | Wails (Go + Vue 3) |
| **Client Agent (OS-level)** | Windows Service (Go/Named Pipes IPC) |
| **Admin Web** | Vue 3 + Vite + Pinia + Element Plus |
| **Database** | PostgreSQL |
| **Cache / Real-time** | Redis + WebSocket (gorilla/websocket) |

## Kiến trúc tổng thể

```
┌──────────────────────────────────────────────┐
│             VNET Admin                       │
│         (Vue 3 Web App)                      │
└──────────────────┬───────────────────────────┘
                   │ HTTPS (REST + WebSocket)
                   ▼
┌──────────────────────────────────────────────┐
│         VNET Backend (Go API)                │
│  - Gin/Echo HTTP server                      │
│  - JWT Auth middleware                       │
│  - WebSocket hub (gorilla/websocket)         │
│  - Redis pub/sub cho real-time               │
│  - GORM/sqlx for PostgreSQL                  │
│  - Row-level locking (SELECT FOR UPDATE)     │
└──────┬───────────────────────────────┬───────┘
       │ HTTPS                         │ WS + IPC
       ▼                               ▼
┌────────────────────┐    ┌───────────────────────────────┐
│  VNET Client       │    │  VNET Agent (Windows Service) │
│  (Wails Desktop)   │    │  - Screen Lock                │
│  - Vue 3 UI        │    │  - HW Monitor                 │
│  - Go backend      │    │  - IPC (Named Pipes)          │
│  - Local cache     │    │  - Heartbeat watchdog         │
└────────────────────┘    └───────────────────────────────┘
```

## Modules

| Module | Mô tả | Tài liệu |
|---|---|---|
| **VNET Core** | Hội viên, Máy, Combo, Khuyến mãi, Booking, Curfew, Bonus balance | [core/](core/README.md) |
| **VNET F&B** | Menu, Sản phẩm, Order, POS, Inventory, Printers | [fnb/](fnb/README.md) |
| **VNET Agent & Security** | Agent architecture, Screen lock, HW Monitor, Remote Control, Threat Model, Offline Mode | [agent/](agent/README.md) |
| **API Design** | REST endpoints cho tất cả modules + conventions | [api/](api/README.md) |
| **Business Logic Flows** | Session lifecycle, Pricing engine, Combo activation, Shift close, Tier upgrade, Refund, Booking | [core/business.md](core/business.md) |
| **Admin UI** | Vue Router routes, Component tree, Pinia stores, POS layout | [ui/](ui/README.md) |
| **Database** | All CREATE TABLE, indexes, enums, seed data | [db/](db/README.md) |
| **Chat** | Chat flow, WebSocket, API, UI | [chat/](chat/README.md) |

## Tính năng hệ thống

| Tính năng | Mô tả |
|---|---|
| **Sao lưu & Khôi phục** | Auto backup database, restore khi cần |
| **Cập nhật phiên bản** | Auto-update cho desktop client + agent |
| **Thông số hệ thống** | Cấu hình giá, grace period, làm tròn tiền, màu sắc... |
| **Khống chế website** | Chặn web theo danh sách (qua Windows Agent) |
| **Quản lý ứng dụng** | Whitelist/blacklist app trên máy trạm |
| **Multi-store** | Quản lý nhiều chi nhánh — thiết kế từ Phase 1 |

## Lộ trình

| Phase | Nội dung | Thời gian |
|---|---|---|
| **Phase 0** | Setup Go project, Vue admin, Wails client, Docker, CI/CD | 1 tuần |
| **Phase 1** | VNET Core: Auth, Hội viên, Máy, Tính giờ, Báo cáo, Chat | 6-8 tuần |
| **Phase 1.1** | VNET F&B: Menu, POS, Payment split, Order, Kho, Printer | 6-8 tuần |
| **Phase 2** | VNET Agent (Windows Service), FastUp | 8-12 tuần |
| **Phase 3** | E-Invoice, Multi-store, Integrations nâng cao | 4-6 tuần |

## Third-party Integrations

| Đối tác | Chức năng | Phase |
|---|---|---|
| **VietQR / Momo** | Thanh toán QR, nạp tiền hội viên | Phase 1 |
| **Bank QR (OCB, TPBank...)** | Thanh toán QR ngân hàng | Phase 1 |
| **MISA / MeInvoice** | Hóa đơn điện tử | Phase 2 |
| **Viettel / VNPT** | Hóa đơn điện tử (thay thế) | Phase 2 |
| **CCCD API** | Xác thực tuổi, check minor | Phase 1 |
| **Zalo OA** | Gửi thông báo, OTP, marketing | Phase 2 |

## Cấu trúc thư mục

```
vnet/
├── backend/                    # Go backend
│   ├── cmd/server/             # Entry point
│   ├── internal/
│   │   ├── auth/
│   │   ├── members/
│   │   ├── machines/
│   │   ├── assets/             # Machine asset management
│   │   ├── billing/
│   │   ├── bookings/           # Machine booking & deposit
│   │   ├── orders/
│   │   ├── products/           # VNET F&B products
│   │   ├── inventory/          # VNET F&B inventory
│   │   ├── printer/            # Printer routing
│   │   ├── reports/
│   │   ├── chat/
│   │   ├── curfew/             # Minor policy & curfew
│   │   ├── einvoice/           # E-invoice integration
│   │   ├── realtime/           # WebSocket
│   │   └── settings/
│   ├── pkg/
│   │   ├── database/
│   │   ├── middleware/
│   │   └── utils/
│   ├── migrations/
│   ├── go.mod
│   └── go.sum
├── admin/                      # Vue 3 admin web
│   ├── src/
│   │   ├── views/
│   │   ├── stores/
│   │   ├── components/
│   │   ├── router/
│   │   └── api/
│   ├── package.json
│   └── vite.config.ts
├── client/                     # Wails desktop client
│   ├── app.go
│   ├── main.go
│   ├── frontend/
│   └── wails.json
├── agent/                      # VNET Windows Service
│   ├── main.go
│   ├── ipc/                    # Named pipe server
│   ├── locker/                 # OS-level screen lock
│   ├── monitor/                # Hardware + process monitoring
│   └── watchdog/               # Heartbeat watchdog
├── plans/                      # Project plans & docs
│   ├── PLAN.md                 # This file
│   ├── core/
│   ├── fnb/
│   ├── agent/
│   ├── api/
│   ├── ui/
│   ├── db/
│   └── chat/
├── docker-compose.yml
├── opencode.jsonc
└── README.md
```
