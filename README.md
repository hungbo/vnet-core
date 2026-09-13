# VNET Core

Hệ thống quản lý tiệm net toàn diện — quản lý máy trạm, hội viên, ca trực, đặt chỗ,
bán hàng, báo cáo doanh thu và hỗ trợ khách hàng qua chat real-time.

> **Phiên bản:** 0.1.0-dev | **License:** AGPL-3.0

> **Hướng dẫn sử dụng** — triển khai, cài máy trạm và mọi chức năng, viết cho người
> dùng chứ không cho lập trình viên: [docs/HUONG-DAN.md](./docs/HUONG-DAN.md).

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
| **Products / Inventory** | Ingredients, stock transactions, suppliers |
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
pnpm dev         # → http://localhost:20900 (proxies /api → :20800)
```

### 3. Migrate & seed (lần đầu)

```bash
cd backend
go run ./cmd/migrate   # vá cột, chỉ mục — chạy SAU khi server đã dựng bảng
go run ./cmd/seed      # tạo admin/admin123, manager/admin123, staff/admin123
go run ./cmd/seed -menu  # thêm thực đơn mẫu 100 món (mì, cơm, nước, đồ ăn vặt)
```

`-menu` là dữ liệu để thử nghiệm, không phải dữ liệu quán. Nó tạo bốn danh mục
và 100 món giá theo mặt bằng quán net, kèm ảnh SVG sinh ra ngay lúc chạy và ghi
vào `UPLOAD_DIR`. Chạy lại nhiều lần được: món đã có thì bỏ qua, món từng bị xoá
thì khôi phục — không món nào bị ghi đè giá.

`cmd/migrate` giả định các bảng đã tồn tại (server tự dựng qua AutoMigrate lúc
khởi động lần đầu), nên phải chạy server trước, migrate sau — không ngược lại.

Đổi ba mật khẩu mặc định ngay sau khi đăng nhập lần đầu.

## Project Structure

```
vnet-core/
├── backend/
│   ├── cmd/
│   │   ├── server/          # Entrypoint (Gin, auto-migrate, routes)
│   │   ├── seed/            # DB seed
│   │   └── migrate/         # DB migration
│   ├── internal/
│   │   ├── config/          # Env-based config
│   │   ├── database/        # Kết nối GORM
│   │   ├── handler/         # HTTP handlers
│   │   ├── service/         # Business logic
│   │   ├── model/           # GORM models
│   │   ├── middleware/      # Auth, phân quyền
│   │   ├── router/          # Route registration
│   │   ├── scheduler/       # Tác vụ định kỳ (giới nghiêm, tự trả máy, xếp hạng)
│   │   └── hub/             # WebSocket hub
│   └── pkg/                 # JWT, pagination, response, utils
├── admin/
│   └── src/
│       ├── views/vnet/      # 30 trang nghiệp vụ
│       ├── views/system/    # Người dùng, vai trò, menu (Soybean)
│       ├── hooks/chat/      # Chat composables
│       └── locales/         # en, vi, zh
├── client/
│   ├── src/                 # Go (Wails app) — khoá máy, chặn web, chụp màn hình
│   └── src/frontend/src/    # Vue UI (điểm danh, đánh giá, nạp tiền, cài đặt)
├── scripts/
│   ├── build-server.sh      # Admin dist + Go server → single binary
│   ├── build-client.sh      # Máy trạm Windows → exe + zip + SHA-256 + bộ cài
│   ├── verify.py            # phép kiểm đầu-cuối trên hệ thống thật
│   ├── check-admin.py       # Dò khoá i18n, nhóm menu, nút câm
│   └── KIEM-CHUNG-WINDOWS.md # Kiểm tay phần chỉ chạy trên Windows
├── installer/               # Bộ cài Inno Setup cho máy trạm
├── docker-compose.yml       # Postgres + app + migrate/seed (profile tools)
├── .env.docker.example      # Mẫu cấu hình triển khai
├── AGENTS.md                # Dev conventions
└── VERSION
```

## Deployment

### Docker (khuyến nghị)

Giao diện quản trị được nhúng thẳng vào binary, nên **một cổng duy nhất** phục vụ
cả giao diện lẫn API — không cần nginx hay web server riêng.

```bash
cp .env.docker.example .env
```

Điền ba giá trị **bắt buộc** trong `.env` — Compose từ chối khởi động nếu thiếu,
và ở chế độ release server cũng tự từ chối nếu giá trị không an toàn:

| Biến | Yêu cầu |
|------|---------|
| `DB_PASSWORD` | không có mặc định, tự đặt |
| `JWT_SECRET` | tối thiểu 32 ký tự — `openssl rand -base64 48` |
| `ALLOWED_ORIGINS` | địa chỉ thật, ngăn cách bằng dấu phẩy; `*` bị từ chối |
| `HARDWARE_HISTORY_DAYS` | số ngày giữ lịch sử phần cứng, mặc định 7; `0` = giữ tất cả |

```bash
docker compose up -d --build
docker compose --profile tools run --rm migrate
docker compose --profile tools run --rm seed
docker compose --profile tools run --rm seed -menu   # tuỳ chọn: thực đơn mẫu 100 món
```

Mở `http://localhost:20800` (hoặc `APP_PORT` bạn đặt) và đăng nhập bằng
`admin` / `admin123`, rồi **đổi mật khẩu ngay** ở menu avatar.

