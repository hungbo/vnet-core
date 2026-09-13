# GOAL: Chạy thật VNET (server + web admin) và bấm tay từng chức năng cho tới khi sạch lỗi

Làm việc trong `/Users/kane/Desktop/work/vnet/core`. Đọc `AGENTS.md` + `CLAUDE.md` trước khi sửa gì.

Nhiệm vụ nhiều vòng. **Không hỏi "có tiếp không", không bàn giao giữa chừng.**
Mỗi lần được gọi: đọc `docs/UI-AUDIT.md`, làm **đúng một vòng** (một trang), cập nhật sổ, dừng vòng.
Chỉ kết thúc toàn bộ khi mọi dòng checklist `PASS` và không còn lỗi `OPEN`.

Nguyên tắc gốc: **bằng chứng là thao tác thật trên trình duyệt + tab Network**, không phải build xanh,
không phải đọc code rồi suy luận. **Chưa bấm = chưa PASS.**

## Giai đoạn 0 — Môi trường (chỉ làm khi chưa chạy)

Kiểm tra trước, đã chạy rồi thì bỏ qua — đừng khởi động trùng.

1. **Postgres**: container `vnet-ui-audit-pg` (postgres:16-alpine, 127.0.0.1:5432, user/pass/db = `vnet`/`vnet`/`vnet`
   — trùng mặc định của `internal/config/config.go`, nên không cần đặt biến môi trường nào).
   - Kiểm: `docker exec vnet-ui-audit-pg pg_isready -U vnet -d vnet`
   - Dựng lại nếu mất:
     `docker run -d --name vnet-ui-audit-pg -e POSTGRES_USER=vnet -e POSTGRES_PASSWORD=vnet -e POSTGRES_DB=vnet -e TZ=Asia/Ho_Chi_Minh -p 127.0.0.1:5432:5432 postgres:16-alpine`
   - Máy này **không có `psql`/`pg_isready` trên host** — mọi lệnh SQL phải đi qua `docker exec vnet-ui-audit-pg psql -U vnet -d vnet -c "..."`.
2. **Schema + dữ liệu**: `cd backend && go run ./cmd/server` tự `AutoMigrate` khi khởi động.
   Sau lần boot đầu: `go run ./cmd/migrate` (fixup cột, chạy SAU khi bảng đã có) rồi
   `go run ./cmd/seed` → tài khoản `admin/admin123`, `manager/admin123`, `staff/admin123`.
   Cần thực đơn mẫu 100 món thì `go run ./cmd/seed -menu`.
3. **Backend nền**: `cd backend && go run ./cmd/server` chạy nền, log ra file
   (ví dụ `/tmp/vnet-backend.log`) — **log backend là nguồn chẩn đoán chính, luôn đọc nó khi có lỗi.**
   Chờ `curl -s localhost:20800/api/health` trả 200.
   **Khởi động lại phải diệt theo CỔNG, không diệt theo tên lệnh**: `go run` sinh tiến trình con tên
   `exe/server`, nên `pkill -f "cmd/server"` để sót và server CŨ vẫn giữ cổng — mọi phép kiểm chứng
   sau đó chạy trên code cũ mà trông như "sửa không ăn". Cách chắc chắn:
   `kill -9 $(lsof -nP -iTCP:20800 -sTCP:LISTEN -t)` rồi mới chạy lại và chờ `/api/health`.
