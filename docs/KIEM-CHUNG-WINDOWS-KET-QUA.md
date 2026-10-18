# Kết quả kiểm chứng máy trạm Windows

## Vòng 3 — 01/10/2026: bảo vệ tiền giờ chơi cho máy diskless

Làm theo kế hoạch đã duyệt: can thiệp phần mềm thì khởi động lại, mất kết nối
máy chủ thì khoá rồi khởi động lại, card mạng lạ và tài khoản Windows quản trị
thì báo về trang quản trị. Bản máy trạm **v0.4.0**, kiểm trên cùng máy Windows
11 ARM, dưới **cả tài khoản quản trị lẫn tài khoản thường**.

### Đo được trên máy thật

| Bài kiểm | Kết quả |
|---|---|
| Vô hiệu hoá và dừng dịch vụ nền | Lớp canh ra lệnh khởi động lại sau 27 giây; nhật ký có đủ `[guard] ... (n/6 lượt)`; máy khởi động lại thật |
| Đóng băng tiến trình giao diện | Dịch vụ tắt nó và bật giao diện mới sau 17,6 giây |
| Mất máy chủ (ngưỡng thử 30 giây) | Màn hình khoá "Mất kết nối máy chủ — đang thử lại" sau 38 giây kể từ lúc máy chủ tắt; phím thoát bị chặn |
| Máy chủ trở lại trước ngưỡng khởi động lại | Tự mở sau 7 giây, **phiên cũ vẫn còn**, máy không khởi động lại |
| Mất máy chủ quá ngưỡng (thử 90 giây), có hội viên | Lệnh khởi động lại ở giây 86; máy khởi động lại thật |
| Sau khi khởi động lại, máy chủ vẫn tắt | Ở màn hình khoá, 3 phút sau **không** khởi động lại lần nữa |
| Thêm một card mạng (loopback, có IP) | Máy chủ ghi `Ethernet 2 (10.99.0.5)`, đúng **một** cảnh báo, một dòng nhật ký, không lặp ở các nhịp tim sau; máy không bị ngắt; gỡ card thì hết |
| Tài khoản Windows quản trị | `user_is_admin = true`, bộ lọc "Chỉ máy có cảnh báo" ra đúng máy |
| Tài khoản Windows thường | `user_is_admin = false`; giao diện chạy ổn định; hội viên đăng nhập, mở phiên, phím thoát bị chặn |
| Tài khoản thường thử `sc stop`, `sc config ... disabled` | Cả hai: Access is denied |
| Tài khoản thường thử sửa `config.json`, tạo `maintenance.flag` | Cả hai bị từ chối |
| Tài khoản thường tắt giao diện | Giao diện mới sau 1,8 giây, ở màn hình khoá |
| Đặt biến môi trường `VNET_SERVER_URL`, `VNET_GUARD=0` cho người dùng | Bị bỏ qua: đăng nhập vẫn đi đúng máy chủ trong `config.json` |
| Đổi chính sách ở Cài đặt → Máy trạm | Máy trạm nhận trong một nhịp tim và lưu vào `policy.json` |
| Quầy kết thúc phiên | Giao diện về màn hình khoá; dịch vụ không can thiệp oan |
| Trang quản trị (xem bằng trình duyệt) | Cột Cảnh báo, nhãn đỏ/vàng, ô lọc, tab Máy trạm lưu được |

### Kiểm tự động

| | Kết quả |
|---|---|
| Backend `go test ./...` | Xanh; thêm test chính sách máy trạm và hai trường nhịp tim mới |
| Máy trạm `go test ./...` | Xanh; thêm 9 test cho bộ giám sát, test card mạng, test bỏ qua biến môi trường |
| `golangci-lint` backend / lint admin / typecheck admin | 53 / 71 / 49, đúng bằng mốc cũ |

### Chưa kiểm trên máy thật

- **Bộ đếm "bật lại giao diện lần thứ ba trong 5 phút thì khởi động lại"** và **"phiên kết thúc mà giao diện không về màn hình khoá"**: chỉ có test tự động, vì không có cách dựng lại một giao diện hỏng kiểu đó trên máy thật.
- **Card mạng thật** (USB Wi-Fi, điện thoại phát mạng): đã thử bằng card loopback của Windows.
- **Máy diskless thật**: máy thử có ổ cứng. Riêng việc "khởi động lại là về bản gốc" là tính chất của hệ thống boot, không phải của VNET.
- Như các vòng trước: in hoá đơn, bộ cài Inno Setup, game toàn màn hình, QR, bàn phím thật trên x64.

