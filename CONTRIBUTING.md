# Contributing to VNET Core

Cảm ơn bạn đã quan tâm đến VNET Core! Dưới đây là hướng dẫn để đóng góp hiệu quả.

## Setup

```bash
git clone https://github.com/hungbo/vnet-core.git
cd vnet-core

# Backend
cd backend && go mod download

# Admin
cd ../admin && pnpm install

# Client (optional)
cd ../client && go mod download
```

## Branch Convention

- `feature/<name>` — tính năng mới
- `fix/<name>` — sửa lỗi
- `chore/<name>` — bảo trì, cấu hình
- `refactor/<name>` — tái cấu trúc

Luôn tạo nhánh từ `main` và tạo PR vào `main`.

## Commit Style

Sử dụng [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add machine group bulk assign
fix: correct session cost calculation on DST boundary
refactor: unify conversation -> room naming
chore: update dependencies
docs: add API usage example
```

## Code Conventions

Chi tiết đầy đủ tại [AGENTS.md](./AGENTS.md). Tóm tắt:

### Backend (Go)

- **JSON**: `json:"snake_case"` trên tất cả model fields
- **PK**: UUID với `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
- **Response**: `{ code: 0, message: "...", data: {...} }` (trừ `/systemManage/*` dùng `{ records, total, current, size }`)
- **Handler/Service pattern**: handler gọi service, service gọi DB
- **Auto-migrate**: thêm model mới vào `cmd/server/main.go` `AutoMigrate()`
- **WS events**: `gorilla/websocket` tại `/ws/client`

### Admin (Vue 3)

- Route `Name` format: `vnet_{feature}`
- View file: `src/views/vnet/{feature}/index.vue`
- Locale keys: cập nhật cả 3 ngôn ngữ (en-us, vi-vn, zh-cn)
- Dùng `@sa/axios` `createFlatRequest` — success = `code === '0'`

### Client (Wails)

- Go backend methods trong `client/src/app.go`
- WS events đồng bộ với backend (`chat:*`, `room:*`)
- Bindings tự động qua `wailsjs` — chạy `wails build` sau khi sửa signature

## Pull Request Process

1. Chạy `go test ./...` và `pnpm typecheck` trước commit
2. Mô tả rõ vấn đề / tính năng trong PR description
3. Nếu thay đổi UI, kèm screenshot / video
4. Nếu thêm API endpoint, cập nhật Swagger annotation
5. Sẽ có ít nhất 1 review trước khi merge

## GitNexus

Dự án sử dụng GitNexus để phân tích tác động code:

- **Trước khi sửa symbol**: chạy `impact({target: "symbolName", direction: "upstream"})` để biết code nào phụ thuộc
- **Trước commit**: chạy `detect_changes()` để verify scope thay đổi
- **Cảnh báo HIGH/CRITICAL**: phải được giải quyết trước khi merge

## Questions?

Mở issue trên GitHub hoặc liên hệ maintainer.
