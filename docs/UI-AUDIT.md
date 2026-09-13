# Sổ kiểm chứng giao diện VNET (bấm tay trên trình duyệt)

Prompt điều khiển: `docs/UI-AUDIT-PROMPT.md`. Ảnh chụp: `docs/ui-audit-shots/`.

## Môi trường (vòng 1 — 2026-09-13)

| Thành phần | Trạng thái |
|---|---|
| Postgres | container `vnet-ui-audit-pg` (postgres:16-alpine, 127.0.0.1:5432, vnet/vnet/vnet). Host **không có** `psql` — dùng `docker exec vnet-ui-audit-pg psql -U vnet -d vnet -c "…"` |
| Backend | `go run ./cmd/server` chạy nền, log `/tmp/vnet-backend.log`, `:8080`, `/api/health` → 200 |
| Admin | `pnpm dev` chạy nền, log `/tmp/vnet-admin.log`, Vite `:3000` |
| Chrome | chrome-devtools MCP. **Dùng pageId = 31, `isolatedContext="vnet-audit"`** — profile mặc định (pageId 30) có extension chèn header `x-vibex-cached-at` và phục vụ lại body cũ **kể cả khi response có `Cache-Control: no-store`**, làm mọi quan sát sai lệch. Context cô lập sạch. Đã đăng nhập `admin/admin123` |
| Dữ liệu nền | `cmd/migrate` 19 bước OK · `cmd/seed` OK (3 user, 3 role, 3 nhóm hội viên, 2 nhóm máy). **Chưa có máy / hội viên / sản phẩm** — tạo dần qua giao diện |
| Nhánh git | `feature/core`, **đang có thay đổi chưa commit của người dùng** (machines, locales, router). Chưa tự commit gì — sẽ hỏi trước khi commit |

## Checklist chức năng

Trạng thái: `TODO` → `IN_PROGRESS` → `PASS` · `PARTIAL` = đã bấm nhưng còn phần phải kiểm lại.

| # | Trang | Xem | Lọc/Phân trang | Thêm | Sửa | Xoá | Nút khác | Console/Network | Ảnh | Trạng thái |
|---|---|---|---|---|---|---|---|---|---|---|
| 1 | dashboard | ✅ | n/a | n/a | n/a | n/a | reload giữ phiên ✅ | sạch | dashboard-01-empty.jpeg | PARTIAL |
| 2 | members | ✅ | ✅ | ✅ | ✅ | ✅ | Chi tiết ✅ Nạp ✅ Hoàn tiền ✅ Reset MK ✅ Xếp hạng ✅ Cài đặt cột ✅ | sạch | members-01-pass.jpeg | PASS |
| 3 | member-groups | ✅ | tìm kiếm ✅ (không phân trang) | ✅ | ✅ | ✅ | cờ Mặc định ✅ chặn xoá nhóm có hội viên ✅ chặn xoá nhóm mặc định ✅ | sạch | member-groups-01-pass.jpeg | PASS |
| 4 | categories | ✅ cây phân cấp | n/a (không có ô tìm) | ✅ | ✅ | ✅ chặn khi còn danh mục con | cây cha–con ✅ | sạch (1×400 do cố ý thử) | categories-01-pass.jpeg | PASS |
| 5 | products | ✅ | tìm kiếm ✅ (đã bỏ dấu sẵn) | ✅ | ✅ | ✅ | Nhập/Trừ kho khoá đúng khi chưa có tồn ✅ | 1 cảnh báo el-radio (thư viện) | products-01-pass.jpeg | PASS |
| 6 | suppliers | ✅ | n/a (không có ô tìm) | ✅ | ✅ | ✅ chặn khi còn sản phẩm | — | sạch | suppliers-01-pass.jpeg | PASS |
| 7 | machines | ✅ | ✅ | ✅ (đơn + **tạo hàng loạt**) | ✅ | ✅ | điều khiển từ xa ✅ nhịp tim ✅ Phần cứng ✅ | sạch | machines-01-pass.jpeg | PASS |
| 8 | machine-groups | ✅ | tìm kiếm ✅ | ✅ | ✅ | ✅ chặn khi còn phụ thuộc | **Bảng giá** (2 tab) ✅ | sạch | machine-groups-01-pass.jpeg | PASS |
| 9 | machine-assets | ✅ | lọc theo máy ✅ | ✅ | ✅ (qua "Kiểm tra") | ✅ | Kiểm tra ghi tình trạng + mốc thời gian ✅ | sạch (sau khi sửa) | machine-assets-01-pass.jpeg | PASS |
| 10 | sessions | ✅ | n/a | ✅ Mở máy | n/a | ✅ Kết thúc | Tính thử ✅ Đổi máy ✅ | sạch | sessions-01-pass.jpeg | PASS |
| 11 | bookings | ✅ | tìm kiếm ✅ (đã sửa) lọc trạng thái ✅ | ✅ | ✅ | ✅ | Check-in / Đánh vắng / Huỷ ✅ tác vụ nền tự đánh vắng ✅ | sạch | bookings-01-pass.jpeg | PASS |
| 12 | shifts | ✅ | lọc trạng thái ✅ | ✅ Mở ca | n/a | n/a | Đóng ca ✅ Bàn giao ✅ đối chiếu quỹ ✅ | sạch | shifts-01-pass.jpeg | PASS |
| 13 | attendance | ✅ | lọc theo ngày ✅ | cấu hình thưởng ✅ | – (không sửa lượt) | – (không xoá lượt) | Lưu cấu hình ✅ Làm mới ✅ xoá bộ lọc ✅ | sạch | attendance-01-pass.jpeg | PASS |
| 14 | orders | ✅ | lọc ngày + tìm kiếm ✅ | ✅ tạo đơn | ✅ sửa/tách đơn | ✅ huỷ đơn | Xác nhận ✅ Thanh toán ✅ In hoá đơn ⏳ | sạch | orders-01-pass.jpeg | PASS |
| 15 | transactions | ✅ | lọc ngày ✅ loại ✅ tìm kiếm ✅ | n/a (sổ chỉ đọc) | n/a | n/a | 2 cột khuyến mãi mới ✅ | sạch | transactions-01-pass.jpeg | PASS |
| 16 | combos | ✅ | tìm kiếm ✅ (đã sửa bỏ dấu) | ✅ | ✅ | ✅ | Mua ✅ Kích hoạt ✅ Combo đã mua ✅ | sạch | combos-01-pass.jpeg | PASS |
| 17 | promotions | ✅ | tìm kiếm ✅ (đã sửa) | ✅ bộ dựng luật | ✅ | ✅ | Ô thưởng vòng quay ✅ Quay thử ✅ | sạch | promotions-01-pass.jpeg | PASS |
| 18 | cards | ✅ 2 tab | tìm kiếm ✅ lọc trạng thái ✅ | ✅ Sinh thẻ lô | n/a | ✅ Huỷ thẻ | Đổi thẻ ✅ Bán thẻ ✅ Tra thẻ quà tặng ✅ | sạch | cards-01-pass.jpeg | PASS |
| 19 | printers | ✅ | n/a | ✅ | ✅ | ✅ (kèm chặn xoá máy mặc định) | In thử ✅ Phân luồng món ✅ | sạch | round31-printers.png | PASS |
| 20 | ~~inventory~~ | — | — | — | — | — | **Không tồn tại trang này**: menu "Tồn kho" trỏ tới `stock-transactions` (dòng 22). Đã gộp | — | — | KHÔNG ÁP DỤNG |
| 21 | inventory-counts | ✅ | n/a | ✅ mở phiên | ✅ ghi/bỏ dòng đếm | ✅ huỷ phiên | Đếm hàng ✅ Chốt phiên ✅ | sạch | inventory-counts-01-pass.jpeg | PASS |
| 22 | stock-transactions ("Tồn kho") | ✅ | n/a | ✅ phiếu nhập/xuất thủ công | n/a (sổ chỉ ghi thêm) | n/a | thành tiền tự tính ✅ | sạch | stock-transactions-01-pass.jpeg | PASS |
| 23 | curfew | ✅ | n/a | ✅ | ✅ | ✅ | Miễn trừ ✅ cưỡng chế thật ✅ | sạch (sau khi sửa) | curfew-01-pass.jpeg | PASS |
| 24 | website-blocking | ✅ | ✅ tìm + xoá bộ lọc | ✅ | ✅ | ✅ | Bật/tắt ✅ lịch ✅ phạm vi máy ✅ tab vi phạm ✅ | sạch | round24-website-blocking.png | PASS |
| 25 | notifications | ✅ | n/a | ✅ | ✅ | ✅ | Gửi tới hội viên ✅ chống gửi trùng ✅ | sạch | round23-notifications.png | PASS |
| 26 | feedback | ✅ | ✅ lọc máy/sao/có nhận xét + phân trang | n/a (trang chỉ đọc) | n/a | n/a | Thẻ tổng hợp ✅ phân bố sao ✅ | sạch | round25-feedback.png | PASS |
| 27 | app-updates | ✅ | n/a (không có lọc/phân trang) | ✅ | n/a (không có sửa) | ✅ | Bật/tắt phát hành ✅ chặn xoá bản đang phát hành ✅ | sạch (chỉ 4xx do cố tình test) | round26-app-updates.png | PASS |
| 28 | reports | ✅ 7 tab | ✅ lọc ngày + cài đặt cột theo tab | n/a (trang chỉ đọc) | n/a | n/a | Làm mới ✅ xoá lọc ✅ | sạch | round27-reports.png | PASS |
| 29 | backups | ✅ | ✅ phân trang | ✅ tạo sao lưu | n/a | ✅ (kèm xoá tệp) | Phục hồi ✅ (đường lỗi) chặn xoá bản đang chạy ✅ | sạch | round28-backups.png | PASS* |
| 30 | audit | ✅ | ✅ lọc hành động/đối tượng/ngày + phân trang | n/a (chỉ đọc) | n/a | n/a | Mở rộng xem metadata ✅ | sạch | round29-audit.png | PASS |
| 31 | settings | ✅ 5 tab | n/a | ✅ lưu từng nhóm | ✅ | ✅ (xoá mệnh giá) | Công tắc tính năng ✅ có hiệu lực thật ✅ | sạch (sau khi sửa) | round30-settings.png | PASS |
| 32 | system/user | ✅ | ✅ lọc + đặt lại | ✅ | ✅ | ✅ (đơn + hàng loạt) | Gán vai trò ✅ khoá tài khoản ✅ | sạch | round32-system-user.png | PASS |
| 33 | system/role | ✅ | ✅ lọc + đặt lại | ✅ | ✅ | ✅ (đơn + hàng loạt, chặn vai trò đang dùng) | **Phân quyền ✅ có hiệu lực thật** | sạch | round33-system-role.png | PASS |
| 34 | system/menu | ✅ | ✅ phân trang (sau khi sửa) | n/a (chỉ đọc) | n/a | n/a | Cài đặt cột ✅ | sạch (sau khi sửa) | round34-system-menu.png | PASS |
| 35 | user-center + đổi mật khẩu | ✅ | n/a | n/a | n/a | n/a | **Đổi mật khẩu ✅ có hiệu lực thật** | sạch | round35-user-center.png | PASS |

### Kiểm tra xuyên trang

| # | Hạng mục | Trạng thái | Ghi chú |
|---|---|---|---|
| X1 | Realtime `balance:updated` / `stock:changed` (2 tab) | **PASS** | Nạp tiền ở tab A → bảng Hội viên tab B tự đổi 129.750 → 206.750 (sự kiện `member:updated`, gửi tới mọi kết nối admin). Xác nhận đơn 3 chai nước → bảng Sản phẩm tab B tự đổi 62 → 59 (`stock:changed`). Không tải lại trang lần nào |
| X2 | Luồng đầu-cuối: hội viên → nạp tiền → phiên → bán hàng → đóng phiên → báo cáo | **PASS** (kèm 1 phát hiện) | Khách mới `khachx2`: nạp 200.000 → bán 2 chai nước 20.000 (trả bằng số dư) → phiên 1 phút 250₫ → số dư 179.750. Sổ ví khớp từng dòng (0 → 200.000 → 180.000 → 179.750), `total_spent` = 20.250 (không tính tiền nạp), phút chơi = 1, tồn kho 59 → 57. Xem phát hiện #79 về công thức doanh thu |
| X3 | Phân quyền manager / staff | **PASS sau khi sửa** | Menu ba vai trò khác nhau đúng (owner 35 mục, manager 30, staff 24). Kiểm cả phía API chứ không chỉ ẩn nút. Phát hiện lỗi #80 (leo thang đặc quyền) và sửa |
| X4 | Phiên đăng nhập 9999 / 8888 / 7777 | **PASS sau khi sửa** | 8888 khi thiếu token / dùng refresh token làm access token ✅; 9999 khi token hết hạn ✅. Phát hiện lỗi #81 (26 trang VNET không làm mới token) và sửa. **7777 không có chỗ nào phát ra** — `response.SessionExpired()` tồn tại nhưng không ai gọi; admin vẫn xử lý được nếu sau này dùng tới |
| X5 | i18n vi / en / zh | **PASS sau khi sửa** | Quét 26 trang × 3 ngôn ngữ: **0 khoá thô** lọt ra màn hình. Ba file locale có cùng bộ khoá. Phát hiện lỗi #83 (tên sản phẩm thành "SoybeanAdmin" khi đổi sang EN/ZH) và sửa |
| X6 | Thu nhỏ cửa sổ + chế độ tối | **PASS sau khi sửa** | Thu về bề ngang điện thoại: 7 trang chính **không tràn ngang**, menu tự thu, form đọc được. Chế độ tối: phát hiện lỗi #84 (chữ/nền viết cứng màu sáng) và sửa |
| X7 | `go test ./...` · `golangci-lint` · `pnpm typecheck && lint` · `check-admin.py` · `verify.py` | **PASS** | `go test ./...` 13/13 gói xanh · `go vet` sạch · `check-admin.py` 0 phát hiện · `golangci-lint` 54 · `pnpm typecheck` 49 · `pnpm lint` 71 — **cả ba con số đều là mốc có sẵn của bản gốc, đợt sửa không thêm lỗi nào**. `verify.py` trên CSDL sạch: **293 ok / 14 hỏng** so với bản gốc **291 ok / 14 hỏng**, danh sách 14 lỗi **trùng khớp từng dòng** |

## Lỗi