```bash
docker compose logs -f app   # xem log
docker compose down          # dừng, GIỮ dữ liệu
docker compose down -v       # dừng và XOÁ SẠCH database
```

Đổi `APP_PORT` thì phải đổi `ALLOWED_ORIGINS` theo cho khớp.

### Binary thủ công

```bash
bash scripts/build-server.sh v1.0.0   # vnet-server, đã nhúng admin dist
bash scripts/build-client.sh v1.0.0          # máy trạm Windows: amd64 + arm64 + SHA-256
bash scripts/build-client.sh v1.0.0 arm64   # chỉ một kiến trúc
```

`build-client.sh` in ra chuỗi SHA-256 của từng tệp — dán vào ô **Băm** khi công
bố bản cập nhật, nếu không máy trạm sẽ từ chối tải.

### Máy trạm

Máy trạm **không** cấu hình trong `.env` của máy chủ. Mỗi máy có khoá riêng, cấp
ở trang **Máy → Cấp khoá** — khoá chỉ hiện đúng một lần.

**Cách thường dùng: chạy bộ cài.** `vnet-client-setup-<phiên bản>.exe` hỏi ba
thông tin dưới đây, ghi ra `config.json` cạnh tệp thực thi, rồi đăng ký dịch vụ
nền khởi động cùng Windows.

**Bản chạy tay (portable)** vẫn dùng biến môi trường như trước:

```
VNET_SERVER_URL=http://<địa chỉ máy chủ>:20800
VNET_MACHINE_CODE=PC-01
VNET_AGENT_TOKEN=<khoá cấp cho PC-01>
```

Biến môi trường **ghi đè** `config.json`, tiện khi thử tay. Nhưng dịch vụ Windows
chạy ở session 0 và không thừa hưởng biến môi trường của người đăng nhập, nên
bản cài đặt bắt buộc dùng tệp.

Thiếu khoá thì máy chủ trả 401 cho heartbeat, máy hiện ngoại tuyến, và tiến trình
nền không giữ được WebSocket — nghĩa là không tắt/khoá được máy đó từ xa.

Ba tiến trình sau khi cài:

| Tiến trình | Việc |
|---|---|
| `vnet-client.exe --service` | Dịch vụ nền: nhịp tim, WebSocket bằng khoá máy 24/7, tắt/khởi động lại máy, bật lại giao diện khi bị tắt |
| `vnet-client.exe` | Thanh điều khiển dán mép phải, luôn nổi. Khoá màn hình và chặn phím |
| `vnet-client.exe --window order\|support` | Cửa sổ gọi món / hỗ trợ, tách rời |
| `vnet-client.exe --ensure-service` | Bật lại dịch vụ nếu nó đang dừng — hai tác vụ theo lịch gọi lệnh này |
| `vnet-client.exe --set-pin <PIN>` | Đặt PIN kỹ thuật — đường mở khoá khi mất mạng và chưa ai từng đăng nhập |

Máy trạm có **một ô đăng nhập** cho cả nhân viên lẫn hội viên: máy chủ nhìn tên
tài khoản rồi tự chọn đường (`POST /api/auth/client-login`). Tên có trong bảng
người dùng thì vào kiểu nhân viên — **không mở phiên, không tính tiền**; còn lại
là hội viên và phiên bắt đầu ngay.

Mất mạng thì có **hai** đường vào máy trạm, cả hai đều chỉ mở khoá chứ không mở
phiên và không tính tiền:

1. **Tài khoản nhân viên đã từng đăng nhập trên chính máy đó.** Không phải cấu
   hình gì: mỗi lần đăng nhập thành công lúc còn mạng, máy trạm lưu lại tên và
   băm mật khẩu (hạn 30 ngày). Đây là đường dùng hằng ngày.
2. **PIN kỹ thuật** đặt lúc cài. Dành cho máy vừa cài xong, chưa ai kịp đăng
   nhập lần nào.
3. **`admin` / `admin` có sẵn trong bản build**, dùng được khi máy **chưa từng**
   có nhân viên nào đăng nhập qua máy chủ. Lần đăng nhập nhân viên đầu tiên là
   nó tự tắt. Chừng nào còn hiệu lực, mỗi lần nhân viên đăng nhập máy trạm sẽ
   hiện cảnh báo — vì mọi máy cài từ cùng bản build đều mở được bằng đúng cặp đó.

## Kiểm chứng

```bash
python3 scripts/verify.py     # phép kiểm đầu-cuối trên hệ thống đang chạy
python3 scripts/check-admin.py  # khoá i18n thiếu, trang ngoài nhóm menu, nút câm
```

`verify.py` gọi API thật rồi soi lại database — bộ unit test dùng SQL giả không
bắt được lỗi ở tầng database. Kịch bản giả định **database sạch** và tự cảnh báo
khi chạy trên dữ liệu của lần trước.

Năm tính năng gọi thẳng API Windows (khoá màn hình + chặn phím, in máy in nhiệt,
ghi tệp hosts, cài bản cập nhật, chụp màn hình GDI) không kiểm tự động được —
xem [scripts/KIEM-CHUNG-WINDOWS.md](./scripts/KIEM-CHUNG-WINDOWS.md).

## License

GNU Affero General Public License v3.0. See [LICENSE](./LICENSE).
