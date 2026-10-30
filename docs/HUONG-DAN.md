# VNET — Hướng dẫn sử dụng

Tài liệu này dành cho ba người khác nhau:

- **Người triển khai** — dựng máy chủ, cài máy trạm lên từng PC. Đọc phần 1 đến 3.
- **Nhân viên quầy** — mở máy, thu tiền, chốt ca. Đọc phần A, tra phần B khi cần.
- **Chủ quán** — phân quyền, xem báo cáo, xử lý sự cố. Đọc phần C và D.

Mọi câu trong tài liệu này viết ra từ mã nguồn tại thời điểm cập nhật, không chép lại tài liệu cũ. Chỗ nào hệ thống làm khác với điều người ta thường tưởng, tài liệu nói thẳng ra.

Mới bắt đầu và chỉ muốn chạy thử? Đọc [BAT-DAU-NHANH.md](./BAT-DAU-NHANH.md) trước — hai trang, từ máy chủ tới một phiên khách đầu tiên.

---

## Mục lục

**Triển khai**
- [0. Hệ thống gồm những gì](#0-hệ-thống-gồm-những-gì)
- [1. Dựng máy chủ](#1-dựng-máy-chủ)
  - [1.2. Docker](#12-cách-1--docker) · [1.2a. Bộ cài Windows](#12a-cách-2--bộ-cài-windows-không-cần-docker) · [1.3. Tệp thực thi](#13-cách-3--tệp-thực-thi-tự-build)
  - [1.7. Bật cập nhật game (tuỳ chọn)](#17-bật-cập-nhật-game-tuỳ-chọn)
- [2. Cài máy trạm lên PC khách](#2-cài-máy-trạm-lên-pc-khách)
- [3. Máy trạm dưới mắt khách](#3-máy-trạm-dưới-mắt-khách)

**Sử dụng**
- [A. Việc thường ngày](#a-việc-thường-ngày)
- [B. Tra cứu từng màn hình](#b-tra-cứu-từng-màn-hình)
  - [Điều khiển máy từ xa (nút Khác của trang Máy)](#điều-khiển-máy-từ-xa-nút-khác)
  - [Cập nhật game](#cập-nhật-game)

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

Ba đường. Chọn theo máy chủ bạn có:

| Máy chủ của bạn | Đường nên đi | Mục |
|---|---|---|
| **Windows** — Windows Server 2016 trở lên, hoặc Windows 10/11 64-bit | **Bộ cài Windows** `vnet-server-setup-<phiên bản>.exe`: một tệp gồm máy chủ chạy như dịch vụ Windows, PostgreSQL đi kèm và mọi bước cấu hình. Không cần Docker, không cần cài PostgreSQL. Đây là đường duy nhất cho **Windows Server 2019**, nơi không chạy được container Linux | [1.2a](#12a-cách-2--bộ-cài-windows-không-cần-docker) |
| Linux, macOS, hoặc máy đã có Docker | **Docker** | [1.2](#12-cách-1--docker) |
| Muốn tự lo PostgreSQL và dịch vụ | **Tệp thực thi** tự build | [1.3](#13-cách-3--tệp-thực-thi-tự-build) |

Dù đi đường nào, các mục [1.4 biến môi trường](#14-biến-môi-trường), [1.5 ba luật từ chối khởi động](#15-ba-luật-khiến-máy-chủ-từ-chối-khởi-động) và [1.6 cổng](#16-cổng-và-mạng) đều áp dụng. Máy trạm trỏ về `http://<IP máy chủ>:20800`.

## 1.1. Cần gì trước

| Thứ | Bản |
|---|---|
| PostgreSQL | 15 trở lên. Docker dùng 16; **bộ cài Windows kèm sẵn PostgreSQL 16**, không cần cài thêm |
| Docker + Docker Compose | Chỉ cho đường Docker. Bản nào cũng được, miễn có `docker compose` |
| Windows 64-bit + quyền quản trị | Chỉ cho bộ cài Windows |

Nếu tự biên dịch (đường Docker hay tệp thực thi) thì cần thêm **Go 1.25+**, **Node 20.19+**, **pnpm 8.6+**. Tạo bộ cài Windows cần thêm `makensis`, `curl`, `unzip` (xem [1.2a](#12a-cách-2--bộ-cài-windows-không-cần-docker)).

## 1.2. Cách 1 — Docker

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

> **Đổi cả ba mật khẩu ngay sau khi đăng nhập lần đầu.** Mật khẩu này công khai trong mã nguồn, nên lần đăng nhập đầu của mỗi tài khoản hiện hộp **Bắt buộc đổi mật khẩu** — không đóng được, mật khẩu mới ít nhất 8 ký tự. Sau này đổi lại ở **tên tài khoản góc trên phải → Đổi mật khẩu**.

### Vận hành hằng ngày

```bash
docker compose logs -f app        # xem nhật ký
docker compose restart app        # khởi động lại máy chủ
docker compose down               # dừng, GIỮ dữ liệu
docker compose down -v            # dừng và XOÁ SẠCH database
```

`down -v` xoá cả ổ đĩa chứa database. Không có bước hỏi lại.

## 1.2a. Cách 2 — Bộ cài Windows (không cần Docker)

Xem cả quá trình trong [video hướng dẫn](https://www.facebook.com/groups/1475734131291921/posts/1495823275949673/) (phần 1, bản 1.1.19).

Một tệp `vnet-server-setup-<phiên bản>.exe` (khoảng 64 MB) cài trọn máy chủ lên Windows 64-bit: `vnet-server.exe` (API cùng trang quản trị nhúng sẵn) chạy như **dịch vụ Windows**, PostgreSQL 16 bản portable đi kèm (có `pg_dump` và `pg_restore` cho sao lưu), `vnet-migrate.exe`, `vnet-seed.exe`, và bộ thư viện Visual C++ mà PostgreSQL cần. Bật máy lên là chạy, không ai phải đăng nhập Windows.

### Bước 1: lấy bộ cài

Có hai cách:

- **Tải về** từ mục **Releases** của repo GitHub, khi đã có bản phát hành.
- **Tự build** trên Mac hoặc Linux — không cần máy Windows, vì bộ cài làm bằng NSIS. Cần Go 1.25+, **pnpm 10** (`npm install -g pnpm@10`), `makensis` (`brew install makensis`), `curl` và `unzip`. Với pnpm 11, lần build đầu — khi `admin/node_modules` chưa có — dừng ở bước cài thư viện với lỗi `ERR_PNPM_IGNORED_BUILDS`. Chạy:

  ```bash
  cd core
  bash scripts/build-server-windows.sh 1.2.0
  ```

  Tham số duy nhất là số hiệu phiên bản (bỏ trống thì thành `dev`). Lần đầu lệnh tải PostgreSQL (~320 MB, kiểm băm SHA-256, lưu ở `~/.cache/vnet-build`), rồi dựng trang quản trị, biên dịch ba tệp `.exe` cho Windows và đóng gói. Kết quả là `dist/vnet-server-setup-1.2.0.exe`; cuối lệnh in SHA-256 của tệp để bạn đối chiếu sau khi chép sang máy chủ.

Chép tệp sang máy chủ Windows.

### Bước 2: chạy bộ cài

Bấm đúp tệp; Windows hỏi quyền quản trị vì bộ cài cần. Bộ cài chưa ký số nên SmartScreen có thể chặn với *Windows protected your PC*: bấm **More info** rồi **Run anyway** (muốn chắc tệp đúng bản thì so SHA-256 với trang Releases). Các trang, chữ tiếng Việt: **Chào mừng** → **Chọn thư mục cài đặt** → **Cài đặt** (khoảng một phút) → **Đã cài xong**. Thư mục mặc định là **`C:\VNET`** (ổ C gốc, không phải Program Files, để bạn dễ tìm dữ liệu và bản sao lưu và đường dẫn không có khoảng trắng).

Ô chi tiết khi cài hiện từng việc bộ cài làm, bằng chữ **không dấu** — cố ý, vì Windows hiển thị chữ có dấu ở đó thành rác:

1. Dừng dịch vụ cũ nếu có, rồi chép tệp.
2. Cài Visual C++ Runtime.
3. Tạo `data\config.env` với mật khẩu database và `JWT_SECRET` **ngẫu nhiên**, cổng 20800.
4. Khởi tạo PostgreSQL trong `data\pgdata`, ở cổng riêng **25432**, **chỉ nghe trên chính máy này**.
5. Đăng ký hai dịch vụ Windows tự chạy khi bật máy: **VNETPostgres** và **VNETServer** (phụ thuộc Postgres; chết thì Windows tự bật lại sau 10 giây).
6. Dựng bảng và sửa schema (`vnet-migrate`).
7. Tạo ba tài khoản `admin`, `manager`, `staff`, **mỗi tài khoản một mật khẩu ngẫu nhiên** — chỉ ở lần cài đầu.
8. Mở cổng TCP 20800 trong tường lửa Windows (luật `VNET-Server`, mọi profile mạng).

Cài lần đầu trên máy chưa từng có VNET, ô chi tiết in thêm vài dòng `The service name is invalid` / `NET HELPMSG 2185`: đó là bước 1 không tìm thấy dịch vụ cũ, bình thường.

Dòng cuối ghi địa chỉ cho máy trạm, dạng `Xong. Dia chi cho may tram: http://192.168.x.x:20800`, nhưng bộ cài chuyển sang trang **Đã cài xong** ngay khi chạy xong nên thường không kịp đọc. Mọi máy trạm cần địa chỉ này: nó là `http://<IP máy chủ>:20800`, IP xem bằng `ipconfig` (dòng IPv4). Cài thất bại thì hiện hộp thoại trỏ tới `C:\VNET\logs\install.log` (xem [D.3](#bộ-cài-máy-chủ-windows-báo-lỗi-hoặc-máy-chủ-không-lên)).

Trang **Đã cài xong** có ô **Mở trang quản trị** tick sẵn: bấm **Hoàn thành** là trình duyệt mở trang đăng nhập. Cài xong có lối tắt **VNET Quản lý** trên Desktop (mở `http://localhost:20800`) và thư mục **VNET Server** trong Start Menu: *VNET Quản lý*, *Thư mục dữ liệu*, *Nhật ký*, *Gỡ cài đặt*.

### Bước 3: đăng nhập và đổi mật khẩu

Mở **`C:\VNET\data\initial-admin-password.txt`** (chỉ tài khoản quản trị Windows mở được), có ba dòng dạng `admin : <mật khẩu>`. Mỗi tài khoản **một mật khẩu khác nhau**, và không có `admin123` ở đâu cả: giao `staff` cho thu ngân mà chung mật khẩu thì cũng là giao luôn chìa khoá `admin`.

Đăng nhập tại `http://localhost:20800` (hoặc `http://<IP máy chủ>:20800` từ máy khác). Khác đường Docker, trang **không** ép đổi mật khẩu vì mật khẩu này không phải mật khẩu mặc định công khai — nhưng vẫn nên đổi ngay: **bấm tên tài khoản ở góc trên phải → Đổi mật khẩu**, rồi **xoá tệp mật khẩu**, vì nó nằm trên đĩa dưới dạng chữ thường.

### Vận hành hằng ngày

| Việc | Cách làm |
|---|---|
| Xem nhật ký | `C:\VNET\logs\server.log` — xoay vòng ở 20 MB, giữ một bản cũ `server.log.1`; quán 50 máy ra cỡ 30 MB mỗi ngày. Nhật ký cài đặt: `logs\install.log`. Nhật ký PostgreSQL: `data\pgdata\log` |
| Dừng, chạy lại máy chủ | `services.msc` → **VNET Server**, hoặc PowerShell quản trị: `Restart-Service VNETServer` |
| Sửa cấu hình | Sửa `C:\VNET\data\config.env`, mỗi dòng `TÊN=giá trị`, **không dấu nháy**, rồi khởi động lại **VNET Server**. Biến nào đặt được: [1.4](#14-biến-môi-trường). **Đừng đổi `DB_PASSWORD` và `DB_PORT`** — database đã tạo theo hai giá trị này. Đổi `SERVER_PORT` thì chạy lại bộ cài để mở cổng tường lửa mới |
| Sao lưu | Trang **Hệ thống → Sao lưu** gọi `pg_dump` đi kèm bộ cài; tệp nằm ở `C:\VNET\data\backups`. Chép ra ổ khác hoặc máy khác — cùng ổ với dữ liệu thì ổ hỏng là mất cả hai. Xem thêm [D.2](#d2-sao-lưu) |
| Nâng cấp | Tải hoặc build bộ cài **phiên bản mới**, chạy lại như lần đầu. Dữ liệu, `config.env`, mật khẩu và tài khoản được **giữ nguyên**; bộ cài chỉ thay tệp chương trình, bật lại dịch vụ và sửa schema. **Sao lưu trước.** Máy chủ ngừng phục vụ trong lúc cài; máy chủ tắt quá 5 phút thì máy trạm đang có khách khởi động lại ([2.6](#26-vì-sao-dịch-vụ-khó-tắt)), nên làm lúc quán vắng |
| Gỡ | **Settings → Apps → VNET Server**, hoặc *Gỡ cài đặt* trong Start Menu. Bộ cài dừng và xoá hai dịch vụ, xoá luật tường lửa, rồi **hỏi có xoá luôn dữ liệu của quán không — mặc định là Không**. Chọn Không thì `C:\VNET\data` còn nguyên và lần cài lại sau dùng tiếp dữ liệu cũ. Gỡ im lặng (`/S`) cũng giữ dữ liệu |

### Điều dễ nhầm

- **Thư mục `C:\VNET` được khoá quyền.** Người dùng Windows thường chỉ đọc và chạy được; `data\` chỉ cho SYSTEM và Administrators. Nhờ vậy một khách có tài khoản Windows trên máy chủ cũng không đọc được `config.env` (mật khẩu database, `JWT_SECRET`).
- **PostgreSQL đi kèm chạy ở cổng 25432, chỉ trên máy này.** Nó không đụng một PostgreSQL khác đã có sẵn, và máy khác trong mạng quán không nối được vào database.
- **Đổi bản chính của PostgreSQL thì bộ cài từ chối cài đè** (thông báo *"Database tao bang PostgreSQL 16, bo cai mang PostgreSQL 17..."*): database cũ phải được `pg_upgrade` trước. Đổi bản vá thì an toàn.
- **`UPLOAD_DIR` và `BACKUP_DIR` do dịch vụ tự đặt** là `data\uploads` và `data\backups`; đặt chúng trong `config.env` không có tác dụng.
- **Chạy lại bộ cài sau một lần cài hỏng được.** Nó kiểm database thật chứ không tin dấu vết lần trước: chưa có database thì tạo, bảng người dùng còn trống thì mới tạo tài khoản và sinh mật khẩu.
- **Đã chạy trong dịch vụ, không có cửa sổ lệnh.** Mọi dòng nhật ký, kể cả lý do máy chủ từ chối khởi động (`Refusing to start:` — xem [1.5](#15-ba-luật-khiến-máy-chủ-từ-chối-khởi-động)), nằm trong `logs\server.log`.

> **Đã kiểm khi viết mục này:** cấu trúc bộ cài (mở bộ cài bản 1.1.15 bằng 7-Zip: ba tệp `.exe`, `pgsql\` có `pg_dump`/`pg_restore`/`psql`/`pg_ctl`, `setup\vc_redist.x64.exe` và `setup\vnet-server-install.ps1`), lệnh build trên macOS, và cơ chế mật khẩu ngẫu nhiên từng tài khoản (`SEED_PASSWORD_<TÊN>`, đăng nhập đúng, không bị ép đổi). **Đã chạy thật ngày 30/10/2026** trên Windows Server 2019 với bản phát hành 1.1.19, cài mới vào máy chưa có VNET: SmartScreen, các trang bộ cài, hai dịch vụ, tệp mật khẩu ba tài khoản, đăng nhập và đổi mật khẩu, máy khác trong mạng mở được `http://<IP>:20800` (tường lửa đúng); gỡ im lặng (`uninstall.exe /S`) xoá dịch vụ, luật tường lửa, chương trình và giữ `data\`. **Chưa chạy thật:** sao lưu bằng `pg_dump` qua bộ cài này, nâng cấp bằng bộ cài, hộp hỏi xoá dữ liệu khi gỡ bằng tay.

## 1.3. Cách 3 — Tệp thực thi tự build

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

Máy chủ **không tự đọc tệp `.env`**. Các tệp `.env` trong repo chỉ để tham khảo hoặc để Docker Compose đọc; chạy tay thì phải `export` hoặc truyền qua trình quản lý dịch vụ. Ngoại lệ là bản cài bằng bộ cài Windows ([1.2a](#12a-cách-2--bộ-cài-windows-không-cần-docker)): dịch vụ tự đọc `C:\VNET\data\config.env`, mỗi dòng `TÊN=giá trị`.

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
| Máy chủ cài bằng bộ cài Windows | `20800` (`SERVER_PORT` trong `data\config.env`); bộ cài tự mở cổng này trong tường lửa Windows |
| PostgreSQL của bộ cài Windows | `25432`, chỉ nghe trên chính máy chủ — không đụng PostgreSQL khác trên máy |
| Trang quản trị khi phát triển | `20900` |

Đổi `APP_PORT` thì **phải sửa `ALLOWED_ORIGINS` cho khớp**, nếu không trình duyệt sẽ bị chặn khi gọi API.

## 1.7. Bật cập nhật game (tuỳ chọn)

Tính năng này mặc định **tắt**. Chưa bật thì máy chủ không mở cổng BitTorrent, không ghi vào ổ đĩa nào, và trang **Hệ thống → Cập nhật game** chỉ hiện dòng *"Máy chủ này chưa bật cập nhật game"*. Cách dùng trang đó nằm ở [Cập nhật game](#cập-nhật-game).

Nó sinh ra cho phòng máy **diskless dùng Cloud Update**: game nằm trên một ổ riêng của máy chủ (Game Disk), máy trạm thấy ổ đó với một chữ ổ khác (mặc định `D:`). VNET chỉ cần ghi vào thư mục game trên ổ ấy; máy trạm chạy game thẳng từ đó.

Hai vai trò, chọn bằng biến `GAME_ROLE`:

| Vai trò | Làm gì | Đặt ở đâu |
|---|---|---|
| `master` | Giữ **bản gốc** của từng game. Khi thư mục một game đứng yên đủ lâu, tự đóng gói ("publish") thành một bản có số phiên bản và phát cho các quán | Một máy duy nhất — nơi launcher (Riot, Garena...) tự cập nhật game |
| `cafe` | Hỏi master có bản mới không, rồi tải **chỉ những phần đã đổi** về ổ game của quán | Máy chủ của từng quán |

Một quán bình thường chỉ cần vai trò `cafe`; địa chỉ master và khoá do bên giữ master đưa cho bạn.

| Biến | Master | Quán | Ý nghĩa |
|---|---|---|---|
| `GAME_ROLE` | `master` | `cafe` | Để trống là tắt |
| `GAME_ROOT` | bắt buộc | bắt buộc | Đường dẫn **tuyệt đối** tới thư mục chứa game *trên máy chủ này*, mỗi game một thư mục con trùng tên game. Ví dụ `D:\Games` |
| `GAME_CATALOG_KEY` | bắt buộc | bắt buộc | Chuỗi ít nhất 16 ký tự. **Master và mọi quán phải dùng cùng một chuỗi** |
| `GAME_PUBLIC_URL` | bắt buộc | — | Địa chỉ master mà các quán gọi tới, ví dụ `http://<địa chỉ master>:20800` |
| `GAME_UPSTREAM_URL` | — | bắt buộc | Chính là địa chỉ master ở dòng trên |
| `GAME_STABLE_FOR` | `10m` | — | Thư mục game phải đứng yên bấy lâu mới được publish |
| `GAME_POLL_INTERVAL` | `5m` | `5m` | Bao lâu máy chủ làm một vòng: master quét thư mục, quán hỏi master |
| `GAME_STALL_TIMEOUT` | — | `10m` | Một lượt tải không thêm được mảnh dữ liệu đúng nào trong bấy lâu thì bị coi là treo |
| `GAME_LISTEN_PORT` | `19999` | `19999` | Cổng của bộ tải BitTorrent bên trong máy chủ |
| `GAME_DATA_DIR` | `GAME_ROOT\.vnet-gameupdate` | như master | Trạng thái riêng của bộ tải, không chứa game |

Cấu hình sai thì máy chủ **từ chối khởi động** với dòng `Refusing to start:`, ví dụ `GAME_UPSTREAM_URL is required when GAME_ROLE=cafe`, `GAME_ROOT must be an absolute path when GAME_ROLE is set`, `GAME_CATALOG_KEY must be at least 16 characters when GAME_ROLE is set`.

**Cách đặt biến**

- **Docker:** tạo tệp `docker-compose.override.yml` cạnh `docker-compose.yml` (Compose tự đọc tệp này), rồi chạy `docker compose up -d`.

  ```yaml
  services:
    app:
      environment:
        GAME_ROLE: cafe
        GAME_ROOT: /games
        GAME_UPSTREAM_URL: http://<địa chỉ master>:20800
        GAME_CATALOG_KEY: <chuỗi dùng chung với master>
      volumes:
        - /duong/dan/tren/may/chu:/games
  ```

  `GAME_ROOT` ở đây là đường dẫn **trong container**; thư mục thật phải được gắn vào và ghi được. Địa chỉ master phải gọi được *từ trong container* — `localhost` ở đó là chính container.
- **Máy chủ Windows (bộ cài `vnet-server-setup`):** thêm vào cuối tệp `C:\VNET\data\config.env`, mỗi dòng một biến, **không có dấu nháy**, rồi khởi động lại dịch vụ `VNETServer`.

  ```
  GAME_ROLE=cafe
  GAME_ROOT=D:\Games
  GAME_UPSTREAM_URL=http://<địa chỉ master>:20800
  GAME_CATALOG_KEY=<chuỗi dùng chung với master>
  ```

  Nhật ký mà có dòng `CẢNH BÁO: dịch vụ thiếu TORRENT_STORAGE_DEFAULT_FILE_IO=classic` thì chạy lại bộ cài `vnet-server-setup`: thiếu biến này, tệp game đã phát bị khoá và quán không xoá được tệp thừa.

Bật xong, mở **Hệ thống → Cập nhật game**: góc trên trái hiện thẻ **Máy master** hoặc **Máy chủ quán**, và nhật ký máy chủ có dòng `[game] bật vai trò cafe`.

> **Đã chạy thử khi viết mục này** (hai máy chủ chạy Docker, một master một quán): thêm game, publish, quán tự tải, game có bản mới nằm ngoài khung giờ rồi **Tải ngay** — thư mục game hai bên giống hệt nhau từng tệp. **Chưa chạy thử:** cách đặt biến trên máy chủ Windows (viết theo mã nguồn bộ cài), và việc máy trạm Cloud Update đọc đúng thư mục game sau khi khởi động lại.

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

Có [video](https://www.facebook.com/groups/1475734131291921/posts/1495823275949673/) (phần 2, bản 1.0.23). Chạy `vnet-client-setup-*.exe` với quyền quản trị. Gặp *Windows protected your PC* thì bấm **More info** → **Run anyway**. Trang đầu chọn thư mục, giữ `C:\Program Files\VNET Client` và bấm **Next**; trang **Cấu hình máy trạm** hỏi:

| Ô | Điền gì |
|---|---|
| **Địa chỉ máy chủ** | Ô điền sẵn ví dụ `http://192.168.1.10:20800`: xoá đi, gõ đúng địa chỉ và cổng máy chủ trong mạng LAN |
| **Tài khoản quản trị máy trạm** | Tên đăng nhập cho nhân viên kỹ thuật, mặc định `admin` |
| **Mật khẩu**, **Nhập lại mật khẩu** | Ít nhất 6 ký tự, không có dấu nháy kép. Đặt chung một mật khẩu cho cả quán |

Mã máy không hỏi: máy trạm dùng tên máy Windows (mục 2.2).

**Tài khoản quản trị máy trạm là đường vào chắc chắn có khi mất mạng.** Nhân viên kỹ thuật gõ nó vào chính ô đăng nhập trên màn hình khoá. Máy mở ở chế độ bảo trì: không mở phiên, không tính tiền, không bị khoá hay khởi động lại vì mất kết nối. Máy trạm kiểm tài khoản này ngay tại máy, không hỏi máy chủ.

**Đổi tài khoản cho cả quán** ở trang quản trị, **Cài đặt → Máy trạm**. Mọi máy nhận tài khoản mới ở nhịp tim kế tiếp, khoảng 15 giây, và bỏ tài khoản đặt lúc cài. Xoá tên tài khoản ở đó thì các máy quay về tài khoản đặt lúc cài.

Trình cài làm ba việc, theo thứ tự: ghi tệp cấu hình, băm mật khẩu rồi lưu, đăng ký dịch vụ Windows. Tệp `config.json` chỉ chứa băm, không chứa mật khẩu.

**Màn hình khoá chỉ mở trên phiên console**, tức màn hình thật của máy: dịch vụ mở giao diện vào phiên đang ngồi trước máy. Cài qua Remote Desktop thì dịch vụ chạy và máy vẫn hiện **Sẵn sàng** trên trang Máy, nhưng phiên Remote Desktop không thấy màn hình khoá. Nó lên khi có người đăng nhập Windows tại máy, hoặc khi máy khởi động lại và Windows tự đăng nhập (mục 2.6b). Đang ở phiên Remote Desktop mà muốn thấy ngay thì chuyển phiên đó về console: PowerShell quản trị, `tscon <ID phiên> /dest:console` (ID xem bằng `quser`), rồi nối Remote Desktop lại.

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

## 3.2. Các ô chức năng

Sau khi đăng nhập, khách thấy màn hình chính với các ô:

| Ô | Ai thấy | Việc |
|---|---|---|
| **Nạp tiền** | Chỉ hội viên | Gửi yêu cầu nạp lên quầy, số tiền chọn từ danh sách mệnh giá cấu hình ở **Cài đặt → Nạp tiền** |
| **Giờ chơi** | Chỉ nhân viên | Hiện dòng "Phiên quản trị — không tính giờ" |
| **Đồ ăn** | Mọi người trừ nhân viên | Xem thực đơn, đặt món. Đơn nhảy thẳng lên trang **Đơn hàng** |
| **Game** | Mọi người | Mở menu game của quán, xem bên dưới |
| **Hỗ trợ** | Mọi người | Nhắn tin với quầy |
| **Điểm danh** | Chỉ hội viên | Chỉ hiện khi bật **Cài đặt → Tính năng → Cho phép điểm danh** |
| **Đánh giá** | Mọi người trừ nhân viên | Chỉ hiện khi bật **Cài đặt → Tính năng → Cho phép đánh giá dịch vụ** |

Ngoài ra trên màn hình chính luôn có: **đồng hồ phiên chơi** đếm ngược tới lúc hết tiền, **số dư** (cả tiền thật lẫn điểm thưởng), và **chuông thông báo**.

**Menu Game.** Ô **Game** mở một cửa sổ liệt kê các game đang **Bật** ở trang [Cập nhật game](#cập-nhật-game) và có tệp chạy. Mỗi game là một ô có nhãn trạng thái: **Sẵn sàng**, **Đang cập nhật xx%**, **Chờ cập nhật** hoặc **Lỗi**. **Chỉ game Sẵn sàng mới bấm Chơi được** — đang cập nhật mà cho chạy thì game báo thiếu tệp, hoặc tệ hơn là chạy bản hỏng. Cửa sổ có ô **Tìm game** (tìm không dấu, theo nhóm hoặc theo chữ cái đầu, ví dụ `lmht`), nút lọc theo nhóm khi quán có từ hai nhóm game trở lên, và nút làm mới.

Game chạy thẳng từ ổ game mà máy trạm nhìn thấy, theo đường dẫn `<thư mục game trên máy trạm>\<tên thư mục game>\<tệp chạy>`. Máy trạm chỉ nhận tên game từ máy chủ rồi tự dựng đường dẫn, và từ chối đường dẫn có `..`, đường dẫn tuyệt đối hay UNC. Không mở được game thì khách thấy câu *"vui lòng báo nhân viên"* và lỗi tự gửi về trang **Nhật ký lỗi**. Mỗi lần bấm **Chơi** mở được game được đếm vào cột **Lượt chơi** của trang Cập nhật game.

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

1. Vào **Quản lý → Máy**, tìm máy, bấm **Khác** ở cuối dòng. (Máy đang có khách cũng có nút này ở bảng **Máy đang hoạt động** trên Bảng điều khiển.)
2. Chọn việc:

| Mục trong menu | Việc |
|---|---|
| **Chụp màn hình** | Xem khách đang gặp gì |
| **Điều khiển màn hình** | Xem hẳn màn hình máy ngay trong trình duyệt, và nếu cần thì tự bấm chuột, gõ phím sửa hộ khách |
| **Xem ứng dụng** | Danh sách ứng dụng đang chạy kèm mức RAM. Bấm **Tắt** ở một dòng để đóng ứng dụng đang treo |
| **Nhắn tin** | Trò chuyện hai chiều với hội viên đang ngồi máy |
| **Thông báo** | Hiện một dòng chữ lên màn hình máy, khách không trả lời được |
| **Khởi động lại** / **Tắt máy** | **Kết thúc phiên chơi** |
| **Bật máy (Wake-on-LAN)** | Bật một máy đang tắt từ xa |

Điều kiện chạy được, cách dùng từng mục và lỗi thường gặp: [Điều khiển máy từ xa](#điều-khiển-máy-từ-xa-nút-khác).

**Không còn nút Khoá/Mở khoá**, và **chặn ứng dụng không còn là nút của từng máy**: danh sách ứng dụng bị cấm nằm ở **Cài đặt → Máy trạm → Chặn ứng dụng** và áp cho **mọi** máy.

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

**Thao tác chính.** Thêm · **Tạo hàng loạt** · Sửa · Xoá · nút **Khác** ở cuối mỗi dòng (xem số đo phần cứng và điều khiển máy từ xa — chi tiết ở [Điều khiển máy từ xa](#điều-khiển-máy-từ-xa-nút-khác)). Phiên và tài sản của máy nằm ở hai trang riêng: **Phiên** và **Tài sản máy**.

**Điều dễ nhầm.**
- **Chỉ có ba trạng thái**: Ngoại tuyến → Sẵn sàng → Đang sử dụng. Không có trạng thái "bảo trì".
- **Cột Cảnh báo** có hai nhãn do máy trạm tự báo. **TK Windows quản trị** (đỏ): tài khoản Windows trên máy đó có quyền quản trị, người ngồi máy gỡ được mọi lớp bảo vệ — xem [mục 2.6a](#26a-tài-khoản-windows-cho-khách-và-máy-diskless). **Có mạng lạ** (vàng): máy vừa có thêm một card mạng (Wi-Fi, điện thoại phát mạng); rê chuột để xem tên và IP. Máy trạm không tự ngắt gì, chỉ báo. Tích **Chỉ máy có cảnh báo** để lọc.
- **Muốn tạm ngừng một máy thì tắt công tắc Đang hoạt động, đừng xoá.** Máy tạm ngừng vẫn hiện trong danh sách để bật lại; máy đã xoá thì mất lịch sử.
- Máy còn phiên chơi, đơn hàng hoặc lịch đặt thì không xoá được.
- Lệnh điều khiển cần máy đang kết nối. Máy chưa nối thì hệ thống báo "chưa kết nối — lệnh không tới nơi", không im lặng.
- **Mã máy phải trùng khớp với mã ghi trong cấu hình máy trạm**, nếu không nhịp tim không khớp vào đâu cả. Vì lý do này, dựng quán mới thì dùng **Tạo hàng loạt** thay vì gõ tay từng cái — xem [mục 2.2](#22-mã-máy-là-tên-máy-windows-không-phải-khai).
- **Tạo hàng loạt là toàn bộ hoặc không có gì.** Vướng một mã thì không máy nào được tạo, và hệ thống báo trước khi bạn bấm Tạo.
- **Máy đã xoá vẫn giữ mã của nó.** Tạo lại đúng mã đó sẽ bị chặn, dù danh sách không hiện máy nào mang mã ấy — thêm một lý do để tắt hoạt động thay vì xoá.

### Điều khiển máy từ xa (nút Khác)

**Để làm gì.** Nhìn và điều khiển một máy trạm từ trang quản trị mà không phải đi tới máy: bật máy đang tắt, xem màn hình, đóng ứng dụng treo, nhắn khách, khởi động lại, tắt máy.

**Thao tác chính.** Ở **Quản lý → Máy** (và bảng **Máy đang hoạt động** ở Bảng điều khiển), bấm **Khác** ở dòng của máy. Menu chia năm cụm, ngăn bằng vạch; mục nào bạn không có quyền thì ẩn:

| Cụm | Mục | Làm gì | Quyền |
|---|---|---|---|
| Trang | **Phần cứng** | Bảng 50 lần máy báo số đo gần nhất: nhiệt độ CPU/GPU, %CPU, %RAM, %ổ đĩa, thời gian bật. *(Chỉ có ở trang Máy)* | `machines.view` |
| Bật | **Bật máy (Wake-on-LAN)** | Máy chủ phát gói "đánh thức" ra mạng LAN để bật máy đang tắt | `machines.remote` |
| Xem | **Điều khiển màn hình** | Cửa sổ xem — và nếu muốn, điều khiển — màn hình máy ngay trong trình duyệt | `machines.remote_desktop` |
| | **Chụp màn hình** | Một ảnh màn hình khách, có nút **Tải ảnh** và **Chụp lại** | `machines.remote` |
| | **Xem ứng dụng** | Bảng ứng dụng đang chạy (tên, số tiến trình, RAM), có ô tìm và nút **Tắt** | `machines.remote` |
| Nói | **Nhắn tin** | Khung chat hai chiều với hội viên đang ngồi máy; xám đi khi máy không có hội viên | mọi nhân viên |
| | **Thông báo** | Hộp thông báo một chiều hiện lên màn hình máy | `machines.remote` |
| Nguồn | **Khởi động lại** / **Tắt máy** | Có hộp xác nhận. Kết thúc phiên chơi | `machines.remote` |

**Điều dễ nhầm.**
- **Chỉ có sáu lệnh gửi xuống máy qua WebSocket**: tắt máy, khởi động lại, thông báo, chụp màn hình, xem ứng dụng, tắt ứng dụng. Không có Khoá/Mở khoá, và **cố ý không có lệnh chạy câu lệnh tuỳ ý** — có thì mọi tài khoản nhân viên thành người chạy được mã trên cả phòng máy. Cấm một ứng dụng thì làm ở **Cài đặt → Máy trạm → Chặn ứng dụng**, áp cho mọi máy.
- **Máy chưa kết nối thì lệnh không tới nơi**: hệ thống báo vàng *"máy trạm chưa kết nối"* thay vì im lặng, và vẫn ghi lần thử đó vào **Nhật ký hoạt động**.
- **Ai làm việc gì trên máy trạm.** Tắt máy và khởi động lại do **dịch vụ nền** nhận, nên chạy được cả khi chưa ai đăng nhập. Chụp màn hình, xem ứng dụng và thông báo do **giao diện** làm vì cần màn hình của khách; giao diện không chạy thì cửa sổ ảnh chờ mãi *"Đang chờ máy trạm gửi ảnh..."*.
- **Tắt máy và khởi động lại kết thúc phiên đang chạy** — nhưng chỉ khi lệnh đã tới máy; không tới thì phiên giữ nguyên (xem [A.8](#a8-khách-báo-lỗi-máy)).
- **Bật máy (Wake-on-LAN):**
  - Chỉ bật được máy đang **Ngoại tuyến** và **đã từng báo địa chỉ MAC**. Máy đang bật thì báo *"máy đang bật"*; máy chưa từng chạy phần mềm VNET thì báo *"máy chưa báo địa chỉ MAC — cần bật máy và chạy phần mềm VNET ít nhất một lần"*.
  - Gói phát đi từ **máy chủ VNET**, tới địa chỉ broadcast của mạng con `/24` nơi máy từng báo IP (ví dụ `192.168.5.255`) và tới `255.255.255.255`, cổng 9. Máy chủ phải nằm **cùng mạng LAN** với máy trạm.
  - **"Đã gửi" chưa có nghĩa là máy đã dậy** — hệ thống chỉ biết gói đã đi ra. Máy phải bật Wake-on-LAN trong BIOS và ở card mạng, và Windows phải tắt Fast Startup. Chờ khoảng 1 phút.
  - **Chưa kiểm tra:** máy chủ chạy trong Docker thì gói phát từ trong container có ra tới mạng quán hay không (mạng ảo mặc định của Docker có thể giữ nó lại). Bấm báo thành công, BIOS đúng mà máy vẫn không dậy thì nghi ngờ chỗ này trước. Chưa thử với PC thật.
- **Điều khiển màn hình:**
  - Trình duyệt hiển thị bằng noVNC; máy chủ làm cầu nối tới **TightVNC** chạy trên máy trạm (cổng 5900). Máy trạm chỉ nhận kết nối từ địa chỉ IP của máy chủ VNET (luật tường lửa do dịch vụ nền tự tạo).
  - Mật khẩu VNC do máy chủ tự sinh cho từng máy và gửi xuống ở nhịp tim; bạn không phải nhập, cũng không thấy nó.
  - **TightVNC không nằm trong repo.** Muốn một máy điều khiển màn hình được, tải `tightvnc.msi` (bản 64-bit, từ tightvnc.com) đặt vào `installer\tightvnc.msi` **trước khi** build bộ cài máy trạm (hoặc đặt cạnh `vnet-client.exe` trên máy). Dịch vụ nền tự cài nó khi nhận mật khẩu VNC. Không có tệp thì bộ cài vẫn build, chỉ là máy đó không điều khiển màn hình được — `service.log` ghi *"chưa có TightVNC và không thấy tightvnc.msi cạnh vnet-client.exe"*.
  - Cửa sổ **mở ở chế độ Chỉ xem** để khỏi lỡ tay bấm vào máy khách. Nút **Chỉ xem** đang sáng là đang chỉ xem; bấm một lần nữa để bật chuột và bàn phím. Còn có **Ctrl+Alt+Del**, **Toàn màn hình** và **Kết nối lại**.
  - Cần quyền riêng `machines.remote_desktop`. Vai trò nhân viên mặc định **không** có quyền này; quản lý và chủ thì có.
  - Máy phải đang bật và có IP. Lỗi hay gặp: *"máy trạm chưa kết nối"* (máy ngoại tuyến); *"không nối được VNC trên máy MAY01 (192.168.5.11): máy trạm chưa cài TightVNC hoặc tường lửa chặn"*; *"VNC từ chối mật khẩu — chờ nhịp tim kế tiếp rồi thử lại"* (máy vừa đổi mật khẩu VNC, thường tự hết sau 15 giây).
  - Mỗi lần mở và đóng cửa sổ đều ghi vào **Nhật ký hoạt động** kèm tên người mở (`remote_desktop_start`, `remote_desktop_stop`). Khi không để chế độ Chỉ xem, thao tác của bạn diễn ra trên chính máy khách đang ngồi.
- **Chụp màn hình** không lưu ảnh trên máy chủ: ảnh đi thẳng từ máy trạm tới trình duyệt của bạn, đóng cửa sổ là mất; muốn giữ thì bấm **Tải ảnh**. Bấm vào ảnh để phóng to (lăn chuột để zoom, kéo để di chuyển).
- **Xem ứng dụng** gộp các tiến trình cùng tên thành một dòng. **Tắt** có hộp xác nhận và đóng **mọi** tiến trình mang tên đó. Tiến trình hệ thống và chính phần mềm VNET nằm trong danh sách cấm tắt: bấm **Tắt** không có tác dụng và trang không báo gì.
- **Nhắn tin khác Thông báo.** **Nhắn tin** là cuộc trò chuyện hai chiều, khách trả lời được từ ô **Hỗ trợ** trên máy. **Thông báo** chỉ hiện một hộp chữ lên màn hình, khách không trả lời được.
- Mọi lệnh đều ghi vào **Nhật ký hoạt động** với tên `remote_*` (`remote_wake`, `remote_restart`, `remote_screenshot`...), kể cả lệnh thất bại.

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

**Để làm gì.** Cấu hình toàn quán. Bảy tab; **mỗi tab lưu riêng** — nút **Lưu** chỉ lưu tab đang mở, và chuyển tab khi chưa lưu thì những gì vừa gõ ở tab cũ mất.

| Tab | Gồm |
|---|---|
| **Chung** | Tên cửa hàng, địa chỉ, số điện thoại |
| **Giới hạn** | Giới hạn đặt chỗ mỗi ngày và mỗi hội viên, thời gian huỷ trước, **Nợ tối đa**, **Tiền tối thiểu mỗi lần đăng nhập** |
| **Hóa đơn** | Tiêu đề, chân trang, mã số thuế in trên hoá đơn |
| **Nạp tiền** | **Mệnh giá nạp** — danh sách số tiền hiện trên máy khách |
| **Thanh toán** | **Phương thức thanh toán** quán nhận — danh sách hiện ở mọi chỗ thu tiền (thanh toán đơn, nạp tiền, mua gói). Mặc định có **Tiền mặt**, **Chuyển khoản**, **Ví điện tử**; thêm bằng **Thêm phương thức**, bật/tắt ở cột **Đang dùng** |
| **Tính năng** | Bật/tắt **Cho phép điểm danh** và **Cho phép đánh giá dịch vụ** |
| **Máy trạm** | Máy trạm làm gì khi phần mềm bị tắt (khởi động lại hay tắt máy), mất kết nối bao lâu thì khoá, bao lâu thì khởi động lại, **không ai đăng nhập bao lâu thì tắt máy hoặc cho ngủ chờ**, **Chặn ứng dụng**, **Ẩn lối tắt trên desktop**, **Metadata Riot** (máy diskless chơi Liên Minh) và **tài khoản quản trị máy trạm** cho cả quán (để trống mật khẩu là giữ nguyên). Máy trạm nhận giá trị mới trong khoảng 15 giây |

**Điều dễ nhầm.**
- **Nợ tối đa** là thứ quyết định khách nợ được bao nhiêu trước khi bị chặn mở máy.
- **Máy ở màn hình khoá mà không ai đăng nhập quá 5 phút thì tự tắt** — đó là mặc định của tab **Máy trạm**. Đang thử máy trạm mà thấy máy tự tắt thì đây là lý do; đặt `0` ở ô *Không ai đăng nhập bao lâu thì tắt máy (phút)* để không bao giờ tắt.
- **Chặn ứng dụng** áp cho **mọi** máy, không phải từng máy. Mỗi dòng một tên tiến trình, không cần đuôi `.exe` (xem tên ở mục **Xem ứng dụng** của máy — [Điều khiển máy từ xa](#điều-khiển-máy-từ-xa-nút-khác)). Dịch vụ nền tắt tiến trình trùng tên mỗi vài giây; danh sách được máy trạm giữ lại nên vẫn áp sau khi khởi động lại và cả khi mất mạng. Tiến trình hệ thống của Windows và chính phần mềm VNET không chặn được. Xoá một dòng là bỏ chặn.
- **Ẩn lối tắt trên desktop**: ghi tên lối tắt không có đuôi (ví dụ `Game Menu, Teamfight Tactics`), cách nhau bằng dấu phẩy; máy trạm tự xoá các lối tắt đó khỏi desktop của mọi tài khoản Windows. Lối tắt VNET không bao giờ bị xoá. Để trống là tắt.
- **Thanh toán — Tiền mặt luôn bật.** Công tắc của dòng này xám và không tắt được, vì đối soát két cuối ca đếm theo tiền mặt.
- **Thanh toán — mã tự sinh khi lưu** từ tên, bỏ dấu, chữ thường, nối bằng dấu gạch dưới (**Quẹt thẻ POS** thành `quet_the_pos`), tối đa 20 ký tự, và không đổi được sau đó. Mấy mã `balance`, `bonus_balance`, `gift_card`, `topup_card` dành riêng cho hệ thống nên không đặt được.
- **Thanh toán — chỉ xoá được dòng chưa lưu.** Dòng đã lưu có thể đã nằm trong lịch sử giao dịch; muốn ngừng dùng thì tắt ở cột **Đang dùng**. Phương thức đã tắt biến mất khỏi các ô chọn **và** máy chủ từ chối nếu ai gửi lên (lỗi *phương thức thanh toán "…" đang tắt*); lịch sử cũ vẫn hiện đúng tên.
- Tắt một tính năng thì nút biến mất trên máy khách **và** máy chủ cũng từ chối nếu ai đó gọi thẳng — không phải chỉ ẩn đi.
- Mở trang cần quyền **Xem cài đặt** (`settings.view`), nhân viên mặc định có; **Lưu** cần **Sửa cài đặt** (`settings.edit`), chỉ quản lý và chủ có. Nhân viên mở được trang và vẫn thấy nút **Lưu**, nhưng bấm sẽ nhận lỗi `Insufficient permissions`.

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
- **Đây là cập nhật phần mềm VNET trên máy trạm, không phải game.** Game cập nhật ở trang [Cập nhật game](#cập-nhật-game) ngay dưới.

### Cập nhật game

**Để làm gì.** Giữ game trên ổ game của quán luôn đúng bản mới nhất mà không phải đi cài lại từng máy: **máy master** phát hành, **máy chủ quán** tự kéo về. Đây cũng là danh sách quyết định game nào hiện trong menu **Game** của máy trạm. Phải bật bằng biến môi trường trước — xem [1.7](#17-bật-cập-nhật-game-tuỳ-chọn); chưa bật thì trang chỉ hiện một dòng hướng dẫn và bảng rỗng.

**Thao tác chính.** Phần đầu trang có thẻ vai trò (**Máy master** hoặc **Máy chủ quán**), thư mục game trên máy chủ, ô **Thư mục game trên máy trạm** (kèm **Lưu**), và riêng ở quán có thêm ô **Khung giờ tự cập nhật** (kèm **Lưu**). Bảng game gồm các cột Game, Nhóm, Phiên bản, Dung lượng, Lượt chơi, Trạng thái, **Tự cập nhật**, **Bật**, **Ưu tiên**, Cập nhật lúc, Thao tác.

| Việc | Ở master | Ở quán | Quyền |
|---|---|---|---|
| Xem trang | ✓ | ✓ | `games.view` |
| **Thêm game** | ✓ | — | `games.create` |
| **Sửa** (tên hiển thị, nhóm, tệp chạy, tham số, icon, mục menu) | ✓ | — *(do master quyết)* | `games.update` |
| **Publish ngay** | ✓ | — | `games.update` |
| **Tải ngay** | — | ✓ | `games.update` |
| Công tắc **Tự cập nhật** và **Bật**, ô **Ưu tiên** | ✓ | ✓ | `games.update` |
| **Xoá** (bỏ khỏi danh mục) | ✓ | — | `games.delete` |

Vai trò nhân viên mặc định chỉ có `games.view`; quản lý và chủ có đủ.

**Ở master**, theo thứ tự:
1. Đặt game vào thư mục game của máy chủ — mỗi game một thư mục con — và để launcher chính hãng tự cập nhật nó ở đó.
2. Bấm **Thêm game**, điền: **Tên thư mục** (đúng tên thư mục con), **Tên hiển thị** (để trống là lấy tên thư mục), **Nhóm** (ví dụ `Online`), **Tệp chạy** (đường dẫn *tương đối* trong thư mục game, ví dụ `Riot Client/RiotClientServices.exe`), **Tham số chạy**, **Icon** (tệp lấy hình cho menu, `.exe` hoặc `.ico`; để trống là lấy từ tệp chạy) và **Ưu tiên**. Thư mục có nhiều chương trình (LibreOffice → Word, Excel) thì dùng **Mục trên menu** thay cho tệp chạy: mỗi mục một tên và một tệp chạy.
3. Game mới thêm hiện **Chưa có** cho tới lần publish đầu. Lần đó chạy khi thư mục đứng yên đủ lâu (mặc định 10 phút), hoặc bấm **Publish ngay**. Trạng thái qua **Đang băm** rồi **Sẵn sàng**, cột Phiên bản thành `1` và Dung lượng hiện ra.
4. Launcher vá game thì thư mục đổi; đứng yên đủ lâu là master tự publish bản kế tiếp (`2`, `3`...), hoặc bấm **Publish ngay** nếu chắc game đã cập nhật xong.

**Ở quán**, mọi việc tự chạy:
1. Cứ mỗi vòng (mặc định 5 phút) máy chủ quán hỏi master. Thấy game mới thì nó hiện trong bảng, rồi được tải về nếu **Tự cập nhật** đang bật và giờ hiện tại nằm trong **Khung giờ tự cập nhật** (để trống là lúc nào cũng được, định dạng `02:00-08:00`, vắt qua nửa đêm được).
2. Master có bản mới hơn bản đang nằm trên ổ thì cột **Phiên bản** hiện `1 → 2` (số mới màu cam). Chỉ phần dữ liệu đã đổi mới được tải.
3. Muốn tải ngay, bấm **Tải ngay**: trạng thái sang **Chờ tải**, **Đang tải** (kèm thanh phần trăm), rồi **Sẵn sàng**.

**Điều dễ nhầm.**
- **Tên thư mục là khoá nối master với các quán**, nên không đổi được sau khi thêm. Chỉ chữ, số, khoảng trắng và `. _ ( ) + -`, tối đa 100 ký tự, không kết thúc bằng dấu chấm, không trùng tên thiết bị Windows (`CON`, `NUL`, `COM1`...). Thư mục phải có sẵn, không thì báo *"không thấy thư mục ..."*.
- **Có hai đường dẫn khác nhau.** **Thư mục game** (thẻ phía trên) là đường dẫn trên *máy chủ này*. **Thư mục game trên máy trạm** là đường dẫn ổ game như *máy trạm nhìn thấy* — chữ ổ Cloud Update gán, mặc định `D:\Games`. Menu Game của máy trạm ghép đường dẫn này với tên game và tệp chạy. Gõ sai (không dạng `D:\Games`) thì máy chủ báo lỗi ngay lúc lưu; để trống là về mặc định; `e:/Games/` được chuẩn hoá thành `e:\Games`.
- **Khung giờ gõ sai cú pháp thì coi như không giới hạn**, không báo lỗi — một ô gõ nhầm không được làm quán đứng im không bao giờ cập nhật. Xem lại ô sau khi **Lưu**: phải đúng dạng `02:00-08:00`.
- **Máy trạm diskless chỉ thấy bản mới sau khi khởi động lại.** Cloud Update cố định ổ game lúc máy boot, nên máy đang chạy vẫn đọc bản cũ; dùng **Khung giờ tự cập nhật** để việc ghi diễn ra lúc quán vắng rồi khởi động lại máy. **Tải ngay** không bị khung giờ giới hạn. Hệ thống **chưa kiểm tra** có máy đang chơi đúng game đó hay không trước khi ghi đè.
- **Publish chạy nền.** Băm một game hàng chục GB mất vài phút; bấm xong trạng thái là **Đang băm** và trang tự làm mới. Bấm lại trong lúc đó không làm gì thêm.
- **Master chỉ phát game đang Bật và đã publish.** Tắt **Bật** ở master là game biến khỏi danh mục các quán nhận được; game đã nằm sẵn ở quán thì giữ nguyên, không bị xoá. Ở quán, tắt **Bật** là ẩn game khỏi menu Game của máy trạm.
- **Ở quán, sửa tên hiển thị, nhóm, tệp chạy là vô ích** — những thứ đó đến từ master và bị ghi đè ở vòng sau. Nút **Sửa** chỉ có ở master.
- **Xoá chỉ bỏ game khỏi danh mục**, tệp trên ổ đĩa giữ nguyên.
- **Mỗi lần chỉ tải một game**, game **Ưu tiên** số lớn tải trước. Một game tải treo (không có thêm dữ liệu đúng trong 10 phút) bị đánh dấu **Lỗi** để nhường game khác, rồi tự thử lại sau 5 phút, 10, 20... tối đa 1 giờ một lần. Master publish bản mới thì lượt tải bản cũ bị huỷ và tính lại từ đầu. Game tắt **Tự cập nhật** mà lỗi thì dòng lỗi dặn *"Bấm Tải ngay để thử lại"*.
- Trạng thái **Lỗi** luôn kèm nguyên nhân bằng tiếng Việt ngay dưới nhãn. Ở master, lỗi hay gặp là *"không đọc được thư mục"* hoặc lần publish hỏng; ở quán là *"Tải bị treo"* hoặc không gọi được master. Quán không gọi được master thì lỗi chỉ nằm trong nhật ký máy chủ (`[game] đọc catalog của master: ...`), trang vẫn hiện nguyên trạng thái cũ.
- Cột **Lượt chơi** đếm số lần khách bấm **Chơi** mở được game trên menu của máy trạm (client từ bản 1.0.19); nhân viên mở game cũng được đếm.
- Game dùng launcher riêng như Liên Minh (Riot) cần thêm việc ngoài thư mục game: trạng thái cài đặt Riot nằm ở `C:\ProgramData\Riot Games`. Xem ô **Metadata Riot** ở **Cài đặt → Máy trạm**, và ô **Icon** khi tệp chạy chỉ là launcher chung.

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

Mỗi API dành cho nhân viên được gác bằng đúng một **mã quyền** dạng `nhóm.việc` — `machines.remote`, `products.update`, `backups.create`… Danh mục có **140 mã**, cộng mã `*` (toàn quyền); nguồn duy nhất là `backend/internal/authz/catalog.go`, cả cài mới lẫn nâng cấp đều lấy danh mục từ đó. Một **vai trò** là một tập mã. Một tài khoản có thể mang **nhiều vai trò** và được hợp các mã của chúng.

Menu cũng lọc theo mã: trang nào vai trò không có mã `….view` tương ứng thì không hiện (riêng Sao lưu, Nhật ký hoạt động, Thông báo và Quản lý hệ thống còn đòi thêm `client.admin`). Nhưng ẩn menu chưa phải là chặn — đọc [Điều dễ nhầm](#điều-dễ-nhầm-khi-phân-quyền) trước khi tin vào nó.

## Nơi xem và sửa quyền

Ở nhóm menu **Quản lý hệ thống** — nhóm này chỉ hiện với ai có cả `client.admin` lẫn `system.users.view`, mặc định là chủ:

- **Quản lý người dùng** — tạo tài khoản nhân viên, chọn vai trò (chọn được nhiều), đổi trạng thái.
- **Quản lý vai trò** — bấm **Sửa** ở dòng vai trò, rồi **Phân quyền**. Hộp thoại là một cây: mỗi nhánh là một *nhóm* mang tên kỹ thuật (`machines`, `backoffice`… xếp theo chữ cái), mỗi ô là *Tên (mã)*. Tick nhánh cha là tick cả nhóm.
- **Quản lý menu** — chỉ để xem; cây menu viết cứng trong mã nguồn.

## Ba vai trò có sẵn

`seed` tạo ba vai trò và ba tài khoản trùng tên (`admin` mang vai trò `owner`).

| Vai trò | Mã | Ý định |
|---|---|---|
| **owner** — Chủ | `*` và cả 140 mã | Toàn quyền. `*` bỏ qua mọi lần kiểm |
| **manager** — Quản lý | 117 mã: tất cả **trừ** `*`, `client.admin`, nhóm `system` và nhóm `backoffice` | Lo toàn bộ việc kinh doanh hằng ngày — giá, khuyến mãi, sản phẩm, báo cáo, điều khiển màn hình máy, game, cập nhật máy khách — nhưng không chạm back-office |
| **staff** — Nhân viên | 52 mã, xem cột *Nhân viên có* ở bảng dưới | **Vận hành** quầy chứ không **cấu hình**: xem được hầu hết mọi thứ, mở/trả/chuyển máy, nạp tiền, bán gói, nhận đặt chỗ, làm đơn và thu tiền, kiểm kê, mở/đóng/bàn giao ca — nhưng không đổi giá, không sửa khuyến mãi, không xoá dữ liệu nền |

Nhìn từ menu:

- **owner** thấy tất cả.
- **manager** không thấy **Nhật ký hoạt động**, **Nhật ký lỗi**, **Thông báo**, **Sao lưu**, và cả nhóm **Quản lý hệ thống**.
- **staff** không thấy thêm **Báo cáo** và **Cập nhật máy khách**. Nhân viên vẫn thấy **Cài đặt**, **Máy in**, **Chặn website**, **Cập nhật game**, nhưng gần như chỉ để xem.

## Danh sách mã quyền

Hai cột cuối là cấp mặc định của `seed`. Trong cột *Nhân viên có*, chỗ nhóm chỉ có một tiền tố thì ghi phần sau dấu chấm (`view` là `members.view`, `topup` là `members.topup`…).

| Nhóm (nhánh trong hộp Phân quyền) | Các mã | Nhân viên có | Quản lý có |
|---|---|---|---|
| **Hội viên** (`members`) | `members.view` · `members.create` · `members.update` · `members.delete` · `members.topup` · `members.refund` · `members.reset_password` · `members.refresh_tiers` | `view`, `create`, `topup` | tất cả |
| **Nhóm hội viên** (`member_groups`) | `member_groups.view` · `member_groups.create` · `member_groups.update` · `member_groups.delete` | `view` | tất cả |
| **Máy** (`machines`) | `machines.view` · `machines.create` · `machines.update` · `machines.delete` · `machines.remote` · `machines.remote_desktop` | `view`, `remote` | tất cả |
| **Nhóm máy** (`machine_groups`) | `machine_groups.view` · `machine_groups.create` · `machine_groups.update` · `machine_groups.delete` | `view` | tất cả |
| **Tài sản máy** (`machine_assets`) | `machine_assets.view` · `machine_assets.create` · `machine_assets.update` · `machine_assets.delete` | `view` | tất cả |
| **Bảng giá giờ chơi** (`pricing`) | `pricing.view` · `pricing.create` · `pricing.update` · `pricing.delete` | `view` | tất cả |
| **Phiên chơi** (`sessions`) | `sessions.view` · `sessions.start` · `sessions.end` · `sessions.switch` · `sessions.delete` | `view`, `start`, `end`, `switch` | tất cả |
| **Gói dịch vụ** (`combos`) | `combos.view` · `combos.create` · `combos.update` · `combos.delete` · `combos.sell` | `view`, `sell` | tất cả |
| **Đặt chỗ** (`bookings`) | `bookings.view` · `bookings.create` · `bookings.update` · `bookings.delete` · `bookings.checkin` | `view`, `create`, `update`, `checkin` | tất cả |
| **Khuyến mãi** (`promotions`) | `promotions.view` · `promotions.create` · `promotions.update` · `promotions.delete` | `view` | tất cả |
| **Vòng quay may mắn (trang Khuyến mãi)** (`lucky_spin`) | `lucky_spin.view` · `lucky_spin.create` · `lucky_spin.update` · `lucky_spin.delete` · `lucky_spin.spin` | `view`, `spin` | tất cả |
| **Giới nghiêm** (`curfew`) | `curfew.view` · `curfew.create` · `curfew.update` · `curfew.delete` · `curfew.override` | `view` | tất cả |
| **Danh mục** (`categories`) | `categories.view` · `categories.create` · `categories.update` · `categories.delete` | `view` | tất cả |
| **Sản phẩm** (`products`) | `products.view` · `products.create` · `products.update` · `products.delete` · `products.ingredients` | `view` | tất cả |
| **Đơn hàng** (`orders`) | `orders.view` · `orders.create` · `orders.update` · `orders.delete` · `orders.status` · `orders.split` · `orders.pay` · `orders.print` | `view`, `create`, `update`, `status`, `split`, `pay`, `print` | tất cả |
| **Cập nhật game** (`games`) | `games.view` · `games.create` · `games.update` · `games.delete` | `view` | tất cả |
| **Cập nhật máy khách** (`app_updates`) | `app_updates.view` · `app_updates.create` · `app_updates.update` · `app_updates.delete` | — | tất cả |
| **Chặn website** (`website_rules`) | `website_rules.view` · `website_rules.create` · `website_rules.update` · `website_rules.delete` | `view` | tất cả |
| **Đánh giá dịch vụ** (`feedback`) | `feedback.view` | tất cả | tất cả |
| **Điểm danh** (`attendance`) | `attendance.view` | tất cả | tất cả |
| **Kho: kiểm kê, sổ kho, đơn vị tính** (`inventory`) | `inventory_counts.view` · `inventory_counts.open` · `inventory_counts.count` · `inventory_counts.commit` · `inventory_counts.cancel` · `stock.view` · `stock.create` · `units.view` | `inventory_counts.view`, `inventory_counts.open`, `inventory_counts.count`, `stock.view`, `stock.create`, `units.view` | tất cả |
| **Máy in** (`printers`) | `printers.view` · `printers.create` · `printers.update` · `printers.delete` · `printers.test` | `view`, `test` | tất cả |
| **Nhà cung cấp** (`suppliers`) | `suppliers.view` · `suppliers.create` · `suppliers.update` · `suppliers.delete` | `view` | tất cả |
| **Ca làm việc** (`shifts`) | `shifts.view` · `shifts.open` · `shifts.close` · `shifts.handover` | tất cả | tất cả |
| **Giao dịch và báo cáo** (`reports`) | `transactions.view` · `reports.view` | `transactions.view` | tất cả |
| **Cài đặt** (`settings`) | `settings.view` · `settings.edit` | `view` | tất cả |
| **Phòng chat** (`chat`) | `chat.moderate` | — | tất cả |
| **Tải tệp** (`files`) | `upload.file` | tất cả | tất cả |
| **Tài khoản và vai trò, menu** (`system`) | `system.users.view` · `system.users.create` · `system.users.update` · `system.users.delete` · `system.roles.view` · `system.roles.create` · `system.roles.update` · `system.roles.delete` · `system.roles.permissions` · `system.menus.view` | — | — |
| **Nhật ký, sao lưu, thông báo hàng loạt** (`backoffice`) | `audit.view` · `errors.view` · `errors.resolve` · `backups.view` · `backups.create` · `backups.restore` · `backups.delete` · `notifications.view` · `notifications.create` · `notifications.update` · `notifications.delete` · `notifications.dispatch` | — | — |
| **Cổng back-office** (`client`) | `client.admin` | — | — |

Mã khó đoán từ tên:

- `machines.remote` — các mục **Bật máy**, **Chụp màn hình**, **Xem ứng dụng**, **Thông báo**, **Khởi động lại**, **Tắt máy** trong menu **Khác** của trang Máy. `machines.remote_desktop` riêng cho **Điều khiển màn hình** — xem [Điều khiển máy từ xa](#điều-khiển-máy-từ-xa-nút-khác).
- `games.update` — **Sửa**, **Publish ngay**, **Tải ngay**, công tắc **Tự cập nhật**/**Bật** và ô **Ưu tiên** của trang [Cập nhật game](#cập-nhật-game).
- `products.update` — gồm cả **đổi giá**. `products.ingredients` — sửa định lượng nguyên liệu của một món.
- `orders.status` — xác nhận, duyệt, huỷ đơn và đổi trạng thái từng món; `orders.update` — sửa đơn; `orders.print` — in hoá đơn và phiếu chế biến.
- `members.refresh_tiers` — nút xếp lại hạng hội viên; `members.reset_password` — đặt lại mật khẩu hội viên.
- `sessions.switch` — chuyển phiên sang máy khác; `combos.sell` — bán gói cho hội viên; `bookings.checkin` — nhận khách đặt chỗ; `lucky_spin.spin` — quay thưởng hộ khách; `curfew.override` — miễn trừ giới nghiêm.
- `inventory_counts.commit` / `inventory_counts.cancel` — chốt / huỷ phiếu kiểm kê. Nhân viên mặc định chỉ mở phiếu và ghi số đếm.
- `transactions.view` — trang **Giao dịch**; `reports.view` — trang **Báo cáo**.
- `chat.moderate` — xoá phòng chat.
- `upload.file` — tải ảnh lên (jpg, png, gif, webp): ảnh sản phẩm, ảnh giấy tờ và giấy đồng ý của phụ huynh ở hồ sơ hội viên, ảnh kiểm tra ở trang Tài sản máy. Quản lý và nhân viên có sẵn.
- `client.admin` — cổng vào back-office: Sao lưu, Nhật ký hoạt động, Thông báo và Quản lý hệ thống đòi mã này cộng mã riêng của từng trang, xem điều dễ nhầm số 1 bên dưới.

## Điều dễ nhầm khi phân quyền

**1. Trang back-office đòi hai mã, không phải một.** Sao lưu, Nhật ký hoạt động, Thông báo (soạn và gửi) và Quản lý hệ thống: API của chúng kiểm **cả** `client.admin` **lẫn** mã riêng (`backups.view`, `audit.view`, `notifications.view`, `system.users.view`…), và menu cũng đòi cả hai. Chủ qua hết nhờ `*`. Vai trò tự tạo phải tick **cả hai** (`client.admin` ở nhánh `client`): chỉ có mã riêng, hay chỉ có `client.admin`, thì menu không hiện và gọi API bị `Insufficient permissions`. Nhóm **Quản lý hệ thống** hiện cả ba trang Người dùng, Vai trò, Menu khi có `system.users.view`; trang Vai trò và Menu vẫn cần thêm `system.roles.view` và `system.menus.view` mới mở được. Riêng **Nhật ký lỗi** (`errors.view`) không đòi `client.admin`.

**2. Cổng back-office mặc định chỉ mở cho chủ.** Quản lý và nhân viên không có `client.admin` (quản lý cũng không có các mã nhóm `system` và `backoffice`), nên không vào được Sao lưu, Nhật ký hoạt động, Nhật ký lỗi, Thông báo và Quản lý hệ thống. Muốn giao việc này cho một người, tạo vai trò riêng với `client.admin` cộng các mã cần dùng, đừng nới quyền của `staff` hay `manager`. Cẩn thận nhất với `system.roles.permissions`: người có mã này cùng `client.admin` tự tick được `*` cho vai trò của chính mình (đã thử).

**3. Tải ảnh lên cần `upload.file`.** Quản lý và nhân viên có sẵn mã này (nhóm `files`). Vai trò tự tạo thì không: thêm ảnh sản phẩm, ảnh giấy tờ hội viên hay ảnh tài sản máy sẽ bị từ chối `Insufficient permissions` cho tới khi bạn tick nhánh `files`. Máy chủ nâng cấp từ bản cũ — nơi `upload.file` còn nằm trong nhóm `system` nên `manager` và `staff` chưa có — được `migrate` cấp bù đúng một lần; ai đã tự cấp thì giữ nguyên.

**4. Ẩn menu và ẩn nút không phải là chặn.** Máy chủ mới là nơi chặn: thiếu mã thì API trả 403 và trang hiện thông báo đỏ `Insufficient permissions`. Nút thì chỉ ẩn theo quyền ở **Máy**, **Bảng điều khiển** và **Đơn hàng**; ở các trang khác (Sản phẩm, Khuyến mãi, Cài đặt…) nhân viên vẫn thấy nút **Thêm**, **Sửa**, **Lưu** và bấm vào sẽ nhận lỗi đó. Ví dụ: nhân viên mở được **Cài đặt** (`settings.view`), nhưng **Lưu** bị từ chối vì thiếu `settings.edit`.

**5. Ba mã chỉ che menu hoặc nút, không chặn API.** Danh sách sản phẩm và danh mục (`GET /api/products`, `/api/categories`) mở cho mọi tài khoản đã đăng nhập vì máy trạm dựng thực đơn từ đó, nên `categories.view` chỉ ẩn hiện mục **Danh mục**, còn `products.view` chỉ gác danh sách nguyên liệu của món. `orders.create` chỉ ẩn hiện nút **Thêm** ở trang Đơn hàng: API tạo đơn là cửa chung của khách và nhân viên, không kiểm mã. Mọi mã còn lại, kể cả `orders.view`, `orders.pay`, `orders.status`, đều được máy chủ kiểm.

**6. Đổi quyền chưa có hiệu lực ngay.** Quyền được ghi vào thẻ đăng nhập lúc cấp. Lưu xong ở hộp **Phân quyền**, người đang đăng nhập vẫn giữ quyền cũ cho tới khi thẻ được làm mới (hết hạn mặc định sau 24 giờ, đổi bằng `JWT_ACCESS_TTL`) hoặc họ đăng nhập lại. Muốn có hiệu lực ngay, bảo họ **đăng xuất rồi đăng nhập lại**.

**7. Chạy lại `seed` đặt lại quyền của cả ba vai trò về mặc định.** Mọi chỉnh sửa trong **Phân quyền** của `owner`, `manager`, `staff` mất — kể cả khi bạn chạy lại chỉ để thêm thực đơn mẫu bằng `seed -menu`, vì lệnh đó cũng chạy phần chính. Vai trò tự tạo không bị đụng. `migrate` thì chạy lại được: nó chỉ thêm mã mới vào danh mục, cấp mặc định cho `manager` và `staff` đúng một lần, và không gỡ gì ai đã tự cấp. Bộ cài Windows khi nâng cấp chỉ chạy `migrate`, không chạy `seed`.

## Hội viên khác nhân viên

Khách đăng nhập từ máy trạm nhận một loại thẻ riêng, không phải thẻ nhân viên. Mọi API quản trị đều từ chối thẻ đó với `403 Staff access required`. Khách chỉ đọc được dữ liệu của chính mình (hồ sơ, giao dịch, phiên chơi, đơn), cộng những thứ máy trạm cần để vẽ màn hình: thực đơn, danh mục, mệnh giá nạp và các phương thức thanh toán.

> **Đã kiểm khi viết mục này**, đợt đầu trên bản Docker dựng trước khi sửa `client.admin` và `upload.file`, đăng nhập `admin`, `manager`, `staff`:
> - gọi cả 196 route có mã quyền bằng từng tài khoản rồi so với danh mục — khớp hết, trừ ba route game (`DELETE /api/games/:id`, `POST /api/games/:id/publish`, `POST /api/games/:id/sync`) bị từ chối vì vai trò **master / quán** của máy chủ chứ không vì mã quyền;
> - menu của ba tài khoản qua `GET /api/route/getUserRoutes`; hộp **Phân quyền** và nút **Lưu** của nhân viên trên giao diện thật;
> - thẻ cũ sau khi đổi quyền, và sau khi làm mới thẻ; tài khoản mang hai vai trò nhận hợp các mã; vai trò có `system.roles.permissions` cùng `client.admin` tự tick `*`;
> - chạy lại `seed` và `migrate` sau khi gỡ một mã khỏi `manager`; thẻ hội viên gọi cả 196 route trên (đều 403).
>
> Các mục 1 và 3 viết lại sau khi sửa hai mã đó, kiểm riêng trên bản Docker dựng lại, chỉ bằng API (không chạy lại cả 196 route; `POST /api/upload` là route duy nhất gác bằng `upload.file`):
> - `POST /api/upload`: `admin`, `manager`, `staff` đều 200; vai trò thử chỉ có `members.view` bị 403;
> - vai trò thử chỉ có `backups.view`: menu không có **Sao lưu**, `GET /api/backups` 403; thêm `client.admin` thì menu hiện và API 200; chỉ có `client.admin` thì không hiện và 403. Với Nhật ký hoạt động, Thông báo và nhóm Quản lý hệ thống: chỉ có mã riêng thì ẩn menu và 403, thêm `client.admin` thì hiện và 200; **Nhật ký lỗi** hiện và gọi được chỉ với `errors.view`;
> - nâng cấp, **mô phỏng bằng SQL** (kéo `upload.file` về nhóm `system`, gỡ nó khỏi `manager` và `staff`, xoá hai mốc `move_upload_file_to_files_module` và `grant_upload_file_v1`, cho một vai trò tự tạo `upload.file`): sau `migrate`, mã nằm ở nhóm `files`, `manager` và `staff` có lại mã, vai trò tự tạo không đổi; chạy `migrate` lần hai thì bỏ qua cả hai mốc. Chưa thử trên một bản cài thật được nâng cấp.
>
> **Đọc từ mã, chưa chạy:** bộ cài Windows nâng cấp chỉ chạy `migrate` (`installer/vnet-server-install.ps1`).

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

Hệ thống gọi `pg_dump` chạy nền. Tệp lưu trong thư mục `BACKUP_DIR`, mặc định là `backups` cạnh nơi chạy máy chủ. Với bộ cài Windows, dịch vụ tự đặt nó là `C:\VNET\data\backups` và đã kèm sẵn `pg_dump`/`pg_restore` — không cài gì thêm, nhưng vẫn nên chép tệp ra ổ khác.

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

Xem nhật ký: `docker compose logs app` (bộ cài Windows: `C:\VNET\logs\server.log`; xem thêm [mục sự cố riêng của bộ cài Windows](#bộ-cài-máy-chủ-windows-báo-lỗi-hoặc-máy-chủ-không-lên) ngay dưới).

| Dòng thấy trong nhật ký | Nguyên nhân | Sửa |
|---|---|---|
| `Refusing to start: JWT_SECRET is still the development placeholder` | Chưa đổi `JWT_SECRET` | Đặt chuỗi thật, ≥ 32 ký tự |
| `Refusing to start: JWT_SECRET must be at least 32 characters` | Chuỗi quá ngắn | `openssl rand -base64 48` |
| `Refusing to start: ALLOWED_ORIGINS must list the admin origins` | Để trống | Điền địa chỉ công khai |
| `Refusing to start: ALLOWED_ORIGINS must not contain "*"` | Dùng `*` | Liệt kê từng địa chỉ |
| Compose báo thiếu biến | Thiếu một trong `DB_PASSWORD`, `JWT_SECRET`, `ALLOWED_ORIGINS` | Điền vào `.env` |

### Bộ cài máy chủ Windows báo lỗi, hoặc máy chủ không lên

Chỉ cho đường [1.2a](#12a-cách-2--bộ-cài-windows-không-cần-docker). Nhật ký cài đặt ở `C:\VNET\logs\install.log`, nhật ký máy chủ ở `C:\VNET\logs\server.log`. Dòng báo lỗi của bộ cài bắt đầu bằng `LOI:` (không dấu):

| Thấy | Nguyên nhân | Sửa |
|---|---|---|
| Hộp thoại *"Cài Visual C++ Runtime thất bại (mã N)"* | `setup\vc_redist.x64.exe` không cài được; mã `N` là mã thoát của nó | Khởi động lại máy chủ rồi chạy lại bộ cài; còn lỗi thì tra mã `N` của Microsoft |
| `LOI: PostgreSQL khong len sau 30 giay` | PostgreSQL không bật kịp trong 30 giây (máy chậm, antivirus quét ổ, cổng 25432 bị chiếm...) | Xem `C:\VNET\data\pgdata\log` (chính thông báo chỉ chỗ này); chạy lại bộ cài |
| `LOI: VNET Server khong tra loi sau 60 giay` kèm vài dòng cuối `server.log` | Máy chủ không lên. Hay gặp: cổng 20800 bị chương trình khác chiếm, hoặc `config.env` sai luật ở [1.5](#15-ba-luật-khiến-máy-chủ-từ-chối-khởi-động) | Đọc dòng `Refusing to start:` trong `server.log`; đổi `SERVER_PORT` trong `config.env` rồi chạy lại bộ cài |
| `LOI: Database tao bang PostgreSQL X, bo cai mang PostgreSQL Y` | Bộ cài mới mang bản chính PostgreSQL khác với database đang có | Cài bộ cài cùng bản chính, hoặc `pg_upgrade` database trước |
| `LOI: Khong khoa duoc quyen thu muc ...` | Bộ cài không đặt được quyền cho thư mục cài (`icacls` báo lỗi) | Chạy bộ cài bằng tài khoản quản trị rồi thử lại; xem `install.log` để biết thư mục nào |
| Cài xong, máy khác trong mạng không mở được `http://<IP>:20800` nhưng máy chủ tự mở được | Luật tường lửa `VNET-Server` bị tắt hoặc xoá, hoặc phần mềm tường lửa khác chặn | Chạy lại bộ cài — nó tạo lại luật mỗi lần; kiểm tường lửa của antivirus |
| Sau khi bật máy chủ, trang không mở trong vài phút đầu | Dịch vụ **VNET Server** phải chờ **VNETPostgres** lên; nếu database chưa sẵn sàng máy chủ tự thoát và Windows bật lại sau 10 giây | Chờ một lúc; vẫn không lên thì xem `server.log` |

### Máy hiện Ngoại tuyến

Đi lần lượt:

1. **Dịch vụ có chạy không?** Trên máy đó mở `services.msc`, tìm **VNET Client Agent**. Không thấy thì chạy `vnet-client.exe --install-service`.
2. **Mã máy có khớp không?** So mã trong `config.json` cạnh tệp `.exe` với mã ở trang **Máy**. Sai một ký tự là nhịp tim không khớp vào đâu cả.
3. **Địa chỉ máy chủ đúng chưa?** Trên máy đó mở trình duyệt vào `http://<địa chỉ máy chủ>:20800/api/health`. Không ra gì thì là chuyện mạng hoặc tường lửa.
4. **Máy chủ có thấy nhịp tim không?** `docker compose logs -f app` (bộ cài Windows: mở `C:\VNET\logs\server.log`) rồi xem có dòng nào nhắc mã máy đó.

Máy chỉ cần im 45 giây là bị đánh ngoại tuyến, nên mạng chập chờn cũng đủ gây ra.

### Bấm nút điều khiển thì báo "chưa kết nối"

Máy trạm không giữ kết nối tới máy chủ. Thường là dịch vụ nền đã bị tắt. Bấm **Bật** dịch vụ trong `services.msc` — người dùng thường cũng bật được, không cần tài khoản quản trị.

Lưu ý: **lệnh không tới nơi thì phiên chơi giữ nguyên**. Hệ thống không kết phiên khi chưa chắc khách đã rời máy.

### Bật máy (Wake-on-LAN) báo thành công mà máy không dậy

Hệ thống chỉ biết gói "đánh thức" đã phát ra khỏi máy chủ, không biết máy có nhận được không. Kiểm lần lượt:

1. **BIOS và card mạng** của máy trạm đã bật Wake-on-LAN chưa, và Windows đã tắt Fast Startup chưa. Thiếu một trong ba thì gói tới nơi cũng không có máy nào thức.
2. **Máy chủ có cùng mạng LAN với máy trạm không.** Gói chỉ đi trong mạng con `/24` của IP máy từng báo (và tới `255.255.255.255`); khác mạng con hoặc phải qua router thì không tới.
3. **Máy chủ chạy trong Docker?** Gói phát từ trong container có thể không ra tới mạng quán. Chưa kiểm tra.
4. Báo *"máy chưa báo địa chỉ MAC"* thì máy chưa từng chạy phần mềm VNET: bật tay một lần cho nó báo địa chỉ MAC.

### Điều khiển màn hình không mở được

Lỗi hiện màu đỏ ở đầu cửa sổ. Theo câu báo:

| Câu báo | Nguyên nhân | Sửa |
|---|---|---|
| *máy trạm chưa kết nối* | Máy đang ngoại tuyến | Xem [Máy hiện Ngoại tuyến](#máy-hiện-ngoại-tuyến) |
| *máy chưa báo địa chỉ IP, chưa thể điều khiển từ xa* | Máy chưa gửi IP ở nhịp tim nào | Chờ một nhịp tim (15 giây) sau khi máy lên |
| *không nối được VNC trên máy ...: máy trạm chưa cài TightVNC hoặc tường lửa chặn* | Bộ cài máy trạm được build mà **không có `tightvnc.msi`**, hoặc dịch vụ nền chưa cấu hình xong TightVNC, hoặc một tường lửa khác chặn cổng 5900 | Mở `service.log` của máy ([2.6c](#26c-nhật-ký-máy-trạm)), tìm dòng `[VNC]`. Dòng *chưa có TightVNC và không thấy tightvnc.msi cạnh vnet-client.exe* nghĩa là thiếu tệp — xem [Điều khiển máy từ xa](#điều-khiển-máy-từ-xa-nút-khác) |
| *VNC từ chối mật khẩu — chờ nhịp tim kế tiếp rồi thử lại* | Máy vừa nhận mật khẩu VNC mới | Chờ khoảng 15 giây rồi bấm **Kết nối lại** |

Không thấy mục **Điều khiển màn hình** trong menu **Khác** là do tài khoản chưa có quyền `machines.remote_desktop` (vai trò nhân viên mặc định không có).

### Chụp màn hình hoặc Xem ứng dụng chờ mãi không ra

Lệnh đã tới máy nhưng không có gì trả về. Hai lệnh này (cùng **Thông báo**) do **giao diện** máy trạm làm, không phải dịch vụ nền: giao diện không chạy hoặc đang treo thì không có ảnh. Mở `%LOCALAPPDATA%\VNET\logs\ui.log` trên máy đó ([2.6c](#26c-nhật-ký-máy-trạm)), tìm dòng `[remote] chụp màn hình thất bại` hoặc `[remote] gửi ảnh lên thất bại`. Giao diện bị tắt thì dịch vụ nền tự bật lại.

### Game kẹt ở Lỗi, Chờ tải, hoặc quán không thấy bản mới

| Thấy | Nguyên nhân hay gặp | Sửa |
|---|---|---|
| Trang chỉ hiện *"Máy chủ này chưa bật cập nhật game"* | Chưa đặt `GAME_ROLE`, hoặc chưa khởi động lại máy chủ sau khi sửa cấu hình | [1.7](#17-bật-cập-nhật-game-tuỳ-chọn) |
| Máy chủ không lên, nhật ký có `Refusing to start: GAME_...` | Thiếu hoặc sai một biến `GAME_*` bắt buộc | Bảng biến ở [1.7](#17-bật-cập-nhật-game-tuỳ-chọn) |
| Quán không thấy game mới, bản vẫn cũ, **trang không báo lỗi gì** | Quán không gọi được master, hoặc hai bên khác `GAME_CATALOG_KEY`. Chỉ nhật ký máy chủ có dòng `[game] đọc catalog của master: ...` | Từ máy chủ quán mở `<GAME_UPSTREAM_URL>/api/health` trong trình duyệt; so lại khoá hai bên |
| Cột Phiên bản hiện `1 → 2` mà đứng im | Giờ hiện tại nằm ngoài **Khung giờ tự cập nhật**, hoặc công tắc **Tự cập nhật** của game đang tắt | Bấm **Tải ngay**, hoặc sửa khung giờ |
| Lỗi *"Tải bị treo: ... không có thêm dữ liệu nào đúng hash"* | Master không phản hồi, hoặc master đổi tệp sau khi publish nên không mảnh nào khớp | Chờ máy tự thử lại (5, 10, 20 phút...) hoặc bấm **Tải ngay**. Bên master bấm **Publish ngay** để chốt lại bản. Đường truyền chậm hơn khoảng 7 kB/s thì tăng `GAME_STALL_TIMEOUT` |
| Ở master, game mới thêm vẫn **Chưa có** | Thư mục chưa đứng yên đủ `GAME_STABLE_FOR` | Bấm **Publish ngay** |
| Khách bấm **Chơi** báo không tìm thấy tệp chạy trên ổ game | Ô **Thư mục game trên máy trạm** sai chữ ổ, tệp chạy sai đường dẫn, hoặc máy diskless chưa khởi động lại sau lần cập nhật | So ô đó với chữ ổ game mà máy trạm thật sự thấy; khởi động lại máy |

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
