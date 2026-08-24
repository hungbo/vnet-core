# VNET Core — AGENTS.md

## Repository Structure

- `backend/` — Go (Gin + GORM + PostgreSQL)
  - **Entrypoint**: `cmd/server/main.go` — runs Gin, auto-migrates all models, registers routes, starts the scheduler
  - **Routes**: `internal/router/router.go` — single file defining all API groups
  - **Handler/Service pattern**: `internal/handler/handler.go:NewHandlers()` wires services → handlers
  - **Scheduler**: `internal/scheduler/` — five background jobs that enforce business rules nothing else enforces:
    `machines:mark-offline`, `bookings:expire`, `sessions:enforce-limits`, `curfew:enforce`, `members:refresh-tier`
  - **Response**: `pkg/response` — `{ code: 0, message: "...", data: {...} }`; paginated: `{ items, total, page, page_size }`
  - **Config**: `internal/config/config.go` — env-based (`DB_HOST`, `JWT_SECRET`, etc.). In `GIN_MODE=release`
    startup is refused on a placeholder/short `JWT_SECRET` or `ALLOWED_ORIGINS=*`
  - **Seed**: `go run ./cmd/seed` — creates `admin/admin123`, `manager/admin123`, `staff/admin123`.
    `-menu` adds a 100-item sample menu (`cmd/seed/menu.go`) with images generated into `UPLOAD_DIR`;
    it is test data, kept behind a flag so the default seed stays production-safe
- `admin/` — Vue 3 + Soybean Admin (Element Plus, UnoCSS, elegant-router)
  - **Proxy**: Vite dev server proxies `/api` → `http://localhost:8080`
  - **Auth route mode**: `VITE_AUTH_ROUTE_MODE=dynamic` — routes fetched from backend `GET /api/route/getUserRoutes`
  - **TWO HTTP clients — do not mix them** (biggest source of confusion in this repo):
    - `src/api/client.ts` — plain axios, `baseURL: '/api'`, unwraps to `data.data`, rejects when `code !== 0`.
      Used by every VNET feature view (`src/views/vnet/**`, ~36 files).
    - `src/service/request` — `@sa/axios` `createFlatRequest`, returns `{ data, error }`, handles token refresh.
      Used ONLY by `src/service/api/*` and the built-in `src/views/system/**` pages.
    - Because `client.ts` returns `{ items, total, page, page_size }` while Soybean's `useTable` expects
      `{ data, pageNum, pageSize, total }`, VNET tables pipe responses through `vnetTransform` /
      `vnetSimpleTransform` (`src/hooks/common/vnet-table.ts`).
  - **Locale**: 3 files in `src/locales/langs/` (en-us, vi-vn, zh-cn)
- `client/` — Wails v2, single binary (GUI + background monitoring).
  `main.go --watch <pid> <machineCode> <serverURL>` runs a detached babysitter that shuts the machine down
  if the GUI dies. Windows-only code sits behind `//go:build windows` (`locker.go`, `screenshot_windows.go`,
  `webblock_os_windows.go`) with non-Windows stubs beside it so the package still builds on macOS/Linux.
- `scripts/` — `build-server.sh` (backend + embed admin dist), `build-client.sh` (client → Windows zip + SHA-256),
  `verify.py` (345 end-to-end checks against a running system), `check-admin.py` (i18n keys, menu grouping,
  dead buttons), `KIEM-CHUNG-WINDOWS.md` (manual checklist for the five Windows-only features)

## Commands

| Action | Path | Command |
|--------|------|---------|
| Dev (backend) | `backend/` | `go run ./cmd/server` (or `air` for hot-reload) |
| Dev (admin) | `admin/` | `pnpm dev` (Vite on :3000) |
| Test (backend) | `backend/` | `go test ./...` |
| Lint (backend) | `backend/` | `golangci-lint run` |
| Migrate | `backend/` | `go run ./cmd/migrate` |
| Seed | `backend/` | `go run ./cmd/seed` (thêm `-menu` để có thực đơn mẫu 100 món) |
| Build server | `./scripts/` | `bash build-server.sh` |
| Build client | `./scripts/` | `bash build-client.sh` |
| Lint (admin) | `admin/` | `pnpm lint` |
| Typecheck (admin) | `admin/` | `pnpm typecheck` |
| Regenerate routes | `admin/` | `pnpm gen-route` (after adding `views/vnet/{feature}/index.vue`) |
| Test (client) | `client/src/` | `go test ./...` — Windows-only tests need a Windows host |
| Cross-compile check | `client/src/` | `GOOS=windows go vet ./...` |
| Run full stack | repo root | `docker compose up -d --build` then `--profile tools run --rm migrate` / `seed` |
| End-to-end verify | repo root | `python3 scripts/verify.py` (needs a running system + clean DB) |
| Admin consistency | repo root | `python3 scripts/check-admin.py` |