### Ngoài phạm vi vòng này

Xác thực máy trạm với máy chủ bằng khoá riêng (các đường `machines/by-code/*`
vẫn nhận diện bằng mã máy) được tách thành kế hoạch riêng, chưa làm.

### Hai điều đổi so với kế hoạch

- **Chỉ khởi động lại vì mất kết nối khi có hội viên đang đăng nhập.** Máy chưa ai đăng nhập đã ở màn hình khoá, khởi động lại không được gì và sẽ thành vòng lặp khi máy chủ tắt qua đêm.
- **Bài test thời gian của lớp canh đổi từ "ít nhất một phút" xuống "ít nhất 30 giây"**, đúng con số trong kế hoạch.

---

## Vòng 2 — 30/09 → 01/10/2026: đã sửa và kiểm lại toàn bộ

Mọi lỗi của vòng 1 (bên dưới) đã được sửa và **kiểm lại trên chính máy Windows đó**,
cùng cách điều khiển từ xa. Bản cuối cùng là **v0.2.9**, và nó tự cài lên máy qua
chức năng cập nhật mới — không chép tay.

### Kết quả

| Lỗi vòng 1 | Kiểm lại thấy gì |
|---|---|
| N1 Đăng xuất không trả máy | Đăng xuất → phiên đóng ngay, máy Sẵn sàng, số dư đứng yên sau 70 giây |
| N2 Reboot là chơi miễn phí | Reboot giữa phiên → bật lên là màn hình khoá; phiên chốt đúng lúc máy tắt, sổ khớp |
| N3 Quầy trả máy mà máy trạm không khoá | Về màn hình khoá trong 3 giây, kèm "Phiên chơi đã kết thúc" |
| N4 Hết tiền thì máy kẹt | Về **màn hình đăng nhập** kèm "Hết số dư — vui lòng nạp thêm để chơi tiếp"; nạp thêm là đăng nhập lại được, không cần ai bấm Mở khoá |
| N5 Mở khoá xong chơi miễn phí | Không còn đường: hết phiên là đăng xuất. Thêm chốt chặn: xoá phiên thẳng trong CSDL (không sự kiện nào) → máy tự khoá sau 14 giây |
| N6 Khung chat phủ kín màn hình | Khoá hoặc hết tiền lúc đang mở Hỗ trợ → cửa sổ phụ tự đóng |
| N7 Ví bị trừ lố khi máy tắt | Mô phỏng mất điện: phiên 2 phút = 2.000₫, ví 4.500 → 2.500₫, phần lỡ trừ thêm được hoàn |
| V1 Hai nguồn địa chỉ máy chủ | Khôi phục lại hồ sơ trình duyệt cũ chứa địa chỉ sai: vẫn đăng nhập bình thường. Ô Cài đặt hiện đúng địa chỉ, chỉ xem |
| V2 Chặn ứng dụng không chặn gì | Chặn notepad → mở lên bị tắt trong 2 giây, danh sách giữ qua khởi động lại |
| V3 Tác vụ canh chừng | Ngay sau khi cài, không reboot: dừng dịch vụ + giết giao diện → tự bật lại sau 47 giây |
| V4 Cập nhật không thay được bản đang chạy | Máy tự cài 7 lần liên tiếp (0.2.1 → 0.2.9); đang có khách thì hoãn; băm sai thì từ chối; dọn sạch tệp cũ |
| V5 Không có nhật ký | `logs\service.log` và `%LOCALAPPDATA%\VNET\logs\ui*.log` |
| H1 Nạp giữa phiên đồng hồ không dài ra | 9:30 → 14:53 trong 2,5 giây |
| H2 Phiên do quầy mở đếm lên | Đếm ngược đúng |
| H3 Cửa sổ phụ đè thanh dọc | Căn giữa phần bên trái thanh dọc |
| H4 Mất luôn-nổi sau Mở khoá | Vẫn nổi trên, phím thoát vẫn bị chặn |
| H5 Chat nền trắng, chữ tiếng Anh | Tông tối, tiếng Việt |
| H6 Chữ tiếng Anh | "Tiền giờ máy PC-01", cảnh báo nhiệt tiếng Việt |
| H7 Thông báo ghép hai nguyên nhân | "Không kết nối được máy chủ. Khi mất mạng chỉ tài khoản nhân viên từng đăng nhập trên máy này mới mở được máy" |
| H8 os_info rỗng, Nạp tiền thiếu số dư | "Microsoft Windows 11 Pro 10.0.26100.9457 (aarch64)"; số dư hiện, khung chữ đọc được |
| H9 Cửa sổ phụ mở chậm | 0,8 giây — không tái hiện |
| Thiếu màn hình đổi thẻ nạp | Thêm **Nạp tiền → Nạp bằng thẻ**: tiền vào ví, đồng hồ dài ra ngay; đổi lần hai bị từ chối |

