# VNET Core — AGENTS.md

## Repository Structure

- `backend/` — Go (Gin + GORM + PostgreSQL)
  - **Entrypoint**: `cmd/server/main.go` — runs Gin, auto-migrates all models, registers routes
  - **Routes**: `internal/router/router.go` — single file defining all API groups
  - **Handler/Service pattern**: `internal/handler/handler.go:NewHandlers()` wires services → handlers
  - **Response**: `pkg/response` — `{ code: 0, message: "...", data: {...} }`; paginated: `{ items, total, page, page_size }`
  - **Config**: `internal/config/config.go` — env-based (`DB_HOST`, `JWT_SECRET`, etc.)
  - **Seed**: `go run ./cmd/seed` — creates `admin/admin123`, `manager/admin123`, `staff/admin123`
- `admin/` — Vue 3 + Soybean Admin (Element Plus, UnoCSS, elegant-router)
  - **Proxy**: Vite dev server proxies `/api` → `http://localhost:8080`
  - **Auth route mode**: `VITE_AUTH_ROUTE_MODE=dynamic` — routes fetched from backend `GET /api/route/getUserRoutes`
   - **Request**: `@sa/axios` `createFlatRequest` — success = `String(response.data.code) === VITE_SERVICE_SUCCESS_CODE` (default `'0'`)
  - **Locale**: 3 files in `src/locales/langs/` (en-us, vi-vn, zh-cn)
- `client/` — Wails v2, single binary (GUI + background monitoring)
- `scripts/` — `build-server.sh` (backend + embed admin dist), `build-client.sh` (client → Windows zip)

## Commands

| Action | Path | Command |
|--------|------|---------|
| Dev (backend) | `backend/` | `go run ./cmd/server` (or `air` for hot-reload) |
| Dev (admin) | `admin/` | `pnpm dev` (Vite on :3000) |
| Test (backend) | `backend/` | `go test ./...` |
| Lint (backend) | `backend/` | `golangci-lint run` |
| Migrate | `backend/` | `go run ./cmd/migrate` |
| Seed | `backend/` | `go run ./cmd/seed` |
| Build server | `./scripts/` | `bash build-server.sh` |
| Build client | `./scripts/` | `bash build-client.sh` |
| Lint (admin) | `admin/` | `pnpm lint` |
| Typecheck (admin) | `admin/` | `pnpm typecheck` |

## Critical Conventions

1. **`/systemManage/*` endpoints** use Soybean Admin pagination: params `current`/`size`, response shape `{ code: 0, data: { records, total, current, size } }` (NOT `pkg/response.PaginatedData`)
2. **UUID for all PKs** — use `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`. StoreID in models is `*string`; guard empty string before `&storeID` (PostgreSQL rejects UUID `""`)
3. **No menu DB model** — menu/route data is hardcoded in `internal/service/route.go`; update both route defs and `system_manage.go` menu stubs when adding pages
4. **Admin view naming** — route `Name` = `vnet_{feature}`, file in `admin/src/views/vnet/{feature}/index.vue`. Add locale keys in both `route.*` and `vnetPages.*` for all 3 languages
5. **Auto-migrate** — all models listed in `main.go` `AutoMigrate()` call; adding a new model requires adding it there
6. **WebSocket** — `gorilla/websocket` at `/ws/client`, `hub.Hub` injected into Session handler
7. **Response error codes** matching admin `.env`: `8888`=force logout, `7777`=modal logout, `9999`=expired token
8. **JSON serialization** — ALL model structs MUST have `json:"snake_case"` tags (e.g., `json:"machine_code"`). Never omit json tags (Go defaults to PascalCase). `PasswordHash` fields must use `json:"-"`. GORM relationship fields (has many / many2many) use `json:"field_name,omitempty"`. Timestamp fields (`CreatedAt`, `UpdatedAt`, `DeletedAt`) use `json:"field_name,omitempty"`. The frontend (VNET feature endpoints) expects snake_case; the service layer DTOs already follow this convention.

## Working Style (Karpathy-Inspired)

These guidelines bias toward caution over speed. For trivial fixes use judgment.