`golangci-lint` and `air` are not vendored; install them yourself or skip. There is no `.golangci.yml`,
so `golangci-lint run` uses defaults.

## Critical Conventions

1. **`/systemManage/*` endpoints** use Soybean Admin pagination: params `current`/`size`, response shape `{ code: 0, data: { records, total, current, size } }` (NOT `pkg/response.PaginatedData`)
2. **UUID for all PKs** — use `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`. Nullable FK / date columns
   must be `*string` and you must guard the empty string before taking its address: PostgreSQL rejects `''` for
   `uuid` and `date`, and GORM swallows the error into a plain `err != nil` branch. This has silently discarded
   real data before (`AuditLog.EntityID`, `Notification.ReferenceID`, `MachinePrice.EffectiveTo`)
3. **Postgres arrays and jsonb need the wrapper types** — a bare `[]int` with `type:integer[]` fails at runtime
   (`SQLSTATE 42804 … is of type record`) and a bare `[]string` with `type:jsonb` is silently ignored.
   Use `model.IntArray` / `model.StringArray` (`internal/model/intarray.go`, `stringarray.go`)
4. **Soft delete cuts both ways** — every default GORM query filters `deleted_at IS NULL`, but unique indexes
   cover deleted rows too. Sequence generators (`GenerateOrderCode`) must use `Unscoped()` or they reissue a code
   that already exists; and lookups that decorate historical records (machine code on a past session, booking or
   feedback) must use `Unscoped()` or the column shows blank once the parent is deleted
5. **No menu DB model** — menu/route data is hardcoded in `internal/service/route.go`; update both route defs and
   `system_manage.go` menu stubs when adding pages. `views/system/menu/` is read-only for the same reason
6. **Adding a page touches FIVE places**, not four:
   `internal/service/route.go` (child `RouteItem`, `Name: "vnet_{feature}"`, `Component: "view.vnet_{feature}"`) ·
   `admin/src/views/vnet/{feature}/index.vue` (then `pnpm gen-route`) ·
   `admin/src/locales/langs/{en-us,vi-vn,zh-cn}.ts` (keys under both `route.*` and `vnetPages.*`) ·
   `internal/service/system_manage.go` (menu stub) ·
   `admin/src/store/modules/route/shared.ts` (**menu group — omit it and the page falls into `ungrouped`
   and sits flat at the top level**). `python3 scripts/check-admin.py` catches the locale and menu-group
   mistakes; the `route.go` and `system_manage.go` halves are on you
7. **Auto-migrate** — all models listed in `main.go` `AutoMigrate()` call; adding a new model requires adding it there.
   AutoMigrate never DROPS a column: removing a field from a model leaves the column orphaned in the database
8. **WebSocket** — `gorilla/websocket` at `GET /api/ws/client` behind `AuthRequired`; the token may be passed as
   `?token=` because browsers cannot set headers on a WS upgrade. `hub.Hub` is injected into the Machine, Chat,
   Session, Order and NotificationAdmin **services** (not the handlers)
9. **Response error codes** matching admin `.env`: `8888`=force logout, `7777`=modal logout, `9999`=expired token
10. **JSON serialization** — ALL model structs MUST have `json:"snake_case"` tags (e.g., `json:"machine_code"`). Never omit json tags (Go defaults to PascalCase). `PasswordHash` fields must use `json:"-"`. GORM relationship fields (has many / many2many) use `json:"field_name,omitempty"`. Timestamp fields (`CreatedAt`, `UpdatedAt`, `DeletedAt`) use `json:"field_name,omitempty"`. The frontend (VNET feature endpoints) expects snake_case; the service layer DTOs already follow this convention.

