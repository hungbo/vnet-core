# VNET — Bắt đầu nhanh

Đọc từ trên xuống, khoảng 30 phút. Cuối bài bạn có một máy chủ đang chạy, một PC khách nối vào đó và một phiên khách thử tính tiền đúng.

**Cần có**
- **Một máy làm máy chủ**, bật suốt giờ mở cửa, cùng mạng LAN với các PC khách.
- **Một PC Windows** để thử làm máy trạm.
- **Địa chỉ IP của máy chủ** trong mạng quán — gõ `ipconfig` trong Command Prompt, lấy dòng IPv4, ví dụ `192.168.1.10`. Dưới đây viết là `<IP>`.

## Chọn đường dựng máy chủ

| Máy chủ của bạn | Đi đường |
|---|---|
| **Windows** (Windows Server 2016 trở lên, hoặc Windows 10/11 64-bit). Windows Server 2019 không chạy được container Linux, nên đi đường này | **Đường A** — bộ cài `.exe`, không cần Docker |
| Linux, Mac, hoặc máy đã cài Docker | **Đường B** — Docker |

Xong một đường thì nhảy tới [Cài máy trạm](#cài-máy-trạm-lên-một-pc-10-phút).

## Đường A — Máy chủ Windows, bộ cài .exe (10 phút)

1. **Lấy bộ cài** `vnet-server-setup-<phiên bản>.exe`. Tải ở mục **Releases** của repo GitHub nếu đã có bản phát hành. Chưa có thì tự build trên Mac hoặc Linux (cần Go, pnpm 10, `makensis`): `bash scripts/build-server-windows.sh 1.2.0`, ra `dist/vnet-server-setup-1.2.0.exe` — chi tiết ở [HUONG-DAN.md, mục 1.2a](./HUONG-DAN.md#12a-cách-2--bộ-cài-windows-không-cần-docker). Chép tệp sang máy chủ.
2. **Chạy bộ cài** trên máy chủ: bấm đúp, đồng ý quyền quản trị, giữ thư mục mặc định `C:\VNET`, bấm tiếp tới hết. Nó tự cài cơ sở dữ liệu PostgreSQL, hai dịch vụ Windows chạy mỗi khi bật máy, và mở cổng 20800 trong tường lửa. Chữ trong ô chi tiết **không dấu** — cố ý.
3. **Chép địa chỉ máy trạm** ở dòng cuối ô chi tiết: `Xong. Dia chi cho may tram: http://192.168.x.x:20800`. Đó chính là `<IP>`.
4. **Lấy mật khẩu**: mở `C:\VNET\data\initial-admin-password.txt` (chỉ tài khoản quản trị Windows mở được). Ba dòng `admin`, `manager`, `staff`, **mỗi tài khoản một mật khẩu khác nhau**.
5. **Đăng nhập**: bấm lối tắt **VNET Quản lý** trên Desktop (hoặc mở `http://<IP>:20800`), tài khoản `admin`. Rồi bấm tên tài khoản ở góc trên phải → **Đổi mật khẩu**, và **xoá tệp mật khẩu** ở bước 4.

**Xong khi:** từ một máy khác trong mạng, mở `http://<IP>:20800` thấy trang đăng nhập **VNET Quản Lý**. Không vào được thì chạy lại bộ cài — nó tạo lại luật tường lửa. Bảng điều khiển toàn số 0 là đúng, chưa có gì để đếm.

## Đường B — Máy chủ Docker (10 phút)

Cần cài [Docker](https://docs.docker.com/get-docker/) (Docker Desktop nếu là Windows hoặc Mac). Gõ lệnh vào Terminal (Windows: dùng PowerShell).

1. **Tải mã nguồn và tạo tệp cấu hình.** Chưa cài Git thì vào trang GitHub của repo, bấm **Code → Download ZIP**, giải nén rồi mở Terminal trong thư mục đó thay cho hai lệnh đầu.

   ```bash
   git clone https://github.com/hungbo/vnet-core.git
   cd vnet-core
   cp .env.docker.example .env
   ```

2. **Điền ba dòng** vào tệp `.env` (mở bằng Notepad) rồi lưu. Thiếu một dòng thì Docker từ chối chạy — cố ý.

   ```
   DB_PASSWORD=<mật khẩu cơ sở dữ liệu, tự đặt>
   JWT_SECRET=<một chuỗi lộn xộn dài ít nhất 32 ký tự>
   ALLOWED_ORIGINS=http://<IP>:20800
   ```

   Chỉ dùng chữ và số cho hai chuỗi đầu (tránh khoảng trắng, `$`, `#`). `JWT_SECRET` là chuỗi ký thẻ đăng nhập: đổi nó sau này là đăng xuất tất cả mọi người.
3. **Chạy ba lệnh, đúng thứ tự này:**

   ```bash
   docker compose up -d --build
   docker compose --profile tools run --rm migrate
   docker compose --profile tools run --rm seed
   ```

   Lệnh đầu mất vài phút. `migrate` có thể in vài dòng đỏ `record not found` — bình thường; xong khi thấy `Migrations completed`, và `seed` in `Seed completed!`.
4. **Đăng nhập** `admin` / `admin123` tại `http://<IP>:20800`. Hộp **Bắt buộc đổi mật khẩu** hiện ra và không đóng được: nhập `admin123` ở *Mật khẩu hiện tại*, rồi mật khẩu mới (ít nhất 8 ký tự) hai lần, **Xác nhận**. Đăng xuất, làm lại với `manager` và `staff` (cũng `admin123`) — ba mật khẩu mặc định này công khai trong mã nguồn.

**Xong khi:** từ một máy khác trong mạng, mở `http://<IP>:20800` thấy trang đăng nhập. Không vào được thì mở cổng TCP 20800 trong tường lửa của máy chủ.

## Cài máy trạm lên một PC (10 phút)

Lấy bộ cài `vnet-client-setup-<phiên bản>.exe` ở mục **Releases** của repo GitHub nếu đã có bản phát hành; chưa có thì tự dựng theo [HUONG-DAN.md, mục 2.1](./HUONG-DAN.md#21-tạo-bộ-cài).

1. Chạy bộ cài trên PC với quyền quản trị. Trang **Cấu hình máy trạm** hỏi bốn ô: **Địa chỉ máy chủ** (`http://<IP>:20800`), **Tài khoản quản trị máy trạm** (để `admin`), **Mật khẩu** (ít nhất 6 ký tự) và nhập lại. Tài khoản này là lối vào cho kỹ thuật khi mất mạng, không phải tài khoản trang quản trị.
2. Cài xong, PC hiện màn hình khoá phủ kín, **tên máy** ngay dưới logo. Mã máy chính là tên máy Windows.
3. Trên trang quản trị, vào **Quản lý → Máy**. Trong vòng 15 giây PC tự hiện ra ở trạng thái **Sẵn sàng**, kèm nhãn vàng **Chưa gán nhóm — 0₫/giờ** ở cột **Nhóm**.
4. Bấm **Sửa** ở dòng đó, chọn **Nhóm** rồi **Lưu**. Bước này **không được bỏ**: chưa có nhóm thì khách đăng nhập bị từ chối với câu *"máy chưa được thêm vào hệ thống — báo quầy xếp nhóm cho máy này"*. Dữ liệu mẫu có sẵn nhóm **Thường** (5.000₫/giờ) và **VIP** (15.000₫/giờ); sửa giá ở **Quản lý → Nhóm máy → Bảng giá**. Có nhiều máy thì dùng **Tạo hàng loạt** (tiền tố, khoảng số, nhóm) — mã tạo ra phải trùng tên máy Windows.

⚠️ Mặc định, máy đứng ở màn hình khoá **5 phút không ai đăng nhập là tự tắt**. Thử mà thấy PC tắt là vì thế; đổi ở **Cài đặt → Máy trạm**.

## Thử một phiên khách (5 phút)

1. **Quản lý → Hội viên → Thêm**: nhập **Tên đăng nhập** (ví dụ `khach01`) và **Mật khẩu**, **Lưu**. Đừng đặt trùng `admin`, `manager`, `staff`: tên trùng nhân viên là vào máy **miễn phí**, không tính tiền.
2. Ở dòng khách bấm **Nạp**, để **Tiền mặt**, **Xác nhận** (mặc định 10.000₫).
3. Ra PC, gõ tên và mật khẩu khách vào màn hình khoá. Màn hình mở ra, đồng hồ bắt đầu chạy.
4. Về trang quản trị, **Quản lý → Máy**: máy chuyển **Đang sử dụng**, cột **Phiên** hiện tên khách, đã chơi bao lâu và còn bao lâu (10.000₫ ở giá 5.000₫/giờ là còn khoảng 2 giờ).
5. Kết thúc: khách bấm **Đăng xuất** trên PC, hoặc bạn vào **Quản lý → Phiên → Kết thúc**. Tiền tính theo phút chơi trọn; xem ở **Quản lý → Giao dịch**.

## Thử điều khiển từ xa (tuỳ chọn)

**Quản lý → Máy → Khác** ở dòng PC: **Chụp màn hình** cho ảnh màn hình PC; **Thông báo** hiện một dòng chữ lên PC. **Điều khiển màn hình** và **Bật máy (Wake-on-LAN)** cần chuẩn bị thêm trước — xem [Điều khiển máy từ xa](./HUONG-DAN.md#điều-khiển-máy-từ-xa-nút-khác).

## Tiếp theo

- **Việc hằng ngày** (mở ca, nạp tiền, bán hàng, chốt ca): [HUONG-DAN.md, phần A](./HUONG-DAN.md#a-việc-thường-ngày).
- **Vận hành máy chủ** — xem nhật ký, sửa cấu hình, nâng cấp, sao lưu, gỡ: bộ cài Windows ở [mục 1.2a](./HUONG-DAN.md#12a-cách-2--bộ-cài-windows-không-cần-docker), Docker ở [mục 1.2](./HUONG-DAN.md#12-cách-1--docker).
- **Cài đủ phòng máy**, tài khoản Windows cho khách, máy diskless: [phần 2](./HUONG-DAN.md#2-cài-máy-trạm-lên-pc-khách).
- **Cập nhật game cho máy trạm**: [mục 1.7](./HUONG-DAN.md#17-bật-cập-nhật-game-tuỳ-chọn).
- **Sự cố** (máy chủ không lên, máy hiện Ngoại tuyến, khách không đăng nhập được): [phần D](./HUONG-DAN.md#d-vận-hành-và-sự-cố).

Hệ thống không tự sao lưu: vào **Hệ thống → Sao lưu** và chép tệp ra ổ khác ([mục D.2](./HUONG-DAN.md#d2-sao-lưu)). Riêng Docker: dừng máy chủ mà giữ dữ liệu là `docker compose down`; **không** chạy `docker compose down -v` — lệnh đó xoá sạch cơ sở dữ liệu, không hỏi lại.