| # | Trang | Các bước bấm | Hiện tượng | Tầng | File:dòng | Cách sửa | Trạng thái |
|---|---|---|---|---|---|---|---|
| 1 | members | Thêm → lưu → không bấm Làm mới | Bảng không hiện bản ghi mới (lặp 2/2 lần), phải bấm "Làm mới" mới thấy | **môi trường, KHÔNG phải app** | – | Chạy audit trong `isolatedContext`; trong context sạch dòng mới hiện sau **251 ms** | ĐÓNG (không phải lỗi app) |
| 2 | toàn API | — | `/api/*` không gửi bất kỳ header hạn dùng nào (không `Cache-Control`/`ETag`/`Last-Modified`), nên mọi tầng cache đứng giữa được phép trả lại danh sách cũ | backend | `internal/middleware/nocache.go` (mới) + `internal/router/router.go:78` | Thêm `middleware.NoCache()` → `Cache-Control: no-store` cho nhóm `/api`, kèm test `TestNoCache_SetsNoStore` | FIXED (gia cố — **không** phải nguyên nhân lỗi #1) |
| 3 | members (và mọi trang danh sách) | Tạo 5 hội viên rồi xem thứ tự bảng | Danh sách sắp theo `id DESC` mà `id` là UUID ngẫu nhiên → thứ tự vô nghĩa; hội viên vừa tạo có thể rơi vào trang bất kỳ, người dùng tưởng tạo hỏng | backend | `pkg/pagination/pagination.go:35` | Đổi `DefaultSort` từ `id` sang `created_at`. `backup_logs` không có cột đó nên `BackupService.List` tự đặt `started_at`. Kiểm thật: danh sách hội viên nay ra đúng thứ tự mới-nhất-trước. 3 test phân trang cập nhật | FIXED |
| 4 | members | Xoá hội viên → tạo lại cùng tên đăng nhập | Thông báo lọt tên index thô: `Dữ liệu 'idx_members_username' đã tồn tại` | backend | `internal/handler/error.go:19` | 3 khoá trong `constraintMessages` viết sai tiền tố (`uni_` thay vì `idx_` — GORM đặt tên thẻ `uniqueIndex` là `idx_<bảng>_<cột>`): members/users/orders. Đã sửa + test `TestHandleCreateError_DichTenIndexTrung` | FIXED |
| 5 | members | Gõ "Nguyen Van" ở ô tìm kiếm | Không ra kết quả dù có "Nguyễn Văn Anh"; trang Sản phẩm thì tìm được vì đã bỏ dấu | backend | `internal/service/member.go:272` | Dùng `unaccent()` như `product.go:125`. Đã sửa + test `TestMemberService_List_SearchBoDauTiengViet` | FIXED |
| 6 | mọi bảng | Thu nhỏ cửa sổ về cỡ điện thoại | `pagerCount: 3` bị validator của Element Plus loại (chỉ nhận số lẻ 5..21) → rơi về mặc định 7, dải số trang **dài hơn** trên điện thoại, kèm cảnh báo Vue ở mọi bảng | admin | `src/hooks/common/table.ts:115` | Đổi 3 → 5. Đã kiểm ở bề rộng 500px: console sạch, không có thanh cuộn ngang | FIXED |
| 7 | toàn admin | `pnpm typecheck` | **51 lỗi TypeScript có sẵn** ở 19 file (products 6, orders 6, store/route 5, …). Pre-commit hook chạy `pnpm typecheck && pnpm lint` nên về nguyên tắc mọi commit đều bị chặn | admin | 19 file | **Chưa sửa** — không phải do thay đổi của tôi (0 lỗi ở file tôi đụng), và sửa 51 lỗi vượt xa phạm vi một vòng. Cần người dùng quyết | OPEN (chờ quyết định) |
| 8 | toàn backend | Hoàn tiền vượt số dư / trùng tên đăng nhập | Thông báo lỗi tiếng Anh lọt ra giao diện tiếng Việt: `insufficient balance`, `username already exists` | backend | nhiều service | Dịch **89 thông điệp** ở 11 file service sang tiếng Việt; sửa kèm 6 file test khẳng định chuỗi cũ. Kiểm thật: `GET /members/<id sai>` → "không tìm thấy hội viên", `GET /machines/<id sai>` → "không tìm thấy máy" | FIXED |
| 9 | member-groups | Gõ bất kỳ từ khoá nào rồi bấm Tìm kiếm | Ô tìm kiếm là **nút chết**: admin gửi `?search=`, handler gọi `GetGroups()` không nhận tham số nên luôn trả đủ danh sách — không lỗi, không dấu hiệu | backend | `internal/service/member.go` `GetGroups` + `internal/handler/member.go:297` | Cho `GetGroups(search string)` lọc theo tên có bỏ dấu; handler truyền `c.Query("search")`. 2 test mới | FIXED |
| 10 | member-groups | `POST /api/member-groups` với `discount_percent:150` / `-20`, `min_spent:-500000` | Backend nhận tuốt; chỉ ô nhập trên giao diện mới kẹp. Bảng quản trị hiện "150%" cho người vận hành đọc và tin (giá phiên chơi có tự kẹp về 100 nên tiền không âm) | backend | `internal/service/member.go` DTO `CreateGroupRequest`/`UpdateGroupRequest` | Thêm `binding:"omitempty,min=0,max=100"` và `min=0`; bổ sung 2 nhãn tiếng Việt vào `validationFieldLabels`. 2 test mới | FIXED |
| 11 | machines (cả 3 đường) | `POST /machines`, `PUT /machines/:id`, `POST /machines/batch` với `group_id:""` | Lỗi PostgreSQL thô lọt thẳng ra người dùng: `ERROR: invalid input syntax for type uuid: "" (SQLSTATE 22P02)`. Đúng cái bẫy `*string` rỗng mà AGENTS.md §2 cảnh báo | backend | `internal/service/machine.go` (Create, Update, BatchCreateMachines) | Thêm `nhomRongThanhNil()` dùng chung cho cả 3 đường → ghi NULL. 2 test mới | FIXED |
| 12 | machine-groups | Gõ từ khoá rồi bấm Tìm kiếm | Nút chết y hệt lỗi #9: `ListGroups()` không nhận tham số, gõ gì cũng ra đủ 3 nhóm | backend | `internal/service/machine.go` `ListGroups` + `internal/handler/machine.go:320` | Cho nhận `search`, lọc theo tên có bỏ dấu. 1 test mới, sửa 1 test cũ | FIXED |
| 13 | machine-groups | Xoá nhóm còn dòng bảng giá | Chặn đúng (409) nhưng chỉ sai lối thoát: "hãy **chuyển chúng sang nhóm khác** trước" — dòng bảng giá không có thao tác chuyển nhóm, chỉ xoá được. Chính chú thích trong code cũng nói thông điệp phải nêu đúng cách sửa | backend | `internal/service/rang_buoc.go:26` + `machine.go` DeleteGroup | Thêm trường `GoiY` riêng cho từng phụ thuộc, giữ gợi ý chung làm mặc định. 2 test mới | FIXED |
| 14 | machines | Gõ từ khoá vào ô "Tìm mã máy / nhóm" | **Lần thứ ba** cùng một lỗi: `MachineService.List` bỏ qua `params.Search`, gõ gì cũng ra đủ máy. Phát hiện bằng cách quét toàn bộ endpoint danh sách chứ không chờ gặp | backend | `internal/service/machine.go` `List` | Lọc theo mã máy **và** tên nhóm (truy vấn con, bỏ dấu, không tính nhóm đã xoá). 1 test mới | FIXED |
| 15 | machine-assets | Vào trang khi bảng chưa có dữ liệu | Console cảnh báo `vnetPages.assets.types.undefined`; el-table dựng ô mẫu một lần với `row` rỗng. Nếu backend trả về dòng thiếu `asset_type`/`status` thì ô sẽ in **nguyên khoá thô** ra cho người vận hành đọc | admin | `src/views/vnet/machine-assets/index.vue:20,197` | Chặn tra khoá khi giá trị rỗng, hiện "-" | FIXED |
| 16 | sessions | Mở phiên trên máy đã có phiên, sau khi tác vụ nền đặt máy về "offline" | **Hai phiên cùng chạy trên một máy** — hai khách bị tính tiền cho một chỗ ngồi. Chốt chặn cũ so `machine.Status == "in_use"`, mà `status` bị tác vụ `machines:mark-offline` ghi đè khi máy trạm treo/rớt mạng giữa phiên. Dựng lại được trên hệ thống thật: phiên mở 01:29:12, tác vụ nền đặt offline 01:29:15, lệnh mở phiên tiếp theo lọt | backend | `internal/service/session.go` StartSession | Đếm phiên đang chạy **trên máy** trong transaction thay vì tin cột status (đúng cách phía hội viên đang làm). Thông báo nêu rõ mã máy. 1 test mới + sửa 3 test cũ | FIXED |
| 17 | sessions / tác vụ nền | Máy đang có khách chơi nhưng ngừng gửi nhịp tim | `machines:mark-offline` đặt máy về "offline" dù phiên còn chạy: quầy thấy máy trống trong khi khách đang ngồi, và đó chính là mắt xích tạo ra lỗi #16 | backend | `internal/scheduler/jobs.go` markStaleMachinesOffline | Loại trừ máy còn phiên `is_active`. Kiểm chứng thật: ép nhịp tim lùi 10 phút, sau 32s máy vẫn giữ `in_use`. 1 test mới | FIXED |
| 18 | sessions | Mở phiên trên máy **chưa gán nhóm** | Giá = 0₫ → khách chơi **miễn phí**, không cảnh báo ở bất kỳ đâu; cột "Còn lại" chỉ hiện "—". Quán thêm máy mà quên gán nhóm là mất doanh thu im lặng | backend + admin | `internal/service/session.go` CostBreakdown | Không chặn (quán có thể cố ý để máy miễn phí) mà **nói ra**: `CostBreakdown` thêm cờ `no_pricing`; hộp **Mở máy** hiện cảnh báo vàng ngay khi chọn phải máy chưa gán nhóm; trang Máy đánh dấu "Chưa gán nhóm — 0₫/giờ". Kiểm thật: chọn PC-05 → cảnh báo hiện, chọn PC-03 (nhóm VIP) → cảnh báo tắt. 2 test mới | FIXED |
| 19 | toàn hệ thống | Thăng hạng hội viên | `total_spent` — cột duy nhất quyết định thăng hạng — **chỉ tăng khi mua gói cước** (`combo.go:444`). Tiền chơi máy và đơn hàng không cộng vào. Khách tiêu 5 triệu tiền giờ vẫn ở hạng Đồng; nút "Xếp lại hạng" và tác vụ nền 15 phút đều vô tác dụng | backend | `internal/service/combo.go:444` là nơi ghi duy nhất; `member.go:796` là nơi đọc | **Chốt ý nghĩa: `total_spent` = tiền khách thực tiêu tại quán** = tiền giờ chơi + đơn đã hoàn tất (trừ đơn nạp tiền) + gói cước. Cộng tại `chargeSessionTo` và `settleOrder`; **nạp tiền không tính**. Migration `backfill_member_total_spent` tính lại cho dữ liệu cũ. Kiểm thật: phiên 1 phút → +250₫, đơn 20.000₫ → +20.000₫, nạp 50.000₫ → không đổi, đặt mốc 600.000₫ → "Xếp lại hạng" đưa lên Bạc. 1 test mới + 2 test cũ cập nhật | FIXED |
| 20 | transactions | Chọn khoảng ngày = **hôm nay** | Bảng ra **0 dòng** dù cả 10 giao dịch đều của hôm nay. `time.Parse` neo vào UTC còn `created_at` lưu giờ Việt Nam (+7), nên "từ 13/09" thành 07:00 sáng giờ VN — **mọi giao dịch ca đêm 00:00–07:00 biến mất khỏi chính ngày của nó**, còn tối hôm trước lại lọt vào | backend | `internal/service/report.go` ListTransactions | Dùng `time.ParseInLocation` + `utils.StartOfDay/EndOfDay` (đã neo sẵn Asia/Ho_Chi_Minh); thêm `utils.VietnamLocation()`. 1 test mới | FIXED |
| 21 | transactions | Gõ "Tran Thi" vào ô tìm kiếm | 0 kết quả dù có "Trần Thị Bích" — lần thứ tư cùng lỗi bỏ dấu | backend | `internal/service/report.go` ListTransactions | `unaccent()` cho họ tên và tên đăng nhập | FIXED |
| 22 | transactions | Xem cột "Loại" | `attendance_bonus` hiện **nguyên khoá thô**; bản đồ dịch chỉ có 5/12 loại backend thật sự ghi ra (thiếu cả thưởng nạp, thẻ nạp, đặt cọc, thanh toán đơn, vòng quay…) | admin | `src/views/vnet/transactions/index.vue:19` + 3 file locale | Bổ sung đủ 14 loại, thêm 9 khoá × 3 ngôn ngữ | FIXED |
| 23 | transactions | Xem dòng thưởng điểm danh / phí chơi | "Số dư trước = Số dư sau" vì các dòng này chỉ động tới **số dư khuyến mãi** — sổ sách không đối chiếu được. API đã trả sẵn `bonus_before`/`bonus_after`, giao diện chỉ không hiện | admin | `src/views/vnet/transactions/index.vue` | Thêm 2 cột "Khuyến mãi trước/sau" + khoá locale 3 ngôn ngữ | FIXED |
| 24 | categories | Đặt A làm con của B, rồi đặt B làm con của A (hai lệnh PUT bình thường) | **Mất sạch cây danh mục**: khi không danh mục nào còn `parent_id` rỗng, hàm dựng cây không tìm ra gốc và `GET /api/categories` trả về **`null`** — trang Sản phẩm và thực đơn máy trạm mất theo, không một thông báo lỗi nào. Database không có ràng buộc nào chặn | backend | `internal/service/category.go` Create + Update | Thêm `chaHopLe()`: chặn tự làm cha của chính nó, đi ngược chuỗi cha để chặn vòng lặp, và chuẩn hoá `parent_id` rỗng thành NULL. 3 test mới | FIXED |
| 25 | categories | `POST /api/categories` với `parent_id:""` | Lỗi PostgreSQL thô lọt ra: `invalid input syntax for type uuid ""` — cùng bẫy với lỗi #11 nhưng ở bảng khác | backend | `internal/service/category.go` | Gộp vào `chaHopLe()` ở trên | FIXED |
| 26 | toàn backend (bổ sung cho #8) | Quét code tìm chỗ so khớp không bỏ dấu | Còn **6 chỗ** tìm kiếm chưa bỏ dấu, đều là chữ tiếng Việt người dùng gõ: tên khách đặt chỗ (`booking.go:118`), tên gói (`combo.go:147`), tên khuyến mãi (`promotion.go:173`), ghi chú đơn (`order.go:142`), họ tên nhân viên và tên vai trò (`system_manage.go:215,475`). Các chỗ còn lại là mã/serial/domain nên không cần | backend | `order.go`, `system_manage.go` | Thêm `unaccent()` cho các chỗ còn lại có chữ tiếng Việt: ghi chú đơn hàng, họ tên người dùng (2 chỗ), tên + mô tả vai trò. Seri thẻ, tên miền, số điện thoại, email giữ `ILIKE` trần vì không bao giờ có dấu | FIXED |
| 27 | products | Gán sản phẩm vào **danh mục con** | Cột Danh mục ghi **"Đã xoá"** cho một danh mục vẫn đang tồn tại, và ô chọn Danh mục **chỉ liệt kê danh mục gốc** nên không gán được vào danh mục con. Nguyên nhân: `GET /categories` trả về CÂY (con nằm trong `children`), trang Sản phẩm dùng thẳng mảng ngoài | admin | `src/views/vnet/products/index.vue` fetchCategories + ElOption | Thêm `traiPhangDanhMuc()` giữ thứ tự cha–con và thụt đầu dòng cho con | FIXED |
| 28 | products | `POST /api/products` với `category_id:""` | Lỗi PostgreSQL thô lọt ra — **lần thứ ba** cùng bẫy (máy, danh mục cha, sản phẩm) | backend | `internal/service/product.go` Create + Update | Đặt hàm dùng chung `uuidRongThanhNil()` (`internal/service/uuid_rong.go`) thay vì viết lại lần thứ ba. 1 test mới | FIXED |
| 29 | products | `POST /api/products` với `category_id` là UUID không tồn tại | Vẫn tạo được (`created`) — không có khoá ngoại, không kiểm tra; sản phẩm sau đó hiện "Đã xoá" ở cột danh mục | backend | `internal/service/product.go` | `danhMucTonTai()` kiểm mã danh mục trước khi ghi, dùng cho cả Create lẫn Update. Kiểm thật: mã bịa → "không tìm thấy danh mục", mã thật → tạo bình thường. 1 test mới + 2 test cũ cập nhật | FIXED |
| 30 | orders | Xác nhận đơn 5 món → **Huỷ đơn** | **Tồn kho không được hoàn lại**: 47 → 42 khi xác nhận, huỷ xong vẫn 42 — hàng bốc hơi khỏi sổ. Nhánh hoàn kho có sẵn trong code nhưng không bao giờ chạy: `tx.Model(&order).Updates(...)` của GORM **ghi giá trị mới ngược vào chính struct**, nên ngay dòng sau `order.Status` đã là `"cancelled"` và điều kiện `== "confirmed"` luôn sai | backend | `internal/service/order.go` UpdateStatus, nhánh `cancelled` | Nhớ `trangThaiTruoc := order.Status` trước khi ghi. Kiểm chứng thật: 42 → 37 → **42**. Thêm 2 phép kiểm vào `scripts/verify.py` | FIXED |
| 31 | orders | Xem tiêu đề cột bảng đơn | Hai cột cùng tên **"Thao tác"**: cột `updated_by_name` dùng khoá `orders.operator` mà bản tiếng Việt dịch nhầm thành "Thao tác" (tiếng Anh "Operator", tiếng Trung "操作人" đều đúng) | admin | `src/locales/langs/vi-vn.ts` | Đổi thành "Người thực hiện", khớp với trang Giao dịch | FIXED |
| 32 | products / toàn hệ thống | Xoá sản phẩm rồi tạo lại **cùng tên** | Báo "Tên sản phẩm đã tồn tại" trong khi danh sách không có sản phẩm nào tên đó: `idx_products_name` là UNIQUE thường nên **giữ luôn tên của bản ghi đã xoá mềm**. Quán ngừng bán rồi bán lại một món là gặp ngay | backend + migration | `cmd/migrate/main.go` | 7 migration đổi unique index sang **unique một phần** (`WHERE deleted_at IS NULL`) cho categories, combos, machine_groups, machines, products, members, users — giữ nguyên TÊN index để AutoMigrate không dựng lại bản đầy đủ. **Cố ý không đổi 4 index**: `orders.order_code` (mã đơn tra cứu Unscoped) và 3 index seri thẻ (seri không bao giờ được cấp lại). Kiểm thật: tạo "Mi Tom Thu 32" → xoá → tạo lại cùng tên → thành công | FIXED |
| 33 | stock-transactions | Bán hàng rồi mở sổ "Giao dịch tồn kho" | **Sổ kho trống trơn** dù tồn kho vẫn chạy: trừ kho cho **hàng bán thẳng** chỉ gọi `Update("current_stock", …)` mà không tạo bút toán nào. Nguyên liệu của món chế biến thì có ghi sổ — nên món chế biến truy được, còn nước đóng chai, mì gói (phần lớn hàng hoá) thì không. Kỳ kiểm kê không có gì để đối chiếu | backend | `internal/service/order.go` deductStockForOrder + restoreStockForOrder | Thêm `ghiSoKhoHangBanThang()` tạo bút toán `outbound`/`inbound` đúng khuôn nhánh nguyên liệu, kèm tồn trước–sau và mã đơn. Cập nhật 2 test cũ bị gãy do có thêm INSERT | FIXED |
| 34 | stock-transactions | Cột "Người thực hiện" của sổ kho | Phiếu thủ công ghi tên nhân viên, còn bút toán sinh từ đơn hàng bỏ trống — sổ không trả lời được "ai bán chỗ hàng này" | backend | `internal/service/order.go` | Truyền người thực hiện xuống bút toán. Riêng đường `Pay` chưa có (hàm `Pay` không nhận tham số người thực hiện, `settleOrder` cũng đang truyền `""`) — ghi chú trong code | FIXED (một phần) |
| 35 | inventory-counts | Mở 2 phiên kiểm kê cùng lúc, mỗi phiên đếm ra cùng một con số, rồi chốt cả hai | **Chốt hai lần áp chênh lệch hai lần**: sổ 35, hai người cùng đếm 30 → mỗi phiên ghi lệch −5 → chốt cả hai ra **25** trong khi kho thật có 30. Kiểm kê — việc sinh ra để sửa sai lệch — lại tự tạo sai lệch | backend | `internal/service/inventory_count.go` Open | Chỉ cho một phiên mở tại một thời điểm, kiểm bên trong advisory lock sẵn có. Thông báo nêu rõ mã phiên đang mở. 2 test mới, **đã chứng minh test fail khi gỡ chốt chặn** | FIXED |
| 36 | suppliers | Bấm Lưu khi chưa nhập tên | Ô chuyển đỏ nhưng **dòng báo lỗi trống không có chữ nào**: quy tắc validate viết `message: ''` trong khi khoá dịch `suppliers.form.nameRequired` đã có sẵn ở cả 3 ngôn ngữ, chỉ là chưa nối vào | admin | `src/views/vnet/suppliers/index.vue:18` | Nối khoá dịch vào. Quét toàn bộ `src/views`: đây là chỗ duy nhất còn `message: ''` | FIXED |
| 37 | products | `PUT /api/products/:id` chỉ gửi **một** trường (ví dụ đổi nhà cung cấp) | Bị chặn bằng câu **"Giá tối thiểu là 0"** — nói về một trường người gọi không hề đụng tới. `Price *int64` khai `binding:"min=0"` **thiếu `omitempty`**, nên validator coi con trỏ nil là vi phạm. Giao diện không lộ vì form Sửa luôn gửi đủ trường, nhưng mọi lệnh cập nhật một phần đều hỏng | backend | `internal/service/product.go:93` | Đổi thành `omitempty,min=0` — đúng khuôn cả repo đang dùng. Quét toàn bộ DTO: đây là chỗ duy nhất lệch. 2 test mới (vẫn chặn giá âm) | FIXED |
| 38 | suppliers | Tạo nhà cung cấp với email `khong-phai-email` | Chấp nhận, không kiểm định dạng email. Trùng tên cũng cho qua | backend | `internal/service/inventory.go` | `binding:"omitempty,email"` ở cả Create lẫn Update. Kiểm thật: `khong-phai-email` → 400 "Email không hợp lệ"; `a@b.com` → tạo được; sửa sang email sai → 400 | FIXED |
| 39 | combos / sessions | `POST /sessions/start` kèm `combo_purchase_id` | **Mở phiên bằng gói trả trước không bao giờ thành công**, người dùng nhận đúng chuỗi `current transaction is aborted (SQLSTATE 25P02)`. Nguyên nhân: lệnh gắn phiên vào gói chạy **trước** `tx.Create(&session)` nên ghi `current_session_id = ""` vào cột uuid; lỗi lại **không được kiểm**, transaction hỏng âm thầm rồi mọi lệnh sau đó nổ 25P02 | backend | `internal/service/session.go:486` | Chuyển lệnh xuống sau khi phiên đã có ID và kiểm lỗi. Cập nhật thứ tự kỳ vọng trong test cũ | FIXED |
| 40 | combos | Ô tìm kiếm gói | Chưa bỏ dấu — gõ "goi gio vang" không ra "Gói giờ vàng". Đây là 1 trong 6 chỗ đã liệt kê ở #26 | backend | `internal/service/combo.go:147` | `unaccent()`, cập nhật test cũ | FIXED |
| 41 | combos | Ô chọn "Loại" khi thêm gói | Tiếng Việt để nguyên **"Fixed slot"** trong khi tiếng Trung đã dịch (固定时段) và khoá anh em `prepaid` đã là "Trả trước" | admin | `src/locales/langs/vi-vn.ts` | Đổi thành "Khung giờ cố định". Quét cả file tiếng Việt: 30 chuỗi còn lại là tên riêng/thuật ngữ kỹ thuật, giữ nguyên là đúng | FIXED |
| 42 | cards + 3 trang khác | Gửi thiếu trường khi đổi thẻ nạp | Lọt **chuỗi lỗi thô của validator Go** ra người dùng: `Key: 'RedeemTopupCardRequest.Serial' Error:Field validation for 'Serial' failed`. 10 handler dùng `response.BadRequest(c, err.Error())` thay vì `handleValidationError(c, err)` — trong khi **92 chỗ khác trong repo dùng đúng** | backend | `card.go` (5), `website_block.go` (3), `app_update.go` (1), `printer.go` (1) | Đổi cả 10 sang `handleValidationError`. Sau khi sửa: "Số serial không được để trống" | FIXED |
| 43 | shifts | Đóng ca và đọc bảng | **Hai cột tiền cạnh nhau tự mâu thuẫn**: ca đầu 500.000₫, thu trong ca 40.000₫, đếm được 540.000₫ → bảng ghi "Tiền cuối 540.000 / **Tiền dự kiến 40.000** / Thừa-Thiếu 0". Người trực quầy đọc ra như đang thừa 500.000₫. Nguyên nhân: `expected_total` của backend là *tiền thu trong ca*, chưa gồm tiền đầu ca, nhưng cột lại mang nhãn "Tiền dự kiến" | admin | `src/views/vnet/shifts/index.vue:125` | Cộng tiền đầu ca vào khi hiển thị, đúng công thức backend dùng để tính Thừa/Thiếu. **Phép tính của backend vốn đã đúng**, chỉ hiển thị sai | FIXED |
| 44 | bookings | Gõ "Le Thi" hoặc "PC-05" vào ô tìm | Cả hai ra **rỗng**. Ô nhập ghi rõ "Tìm theo tên / mã máy" nhưng code chỉ tra `customer_name` và `customer_phone` bằng ILIKE trần — không bỏ dấu, và **không hề tra mã máy** dù đã hứa | backend | `internal/service/booking.go:118` | Bỏ dấu cho tên khách + thêm truy vấn con tra mã máy (không join để không hỏng Count). 1 test mới | FIXED |
| 45 | promotions | Mở ô chọn "Phần thưởng" khi thêm khuyến mãi | Danh sách hiện **nguyên khoá i18n**: `vnetPages.promotions.rewardTypes.discount_percent`. Nguyên nhân: khối `rewardTypes` bị **khai hai lần trong cùng một object** locale (dòng 1591 và 1617) — JavaScript lấy khối sau, xoá sạch `discount_percent` và `discount_amount`. Lỗi này im lặng tuyệt đối: file vẫn hợp lệ, build vẫn chạy | admin | 3 file `src/locales/langs/*.ts` | Gộp hai khối thành một ở cả 3 ngôn ngữ. Quét toàn bộ locale tìm khoá trùng: chỉ còn `orders.product` trùng nhưng **giá trị y hệt nhau** nên vô hại, để nguyên | FIXED |
| 46 | promotions | Ô tìm khuyến mãi | Chưa bỏ dấu — chỗ thứ 5 trong danh sách 6 chỗ ở #26 | backend | `internal/service/promotion.go:173` | `unaccent()` | FIXED |
| 47 | curfew | Vị thành niên mở phiên trong khung giờ cấm | Thông báo bằng **tiếng Anh**: `curfew is in effect: minors cannot start a session between 22:00:00 and 06:00:00`. Câu này **hiện thẳng lên màn hình khoá của khách** — một bạn 15 tuổi ở quán net Việt Nam — trong khi câu cùng loại ngay 20 dòng bên dưới (giới hạn giờ chơi) đã viết tiếng Việt. Còn kèm cả phần giây thừa | backend | `internal/service/curfew.go:323` | Dịch sang tiếng Việt và cắt giây: "đang trong khung giờ cấm: khách vị thành niên không được chơi từ 22:00 đến 06:00". Cập nhật test cũ | FIXED |
| 48 | curfew | Vào trang khi chưa có khung giờ nào | Console cảnh báo `vnetPages.curfew.days.undefined` — **đúng lỗi #15 ở trang khác**: el-table dựng ô mẫu với `row` rỗng. Nếu dữ liệu thiếu `day_of_week` thì ô sẽ in nguyên khoá thô | admin | `src/views/vnet/curfew/index.vue:15` | Chắn giá trị rỗng. Quét 6 chỗ tra khoá i18n tương tự: 4 chỗ đã an toàn, còn `website-blocking:261` sẽ kiểm khi audit tới trang đó | FIXED |
| 49 | notifications | Bấm **Gửi** trên một thông báo vừa bị xoá ở tab khác (máy chủ trả 500 `notification not found`) | **Không hiện gì cả** — không toast báo lỗi, không toast báo thành công. Quản trị viên bấm xong tưởng đã gửi tới toàn bộ hội viên trong khi chẳng ai nhận được. `handleDispatch` bắt lỗi bằng `catch (_) {}` rỗng, đúng cái khuôn mà `handleDelete` ngay bên trên đã được sửa và ghi chú | admin | `src/views/vnet/notifications/index.vue:145` | Bắt lỗi giống `handleDelete`: `'cancel'` là người dùng bấm Huỷ, còn lại hiện `ElMessage.error`. Kiểm lại đúng kịch bản cũ → nay hiện đỏ `notification not found` | FIXED |
| 50 | notifications | Bấm **Gửi** hai lần trên cùng một thông báo | **Nhân đôi hộp thư của mọi hội viên**: 12 hội viên → `member_notifications` nhảy 12 → 24 dòng cùng tiêu đề, `notification_recipients` 12 → 24. Không có gì chặn, giao diện cũng không đánh dấu "đã gửi", nên hai người trực cùng bấm là khách nhận hai lần một tin | backend | `internal/service/notification.go` → `notification_admin.go` Dispatch | Loại ngay trong câu truy vấn những hội viên đã có dòng trong sổ người nhận (`NOT EXISTS`). Người đăng ký sau lần gửi đầu vẫn nhận ở lần bấm sau. Kiểm thật: lần bấm thứ ba báo "Đã gửi thông báo tới **0** hội viên", CSDL đứng yên 24/36. 1 test mới, **đã chứng minh test fail khi gỡ chốt chặn** | FIXED |
| 51 | website-blocking | Tìm "zzzz" (không ra gì) → bấm dấu **x** xoá ô tìm | Ô tìm trống trở lại nhưng bảng **vẫn trống**, phải bấm Tìm kiếm lần nữa mới thấy lại luật. Người trực nhìn màn hình "Không có dữ liệu" sau khi xoá bộ lọc rất dễ tưởng toàn bộ luật đã bị xoá | admin | `src/views/vnet/website-blocking/index.vue:238` | Thêm `@clear="load"` cạnh `@keyup.enter="load"` sẵn có. Bấm lại đúng kịch bản: xoá ô tìm → danh sách quay lại ngay | FIXED |
| 52 | website-blocking | Thêm luật với **Danh mục** dài hơn 30 ký tự | Lỗi PostgreSQL thô lọt thẳng ra màn hình: `ERROR: value too long for type character varying(30) (SQLSTATE 22001)` — cùng loại với lỗi #4. Cùng lúc thử tên miền dài hơn 500 ký tự: `character varying(500)` cũng lọt y như vậy | backend | `internal/service/website_block.go` CreateRuleRequest/UpdateRuleRequest + CreateRule/UpdateRule | Danh mục: `binding:"omitempty,max=30"` + nhãn tiếng Việt "Danh mục" trong `error.go` → "Danh mục tối đa 30 ký tự". Tên miền: kiểm độ dài **sau khi chuẩn hoá** (ô nhập nhận cả URL dài, phần đường dẫn bị cắt — chặn theo chuỗi thô sẽ từ chối oan) → "tên miền quá dài (tối đa 500 ký tự)". 2 test mới, **đã chứng minh test fail khi gỡ chốt chặn** | FIXED |
| 53 | feedback | Gửi đánh giá 0 sao hoặc 9 sao (đường máy trạm dùng: `POST /api/feedback`) | Câu chặn **lẫn lộn Anh–Việt**: `rating tối thiểu là 1`, `rating tối đa là 5`. Câu này hiện trên màn hình khách khi họ chấm điểm | backend | `internal/handler/error.go` bảng `validationFieldLabels` | Thêm nhãn `"Rating": "Điểm đánh giá"` (bảng nhãn sinh ra đúng để làm việc này) → "Điểm đánh giá tối thiểu là 1". Kiểm lại bằng chính hai lệnh gọi cũ | FIXED |
| 54 | reports | Chọn khoảng ngày **13/09 → 13/09** ở tab Doanh thu ngày (ngày mà bảng không lọc đang hiện 216.349 ₫ / 11 đơn) | Bảng thành **"Không có dữ liệu"**. Toàn bộ doanh thu của ngày biến mất chỉ vì đúng ngày đó được chọn. Nguyên nhân: 16 chỗ trong `report.go` dùng `time.Parse` — nửa đêm **UTC**, tức 07:00 giờ ta — nên mốc đầu cắt mất ca đêm 00:00–07:00 (giờ đông khách nhất của quán net) và mốc cuối `t+24h` lại cộng nhầm 7 tiếng đầu của ngày kế tiếp. Đây là **cùng một lỗi đã sửa cho trang Giao dịch**, còn sót ở toàn bộ 6 hàm báo cáo | backend | `internal/service/report.go` (6 hàm, 16 chỗ) | Chuyển hết sang `time.ParseInLocation` + `utils.StartOfDay/EndOfDay`, đúng khuôn `ListTransactions` ngay trong cùng file. Kiểm lại trên giao diện: 13/09 → 13/09 hiện lại 216.349 ₫ / 11 đơn; 12/09 → 12/09 vẫn rỗng đúng. 1 test mới, **đã chứng minh test fail khi trả lại cách cũ** | FIXED |
| 55 | reports | Tab **Món bán chạy** cùng khoảng ngày với các tab khác | "Mì tôm — 1027 phần — **15.405.000 ₫**" trong khi doanh thu thật cả ngày là 216.349 ₫ và tab Theo nhân viên chỉ ghi nhận 20.000 ₫. Báo cáo tự mâu thuẫn ngay trên cùng một màn hình. Nguyên nhân: `TopProducts` đếm `order_items` **không lọc trạng thái đơn** — một đơn nháp 999 gói mì và 4 đơn **đã huỷ** vẫn được tính. Mọi báo cáo doanh thu khác trong file đều lọc `status = 'completed'` | backend | `internal/service/report.go` TopProducts | Nối sang `orders` và lọc `status = 'completed'`. Sau sửa: "Nước suối — 2 — 20.000 ₫", khớp đúng đơn hoàn tất duy nhất và khớp tab Theo nhân viên. Cập nhật test cũ (assert SQL chính xác) có chủ đích | FIXED |
| 56 | backups | Bấm **Tạo sao lưu** → OK ở hộp xác nhận | **Không có gì xảy ra**: không toast, không dòng mới, bảng vẫn trống. Tính năng sao lưu **chưa từng chạy được từ giao diện**. Giao diện gửi `POST /api/backups` không kèm thân, handler lại bắt buộc có thân JSON nên trả 400 `Dữ liệu không hợp lệ: EOF` — trong khi cả hai trường của `CreateBackupRequest` đều không bắt buộc | backend | `internal/handler/backup.go` Create | Coi thân request là tuỳ chọn: bỏ qua `io.EOF`, giữ nguyên mọi lỗi giải mã khác. Swagger đổi `true` → `false`. Bấm lại: dòng mới xuất hiện ngay | FIXED |
| 57 | backups | Cùng thao tác trên | Lỗi 400 đó **bị nuốt hoàn toàn** — `handleCreateBackup` và `handleRestore` bắt bằng `catch (_) {}`, đúng khuôn lỗi #49. Chính nó làm lỗi #56 ẩn mình: người trực bấm xong tưởng bản sao lưu đang chạy | admin | `src/views/vnet/backups/index.vue:92,112` | Bắt lỗi giống `handleDelete` ngay bên dưới. Kiểm lại đường phục hồi: nay hiện đỏ "không tìm thấy tệp sao lưu: backups/…" thay vì im lặng | FIXED |
| 58 | backups | Sao lưu hỏng vì máy chủ thiếu `pg_dump` | Bảng ghi "Thất bại" **không kèm lý do ở bất cứ đâu** — cột ghi chú rỗng, giao diện không hiện notes. Lỗi không đến từ chính pg_dump (thiếu lệnh, thiếu quyền, đầy đĩa) thì `CombinedOutput` trả chuỗi rỗng, bản cũ ghi đúng chuỗi rỗng đó vào `notes` | backend + admin | `internal/service/backup.go` runPgDump · `views/vnet/backups/index.vue:67` | Backend: rỗng thì ghi `err.Error()`. Admin: gắn lý do vào `title` của thẻ trạng thái (cùng cách trang Cập nhật máy khách làm với băm). Kiểm lại: rê chuột vào "Thất bại" hiện `exec: "pg_dump": executable file not found in $PATH` | FIXED |
| 59 | backups | Xoá bản sao lưu đang chạy | Câu chặn bằng **tiếng Anh**: `cannot delete a backup that is still running`, cùng file với `backup not found` và `backup file not found` — trong khi hai câu ngay cạnh đã viết tiếng Việt ("không tìm thấy tệp sao lưu", "phục hồi thất bại"). Cùng khuôn lỗi #47 | backend | `internal/service/backup.go` (4 câu) | Dịch cả 4 câu, cập nhật 2 test cũ có chủ đích | FIXED |
| 60 | audit | Chọn khoảng ngày **13/09 → 13/09** (nhật ký đang có 273 dòng, tất cả của hôm nay) | Bảng trống trơn. **Cùng lỗi múi giờ #54**, còn sót ở `audit.go` | backend | `internal/service/audit.go` List | `time.ParseInLocation` + `utils.StartOfDay/EndOfDay`. Kiểm lại trên giao diện: 273 dòng hiện lại. 1 test mới, **đã chứng minh fail khi trả lại cách cũ**. Đã quét cả repo: chỉ còn `feedback.go parseDateOnly` cùng khuôn nhưng giao diện Đánh giá **không có ô lọc ngày** nên chưa chạm tới được — ghi vào mục theo dõi | FIXED |
| 61 | audit | Mở ô lọc **Đối tượng**, chọn "session" / "booking"; ô **Hành động** chọn "login", "logout", "export" | Luôn rỗng — 5 lựa chọn chết. Bảng ghi dùng `machine_session`, `machine_booking`; còn `login`/`logout`/`export` **không có một dòng nào trong mã nguồn** (đăng nhập không ghi nhật ký) | admin | `src/views/vnet/audit/index.vue:17-30` | Đối chiếu với danh sách hành động/đối tượng mà backend thật sự ghi (grep `Action:`/`EntityType:`): sửa `session`→`machine_session`, `booking`→`machine_booking`, bỏ 3 hành động không tồn tại, thêm `update_status`/`topup`/`refund`. Kiểm lại: `machine_session` ra 27 dòng | FIXED |
| 62 | audit | Xem cột **Mô tả** | 23 kiểu việc hiện **nguyên tên hàm tiếng Anh** giữa câu tiếng Việt: "publish_app_update app_update", "reset_password hội viên", "dispatch notification"… Riêng phiên chơi thì lặp từ: "Bắt đầu phiên chơi **phiên chơi**" | backend | `internal/service/audit.go` (2 bảng nhãn + nhánh start_session) | Thêm 27 nhãn hành động và 9 nhãn đối tượng còn thiếu — nhãn hành động chỉ mang **động từ** vì câu được ghép theo khuôn "&lt;hành động&gt; &lt;đối tượng&gt;". Bỏ phần ghép thừa ở `start_session`. Kiểm lại bằng việc làm mới: "Đặt lại mật khẩu hội viên", "Xoá bản cập nhật máy khách" | FIXED |
| 63 | settings | Mở tab **Giới hạn** đã có số | Console cảnh báo 4 lần mỗi lần mở tab: `Invalid prop: type check failed for prop "modelValue". Expected Number | Null, got String with value "5"`. Cột `jsonb` trả giá trị về dạng **chuỗi** (chính chú thích trong `settings.go` đã nói), mà `ElInputNumber` đòi số | admin | `src/views/vnet/settings/index.vue` fetchSettings | Thêm `coerceNumbers()` cho nhóm `limits`, cùng khuôn `parsePresets()` sẵn có ở tab Nạp tiền | FIXED |
| 64 | settings | Mở trang lần đầu (đang ở tab Chung) | Console cảnh báo `[ElSwitch] model-value must be active-value or inactive-value`. `ElTabs` dựng sẵn **mọi** pane nên hai công tắc của tab Tính năng tồn tại ngay cả khi chưa nạp khoá, mà `applyFeatureDefaults()` chỉ chạy khi đang ở tab đó | admin | `src/views/vnet/settings/index.vue` (2 ElSwitch) | Cho công tắc giá trị mặc định `'true'` ngay tại chỗ bind (khớp mặc định BẬT của backend) thay vì nới phạm vi `applyFeatureDefaults` — nới ra sẽ khiến bấm Lưu ở tab Chung ghi nhầm khoá `attendance_enabled` sang nhóm `general` | FIXED |
| 65 | settings | Mở tab **Hóa đơn** khi chưa từng lưu | Lỗi đỏ 404 trong console mỗi lần mở một tab chưa dùng tới. Backend coi "nhóm chưa có dòng nào" là **lỗi không tìm thấy**, trong khi đó là trạng thái lần đầu bình thường — chính giao diện cũng đã phải viết một nhánh riêng để nuốt nó | backend | `internal/service/settings.go` GetByGroup | Trả danh sách rỗng + 200, giống mọi endpoint danh sách khác. Cập nhật test cũ có chủ đích | FIXED |
| 66 | printers | Thêm máy in, ô **Địa chỉ IP** gõ `không-phải-ip` (hoặc `192.168.1.999`) | Lưu thành công. Địa chỉ rác nằm im trong cấu hình cho tới lúc bấm In thử — hoặc tệ hơn, tới lúc **bếp không nhận được phiếu giữa giờ cao điểm** | backend | `internal/service/printer.go` CreatePrinterRequest/UpdatePrinterRequest | `binding:"omitempty,ip|hostname"` — vẫn nhận tên máy trong mạng LAN (`printer-bar.local`) nhưng chặn chuỗi rác. Kiểm lại trên giao diện: `192.168.1.999` → "Địa chỉ IP không hợp lệ", không tạo dòng nào | FIXED |
| 67 | printers | Bấm **In thử** vào máy in không tồn tại | Câu lỗi **tiếng Anh**: `cannot connect to printer at …: no such host`, cùng `printer not found`, `printer has no IP address configured`, `failed to send test data` — trong khi nửa dưới của chính file đã viết tiếng Việt ("không tìm thấy máy in", "có mã sản phẩm không tồn tại"). Cùng khuôn lỗi #47/#59 | backend | `internal/service/printer.go` (4 câu) | Dịch cả 4, cập nhật test cũ có chủ đích | FIXED |
| 68 | system/user | Chọn một dòng → **Xóa hàng loạt** → Có | Lỗi PostgreSQL thô: `invalid input syntax for type uuid: "[object Object]" (SQLSTATE 22P02)`, không xoá được gì. `@selection-change="checkedRowKeys = $event"` nhận **mảng đối tượng dòng** của Element Plus, trong khi `checkedRowKeys` được khai là `string[]` và `.map(String)` biến mỗi dòng thành `"[object Object]"`. Trang Đơn hàng của VNET đã làm đúng (`$event.map(r => r.id)`) | admin | `src/views/system/user/index.vue:184` | Ánh xạ sang `id` đúng như trang Đơn hàng. Bấm lại: "Xóa thành công", CSDL xoá mềm đúng người. **`pnpm typecheck` đã cảnh báo đúng lỗi này từ trước** (`Type 'User[]' is not assignable to type 'string[]'`) — số lỗi typecheck giảm 51 → 50 | FIXED |
| 69 | system/user | Tìm "thungan" → 1 kết quả → bấm **Đặt lại** | Ô tìm trắng trở lại nhưng bảng **vẫn 1 dòng**: `resetSearchParams()` chỉ xoá tham số, không gọi lại API. Người trực nhìn màn hình tưởng danh sách nhân viên chỉ còn một người. Cùng khuôn lỗi #51 | admin | `src/views/system/user/index.vue:147` | Gọi `getDataByPage()` sau khi đặt lại. Kiểm lại: lọc còn 1 → Đặt lại → đủ 4 dòng | FIXED |
| 70 | system/user | Thêm người dùng mới, lưu xong | Toast báo **"Cập nhật thành công"** cho một thao tác **thêm mới** | admin | `src/views/system/user/modules/user-operate-drawer.vue:134` | Chọn khoá theo `isEdit` — `common.addSuccess` ("Thêm thành công") đã có sẵn trong cả 3 locale | FIXED |
| 71 | auth (phát hiện khi kiểm khoá tài khoản) | Khoá tài khoản ở trang Người dùng rồi đăng nhập lại | Câu chặn **tiếng Anh** hiện trên màn hình đăng nhập: `account is disabled`, `invalid username or password`, `invalid or expired refresh token`… trong khi cùng file đã có câu tiếng Việt ("mật khẩu hiện tại không đúng"). Đây là những câu **được đọc nhiều nhất toàn sản phẩm** | backend | `internal/service/auth.go` (7 câu) | Dịch cả 7, cập nhật 6 test cũ có chủ đích. Kiểm lại: "tài khoản đã bị khoá — liên hệ quản lý", "sai tên đăng nhập hoặc mật khẩu" | FIXED |
| 72 | system/role | Chọn một dòng → **Xóa hàng loạt** → Có | Y hệt lỗi #68: `invalid input syntax for type uuid: "[object Object]"`. Đúng dòng mà `pnpm typecheck` đã chỉ ra ở vòng trước (`Type 'Role[]' is not assignable to type 'string[]'`) | admin | `src/views/system/role/index.vue:146` | Ánh xạ sang `id`. Bấm lại: "Xóa thành công". Typecheck 50 → 49 lỗi | FIXED |
| 73 | system/role | Lọc "Thu ngân" → 1 kết quả → **Đặt lại** | Bảng vẫn 1 dòng — cùng khuôn #69 | admin | `src/views/system/role/index.vue:113` | Gọi `getDataByPage()` sau khi đặt lại. Kiểm lại: 1 → 4 dòng | FIXED |
| 74 | system/role | Thêm vai trò mới | Toast "Cập nhật thành công" cho thao tác thêm — cùng khuôn #70 | admin | `src/views/system/role/modules/role-operate-drawer.vue:96` | Chọn khoá theo `isEdit` | FIXED |
| 75 | machines | Xem cột **Nhóm** ở trang Máy | Mọi máy đều hiện `-`, kể cả PC-03 đang thuộc nhóm VIP 15.000₫/h. API danh sách máy trả `group_id` nhưng **không kèm object `group`**, trong khi cột đọc `row.group?.name` — cột này chưa bao giờ hiện tên nhóm. Phát hiện khi sửa #18: nhãn cảnh báo mới bám theo `row.group` nên lúc đầu báo nhầm cả máy đã có nhóm | admin | `src/views/vnet/machines/index.vue:356` | Tra tên nhóm từ danh sách `groups` mà trang đã nạp sẵn (`fetchGroups`), không phụ thuộc object thiếu. Kiểm lại: PC-03 hiện **VIP**, 4 máy còn lại hiện nhãn vàng "Chưa gán nhóm — 0₫/giờ" | FIXED |
| 76 | system/menu | Bấm **sang trang 2** hoặc đổi số dòng/trang | Bảng nạp lại **đúng 20 dòng đầu**: lệnh gọi `fetchGetMenuList()` không gửi `current`/`size` nên backend luôn trả trang 1. Thanh phân trang ghi "Tổng 35" mà **15 mục cuối không có đường nào xem được** | admin | `src/service/api/system-manage.ts:34` + `src/views/system/menu/index.vue` | Truyền `current`/`size` và nối `onPaginationParamsChange`, cùng khuôn hai trang Người dùng/Vai trò. Kiểm lại: trang 2 bắt đầu từ `vnet_notifications` (15 dòng), đổi 30/trang ra 30 dòng | FIXED |
| 77 | system/menu | Đổi số dòng/trang lên 30 rồi xem console | 13 cảnh báo `Duplicate keys found during update: "vnet_notifications"…`. `GetMenuList` vừa đưa mỗi mục con vào danh sách phẳng, **vừa** gắn nó vào `Children` của mục cha, nên el-table trải thêm một lần nữa: 33/35 mục hiện **hai lần** trên cùng một trang | backend | `internal/service/system_manage.go` GetMenuList | Danh sách này vốn phẳng (đã có cột `parentId`, và cây menu có endpoint riêng `GetMenuTree`) — bỏ phần gắn `Children`. Kiểm lại: API trả 35 bản ghi, **0 nút con lồng**, console sạch. 1 test mới, **đã chứng minh test fail khi trả lại cách cũ** | FIXED |
| 78 | đổi mật khẩu (toàn hệ thống) | Đổi mật khẩu từ menu avatar rồi mở **Nhật ký hoạt động** | **Không có dòng nào**. Việc đổi mật khẩu — thao tác nhạy cảm nhất của một tài khoản — không để lại dấu vết. Log máy chủ có `[audit] không ghi được nhật ký action=change_password: invalid input syntax for type json (SQLSTATE 22P02)`: cột `metadata` là `jsonb` mà chỗ gọi không kèm metadata ghi vào **chuỗi rỗng**, PostgreSQL từ chối cả dòng. Đúng 2 chỗ gọi trong toàn bộ mã nguồn bị dính, và cả hai đều là đổi mật khẩu (nhân viên + hội viên) | backend | `internal/service/audit.go` Log | Thiếu metadata thì ghi `{}` thay vì chuỗi rỗng. Kiểm lại: đổi mật khẩu hai lần → hai dòng `change_password` hiện trên trang Nhật ký kèm tên người thực hiện. 1 test mới, **đã chứng minh test fail khi trả lại cách cũ** | FIXED |
| 79 | reports / dashboard | Chạy trọn một luồng thật rồi so "Doanh thu hôm nay" với tiền thật | Con số **582.849₫** của hôm nay = nạp ví 639.000 **+** đơn hàng 60.000 **−** gói cước 100.000 **−** tiền giờ 14.151 **−** hoàn tiền 2.000. Hai chỗ sai nhìn thấy được: (a) **bán gói cước và thu tiền giờ LÀM GIẢM doanh thu** — chúng là khoản trừ ví nên bị cộng nguyên dấu âm; bán gói 100.000₫ kéo doanh thu xuống 100.000₫; (b) **nạp tiền và đơn trả bằng số dư cộng hai lần** cùng một đồng tiền. Dashboard dùng đúng endpoint này nên hiện cùng con số | backend | `internal/service/report.go` DailyRevenue + txRevenueQuery | **Đã sửa theo quyết định của anh: ghi nhận doanh thu lúc TIỀN VÀO QUÁN, không tính lại khi khách tiêu từ ví.** Sổ ví chỉ còn cộng `topup` + `topup_card` + gói cước trả tiền mặt, trừ `refund`; bỏ `session_fee`, `order_payment`, gói cước trả bằng số dư, và mọi loại số dư tặng (`topup_bonus`, `attendance_bonus`, `lucky_spin_balance`). Bảng đơn hàng chỉ còn tính đơn trả **tiền mặt** và loại đơn `order_type='topup'` (đơn này sinh kèm một dòng ví `topup`, đếm cả hai là đếm hai lần). Kiểm trên dữ liệu thật: API trả **2.084.000₫**, cộng tay từng khoản cũng ra **2.084.000₫** (1.816.000 nạp + 250.000 thẻ nạp − 2.000 hoàn tiền + 20.000 đơn tiền mặt), Bảng điều khiển hiện đúng con số đó. Ngoặc bao vế `OR` là bắt buộc — thiếu nó thì `AND` bám chặt hơn và bộ lọc ngày chỉ áp cho vế cuối; có test khoá riêng chỗ này | FIXED |
| 80 | phân quyền (toàn hệ thống) | Đăng nhập bằng `staff` rồi gọi `/api/systemManage/addUser` | **Nhân viên quầy tạo được tài khoản mang vai trò `owner`** — tự nâng mình lên toàn quyền. Ngoài ra staff đọc được danh sách người dùng, sao lưu, nhật ký hoạt động, trong khi **manager bị 403 ở cả ba**. Nguyên nhân: seed cấp `client.admin` cho vai trò staff, mà đó chính là quyền gác back-office; chú thích ngay trên trong seed nói rõ ý định ngược lại ("Manager runs day-to-day business but not the back-office tooling") | backend | `cmd/seed/main.go:158` + dữ liệu đang chạy | Bỏ `client.admin` khỏi danh sách quyền của staff trong seed, thêm migration `revoke_client_admin_from_staff` thu hồi trên hệ thống đã cài. Kiểm lại: staff gọi 4 nhóm back-office đều 403, tạo tài khoản owner → 403 "Insufficient permissions", còn xem máy/hội viên/đơn vẫn 200. Đã xoá tài khoản owner mà tôi tạo ra lúc dựng lại lỗi | FIXED |
| 81 | toàn bộ 26 trang VNET | Để access token hết hạn (24 giờ) rồi bấm sang trang khác | **Văng thẳng ra màn hình đăng nhập giữa ca**, mất thao tác đang làm dở — dù refresh token còn hạn 7 ngày. `src/api/client.ts` coi MỌI 401 là "đăng xuất ngay": xoá token rồi `location.href='/login'`, không hề gọi `/auth/refresh`. Client Soybean (`src/service/request`) làm đúng giao kèo 9999 từ lâu, nhưng 26 trang VNET lại dùng client kia | admin | `src/api/client.ts:45` | Mã 9999 → gọi `/auth/refresh` (một lần dùng chung cho mọi request đang chờ) rồi gửi lại đúng một lần; thất bại hoặc mã khác → đăng xuất như cũ. Kiểm lại trên hệ thống thật: log máy chủ hiện `401 → POST /auth/refresh 200 → GET /members 200`, trang vẫn đứng nguyên với đủ 10 dòng | FIXED |
| 82 | phân quyền (yêu cầu của anh) | Mở màn hình **Phân quyền** của một vai trò | Chỉ có **11 quyền** để tick, và **5 trong số đó không gác gì** (`members.view`, `machines.view`, `orders.view`, `orders.create`, `orders.pay`). ~25 nhóm tính năng không có quyền nào: sản phẩm, giá, khuyến mãi, kho, máy in, chặn web… — nên nhân viên quầy đổi được giá bán, sửa khuyến mãi, xoá nhóm máy | backend | `internal/router/router.go` · `cmd/seed` · `internal/service/route.go` | Dựng danh mục **140 quyền** ở `internal/authz` (một nguồn sự thật), gắn quyền riêng cho **187/189 API nhân viên** (2 API còn lại là menu + danh sách quyền của chính mình, gác thì không ai đăng nhập được), menu cũng theo quyền xem của từng nhóm. Migration bơm quyền mới + cấp mặc định cho manager/staff, chỉ thêm không gỡ. 4 test chặn hồi quy (mã sai danh mục, route thiếu quyền, vai trò vượt ranh) | FIXED |
| 83 | toàn admin (i18n) | Đổi ngôn ngữ sang **English** hoặc **中文** | Tên sản phẩm đổi thành **"SoybeanAdmin"** / **"Soybean 管理系统"** — hiện ở thanh bên, màn hình chờ và **cả màn hình đăng nhập**. Chỉ bản tiếng Việt được đặt tên đúng | admin | `src/locales/langs/en-us.ts:4` · `zh-cn.ts:4` | Đặt "VNET Admin" / "VNET 管理系统". Kiểm lại: đổi qua 3 ngôn ngữ, tên luôn là VNET | FIXED |
| 84 | feedback · audit · orders (chế độ tối) | Bật chế độ tối rồi mở các trang đó | Màu viết cứng không theo chủ đề: (a) **điểm trung bình** ở trang Đánh giá là `#303133` trên thẻ nền tối `rgb(29,30,31)` — con số lớn nhất trang **biến mất**; (b) khối JSON ở Nhật ký có nền `#f5f7fa` sáng trong khi chữ theo chủ đề tối → gần như không đọc được (tương phản ~1,25:1); (c) khối xem trước hoá đơn ở Đơn hàng cùng lỗi | admin | `feedback/index.vue:93,111` · `audit/index.vue:147` · `orders/index.vue:798` | Dùng biến của Element Plus: `var(--el-text-color-primary)`, `var(--el-fill-color)`, `var(--el-fill-color-light)`. Kiểm cả hai chế độ: tối → chữ `rgb(229,234,243)` trên nền `rgb(29,30,31)`; sáng → `rgb(48,49,51)` trên nền trắng | FIXED |
| 85 | sessions (tính tiền) | Mở phiên ở nhóm giá 20.000₫/h → để chạy 2 phút → đổi giá nhóm sang 80.000₫/h (mô phỏng bước vào khung giờ cao điểm) → chờ lượt trừ tiền kế tiếp | **Tính lại tiền cho cả thời gian đã chơi theo giá MỚI.** Đo thật: phút 1–2 ở giá 20.000₫ đã trừ **334₫**; đổi giá xong, lượt kế tiếp nâng tổng lên **2.667₫** = 2 phút × 80.000₫/h. Đúng ra phải là 334₫ + 1.334₫ ≈ **1.668₫** → khách bị thu dư **~60%**. Cùng lỗi ở `EndSessionAt`: trả máy lúc nào thì cả phiên bị áp giá lúc đó | backend | `internal/service/session.go` `ChargeTick`→`CalculateCost(…, tổng số phút)` và `EndSessionAt` cùng kiểu | **Đã sửa**: `CalculateCost` nay nhận mốc bắt đầu và cộng tiền **từng phút**, mỗi phút giữ giá của khung giờ nó thuộc về (`tinhTienTheoDoan` + `giaTaiThoiDiem`). Làm tròn lên vẫn áp ở TỔNG nên phiên không vắt mốc ra đúng con số cũ. `bangGiaKhungGio` bỏ lọc `day_of_week` ở SQL vì phiên vắt nửa đêm nằm trên hai thứ khác nhau. 3 test mới. **Kiểm thật đầu-cuối**: khung cao điểm 80.000₫ đặt ngay phút kế tiếp, phiên mở trước đó ở giá 20.000₫ → phút 4 thu **3.334₫** (2×20.000 + 2×80.000) thay vì 5.334₫ của bản cũ; phút 5 → 4.667₫; phút 6 → 6.000₫; trả máy chốt 6.000₫. Đơn giá hiển thị nhảy sang 80.000 đúng lúc qua mốc | FIXED |
| 86 | members (nạp tiền) | `POST /api/members/:id/topup` với `payment_method: "con-meo-cua-toi"` | Nhận tuốt, ghi thẳng vào sổ: 77.000₫ với hình thức thanh toán bịa. Giao diện chỉ cho chọn `cash`/`transfer`/`ewallet`, nhưng API không kiểm. Hậu quả thật nằm ở **chốt ca**: `ShiftService` chỉ cộng `payment_method = 'cash'`, nên một giá trị gõ sai làm khoản tiền mặt đó biến mất khỏi số tiền phải có trong két — két thừa mà không ai biết vì sao | backend | `internal/service/member.go` `TopupRequest.PaymentMethod` (không có thẻ `binding`) | **Đã sửa**: `binding:"required,oneof=cash transfer ewallet bonus_balance"`. Kiểm thật: `con-meo-cua-toi` → 400 "Phương thức thanh toán không hợp lệ"; bỏ trống → 400 "...không được để trống"; `transfer` → thành công | FIXED |
| 87 | topup-cards | Bán một thẻ nạp cho khách (`POST /topup-cards/:id/sell`) rồi tìm khoản tiền đó trong báo cáo và trong phần chốt ca | **Tiền mặt khách trả để mua thẻ không được ghi ở đâu cả.** `SellTopupCard` chỉ cập nhật `sold_to`/`sold_at` rồi ghi nhật ký; không sinh dòng ví, không sinh đơn. Nên tiền bán thẻ không vào báo cáo doanh thu và không vào số tiền phải có trong két lúc chốt ca | backend | `internal/service/card.go:480` `SellTopupCard` | **Đã sửa**: thêm cột `sold_payment_method`, `SellTopupCardRequest` bắt buộc `payment_method`, giao diện Bán thẻ có ô chọn (mặc định Tiền mặt, đủ 3 ngôn ngữ). Doanh thu nay đếm thẻ ở **lúc bán** (`thuTienBanThe`, theo `sold_at`, bỏ thẻ đã huỷ) và **bỏ hẳn** `topup_card` khỏi sổ ví để không đếm hai lần. Kiểm thật: bán thẻ 50.000₫ → doanh thu 2.434.000 → **2.484.000**, đúng bằng mệnh giá; cộng tay từng khoản trong CSDL cũng ra 2.484.000 | FIXED |

## Nhật ký vòng

### Vòng 1 — dựng môi trường + dashboard
- Dựng Postgres container, migrate (19 bước OK), seed OK, backend `:8080` OK, admin `:3000` OK.
- Đăng nhập `admin/admin123` OK; menu động `GET /api/route/getUserRoutes` 200; WebSocket kết nối OK.
- Dashboard: 6 request API đều 200, console sạch, reload giữ nguyên phiên.
- Số liệu dashboard đang là 0 vì cơ sở dữ liệu trống → **PARTIAL**, phải kiểm lại ở X2 khi đã có dữ liệu thật.
- Chưa phát hiện lỗi.

### Vòng 2 — members (tạo bản ghi) + phát hiện nhiễu môi trường
- Thêm hội viên: validate rỗng đúng (2 thông báo bắt buộc), tạo thành công với tiếng Việt có dấu, form reset sạch khi mở lại.
- Phát hiện "bảng không cập nhật sau khi tạo" — **truy đến cùng thì không phải lỗi app**: extension trong profile Chrome mặc định phục vụ lại response cũ, bỏ qua cả `no-store`. Bằng chứng: cùng lúc đó `curl` và context cô lập đều trả dữ liệu đúng; trong context cô lập dòng mới hiện sau 251 ms.
- Đã thêm `middleware.NoCache()` như một bước gia cố thật sự cần (API không hề có header hạn dùng), kèm test; không tính là sửa lỗi #1.
- Phát hiện mới cần quyết định: danh sách sắp theo UUID ngẫu nhiên (lỗi #3).
- **Dữ liệu test hiện có**: 5 hội viên (nguyenvana, tranthib, levanc, phamthid, hoangvane) — giữ lại để dùng cho các luồng sau.
- Còn dở ở trang members: phân trang, tìm kiếm, Sửa, Xoá, Chi tiết, Reset MK, Nạp, Hoàn tiền, Xếp lại hạng, Cài đặt cột.

### Vòng 3 — members (hoàn tất) → PASS
- Bấm đủ: phân trang (13 = 10+3, tham số `page`/`page_size` đúng, không trùng dòng), đổi 10→20/trang, tìm kiếm (có dấu / không dấu / SĐT / rỗng / xoá từ khoá), Thêm, Sửa (form điền sẵn đúng giá trị cũ), Xoá (Hủy không xoá, OK mới xoá, soft delete đúng), Chi tiết (3 tab, lịch sử giao dịch khớp số dư), Nạp, Hoàn tiền, Reset MK, Xếp lại hạng, Cài đặt cột.
- Kiểm biên: nạp rỗng → chặn; nạp 0 và âm → giao diện kẹp về 1000; gọi thẳng API với `amount<=0` → backend trả 400 "amount must be positive" ✅. Hoàn quá số dư → 400 `insufficient balance`, hộp thoại giữ nguyên ✅.
- Reset MK kiểm chứng thật: mật khẩu cũ bị từ chối, mật khẩu mới qua được lớp xác thực.
- 3 lỗi đã sửa (#4 tên index, #5 tìm không dấu, #6 pagerCount), 2 lỗi mới cần quyết định (#7 typecheck, #8 thông báo tiếng Anh).
- `go test ./... -count=1`: **xanh toàn bộ** (12 package). Console trang members: **sạch**.
- Dữ liệu còn lại: 12 hội viên (đã xoá mềm khach8).

### Vòng 4 — member-groups → PASS
- Điều hướng bằng **bấm menu** qua lại 3 lần: không có lỗi trang trắng (bẫy fragment root của AGENTS.md).
- Bấm đủ: Thêm (validate rỗng chặn đúng, không gửi request), Sửa (form điền sẵn đúng), Xoá, Làm mới, Cài đặt cột, Tìm kiếm.
- Nghiệp vụ kiểm được: bật "Mặc định" cho nhóm mới **tự gỡ** cờ ở nhóm cũ ✅; xoá nhóm còn 12 hội viên → 409 kèm câu chỉ việc cần làm ✅; xoá chính nhóm mặc định → 409 ✅. Phần này code viết tốt.
- 2 lỗi mới đã sửa: #9 ô tìm kiếm là nút chết, #10 backend không chặn giảm giá ngoài khoảng.
- `go test ./... -count=1`: **xanh toàn bộ**. Console chỉ có lỗi WebSocket do tôi khởi động lại backend và 2 request 409 do tôi cố tình thử.
- Trạng thái dữ liệu đã khôi phục: 3 nhóm gốc (Đồng mặc định), 12 hội viên.

### Vòng 5 — attendance → PASS (không phát hiện lỗi app)
- Cấu hình thưởng: đặt 5.000/ngày, mốc chuỗi 3 ngày, thưởng mốc 20.000 → Lưu → tải lại trang vẫn giữ ✅.
- Nghiệp vụ điểm danh kiểm bằng API (nhân viên điểm danh hộ hội viên):
  - Lần 1 trong ngày → `streak_days:1`, thưởng 5.000, cộng vào **bonus_balance** (không phải balance) ✅
  - Lần 2 cùng ngày → **409 "hôm nay đã điểm danh rồi"**, không cộng thêm đồng nào ✅
  - Dựng sẵn 2 ngày trước rồi điểm danh → `streak_days:3`, `streak_reached:true`, thưởng 5.000+20.000 = 25.000 ✅
  - Giao dịch ghi `attendance_bonus` với `balance_before = balance_after` và chỉ `bonus` đổi ✅
  - Tắt công tắc `attendance_enabled` → chặn ngay ở backend ("quán đang tắt tính năng điểm danh"), không chỉ ẩn nút ✅
- Bảng + bộ lọc ngày + nút xoá bộ lọc: đúng. Console **sạch**.
- **Hai báo động giả do cách đo của tôi, đã ghi vào prompt để không lặp lại:**
  1. "Một lần bấm Lưu gửi 2 request" — do hook XHR bị cài chồng qua các vòng, không phải lỗi app.
  2. "Bộ lọc ngày là nút chết" — cú click tổng hợp không kích hoạt `@change` của Element Plus; bấm bằng công cụ thật thì lọc đúng (Tổng 1).
- Dữ liệu để lại: 3 lượt điểm danh của nguyenvana (chuỗi 1→3), bonus_balance 55.000.

### Vòng 6 — machines → PASS (soi kỹ tính năng "tạo hàng loạt" đang làm dở)
- **Tính năng mới `BatchCreateMachines` (chưa commit) kiểm rất kỹ, chất lượng tốt:**
  - Tạo lô PC-01…PC-05 từ giao diện: đúng 5 máy, xem trước khớp ✅
  - Tạo lại đúng dải → **409, không tạo máy nào** (toàn bộ hoặc không có gì) ✅
  - Chồng một phần (3..8) → 409, liệt kê đúng 3 mã vướng ✅
  - Số cuối < số đầu → 400 ✅ · quá 500 máy → 400 ✅ · tiền tố rỗng → 400 ✅ · tiền tố dài quá 20 ký tự → 400 kèm mã ví dụ ✅
  - **Nhánh "mã thuộc máy đã xoá" tách riêng khỏi "mã đã có máy"** — xoá mềm ZZ-02 rồi tạo lại: báo đúng hai loại, đây là chi tiết dễ bỏ sót nhất và code xử lý đúng ✅
- Nhịp tim máy trạm: `offline → available`, ghi đủ nhiệt độ / IP / MAC / CPU / GPU / RAM; mã máy lạ → 404 ✅
- Điều khiển từ xa khi máy chưa kết nối: API 409, giao diện hiện cảnh báo vàng "Máy PC-01 chưa kết nối — lệnh không tới nơi" ✅. Hộp thoại Khóa có hỏi lý do và nói rõ khách sẽ thấy dòng đó.
- Xoá máy: hỏi xác nhận, xoá mềm, bảng cập nhật ✅
- 1 lỗi mới đã sửa: #11 `group_id` rỗng làm lộ lỗi PostgreSQL thô (cả đường tạo đơn và sửa vốn đã hỏng từ trước, không chỉ tính năng mới).
- `go test ./... -count=1`: **xanh toàn bộ**. Console chỉ có lỗi WebSocket do khởi động lại backend và 3 request 409 do tôi cố tình thử.
- Còn dở ở trang này: nút **Phần cứng** (lịch sử phần cứng) chưa bấm — để vòng sau.
- Dữ liệu để lại: 5 máy PC-01…PC-05 (PC-01 đang "Sẵn sàng") để dùng cho luồng phiên chơi.

### Vòng 7 — nốt machines (Phần cứng) + machine-groups → PASS
- **Phần cứng** (machines): hộp thoại "Nhật ký phần cứng — PC-01" hiện đúng số đo từ nhịp tim (55.5°C / 48.2°C / CPU 33% / RAM 61% / đĩa 45% / đã bật 1g 0p), có câu giải thích khi bảng rỗng ✅. Xác nhận bằng danh sách network thật: **1 request**, bản ghi đôi trong hook của tôi là artefact.
- **machine-groups**: Thêm (validate rỗng chặn, không gửi request) ✅ · Sửa ✅ · Xoá ✅ · Làm mới ✅ · Cài đặt cột ✅ · Tìm kiếm (sau khi sửa) ✅
- **Bảng giá** — phần đáng giá nhất của trang: 2 tab (theo hạng hội viên / theo khung giờ), câu giải thích thứ tự ưu tiên, cột "Đang dùng" chỉ đúng dòng máy tính tiền đang lấy. Thêm dòng giá cho hạng Vàng để **ô "Không hạn" trống** → database ghi `effective_to = NULL` đúng, **không dính bẫy ngày rỗng** mà AGENTS.md nêu đích danh ✅
- Ràng buộc xoá nhóm: còn máy → 409 ✅; còn dòng bảng giá → 409 ✅ (đã sửa câu chỉ dẫn).
- 2 lỗi mới đã sửa (#12 ô tìm kiếm chết, #13 câu chỉ dẫn sai thao tác). `go test ./... -count=1`: **xanh toàn bộ**.
- Dữ liệu đã dọn: nhóm "Phòng máy lạnh" đã xoá, PC-02 gỡ khỏi nhóm. Còn lại 2 nhóm gốc + 1 dòng bảng giá VIP/Vàng (giữ để kiểm tính tiền ở vòng Phiên chơi).

### Vòng 8 — quét toàn bộ ô tìm kiếm + machine-assets → PASS
- **Quét chủ động**: sau 2 lần gặp cùng lỗi "ô tìm kiếm chết", tôi gọi thử 18 endpoint danh sách với từ khoá vô nghĩa và so số dòng. Bắt được **machines** cũng bỏ qua `search` (lỗi #14). Các endpoint còn lại đang rỗng dữ liệu nên chưa kết luận được — sẽ kiểm lại khi tới lượt từng trang.
- **machine-assets**: Thêm tài sản (validate "Chọn máy" ✅) · Kiểm tra (form điền sẵn đúng, đổi tình trạng → ghi "Kiểm lúc" ✅) · Xoá (hỏi xác nhận ✅) · lọc theo máy + xoá bộ lọc ✅
- 2 lỗi mới đã sửa (#14 tìm kiếm máy, #15 khoá i18n `undefined`). Console sau khi sửa: **sạch** cả khi bảng rỗng lẫn khi có dữ liệu.
- `go test ./... -count=1` xanh; `pnpm typecheck` không phát sinh lỗi mới ở file đã sửa.
- Dữ liệu để lại: 3 tài sản của PC-01 (màn hình / bàn phím / chuột), PC-03 đã gán nhóm VIP.

### Vòng 9 — sessions → PASS (2 lỗi nghiêm trọng đã sửa, 2 vấn đề chờ quyết định)
- **Chuỗi tính giá kiểm đủ ba tầng và đúng tuyệt đối**: giá cơ bản VIP 15.000₫/h → giá theo hạng Vàng 10.000₫/h (dòng bảng giá tạo ở vòng 7) → giảm 10% theo hạng = **9.000₫/h**. Nút "Tính thử" trên giao diện và endpoint `calculate-cost` cho cùng con số.
- **Tính tiền cuối phiên đúng**: phiên 30 phút → `total_cost = 4.500₫` = 30 phút × 9.000₫/h; trừ vào **bonus_balance** (55.000 → 50.500) chứ không đụng tiền thật; `total_played_minutes` +30; giao dịch `session_fee -4500` ghi đủ.
- **Lỗi #16 là lỗi nặng nhất từ đầu đợt**: mở được hai phiên trên cùng một máy. Đã sửa và dựng lại đúng cảnh cũ để chứng minh: giờ trả `máy PC-03 đang có người chơi — hãy kết thúc phiên đó trước`.
- Các chốt chặn khác đều đúng: một tài khoản chỉ chơi một máy (`tài khoản đang chơi ở máy PC-04 — hãy trả máy đó trước`), số dư 0 bị chặn, "Đổi máy" chuyển trạng thái hai máy đúng, tác vụ "máy mất tín hiệu đóng phiên" chốt sổ tại nhịp tim cuối để không tính phần máy đã tắt.
- `go test ./... -count=1`: xanh toàn bộ sau khi cập nhật 3 test cũ bị gãy do đổi truy vấn (đúng cảnh báo của AGENTS.md về test chốt SQL).

### Vòng 10 — transactions → PASS (4 lỗi, sửa cả 4)
- Sổ giao dịch giờ đọc được: `Phí chơi -4.500₫ | số dư 210.000 → 210.000 | khuyến mãi 55.000 → 50.500` — trước đó hai cột số dư bằng nhau mà không giải thích được tiền đi đâu.
- **Lỗi múi giờ (#20) là lỗi có hậu quả thật nhất vòng này**: quán net đông khách nhất ca đêm, mà đúng các giao dịch 00:00–07:00 lại biến mất khi lọc theo ngày đó. Repo đã có sẵn `utils.StartOfDay/EndOfDay` neo giờ Việt Nam — chỉ là chỗ này không dùng.
- Lỗi bỏ dấu khi tìm kiếm nay đã gặp **lần thứ tư** (hội viên, nhóm hội viên, nhóm máy, giao dịch).
- `go test ./... -count=1` xanh; `python3 scripts/check-admin.py` → **0 phát hiện**; `pnpm typecheck` không phát sinh lỗi mới ở file đã sửa.
- **Bài học quy trình đã ghi vào prompt**: `pkill -f "cmd/server"` KHÔNG diệt tiến trình con `exe/server`, nên có lúc tôi kiểm chứng nhầm trên server cũ và tưởng "sửa không ăn". Từ nay diệt theo cổng: `kill -9 $(lsof -nP -iTCP:8080 -sTCP:LISTEN -t)`.

### Vòng 11 — categories → PASS (2 lỗi, sửa cả 2)
- Cây phân cấp hoạt động đúng: API trả `children`, bảng render tree (cấp 0 có nút mở rộng, cấp 1 thụt 16px), ô "Danh mục cha" trong form Sửa hiện đúng **tên** danh mục cha.
- **Lỗi #24 là loại nguy hiểm nhất: hỏng im lặng và mất dữ liệu hiển thị.** Hai thao tác hoàn toàn bình thường trên giao diện là đủ để cả cây danh mục biến mất (`data: null`), không lỗi, không dấu hiệu. Đã chặn và kiểm chứng lại trên hệ thống thật, cây trở về nguyên vẹn.
- Xoá danh mục còn con bị chặn đúng (nhưng bằng câu tiếng Anh `cannot delete category with sub-categories` — thêm một bằng chứng cho lỗi #8 đang chờ quyết định).
- Quét chủ động: còn 6 chỗ tìm kiếm chưa bỏ dấu, đã liệt kê đích danh (#26) để sửa khi tới lượt trang.
- `go test ./... -count=1` xanh toàn bộ.

### Vòng 12 — products → PASS (2 lỗi sửa, 1 chờ quyết định)
- **Lỗi #27 là hỏng im lặng đúng kiểu AGENTS.md §11 cảnh báo**: sản phẩm thuộc danh mục con bị ghi là **"Đã xoá"** dù danh mục vẫn sống. Chú thích ngay trên hàm `getCategoryName` còn viết "danh mục chỉ biến mất khi đã bị xoá mềm" — giả định đó sai vì mảng chỉ chứa danh mục gốc. Kèm theo đó là không gán được sản phẩm vào danh mục con.
- Bẫy `uuid` rỗng đã gặp **lần thứ ba**, nên lần này đặt hàm dùng chung `uuidRongThanhNil()` thay vì viết lại.
- Biên giá kiểm đủ: giá âm → 400 "Giá tối thiểu là 0" ✅, giá 0 → cho phép (hợp lệ với hàng tặng kèm), tên rỗng → 400 ✅, trùng tên → 400 "Tên sản phẩm đã tồn tại" ✅.
- Nút Nhập/Trừ kho bị khoá khi sản phẩm chưa có bản ghi tồn kho — **đúng**, không phải lỗi (đã suýt báo nhầm lần thứ 8).
- Tìm kiếm sản phẩm đã bỏ dấu sẵn từ trước ("Mi xao" → "Mì xào bò") — đây là chỗ duy nhất trong repo làm đúng từ đầu, và là khuôn mẫu cho 4 bản sửa tìm kiếm ở các vòng trước.
- `go test ./... -count=1` xanh toàn bộ. Console chỉ còn 1 cảnh báo deprecation của Element Plus (thư viện, không phải code repo).

### Vòng 13 — orders → PASS (2 lỗi sửa, 1 chờ quyết định)
- Luồng bán hàng chạy đủ: tạo đơn (tạm tính đúng 3 × 15.000 = 45.000₫) → **Xác nhận** (trừ kho 50 → 47) → **Thanh toán** (đơn Hoàn thành, ghi `completed_at`, có 3 hình thức: tiền mặt / trừ số dư hội viên / chuyển khoản).
- Chặn bán quá tồn rất tốt: `sản phẩm Mì tôm không đủ tồn kho (còn 47.00, cần 999)` — nêu đúng còn bao nhiêu, cần bao nhiêu, và tồn kho giữ nguyên.
- **Lỗi #30 là lỗi sổ kho nguy hiểm nhất vòng này** và nguyên nhân rất dễ tái phát: GORM `Updates` ghi ngược vào struct làm điều kiện ngay dòng sau luôn sai.
- **Một bài kiểm tôi viết ra đã bị loại bỏ vì nó vô dụng**: test sqlmock vẫn PASS khi tôi cố tình khôi phục lại lỗi cũ. Thay vì giữ một bài kiểm cho cảm giác an toàn giả, tôi bỏ nó và đưa phép kiểm vào `scripts/verify.py` — nơi chạy trên hệ thống thật và bắt được đúng lỗi này.
- `go test ./... -count=1` xanh toàn bộ; `verify.py` đã thêm 2 phép kiểm mới (cú pháp đã kiểm).

### Vòng 14 — stock-transactions ("Tồn kho") → PASS (2 lỗi sửa)
- **Đính chính checklist**: không có trang `inventory` riêng; menu "Tồn kho" chính là `stock-transactions`. Dòng 20 đã đánh dấu KHÔNG ÁP DỤNG thay vì để TODO treo.
- **Lỗi #33 tiếp nối đúng mạch lỗi #30 của vòng trước và nặng hơn**: không chỉ huỷ đơn làm mất hàng, mà **mọi** lần bán hàng bán thẳng đều không để lại dấu vết trong sổ kho. Tồn kho đổi 47→42→37→42 qua các thao tác mà bảng "Giao dịch tồn kho" vẫn trống 0 dòng. Sự bất đối xứng rất dễ bỏ sót: món chế biến (trừ theo nguyên liệu) **có** ghi sổ, hàng bán thẳng **không**.
- Sau khi sửa, sổ ghi đủ hai vế và đủ thông tin: `outbound 5 (42→37) Xuất kho theo đơn ORD-00006 — Admin` / `inbound 5 (37→42) Hoàn kho do huỷ đơn ORD-00006 — Admin`.
- Phiếu nhập/xuất thủ công hoạt động tốt: chọn sản phẩm hiện kèm tồn hiện tại ("Mì tôm — Trước: 42"), thành tiền tự tính (20 × 6.000 = 120.000₫), validate rỗng chặn đúng.
- `go test ./... -count=1` xanh toàn bộ sau khi cập nhật 2 test cũ và 8 chỗ gọi đổi chữ ký.

### Vòng 15 — inventory-counts → PASS (1 lỗi sửa)
- **Phần thiết kế đúng và tôi đã kiểm chứng bằng đúng tình huống khó nhất**: hộp thoại ghi rõ "khi chốt, hệ thống áp CHÊNH LỆCH lên tồn kho hiện tại chứ không đặt tồn bằng số đã đếm". Thử: đếm 40 / sổ 42 (lệch −2) → **bán xen 5 món** (42→37) → chốt phiên → tồn **35**, đúng bằng 37−2. Nếu đặt tồn bằng số đếm thì đã thành 40 và 5 món bán ra biến mất. Bút toán để lại: `adjustment -2 (37→35) Kiểm kê KK-00001: đếm 40.000, sổ 42.000`.
- Chính tính năng này là nơi hai lỗi kho ở vòng 13–14 gây hậu quả, và cũng là nơi bản sửa của tôi phát huy: dòng `outbound` của lần bán xen giờ đã có trong sổ.
- Các biên chặn đúng: số đếm âm → 400, chốt phiên không có dòng nào → 400, chốt lại phiên đã chốt → 400.
- **Lỗi #35 cùng họ với lỗi #16 (hai phiên chơi trên một máy)**: thiếu ràng buộc duy nhất. Lần này tôi đã **chứng minh bài kiểm bắt được lỗi** bằng cách tạm gỡ chốt chặn và thấy test fail — khác với bài kiểm ở vòng 13 mà tôi đã phải bỏ vì nó pass cả khi lỗi còn nguyên.
- `go test ./... -count=1` xanh toàn bộ. Console sạch.

### Vòng 16 — suppliers → PASS (2 lỗi sửa, 1 chờ quyết định)
- Trang đơn giản nhưng lộ ra **một lỗi backend nằm ở trang khác**: khi tôi thử gán nhà cung cấp cho sản phẩm để kiểm ràng buộc xoá, lệnh `PUT /products/:id` trả về "Giá tối thiểu là 0". Đây là loại lỗi chỉ lộ khi gọi API thật chứ bấm giao diện không thấy, vì form Sửa luôn gửi đủ mọi trường.
- Ràng buộc xoá viết tốt: `không xoá được: còn 1 sản phẩm — hãy ngừng hợp tác (bỏ đang hoạt động) thay vì xoá` — nêu đúng lối thoát phù hợp với nhà cung cấp (không xoá mà ngừng hợp tác).
- Quét chủ động 2 lần trong vòng này: `message: ''` (1 chỗ, đã sửa) và con trỏ thiếu `omitempty` trong binding (1 chỗ, đã sửa) — cả hai đều là chỗ duy nhất lệch khuôn trong toàn repo.
- **Suýt báo nhầm lần thứ 9**: tưởng form Sửa làm mất SĐT/email, soi database mới thấy bản ghi đó vốn không có hai trường đó (bản đầy đủ đã bị chính script kiểm thử của tôi xoá).
- `go test ./... -count=1` xanh toàn bộ. Console sạch.

### Vòng 17 — combos → PASS (3 lỗi sửa)
- **Lỗi #39 là lỗi chức năng nặng**: cả tính năng "mở phiên bằng gói trả trước" hỏng hoàn toàn qua đường `sessions/start`, và thông báo cho người dùng là một chuỗi SQLSTATE vô nghĩa. Chỉ lộ ra khi gọi API thật — nút "Kích hoạt" trên giao diện đi đường khác nên vẫn chạy.
- **Nghiệp vụ gói kiểm được trọn vẹn và đúng**:
  - Mua gói: số dư 210.000 → 110.000, `total_spent` +100.000, gói 300 phút hạn 30 ngày ✅
  - Chơi 60 phút bằng gói: tiền **0₫**, số dư không đổi, phút gói 300 → 240 ✅
  - Chơi **quá** số phút gói (còn 30, chơi 90): 30 phút đầu miễn phí, 60 phút sau tính **9.000₫** đúng theo giá hạng Vàng, phút gói về 0 ✅
- Xác nhận lại lỗi #19: mua gói là **nguồn duy nhất** làm `total_spent` tăng.
- **Suýt báo nhầm lần thứ 10 và 11**: hộp "Combo đã mua" trống (thực ra phải chọn hội viên trước) và ô chọn hội viên không có lựa chọn (thực ra là ô tìm kiếm từ xa, phải gõ mới có — và gõ "Nguyen" không dấu ra đúng "Nguyễn Văn Ánh" nhờ bản sửa ở vòng 3).
- `go test ./... -count=1` xanh toàn bộ sau khi cập nhật 2 test cũ. Console sạch.

### Vòng 18 — cards → PASS (1 lỗi sửa)
- **Phần bảo mật của trang này làm rất tốt và tôi đã kiểm chứng từng điểm:**
  - Mã bí mật chỉ hiện **một lần** khi sinh thẻ, kèm cảnh báo rõ ("đóng cửa sổ này mà chưa lưu là mất cả lô thẻ") và nút Chép / Tải CSV.
  - Database **chỉ lưu băm SHA-256** (64 ký tự); tìm mã bí mật nguyên văn trong bảng: 0 kết quả.
  - Sai seri và sai mã bí mật trả **cùng một thông báo** "thẻ không hợp lệ hoặc đã được sử dụng" — không tiết lộ sai ở đâu, đúng nguyên tắc.
  - Đổi thẻ đúng: số dư 110.000 → 160.000, khuyến mãi +5.000 ✅; đổi lại lần hai → chặn ✅; thẻ đã huỷ → chặn ✅.
- Lỗi #42 tìm được nhờ chính phép thử gửi thiếu trường. Đây là loại lỗi lộ ngay cho người dùng cuối nhưng dễ bỏ qua khi chỉ bấm giao diện (form luôn gửi đủ trường).
- `go test ./... -count=1` xanh toàn bộ. Console sạch.

### Vòng 19 — shifts → PASS (1 lỗi sửa)
- **Đối chiếu quỹ tiền mặt của backend tính đúng hoàn toàn**, tôi kiểm 3 kịch bản:
  - Ca khớp: đầu 500.000 + thu 40.000, đếm 540.000 → lệch **0** ✅
  - Ca thiếu: đầu 200.000 + thu 20.000, đếm 205.000 → lệch **−15.000** ✅
  - Có **bàn giao** giữa ca: đầu 300.000 + thu tay 150.000, đếm 450.000 → lệch **0** ✅ (tiền bàn giao có vào tiền dự kiến)
- Chặn mở ca thứ hai khi ca cũ còn mở ✅ (thông báo tiếng Anh — thêm một bằng chứng cho lỗi #8).
- Hộp "Đóng ca" **không hiện tiền dự kiến** trước khi nhập — đúng nguyên tắc kiểm quỹ: đếm trước rồi mới đối chiếu.
- Lỗi #43 là loại làm nhân viên mất tin vào báo cáo quỹ: con số đúng nằm trong database, chỉ là cột hiển thị sai nghĩa. Sau khi sửa, hai dòng ca đọc thông suốt: `500.000 → 540.000 / dự kiến 540.000 / lệch 0` và `200.000 → 205.000 / dự kiến 220.000 / lệch −15.000`.
- 3 lỗi typecheck ở file này nằm trong 51 lỗi có sẵn đã ghi ở vòng 3, không phát sinh mới.

### Vòng 20 — bookings → PASS (1 lỗi sửa)
- Tạo đặt chỗ **hoàn toàn qua giao diện**, kể cả ô chọn khoảng ngày giờ (phải bấm thật vào lịch — cách gõ tổng hợp không ăn).
- Validate theo từng bước rất rõ: thiếu tên/SĐT → "Nhập tên khách và số điện thoại"; thiếu giờ → "Chọn khoảng giờ giữ máy".
- **Tác vụ nền `bookings:expire` chạy đúng**: đặt chỗ quá giờ 15 phút mà không check-in tự chuyển "Không đến", và hàng nút thu lại chỉ còn "Xóa" — không cho check-in một lượt đã hết hiệu lực.
- Tiền cọc của **khách vãng lai** chỉ ghi bằng con số, không sinh bút toán (tiền mặt giữ ở quầy); code chỉ trừ/hoàn số dư khi đặt chỗ gắn với hội viên (`booking.go:522` kiểm `MemberID != nil`) — thiết kế hợp lý, không phải lỗi.
- Lỗi #44 cùng họ với #14 (trang Máy): **ô nhập hứa nhiều hơn code làm**. Sau khi sửa, cả "Le Thi", "Lê Thị", "PC-05" và "pc-05" đều ra đúng.
- `go test ./... -count=1` xanh toàn bộ.

### Vòng 21 — promotions → PASS (2 lỗi sửa)
- **Bộ máy khuyến mãi chạy chuẩn xác**, kiểm bằng tiền thật trên đơn hàng:
  - đơn 40.000 (dưới ngưỡng 50.000) → không áp ✅
  - đơn 50.000 → giảm **5.000** (10%), đơn ghi rõ tên khuyến mãi đã áp ✅
  - đơn 300.000 → giảm **20.000**, chạm trần `max_discount` chứ không phải 30.000 ✅
  - khoá điều kiện gõ sai (`min_amout`) → **chặn ngay lúc lưu** kèm danh sách khoá hợp lệ, không để sinh ra "khuyến mãi câm" ✅
- Vòng quay may mắn: tổng xác suất vượt 100% bị chặn kèm số cụ thể ("là 110.00%, vượt quá 100%"), đúng 100% thì cho; quay lần hai trong ngày bị chặn theo hạn mức ✅
- **Lỗi #45 là loại hỏng im lặng đáng sợ nhất của file locale**: khai trùng khoá trong cùng một object thì JavaScript lặng lẽ lấy khối sau, không lỗi cú pháp, không cảnh báo build — cả một nhóm bản dịch biến mất. Tôi đã viết bộ quét khoá trùng cho cả 3 file để chắc không còn chỗ nào khác.
- **Suýt báo nhầm lần thứ 12 và 13**: "khuyến mãi không áp dụng" (thực ra tôi gửi `condition_value` dạng chuỗi thay vì số) và "đơn vị xác suất lệch giữa giao diện và backend" (thực ra giao diện có chia 100 khi gửi, nhân lại khi hiện).

### Vòng 22 — curfew → PASS (2 lỗi sửa)
- **Cưỡng chế giới nghiêm chạy đúng**, kiểm bằng hội viên thật:
  - Đặt tranthib thành 15 tuổi, giờ hiện tại 02:50 nằm trong khung 22:00–06:00 → **chặn** ✅
  - Khách người lớn cùng lúc đó → mở phiên bình thường ✅
  - Khung **vắt qua nửa đêm** hoạt động đúng (02:50 thuộc 22:00–06:00) ✅
  - **Miễn trừ** kèm lý do → vị thành niên mở phiên được ngay, bảng ghi rõ lý do và thời điểm ✅
- Lỗi #47 đáng chú ý vì **đối tượng đọc câu đó là khách hàng cuối**, không phải nhân viên: chính chú thích trong `session.go` đã nêu nguyên tắc "phải bằng tiếng Việt vì nó hiện thẳng lên màn hình khoá của khách", và câu giới hạn giờ chơi ngay bên dưới cũng đã tuân theo — chỉ câu này lệch.
- Lỗi #48 là **lần thứ hai** gặp cùng khuôn (tra khoá i18n với row rỗng). Tôi đã quét cả 6 chỗ tương tự trong `src/views/vnet`: 4 chỗ đã an toàn sẵn, 1 chỗ (`website-blocking`) sẽ kiểm khi tới lượt.
- Kiểm lại trang Khuyến mãi với bảng rỗng để chắc PASS ở vòng 21 là thật: console sạch.
- `go test ./... -count=1` xanh toàn bộ. Dữ liệu đã dọn (xoá khung giờ, trả ngày sinh tranthib về rỗng).

### Vòng 23 — notifications → PASS (2 lỗi sửa)
- Thêm / Sửa / Xoá / Gửi đều chạy, kiểm đối chiếu CSDL sau mỗi thao tác. Bốn loại thông báo (Thông tin, Cảnh báo, Khuyến mãi, Hệ thống) đều có bản dịch, cột "Loại" hiện tiếng Việt chứ không phải mã thô. Đổi số dòng/trang gửi đúng `page_size=20`. Console sạch.
- Xoá thông báo dọn luôn sổ người nhận trong cùng transaction ✅ (12 → 0).
- **Lỗi #50 là loại chỉ lộ ra khi bấm thật hai lần**: code đọc thì hợp lý, `Dispatch` ghi hai bảng có chủ đích (sổ người nhận + hộp thư riêng của từng hội viên) — nhưng không ai chặn lần gửi thứ hai. Tôi đã nghi ngờ đúng chỗ này từ vòng trước khi thấy `notifications` có 1 dòng mà hai bảng con đều có 12.
- **Giả định đã chốt khi sửa #50**: "Gửi" nay có nghĩa là *gửi cho ai chưa nhận*, không phải *gửi lại cho tất cả*. Muốn thông báo lại nội dung đã sửa thì tạo thông báo mới. Nếu anh muốn ngược lại (cho phép gửi lại có cảnh báo), nói tôi đổi.
- **Ghi nhận, chưa sửa**: (a) `Dispatch` với ID không tồn tại trả **500** thay vì 404 — `GetByID` ngay cùng file đã trả 404 đúng; (b) xoá thông báo **không** thu hồi bản sao đã nằm trong hộp thư hội viên (`member_notifications` giữ nguyên 12 dòng) — đúng kiểu "thư đã gửi đi rồi", nhưng nếu anh muốn thu hồi thì phải thêm khoá ngoại; (c) câu lỗi `notification not found` vẫn tiếng Anh — nằm trong mục OPEN #8.
- `go test ./internal/service` xanh.

### Vòng 24 — website-blocking → PASS (2 lỗi sửa)
- **Bộ máy chặn web chạy đúng từ đầu tới cuối**, kiểm bằng chính endpoint mà máy trạm gọi (`GET /api/machines/by-code/:code/blocklist`, không cần đăng nhập):
  - Chuẩn hoá tên miền: nhập `https://www.Facebook.com:443/groups/abc?x=1` → lưu thành `facebook.com` ✅
  - **Phạm vi máy**: luật gán nhóm VIP → PC-03 (VIP) nhận `["facebook.com"]`, PC-01 (không nhóm) nhận `[]` ✅
  - **Lịch áp dụng**: khung 08:00–22:00, lúc kiểm là 03:33 → danh sách rỗng; bỏ khung giờ (24/7) → có lại ngay ✅
  - **Nhật ký vi phạm**: giả lập máy trạm POST 2 vi phạm → tab "Nhật ký vi phạm" hiện đủ mã máy, tên miền đã chuẩn hoá (`www.facebook.com/groups` → `facebook.com`), tiến trình, thời điểm ✅
  - Tên miền không có dấu chấm (`facebook`) bị chặn kèm câu tiếng Việt ✅
- Sửa luật điền sẵn **đúng cả lịch lẫn nhóm máy** (7 ô ngày, giờ, thẻ VIP); bỏ lịch/bỏ nhóm khi lưu thì xoá sạch bản ghi con trong CSDL ✅. Xoá luật dọn luôn lịch và ánh xạ nhóm ✅. Bấm Huỷ ở hộp xác nhận không xoá gì ✅.
- **Ghi nhận, chưa sửa (cần anh quyết)**: thêm `FACEBOOK.com` khi đã có `facebook.com` thì tạo ra **luật thứ hai trùng tên miền** — bảng nhìn như hai dòng y hệt. Không chặn cứng được vì mô hình dữ liệu cho phép nhiều luật cùng tên miền khác nhóm máy/khác khung giờ (và cặp *chặn + cho phép* cùng tên miền là tính năng thật: `EffectiveFor` để "cho phép" thắng "chặn"). Hướng an toàn là **cảnh báo** khi trùng, không phải từ chối — nói tôi làm nếu anh muốn.
- `go test ./... -count=1` xanh. Dữ liệu đã dọn (3 luật, 2 dòng vi phạm).

### Vòng 25 — feedback → PASS (1 lỗi sửa)
- Trang này **chỉ đọc** (không Thêm/Sửa/Xoá), nên trọng tâm là số liệu có đúng không. Tạo 25 đánh giá thật qua đúng đường máy trạm/nhân viên gọi rồi đối chiếu:
  - Điểm trung bình giao diện **3.2** ↔ CSDL `avg = 3.24` ✅, tổng 25 ✅, phân bố sao khớp từng mức ✅
  - Lọc theo máy PC-02: bảng còn 3 dòng **và thẻ tổng hợp tính lại riêng cho máy đó** (3.0) ✅
  - Lọc sao + "Chỉ xem có nhận xét" chồng nhau: PC-02 + 5★ + có nhận xét → rỗng đúng, không vỡ giao diện ✅
  - Xoá từng bộ lọc (dấu x) → danh sách và thẻ tổng hợp quay về toàn quán ✅
  - Phân trang: 24 dòng → trang 2 gửi đúng `page=2&page_size=20`, hiện 4 dòng ✅
  - Máy chưa có đánh giá nào (PC-04): hiện "–", 0 lượt, mọi thanh bằng 0, không hiện thanh phân trang ✅
- **Đường hội viên tự đánh giá chạy đúng như thiết kế**: đăng nhập hội viên (`member-login` kèm mã máy) **tự mở phiên**, đánh giá bám đúng máy của phiên đó, cột Hội viên hiện "Nguyễn Văn Ánh (nguyenvana)" thay vì "Khách vãng lai" ✅. Đánh giá gắn đơn hàng hiện mã `ORD-00009`; đánh giá lại cùng đơn bị chặn 409 "đơn hàng này đã được đánh giá" ✅.
- **Suýt báo nhầm lần thứ 14**: thấy đánh giá của hội viên tạo được trong khi CSDL không còn phiên nào `is_active` → tưởng chốt chặn "phải đang trong phiên" bị hở. Đọc log backend mới rõ: phiên do chính lệnh đăng nhập mở lúc 04:02:52, tác vụ đóng phiên máy offline đóng nó lúc 04:03:01 — đúng 9 giây sau. Không phải lỗi.
- `go test ./... -count=1` xanh. Dữ liệu đã dọn (25 đánh giá).

### Vòng 26 — app-updates → PASS (không có lỗi mới)
- Vòng đầu tiên **không tìm ra lỗi nào**. Trang này được viết cẩn thận hơn hẳn mức trung bình của repo và tôi đã cố bẻ đủ đường:
  - Lưu form rỗng → "Phiên bản không được để trống" ✅; băm `abc` → "băm phải là SHA-256 dạng hex 64 ký tự" ✅; đường tải `ftp://…` → "đường tải phải bắt đầu bằng http:// hoặc https://" ✅
  - Gõ `v1.2.0` → lưu thành `1.2.0`; băm chữ HOA → lưu thành chữ thường ✅
  - Trùng phiên bản + nền tảng → chặn kèm câu rõ nghĩa; **cùng phiên bản khác nền tảng thì cho** ✅
  - Dung lượng 0 → cột hiện `-` thay vì "0.0 MB" ✅
  - Xoá bản **đang phát hành** → chặn: "hãy tắt phát hành trước khi xoá"; tắt phát hành rồi xoá thì được ✅; bấm Huỷ ở hộp xác nhận không xoá gì ✅
- **Đối chiếu bằng đúng endpoint máy trạm gọi** (`GET /api/machines/by-code/:code/app-update`): máy ở 1.2.0 và 1.9.0 đều thấy 1.10.0 — tức so phiên bản theo số chứ không theo chuỗi (bẫy `"1.10.0" < "1.9.0"` mà chú thích trong code đã cảnh báo) ✅; máy ở 1.10.0 và 1.11.0 → không có bản mới ✅; tắt phát hành 1.10.0 → máy quay về thấy 1.2.0 ✅; nền tảng khác không lẫn sang nhau ✅; thiếu nền tảng → "thiếu nền tảng" ✅
- **Suýt báo nhầm lần thứ 15**: "cùng phiên bản khác nền tảng cũng bị chặn" — thực ra ô chọn nền tảng của tôi chưa đổi giá trị (thao tác bàn phím trên lớp phủ Element Plus), chọn lại đúng thì lưu được ngay.
- **Ghi nhận, không phải lỗi**: trang nạp cố định `page_size=100` và không có thanh phân trang — quán có quá 100 bản phát hành thì phần cũ sẽ không hiện. Thực tế vài chục bản là cùng, để nguyên.
- Console chỉ có các 4xx do chính tôi cố tình test. Log backend không panic. `go test ./... -count=1` xanh. Dữ liệu đã dọn (2 bản phát hành).

### Vòng 27 — reports → PASS (2 lỗi sửa, cả hai đều sai TIỀN)
- Cả 7 tab đều nạp đúng dữ liệu, không cột nào trống hay `undefined`; "Cài đặt cột" hiện đúng bộ cột của **tab đang mở** (bỏ chọn "Số đơn hàng" thì cột biến mất, chọn lại thì về) ✅; tab Doanh thu tháng tự suy `year=2026&month=09` từ đầu khoảng ngày ✅; xoá khoảng ngày thì các tab nạp lại toàn kỳ ✅.
- **Lỗi #54 là lỗi nguy hiểm nhất của vòng này**: người quản lý chốt sổ cuối ca, chọn đúng ngày hôm nay, và thấy **doanh thu bằng 0**. Với quán net Việt Nam thì 00:00–07:00 chính là ca đông nhất — toàn bộ phần đó rơi ra ngoài bộ lọc. Tôi đã sửa cùng một lỗi này cho trang Giao dịch ở vòng trước mà không quét hết các chỗ khác; lần này đổi cả 16 chỗ trong `report.go` một lượt.
- **Lỗi #55 khiến hai tab cùng màn hình nói hai con số khác nhau**: "Món bán chạy" gộp cả đơn nháp và đơn đã huỷ. Con số này dùng để quyết định **nhập hàng** — một đơn nháp gõ nhầm 999 gói mì đủ sức khiến quán nhập thừa cả thùng.
- Console sạch (một lỗi WebSocket là do chính tôi khởi động lại backend giữa chừng, tải lại trang là hết). Log backend không panic. `go test ./... -count=1` xanh.

### Vòng 28 — backups → PASS* (4 lỗi sửa)
- **`PASS*`** vì máy kiểm thử **không có `pg_dump`/`pg_restore`**, nên đường *thành công* của sao lưu và phục hồi không chạy được ở đây — đánh dấu `MANUAL`, cần kiểm trên máy chủ thật có bộ công cụ PostgreSQL. Mọi thứ còn lại đã bấm thật.
- **Lỗi #56 là loại tệ nhất có thể có ở một trang sao lưu**: nút bấm không làm gì cả, và vì lỗi #57 nuốt mất câu lỗi nên **không có dấu hiệu nào** cho biết. Một quán tin rằng mình có sao lưu hằng ngày, đến lúc hỏng ổ cứng mới biết chưa từng có tệp nào. Hai lỗi này che nhau: sửa một cái mà không sửa cái kia thì lần sau vẫn hỏng câm như vậy.
- Sau sửa, đường hỏng hiển thị đàng hoàng: dòng mới hiện ngay, trạng thái "Thất bại", rê chuột thấy đúng lý do.
- Đã bấm: tạo sao lưu ✅ · làm mới ✅ · phục hồi (đường lỗi) ✅ · xoá kèm hộp xác nhận nêu rõ "Tệp trên đĩa cũng bị xoá" ✅ · bấm Huỷ không xoá gì ✅ · nút Phục hồi chỉ hiện với bản `completed`, nút Xoá ẩn với bản `running` ✅ · chặn xoá bản đang chạy kiểm qua API ✅.
- Console sạch, log backend không panic, `go test ./... -count=1` xanh, dữ liệu đã dọn (2 dòng nhật ký, không tệp nào còn lại).

### Vòng 29 — audit → PASS (3 lỗi sửa)
- Lọc hành động/đối tượng/ngày, phân trang (`page=2&page_size=10` → `page_size=20`), mở rộng hàng xem metadata JSON — tất cả chạy đúng sau khi sửa. Console sạch, `go test ./... -count=1` xanh.
- **Lỗi #60 cho thấy quét một lần là chưa đủ**: vòng 27 tôi đã sửa 16 chỗ trong `report.go` nhưng không grep toàn repo, nên `audit.go` vẫn giữ nguyên lỗi. Lần này đã quét `time.Parse("2006-01-02"` trên toàn bộ `internal/` — còn đúng một chỗ cùng khuôn ở `feedback.go` (`parseDateOnly`) nhưng giao diện Đánh giá không có ô lọc ngày nên chưa ai chạm tới được; ghi lại để xử lý khi có ô lọc.
- **Lỗi #61 là kiểu tệ nhất của một trang nhật ký**: lọc ra rỗng khiến người quản lý tin rằng "không có ai làm gì" — trong khi thật ra chỉ là tên giá trị viết sai. Tôi đối chiếu với chính mã nguồn (`grep Action:/EntityType:`) chứ không đoán.
- **Ghi nhận, chưa sửa (cần anh quyết)**: (a) ô lọc mới chỉ phủ ~8/40 hành động và ~11/34 đối tượng mà hệ thống thật sự ghi; muốn phủ hết thì nên có endpoint trả danh sách giá trị phân biệt thay vì danh sách cứng trong giao diện; (b) **177/273 dòng nhật ký không có người thực hiện** (cột "Người dùng" là "-") vì phần lớn service không nhận actor — nhật ký không nói được ai tạo hội viên, ai sửa sản phẩm; đây là thay đổi xuyên suốt ~20 service nên tôi không tự ý làm.

### Vòng 30 — settings → PASS (3 lỗi sửa)
- **Kiểm bằng hiệu lực thật, không chỉ bằng việc lưu được**:
  - `max_bookings_per_member = 2` → đặt chỗ thứ ba bị chặn: "hội viên đã đặt 2 chỗ trong ngày 20/09/2026 — trần mỗi người là 2" ✅
  - `cancel_before_minutes = 30` → huỷ chỗ bắt đầu sau 10 phút bị chặn kèm hạn huỷ cụ thể ✅
  - Tắt **Cho phép đánh giá** → `POST /api/feedback` trả "quán đang tắt tính năng đánh giá" ✅; tắt **Cho phép điểm danh** → "quán đang tắt tính năng điểm danh" ✅; bật lại thì chạy bình thường ✅
  - Mệnh giá nạp: xoá một mức, thêm mức 30.000, lưu → CSDL giữ đúng `{"values": [...]}` đã sắp xếp ✅
  - Mỗi nhóm lưu đúng khoá của mình, **không lẫn khoá sang nhóm khác** (kiểm bằng cách liệt kê khoá theo nhóm sau khi lưu lần lượt cả 5 tab) ✅
- Ba lỗi của vòng này đều là **rác console** — không làm sai dữ liệu, nhưng đúng loại tiếng ồn khiến lần sau có lỗi thật thì không ai nhận ra. Sau khi sửa, mở lần lượt cả 5 tab: console **sạch hoàn toàn**.
- **Ghi nhận, chưa sửa (cần anh quyết)**: hai ô **không nơi nào đọc** — `store_email` (hoá đơn chỉ dùng tên/địa chỉ/điện thoại) và **`Múi giờ`** (toàn bộ backend neo cứng `Asia/Ho_Chi_Minh` trong `utils.VietnamLocation()`). Ô Múi giờ nguy hiểm hơn vì nhìn như có tác dụng: đổi nó xong người vận hành tưởng báo cáo đã tính theo múi giờ mới. Hoặc gỡ khỏi giao diện (đúng tiền lệ tab "Máy in" đã gỡ), hoặc nối thật vào `VietnamLocation()` — cả hai đều là quyết định của anh.

### Vòng 31 — printers → PASS (2 lỗi sửa)
- **Phân luồng món kiểm được tới tận đầu ra**, không chỉ ở màn hình cấu hình:
  - Gán "Mì tôm" + "Mì xào bò" cho Máy in bếp → CSDL đúng 2 dòng ánh xạ ✅; ô lọc tên món hoạt động ✅
  - `POST /api/orders/:id/print-stations` với đơn chỉ có "Nước suối" (chưa gán máy nào) → trả `unrouted: ["Nước suối"]`, đúng như thiết kế: món không gán thì **không im lặng biến mất** mà được báo ra ✅
  - Gán "Nước suối" cho Máy in bar rồi in lại → phiếu được định tuyến đúng máy, lỗi kết nối báo theo từng máy kèm 502 và câu tiếng Việt ✅
- Đặt **Mặc định** cho máy khác thì máy cũ tự mất nhãn — chỉ một máy mặc định tại một thời điểm ✅. Xoá máy in mặc định bị chặn kèm hướng dẫn ✅. Xoá máy in thường dọn luôn ánh xạ món ✅.
- **Suýt báo nhầm lần thứ 16**: bấm In thử xong 4 giây không thấy toast nào → tưởng nút câm như lỗi #49. Thực ra `pg`… không, thực ra lệnh quay số TCP mất ~6 giây mới hết giờ chờ, toast hiện sau đó rồi tự tắt trước lần tôi đọc lại. Cài `MutationObserver` trước khi bấm mới thấy đúng câu lỗi. **Bài học đo đạc**: với thao tác có chờ mạng, phải theo dõi từ trước khi bấm chứ không đọc sau.
- Console sạch, `go test ./... -count=1` xanh, dữ liệu đã dọn (3 máy in, mọi ánh xạ món).

### Vòng 32 — system/user → PASS (4 lỗi sửa)
- Trang này dùng **quy ước API thứ hai** (`/api/systemManage/*`, `current`/`size`, `records`) và **client HTTP thứ hai** — kiểm riêng vì đây là chỗ hai quy ước hay lẫn nhau. Danh sách, phân trang, lọc, thêm, sửa, xoá đều đi đúng đường.
- **Kiểm tới tận hiệu lực của tài khoản, không dừng ở "lưu thành công"**: tạo `thungan1` → gán vai trò `staff` → **đăng nhập thật được** với đúng vai trò ✅; đổi trạng thái sang **Tắt** → đăng nhập bị chặn ✅; xoá hàng loạt → CSDL xoá mềm đúng người ✅.
- **Lỗi #68 là lỗi đáng chú ý nhất vòng này**: nút "Xóa hàng loạt" chưa bao giờ chạy, và `pnpm typecheck` đã chỉ thẳng vào nó từ lâu (`Type 'User[]' is not assignable to type 'string[]'`) — nằm im trong đống 51 lỗi typecheck mà mục OPEN đã ghi nhận. Sửa xong còn 50. **Trang `system/role` còn nguyên dòng y hệt (dòng 143)** — sẽ kiểm và sửa ở vòng sau khi bấm thật trang đó.
- Console sạch, `go test ./... -count=1` xanh, dữ liệu đã dọn (tài khoản thungan1 đã xoá).
- **Ghi nhận, chưa sửa**: ô Email không kiểm định dạng (`khong-phai-email` lưu được) — cùng mục OPEN với email nhà cung cấp, chờ anh quyết là chặn ở đâu.

### Vòng 33 — system/role → PASS (3 lỗi sửa, 2 ô "chết" cần anh quyết)
- **Phân quyền là phần quan trọng nhất của trang này và nó chạy thật**: mở vai trò `staff` → Phân quyền → cây quyền đọc đúng bảng `permissions` của backend, tick một nhóm → lưu → `role_permissions` thay đổi đúng, và **đăng nhập lại bằng `staff` thì token mang đúng danh sách quyền mới**. Toast còn nhắc "người dùng phải đăng nhập lại mới có hiệu lực" ✅. Đã trả lại nguyên trạng quyền của `staff` sau khi kiểm.
- Xoá vai trò **đang gán cho tài khoản** bị chặn: "không xoá được vai trò đang gán cho 1 tài khoản — hãy gỡ vai trò khỏi họ trước" ✅.
- Ba lỗi #72–#74 là **bản sao y hệt** của #68–#70 ở trang Người dùng — hai trang Soybean dùng chung một khuôn mã. Vòng trước tôi đã ghi trước rằng dòng `@selection-change` ở đây cũng hỏng; lần này bấm thật để xác nhận rồi mới sửa.
- **Hai ô "chết" — cần anh quyết**:
  1. **Mã vai trò**: bảng `roles` **không có cột code**; DTO trả `RoleCode = role.Name`. Gõ `cashier_night` vào ô này thì bị vứt đi và cột "Mã vai trò" hiện lại chính cái tên. Hoặc bỏ ô đó khỏi giao diện, hoặc thêm cột `code` thật.
  2. **Trạng thái vai trò**: `toRoleManageResponse` neo cứng `Status: "1"`. Chuyển một vai trò sang **Tắt**, lưu "thành công", tải lại vẫn **Bật**. Nguy hiểm hơn ô trên: người quản lý tin rằng mình vừa khoá quyền của cả một nhóm nhân viên. Sửa đúng nghĩa là thêm cột trạng thái **và** quyết định nó chặn ở đâu (đăng nhập? cấp quyền? token đang hiệu lực?) — đó là quyết định về an toàn hệ thống nên tôi không tự làm.
- Console sạch, `go test ./... -count=1` xanh, dữ liệu đã dọn (vai trò thử đã xoá, quyền `staff` đã trả về như cũ).

### Ngoài vòng — dọn 3 mục OPEN theo yêu cầu (#18, #19, #29)
- **#18 — máy chưa gán nhóm = chơi miễn phí.** Không chặn, vì có quán cố ý để máy miễn phí; thay vào đó **không thể bỏ sót**: cờ `no_pricing` trong phản hồi tính tiền, cảnh báo vàng ngay trong hộp **Mở máy** khi chọn phải máy đó, và nhãn "Chưa gán nhóm — 0₫/giờ" ở trang Máy. Kiểm thật: chọn PC-05 → hiện cảnh báo; chọn PC-03 (nhóm VIP) → cảnh báo biến mất.
- **#19 — hệ thống hạng hội viên nằm im.** Chốt định nghĩa **`total_spent` = tiền khách thực tiêu tại quán**: tiền giờ chơi + đơn hàng đã hoàn tất + gói cước. **Nạp tiền không tính** (đó là tiền vào ví, chưa tiêu). Kiểm thật đầu-cuối trên hệ thống đang chạy:
  - Phiên 1 phút trên máy 15.000₫/h → trừ 250₫, `total_spent` 0 → **250** (trước đây đứng yên)
  - Đơn 20.000₫ thanh toán bằng số dư → `total_spent` → **20.250**
  - Nạp 50.000₫ → số dư tăng, `total_spent` **không đổi** ✅
  - Đặt mốc 600.000₫ rồi bấm "Xếp lại hạng" → hội viên lên hạng **Bạc** (`moved: 1`) — thứ trước đây không bao giờ xảy ra
  - Migration `backfill_member_total_spent` tính lại cho dữ liệu cũ: `nguyenvana` 0 → **113.651₫** (100.000 gói + 13.651 tiền giờ), khớp đúng tổng trong sổ giao dịch
  - **Ghi nhận thêm**: `RefreshTiers` chỉ **thăng** hạng, không bao giờ hạ. Hội viên được gán tay vào hạng cao vẫn giữ nguyên dù chi tiêu thấp hơn ngưỡng. Cần anh quyết có hạ hạng hay không.
- **#29 — mã danh mục không tồn tại vẫn tạo được sản phẩm.** `danhMucTonTai()` chặn ở cả Create lẫn Update: mã bịa → `không tìm thấy danh mục`; mã thật → tạo bình thường.
- Sửa #18 lộ ra **lỗi #75**: cột "Nhóm" ở trang Máy chưa bao giờ hiện tên nhóm. Đã sửa luôn.
- `go test ./... -count=1` xanh; `pnpm typecheck` giữ nguyên 49 lỗi có sẵn (không thêm lỗi mới); console sạch.

### Vòng 34 — system/menu → PASS (2 lỗi sửa)
- Trang chỉ để xem, và nó **nói thật**: khối cảnh báo giải thích cây menu nằm trong `internal/service/route.go`, không có bảng menu trong CSDL, nên không có nút Thêm/Sửa/Xoá giả nào. Đây là cách xử lý đúng với một trang mẫu không có backend tương ứng — khác hẳn hai ô "chết" ở trang Vai trò (#33).
- Hai lỗi vòng này đều là **dữ liệu bị che**: 15/35 mục menu không xem được vì phân trang không gửi tham số, và 33 mục bị đếm hai lần vì danh sách vừa phẳng vừa lồng.
- **Suýt báo nhầm lần thứ 17**: cột "Biểu tượng" đọc ra chuỗi rỗng nên tôi tưởng cột chết như #75. Thực ra nó vẽ `<svg>` — không có chữ nào để `textContent` trả về. Phải kiểm `querySelector('svg')` mới thấy. **Bài học**: cột chỉ chứa biểu tượng không kiểm được bằng text.
- Console sạch, `go test ./... -count=1` xanh, `pnpm typecheck` trở lại 49 lỗi có sẵn (lúc sửa tôi làm tăng lên 52 vì dùng sai kiểu tham số, đã sửa đúng kiểu `CommonSearchParams`).

### Vòng 35 — user-center + đổi mật khẩu → PASS (1 lỗi sửa) — **hết trang TODO**
- **Đổi mật khẩu chạy thật**, kiểm bằng chính tài khoản đang dùng: bỏ trống → "Vui lòng nhập đủ mật khẩu cũ và mới"; mật khẩu mới 3 ký tự → "Mật khẩu mới phải từ 6 ký tự"; nhập lại lệch → "Hai lần nhập mật khẩu mới không khớp"; mật khẩu cũ sai → "mật khẩu hiện tại không đúng"; đổi thành công → **mật khẩu cũ đăng nhập không được nữa, mật khẩu mới đăng nhập được**. Đã đổi trả về `admin123` và xác nhận đăng nhập lại bình thường.
- Hộp thoại dịch đủ 3 ngôn ngữ (kiểm tiếng Anh: "Change password / Current password / New password / Confirm new password").
- **Lỗi #78 là loại chỉ lộ ra khi đối chiếu hai trang với nhau**: thao tác thành công trên màn hình, nhưng trang Nhật ký hoạt động không có dòng nào. Nếu chỉ kiểm trang đổi mật khẩu thì nó "PASS" hoàn toàn.
- **Ghi nhận, chưa sửa (cần anh quyết)**: mục **"Trung tâm người dùng"** trong menu avatar dẫn tới một trang trống chỉ ghi "Sắp ra mắt" (`views/user-center/index.vue` chỉ render `<LookForward />`). Nó không nói dối như hai ô ở trang Vai trò, nhưng vẫn là một mục menu không dùng được trong sản phẩm đang chạy. Hoặc ẩn mục đó khỏi menu avatar (sửa một dòng), hoặc làm nội dung cho nó.
- Console sạch, `go test ./... -count=1` xanh.

## Đã hết 35/35 trang — còn lại các mục kiểm xuyên trang X1–X7

### Vòng 36 — X1 (realtime) + X2 (luồng đầu-cuối) → cả hai PASS, 1 phát hiện lớn
- **X1**: hai tab cùng mở trang Hội viên — nạp tiền ở tab A thì tab B tự đổi số dư, không tải lại. Tương tự hai tab trang Sản phẩm với `stock:changed` khi xác nhận đơn. Backend gửi **hai sự kiện khác nhau có chủ đích**: `balance:updated` tới thiết bị của chính hội viên, `member:updated` broadcast tới mọi kết nối admin — đọc `balance_event.go` mới thấy, và admin đăng ký đúng cái thứ hai.
- **X2**: dựng trọn một khách mới qua giao diện rồi đối chiếu từng con số — sổ ví khớp từng dòng, `total_spent` đúng nghĩa mới (không tính tiền nạp), tồn kho đúng, trang Giao dịch hiện đủ 3 dòng theo thứ tự thời gian với số dư trước/sau liền mạch.
- **Phát hiện #79 là thứ đáng giá nhất vòng này**: công thức "Doanh thu hôm nay" **trừ đi** tiền bán gói cước và tiền giờ chơi, đồng thời **cộng hai lần** tiền nạp với đơn trả bằng số dư. Đây là con số lớn nhất trên Bảng điều khiển. Tôi không tự sửa vì phải chọn mô hình kế toán trước — nhưng phần dấu âm thì sai ở mọi mô hình.
- **Suýt báo nhầm lần thứ 18**: thấy Bảng điều khiển ghi 286.099₫ còn Báo cáo ghi 582.849₫ → tưởng hai chỗ tính khác nhau. Thực ra Bảng điều khiển gọi đúng `/reports/daily-revenue`; hai số chênh nhau chỉ vì tôi xem cách nhau nửa tiếng và giữa đó có thêm giao dịch.
- Dữ liệu để lại có chủ đích: khách `khachx2` cùng 3 dòng ví và 1 phiên chơi là bằng chứng của X2, xoá đi thì sổ cái không còn khớp.

### Vòng 37 — X3 (phân quyền) + X4 (phiên đăng nhập) → PASS sau khi sửa 2 lỗi, trong đó 1 lỗi bảo mật
- **Lỗi #80 là lỗi nghiêm trọng nhất của cả đợt kiểm**: một tài khoản `staff` — nhân viên quầy — tạo được tài khoản mới mang vai trò `owner`. Tôi dựng lại bằng chính API, tài khoản được tạo thật, rồi mới sửa và xoá nó đi. Nguyên nhân chỉ là một chuỗi thừa trong `cmd/seed/main.go`, ngay dưới một dòng chú thích nói đúng ý định ngược lại.
- Sau khi thu hồi: staff 403 ở `systemManage`, `backups`, `audit-logs`, `admin/notifications`; menu của staff cũng không còn các mục đó (route.go gác theo cùng một quyền). Menu ba vai trò: owner 35 mục · manager 30 · staff 24.
- **Lỗi #81 ảnh hưởng mọi nhân viên mỗi ngày**: access token sống 24 giờ, refresh token 7 ngày, nhưng 26 trang VNET không bao giờ làm mới — hết hạn là văng ra đăng nhập giữa ca. Đây đúng là thứ mà `CLAUDE.md` mô tả là "giao kèo" của mã 9999, và nửa còn lại của ứng dụng (client Soybean) vẫn giữ đúng giao kèo đó.
- **Ghi nhận, chưa sửa**: (a) `manager` **không** vào được Quản lý người dùng / Sao lưu / Nhật ký — đúng như seed cố ý, nhưng anh nên xác nhận có phải ý mình không; (b) quyền chỉ gác 6 điểm (`members.create`, `members.topup`, `reports.view`, `settings.edit`, `client.admin`) — mọi thứ khác chỉ cần là nhân viên, nên **staff đổi được giá sản phẩm** (tôi đã thử đổi thành 999.000₫ rồi trả về 10.000₫); (c) mã **7777** không có chỗ nào phát ra; (d) Bảng điều khiển của staff gọi `/reports/daily-revenue` rồi nhận 403 hai lần mỗi lần mở.
- `go test ./... -count=1` xanh; `pnpm typecheck` giữ nguyên 49 lỗi có sẵn.

### Ngoài vòng — phân quyền chi tiết tới từng API (theo yêu cầu)
- **Trước**: 11 quyền, 6 quyền có tác dụng, phần lớn API chỉ cần "là nhân viên".
- **Sau**: **140 quyền** trong `internal/authz/catalog.go`, **187/189 API nhân viên** có quyền riêng; mã dùng ở router luôn phải nằm trong danh mục (có test đọc thẳng `router.go` để bắt gõ sai).
- Menu đi theo quyền: mỗi mục gác bằng `<nhóm>.view`, nhóm "Quản lý hệ thống" gác bằng `system.users.view` (không còn dựa vào `client.admin`).
- Mặc định sau migration: owner **140** quyền · manager **117** (mọi thứ trừ back-office) · staff **53** (vận hành, không cấu hình). Migration chỉ **thêm**, không gỡ quyền ai đó tự cấp.
- Kiểm bằng ba tài khoản thật: staff đổi giá sản phẩm → **403**, tạo khuyến mãi → **403**, xem phiên/bán hàng vẫn 200; manager đổi giá → 200 nhưng back-office → 403; owner → 200 tất cả.
- **Chứng minh mức chi tiết tới từng API**: tick thêm đúng ô `products.update` trên màn hình Phân quyền → staff đổi giá được ngay (200), trong khi `products.delete` vẫn 403. Sau đó đã gỡ lại ô vừa tick.
- Kiểm giao diện bằng tài khoản staff: Sản phẩm, Đơn hàng, Phiên, Máy, Kiểm kê, Ca làm việc, Giao dịch đều nạp bình thường, không có lỗi đỏ nào.
- `go test ./... -count=1` xanh · `check-admin.py` 0 phát hiện · `pnpm typecheck` giữ nguyên 49 lỗi có sẵn.

### Vòng 38 — X5 (i18n) + X6 (thu nhỏ + chế độ tối) → PASS sau khi sửa 2 lỗi
- **X5**: viết một bộ quét chạy qua **26 trang × 3 ngôn ngữ**, tìm mọi chuỗi dạng `vnetPages.*`, `route.*`, `common.*` lọt ra màn hình — **không còn chỗ nào**. Ba file locale cũng có cùng bộ khoá.
- **Suýt báo nhầm lần thứ 19**: bộ so khớp khoá của tôi báo 5 khoá thiếu ở EN/ZH; hoá ra chúng chỉ nằm ở dòng kế tiếp do prettier ngắt dòng. Phải mở đúng số dòng ra xem mới biết.
- **Lỗi #83 lộ ra ngay ở màn hình đăng nhập**: đổi sang tiếng Anh thì sản phẩm tự xưng là "SoybeanAdmin" — tên của bản mẫu.
- **X6**: ở bề ngang điện thoại, 7 trang chính không tràn ngang, menu thu gọn, form vẫn đọc được. Chế độ tối lộ ra **lỗi #84**: những chỗ viết cứng mã màu sáng — nặng nhất là điểm trung bình ở trang Đánh giá (chữ đen trên nền đen, mất hẳn) và khối JSON ở Nhật ký (chữ sáng trên nền sáng). Sửa bằng biến chủ đề nên cả hai chế độ đều đúng.
- `pnpm typecheck` 49 lỗi có sẵn · `check-admin.py` 0 phát hiện.

### Vòng 39 — X7 (bộ kiểm cuối) → PASS, không có lỗi mới do đợt sửa gây ra
- Câu hỏi duy nhất của vòng này: **đợt sửa 82 file có làm hỏng thứ gì đang chạy được không?** Nên mọi con số đều được đo **hai lần** — trên nhánh đã sửa và trên bản gốc `HEAD` dựng ở `git worktree` riêng, hai CSDL riêng, hai cổng riêng.
- `go test ./... -count=1`: **13/13 gói xanh**. `go vet ./...`: **sạch**. `check-admin.py`: **0 phát hiện**.
- Ba bộ lint đều giữ nguyên mốc có sẵn: `golangci-lint` **54** (errcheck/unused, không dòng nào thuộc file tôi sửa) · `pnpm typecheck` **49** · `pnpm lint` **71**. Trong lúc kiểm tôi tự làm phát sinh **1 lỗi lint mới** (`__retried` vi phạm `no-underscore-dangle`) — đã đổi tên thành `daGuiLai` và chạy lại luồng làm mới token trên trình duyệt để chắc nó vẫn đúng (log server: `401 → POST /auth/refresh 200 → GET /members 200`).
- `scripts/verify.py` trên CSDL dựng mới hoàn toàn: **293 ok / 14 hỏng** · bản gốc: **291 ok / 14 hỏng**. So từng dòng: **14 lỗi giống hệt nhau**, đều có sẵn từ trước, cùng một gốc (một phiên bị tác vụ đóng-phiên-ngoại-tuyến kết thúc trước khi harness kịp dùng). 2 mục ok nhiều hơn là do tôi sửa chính harness: nó mở phiên kiểm kê mới mà không huỷ phiên rỗng đang mở, nên tự chặn mình.
- `go vet` lộ thêm một chỗ **thật sự thành lỗi vì bản sửa của tôi**: `printer.go` ghép địa chỉ bằng `ip + ":" + port`. Trước đây chỉ là cảnh báo, nhưng validation mới (`ip|hostname`) cho phép nhập IPv6 → phải dùng `net.JoinHostPort`.
- **Suýt hỏng cả đợt kiểm**: để lấy mốc lint của bản gốc tôi chạy `git stash push` — nó cuốn sạch **82 file / 2521 dòng** của cả đợt. `git stash pop` lấy lại được ngay và tôi đã kiểm chứng từng thứ còn nguyên (185 lời gọi `PermissionRequired`, thư mục `internal/authz/`, test xanh). **Bài học đã ghi vào `UI-AUDIT-PROMPT.md`: đo mốc so sánh thì dùng `git worktree add`, không bao giờ `git stash`.**
- Đã dọn môi trường tạm: gỡ worktree `/tmp/vnet-head`, xoá CSDL `vnet_head`/`vnet_verify`, tắt cổng 8098/8099.

## Tổng kết cả đợt kiểm (vòng 1 → 39)

| Hạng mục | Kết quả |
|---|---|
| Trang bấm tay | **35/35 PASS** |
| Kiểm xuyên trang | **X1–X7 đều PASS** |
| Lỗi ghi nhận | **82** — 74 đã sửa · 7 OPEN chờ anh quyết · 1 hoá ra không phải lỗi app |
| Kiểm chứng không gây hồi quy | `verify.py` 293/14 so với bản gốc 291/14, **danh sách lỗi trùng khớp** |
| Suýt báo nhầm đã chặn lại | **19 lần** — đều do bắt buộc phải bấm/xem thật trước khi ghi sổ |

**Bốn lỗi đáng giá nhất** (đều là loại không bao giờ lộ ra nếu chỉ chạy unit test):
1. **#80 — leo thang đặc quyền**: tài khoản `staff` tạo được tài khoản `owner`. Nguyên nhân là một chuỗi thừa trong seed, nằm ngay dưới dòng chú thích nói ngược lại.
2. **#81 — cả 26 trang VNET văng đăng nhập giữa ca** vì không bao giờ làm mới token, trong khi nửa còn lại của ứng dụng vẫn làm đúng.
3. **#16 — hai phiên cùng chạy trên một máy**: hai khách bị tính tiền cho một chỗ ngồi, chỉ xảy ra khi máy trạm rớt mạng giữa phiên. Dựng lại được trên hệ thống thật.
4. **Phân quyền chi tiết tới từng API** (theo yêu cầu): từ 11 quyền / 6 quyền có tác dụng → **140 quyền, 187/189 API được gác**, có test đọc thẳng `router.go` để không ai gõ sai mã quyền.

**Đã chốt và đã sửa hết theo quyết định của anh** (xem mục "Ngoài vòng — sửa trọn danh sách chờ quyết định"):
| # | Vấn đề | Đã làm |
|---|---|---|
| 3 | Danh sách sắp theo UUID ngẫu nhiên | `DefaultSort` → `created_at`; `backup_logs` dùng `started_at` |
| 8 / 26 | Thông báo tiếng Anh + tìm kiếm chưa bỏ dấu | Dịch 89 thông điệp ở backend; thêm `unaccent()` cho mọi chỗ có chữ tiếng Việt |
| 32 | Soft delete làm cháy tên | 7 unique index chuyển sang một phần; 4 index nhạy cảm giữ nguyên |
| 38 | Email nhà cung cấp | `binding:"omitempty,email"` ở Create + Update |
| 79 | Công thức doanh thu | Ghi nhận lúc tiền vào quán, đếm đúng một lần |
| 85 | Tính lại tiền theo giá mới | Tính từng phút theo giá của khung giờ nó thuộc về |
| 86 | Hình thức thanh toán không kiểm | `oneof=cash transfer ewallet bonus_balance` |
| 87 | Bán thẻ không ghi tiền | Thêm `sold_payment_method`; doanh thu đếm ở lúc bán |
| — | Bốn nút chết | Đã ẩn: mã vai trò, trạng thái vai trò, "Trung tâm người dùng", múi giờ + email cửa hàng |

**Còn lại đúng 1 mục OPEN:**
| # | Vấn đề | Quyết định |
|---|---|---|
| 7 | 49 typecheck + 53 golangci-lint + 71 lint admin **có sẵn** | Theo đề xuất: **dọn dần theo từng file khi có việc chạm tới**, không làm một đợt lớn. Pre-commit hook vẫn bị chặn cho tới khi dọn xong |

**Ba mục tôi CỐ Ý không tự quyết** (vì ở danh sách trước tôi chỉ nêu câu hỏi chứ không đưa đề xuất):
- **Hạ hạng hội viên**: `RefreshTiers` chỉ thăng, không bao giờ hạ. Hạ hạng là thay đổi khách nhìn thấy được, phải do anh chốt.
- **Mã phiên 7777**: tồn tại trong code, không chỗ nào phát ra. Bỏ hẳn hay dùng cho một tình huống đăng xuất cụ thể?
- **Nhật ký thiếu người thực hiện**: 177/273 dòng không có `user_id` (tác vụ nền và vài đường ghi sổ). Có cần truy ra ai làm gì không?

**Chưa commit gì cả** — toàn bộ thay đổi nằm trong cây làm việc của nhánh `feature/core`, và stash WIP của anh (`stash@{0}`) không bị đụng tới.

### Ngoài vòng — chốt mô hình doanh thu (#79) theo quyết định của anh
- **Quyết định**: doanh thu ghi nhận **một lần, lúc tiền đi từ tay khách vào quầy**. Khách tiêu từ ví sau đó không tính lại, vì số tiền ấy đã được ghi nhận từ lúc nạp.
- Nguyên tắc này chạm tới **ba khoản nữa** ngoài `session_fee` mà anh nêu, và tôi áp nhất quán cả ba: gói cước trả bằng **số dư**, đơn hàng trả bằng **số dư**, và **đơn `order_type='topup'`** (phiếu khách bấm nạp từ máy trạm — nó sinh kèm một dòng ví `topup`, đếm cả hai là đếm một khoản tiền hai lần).
- Số dư **tặng** (`topup_bonus`, `attendance_bonus`, `lucky_spin_balance`) cũng bị loại: quán cho không, không có đồng nào vào két. Trước đây `topup_bonus` được tính như tiền thật.
- Kiểm chứng: API `/reports/daily-revenue` trả **2.084.000₫**; cộng tay từng khoản trong CSDL cũng ra **2.084.000₫**; Bảng điều khiển hiện đúng con số đó.
- **Chỗ nguy hiểm suýt lọt**: điều kiện mới có `OR`, mà `AND` bám chặt hơn `OR` trong SQL. Không tự đặt ngoặc bao ngoài thì bộ lọc ngày ghép vào sau chỉ áp cho vế cuối, và "doanh thu hôm nay" sẽ cộng cả tiền nạp của mọi ngày khác. Đã đặt ngoặc tường minh và **siết test mốc ngày** để nó khẳng định luôn cả cặp ngoặc này, thay vì khớp lỏng như trước.
- **Lộ ra một lỗ hổng mới, chưa sửa**: `SellTopupCard` chỉ đánh dấu `sold_to`/`sold_at`, **không ghi lại đồng tiền nào**. Tiền mặt khách trả để mua thẻ nạp không xuất hiện ở bất kỳ báo cáo nào, cũng không vào phần đếm tiền mặt lúc chốt ca. Vì vậy tôi tạm tính thẻ nạp ở **lúc khách dùng thẻ** — đúng một lần, không đếm trùng, nhưng ghi nhận muộn, và thẻ đã bán mà chưa dùng thì không bao giờ được tính. Xem mục #87.

### Ngoài vòng — sửa trọn danh sách chờ quyết định (#3, #8, #26, #32, #38, #85, #86, #87 + 4 nút chết)

**#85 là mục đáng giá nhất.** Bản cũ tra MỘT đơn giá tại thời điểm tính tiền rồi nhân với toàn bộ số phút đã chơi, nên phiên vắt qua mốc cao điểm bị tính lại từ đầu theo giá mới. Nay `CalculateCost` nhận thêm mốc bắt đầu và cộng tiền **từng phút**, mỗi phút giữ giá của khung giờ nó thuộc về. Ba chi tiết phải giữ đúng:
- **Làm tròn lên vẫn áp ở TỔNG**, không ở từng phút — nếu không, một phiên không đổi khung giờ sẽ đắt hơn bản cũ do cộng dồn phần làm tròn.
- **Bỏ lọc `day_of_week` ở SQL**: phiên vắt nửa đêm nằm trên hai thứ khác nhau, lọc sẵn theo thứ hôm nay thì phần sau 00:00 mất giá của nó.
- **Giá theo hạng hội viên không cắt đoạn** vì nó không đổi theo giờ; chỉ nhánh giá khung giờ mới cần.

Kiểm chứng đầu-cuối trên hệ thống đang chạy: đặt khung cao điểm 80.000₫ bắt đầu ngay phút kế tiếp, mở phiên trước đó ở giá 20.000₫ → phút 4 thu **3.334₫** (= 2×20.000 + 2×80.000) thay vì **5.334₫** của bản cũ; phút 5 → 4.667₫; phút 6 → 6.000₫; trả máy chốt đúng 6.000₫.

**Giới hạn còn lại của #85, ghi rõ để không ai tưởng đã hết**: nếu **người quản trị tự tay sửa giá gốc của nhóm máy** giữa lúc có phiên đang chạy thì thời gian đã chơi vẫn bị tính theo giá mới. Giá khung giờ suy ra được từ đồng hồ nên cắt đoạn được; giá gốc bị sửa tay thì không có lịch sử để tra. Muốn đúng cả trường hợp này phải lưu lịch sử giá — chưa làm.

**#87 kéo theo một thay đổi ở công thức doanh thu.** Trước đây thẻ nạp được tính ở lúc khách *dùng*; nay tính ở lúc *bán* (`thuTienBanThe`, theo `sold_at`, bỏ thẻ đã huỷ) và `topup_card` bị **bỏ hẳn** khỏi sổ ví. Đếm cả hai là đếm hai lần cùng một khoản. Kiểm thật: bán thẻ 50.000₫ → doanh thu 2.434.000 → **2.484.000**; cộng tay từng khoản trong CSDL cũng ra 2.484.000.

**Bốn nút chết đã ẩn**, kèm chú thích ngay tại chỗ nói vì sao và cần gì để bật lại: mã vai trò + trạng thái vai trò (bảng `roles` không có cột code, `Status` neo cứng `"1"`), "Trung tâm người dùng" (trang chỉ render `<LookForward />`), múi giờ + email cửa hàng (không nơi nào đọc). Đã kiểm trên giao diện: trang Vai trò còn 4 cột, hộp Sửa còn 2 ô, lưu vẫn 200; menu avatar còn 2 mục; tab Chung của Cài đặt còn 3 ô. **Route `/user-center` vẫn gõ thẳng URL vào được** — nó do elegant-router sinh ra từ thư mục `views/`, gỡ menu không gỡ route.

**Sự cố khi cập nhật chính sổ này**: lệnh đánh dấu FIXED của tôi khớp `| 3 |` ở **bảng danh sách trang** thay vì bảng lỗi, ghi đè 3 cột cuối của 4 dòng (#3, #8, #26, #32). Đã khôi phục ngay: tên ảnh lấy lại từ `ui-audit-shots/`, cột Console và Trạng thái theo đúng khuôn 31 dòng còn lại; bảng nay vẫn đủ 35 dòng với phân bố trạng thái như cũ. **Bài học**: hai bảng trong sổ này dùng chung khuôn `| <số> |`, phải phân biệt bằng số cột (bảng lỗi 8 cột, bảng danh sách trang 10 cột) chứ không bằng số thứ tự.

Kết quả kiểm toàn bộ sau đợt sửa: `go test ./...` **13/13 gói xanh** · `go vet` **sạch** · `golangci-lint` **53** (giảm 1 so với mốc 54 do gỡ hàm `timeBasedRate` mà thay đổi của tôi làm thừa) · `pnpm typecheck` **49** · `eslint` **71** — đúng mốc có sẵn, không thêm lỗi nào. Trong lúc làm tôi có gây ra 1 lỗi lint (`enableStatusRecord` thành thừa sau khi bỏ cột Trạng thái) và đã dọn.