11. **A table column reads the response by KEY NAME, and a wrong key fails silently.** `prop: 'order_count'`
    against a DTO that emits `total_orders` renders an empty column — no error, no warning, and the
    operator reads it as "no data yet". Three separate bugs of this shape shipped (`machine_name` on the
    per-machine report, `created_at` on backups where `BackupLog` only has `started_at`, `order_count` on
    monthly revenue). When you rename a JSON tag, grep `admin/src/views/vnet` for the old name; when you add
    a column, verify it against a live response, not against the model struct. `scripts/verify.py` phase
    `flow_ui_contract` pins the keys the tables currently depend on.

12. **Money is formatted by `@/utils/money`, never by `toLocaleString()`.** A bare `toLocaleString()` uses the
    BROWSER's locale, so the same balance reads `500.000` on one machine and `500,000` on the next. Use
    `formatPrice` (with the ₫ symbol) or `formatAmount` (bare digits).


13. **Every page under `views/vnet/` MUST have exactly ONE root element.** `GlobalContent` wraps each page in
    `<Transition mode="out-in">`, and Vue can only attach transition hooks to a single element root. A page
    with a fragment root (a dialog left outside the main `<div>`, two sibling cards) gets no `enter` hook, the
    transition's `isLeaving` flag stays `true` forever, and **from the second navigation onward every page in
    the app renders blank** — no error, and the warning that explains it only appears in the dev build. Two
    stray `<ElDialog>` elements broke navigation this way. `scripts/check-admin.py` fails the build on a
    fragment root; run it before you commit a page.


14. **The desktop client MUST be built with `-tags desktop,production`.** Without the `production` tag Go
    picks `internal/app/app_default_windows.go` — a stub whose `CreateApp` only shows a message box reading
    *"Wails applications will not build without the correct build tags"* and returns nil. The `.exe` builds
    and runs; it just never opens a window. Nothing on macOS/Linux can catch this, so `scripts/build-client.sh`
    greps that string out of the produced binary and fails the build if it is present. `-H windowsgui` is
    required too, or a console window flashes behind the GUI on every launch. These are the same flags
    `wails build` passes for a production build.


15. **`/auth/member-login` is the door to the machine — never let it succeed when play is not allowed.**
    It is the lock screen the *customer* types into, not `/sessions/start` (the staff path in admin). It must
    validate the machine code, and opening the billing session must succeed or the login must fail. The old
    handler auto-started a session but swallowed every failure with `log.Printf` and still returned 200 —
    unknown machine, zero balance, unpaid debt, minor during curfew all logged in and used the machine with
    nothing metering them, and the log line sat unread inside a container. Every gate on playing lives in
    `service.CheckMemberMayPlay` so `StartSession` and `MemberLogin` cannot drift apart. **Never answer a
    failed business rule with a log line on a success path.**


16. **The WebSocket handshake must accept same-origin requests.** The admin is embedded in the server binary
    and served from the same host and port as the API, so the normal deployment — open `http://localhost:8080`
    — is same-origin. `originAllowed` used to string-match `ALLOWED_ORIGINS` only, so an operator writing
    `ALLOWED_ORIGINS=http://localhost` while browsing on port 8080 got a 403 on every handshake: the socket
    never connected and **every realtime feature died silently** — no error on screen, just nothing ever
    happening again. Same-origin carries no CSRF risk, so it always passes. Anything that stops working "for
    no reason" in realtime, check the WS handshake first: `docker compose logs app | grep CheckOrigin`.

17. **A realtime alert belongs to one owner, mounted globally.** Per-page `wsStore.on(...)` in `onMounted`
    means the alert only fires while that page is open — a top-up request raised while staff sit on the
    dashboard was simply lost. Toasts live in `admin/src/hooks/common/ws-notify.ts` (called from
    `base-layout`) and, for chat, in `useChatWs`; pages keep only their data refresh. Note `wsStore.off`
    requires the handler reference — passing only the event name used to wipe every listener for it,
    including the global one.