4. **Admin nền**: `cd admin && pnpm install && pnpm dev` (Vite `:20900`, proxy `/api` → `:20800`).
5. **Chrome**: dùng công cụ chrome-devtools MCP và **bắt buộc mở trang trong context cô lập**:
   `new_page` với `isolatedContext: "vnet-audit"`. Profile Chrome mặc định của máy này có extension
   chèn header `x-vibex-cached-at` và phục vụ lại body cũ **kể cả khi response có `Cache-Control: no-store`**
   — nó làm bảng trông như không cập nhật sau khi tạo/sửa/xoá và sẽ gây báo lỗi giả ở mọi trang.
   Đăng nhập `admin/admin123`, xác nhận menu động load được (`GET /api/route/getUserRoutes`).
   Không tin `get_network_request` cho URL lặp lại — nó có thể trả body đã lưu của lần gọi trước;
   muốn chắc thì đọc phản hồi bằng `evaluate_script` (hook XHR) hoặc `curl`.
   **Hai cái bẫy đo đạc đã mắc phải, đừng lặp lại:**
   - Với thành phần có lớp phủ của Element Plus (chọn ngày, combobox, dropdown), **phải bấm bằng công cụ
     `click` của MCP**. Cú `.click()` tổng hợp trong `evaluate_script` cập nhật được ô hiển thị nhưng
     **không kích hoạt `@change`**, nhìn y hệt "bộ lọc là nút chết" — đã suýt báo nhầm một lần.
   - Hook XHR cài trong `evaluate_script` **không mất khi điều hướng SPA**. Cài chồng hai lần thì mỗi
     request bị đếm hai, nhìn y hệt "một lần bấm gửi hai request". Tải lại trang trước khi cài hook mới.
   - Toast của Element Plus **tự tắt sau ~3 giây**. Thao tác nào có chờ mạng lâu (in thử phải quay số
     TCP tới máy in, sao lưu, gọi máy trạm) thì đọc `.el-message` sau khi bấm là **hụt**, nhìn y hệt
     "nút bấm không phản hồi". Cài `MutationObserver` lên `document.body` TRƯỚC khi bấm rồi mới chờ.
6. **Máy trạm ảo** (để mở được luồng máy/phiên mà không cần app Wails): curl heartbeat không cần auth
   `POST /api/machines/by-code/:code/heartbeat` cho 2–3 mã máy → máy chuyển `offline` → `available`.

Bất kỳ bước nào hỏng: đó chính là lỗi số 1 — ghi sổ, sửa, rồi mới đi tiếp.

## Giai đoạn 1 — Bấm từng chức năng (một trang mỗi vòng, đúng thứ tự)

**Nền tảng**: dashboard · members · member-groups · categories · products · suppliers
**Vận hành**: machines · machine-groups · machine-assets · sessions · bookings · shifts · attendance
**Bán hàng**: orders · transactions · combos · promotions · cards · printers
**Kho**: inventory · inventory-counts · stock-transactions
**Khác**: curfew · website-blocking · notifications · feedback · app-updates · reports · backups · audit · settings
**Hệ thống (Soybean)**: system/user · system/role · system/menu · user-center · đổi mật khẩu

### Mỗi trang phải bấm đủ 9 mục, không bỏ mục nào

1. **Vào trang**: `navigate_page` → `take_snapshot`. Bảng có load dữ liệu? Cột có đúng
   (không rỗng, không `undefined`, không PascalCase lòi ra, tiền tệ đúng định dạng `@/utils/money`)?
2. **Phân trang**: sang trang 2, đổi số dòng/trang. Kiểm tham số gửi đúng
   (`page`/`page_size` cho API VNET, `current`/`size` cho `/api/systemManage/*`) và `total` khớp.
3. **Tìm kiếm / bộ lọc / sắp xếp**: nhập từ khoá, chọn từng bộ lọc, rồi xoá bộ lọc.
   Kết quả rỗng phải hiển thị đàng hoàng, không vỡ giao diện.
4. **Thêm mới**: mở form → **submit rỗng để xem validate** → điền hợp lệ → lưu →
   dòng mới phải xuất hiện; **reload trang, dữ liệu vẫn còn**.
5. **Sửa**: mở đúng bản ghi vừa tạo, form phải **điền sẵn đúng giá trị cũ**, đổi 1–2 trường, lưu, bảng cập nhật.
6. **Xoá**: xoá bản ghi vừa tạo, xác nhận hộp thoại, bản ghi biến mất.
7. **Mọi nút còn lại**: xem chi tiết, xuất file, in, chọn nhiều dòng, bật/tắt trạng thái, nạp tiền,
   mở/đóng phiên, điều khiển từ xa, đặt lại mật khẩu… — bấm hết, không bỏ nút nào vì "chắc là chạy".
8. **Biên**: số 0, số âm, chuỗi rỗng, ngày quá khứ/tương lai, số dư không đủ, hết hàng, trùng mã,
   tiếng Việt có dấu, chuỗi rất dài.