### Lỗi mới tìm ra trong vòng 2, đã sửa và kiểm lại

- **Dịch vụ khởi động lại thì cứ 5 giây bật thêm một giao diện.** Bản thừa thoát ngay nhưng kịp kéo cửa sổ đang chạy lên, phá chế độ bảo trì. Nay dịch vụ nhận ra giao diện đang chạy qua khoá chạy-một-bản của Wails: 0 lần bật thừa.
- **Tự cập nhật không tắt được giao diện bản cũ**, vì Windows báo tên tiến trình theo tệp đã đổi thành `.exe.old`. Kéo theo: tệp `.old` bị giữ, lần cập nhật sau báo "Access is denied". Đã sửa cả hai.
- **Mỗi bản tải về nằm lại 13 MB** trong thư mục tạm hệ thống. Nay được dọn.
- **Màn hình khoá gọi máy chủ không có token** khi quầy mở máy trước cho khách. Nay bỏ qua.
- **Dòng lý do trên màn hình khoá ở mãi**, khách sau đọc lý do của khách trước. Nay tự ẩn sau hai phút.
- **Hai thông báo chồng nhau** khi đổi thẻ. Còn một.

### Kiểm tự động

| | Kết quả |
|---|---|
| Backend `go test ./...` | Xanh, thêm 1 test hoàn tiền (đã chứng minh fail khi gỡ phần sửa) |
| Máy trạm `go test ./...` | Xanh, thêm test chặn ứng dụng, dọn tệp cập nhật, cửa sổ phụ không đè thanh dọc, tác vụ canh chừng có kích hoạt theo giờ |
| `golangci-lint` backend | 53, đúng bằng mốc cũ |

### Vẫn chưa kiểm được

Không đổi so với vòng 1: in hoá đơn, bộ cài Inno Setup, nổi trên game toàn màn
hình, máy hai màn hình, đăng nhập QR, nhiệt độ, bàn phím thật trên máy x64.
Không phải lỗi đã biết — là chỗ máy ảo không thử được.

---

# Vòng 1 — 30/09/2026

Chạy theo bảng kiểm `scripts/KIEM-CHUNG-WINDOWS.md` trên máy thật, điều khiển từ xa
qua SSH. Mọi dòng dưới đây là thứ **đã quan sát được**: ảnh chụp màn hình, trạng
thái trên máy chủ, hoặc số đo thời gian. Chỗ nào chỉ suy từ code thì ghi rõ.

## Môi trường

| | |
|---|---|
| Máy trạm | Windows 11 Pro 26100, ARM64, máy ảo VMware Fusion, 1703×1043, một màn hình |
| Bản máy trạm | `vnet-client-arm64.exe` (dev) cho toàn bộ bảng kiểm; `vnet-client-amd64.exe` v0.1.1 chạy giả lập cho luồng lõi |
| Máy chủ | build từ `feature/core` (91fb33f), chạy trên Mac, PostgreSQL 16 riêng cho đợt thử |
| Cách cài | Cài tay theo HUONG-DAN 2.4 (chưa có bộ cài Inno Setup) |
| Cách bấm | Chuột và phím được giả lập trong phiên màn hình của Windows; ảnh chụp lấy bằng hai đường độc lập (GDI trong phiên người dùng và chức năng chụp của chính VNET) |
| Giá thử | Nhóm máy 60.000₫/giờ = 1.000₫/phút, để một phiên hết tiền trong vài phút |

Đối chứng cho phép thử chặn phím: khi VNET không chạy, phím `Win` giả lập mở được
Start và `Alt+F4` đóng được Notepad. Vậy các kết quả "bị chặn" bên dưới là do hook
của VNET.

## Tóm tắt

Phần **hạ tầng** của máy trạm chạy tốt: dịch vụ nền, tự dựng lại giao diện, chặn
phím, điều khiển từ xa, chặn website, mở khoá khi mất mạng, lớp chống phá.