18. **Per-minute billing accrues to a target; it never adds up per tick.** Every minute
    `SessionService.ChargeTick` asks *"how much should have been taken by now?"*
    (`CalculateCost(elapsed)`) and takes only the difference against `MachineSession.ChargedAmount`.
    That single mechanism buys three properties at once: it is **idempotent** (re-running, restarting the
    server, or two workers overlapping can never overcharge), it **never drifts on rounding** because
    `math.Ceil` still applies to the total exactly as it did when billing happened once at the end, and
    `min_duration` lands on the first tick for free. Never rewrite this as "cost of this minute, added up" —
    that reintroduces both the drift (+60₫/hour) and the double-charge window.
    `EndSession` goes through the same function, so there is no second billing path to drift from.

19. **The `session_fee` ledger row records money that MOVED, not the accrued total.** One row per session,
    growing; `Amount` accumulates the deltas actually deducted. The two numbers are equal for a normal
    session, and differ in exactly one case: a session already running when per-minute billing was deployed
    is stamped "settled up to deploy time" (`BackfillChargedAmount`), so `ChargedAmount` is non-zero while
    nothing has left the balance. Writing the accrued total there produced a row reading −15.000₫ against a
    balance that only fell 833₫ — the ledger invariant broken on its very first row.


20. **The desktop client is three processes, and each has a job the others cannot do.**
    `vnet-client.exe --service` runs as a Windows service in session 0: heartbeat, a WebSocket held open
    with the **machine key** 24/7, shutdown/restart/block-app, and relaunching the UI. The default mode is
    the right-edge dock in the user session and owns everything needing a window or a keyboard hook —
    screen lock, on-screen messages. `--window order|support` are extra OS windows.
    **Wails v2 has no multi-window API** (`pkg/runtime/window.go` — every `Window*` call takes the one app
    context), so a second window is necessarily a second process. `options.SingleInstanceLock` +
    `OnSecondInstanceLaunch` both prevents duplicates and is the mechanism for raising a window on a new
    message — do not write custom IPC for that.

21. **One machine code, many WebSocket connections.** `Hub.machineClients` is `map[string][]*Client` and
    `SendToMachine` fans out to all of them. It used to hold one client per code, so the service and the UI
    evicted each other. Each side ignores the commands it cannot serve, so nothing runs twice. Related:
    `/api/ws/client` accepts **either** a user JWT **or** `machine_code` + agent key
    (`middleware.AuthOrAgent`) — before that, a machine with nobody logged in had no connection at all and
    staff could not shut it down or lock it, which is exactly when they need to.

22. **Settings values come back as STRINGS, so `"false"` is truthy.** Everything in `system_settings.value`
    is jsonb decoded through `decodeSettingValue`, which returns text. Use `service.settingBool` on the
    server and `<ElSwitch active-value="true" inactive-value="false">` in the admin; testing truthiness
    gives a switch that can never be turned off. Feature flags default to **on** — `GetByGroup` 404s on an
    empty group, and defaulting to off would delete features from every café that never opened the tab.

23. **Client colours live in `client/src/frontend/src/styles/tokens.css`, never as hex in a component.**
    There was no CSS source file at all before; `#f0f2f5` alone was repeated across four files. The client
    is dark-only by design — no theme switcher. `dev-wails-stub.ts` (dev builds only) fakes the Wails
    runtime so the UI can be opened in a plain browser; without it every component's `mounted` hook throws
    on `EventsOn` and the whole tree stops rendering, which makes the client unverifiable off Windows.

24. **A public file URL is built from the static route, never from the path on disk.** `/uploads` is served
    from `cfg.Server.UploadDir`, and that directory is absolute in Docker (`/app/uploads`). Returning
    `"/" + diskPath` yields `/app/uploads/...`, which matches no route, falls through to the embedded admin
    SPA, and answers **200 with HTML**. An `<img>` shows a blank box and reports nothing — every image
    uploaded from the admin was unreachable in the Docker deployment. Build the URL as
    `path.Join("/uploads", dateDir, filename)`.