9. **Bằng chứng bắt buộc**:
   - `list_console_messages` → **0 lỗi đỏ**.
   - `list_network_requests` → **0 request non-2xx ngoài dự kiến**; mọi phản hồi `/api/*` có `code: 0`
     (trừ case đang cố tình test lỗi).
   - `take_screenshot` lưu làm chứng.
   - Đọc log backend cùng khoảng thời gian → không panic, không error lạ.

### Kiểm tra xuyên trang (sau khi xong nhóm liên quan)

- **Realtime**: 2 tab. Tab A nạp tiền hội viên → tab B tự cập nhật số dư (`balance:updated`).
  Tab A bán hàng → tab B thấy tồn kho đổi (`stock:changed`). Không thấy thì soi
  WebSocket `GET /api/ws/client?token=...` trong Network.
- **Luồng đầu-cuối**: tạo hội viên → nạp tiền → mở phiên trên máy → bán thêm đồ ăn → đóng phiên →
  đối chiếu số dư, đơn hàng, tồn kho, báo cáo doanh thu có khớp nhau không.
- **Phân quyền**: đăng nhập lại bằng `manager` rồi `staff`. Menu phải khác nhau; nút vượt quyền bị chặn
  (kiểm cả phản hồi API, không chỉ ẩn nút trên giao diện).
- **Phiên đăng nhập**: token hết hạn tự refresh (`9999`), ép đăng xuất (`8888`), modal đăng xuất (`7777`).
- **i18n**: đổi Việt / English / 中文 trên vài trang — không được lòi khoá thô `vnetPages.*`, `route.*`.
- **Giao diện**: `resize_page` nhỏ lại xem bảng/form có vỡ; thử chế độ tối.

## Giai đoạn 2 — Xử lý lỗi (ngay khi gặp, không để dồn)

1. **Tái hiện** 2 lần, ghi chính xác các bước bấm.
2. **Khoanh tầng**: request/response trong Network + log backend → lỗi ở admin (sai client, sai transform,
   sai tên field) hay backend (handler / service / model)?
3. **Sửa tối thiểu**, đúng chỗ. Không refactor, không "cải thiện" code kề bên. Mỗi dòng đổi phải truy được
   về một lỗi trong sổ.
4. **Bấm lại đúng kịch bản đã hỏng trên trình duyệt** → chụp màn hình chứng minh đã hết.
5. Lỗi ở logic backend → **thêm unit test hồi quy** trong `internal/service` theo khuôn
   `newMockDB(t)` + `go-sqlmock` ở `service_test.go` (test assert SQL chính xác — đổi query là gãy test của nó).
6. Sau khi sửa: backend → khởi động lại tiến trình + `go test ./... -count=1`;
   admin → `pnpm typecheck && pnpm lint`.
7. Ghi sổ, commit một lỗi một commit (Conventional Commits) trên nhánh `fix/ui-audit`.

### Bẫy đã biết của repo — soi kỹ (từ AGENTS.md)

- Field model thiếu `json:"snake_case"` → giao diện hiện rỗng/`undefined`. `PasswordHash` phải `json:"-"`.
- **Tên khoá JSON sai là hỏng im lặng**: `prop: 'order_count'` trỏ vào DTO trả `total_orders` → cột trống,
  không lỗi, không cảnh báo. Đổi tag thì grep `admin/src/views/vnet` tìm tên cũ.
- Model mới không có trong `db.AutoMigrate(...)` ở `cmd/server/main.go` → vô hình. AutoMigrate không bao giờ DROP cột.
- FK/ngày nullable là `*string`; chuỗi rỗng `''` bị PostgreSQL từ chối vì không phải `uuid`/`date`, GORM nuốt lỗi.
- Mảng/jsonb phải dùng `model.IntArray` / `model.StringArray`, `[]int`/`[]string` trần sẽ lỗi hoặc bị bỏ qua.
- Soft delete hai mặt: sinh mã tuần tự phải `Unscoped()`; tra cứu bản ghi lịch sử cũng phải `Unscoped()`
  nếu không cột sẽ trống khi bản ghi cha bị xoá.
- Trạng thái máy chỉ có `offline`/`available`/`in_use`; giá trị khác → 400.
- Hai quy ước API và **hai HTTP client** (`api/client.ts` vs `service/request`) — dùng nhầm là bảng không ra dữ liệu.
  Bảng VNET phải đi qua `vnetTransform`/`vnetSimpleTransform`.