Phần **vòng đời phiên chơi** trên máy trạm có bảy lỗi nặng. Chúng cùng một gốc:
máy trạm coi "đã đăng nhập" và "đang có phiên tính tiền" là hai chuyện rời nhau,
và không có lối nào nối "phiên đã hết" về lại "màn hình khoá có ô đăng nhập".
Hệ quả là chơi miễn phí được, bị trừ tiền oan được, và máy kẹt được.

**Chưa nên cho khách dùng thật** trước khi sửa nhóm lỗi N1–N7.

| Mức | Số lỗi |
|---|---|
| Nặng (tiền, chơi miễn phí, kẹt máy) | 7 |
| Vừa (tính năng không có tác dụng, cấu hình lệch) | 5 |
| Nhẹ (giao diện, chữ, lệch nhỏ) | 9 |

## Lỗi nặng

### N1. Bấm "Đăng xuất" không kết thúc phiên — khách vẫn bị trừ tiền

- **Thấy gì:** đăng xuất lúc 20:34:05, màn hình về ô đăng nhập. Trên máy chủ phiên vẫn `is_active`, máy vẫn `in_use`, số dư 38.000 → 37.000 sau một phút và trừ tiếp.
- **Hệ quả:** khách về là mất sạch số dư còn lại. Máy `in_use` nên người khác không mở được. Máy trạm không có nút "Trả máy" nào.
- **Gốc:** `client/src/app.go:421` `Logout()` chỉ xoá token cục bộ, không gọi máy chủ. `session.store.ts` `logout()` cũng vậy.

### N2. Máy khởi động lại là chơi miễn phí

- **Thấy gì:** đang có phiên thì reboot. Máy chủ đóng phiên cũ đúng. Máy bật lên ở trạng thái đã đăng nhập "Khach thu mot / Chưa có phiên chơi", desktop mở, máy chủ 0 phiên, máy `available`. Ảnh `11-loi16-reboot-xong-van-dang-nhap.jpg`.
- **Hệ quả:** khách bấm Restart là dùng máy không tính tiền. Trái mục 7.7.
- **Gốc:** `session.store.ts` `restore()` đọc `vnet_session` trong localStorage rồi `SetLoggedIn(true)` mà không hỏi máy chủ còn phiên không.

### N3. Quầy "Trả máy" nhưng máy trạm không khoá

- **Thấy gì:** kết thúc phiên từ trang quản trị (`POST /sessions/:id/end`). 25 đến 47 giây sau máy trạm vẫn đếm giờ, desktop mở, Notepad mở được. Thử hai lần: phiên do quầy mở và phiên do khách tự đăng nhập. Ảnh `09-loi9-quay-tra-may-dong-ho-van-chay.jpg`.
- **Gốc:** `client/src/ws.go:70` đưa `session:ended` vào `vnet:session:updated` (gộp dữ liệu) thay vì xoá phiên và khoá máy. Chỉ `session:auto-ended` mới xoá phiên.

### N4. Hết tiền thì máy kẹt ở "Máy đang tạm khoá" cho tới khi có người bấm Mở khoá

- **Thấy gì:** hết số dư, phiên tự đóng đúng, lớp phủ "Hết số dư — vui lòng nạp thêm tại quầy" hiện đúng. Nhưng lớp phủ này không có ô đăng nhập và không tự gỡ. Nạp thêm 20.000₫ rồi chờ, vẫn khoá. Máy chủ ghi `available` trong khi không ai đăng nhập được. Ảnh `07-loi7-khoa-het-so-du-khong-tu-go.jpg`.
- **Hệ quả:** quán không nhân viên thì mỗi máy hết tiền là một máy chết, chủ quán phải mở khoá từng máy từ xa.
- **Gốc:** `backend/internal/scheduler/jobs.go:249` gửi đúng lệnh `remote:lock` của nút Khoá ở quầy. `MachineLockOverlay.vue` chỉ gỡ khi nhận `remote:unlock`.

### N5. Mở khoá sau khi hết tiền là chơi miễn phí

- **Thấy gì:** sau N4, bấm Mở khoá từ quầy. Máy trạm vẫn đăng nhập hội viên, hiện "Chưa có phiên chơi", desktop mở, không phiên nào trên máy chủ. Ảnh `08-loi8-mo-khoa-xong-choi-mien-phi.jpg`.
- **Gốc:** cùng gốc N2 và N3. Phiên bị xoá khỏi giao diện nhưng trạng thái đăng nhập giữ nguyên, và `UnlockScreen` trả cửa sổ về dạng thanh dọc.