25. **`/api/products` hand-rolls its paging and ignores `sort`.** It does not go through
    `pagination.GetParams`, so `page_size` is **not** capped at `MaxPageSize` — three admin pages rely on
    that and ask for 500–1000 rows in one call. It also ignores `sort`/`order`; ordering is fixed at
    `sort_order asc, name asc` in `ProductService.List`. Send neither from a client: they look respected
    and are not.

26. **A partial report must never blank what it did not mention.** `MachineService.Heartbeat` wrote
    `ip_address` and `mac_address` unconditionally, and the client had a second loop posting to the same
    endpoint with a payload that carried neither — so every 60s the machine's address was wiped and the
    next report re-wrote it. Two payload shapes for one endpoint is the root cause, not two separate bugs:
    the client now sends ONE `TelemetryPayload` from ONE loop, and the server only writes fields the
    report actually carries. The same rule covers the self-reported specs (`cpu_name`, `gpu_name`,
    `ram_gb`, `storage_gb`) — absent means "no news", not "erase it".

27. **Anything written into a `varchar(n)` from an agent must be truncated by RUNE.** PostgreSQL rejects
    the whole statement when a string is too long — it does not truncate for you — so one over-long CPU
    name would lose the heartbeat *and* its history row. Cutting by byte instead of rune produces invalid
    UTF-8. Use `truncateRunes`.

28. **`pagination.GetParams` defaults to `sort=id, order=desc`, and every id here is a random UUID.**
    Any "latest N" list that does not set an order is therefore N *arbitrary* rows in arbitrary order —
    and with only a handful of rows it still looks plausible, so it never announces itself.
    `GetHardwareHistory` now forces `created_at desc`; `ChatService.GetMessages` and others chain
    `.Order("created_at DESC")` before `pagination.Apply`, which works because GORM appends order
    clauses. Still unordered at the time of writing: `BackupService.List`, `NotificationService.List`,
    `MachineService.List`, `MemberService.List` / `GetTransactions` / `GetSessions` / `GetCombos`.

29. **A job that runs less often than the server restarts will never run.** The scheduler's ticker
    restarts from zero on every boot, so an hourly job on a server that redeploys every 20 minutes fires
    never. Set `RunAtStart: true` for anything sparser than the deploy cadence — that is also what makes
    such a job observable in `verify.py`, which cannot wait an hour.

30. **A number the server changes on a timer must be pushed, and pushed as an absolute value.**
    `ChargeTick` deducted money every minute in silence: the client reads the balance once at login, so
    the figure on the customer's screen froze while the real money drained — three hours at 20.000₫/h is
    a 60.000₫ lie, ending in a lock screen the customer did not see coming. It now emits `balance:updated`
    after the commit (never inside the transaction — a rollback would leave a number on screen that never
    existed), carrying `balance`, `bonus_balance` and the server-computed `affordable_until`. Absolute
    values, not deltas: a dropped event then heals on the next tick with no resync path to write.
    Target the machine only — this fires once a minute per session, and nothing in the admin listens.

    The same duty falls on **every** path that moves a member's balance, not just the timer. For a long
    while only `ChargeTick` and `processTopupOrder` pushed anything; topup, refund, booking deposit,
    booking cancellation, combo purchase and topup-card redemption all changed the number in silence,
    because `MemberService` and four others were never given the hub. Staff topping a customer up at the
    counter is the worst case: the customer is not in a session, so there is no next tick to heal it and
    the screen keeps the old figure indefinitely. Emit through `phatSoDuMoi` after the commit, and use
    `hub.SendToUser` rather than `SendToMachine` — a balance belongs to a person, who may be standing at
    the counter on no machine at all, and who usually holds several sockets at once because the client
    opens its child windows as separate processes.

    **When the number is derived, push a signal instead of the number.** Stock is the counterpart case:
    `current_stock` in the API is not the raw column — for a dish with a recipe it is
    `min(ingredient stock / per-unit amount)`, so deducting *one* ingredient changes the serving count of
    *every* dish using it (`product.go:352-383`). Recomputing that fan-out at the emit site is a second
    implementation of the derivation, and a wrong one shows a wrong number nobody will catch. So
    `stock:changed` carries only the ids of the raw products touched and the client re-fetches. It is
    still self-healing, and `Broadcast` is correct here where `SendToUser` was correct for balance —
    stock is not private, and every open menu needs it.