- **Mỗi trang `views/vnet/` phải có ĐÚNG MỘT phần tử gốc** — fragment root làm hỏng `<Transition>` và
  **từ lần điều hướng thứ hai trở đi mọi trang render trắng**. `python3 scripts/check-admin.py` bắt được lỗi này.
- Thêm trang mới cần đủ **5 chỗ**: `route.go` · `views/vnet/{feature}/index.vue` (+ `pnpm gen-route`) ·
  3 file locale · `system_manage.go` · `store/modules/route/shared.ts` (nhóm menu).
- Tiền tệ dùng `formatPrice`/`formatAmount` của `@/utils/money`, **không** `toLocaleString()`.
- **Đo mốc so sánh thì dùng `git worktree add`, TUYỆT ĐỐI không `git stash`.** Muốn biết lint/verify của bản
  gốc ra sao, `git worktree add /tmp/vnet-head HEAD` rồi chạy ở đó; `git stash push` cuốn sạch cây làm việc
  (một lần đã cuốn 82 file / 2521 dòng của đợt kiểm này, may mà `stash pop` lấy lại được ngay).
- **`scripts/verify.py` phải chạy trên CSDL sạch** (`DROP DATABASE` → `migrate` → `seed` → `migrate`), và phải
  **tắt server trước khi drop** — còn kết nối thì lệnh drop im lặng không làm gì, harness chạy lại trên dữ liệu cũ
  và đẻ ra hàng chục lỗi giả. URL mặc định của nó là `:8099`, không phải `:20800`.

## Sổ theo dõi `docs/UI-AUDIT.md` — bộ nhớ giữa các vòng

Cập nhật **cuối mỗi vòng**, trước khi làm gì khác. Nếu context bị nén hoặc phiên đứt, việc đầu tiên khi
quay lại là đọc file này rồi tiếp đúng dòng dang dở. Ảnh chụp để trong `docs/ui-audit-shots/`.

```md
## Môi trường
backend PID … (log /tmp/vnet-backend.log) · admin PID … · pg container vnet-ui-audit-pg · trang Chrome …

## Checklist chức năng
| Trang | Xem | Lọc/Phân trang | Thêm | Sửa | Xoá | Nút khác | Console/Network | Ảnh | Trạng thái |
|---|---|---|---|---|---|---|---|---|---|
| members | ✅ | ✅ | ✅ | ✅ | ✅ | nạp tiền ✅ | sạch | members-01.png | PASS |
| orders | – | – | – | – | – | – | – | – | TODO |

## Lỗi
| # | Trang | Các bước bấm | Hiện tượng | Tầng | File:dòng | Cách sửa | Ảnh sau sửa | Trạng thái |
|---|---|---|---|---|---|---|---|---|
| 1 | products | Thêm → bỏ trống giá | 500, uuid rỗng | backend | service/product.go:64 | guard `*string` | products-fix.png | FIXED |
```

Trạng thái trang: `TODO` → `IN_PROGRESS` → `PASS`. Lỗi: `OPEN` → `FIXED`.

## Báo cáo cuối mỗi vòng (≤ 8 dòng)

```
Vòng N — <trang>
Đã bấm: <danh sách nút/luồng>
Console: sạch|<lỗi>    Network: sạch|<request hỏng>
Lỗi mới: <1 dòng, hoặc "không">
Đã sửa: <file:dòng → làm gì>
Còn lại: <số trang TODO>, <số lỗi OPEN>
```

## Điều kiện DỪNG toàn bộ

Mọi dòng checklist `PASS` kèm ảnh · mọi lỗi `FIXED` · các kiểm tra xuyên trang đạt ·
`go test ./... -count=1` xanh · `golangci-lint run` sạch · `pnpm typecheck && pnpm lint` sạch ·
`python3 scripts/check-admin.py` sạch.

Khi đó in tổng kết: số trang đã bấm, toàn bộ lỗi đã sửa, và đánh dấu `MANUAL` cho 5 tính năng chỉ chạy
được trên Windows thật (khoá màn hình + chặn phím, in máy in nhiệt, ghi tệp hosts, cài bản cập nhật,
chụp màn hình GDI — xem `scripts/KIEM-CHUNG-WINDOWS.md`) — rồi dừng vòng lặp.