### N6. Đang mở cửa sổ Hỗ trợ mà bị khoá thì khung chat phủ kín màn hình

- **Thấy gì:** hết tiền lúc cửa sổ Hỗ trợ đang mở. Cửa sổ phụ tự phóng toàn màn hình, luôn nổi, đè lên màn hình khoá. Khách thấy khung chat phủ kín, không có nút đóng, `Alt+F4` bị chặn. Phải giết tiến trình mới thoát. Ảnh `06-loi6-chat-phu-kin-khi-het-tien.jpg`.
- **Gốc:** `client/src/app.go:1085` `LockScreen()` thiếu chốt `a.windowMode != ""` mà `applyLoginLock` có. Tiến trình cửa sổ phụ cũng nối WebSocket và cũng nhận `remote:lock`. Khoá từ quầy lúc khách đang mở Đồ ăn hoặc Nạp tiền cũng dính.

### N7. Máy tắt hoặc reboot giữa phiên: ví bị trừ nhiều hơn tiền phiên

| Lần | Phiên ghi | Ví bị trừ |
|---|---|---|
| Reboot giữa phiên | 8 phút, 8.000₫ | 9.000₫ |
| Tắt hẳn máy trạm | 2 phút, 2.000₫ | 4.000₫ |

- **Thấy gì:** phiên được chốt lùi về nhịp tim cuối, đúng thiết kế. Nhưng các lượt trừ mỗi phút chạy trong 2 đến 3 phút chờ phát hiện thì không được hoàn.
- **Hệ quả:** trái mục 7.10. Báo cáo theo phiên và sổ ví lệch nhau sau mỗi lần mất điện.

## Lỗi vừa

### V1. Hai nguồn địa chỉ máy chủ, giao diện và dịch vụ nền nói chuyện với hai máy chủ khác nhau

- **Thấy gì:** máy ảo còn `vnet_server_url = http://192.168.2.7:8080` trong localStorage từ lần thử tháng 6. Cài mới với `config.json` đúng: dịch vụ nền gửi nhịp tim bình thường, máy hiện `available`, nhưng đăng nhập báo "không kết nối được máy chủ, và sai tài khoản hoặc mật khẩu". Ảnh `02-loi1-dang-nhap-bao-mat-may-chu.jpg`.
- **Gốc:** `frontend/src/App.vue:151` gọi `SetServerURL(localStorage)` đè lên `config.json`. Cài lại không chữa được vì dữ liệu WebView2 nằm ở `%APPDATA%\vnet-client.exe`.
- **Kèm theo:** trang Cài đặt của nhân viên hiện cứng `http://localhost:20800` chứ không phải địa chỉ đang dùng. Bấm "Lưu cấu hình" là tự làm hỏng. Ảnh `10-loi11-cai-dat-hien-localhost.jpg`.

### V2. "Chặn ứng dụng" không chặn được gì

- **Thấy gì:** lệnh tới máy, luật tường lửa `VNET_Block_chrome` được tạo (hai lần, vì dịch vụ và giao diện cùng xử lý).
- **Gốc:** `machine_actions.go:24` tạo luật `dir=out program=%ProgramFiles%\<tên>.exe`. Không ứng dụng nào nằm ở `C:\Program Files\chrome.exe`. Kể cả đúng đường dẫn thì đây là chặn mạng, không phải cấm chạy như hướng dẫn mô tả. Danh sách `blockedApps` trong `runWatchdog` không nơi nào ghi vào. Trên máy thử, tường lửa Windows còn đang tắt ở profile Private và Public.

### V3. Tác vụ canh chừng theo sự kiện không bao giờ chạy trên Windows 11

- **Thấy gì:** `sc stop VNETClient` không sinh event 7036 trong System log (chỉ có 7031, 7040, 7045). Tác vụ "(sự kiện)" có `LastRunTime` = chưa từng chạy suốt buổi thử.
- **Tác vụ mỗi phút** dùng `BootTrigger`, nên chưa chạy cho tới lần khởi động lại đầu tiên sau khi cài. Trước lần đó, dừng dịch vụ cùng lúc giết giao diện thì 90 giây sau vẫn không gì bật lại. Sau reboot với đồng hồ đúng: tự chạy lại sau 44 giây, đạt.
- **Lưu ý:** khi đồng hồ máy bị giật lùi, chuỗi lặp mỗi phút đứng hình.

### V4. "Cập nhật máy khách" chỉ tải tệp về, không thay được bản đang chạy