### 1. Think Before Coding
- **State assumptions explicitly** before touching cross-module code (e.g., "I assume X affects both member service and handler"). If unsure which model/endpoint to change, ask.
- **Present tradeoffs** — e.g., "Adding a DB column requires updating AutoMigrate + model + service + handler; alternatively I could derive this at query time." Don't silently pick an approach.
- **Stop and clarify** when route names, response shapes, or locale keys are ambiguous.

### 2. Simplicity First
- No new Go abstractions (interfaces, wrapper types) for single-use code.
- No speculative "flexibility" — don't add pagination params to an endpoint that's only used once.
- No error handling for impossible scenarios (e.g., `err != nil` after `db.Where("id = ?", uuid).First(&obj)` when the UUID is known valid).
- If a handler is 100 lines of copy-paste pagination that could be 30 with a helper, extract the helper.

### 3. Surgical Changes
- **Touch only what the task requires.** Don't "improve" adjacent Go handler comments, reformat SQL strings, rename vars, or restyle Vue SFCs.
- **Match existing style** — single quotes in Go strings? Keep them. Snake_case JSON tags? Keep them.
- **Clean up YOUR orphans only** — remove Go imports and unused vars that your edit made stale. Don't remove pre-existing dead code.
- **Every changed line must trace to the user's request.** If it doesn't, undo it.

### 4. Goal-Driven Execution
- Turn vague tasks into verifiable goals:
  - "Fix 404 on user list" → "Hit `GET /api/systemManage/getUserList` → expect `{ code: 0, data: { records: [...] } }`"
  - "Add machine groups page" → "Navigate to `/vnet/machine-groups` → table loads with 0 console errors → CRUD dialog opens and submits"
- For multi-step work, plan with checkpoints:
  ```
  1. [Backend] Add `/machine-groups` routes → verify: `curl` returns 200
  2. [Admin] Create view page → verify: SSR route resolves, no 404
  3. [Admin] Wire API calls → verify: table renders with data
  ```
- **Verify the actual UI + network** — open the browser, check console + Network tab. Don't assume success from a green build.

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **vnet-core** (6598 symbols, 16419 relationships, 153 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

> Index stale? Run `node .gitnexus/run.cjs analyze` from the project root — it auto-selects an available runner. No `.gitnexus/run.cjs` yet? `npx gitnexus analyze` (npm 11 crash → `npm i -g gitnexus`; #1939).

## Always Do

- **MUST run impact analysis before editing any symbol.** Before modifying a function, class, or method, run `impact({target: "symbolName", direction: "upstream"})` and report the blast radius (direct callers, affected processes, risk level) to the user.
- **MUST run `detect_changes()` before committing** to verify your changes only affect expected symbols and execution flows. For regression review, compare against the default branch: `detect_changes({scope: "compare", base_ref: "main"})`.
- **MUST warn the user** if impact analysis returns HIGH or CRITICAL risk before proceeding with edits.
- When exploring unfamiliar code, use `query({search_query: "concept"})` to find execution flows instead of grepping. It returns process-grouped results ranked by relevance.
- When you need full context on a specific symbol — callers, callees, which execution flows it participates in — use `context({name: "symbolName"})`.
- For security review, `explain({target: "fileOrSymbol"})` lists taint findings (source→sink flows; needs `analyze --pdg`).

## Never Do

- NEVER edit a function, class, or method without first running `impact` on it.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis.
- NEVER rename symbols with find-and-replace — use `rename` which understands the call graph.
- NEVER commit changes without running `detect_changes()` to check affected scope.

## Resources

| Resource | Use for |
|----------|---------|
| `gitnexus://repo/vnet-core/context` | Codebase overview, check index freshness |
| `gitnexus://repo/vnet-core/clusters` | All functional areas |
| `gitnexus://repo/vnet-core/processes` | All execution flows |
| `gitnexus://repo/vnet-core/process/{name}` | Step-by-step execution trace |

## CLI

| Task | Read this skill file |
|------|---------------------|
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus/gitnexus-cli/SKILL.md` |

<!-- gitnexus:end -->
