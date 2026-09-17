<h1 align="center">VNET Core</h1>

<p align="center">
  Hệ thống quản lý phòng game mã nguồn mở, self-hosted.<br />
  <em>Open-source, self-hosted iCafé management system — built for the Vietnamese market.</em>
</p>

<p align="center">
  <img alt="status" src="https://img.shields.io/badge/status-pre--alpha-orange?style=flat-square" />
  <img alt="version" src="https://img.shields.io/badge/version-0.1.0--dev-blue?style=flat-square" />
  <img alt="license" src="https://img.shields.io/badge/license-AGPL--3.0-green?style=flat-square" />
  <img alt="Go" src="https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white" />
  <img alt="Vue 3" src="https://img.shields.io/badge/Vue_3-4FC08D?style=flat-square&logo=vuedotjs&logoColor=white" />
  <img alt="PostgreSQL" src="https://img.shields.io/badge/PostgreSQL-4169E1?style=flat-square&logo=postgresql&logoColor=white" />
</p>

> [!WARNING]
> **Trạng thái: pre-alpha (`0.1.0-dev`).**
> Repo hiện chứa **đặc tả thiết kế** ([`plans/`](plans/)) và **build scripts** ([`scripts/`](scripts/)) — chưa có mã nguồn chạy được.
> Xem [Lộ trình](#lộ-trình) để biết tiến độ.

## Thành phần

Chủ quán tự host backend, admin web, desktop client và windows agent trên hạ tầng của mình.

| Module | Chức năng | Phase |
| --- | --- | --- |
| **VNET Core** | Tính tiền, hội viên, máy trạm | Phase 1 |
| **VNET F&B** | POS, menu, order, kho hàng | Phase 1.1 |
| **VNET Up** | Cập nhật & đồng bộ game | Phase 2 |

> **VNET Cloud Hub** (central SaaS sync) và **VNET Mobile** (app cho gamer) là sản phẩm riêng, không nằm trong repo này.

## Công nghệ

| Layer | Technology |
| --- | --- |
| Backend API | Go (Gin / Echo / Fiber) |
| Desktop Client | Wails (Go + Vue 3) |
| Client Agent | Windows Service (Go, Named Pipes IPC) |
| Admin Web | Vue 3 + Vite + Pinia + Element Plus |
| Database | PostgreSQL |
| Real-time | Redis pub/sub + WebSocket (gorilla/websocket) |

## Kiến trúc

```
┌──────────────────────────────────────────────┐
│             VNET Admin                       │
│         (Vue 3 Web App)                      │
└──────────────────┬───────────────────────────┘
                   │ HTTPS (REST + WebSocket)
                   ▼
┌──────────────────────────────────────────────┐
│         VNET Backend (Go API)                │
│  - JWT Auth middleware                       │
│  - WebSocket hub                             │
│  - Redis pub/sub cho real-time               │
│  - PostgreSQL, row-level locking             │
└──────┬───────────────────────────┬───────────┘
       │ HTTPS                     │ WS + IPC
       ▼                           ▼
┌────────────────────┐  ┌───────────────────────────────┐
│  VNET Client       │  │  VNET Agent (Windows Service) │
│  (Wails Desktop)   │  │  - Screen lock                │
│  - Vue 3 UI        │  │  - HW monitor                 │
│  - Local cache     │  │  - Named-pipe IPC             │
└────────────────────┘  └───────────────────────────────┘
```

## Tài liệu thiết kế

| Chủ đề | Nội dung |
| --- | --- |
| [Tổng quan](plans/PLAN.md) | Kiến trúc, lộ trình, cấu trúc thư mục |
| [Core](plans/core/) | Hội viên, máy, combo, khuyến mãi, booking, curfew |
| [Business flows](plans/core/business.md) | Session lifecycle, pricing engine, shift close, refund |
| [F&B](plans/fnb/) | Menu, sản phẩm, order, POS, inventory, printers |
| [Agent & Security](plans/agent/) | Screen lock, HW monitor, remote control, threat model |
| [API](plans/api/) | REST endpoints + conventions |
| [Database](plans/db/) | Schema, indexes, enums, seed data |
| [Admin UI](plans/ui/) | Routes, component tree, Pinia stores, POS layout |
| [Chat](plans/chat/) | Chat flow, WebSocket, API, UI |

## Tính năng hệ thống

- **Sao lưu & khôi phục** — auto backup database
- **Cập nhật phiên bản** — auto-update cho desktop client + agent
- **Thông số hệ thống** — cấu hình giá, grace period, làm tròn tiền
- **Khống chế website** — chặn web theo danh sách (qua Windows Agent)
- **Quản lý ứng dụng** — whitelist/blacklist app trên máy trạm
- **Multi-store** — quản lý nhiều chi nhánh, thiết kế từ Phase 1

## Lộ trình

| Phase | Nội dung | Ước tính |
| --- | --- | --- |
| **0** | Setup Go project, Vue admin, Wails client, Docker, CI/CD | 1 tuần |
| **1** | Auth, hội viên, máy, tính giờ, báo cáo, chat | 6–8 tuần |
| **1.1** | F&B: menu, POS, payment split, order, kho, printer | 6–8 tuần |
| **2** | VNET Agent (Windows Service), FastUp | 8–12 tuần |
| **3** | E-invoice, multi-store, integrations nâng cao | 4–6 tuần |

## Tích hợp dự kiến

| Đối tác | Chức năng | Phase |
| --- | --- | --- |
| VietQR / Momo | Thanh toán QR, nạp tiền hội viên | 1 |
| Bank QR (OCB, TPBank…) | Thanh toán QR ngân hàng | 1 |
| CCCD API | Xác thực tuổi, check minor | 1 |
| MISA / MeInvoice | Hóa đơn điện tử | 2 |
| Viettel / VNPT | Hóa đơn điện tử (thay thế) | 2 |
| Zalo OA | Thông báo, OTP, marketing | 2 |

## Build

```bash
./scripts/build-server.sh   # build server + embed admin UI
./scripts/build-client.sh   # build desktop client + agent, đóng gói zip
```

CI tự động phát hành release khi push tag `v*` (xem [`.github/workflows/`](.github/workflows/)).

## Đóng góp

Dự án đang ở giai đoạn thiết kế. Góp ý về kiến trúc hoặc nghiệp vụ phòng game rất được hoan nghênh — mở [issue](https://github.com/hungbo/vnet-core/issues) để thảo luận.

## License

[AGPL-3.0](LICENSE)