- **Thấy gì:** kiểm tra, tải và kiểm băm đều đúng. Tệp nằm ở `%TEMP%\vnet-update\` kèm dòng "đóng ứng dụng rồi chạy tệp này để cài".
- **Vấn đề:** tệp trong Program Files đang bị dịch vụ giữ, giao diện bị dịch vụ dựng lại sau 5 giây. Không có đường tự thay bản. Thực tế phải công bố bộ cài và chạy tay từng máy với quyền quản trị.

### V5. Máy trạm không ghi nhật ký ra đâu cả

- Bản build dùng `-H windowsgui`, `log.Printf` ra stderr, không có tệp log, không ghi Event Log. Các bước "xem nhật ký" trong bảng kiểm (mục 6.1, 8.3, 8.5) không làm được. Ra quán thật thì không có gì để chẩn đoán.

## Lỗi nhẹ

| # | Vấn đề |
|---|---|
| H1 | Nạp tiền giữa phiên: số dư đổi ngay nhưng đồng hồ đếm ngược chờ tới lượt trừ kế tiếp mới dài ra (đo hai lần, trễ 26 giây trở lên). Trái mục 1b.4 |
| H2 | Phiên do quầy mở thì máy trạm hiện "Đã chơi" đếm lên, không phải "Số dư còn chơi được" đếm ngược |
| H3 | Cửa sổ Đồ ăn, Hỗ trợ, Nạp tiền căn giữa 70% bề ngang nên đè lên thanh dọc khoảng 90px. Khi thanh dọc nổi trên thì nút đóng của cửa sổ phụ bị che. Ảnh `12-nap-tien-nut-dong-bi-che.jpg` |
| H4 | Sau một lần Khoá rồi Mở khoá từ xa, thanh dọc mất thuộc tính luôn nổi |
| H5 | Cửa sổ chat nền trắng lệch tông, còn chữ tiếng Anh: "No messages", "Type message", "Conversation started on" |
| H6 | Chữ tiếng Anh lọt ra: "Session fee for PC-01" ở sổ giao dịch, cảnh báo nhiệt độ ở `agent.go:154` |
| H7 | Thông báo đăng nhập ghép hai nguyên nhân thành một câu |
| H8 | `os_info` của máy luôn rỗng. Cửa sổ Nạp tiền dạng riêng không hiện số dư hiện tại. Nút "Tải bản cập nhật" tràn mép khung |
| H9 | Lần mở đầu của cửa sổ phụ mất hơn 4 giây mới hiện |

## Đạt

| Mục bảng kiểm | Kết quả đo |
|---|---|
| 0.2, 8.1 Dịch vụ | `VNETClient` Running, Automatic, LocalSystem, khôi phục 5s/5s/15s |
| 0.4 Máy chưa đăng nhập Windows | Máy vẫn `available`, nhịp tim đều. Lệnh Khởi động lại từ quầy tới nơi, máy reboot thật |
| 0.5, 7a.1 Thanh dọc | Tự bật sau 5 giây, dán mép phải, cao hết vùng làm việc, không đè taskbar |
| 0.6, 8.8 Giết giao diện | Dịch vụ dựng lại sau 1,6 giây |
| 0.7 Cửa sổ Đồ ăn | Cửa sổ Windows riêng, bấm lần hai không mở thêm |
| 0.9 Tắt Điểm danh | Ô biến mất, lưới bốn ô đều |
| 0.10 Gỡ dịch vụ | Dịch vụ và hai tác vụ biến mất |
| 1.1–1.4 Khoá từ xa | Lớp phủ kèm lý do, năm tổ hợp phím không thoát được, mở khoá trả lại thanh dọc |
| 1b.2, 1b.3, 1b.5, 1b.6 | Đếm ngược đúng, dưới 5 phút đổi màu, hết tiền tự đóng phiên, đúng một dòng phí |
| 3.1–3.5 Chặn website | Tệp hosts thêm đúng khối, 20 dòng cũ nguyên vẹn, trang chặn 403, ghi vi phạm, gỡ luật thì hosts về như cũ |
| 4.3–4.5 Cập nhật | Hiện bản mới, tải về khớp SHA-256, băm sai thì không lưu tệp |
| 5, 9 Giám sát | Chụp màn hình là ảnh thật, về sau 4 giây. Danh sách tiến trình kèm RAM. Tắt Notepad được, tắt `vnet-client.exe` bị từ chối |
| 6 Tự khai cấu hình | CPU, GPU, RAM, ổ cứng, IP, MAC tự điền. Ổ C 49,9% so với thật 50%. Giờ bật máy lệch 1 giây |
| 7.1, 7.2 Màn hình khoá | Phủ kín cả taskbar, `Win`, `Alt+Tab`, `Ctrl+Esc`, `Alt+Esc`, `Alt+F4` không thoát được |
| 7.4, 7.4b, 7.5 | Hội viên mở phiên. Nhân viên vào "Không tính giờ", không mở phiên. Đăng xuất về màn hình khoá |
| 7.8, 7.9 Reboot giữa phiên | Phiên cũ tự đóng tại nhịp tim cuối, máy được giải phóng |
| 7b Mở khoá mất mạng | Hội viên bị từ chối, PIN sai báo lỗi, PIN đúng mở máy không phiên, đang bảo trì thì lớp canh không tắt máy (80 giây), PIN lưu dạng băm |
| 7c Nhân viên ngoại tuyến | Tài khoản đã lưu mở được, sai mật khẩu bị từ chối, tệp lưu chỉ có tên và băm |
| 8.2 Giết dịch vụ | Chạy lại sau 2,7 giây |
| 8.3 Lớp chống phá | Dịch vụ Disabled và dừng: lệnh tắt máy được đặt ở giây 59 |
| Đặt món, chat, nạp tiền | Đơn về đúng máy đúng hội viên, chat hai chiều tức thì, duyệt nạp thì số dư đổi ngay |
| Đổi thẻ nạp qua API | Hội viên tự đổi được, cộng hai ví, đổi lần hai bị từ chối. Máy trạm chưa có màn hình này |
| Bản amd64 giả lập | Dịch vụ, nhịp tim, chặn phím, đăng nhập, chụp màn hình, khoá và mở khoá đều như bản ARM |

## Chưa kiểm được

- **Mục 2 in hoá đơn:** không có máy in nhiệt.
- **Mục 7d tài khoản mặc định của bản cài:** cần máy sạch chưa từng nối máy chủ.
- **Bộ cài Inno Setup:** chưa build, đợt này cài tay.
- **Thanh dọc nổi trên game toàn màn hình, máy hai màn hình, máy hai card đồ hoạ:** máy ảo không có.
- **Nhiệt độ CPU/GPU:** máy ảo ARM trả 0.
- **Đăng nhập QR:** cần camera.
- **Bàn phím thật:** mọi phím trong đợt này là phím giả lập. Hook cấp thấp nhận cả hai loại, nhưng nên có người gõ thật xác nhận trên máy x64.
- **Mục 8.5 và 8.6:** cần nhật ký, mà máy trạm không ghi (V5).

## Ghi chú môi trường, không phải lỗi VNET

- Windows 11 bật sẵn chế độ passwordless (`DevicePasswordLessBuildVersion=2`), làm `AutoAdminLogon` vô hiệu. Sau reboot máy nằm ở màn hình đăng nhập Windows và giao diện VNET không lên. Quán cần tắt chế độ này. Hướng dẫn nên thêm một dòng.
- Lệnh reboot đầu tiên kéo dài 7 phút vì Windows cài cập nhật. Nhịp tim vẫn chạy suốt quãng đó.
- Đồng hồ máy ảo lệch 14 giờ sau reboot do múi giờ khách khác máy chủ. Đổi múi giờ cho khớp thì hết.
- Các đường `machines/by-code/*` và kết nối WebSocket của dịch vụ nền nhận diện bằng mã máy, không có khoá. Ai trong mạng LAN cũng gọi được. Đợt này chưa thử giả mạo.

## Trạng thái máy thử sau khi xong

Đã gỡ dịch vụ, hai tác vụ, thư mục cài, thư mục tạm và tác vụ thử. Đã trả lại
`AutoAdminLogon`, passwordless, múi giờ Pacific. Tệp hosts và tường lửa sạch.

Hai thứ để nguyên: dịch vụ cũ `VNETAgent` (trỏ tới `Desktop\dist\dist\vnet-agent.exe`,
đang dừng, có từ trước) và thư mục `%APPDATA%\vnet-client.exe` đã đổi tên thành
`vnet-client.exe.bak-20260930` để địa chỉ máy chủ cũ không bám vào lần cài sau.

## Vòng 4 — 03/10/2026, kiểm lại toàn bộ máy trạm (v0.4.11 → v0.4.15)

Máy ảo Windows 11 ARM, máy trạm cài bằng bộ cài Inno Setup thật, tiền tối thiểu
mỗi lần đăng nhập 500₫, giá nhóm "Thường" 8.000₫/giờ.

| Chức năng | Kết quả |
|---|---|
| Màn hình khoá: tên máy, giờ, sai mật khẩu, phím Win/Alt+Tab/Ctrl+Esc | Đạt |
| Đăng nhập hội viên, trừ ngay 500₫, đồng hồ khớp số dư + phần trả trước | Đạt |
| Số dư dưới mức tối thiểu thì bị chặn, báo rõ số tiền | Đạt |
| Điểm danh: cộng 1.000₫ khuyến mãi, thanh bên cập nhật ngay | Đạt |
| Đánh giá: gửi được, cửa sổ tự ẩn | Đạt |
| Hỗ trợ: chat hai chiều, tiếng Việt đúng | Đạt |
| Đồ ăn: chọn món, giỏ hàng còn nguyên khi đổi màn hình, đặt đơn | Đạt |
| Quầy thu tiền món bằng số dư: thanh bên đổi ngay | **Lỗi → đã sửa** |
| Nạp bằng thẻ: cộng tiền và khuyến mãi, đồng hồ dài ra | Đạt |
| Yêu cầu nạp tới quầy, quầy bấm Thanh toán | **Lỗi → đã sửa** (xem dưới) |
| Gửi yêu cầu nạp lần hai | **Lỗi → đã sửa** |
| Quầy nhắn tin, chụp màn hình, xem tiến trình | **Lỗi → đã sửa** (chạy hai lần) |
| Quầy khoá/mở khoá có lý do | Đạt; cửa sổ phụ dựng lại sau mở khoá **đã sửa** |
| Hết tiền giữa phiên: dừng đúng lúc về 0, số dư không âm, lý do trên màn hình khoá | Đạt |
| Đăng nhập nhân viên: chế độ quản trị, không mở phiên | Đạt |
| Trang Cài đặt máy trạm, kiểm tra cập nhật | Đạt |
| Đăng xuất: về màn hình khoá, cửa sổ phụ tắt | Đạt |
| Tài khoản quản trị máy trạm: chế độ bảo trì | Đạt |
| Giết giao diện: dịch vụ bật lại sau 0,57 giây | Đạt |
| Chặn website: khối hosts, trang chặn 403, gỡ sạch | Đạt |
| Tự cập nhật lên 0.4.13: tải vào thư mục cài, kiểm băm, cài, khởi động lại | Đạt |
| Mất kết nối 60 giây: khoá; có lại: tự mở, phiên còn | Đạt; cửa sổ phụ dựng lại **đã sửa** |

Lỗi tìm được và đã sửa trong vòng này:

1. **Thanh toán đơn nạp tiền không cộng tiền.** Quầy bấm "Thanh toán" trên yêu
   cầu nạp: đơn sang hoàn tất nhưng tài khoản khách không được cộng. Nay đi qua
   đúng hàm cộng tiền; người thực hiện để trống thì ghi NULL thay vì chuỗi rỗng
   (PostgreSQL từ chối "" cho cột uuid).
2. **Trả tiền món bằng số dư không báo về máy trạm.** Số dư trên thanh bên đứng
   nguyên tới lượt trừ tiền giờ kế tiếp — không có lượt nào khi khách đang trong
   phần trả trước hoặc gói cước.
3. **Lệnh điều khiển từ quầy chạy hai lần.** Cửa sổ phụ dựng sẵn cũng xử lý lệnh:
   hai hộp thông báo, hai ảnh chụp, hai danh sách tiến trình.
4. **Màn hình Nạp tiền kẹt ở "Đã gửi yêu cầu".** Biểu mẫu một lần nay dựng mới
   mỗi lần mở; chỉ Đồ ăn và Hỗ trợ giữ trạng thái.
5. **Cửa sổ phụ dựng sẵn mất sau khi quầy khoá hoặc mất kết nối.** Nay dựng lại
   khi mở khoá; WebView2 đôi khi không dựng được khung lúc vừa có mạng lại nên
   thanh chính tự thử lại tối đa ba lần.
6. Ô nhập nền đen trơn, lạc tông giao diện mới.

Chưa kiểm: âm thanh báo giờ lần này (đã kiểm ở vòng trước), máy in nhiệt, máy
x64 thật, game toàn màn hình, nhiều màn hình, quét QR bằng camera.