31. **The machine key is optional, and that is a deliberate trade.** `Create` no longer issues one;
    `VerifyAgentToken` passes any machine whose `agent_token` is empty, so a client plugs in and runs.
    Pressing "Cấp khoá" turns the requirement on **for that machine only**. The cost is real: without a
    key anyone on the LAN who can guess a machine code can post fake telemetry and receive that
    machine's remote commands — and in a net café the customers are on that LAN. `AuthOrAgent` picks
    the agent path whenever there is no user token, so an empty key still reaches the verifier.

32. **Anything that can power off a customer's machine must be a pure function of observations.**
    `guardState.step` decides; `observeGuard` (Windows-only) merely gathers. Every uncertain
    observation — service not installed, SCM unreadable, Windows shutting down, maintenance flag, UI
    younger than the grace period — **clears** the strike counter instead of adding to it. Missing a
    cheat costs a session; a false positive powers off a room full of paying customers. The first line
    of defence is not this code at all — see convention 41 for the layers.

33. **`App.Startup` runs in the child-window processes too.** Order and support windows are separate
    processes sharing the same `App`, so anything started there runs twice: duplicate telemetry loops,
    two writers on the hosts file, and — once login-locking was added — a food menu that covered the
    whole screen. Branch on `a.windowMode` before starting any background work.

34. **Full-screen login lock needs an offline way back in.** Member login, QR scan and the admin
    account all call the API, so once the client locks the desktop, a dead router leaves every machine
    in the café showing a full-screen login box with nothing that can open it. `App.UnlockMaintenance`
    checks a PBKDF2-SHA256 hash held in `config.json` and never touches the network. Plain SHA-256
    would not do: that file sits in Program Files, which blocks writes but not reads, and a six-digit
    PIN falls to a full sweep in under a second. Unlocking this way opens no session and bills nothing,
    and it pauses the tamper guard — the first thing a technician does is stop the service.

35. **Offline staff credentials are cached on every successful online login, never seeded.** There is no
    built-in `admin/admin`: a shipped default is a known backdoor on every machine until it first syncs.
    `rememberStaff` stores username + PBKDF2 hash after a successful `LoginAdmin`, and `LoginAdmin`
    falls back to that cache only when the transport itself failed — `errServerUnreachable`, not a 401,
    or a wrong password would open the machine. Three rules the tests pin down: members are never
    cached (a customer would just unplug the network and play free), entries expire after 30 days (a
    dismissed employee would otherwise keep every machine in the café), and the file holds no plaintext.
    Day one, before anyone has logged in, is covered by the maintenance PIN instead.

36. **One login box on the client; the username picks the path.** `/api/auth/client-login` looks the
    name up in `users` first — found means staff, and staff get **no session and no billing**; anything
    else goes down the member path, which opens the session. A name present in both tables is staff,
    full stop: falling through on a bad staff password would make the same name resolve to a different
    person depending on the password typed. Staff roles (`owner`/`manager`/`staff`) are mapped to the
    client's `admin` role inside `App.Login`, because `GetUserInfo` is what the dashboard reads when it
    builds the screen — leave it and the owner logs in to the customer's layout, Nạp tiền button and all.

37. **The built-in `admin/admin` is a deliberate, self-closing hole.** The owner asked for it twice
    after hearing the objection: a machine must never be locked out, not even one that has never
    reached the server. `builtinAdminActive()` is true only while the credential cache is empty, so the
    first successful staff login through the server closes it — that is what "cập nhật mật khẩu sau"
    means in code. It only opens the **offline maintenance** path (no session, no billing), never a
    session, and never when the server answered: a wrong password must not open a machine. While it is
    live, every staff login raises a non-dismissing warning naming it, because a back door nobody knows
    about is worse than no door. Do not "tidy this away" — it is a decision, not an oversight.

38. **Never wrap a scrollable page section in `el-card`.** Element Plus ships
    `.el-card__body { flex-grow: 1; overflow: auto }`, so inside a flex column every card becomes its
    own scroll container. The client's Settings page had **four** nested scrollbars and the outer one
    never moved, which is invisible until the window is short enough for the content to overflow — a
    tall browser window hides it completely. Use plain `<section>` blocks styled from `tokens.css`.

