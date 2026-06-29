# VNET Core

Hệ thống quản lý tiệm net toàn diện — quản lý máy trạm, hội viên, ca trực, đặt chỗ,
bán hàng, báo cáo doanh thu và hỗ trợ khách hàng qua chat real-time.

> **Phiên bản:** 0.1.0-dev | **License:** AGPL-3.0

## Architecture

```
┌─────────────────────────────────────────────────┐
│                   Admin (Vue 3)                  │
│       Soybean Admin · Element Plus · UnoCSS      │
└──────────────────────┬──────────────────────────┘
                       │ HTTP / WS
┌──────────────────────▼──────────────────────────┐
│            Backend (Go + Gin + GORM)             │
│  REST API · WebSocket · JWT · PostgreSQL · Redis │
└──────────────────────┬──────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────┐
│           Client (Wails v2 — Windows)            │
│         Chat widget + machine monitoring          │
└─────────────────────────────────────────────────┘
```

## Features

| Module | Endpoints |
|--------|-----------|
| **Auth** | Login, Refresh, QR login, Member login |
| **Members** | CRUD, groups, transactions, sessions, combo history |
| **Machines** | CRUD, groups, pricing, assets, heartbeat, remote action |
| **Sessions** | Start/end, switch machine, cost calculation |
| **Bookings** | CRUD, check-in, cancel, no-show |
| **Orders** | CRUD, split, pay, status |
| **Products / Inventory** | Ingredients, stock transactions, suppliers, warehouses |
| **Combos** | CRUD, purchase, activate |
| **Promotions** | CRUD, lucky spin |
| **Chat** | Real-time rooms, messages, topup requests |
| **Shifts** | Open, close, handover |
| **Reports** | Daily/monthly revenue, by member/machine/employee, top products |
| **Curfew** | Schedule, override |
| **Settings / Audit / Backups** | System config, audit trail, DB backup & restore |

## Tech Stack

| Layer | Technology |
|-------|------------|
| Backend | Go 1.25, [Gin](https://github.com/gin-gonic/gin), [GORM](https://gorm.io), PostgreSQL, Redis |
| Admin | Vue 3.5, [Soybean Admin](https://github.com/soybeanjs/soybean-admin), Element Plus, UnoCSS, Vite 7 |
| Client | [Wails v2](https://wails.io), Go + Vue |
| Auth | JWT (access + refresh tokens) |
| Real-time | Gorilla WebSocket |
| Docs | Swagger (swaggo) |

## Quick Start

### Prerequisites

- Go 1.25+
- Node.js 20.19+ & pnpm 8.6+
- PostgreSQL 15+
- (Optional) Wails v2 — for client desktop app

### 1. Backend

```bash
cd backend

# Config — set env vars or use defaults (DB_HOST, DB_USER, DB_PASSWORD, DB_NAME, JWT_SECRET)
cp .env.example .env   # if exists

# Run with hot-reload
go run ./cmd/server
# or: air
```

### 2. Admin

```bash
cd admin
pnpm install
pnpm dev         # → http://localhost:3000 (proxies /api → :8080)
```

### 3. Seed (optional)

```bash
cd backend
go run ./cmd/seed
# Creates: admin/admin123, manager/admin123, staff/admin123
```

## Project Structure

```
vnet-core/
├── backend/
│   ├── cmd/
│   │   ├── server/          # Entrypoint (Gin, auto-migrate, routes)
│   │   ├── seed/            # DB seed
│   │   └── migrate/         # DB migration
│   └── internal/
│       ├── config/          # Env-based config
│       ├── handler/         # HTTP handlers
│       ├── service/         # Business logic
│       ├── model/           # GORM models
│       ├── middleware/      # Auth, store context
│       ├── router/          # Route registration
│       ├── hub/             # WebSocket hub
│       └── pkg/             # JWT, response helpers
├── admin/
│   └── src/
│       ├── views/vnet/      # 22 feature views
│       ├── hooks/chat/      # Chat composables
│       └── locales/         # en, vi, zh
├── client/
│   ├── src/                 # Go backend (Wails app)
│   └── frontend/src/        # Vue UI (ChatWidget)
├── scripts/
│   ├── build-server.sh      # Admin dist + Go server → single binary
│   └── build-client.sh      # Windows desktop → zip
├── AGENTS.md                # Dev conventions
└── VERSION
```

## Build & Deployment

```bash
# Production server (embeds admin dist)
bash scripts/build-server.sh v1.0.0

# Windows desktop client
bash scripts/build-client.sh v1.0.0
```

## License

GNU Affero General Public License v3.0. See [LICENSE](./LICENSE).
