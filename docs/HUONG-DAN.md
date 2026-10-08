# VNET — Hướng dẫn sử dụng

Tài liệu này dành cho ba người khác nhau:

- **Người triển khai** — dựng máy chủ, cài máy trạm lên từng PC. Đọc phần 1 đến 3.
- **Nhân viên quầy** — mở máy, thu tiền, chốt ca. Đọc phần A, tra phần B khi cần.
- **Chủ quán** — phân quyền, xem báo cáo, xử lý sự cố. Đọc phần C và D.

Mọi câu trong tài liệu này viết ra từ mã nguồn tại thời điểm cập nhật, không chép lại tài liệu cũ. Chỗ nào hệ thống làm khác với điều người ta thường tưởng, tài liệu nói thẳng ra.

---

## Mục lục

**Triển khai**
- [0. Hệ thống gồm những gì](#0-hệ-thống-gồm-những-gì)
- [1. Dựng máy chủ](#1-dựng-máy-chủ)
- [2. Cài máy trạm lên PC khách](#2-cài-máy-trạm-lên-pc-khách)
- [3. Máy trạm dưới mắt khách](#3-máy-trạm-dưới-mắt-khách)

**Sử dụng**
- [A. Việc thường ngày](#a-việc-thường-ngày)
- [B. Tra cứu từng màn hình](#b-tra-cứu-từng-màn-hình)

**Quản trị**
- [C. Phân quyền](#c-phân-quyền)
- [D. Vận hành và sự cố](#d-vận-hành-và-sự-cố)

---

# 0. Hệ thống gồm những gì

Ba phần. Chỉ có **hai** thứ phải cài đặt: máy chủ và máy trạm.

```
        ┌──────────────────────────────────────────────┐
        │              MÁY CHỦ (một máy)               │
        │                                              │
        │   vnet-server  ──  một tệp thực thi duy nhất │
        │   ├── API              /api/*                │
        │   ├── Trang quản trị   đã nhúng sẵn bên trong│
        │   └── WebSocket        /api/ws/client        │
        │                                              │
        │   PostgreSQL  ──  nơi chứa toàn bộ dữ liệu   │
        └───────▲──────────────────────────▲───────────┘
                │                          │
     nhịp tim 15 giây               trình duyệt
     + WebSocket 2 chiều            (không cài gì)
                │                          │
    ┌───────────┴──────────┐      ┌────────┴─────────┐
    │  MÁY TRẠM (mỗi PC)   │      │  NHÂN VIÊN QUẦY  │
    │  vnet-client.exe     │      │  điện thoại / PC │
    │  + dịch vụ Windows   │      └──────────────────┘
    └──────────────────────┘
```

**Máy chủ** là *một* tệp thực thi. Trang quản trị đã được nhúng sẵn bên trong nó lúc biên dịch, nên một cổng phục vụ cả giao diện lẫn API. **Không cần nginx**, không cần dựng web server riêng.

**Trang quản trị** chạy trong trình duyệt. Nhân viên chỉ cần mở địa chỉ máy chủ. Không cài gì lên máy quầy, và mở được trên điện thoại.

**Máy trạm** là phần cài lên từng PC khách. Nó gồm hai thứ chạy song song:
- **Giao diện** — thanh dọc dán mép phải màn hình, khách đăng nhập và dùng ở đây.
- **Dịch vụ Windows** — chạy nền 24/7, kể cả khi không ai đăng nhập. Nó gửi nhịp tim, nhận lệnh điều khiển từ xa, và canh cho giao diện luôn sống.

Tách làm hai là có lý do: nếu chỉ có giao diện thì máy trống (chưa ai đăng nhập) sẽ không có kết nối nào tới máy chủ, và nhân viên **không tắt hay khoá được máy trống** — đúng lúc cần nhất.

## Dữ liệu đi lối nào

| Việc | Đường đi |
|---|---|
| Máy trạm báo còn sống, nhiệt độ, %CPU/RAM | `POST /api/machines/by-code/{mã máy}/heartbeat`, mỗi **15 giây** |
| Nhân viên bấm tắt/khoá/chụp màn hình | Trang quản trị → máy chủ → **WebSocket** → máy trạm |
| Máy trạm trả ảnh chụp, danh sách tiến trình | **HTTP**, không phải WebSocket — chiều lên của WebSocket giới hạn 4 KB, ảnh thì vài trăm KB |
| Số dư đổi, đơn mới, tin nhắn | **WebSocket** đẩy tới mọi thiết bị đang mở, không cần tải lại trang |

Máy im lặng quá **45 giây** (lỡ 3 nhịp) thì hệ thống đánh dấu là ngoại tuyến.

---

# 1. Dựng máy chủ

Hai đường. Docker là đường nên đi.

## 1.1. Cần gì trước

| Thứ | Bản |
|---|---|
| PostgreSQL | 15 trở lên (Docker dùng 16) |
| Docker + Docker Compose | bản nào cũng được, miễn có `docker compose` |

Nếu tự biên dịch thay vì dùng Docker thì cần thêm **Go 1.25+**, **Node 20.19+**, **pnpm 8.6+**.

## 1.2. Cách 1 — Docker (khuyến nghị)

### Bước 1: điền cấu hình

```bash
cd core
cp .env.docker.example .env
```

Mở `.env` và điền **ba** giá trị. Compose từ chối khởi động nếu thiếu bất kỳ giá trị nào trong ba:

```bash
DB_PASSWORD=<mật khẩu database, tự đặt>

# Ít nhất 32 ký tự. Sinh bằng:  openssl rand -base64 48
JWT_SECRET=<chuỗi bí mật>

# Địa chỉ công khai của máy chủ. KHÔNG được dùng "*"
ALLOWED_ORIGINS=http://192.168.1.10:20800
```

> **Lưu ý về `.env.docker.example`:** phần cuối tệp mẫu này còn sót hướng dẫn cũ nói máy trạm cần `VNET_AGENT_TOKEN` cấp từ nút "Cấp khoá". **Bỏ qua đoạn đó** — cơ chế khoá máy trạm đã bị gỡ khỏi hệ thống. Cách cấu hình máy trạm đúng nằm ở [phần 2](#2-cài-máy-trạm-lên-pc-khách).

### Bước 2: khởi động

```bash
docker compose up -d --build
```

Lần đầu mất vài phút vì phải biên dịch cả trang quản trị lẫn máy chủ.

### Bước 3: tạo dữ liệu ban đầu

**Thứ tự này bắt buộc, không đảo được.** Máy chủ tự dựng bảng lúc khởi động lần đầu; công cụ `migrate` giả định các bảng đã có sẵn. Chạy `migrate` trước khi máy chủ lên là thất bại.

```bash
docker compose --profile tools run --rm migrate
docker compose --profile tools run --rm seed
```

Compose đã tự ép đúng thứ tự — cả hai lệnh đều chờ máy chủ báo khoẻ mới chạy.

Muốn có thực đơn mẫu 100 món kèm ảnh để thử nghiệm:

```bash
docker compose --profile tools run --rm seed -menu
```

Đây là dữ liệu **để thử**, không phải dữ liệu quán. Đừng chạy trên máy chủ đang phục vụ khách thật.

### Bước 4: đăng nhập và đổi mật khẩu

Mở `http://<địa chỉ máy chủ>:20800`.

`seed` tạo sẵn ba tài khoản, **cả ba dùng chung mật khẩu `admin123`**:

| Tài khoản | Vai trò |
|---|---|
| `admin` | Chủ — toàn quyền |
| `manager` | Quản lý |
| `staff` | Nhân viên |

> **Đổi cả ba mật khẩu ngay sau khi đăng nhập lần đầu.** Mật khẩu này công khai trong mã nguồn. Vào **góc trên phải → Trung tâm người dùng → Đổi mật khẩu**.

### Vận hành hằng ngày

```bash
docker compose logs -f app        # xem nhật ký
docker compose restart app        # khởi động lại máy chủ
docker compose down               # dừng, GIỮ dữ liệu
docker compose down -v            # dừng và XOÁ SẠCH database
```

`down -v` xoá cả ổ đĩa chứa database. Không có bước hỏi lại.

## 1.3. Cách 2 — Tệp thực thi

```bash
cd core
bash scripts/build-server.sh v1.0.0
```

Kịch bản này nhận **đúng một tham số** là số hiệu phiên bản (bỏ trống thì thành `dev`). Nó dựng trang quản trị, chép kết quả vào thư mục nhúng, rồi biên dịch ra `backend/vnet-server`.

Chạy:

```bash
cd backend
export DB_HOST=localhost DB_USER=vnet DB_PASSWORD=... DB_NAME=vnet
export JWT_SECRET=<ít nhất 32 ký tự>
export GIN_MODE=release ALLOWED_ORIGINS=http://192.168.1.10:20800
./vnet-server
```

Rồi tạo dữ liệu ban đầu, vẫn đúng thứ tự đó:

```bash
go run ./cmd/migrate
go run ./cmd/seed
```

> **Repo không có tệp dịch vụ systemd.** Muốn máy chủ tự chạy lại sau khi khởi động máy thì phải tự viết unit.

## 1.4. Biến môi trường

Máy chủ **không tự đọc tệp `.env`**. Các tệp `.env` trong repo chỉ để tham khảo hoặc để Docker Compose đọc; chạy tay thì phải `export` hoặc truyền qua trình quản lý dịch vụ.

### Máy chủ

| Biến | Mặc định | Ý nghĩa |
|---|---|---|
| `SERVER_HOST` | `0.0.0.0` | Địa chỉ lắng nghe |
| `SERVER_PORT` | `8080` | Cổng |
| `GIN_MODE` | `debug` | Đặt `release` khi chạy thật — bật các luật an toàn ở mục 1.5 |
| `SERVER_READ_TIMEOUT` | `30s` | Hạn đọc yêu cầu |
| `SERVER_WRITE_TIMEOUT` | `30s` | Hạn ghi trả lời |
| `MAX_FILE_SIZE` | `5242880` (5 MB) | Trần dung lượng một tệp tải lên |
| `UPLOAD_DIR` | `uploads` | Thư mục chứa ảnh sản phẩm, ảnh giấy tờ |
| `BACKUP_DIR` | `backups` | Thư mục chứa tệp sao lưu |
| `HARDWARE_HISTORY_DAYS` | `7` | Số ngày giữ lịch sử phần cứng. `0` = giữ tất cả |
| `ALLOWED_ORIGINS` | `*` khi debug, **rỗng** khi release | Danh sách địa chỉ được phép gọi API, phân tách bằng dấu phẩy |

### Database

| Biến | Mặc định |
|---|---|
| `DB_HOST` | `localhost` |
| `DB_PORT` | `5432` |
| `DB_USER` | `vnet` |
| `DB_PASSWORD` | `vnet` |
| `DB_NAME` | `vnet` |
| `DB_SSLMODE` | `disable` |
| `DB_MAX_IDLE_CONNS` | `10` |
| `DB_MAX_OPEN_CONNS` | `100` |
| `DB_CONN_MAX_LIFETIME` | `1h` |
| `DB_LOG_LEVEL` | `warn` |

Múi giờ cố định `Asia/Ho_Chi_Minh`, không đổi được qua biến môi trường.

### Đăng nhập

| Biến | Mặc định | Ý nghĩa |
|---|---|---|
| `JWT_SECRET` | `change-me-in-production` | Chuỗi ký thẻ đăng nhập. **Đổi nó là đăng xuất toàn bộ người đang dùng** |
| `JWT_ACCESS_TTL` | `24h` | Thẻ truy cập sống bao lâu |
| `JWT_REFRESH_TTL` | `168h` (7 ngày) | Thẻ làm mới sống bao lâu |
| `JWT_ISSUER` | `vnet` | Tên bên phát hành, ghi trong thẻ |

### Đừng mất công chỉnh

Hai nhóm sau **có trong tệp cấu hình nhưng không dòng mã nào đọc**. Đặt giá trị gì cũng không có tác dụng:

- `MAX_UPLOAD_SIZE` — trần thật sự là `MAX_FILE_SIZE`.
- `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`, `REDIS_DB` — hệ thống không dùng Redis. Không cần cài Redis.

## 1.5. Ba luật khiến máy chủ từ chối khởi động

Khi `GIN_MODE=release`, máy chủ tự kiểm ba điều và **thoát ngay** nếu sai. Đây là nguyên nhân "máy chủ không lên" hay gặp nhất — xem nhật ký sẽ thấy dòng bắt đầu bằng `Refusing to start:`.

| Điều kiện | Thông báo | Cách sửa |
|---|---|---|
| `JWT_SECRET` còn là `change-me-in-production` | `JWT_SECRET is still the development placeholder` | Đặt chuỗi bí mật thật |
| `JWT_SECRET` ngắn hơn 32 ký tự | `must be at least 32 characters` | Dùng `openssl rand -base64 48` |
| `ALLOWED_ORIGINS` rỗng | `must list the admin origins explicitly` | Điền địa chỉ công khai |
| `ALLOWED_ORIGINS` chứa `*` | `must not contain "*"` | Liệt kê từng địa chỉ cụ thể |

Ở chế độ `debug` cả bốn luật đều tắt — tiện khi phát triển, nhưng đừng chạy quán bằng chế độ đó.

## 1.6. Cổng và mạng

| Thứ | Cổng |
|---|---|
| Máy chủ (giao diện + API) | `8080`, đổi bằng `SERVER_PORT` hoặc `APP_PORT` trong Docker |
| PostgreSQL | `5432` — trong Docker **không** mở ra ngoài |
| Trang quản trị khi phát triển | `20900` |

Đổi `APP_PORT` thì **phải sửa `ALLOWED_ORIGINS` cho khớp**, nếu không trình duyệt sẽ bị chặn khi gọi API.

---

# 2. Cài máy trạm lên PC khách

## 2.1. Tạo bộ cài

```bash
cd core
bash scripts/build-client.sh v1.0.0
```

Kịch bản nhận **hai** tham số: phiên bản (mặc định `dev`) và danh sách kiến trúc (mặc định `amd64 arm64`). Chỉ cần một kiến trúc:

```bash
bash scripts/build-client.sh v1.0.0 amd64
```

Kết quả:

| Tệp | Là gì |
|---|---|
| `client/dist/vnet-client-amd64.exe` | Bản chạy được, chép thẳng sang máy Windows |
| `client/dist/vnet-client-arm64.exe` | Bản cho máy ARM |
| `client/vnet-client-windows-v1.0.0.zip` | Cả hai, đóng gói sẵn |
| `dist/vnet-client-setup-v1.0.0.exe` | Bộ cài — **chỉ sinh ra khi biên dịch trên Windows** có sẵn Inno Setup |

Cuối lệnh in ra mã băm SHA-256 của từng tệp `.exe`. Giữ lại: chuỗi này phải dán vào ô **Băm SHA-256** khi công bố bản cập nhật ở trang **Cập nhật máy khách**.

## 2.2. Mã máy là tên máy Windows, không phải khai

Máy trạm lấy **tên máy Windows** làm mã máy và gửi lên ở mỗi nhịp tim. Máy chủ gặp mã chưa có thì **tự thêm máy mới**, kèm thông báo trên trang Máy. Vậy nên:

- Hệ **diskless**: đặt tên máy ở máy chủ boot (`MAY01`, `MAY02`...) là xong. Cài VNET một lần vào bản gốc, không khai gì thêm.
- Máy có ổ cứng: đặt tên máy trong Windows (Settings → System → About → Rename this PC) trước khi cài.

Tên máy tối đa 256 ký tự, chữ cái, số và dấu gạch. Máy đã xoá trên trang quản trị mà vẫn còn cắm thì ở nhịp tim kế tiếp sẽ được thêm lại như máy mới.

**Máy tự thêm chưa có nhóm**, nên hiện nhãn vàng "Chưa gán nhóm — 0₫/giờ" và **không ai đăng nhập được** trên máy đó: màn hình khoá báo *máy chưa được thêm vào hệ thống — báo quầy xếp nhóm cho máy này*. Sau khi cả phòng máy đã lên, vào **Quản lý → Máy**, bấm **Sửa** từng máy hoặc dùng **Tạo hàng loạt** từ trước để xếp nhóm. Nhóm quyết định giá, nên bước này không được bỏ.

**Tạo hàng loạt** vẫn dùng được nếu muốn xếp nhóm sẵn trước khi cắm máy: vào **Quản lý → Máy → Tạo hàng loạt**:

| Ô | Ví dụ |
|---|---|
| **Tiền tố** | `MAY` |
| **Khoảng số** | 1 → 50 |
| **Số chữ số** | 2 (cho ra `MAY01`, không phải `MAY1`) |
| **Nhóm máy** | VIP hoặc Thường |

Mã tạo ở đây phải trùng với tên máy Windows. Hệ thống **kiểm trước và báo ngay** có tạo được hay không, vướng một mã thì không máy nào được tạo. Vướng ở đây chỉ có một kiểu: mã đã có máy đang dùng. Mã của máy đã xoá tạo lại được bình thường.

Một lần tạo tối đa **500 máy**.

## 2.3. Cài bằng bộ cài

Chạy `vnet-client-setup-*.exe` với quyền quản trị. Trình cài hỏi:

| Ô | Điền gì |
|---|---|
| **Địa chỉ máy chủ** | `http://192.168.1.10:20800` — đúng địa chỉ và cổng máy chủ trong mạng LAN |
| **Tài khoản quản trị máy trạm** | Tên đăng nhập cho nhân viên kỹ thuật, mặc định `admin` |
| **Mật khẩu**, **Nhập lại mật khẩu** | Ít nhất 6 ký tự, không có dấu nháy kép. Đặt chung một mật khẩu cho cả quán |

Mã máy không hỏi: máy trạm dùng tên máy Windows (mục 2.2).

**Tài khoản quản trị máy trạm là đường vào chắc chắn có khi mất mạng.** Nhân viên kỹ thuật gõ nó vào chính ô đăng nhập trên màn hình khoá. Máy mở ở chế độ bảo trì: không mở phiên, không tính tiền, không bị khoá hay khởi động lại vì mất kết nối. Máy trạm kiểm tài khoản này ngay tại máy, không hỏi máy chủ.

**Đổi tài khoản cho cả quán** ở trang quản trị, **Cài đặt → Máy trạm**. Mọi máy nhận tài khoản mới ở nhịp tim kế tiếp, khoảng 15 giây, và bỏ tài khoản đặt lúc cài. Xoá tên tài khoản ở đó thì các máy quay về tài khoản đặt lúc cài.

Trình cài làm ba việc, theo thứ tự: ghi tệp cấu hình, băm mật khẩu rồi lưu, đăng ký dịch vụ Windows. Tệp `config.json` chỉ chứa băm, không chứa mật khẩu.

## 2.4. Cài tay

Chép `vnet-client-amd64.exe` vào một thư mục, ví dụ `C:\Program Files\VNET Client\`, rồi tạo tệp `config.json` **cạnh tệp .exe**:

```json
{
  "server_url": "http://192.168.1.10:20800"
}
```

Muốn mã máy khác tên máy Windows thì thêm `"machine_code": "PC-01"`; bình thường không cần.

Rồi mở Command Prompt với quyền quản trị:

```
vnet-client.exe --set-admin-user admin --set-admin-pass "MatKhauKyThuat"
vnet-client.exe --install-service
```

**Cấu hình nằm trong tệp, không phải biến môi trường.** Lý do: dịch vụ Windows chạy ở phiên số 0 và **không thừa hưởng biến môi trường của người đang đăng nhập**. Đặt `VNET_SERVER_URL` trong Windows thì giao diện đọc được còn dịch vụ nền thì không, và máy sẽ hoạt động nửa vời theo cách rất khó truy.

**Bản phát hành bỏ qua hẳn biến môi trường.** `VNET_SERVER_URL`, `VNET_MACHINE_CODE`, `VNET_GUARD` chỉ còn tác dụng ở bản tự build không gắn số phiên bản (`dev`). Lý do: giao diện chạy bằng tài khoản của người ngồi máy, mà biến môi trường của mình thì tài khoản nào cũng tự đặt được.

Bỏ trống mã máy (cách thường) thì hệ thống lấy **tên máy Windows** làm mã.

## 2.5. Cờ dòng lệnh

| Cờ | Việc |
|---|---|
| `--install-service` | Đăng ký dịch vụ Windows |
| `--uninstall-service` | Gỡ dịch vụ Windows |
| `--service` | Chạy như tiến trình nền (Windows tự gọi, không gõ tay) |
| `--set-admin-user <tên> --set-admin-pass <mật khẩu>` | Đặt tài khoản quản trị máy trạm: băm mật khẩu rồi ghi vào `config.json` |
| `--ensure-service` | Bật lại dịch vụ nếu nó đang dừng (tác vụ theo lịch tự gọi) |
| `--window order\|support\|topup` | Mở một cửa sổ phụ |
| *(không cờ)* | Mở giao diện chính — thanh dọc dán mép phải màn hình |

> **`--watch` không còn tồn tại.** Chế độ này đã bị gỡ. Tài liệu cũ trong `AGENTS.md` và phần chú thích đầu `scripts/build-client.sh` vẫn nhắc tới nó — bỏ qua.

## 2.6. Vì sao dịch vụ khó tắt

Dịch vụ nền được giữ sống bằng **ba lớp**, cố ý chồng lên nhau:

1. **Windows tự bật lại** sau 5 giây, 5 giây, rồi 15 giây nếu dịch vụ chết.
2. **Người dùng thường được cấp quyền BẬT** dịch vụ (không được dừng, không được sửa) — để nhân viên khắc phục được mà không cần tài khoản quản trị.
3. **Hai tác vụ theo lịch**: một chạy mỗi phút, bắt đầu ngay từ lúc cài; một bắt sự kiện Windows số 7036 báo dịch vụ đã dừng.

Lớp 3 cần thiết vì lớp 1 **không kích hoạt** khi ai đó dừng dịch vụ một cách sạch sẽ bằng `sc stop` hoặc bảng Services — Windows coi đó là dừng có chủ ý, không phải sự cố. Trên máy thật, dừng dịch vụ và tắt giao diện cùng lúc thì tác vụ mỗi phút bật lại cả hai trong vòng một phút.

> **Trên Windows 11, tác vụ theo sự kiện không bao giờ kích**: `sc stop` không ghi sự kiện 7036 vào nhật ký System. Tác vụ mỗi phút mới là lớp đáng tin.

Muốn gỡ hẳn thì chạy `--uninstall-service`, hoặc gỡ qua Control Panel — bộ gỡ cài đặt tự làm việc đó trước khi xoá tệp.

### Khi các lớp trên không cứu được

Dịch vụ nền và giao diện canh lẫn nhau, và hành động cuối cùng là **khởi động lại máy** (đổi thành tắt máy ở **Cài đặt → Máy trạm**):

| Chuyện xảy ra | Máy trạm làm gì |
|---|---|
| Dịch vụ nền bị vô hiệu hoá, giao diện không bật lại được nó | Khởi động lại sau khoảng 30 giây |
| Giao diện treo, không phản hồi 15 giây | Dịch vụ tắt giao diện và bật bản mới; tới lần thứ ba trong 5 phút thì khởi động lại máy |
| Phiên kết thúc mà giao diện không về màn hình khoá trong 10 giây | Dịch vụ bật lại giao diện |
| Mất kết nối máy chủ 60 giây, đang có hội viên đăng nhập | Khoá màn hình "Mất kết nối máy chủ"; có lại thì tự mở nếu phiên còn |
| Mất kết nối máy chủ 5 phút, đang có hội viên đăng nhập | Khởi động lại |

Hai ngưỡng 60 giây và 5 phút chỉnh được ở **Cài đặt → Máy trạm**. Máy chưa ai đăng nhập, nhân viên đang đăng nhập, và chế độ bảo trì (đăng nhập bằng tài khoản quản trị máy trạm hoặc tệp `maintenance.flag`) thì không bị khoá hay khởi động lại vì mất kết nối.

> **Trước khi tắt máy chủ để bảo trì quá 5 phút**, nhớ rằng mọi máy đang có khách sẽ khởi động lại. Làm lúc quán vắng, hoặc tạm đặt ngưỡng khởi động lại về 0.

## 2.6a. Tài khoản Windows cho khách, và máy diskless

**Khách phải dùng tài khoản Windows THƯỜNG, không phải Administrator.** Mọi lớp bảo vệ ở trên dựa vào việc người ngồi máy không có quyền quản trị: tài khoản thường không dừng được dịch vụ, không sửa được `config.json`, không tạo được tệp cờ bảo trì — đã kiểm trên máy thật, cả ba đều bị Windows từ chối. Tài khoản quản trị thì làm được hết, và không phần mềm nào chặn nổi.

Trang **Máy** hiện nhãn đỏ **TK Windows quản trị** cho máy nào đang chạy bằng tài khoản quản trị; tích **Chỉ máy có cảnh báo** để lọc ra.

Với hệ thống **diskless**, làm trên bản gốc (super client):

1. Tạo tài khoản thường, ví dụ `Khach`, và **đăng nhập vào nó một lần** để qua màn hình thiết lập lần đầu của Windows ("Choose privacy settings..."). Bỏ qua bước này thì màn hình đó che màn hình khoá VNET ở mọi máy.
2. Bằng tài khoản quản trị, cài VNET (mục 2.3 hoặc 2.4).
3. Đặt Windows tự đăng nhập vào tài khoản `Khach` (mục 2.6b).
4. Lưu bản gốc.

Máy diskless khởi động lại là về đúng bản gốc, nên "khởi động lại" ở bảng trên cũng là cách xoá mọi thứ người ngồi máy đã đổi trong phiên đó.

## 2.6b. Để Windows tự đăng nhập

Quán thường để Windows tự đăng nhập một tài khoản dùng chung, còn ai được dùng máy thì màn hình khoá VNET quyết định. Muốn vậy phải bật `AutoAdminLogon` **và tắt chế độ đăng nhập không mật khẩu** của Windows 11 — chế độ này bật sẵn và làm `AutoAdminLogon` mất tác dụng: máy bật lên nằm ở màn hình đăng nhập Windows, và giao diện VNET không lên được.

```
reg add "HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion\PasswordLess\Device" /v DevicePasswordLessBuildVersion /t REG_DWORD /d 0 /f
```

Rồi mở `netplwiz`, bỏ tích **Users must enter a user name and password**, nhập mật khẩu tài khoản dùng chung.

## 2.6c. Nhật ký máy trạm

| Tệp | Của |
|---|---|
| `C:\Program Files\VNET Client\logs\service.log` | Dịch vụ nền: nối máy chủ, chặn ứng dụng, tự cập nhật, lớp canh |
| `%LOCALAPPDATA%\VNET\logs\ui.log` | Thanh điều khiển của tài khoản Windows đang đăng nhập |
| `%LOCALAPPDATA%\VNET\logs\ui-order.log`, `ui-support.log`, `ui-topup.log` | Các cửa sổ phụ |

Quá 5 MB thì tệp cũ đổi tên thành `.1` lúc khởi động.

## 2.7. Kiểm tra máy đã nối chưa

Vào trang **Máy** trên trang quản trị. Máy vừa cài phải:

1. Hiện trong danh sách với mã đúng bằng tên máy Windows.
   Bấm **Sửa** máy đó sẽ thấy CPU, GPU, RAM, ổ đĩa, hệ điều hành và danh sách **thiết bị ngoại vi** (màn hình, bàn phím, chuột, âm thanh) do máy tự báo ở nhịp tim đầu tiên.
2. Trạng thái chuyển từ **Ngoại tuyến** sang **Sẵn sàng** trong vòng 15 giây.
3. Cột **Lần cuối heartbeat** cập nhật liên tục.

Không thấy đổi thì xem [phần D](#d-vận-hành-và-sự-cố).

---

# 3. Máy trạm dưới mắt khách

## 3.1. Màn hình khoá và cách đăng nhập

Máy chưa ai dùng thì hiện màn hình khoá phủ kín. Ngay dưới logo là **tên máy**, đúng mã trên trang Máy, để khách báo quầy "máy nào". Có bốn đường vào:

| Đường | Ai dùng |
|---|---|
| **Tên tài khoản + mật khẩu** | Cả khách lẫn nhân viên — chung một ô |
| **Quét mã QR** | Hội viên, quét mã trên điện thoại |
| **Tài khoản quản trị máy trạm** | Nhân viên kỹ thuật, gõ vào chính ô đăng nhập. Mở chế độ bảo trì, **dùng được cả khi mất mạng** (mục 2.3) |
| **Đăng nhập ngoại tuyến** | Chỉ khi máy chủ không với tới được, và **chỉ tài khoản nhân viên** từng đăng nhập trên máy đó |

Máy chưa được thêm vào hệ thống hoặc chưa xếp nhóm thì khách đăng nhập sẽ bị báo *máy chưa được thêm vào hệ thống — báo quầy*. Nhân viên và tài khoản quản trị máy trạm vẫn vào được.

**Ô đăng nhập chỉ có một, nhưng tên tài khoản quyết định chuyện gì xảy ra:**

- Tên **có trong danh sách nhân viên** → vào như nhân viên. **Không mở phiên, không tính tiền.** Dùng để kiểm tra máy, cài game, dọn dẹp.
- Tên **không có** → vào như hội viên. **Mở phiên và bắt đầu tính tiền ngay.**
- Trùng tên giữa hai bên thì **nhân viên thắng**.

Đây là chỗ dễ nhầm nhất khi vận hành: đặt tên hội viên trùng tên nhân viên thì khách đó chơi miễn phí.

Đăng nhập ngoại tuyến cố ý **chỉ chấp nhận tài khoản nhân viên**. Cho phép hội viên đăng nhập ngoại tuyến nghĩa là khách chỉ cần rút dây mạng là tự mở được máy.

## 3.2. Sáu ô chức năng

Sau khi đăng nhập, khách thấy màn hình chính với các ô:

| Ô | Ai thấy | Việc |
|---|---|---|
| **Nạp tiền** | Chỉ hội viên | Gửi yêu cầu nạp lên quầy, số tiền chọn từ danh sách mệnh giá cấu hình ở **Cài đặt → Nạp tiền** |
| **Giờ chơi** | Chỉ nhân viên | Hiện dòng "Phiên quản trị — không tính giờ" |
| **Đồ ăn** | Mọi người trừ nhân viên | Xem thực đơn, đặt món. Đơn nhảy thẳng lên trang **Đơn hàng** |
| **Hỗ trợ** | Mọi người | Nhắn tin với quầy |
| **Điểm danh** | Chỉ hội viên | Chỉ hiện khi bật **Cài đặt → Tính năng → Cho phép điểm danh** |
| **Đánh giá** | Mọi người trừ nhân viên | Chỉ hiện khi bật **Cài đặt → Tính năng → Cho phép đánh giá dịch vụ** |

Ngoài ra trên màn hình chính luôn có: **đồng hồ phiên chơi** đếm ngược tới lúc hết tiền, **số dư** (cả tiền thật lẫn điểm thưởng), và **chuông thông báo**.

## 3.3. Khi máy khởi động lại giữa phiên

Hệ thống phân biệt hai trường hợp, và xử lý khác nhau:

- **Chỉ giao diện khởi động lại** (khách lỡ tắt, phần mềm treo) → **nối lại phiên cũ**. Tiền đã trả rồi; chặn khách vào lại là nhốt họ ngoài chính cái máy họ đang thuê.
- **Cả máy đã khởi động lại** → đóng phiên cũ, mở phiên **mới**.

Phân biệt bằng thời điểm máy bật, không phải bằng thời gian trôi qua.

## 3.4. Trang Cài đặt trên máy trạm

Khách vào được: **đổi mật khẩu**.

Chỉ nhân viên thấy: **địa chỉ máy chủ** và **mã máy**, chỉ để xem. Muốn đổi thì sửa `config.json` cạnh `vnet-client.exe` (cần quyền quản trị) rồi khởi động lại máy. Địa chỉ máy chủ chỉ có một nguồn là tệp đó, nơi dịch vụ nền cũng đọc; bản cũ cho sửa ở giao diện và làm giao diện với dịch vụ nền nói chuyện với hai máy chủ khác nhau.

**Mã máy không sửa được từ giao diện.** Nó chỉ có một nguồn duy nhất là tệp cấu hình do trình cài ghi. Cho sửa hai nơi là mở đường cho hai nơi lệch nhau.

---

# A. Việc thường ngày

Phần này viết theo việc, không theo màn hình. Mỗi mục là các bước bấm thật.

## A.1. Bắt đầu ca làm

1. Đăng nhập trang quản trị.
2. Vào **Vận hành → Ca làm việc**, bấm **Mở ca**.
3. Nhập **Tiền đầu ca** — số tiền mặt thật sự đang có trong ngăn kéo lúc này.

Một người chỉ được mở **một** ca tại một thời điểm. Chưa đóng ca cũ thì không mở được ca mới.

## A.2. Khách vào quán chơi

**Cách thường: khách tự đăng nhập ở máy.** Khách ngồi vào máy, nhập tài khoản và mật khẩu trên màn hình khoá. Phiên chơi mở ra và bắt đầu tính tiền ngay. Nhân viên không phải làm gì — trạng thái máy tự chuyển sang **Đang sử dụng** trên trang quản trị.

**Khách bấm Đăng xuất là trả máy**: phiên kết thúc, tiền dừng trừ, máy về **Sẵn sàng** cho người sau.

**Cách thứ hai: nhân viên mở hộ từ quầy.**

1. Vào **Quản lý → Phiên**, bấm **Mở máy**.
2. Chọn máy và chọn hội viên.
3. Nếu khách có mua gói, chọn ở ô **Gói cước (tuỳ chọn)**.
4. Bấm **Tính thử** để xem trước ước tính phí nếu muốn.
5. Xác nhận.

**Khách chưa có tài khoản?** Vào **Quản lý → Hội viên → Thêm**, tạo tài khoản rồi nạp tiền trước.

Ba điều kiện phải thoả, nếu không hệ thống từ chối mở máy:

- Máy đang **Sẵn sàng** — không phải đang có người, không phải Tạm ngừng.
- Hội viên còn tiền, hoặc số nợ chưa vượt trần cấu hình ở **Cài đặt → Giới hạn → Nợ tối đa**. Nếu có đặt **Tiền tối thiểu mỗi lần đăng nhập**, số dư phải đủ mức đó.
- Không vướng giới nghiêm, nếu khách là vị thành niên.

**Tiền tối thiểu mỗi lần đăng nhập** (**Cài đặt → Giới hạn**). Tiền giờ tính theo phút đã chơi trọn, nên khách đăng nhập rồi thoát trong vòng một phút không mất đồng nào. Đặt mức này thì đăng nhập là trừ ngay số đó. Nó trả trước cho những phút đầu chứ không cộng thêm:

| Giá 10.000₫/giờ, mức tối thiểu 3.000₫ | Khách trả |
|---|---|
| Thoát sau 30 giây | 3.000₫ |
| Chơi 10 phút (theo phút là 1.667₫) | 3.000₫ |
| Chơi 30 phút (theo phút là 5.000₫) | 5.000₫ |

Không áp cho phiên dùng gói cước và máy chưa có giá. Để 0 là tắt.

**Một tài khoản chỉ chơi được một máy.** Cố mở máy thứ hai thì hệ thống báo rõ khách đang ngồi máy nào.

## A.3. Nạp tiền cho khách

**Khách đưa tiền mặt ở quầy:**

1. Vào **Quản lý → Hội viên**, tìm khách.
2. Bấm **Nạp** ở dòng của họ.
3. Nhập số tiền, chọn **Phương thức** — Tiền mặt, Chuyển khoản, hoặc Ví điện tử.
4. Xác nhận.

Số dư đổi ngay trên màn hình máy khách, không cần họ tải lại gì.

**Khách bấm nút Nạp tiền trên máy:**

1. Yêu cầu hiện ở **Kinh doanh → Đơn hàng** dưới dạng đơn loại **Nạp tiền**, trạng thái **Chờ xác nhận**, kèm thông báo nổi ở góc màn hình.
2. Thu tiền của khách.
3. Bấm **Duyệt nạp**.

Từ chối thì bấm **Từ chối**.

> **Hai thiết bị cùng bấm duyệt không cộng tiền hai lần.** Hệ thống khoá đơn lại khi xử lý; thiết bị thứ hai nhận câu "đơn đã được xử lý". Nhân viên có thể vừa mở trang quản trị trên điện thoại vừa mở trên máy tính mà không sợ nhầm.

## A.4. Khách gọi đồ ăn

**Khách tự đặt trên máy:** đơn hiện ở **Kinh doanh → Đơn hàng**, trạng thái **Chờ xác nhận**.

**Nhân viên tạo hộ:** vào **Đơn hàng → Thêm**, chọn máy và các món.

Rồi xử lý ba bước:

1. **Xác nhận** — hệ thống **trừ tồn kho** ngay lúc này. Món có công thức thì trừ theo nguyên liệu chứ không trừ món.
2. **In phiếu chế biến** — phiếu chạy ra máy in ở đúng bộ phận, theo phân luồng cấu hình ở trang **Máy in**. Món không gán máy in nào thì **không có phiếu**.
3. **Thanh toán** — chọn Tiền mặt, **Trừ số dư hội viên**, hoặc Chuyển khoản. Đơn chuyển sang **Hoàn thành**.

Muốn theo dõi bếp thì đổi trạng thái từng món: **Chờ làm → Đang làm → Xong → Đã phục vụ**.

**Huỷ đơn đã xác nhận thì tồn kho tự hoàn lại.**

> **Đơn đã xác nhận hoặc đã hoàn thành không xoá được.** Bấm xoá sẽ nhận thông báo bảo dùng chức năng huỷ. Đây là cố ý: xoá một đơn đã thu tiền làm lệch cả báo cáo doanh thu lẫn đối soát ca.

## A.5. Bán gói dịch vụ

1. Vào **Kinh doanh → Gói dịch vụ**, chọn gói, bấm **Mua**.
2. Chọn phương thức thanh toán. Hệ thống **tự tạo tài khoản mới**: tên đăng nhập là tên gói viết liền không dấu cộng số thứ tự (gói "Gói Đêm" → `GoiDem1`, `GoiDem2`…), mật khẩu 6 chữ số — tiện cho khách vãng lai mua gói theo giờ.

> **Mật khẩu tài khoản tự tạo chỉ hiện đúng một lần.** Chép ra trước khi đóng cửa sổ.

3. Khách ngồi vào máy, bấm **Kích hoạt** và chọn máy.

**Gói chỉ bắt đầu trừ giờ từ lúc kích hoạt lên một máy cụ thể, không phải từ lúc mua.** Khách mua sáng, tối mới chơi thì không mất giờ nào.

Hai loại gói:

- **Fixed slot** — khung giờ cố định, ví dụ 20h đến 23h. Trong khung đó không tính thêm tiền theo giờ. Hết khung, hệ thống tự trả máy.
- **Trả trước** — mua sẵn số phút. Hết phút thì tự trả máy.

## A.6. Đặt chỗ trước

1. Vào **Vận hành → Đặt chỗ → Thêm**.
2. Nhập tên khách và số điện thoại, hoặc chọn hội viên có sẵn.
3. Chọn máy và **khoảng giờ giữ máy**.
4. Thu **tiền cọc** nếu cần — cọc trừ thẳng vào số dư hội viên.

Trùng lịch cùng một máy thì hệ thống từ chối ngay.

**Khách đến:** bấm **Check-in**.

**Khách không đến:** hệ thống tự đánh **Không đến** sau **15 phút** kể từ giờ bắt đầu. Bấm tay bằng **Đánh vắng** cũng được.

**Huỷ** thì tiền cọc hoàn lại tự động. Dùng **Huỷ** chứ đừng dùng **Xoá** nếu muốn giữ lịch sử.

## A.7. Khách hết tiền giữa chừng

**Không cần làm gì.** Hệ thống tự lo:

1. Trừ tiền theo số phút đã chơi.
2. Số dư **không đủ trả phút kế tiếp** thì **tự kết thúc phiên ngay khi đồng hồ trên máy khách về 0** (trong khoảng một giây). Phần tiền lẻ chưa đủ một phút vẫn nằm trong tài khoản khách, số dư không bị âm.
3. Máy khách tự đăng xuất về **màn hình khoá có ô đăng nhập**, kèm dòng **"Hết số dư — vui lòng nạp thêm để chơi tiếp"**. Cửa sổ phụ đang mở (thực đơn, hỗ trợ, nạp tiền) tự đóng.
4. Máy chuyển về **Sẵn sàng**. Khách nạp thêm (quầy nạp từ xa) là đăng nhập lại được ngay, **không cần ai bấm Mở khoá**.

Cùng cách đó cho mọi lý do phiên kết thúc: hết gói, giới nghiêm, quầy bấm **Trả máy**. Dòng lý do tự ẩn sau hai phút để khách kế tiếp không đọc nhầm.

Đồng hồ trên máy khách đếm ngược tới đúng thời điểm hết tiền, nên khách thấy trước chứ không bị cắt đột ngột.

**Thứ tự tiêu tiền:** điểm thưởng tiêu **trước**, tiền thật tiêu **sau**.

**Về nợ:** hết tiền giữa chừng không còn làm số dư âm. Nhưng việc trừ tiền vẫn không bao giờ bị từ chối, ví dụ khi giá lên giữa phiên: thiếu tiền thì số dư xuống âm và thành nợ, hiện ở cột **Còn nợ** trang Hội viên. Cửa chặn nợ chồng nợ nằm ở lúc **mở phiên mới**: hệ thống so số nợ với **Cài đặt → Giới hạn → Nợ tối đa** và từ chối nếu vượt.

## A.8. Khách báo lỗi máy

1. Vào **Quản lý → Máy**, tìm máy, mở tab **Điều khiển**.
2. Chọn việc:

| Nút | Việc |
|---|---|
| **Tin nhắn** | Hiện một dòng chữ lên màn hình khách |
| **Chụp màn hình** | Xem khách đang gặp gì |
| **Xem ứng dụng** | Danh sách tiến trình đang chạy kèm mức RAM |
| **Tắt ứng dụng** | Tắt một tiến trình đang treo |
| **Khoá** / **Mở khoá** | Khoá màn hình, kèm lý do khách đọc được |
| **Chặn ứng dụng** / **Bỏ chặn** | Cấm một ứng dụng chạy trên máy đó. Gõ tên tiến trình, ví dụ `chrome.exe`. Dịch vụ nền tắt nó trong vòng hai giây mỗi lần nó được mở, kể cả sau khi máy khởi động lại. Không chặn được tiến trình hệ thống (`explorer`, `svchost`...) và chính máy trạm |
| **Khởi động lại** / **Tắt máy** | **Kết thúc phiên chơi** |

**Máy chưa kết nối thì lệnh không tới nơi** và hệ thống báo rõ — không im lặng nuốt.

**Tắt máy và Khởi động lại kết thúc phiên chơi**, nhưng chỉ khi lệnh đã tới được máy. Lệnh không tới thì phiên giữ nguyên — hệ thống không tính tiền khi chưa chắc khách đã rời máy.

Muốn đổi máy cho khách mà giữ nguyên phiên: vào **Quản lý → Phiên**, bấm **Đổi máy**.

**Máy hỏng cần ngừng nhận khách:** vào **Máy → Sửa**, tắt công tắc **Đang hoạt động**. Máy chuyển sang **Tạm ngừng**, không mở phiên mới được nữa nhưng vẫn nằm trong danh sách để bật lại. **Đừng xoá máy** — xoá là mất toàn bộ lịch sử phiên chơi và doanh thu gắn với nó. Hệ thống sẽ chặn nếu máy còn dữ liệu.

## A.9. Nhận hàng nhập kho

1. Vào **Kinh doanh → Tồn kho**, tạo phiếu.
2. Chọn loại **Nhập**, chọn sản phẩm, nhập số lượng và đơn giá.

Tồn kho cập nhật ngay, và mọi màn hình đang mở tự vẽ lại con số mới.

Hàng hỏng, hàng mất thì tạo phiếu **Xuất**.

## A.10. Kiểm kê cuối tháng

1. Vào **Kinh doanh → Kiểm kê kho**, bấm **Mở phiên kiểm kê**. Ghi chú ví dụ "kiểm kê cuối tháng 8".
2. Bấm **Đếm hàng**. Với mỗi mặt hàng: chọn tên, nhập số đếm thực, bấm **Ghi số đếm**. Màn hình hiện luôn số sổ sách và chênh lệch.
3. Đếm xong, xem dòng tổng kết: bao nhiêu mặt hàng, bao nhiêu dòng lệch, thiếu bao nhiêu, thừa bao nhiêu.
4. Bấm **Chốt phiên**.

> **Khi chốt, hệ thống áp CHÊNH LỆCH lên tồn kho hiện tại, không đặt tồn bằng số đã đếm.** Đếm được 8 lon mà sổ ghi 10 thì hệ thống trừ 2 khỏi tồn *lúc chốt*, chứ không đặt tồn về 8. Nhờ vậy hàng bán ra trong lúc đang đếm không bị mất.

Bấm **Huỷ phiên** thì tồn kho không bị đụng tới.

## A.11. Chốt ca, giao tiền

1. Vào **Vận hành → Ca làm việc**.
2. Bấm **Đóng ca**.
3. Đếm tiền mặt thật trong ngăn kéo, nhập vào ô **Tiền cuối ca**.
4. Hệ thống tính **Tiền dự kiến** và hiện cột **Thừa/Thiếu**.

> **"Tiền dự kiến" chỉ đếm tiền mặt thật sự đi qua ngăn kéo.** Nó cộng: đơn trả tiền mặt, nạp tiền mặt, mua gói bằng tiền mặt, cộng thu và trừ chi của các lần bàn giao. Nó **không** cộng đơn trả bằng số dư hội viên, cũng không cộng phí phiên chơi — hai thứ đó trừ trong tài khoản, không có tờ tiền nào đổi tay.

**Giao tiền giữa ca:** bấm **Bàn giao**, chọn **Thu tiền** hoặc **Chi tiền**, nhập số tiền và lý do. Khoản này được tính vào tiền dự kiến của ca.

> **Không có việc tự động chốt ca.** Quên đóng ca thì ca cứ mở mãi.

## A.12. Xem báo cáo

Vào **Vận hành → Báo cáo**. Bảy loại:

| Báo cáo | Trả lời câu hỏi |
|---|---|
| **Doanh thu ngày** | Hôm nay thu bao nhiêu |
| **Doanh thu tháng** | Xu hướng theo tháng |
| **Theo hội viên** | Ai chi nhiều nhất |
| **Theo máy** | Máy nào chạy nhiều, máy nào ế |
| **Theo nhân viên** | Ai bán được nhiều |
| **Món bán chạy** | Nên nhập thêm gì |
| **Hiệu quả khuyến mãi** | Chương trình nào đáng giữ |

Cần quyền **Xem báo cáo**. Nhân viên thường không thấy trang này.

---

# B. Tra cứu từng màn hình

Menu chia làm bốn nhóm. Trang nào bạn không thấy là do vai trò của bạn không có quyền — xem [phần C](#c-phân-quyền).

## Bảng điều khiển

Trang đầu tiên sau khi đăng nhập. Bốn con số: **Hội viên**, **Máy online**, **Đang chơi**, **Doanh thu hôm nay**. Kèm biểu đồ doanh thu 7 ngày và danh sách phiên đang chạy.

---

## Nhóm Quản lý

*Ai và cái gì đang có trong quán: người, máy, và phiên chơi nối hai thứ đó.*

### Hội viên

**Để làm gì.** Danh sách khách. Tạo tài khoản, nạp tiền, xem lịch sử.

**Thao tác chính.** Thêm · Sửa · **Nạp** · **Hoàn tiền** · **Reset MK** · **Xếp lại hạng** · xem tab **Giao dịch** và **Phiên chơi** của từng người.

Form hội viên có ô **Ngày sinh** (dùng để xác định vị thành niên cho giới nghiêm), **Số CMND/CCCD**, **Ảnh CMND/CCCD**, và **Ảnh giấy đồng ý của phụ huynh**.

**Điều dễ nhầm.**
- **Hai ví, không phải một.** Cột **Số dư** là tiền thật; **Điểm thưởng** là tiền khuyến mãi. Điểm thưởng tiêu trước.
- Cột **Còn nợ** hiện khi số dư âm. Đó không phải lỗi — xem [A.7](#a7-khách-hết-tiền-giữa-chừng).
- Khi hoàn tiền, nhớ tích **Trừ vào số dư khuyến mãi** nếu khoản đó vốn là khuyến mãi.
- **Hội viên đã có giao dịch, phiên chơi hoặc đơn hàng thì không xoá được.** Muốn ngừng phục vụ thì bỏ tích **Kích hoạt**.
- **Xếp lại hạng** chạy tự động mỗi 15 phút; nút này chỉ để chạy ngay.

### Nhóm hội viên

**Để làm gì.** Hạng khách: Đồng, Bạc, Vàng. Mỗi hạng có mức chi tiêu tối thiểu và phần trăm giảm giá.

**Thao tác chính.** Thêm · Sửa · Xoá.

**Điều dễ nhầm.**
- Giảm giá ở đây áp cho **phí phiên chơi**, và cộng thêm với bảng giá theo hạng ở trang Nhóm máy.
- Nhóm đánh dấu **Mặc định** không xoá được — hội viên mới cần một nhóm để rơi vào.
- Nhóm còn hội viên hoặc còn dòng bảng giá thì không xoá được. Chuyển họ sang nhóm khác trước.

### Điểm danh

**Để làm gì.** Khách bấm điểm danh mỗi ngày trên máy trạm để nhận điểm thưởng. Trang này xem ai đã điểm danh và chuỗi ngày liên tiếp của họ.

**Thao tác chính.** Chỉ xem.

**Điều dễ nhầm.**
- Cấu hình phần thưởng ở **Cài đặt**: **Thưởng mỗi ngày**, **Mốc chuỗi (ngày)** mặc định 7, **Thưởng khi chạm mốc**.
- Phải bật **Cài đặt → Tính năng → Cho phép điểm danh**, không thì nút không hiện trên máy khách.
- Thưởng cộng vào **điểm thưởng**, không phải tiền thật.
- Bỏ một ngày là chuỗi về 1.

### Máy

**Để làm gì.** Danh sách PC. Theo dõi trạng thái, xem phần cứng, điều khiển từ xa.

**Thao tác chính.** Thêm · **Tạo hàng loạt** · Sửa · Xoá · bốn tab: **Thông tin**, **Phiên**, **Tài sản**, **Điều khiển**.

**Điều dễ nhầm.**
- **Chỉ có ba trạng thái**: Ngoại tuyến → Sẵn sàng → Đang sử dụng. Không có trạng thái "bảo trì".
- **Cột Cảnh báo** có hai nhãn do máy trạm tự báo. **TK Windows quản trị** (đỏ): tài khoản Windows trên máy đó có quyền quản trị, người ngồi máy gỡ được mọi lớp bảo vệ — xem [mục 2.6a](#26a-tài-khoản-windows-cho-khách-và-máy-diskless). **Có mạng lạ** (vàng): máy vừa có thêm một card mạng (Wi-Fi, điện thoại phát mạng); rê chuột để xem tên và IP. Máy trạm không tự ngắt gì, chỉ báo. Tích **Chỉ máy có cảnh báo** để lọc.
- **Muốn tạm ngừng một máy thì tắt công tắc Đang hoạt động, đừng xoá.** Máy tạm ngừng vẫn hiện trong danh sách để bật lại; máy đã xoá thì mất lịch sử.
- Máy còn phiên chơi, đơn hàng hoặc lịch đặt thì không xoá được.
- Lệnh điều khiển cần máy đang kết nối. Máy chưa nối thì hệ thống báo "chưa kết nối — lệnh không tới nơi", không im lặng.
- **Mã máy phải trùng khớp với mã ghi trong cấu hình máy trạm**, nếu không nhịp tim không khớp vào đâu cả. Vì lý do này, dựng quán mới thì dùng **Tạo hàng loạt** thay vì gõ tay từng cái — xem [mục 2.2](#22-khai-máy-trong-hệ-thống-trước).
- **Tạo hàng loạt là toàn bộ hoặc không có gì.** Vướng một mã thì không máy nào được tạo, và hệ thống báo trước khi bạn bấm Tạo.
- **Máy đã xoá vẫn giữ mã của nó.** Tạo lại đúng mã đó sẽ bị chặn, dù danh sách không hiện máy nào mang mã ấy — thêm một lý do để tắt hoạt động thay vì xoá.

### Nhóm máy

**Để làm gì.** Xếp máy theo loại — VIP, Thường — và **đặt bảng giá**.

**Thao tác chính.** Thêm · Sửa · Xoá · **Bảng giá** với hai tab: **Theo hạng hội viên** và **Theo khung giờ**.

**Điều dễ nhầm.**
- **Thứ tự ưu tiên giá: giá theo hạng hội viên thắng giá theo khung giờ, và cả hai thắng giá cơ bản của nhóm máy.** Đặt cả ba mà không nhớ thứ tự này là nguyên nhân "sao tính tiền không đúng như tôi cấu hình".
- Bảng giá theo hạng có ô **Tối thiểu (phút)** — số phút tối thiểu tính tiền dù khách chơi ít hơn.
- Khung giờ **vắt qua nửa đêm được**, ví dụ 22h–02h.
- Nhóm còn máy, còn bảng giá hoặc còn luật chặn web gắn vào thì không xoá được.

### Tài sản máy

**Để làm gì.** Sổ theo dõi màn hình, bàn phím, chuột, tai nghe, ghế gắn với từng máy.

**Thao tác chính.** Thêm tài sản · **Kiểm tra** (ghi nhận lần kiểm kèm ảnh) · Sửa · Xoá.

**Điều dễ nhầm.** Tình trạng có bốn mức: Tốt, Cũ mòn, Hỏng, Mất. Xoá máy thì tài sản gắn với nó cũng bị xoá theo.

### Phiên

**Để làm gì.** Ai đang ngồi máy nào, chơi bao lâu, còn bao nhiêu.

**Thao tác chính.** **Mở máy** · **Đổi máy** · **Kết thúc** · **Tính thử** (ước tính phí trước khi mở).

**Điều dễ nhầm.**
- Cột **Còn lại** tính từ số dư hiện tại và đơn giá đang áp. Khách nạp thêm thì con số này tự dài ra.
- Kết thúc phiên tay thì tiền tính tới đúng thời điểm bấm.
- Phiên có gói **Fixed slot** không hiện Còn lại theo tiền — nó chạy tới hết khung giờ.

### Giao dịch

**Để làm gì.** Sổ cái mọi lần tiền của hội viên thay đổi. Tra cứu, không sửa được.

**Thao tác chính.** Lọc theo khoảng ngày, theo loại, tìm theo tên hoặc số điện thoại.

**Điều dễ nhầm.**
- Cột **Số dư trước** và **Số dư sau** là công cụ đối chiếu chính khi khách khiếu nại.
- **Cả phiên chơi chỉ có MỘT dòng "Phí chơi"**, cộng dồn dần lên chứ không phải mỗi phút một dòng.
- Nạp bằng điểm thưởng hiện là loại riêng, không lẫn với nạp tiền thật.

---

## Nhóm Kinh doanh

*Bán gì và lấy hàng từ đâu.*

### Đơn hàng

**Để làm gì.** Mọi đơn: đồ ăn, nước, và cả yêu cầu nạp tiền của khách.

**Thao tác chính.** Thêm · Sửa · **Xác nhận** · **Thanh toán** · **Huỷ đơn** · **Tách đơn** · **In hoá đơn** · **In phiếu chế biến** · **Duyệt nạp** / **Từ chối** cho đơn nạp tiền.

**Điều dễ nhầm.**
- **Hai loại đơn dùng chung một trang**: Sản phẩm và Nạp tiền. Cột **Loại** phân biệt.
- Trạng thái đi một chiều: Chờ xác nhận → Đã xác nhận → Hoàn thành. Từ hai trạng thái đầu có thể sang Đã hủy. **Không quay lui được.**
- **Tồn kho trừ ở bước Xác nhận, không phải lúc tạo đơn.**
- **Đơn đã xác nhận hoặc đã hoàn thành không xoá được** — dùng Huỷ.
- Đơn đã có phiếu thanh toán hoặc hoá đơn điện tử thì cũng không xoá được.
- **Chỉ MỘT khuyến mãi được áp cho một đơn.** Nhiều chương trình cùng thoả thì chương trình có **Ưu tiên** cao thắng; bằng nhau thì chương trình giảm nhiều tiền hơn thắng.

### Sản phẩm

**Để làm gì.** Thực đơn và hàng hoá. Cũng là nơi khai nguyên liệu.

**Thao tác chính.** Thêm · Sửa · Xoá · **Nguyên liệu** (công thức) · **Tuỳ chọn** (size, mức đá…) · **Nhập kho** / **Trừ kho** nhanh.

**Điều dễ nhầm.**
- Một sản phẩm là **Bán lẻ** hoặc **Nguyên liệu**. Nguyên liệu không hiện trên thực đơn máy khách.
- Món có công thức thì bán ra **trừ nguyên liệu**, không trừ chính món đó.
- Tắt **Theo dõi tồn kho** thì bán không giới hạn — dùng cho món làm tại chỗ.
- Sản phẩm đã từng bán, đã có phiếu kho hoặc đang là nguyên liệu của món khác thì không xoá được. Bỏ tích kích hoạt để ngừng bán.

### Danh mục

**Để làm gì.** Nhóm món trên thực đơn. Có thể lồng nhau qua **Danh mục cha**.

**Thao tác chính.** Thêm · Sửa · Xoá · đổi **Thứ tự** hiển thị.

**Điều dễ nhầm.** Danh mục còn sản phẩm thì không xoá được. Danh mục có gắn máy in quyết định phiếu chế biến chạy ra đâu.

### Gói dịch vụ

**Để làm gì.** Bán giờ chơi theo gói.

**Thao tác chính.** Thêm · Sửa · Xoá · **Mua** · **Kích hoạt** · xem danh sách **Combo đã mua**.

**Điều dễ nhầm.**
- **Mua khác Kích hoạt.** Trừ giờ bắt đầu từ lúc kích hoạt lên một máy cụ thể.
- **Hiệu lực (ngày)** tính từ lúc mua — mua rồi để quên quá hạn thì gói hỏng dù chưa kích hoạt.
- Gói **Fixed slot** cần khai **Ngày áp dụng** và các **slot** giờ.
- Gói đã có người mua thì không xoá được.

### Nhà cung cấp

**Để làm gì.** Danh bạ nơi nhập hàng.

**Thao tác chính.** Thêm · Sửa · Xoá.

**Điều dễ nhầm.** Nhà cung cấp còn gắn với sản phẩm hoặc còn phiếu kho thì không xoá được.

### Tồn kho

**Để làm gì.** Phiếu nhập và phiếu xuất. Lịch sử mọi lần tồn kho thay đổi.

**Thao tác chính.** Tạo phiếu nhập · Tạo phiếu xuất.

**Điều dễ nhầm.**
- Cột **Trước** và **Sau** cho biết chính xác phiếu đó đã đổi tồn thế nào.
- Bán hàng cũng sinh dòng ở đây, không chỉ phiếu nhập xuất tay.
- Điều chỉnh do kiểm kê hiện dưới loại riêng kèm mã phiên kiểm kê.

### Kiểm kê kho

**Để làm gì.** Đếm hàng thực tế và đối chiếu sổ sách.

**Thao tác chính.** **Mở phiên kiểm kê** · **Đếm hàng** → **Ghi số đếm** từng mặt · **Chốt phiên** · **Huỷ phiên**.

**Điều dễ nhầm.**
- **Chốt phiên áp CHÊNH LỆCH lên tồn hiện tại, không đặt tồn bằng số đã đếm.**
- Huỷ phiên thì tồn kho không bị đụng.
- Phiên rỗng không chốt được.
- Hai người cùng bấm chốt không làm điều chỉnh chạy hai lần.

---

## Nhóm Vận hành

*Việc chạy hằng ngày ở quầy.*

### Ca làm việc

**Để làm gì.** Mở ca, đóng ca, đối soát tiền mặt.

**Thao tác chính.** **Mở ca** · **Đóng ca** · **Bàn giao** (Thu tiền / Chi tiền).

**Điều dễ nhầm.**
- **Tiền dự kiến chỉ đếm tiền mặt đi qua ngăn kéo** — không tính đơn trả bằng số dư, không tính phí phiên chơi.
- Một người một ca mở tại một thời điểm.
- **Không có việc tự động đóng ca.**
- Ca chồng lấn nhau thì quy kết doanh thu chỉ là gần đúng — đơn hàng không mang mã ca.

### Đặt chỗ

**Để làm gì.** Giữ máy trước cho khách.

**Thao tác chính.** Thêm · Sửa · **Check-in** · **Huỷ** · **Đánh vắng** · Xoá.

**Điều dễ nhầm.**
- Trùng lịch cùng máy bị từ chối ngay khi lưu.
- Giới hạn số lượt đặt cấu hình ở **Cài đặt → Giới hạn**.
- **Huỷ hoàn cọc; Xoá thì không.** Muốn giữ lịch sử thì dùng Huỷ.
- Lịch đã check-in hoặc đã thu cọc thì không xoá được.
- Hệ thống tự đánh vắng sau 15 phút.

### Khuyến mãi

**Để làm gì.** Chương trình giảm giá đơn hàng, và cấu hình vòng quay may mắn.

**Thao tác chính.** Thêm · Sửa · Xoá · **Thêm điều kiện** · **Thêm phần thưởng** · quản lý **Ô thưởng vòng quay** · **Quay thử**.

**Điều dễ nhầm.**
- **Sáu khoá điều kiện dùng được**: tổng tiền tối thiểu, số lượng món tối thiểu, nhóm hội viên, thứ trong tuần (0 = Chủ nhật), khung giờ trong ngày, danh mục sản phẩm. Gõ khoá lạ thì hệ thống từ chối lưu — cố ý, vì điều kiện không hiểu được mà vẫn áp là tặng tiền nhầm.
- **Hai loại thưởng**: giảm theo phần trăm (có thể đặt trần), hoặc giảm số tiền cố định.
- **Chỉ một khuyến mãi áp cho một đơn.**
- Vòng quay: **tổng xác suất các ô đang bật không được vượt 100%**. Phần còn lại là quay trượt.
- Khuyến mãi đã áp cho đơn nào thì không xoá được.

### Giới nghiêm

**Để làm gì.** Chặn khách vị thành niên chơi khuya hoặc chơi quá nhiều giờ trong ngày.

**Thao tác chính.** Thêm chính sách theo từng thứ · Sửa · Xoá · **Miễn trừ**.

**Điều dễ nhầm.**
- **Hai luật, hai phạm vi khác nhau.** **Khung giờ cấm** chỉ áp khi đang trong khung. **Giờ chơi tối đa** áp **cả ngày**, bất kể mấy giờ.
- Khung giờ **vắt qua nửa đêm được**, ví dụ 22h–06h.
- Số giờ đã chơi tính cả **phiên đang chạy**, không chỉ phiên đã kết thúc — nếu không thì khách chỉ cần không trả máy là trần giờ mất tác dụng.
- **Miễn trừ bắt buộc ghi lý do** và chỉ có hiệu lực **trong ngày**.
- Vị thành niên xác định từ **Ngày sinh** ở hồ sơ hội viên. Bỏ trống ngày sinh thì giới nghiêm không áp được.
- Hệ thống kiểm mỗi phút và **tự trả máy** khi tới giờ, không đợi nhân viên.

### Thông báo

**Để làm gì.** Soạn thông báo và gửi tới toàn bộ hội viên.

**Thao tác chính.** Thêm · Sửa · Xoá · **Gửi**.

**Điều dễ nhầm.**
- **Soạn khác Gửi.** Thông báo chỉ tới tay khách khi bấm **Gửi**.
- Gửi là gửi cho **tất cả** hội viên, không chọn riêng được.
- Bốn loại: Thông tin, Cảnh báo, Khuyến mãi, Hệ thống.

### Đánh giá dịch vụ

**Để làm gì.** Xem khách chấm bao nhiêu sao và viết gì.

**Thao tác chính.** Lọc theo máy, mức sao, khoảng ngày, hoặc **Chỉ xem có nhận xét**.

**Điều dễ nhầm.**
- Phải bật **Cài đặt → Tính năng → Cho phép đánh giá dịch vụ**.
- **Khách phải đang trong phiên chơi mới đánh giá được** — máy được suy từ phiên chứ không tin theo yêu cầu gửi lên.
- Mỗi đơn hàng chỉ đánh giá được một lần.

### Báo cáo

**Để làm gì.** Bảy báo cáo doanh thu và vận hành. Xem [A.12](#a12-xem-báo-cáo).

**Điều dễ nhầm.** Cần quyền **Xem báo cáo**. Mọi báo cáo lọc theo khoảng ngày Từ–Đến.

---

## Nhóm Hệ thống

*Cấu hình hạ tầng và dấu vết vận hành — thứ nhân viên bình thường không đụng tới hằng ngày.*

### Cài đặt

**Để làm gì.** Cấu hình toàn quán. Sáu tab.

| Tab | Gồm |
|---|---|
| **Chung** | Tên cửa hàng, địa chỉ, SĐT, email, múi giờ |
| **Giới hạn** | Giới hạn đặt chỗ mỗi ngày và mỗi hội viên, thời gian huỷ trước, **Nợ tối đa**, **Tiền tối thiểu mỗi lần đăng nhập** |
| **Hóa đơn** | Tiêu đề, chân trang, mã số thuế in trên hoá đơn |
| **Nạp tiền** | **Mệnh giá nạp** — danh sách số tiền hiện trên máy khách |
| **Tính năng** | Bật/tắt **Cho phép điểm danh** và **Cho phép đánh giá dịch vụ** |
| **Máy trạm** | Máy trạm làm gì khi phần mềm bị tắt (khởi động lại hay tắt máy), mất kết nối bao lâu thì khoá, bao lâu thì khởi động lại, và **tài khoản quản trị máy trạm** cho cả quán (để trống mật khẩu là giữ nguyên). Máy trạm nhận giá trị mới trong khoảng 15 giây |

**Điều dễ nhầm.**
- **Nợ tối đa** là thứ quyết định khách nợ được bao nhiêu trước khi bị chặn mở máy.
- Tắt một tính năng thì nút biến mất trên máy khách **và** máy chủ cũng từ chối nếu ai đó gọi thẳng — không phải chỉ ẩn đi.
- Cần quyền **Sửa cài đặt**.

### Máy in

**Để làm gì.** Khai máy in hoá đơn và máy in bếp, rồi phân luồng món nào in ở đâu.

**Thao tác chính.** Thêm · Sửa · Xoá · **In thử** · **Phân luồng**.

**Điều dễ nhầm.**
- **Món không được gán máy in nào thì không có phiếu chế biến.** Đây là nguyên nhân phổ biến của "bếp không nhận được phiếu".
- Ô **Tiếng Việt** chọn giữa **Bỏ dấu** (chạy trên mọi máy in) và **Có dấu CP1258** (cần máy in hỗ trợ). Chọn sai thì phiếu ra ký tự lạ. Bấm **In thử** trước khi chốt.
- Máy in đang gắn với danh mục nào thì phải gỡ ra trước khi xoá. Máy in **Mặc định** không xoá được.

### Chặn website

**Để làm gì.** Cấm truy cập một số tên miền trên máy khách.

**Thao tác chính.** Hai tab: **Luật chặn** (Thêm/Sửa/Xoá, đặt **Lịch áp dụng** và **Phạm vi máy**) và **Nhật ký vi phạm**.

**Điều dễ nhầm.**
- **Chỉ chặn được HTTP.** Trang HTTPS gọi thẳng sẽ báo lỗi kết nối — chặn vẫn có tác dụng nhưng không hiện trang giải thích và không ghi được vi phạm.
- Không đặt lịch nào = chặn 24/7. Không chọn nhóm máy nào = áp cho **tất cả** máy.
- Loại **Cho phép** dùng để mở lại vài tên miền trong một danh mục đã chặn.
- Chỉ cần gõ tên miền gốc — hệ thống tự bỏ `https://`, `www.`, đường dẫn và cổng.

### Cập nhật máy khách

**Để làm gì.** Công bố bản mới của phần mềm máy trạm để các máy tự cập nhật.

**Thao tác chính.** **Công bố bản mới** · bật/tắt · Xoá.

**Điều dễ nhầm.**
- **Máy trạm tự cài, không ai phải đi từng máy.** Dịch vụ nền hỏi máy chủ mỗi 10 phút; có bản mới thì tải, kiểm băm, và chỉ cài khi máy **không có khách** — đang có phiên thì hoãn tới lần hỏi sau. Cài xong giao diện khởi động lại bằng bản mới trong khoảng 15 giây.
- **Tệp công bố là chính tệp `vnet-client-<kiến trúc>.exe`**, không phải bộ cài. Nền tảng ghi `windows-amd64` hoặc `windows-arm64` cho đúng máy.
- **Ô Băm SHA-256 bắt buộc đúng.** Máy trạm tải xong sẽ kiểm băm, lệch thì xoá tệp và không cài. Chuỗi này in ra ở cuối lệnh `scripts/build-client.sh`.
- **Bản đang phát hành phải tắt trước khi xoá.**

### Sao lưu

**Để làm gì.** Sao lưu và khôi phục database.

**Thao tác chính.** **Tạo sao lưu** · **Phục hồi** · Xoá.

**Điều dễ nhầm.**
- **Không có sao lưu tự động.** Phải bấm tay, hoặc tự dựng lịch bên ngoài.
- Sao lưu chạy nền — bấm xong trạng thái là **Đang chạy**, tải lại trang để xem đã xong chưa.
- **Tệp có đuôi `.sql` nhưng thật ra là định dạng nhị phân.** Đừng mở bằng trình soạn thảo, đừng sửa tay.
- **Phục hồi ghi đè dữ liệu hiện tại.** Không có bước hoàn tác.
- **Chạy bằng Docker thì phải chép tệp sao lưu ra ngoài ngay** — xem cảnh báo ở [phần D](#d-vận-hành-và-sự-cố).

### Nhật ký hoạt động

**Để làm gì.** Ai làm gì, lúc nào. Dùng khi có tranh chấp.

**Thao tác chính.** Chỉ xem, lọc theo hành động, đối tượng, người dùng, thời gian.

**Điều dễ nhầm.**
- **Ghi cả những lần thao tác thất bại.** Ai đã cố tắt máy nào mà lệnh không tới nơi cũng nằm trong đây.
- Không xoá được và không sửa được. Đó là điểm mấu chốt của nhật ký.

---

# C. Phân quyền

## Ba vai trò có sẵn

`seed` tạo sẵn ba vai trò. Quản lý người dùng và vai trò ở **Quản lý hệ thống** (menu này chỉ hiện với ai có quyền `client.admin`).

| Vai trò | Được gì |
|---|---|
| **owner** — Chủ | Toàn quyền, không giới hạn |
| **manager** — Quản lý | Tất cả **trừ** `*` và `client.admin` |
| **staff** — Nhân viên | Chỉ `members.view`, `machines.view`, `orders.view`, `client.admin` |

## Danh sách mã quyền

| Mã | Tên | Kiểm ở đâu |
|---|---|---|
| `*` | Toàn quyền | Bỏ qua mọi lần kiểm khác |
| `members.view` | Xem hội viên | Menu Điểm danh, Giới nghiêm |
| `members.create` | Tạo hội viên | Nút thêm hội viên |
| `members.topup` | Nạp tiền | Nạp, hoàn tiền |
| `machines.view` | Xem máy | Menu Tài sản máy, Chặn website |
| `orders.view` | Xem đơn hàng | **Chỉ lọc menu** |
| `orders.create` | Tạo đơn hàng | **Chỉ lọc menu** |
| `orders.pay` | Thanh toán | **Chỉ lọc menu** |
| `reports.view` | Xem báo cáo | Menu Báo cáo, Đánh giá dịch vụ, và mọi API báo cáo |
| `settings.edit` | Sửa cài đặt | Menu Cài đặt, Máy in, Kiểm kê kho |
| `client.admin` | Admin client | Menu Nhật ký, Thông báo, Sao lưu, Cập nhật máy khách, Quản lý hệ thống |

## Hai điều bất ngờ, nói trước để khỏi vấp

**1. Nhân viên thường thấy trang Sao lưu, còn Quản lý thì không.**

`client.admin` được cấp cho `staff` nhưng bị loại khỏi `manager`. Nghĩa là tài khoản `staff` vào được **Sao lưu**, **Nhật ký hoạt động**, **Thông báo**, **Cập nhật máy khách** và **Quản lý hệ thống**, còn `manager` thì không.

Nếu đó không phải điều bạn muốn — và với phần lớn quán thì không — hãy sửa lại quyền của hai vai trò này ngay sau khi cài, ở **Quản lý hệ thống → Quản lý vai trò**.

**2. Ba mã quyền về đơn hàng hiện chỉ dùng để ẩn hiện menu.**

`orders.view`, `orders.create`, `orders.pay` **không** được kiểm ở tầng API. Mọi tài khoản nhân viên gọi thẳng API đơn hàng đều qua được, bất kể có mã quyền đó hay không. Bỏ ba mã này khỏi một vai trò chỉ làm menu ẩn đi, không thật sự chặn.

## Hội viên khác nhân viên

Khách đăng nhập từ máy trạm nhận một loại thẻ đăng nhập riêng. Mọi trang quản trị đều từ chối loại thẻ đó, kể cả khi ai đó tìm cách gắn thêm quyền vào. Khách chỉ đọc được dữ liệu của chính mình.

---

# D. Vận hành và sự cố

## D.1. Bảy việc hệ thống tự làm

Chạy nền, không cần ai bấm:

| Việc | Bao lâu một lần | Làm gì |
|---|---|---|
| `hardware:prune-history` | 1 giờ, **và ngay khi khởi động** | Xoá số đo phần cứng cũ hơn `HARDWARE_HISTORY_DAYS` |
| `idempotency:prune` | 1 giờ | Dọn khoá chống-nạp-trùng quá 1 ngày |
| `machines:mark-offline` | 30 giây | Máy im quá 45 giây → Ngoại tuyến |
| `bookings:expire` | 1 phút | Đánh vắng lịch quá hạn 15 phút; đóng lịch đã hết giờ |
| `sessions:enforce-limits` | 1 phút | Trừ tiền theo phút; đóng phiên hết giờ, hết phút, hết tiền; dọn phiên sót sau khi máy khởi động lại |
| `curfew:enforce` | 1 phút | Trả máy khách vị thành niên khi tới giờ cấm hoặc chạm trần giờ |
| `members:refresh-tier` | 15 phút | Xếp lại hạng theo tổng chi tiêu |

**Không có** việc tự chốt ca. **Không có** việc tự sao lưu. Hai thứ này phải làm tay.

Một việc lỗi không làm chết máy chủ và không chặn các việc khác — nó chỉ ghi nhật ký rồi thử lại ở lượt sau.

## D.2. Sao lưu

Hệ thống gọi `pg_dump` chạy nền. Tệp lưu trong thư mục `BACKUP_DIR`, mặc định là `backups` cạnh nơi chạy máy chủ.

> **Cảnh báo cho ai chạy bằng Docker.** Thư mục sao lưu **không được gắn ổ đĩa ngoài**. Tệp sao lưu nằm trong lớp ghi tạm của container và **mất sạch khi container bị dựng lại** — mà `docker compose up --build` thì dựng lại container. Chép tệp ra ngoài ngay sau khi tạo:
>
> ```bash
> docker compose cp app:/app/backups ./backup-luu-ngoai
> ```
>
> Hoặc thêm một ổ đĩa cho `/app/backups` vào `docker-compose.yml` trước khi dùng thật.

Nên: sao lưu trước mỗi lần nâng cấp, và đặt lịch sao lưu định kỳ bằng công cụ bên ngoài.

## D.3. Bảng sự cố

### Máy chủ không khởi động

Xem nhật ký: `docker compose logs app`.

| Dòng thấy trong nhật ký | Nguyên nhân | Sửa |
|---|---|---|
| `Refusing to start: JWT_SECRET is still the development placeholder` | Chưa đổi `JWT_SECRET` | Đặt chuỗi thật, ≥ 32 ký tự |
| `Refusing to start: JWT_SECRET must be at least 32 characters` | Chuỗi quá ngắn | `openssl rand -base64 48` |
| `Refusing to start: ALLOWED_ORIGINS must list the admin origins` | Để trống | Điền địa chỉ công khai |
| `Refusing to start: ALLOWED_ORIGINS must not contain "*"` | Dùng `*` | Liệt kê từng địa chỉ |
| Compose báo thiếu biến | Thiếu một trong `DB_PASSWORD`, `JWT_SECRET`, `ALLOWED_ORIGINS` | Điền vào `.env` |

### Máy hiện Ngoại tuyến

Đi lần lượt:

1. **Dịch vụ có chạy không?** Trên máy đó mở `services.msc`, tìm **VNET Client Agent**. Không thấy thì chạy `vnet-client.exe --install-service`.
2. **Mã máy có khớp không?** So mã trong `config.json` cạnh tệp `.exe` với mã ở trang **Máy**. Sai một ký tự là nhịp tim không khớp vào đâu cả.
3. **Địa chỉ máy chủ đúng chưa?** Trên máy đó mở trình duyệt vào `http://<địa chỉ máy chủ>:20800/api/health`. Không ra gì thì là chuyện mạng hoặc tường lửa.
4. **Máy chủ có thấy nhịp tim không?** `docker compose logs -f app` rồi xem có dòng nào nhắc mã máy đó.

Máy chỉ cần im 45 giây là bị đánh ngoại tuyến, nên mạng chập chờn cũng đủ gây ra.

### Bấm nút điều khiển thì báo "chưa kết nối"

Máy trạm không giữ kết nối tới máy chủ. Thường là dịch vụ nền đã bị tắt. Bấm **Bật** dịch vụ trong `services.msc` — người dùng thường cũng bật được, không cần tài khoản quản trị.

Lưu ý: **lệnh không tới nơi thì phiên chơi giữ nguyên**. Hệ thống không kết phiên khi chưa chắc khách đã rời máy.

### Đang dùng thì bị đá ra trang đăng nhập

Trang quản trị xử lý theo mã lỗi máy chủ trả về:

| Mã | Chuyện gì | Bạn thấy |
|---|---|---|
| `9999` | Thẻ hết hạn | Tự làm mới và thử lại — thường bạn không nhận ra |
| `8888` | Buộc đăng xuất | Về thẳng trang đăng nhập |
| `7777` | Phiên hết hạn | Hiện hộp thoại rồi mới đăng xuất |

Bị đá ra liên tục thì kiểm tra: `JWT_SECRET` vừa bị đổi (đổi là đăng xuất tất cả), hoặc đồng hồ máy chủ lệch nhiều.

### Ảnh sản phẩm không hiện

Ảnh nằm ở đường dẫn `/uploads`. Kiểm:

1. Thư mục `UPLOAD_DIR` có tồn tại và ghi được không.
2. Chạy Docker thì ổ đĩa `uploads` đã gắn chưa.
3. Mở thẳng đường dẫn ảnh trong trình duyệt xem trả về gì.

### Số liệu không tự cập nhật, phải tải lại trang mới thấy

WebSocket không nối được. Gần như luôn là `ALLOWED_ORIGINS`: địa chỉ bạn đang mở trình duyệt phải **có mặt nguyên văn** trong danh sách đó. Mở qua `http://192.168.1.10:20800` mà danh sách chỉ ghi `http://localhost:20800` thì không khớp.

### Bếp không nhận được phiếu chế biến

Món chưa được gán máy in. Vào **Máy in → Phân luồng**, gán món cho máy in tương ứng. **Món không gán máy in nào thì không có phiếu** — hệ thống không báo lỗi, chỉ đơn giản là không in.

### Phiếu in ra ký tự lạ

Sai bảng mã tiếng Việt. Vào **Máy in → Sửa**, đổi ô **Tiếng Việt** sang **Bỏ dấu**, rồi **In thử**. Chế độ có dấu CP1258 chỉ chạy trên máy in hỗ trợ.

### Xoá gì đó thì báo lỗi màu đỏ

Đúng như thiết kế. Hệ thống chặn xoá dữ liệu còn ràng buộc, và câu thông báo luôn nói rõ còn vướng gì cùng lối đi thay thế. Ví dụ: *"không xoá được: còn 142 phiên chơi — hãy tắt hoạt động máy thay vì xoá"*.

Lối thoát theo từng loại:

| Muốn ngừng dùng | Làm gì thay vì xoá |
|---|---|
| Máy | Tắt công tắc **Đang hoạt động** |
| Hội viên | Bỏ tích **Kích hoạt** |
| Sản phẩm | Bỏ tích kích hoạt |
| Đơn hàng | **Huỷ đơn** |
| Lịch đặt chỗ | **Huỷ** |
| Nhóm máy / Nhóm hội viên | Chuyển thành viên sang nhóm khác trước |
| Máy in | Gỡ khỏi các danh mục trước |

### Tính tiền không giống cấu hình

Kiểm theo thứ tự ưu tiên giá:

1. **Giá theo hạng hội viên** ở **Nhóm máy → Bảng giá → Theo hạng hội viên** — thắng tất cả.
2. **Giá theo khung giờ** — chỉ dùng khi không có dòng giá theo hạng nào khớp.
3. **Giá/giờ của nhóm máy** — chỉ dùng khi hai cái trên không có.

Rồi mới trừ phần trăm giảm của nhóm hội viên. Và kiểm ô **Tối thiểu (phút)**: khách chơi 5 phút mà đặt tối thiểu 30 phút thì tính 30 phút.

## D.4. Cần thêm gì

- Tài liệu kỹ thuật cho lập trình viên: [`AGENTS.md`](../AGENTS.md), [`CONTRIBUTING.md`](../CONTRIBUTING.md)
- Kịch bản kiểm thử máy trạm trên Windows: [`scripts/KIEM-CHUNG-WINDOWS.md`](../scripts/KIEM-CHUNG-WINDOWS.md)
- Danh sách API đầy đủ: chạy máy chủ ở chế độ `debug` rồi mở `/swagger/index.html`