39. **Wails runs `OnStartup` in a goroutine, racing its own `WindowCenter()`.** In
    `internal/frontend/desktop/windows/frontend.go` the main thread calls `f.WindowCenter()` while
    `OnStartup` is dispatched with `go`. Whichever finishes last wins, so geometry set in `OnStartup`
    gets silently overwritten by a centring computed from the *declared* `Width`/`Height` — the window
    ends up the right size in the wrong place. Set window geometry in **`OnDomReady`**, which runs
    inside `navigationCompleted`, long after `WindowCenter` and immediately before the window is shown.
    Keep the `OnStartup` call too: it is idempotent and covers the ordering where it does win.
    (`Screen.Size` is already logical pixels — DPI scaling is not the problem here.)

40. **The dock collapses to an edge strip; it never calls `WindowHide`.** Hiding the window outright
    leaves nothing to click and the service will not help — `superviseUI` relaunches the UI only when
    the process is **dead**, not when it is hidden. Collapsing is refused while nobody is logged in,
    or a single button would shrink the full-screen lock to a 28px sliver.

41. **SCM recovery actions do not fire on a clean stop, so they are not enough on their own.** Go's own
    docs say it plainly: recovery runs only when the service "terminates without reporting a status of
    SERVICE_STOPPED", and `SetRecoveryActionsOnNonCrashFailures(true)` widens that only to *non-zero
    exit codes*. Killing the process in Task Manager is covered; `sc stop` and Services.msc → Stop are
    not — which is exactly how someone with admin rights turns it off. Three layers, each covering what
    the one before misses:
    **(1)** SCM recovery restarts it ~5s after a kill.
    **(2)** Two scheduled tasks running as `S-1-5-18` — one on a boot trigger repeating every minute,
    one triggered by SCM event 7036 filtered to this service's display name — cover the clean stop and
    survive both the service and the UI being killed, because Task Scheduler is neither process's child.
    **(3)** The UI guard powers the machine off if nothing brought the service back within a minute.
    The two tasks are registered separately on purpose: the repeating one has near-unbreakable XML, the
    event one has a fiddly XPath filter, and a typo in the filter must not cost the reliable layer.
    `schtasks /Create /XML` also rejects UTF-8 files on many Windows builds without saying so — write
    UTF-16LE with a BOM.

42. **Billing is server-authoritative and survives the client vanishing — do not "fix" that.**
    `enforceSessionLimits`/`ChargeTick` charge active sessions from the DB by elapsed time; killing or
    uninstalling the client does not stop the clock, and `markStaleMachinesOffline` never touches
    sessions. On a frozen-disk (diskless) café this is the whole security model: a reboot restores the
    golden image, so tamper is self-healing and the machine-shutdown guard is largely redundant.

43. **On frozen disk a reboot leaves an orphaned active session that keeps billing at the lock screen.**
    The server detects it from `uptime`: `Machine.BootedAt = now - uptime` (a *duration*, so no clock-sync
    issue — only network jitter of seconds), and a session whose `started_at` predates the current boot
    by more than `rebootStaleMargin` (2m) is stale. `EndSessionsStaleAfterReboot` settles it **at
    BootedAt**, not now, so the idle boot-to-detection gap is not billed. Two callers: the login path
    (so the next customer starts fresh instead of hitting "machine in use") and `enforceSessionLimits`
    (idle machine nobody logged back into). The margin only absorbs jitter — normal flow boots *then*
    logs in, so `BootedAt < started_at` always; only a real mid-session reboot crosses it, and uptime
    never decreases without one, so there are no false positives that would kill a live customer's session.

44. **A shut-down-without-logout machine bills until the balance hits zero — unless something closes it.**
    Nothing ended a session on a machine merely going offline (`markStaleMachinesOffline` only flips
    status; `enforceSessionLimits` ends only on slot/minutes/balance). A per-hour session on a powered-off
    machine kept charging for hours. `EndSessionsOnOfflineMachines(offlineSessionTimeout=2m)` closes any
    active session whose machine's `last_heartbeat` is older than the timeout, settling **at last_heartbeat**
    so the off period is not billed. It skips machines with `last_heartbeat IS NULL` (never had a client —
    a staff-opened session), and does not fight the reboot-close (a rebooted machine is heartbeating again,
    so its last_heartbeat is fresh). Note for tests: a session with no ongoing heartbeat now closes after
    2m, so any verify flow that holds a session open across a scheduler tick must refresh `last_heartbeat`
    (a live client heartbeats every 15s; the test must simulate that).

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

## Verification

Four levels. Stopping at level 1 is how "it compiles" gets mistaken for "it works":

1. `go test ./...` green in `backend/` and `client/src/`; `pnpm typecheck` in `admin/` shows no NEW errors
   (there is a standing baseline of pre-existing ones — count them before and after).
2. `python3 scripts/verify.py` against a **clean** database. It calls the real API and then reads the database
   back, because the sqlmock unit tests cannot catch anything at the database layer. The script warns when it
   detects leftovers from a previous run — results on a dirty database are not trustworthy.
3. **Open a real browser** on the page you changed: click the actual button, read the number on screen, check the
   console. This is the only level that catches "the API is right but the user sees nothing" — the class of bug
   that left 6 of 7 report tabs blank for months.
4. `python3 scripts/check-admin.py` — missing i18n keys, untranslated Vietnamese, pages outside any menu group,
   and `<TableHeaderOperation>` buttons that emit into the void.

The check count in `verify.py` may only go **up**, and `0 sai / 0 hỏng` is the bar for calling work done.
Five Windows-only features cannot be covered from here — see `scripts/KIEM-CHUNG-WINDOWS.md`.

## Ship it — push when a feature is done

**Finishing a feature means committing and pushing it, not leaving it in the working tree.** Do this as soon as
a unit of work passes Verification above — do not batch several features into one heap.

Why this is a rule and not a preference: this repo has already carried 175 modified files and 100 untracked ones
in a single uncommitted pile. At that size nothing is reviewable, `git stash` becomes dangerous, a bad edit
cannot be isolated with `git bisect`, and any mistake takes the whole pile down with it.

    git checkout -b feature/<ten-tinh-nang>     # never commit straight onto main
    git add -A && git status                    # READ the list before committing
    git commit -m "feat(scope): ..."            # Conventional Commits, see CONTRIBUTING.md
    git push -u origin HEAD

Four things to get right:

- **Read `git status` before every commit.** `go build ./...` drops a binary named after the package directory
  right where it ran (`backend/seed`, `client/src/src`, ~13MB each) and it does not look like a build artifact.
  Anything unignored that is not source belongs in `.gitignore`, not in a commit.
- **One feature, one commit.** A commit spanning backend, admin and client at once cannot be reverted when one
  third of it turns out wrong.
- **Push only what is verified.** A green `go build` is level 1 of four. Pushing a feature that has never been
  opened in a browser or on a real Windows machine just moves the failure somewhere harder to find.
- **Say what is unverified.** Windows-only paths cannot be checked from macOS/Linux. Write that in the commit
  body rather than letting it read as tested.

## GitNexus in practice

The block below is generated by `node .gitnexus/run.cjs analyze` and is overwritten on every re-index —
put durable notes here, above the marker, not inside it.

- **The MCP tools are not always connected.** When `impact` / `query` / `context` are absent from your tool
  list, use the CLI instead of skipping the step: `node .gitnexus/run.cjs impact <symbol> --repo vnet-core`.
  Same for `context`, `query`, `trace`, `detect-changes`.
- **`--repo vnet-core` is required.** Several repositories are indexed on this machine, and a bare command
  fails with `Multiple repositories indexed`.
- **Check freshness before trusting it.** `node .gitnexus/run.cjs status` prints the indexed commit against
  the current one. The counts quoted below go stale the moment code changes; a confident answer from a stale
  index is worse than no answer, so re-analyze or fall back to grep plus `go test ./...`.
- Ambiguous names return `status: "ambiguous"` with candidates rather than one answer — pass the `uid` to
  pick one (a service method and its handler method often share a name here).

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **vnet-core** (8082 symbols, 22573 relationships, 181 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

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
