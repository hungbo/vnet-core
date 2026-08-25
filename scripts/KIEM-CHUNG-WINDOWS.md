# Kiểm chứng phần Windows — làm tay trên máy thật

Năm tính năng dưới đây gọi thẳng API của Windows. Không máy Mac/Linux nào chạy
được, và `scripts/verify.py` cũng không với tới — nên đây là phần **duy nhất**
của hệ thống chưa có phép kiểm tự động.

Chuẩn bị:

```
bash scripts/build-client.sh          # sinh dist/vnet-client-amd64.exe (và arm64)
bash scripts/build-client.sh dev arm64   # chỉ arm64
```

**Đầu tiên hãy mở thử tệp .exe và xem giao diện có lên không.** Nếu hiện hộp
thoại `Wails applications will not build without the correct build tags` thì
binary build thiếu `-tags desktop,production` — script hiện đã tự chặn trường
hợp này, nhưng tệp cũ build trước đó vẫn hỏng, phải build lại.

Chép tệp `.exe` sang một máy Windows **dùng thử được** (đừng chạy trên máy đang
phục vụ khách — bài 1 khoá bàn phím, bài 3 sửa tệp hosts, bài 4 cài đè ứng dụng).
Mở Cài đặt trong máy khách, khai đúng **Mã máy** và **Địa chỉ server**.

Chạy bộ kiểm đơn vị dành riêng cho Windows trước:

```
cd client\src
go test ./...
```

`locker_windows_test.go` chỉ chạy trên Windows; nó phủ bảng phím bị chặn. Phần
còn lại dưới đây phải thử tay.

---

## 0. Bộ cài, dịch vụ nền và cửa sổ

Phần này **không kiểm được từ máy Mac/Linux** — chỉ biên dịch được. Đây là danh
sách bắt buộc trước khi giao cho quán.

1. Chạy `vnet-client-setup-<phiên bản>.exe` với quyền quản trị. Khai địa chỉ máy
   chủ, mã máy và **khoá máy** (lấy ở trang Máy → Cấp lại khoá; khoá chỉ hiện
   đúng một lần).
2. Mở `services.msc` → phải thấy **VNETClient**, trạng thái *Running*, khởi động
   *Automatic*.
3. Mở `C:\Program Files\VNET Client\config.json` → phải có đủ ba trường.
4. **Khởi động lại máy và KHÔNG đăng nhập Windows.** Ở trang quản trị:
   - máy phải hiện **online**,
   - bấm **Tắt máy** phải có tác dụng.
   Đây là lỗ hổng chính đang sửa: bản cũ chỉ nối WebSocket sau khi khách đăng
   nhập, nên máy trống không nhận được lệnh nào.
5. Đăng nhập Windows → giao diện tự bật, **dán mép phải**, cao hết màn hình, và
   nằm trên cả game toàn màn hình.
6. Mở Task Manager, kết thúc `vnet-client.exe` (bản giao diện) → dịch vụ phải bật
   lại nó trong vòng khoảng 5 giây.
7. Bấm **Đồ ăn** → mở một **cửa sổ Windows riêng**, kéo thả và phóng to được.
   Bấm lần hai → **không** mở cửa sổ thứ hai, chỉ đưa cửa sổ cũ lên trước.
8. Vào game toàn màn hình, nhân viên nhắn tin từ trang quản trị → **cửa sổ hỗ trợ
   phải hiện lên trên game**.
9. Ở trang quản trị: Cài đặt → tab **Tính năng** → tắt Điểm danh. Tải lại giao
   diện máy trạm → nút "Điểm danh" biến mất, và lưới ô còn lại vẫn **đều nhau**.
10. Gỡ cài đặt → dịch vụ biến mất khỏi `services.msc`.

## 1. Khoá màn hình + chặn phím

1. Máy khách đăng nhập, đang ở màn hình chính.
2. Trong admin: **Máy → Khoá**, nhập lý do "hết giờ chơi".
3. Trên máy trạm, lớp phủ phải hiện lý do đó.

Bấm lần lượt và xác nhận **không** thoát ra được:

| Phím | Kỳ vọng |
|---|---|
| `Win` | không mở Start |
| `Alt+Tab` | không chuyển cửa sổ |
| `Alt+F4` | không đóng ứng dụng |
| `Ctrl+Esc` | không mở Start |
| `Alt+Esc` | không chuyển cửa sổ |

Đồng thời phải gõ được chữ thường (thử gõ vào ô lý do nếu có) — chặn quá tay
cũng là lỗi.

**Giới hạn đã biết, không phải lỗi:** `Ctrl+Alt+Del` KHÔNG chặn được. Windows xử
lý tổ hợp này ở tầng dưới hook. Lớp khoá là rào cản, không phải nhà tù.

4. Trong admin: **Máy → Mở khoá** → lớp phủ biến mất, phím hoạt động lại.

## 1b. Đồng hồ đếm ngược và hết tiền giữa phiên

1. Nạp cho tài khoản thử một số tiền nhỏ (ví dụ đủ chơi 3 phút ở giá đang cài).
2. Đăng nhập trên máy trạm. Đồng hồ phải hiện **"Số dư còn chơi được"** và **đếm
   ngược**, không phải "Đã chơi" đếm lên.
3. Còn dưới 5 phút thì số phải **đổi màu cảnh báo**.
4. Nạp thêm tiền ở quầy → đồng hồ trên máy trạm phải **dài ra ngay**, không phải
   chờ tới phút sau.
5. Để chạy cho hết tiền. Đúng lúc số dư cạn:
   - phiên phải tự đóng,
   - lớp phủ khoá màn hình phải hiện kèm lý do "Hết số dư — vui lòng nạp thêm tại quầy",
   - máy phải về trạng thái sẵn sàng ở trang Máy.
6. Kiểm trang Giao dịch của khách: **đúng một dòng** "Tiền giờ" cho cả phiên, và
   số tiền bằng đúng phần số dư đã giảm.

## 2. In hoá đơn ra máy in nhiệt

1. Admin → **Máy in** → khai máy in thật (IP + cổng 9100).
2. Cài đặt → tab **Hoá đơn**: đặt tiêu đề, chân trang, mã số thuế.
3. Đơn hàng → **In hoá đơn**.

Trên tờ giấy phải thấy: tên quán, mã số thuế, tiêu đề vừa đặt, danh sách món,
tổng tiền, chân trang vừa đặt — và **giấy tự cắt**.

Quan trọng nhất: **dấu tiếng Việt phải đúng** (`Cà phê sữa`, không phải
`Ca phe sua` hay ô vuông). Nếu sai, kiểm lại `code_page` và `encoding` của máy in
trong trang Máy in.

## 3. Chặn website (ghi tệp hosts + trang chặn cổng 80)

1. Admin → **Chặn website** → thêm `facebook.com`, gán cho nhóm máy đang thử.
2. Trên máy trạm, mở `C:\Windows\System32\drivers\etc\hosts` bằng Notepad
   (chạy với quyền quản trị).
3. Phải thấy một khối `# VNET BEGIN … # VNET END` chứa `127.0.0.1 facebook.com`.
   **Các dòng khác trong tệp phải còn nguyên** — đây là chỗ dễ hỏng nhất.
4. Mở trình duyệt vào `facebook.com` → phải ra trang chặn của VNET, không phải
   lỗi "không kết nối được".
5. Gỡ khỏi danh sách chặn → khối `# VNET …` biến mất, tệp hosts trở lại như cũ.

## 4. Cài bản cập nhật

1. `bash scripts/build-server.sh` rồi `bash scripts/build-client.sh` với số hiệu
   phiên bản cao hơn bản đang chạy.
2. Admin → **Cập nhật máy khách** → công bố bản mới, dán **đúng chuỗi SHA-256**
   mà `build-client.sh` in ra.
3. Máy khách → Cài đặt → **Kiểm tra cập nhật** → hiện bản mới.
4. **Tải bản cập nhật** → báo đã tải xong và kiểm băm.
5. Thử bản băm SAI: sửa một ký tự trong ô Băm rồi tải lại → phải **từ chối**,
   không được lưu tệp.
6. Đóng ứng dụng, chạy tệp vừa tải, mở lại → ô "Phiên bản" trong Cài đặt hiện số
   mới.

## 5. Chụp màn hình (GDI)

1. Máy trạm đang mở một cửa sổ dễ nhận ra.
2. Admin → **Máy → Chụp màn hình**.
3. Ảnh hiện ra phải là **màn hình thật của máy đó**, không phải ảnh đen hoặc ảnh
   của máy khác.
4. Thử với máy có hai màn hình: ảnh phải phủ hết vùng làm việc.

---

## 6. Máy tự khai cấu hình

Máy trạm đọc cấu hình MỘT lần lúc khởi động rồi gửi kèm mỗi lần báo cáo. Ba
trong bốn giá trị lấy qua gopsutil nên chạy được ở mọi nơi; riêng **tên GPU phải
hỏi WMI qua PowerShell**, và đó là thứ duy nhất trong cả gói không kiểm được từ
máy không phải Windows.

1. Chạy máy trạm, xem nhật ký dòng `cấu hình máy: CPU=... GPU=... RAM=...GB Ổ đĩa=...GB`.
   Bốn giá trị phải khớp với **Task Manager → Performance**.
2. Admin → **Máy → Sửa**: bốn ô CPU / GPU / RAM / Ổ cứng phải tự điền đúng, kể
   cả khi trước đó bỏ trống hoặc gõ sai. Đây là chỗ trước đây phải nhập tay.
3. Máy **hai card đồ hoạ** (đồ hoạ tích hợp + card rời): ô GPU phải ra **card
   rời**, không phải card tích hợp.
4. Máy **không có card rời**: ô GPU vẫn phải có tên card tích hợp, không được rỗng.
5. Không được thấy cửa sổ đen PowerShell nhấp nháy trên màn hình khách lúc máy
   trạm khởi động.
6. Rút mạng vài phút rồi cắm lại: cột **IP** và **MAC** trên trang Máy phải
   giữ nguyên chứ không nhấp nháy trống — mỗi báo cáo cách nhau 15 giây.
7. Admin → **Máy → Phần cứng**: cột **Đã bật** phải khớp với `systeminfo | find
   "System Boot Time"`.
8. Ổ đĩa: cột **Ổ cứng** phải là phần trăm ổ `C:` thật, không phải 0%. Đường dẫn
   `"/"` cứng trong bản cũ không trỏ tới đâu trên Windows.

---

## 7. Màn hình khoá và quyền dùng máy

Toàn bộ mục này KHÔNG dính gì tới đăng nhập/khoá máy của Windows. Quán thường
để Windows tự đăng nhập vào một tài khoản dùng chung, nên khoá của Windows không
bảo vệ được gì.

1. Bật máy, chưa ai đăng nhập VNET: màn hình phải bị **phủ kín** bởi ô đăng nhập
   VNET. Không thấy desktop, không thấy taskbar, không mở được ứng dụng nào.
2. Thử `Alt+Tab`, `Win`, `Ctrl+Esc`, `Alt+F4`: không thoát ra được.
   (`Ctrl+Alt+Del` thì Windows luôn ưu tiên — không hook được, và không nên hook.)
3. Trang quản trị phải thấy máy ở trạng thái **Sẵn sàng**, không phải Ngoại tuyến.
4. Đăng nhập bằng tài khoản hội viên: màn hình mở ra, cửa sổ thu về **thanh dọc
   mép phải**, dùng máy bình thường. Phím thoát (`Alt+Tab`, `Win`) vẫn phải bị
   chặn — khách chơi game được nhưng không ra desktop được.
4b. Đăng nhập bằng tài khoản **nhân viên** vào CÙNG ô đó: phải ra màn hình quản
   trị ("Không tính giờ"), và trang quản trị **không được** thấy phiên nào mới
   mở cho máy này.
5. Bấm **Đăng xuất**: quay lại đúng màn hình khoá phủ kín ở bước 1.
6. Khách hết tiền giữa phiên: máy tự về màn hình khoá, không kẹt ở desktop.
7. Khởi động lại máy trong lúc đang đăng nhập: bật lên phải là màn hình khoá,
   không phải tự vào lại phiên cũ.
8. **Reboot giữa phiên (đóng băng)**: đang chơi thì khởi động lại máy. Bật lên
   phải là màn hình khoá. Xem trang quản trị → phiên cũ phải **đã đóng** (chốt
   sổ tới lúc reboot), máy không còn "đang dùng". Đăng nhập lại (kể cả cùng tài
   khoản) phải mở **phiên MỚI**, không nối lại phiên cũ.
9. Reboot rồi **bỏ đó không đăng nhập ai**: sau khoảng một phút, phiên cũ trên
   trang quản trị phải tự đóng — máy không được nằm ở màn hình khoá mà đồng hồ
   vẫn chạy.
10. **Tắt máy giữa phiên mà không đăng xuất** (và KHÔNG bật lại): sau khoảng
    2–3 phút, phiên trên trang quản trị phải tự đóng, và tiền chỉ tính tới lúc
    máy còn báo tín hiệu — không tính phần máy đã tắt. Máy được giải phóng cho
    khách kế tiếp.

### 7a. Kích thước và vị trí cửa sổ

Phần tính toán đã có bài kiểm ở `window_test.go` cho năm độ phân giải, nhưng
việc Wails đặt cửa sổ đúng chỗ thì chỉ Windows mới trả lời được.

1. Thanh điều khiển phải **cao khoảng 70% màn hình** và **căn giữa theo chiều
   dọc** ở mép phải — không che khay hệ thống, không chạm mép trên.
2. Bấm **Đồ ăn**: cửa sổ mới phải nằm **giữa màn hình**, rộng ~70% và cao ~80%.
   Không được nằm đè lên thanh điều khiển ở mép phải.
3. Bấm **Hỗ trợ**: giống hệt bước 2.
4. Máy hai màn hình: cửa sổ phải ra giữa **màn hình đang chứa thanh điều khiển**,
   không phải màn hình thứ nhất.
5. Trong cửa sổ Đồ ăn, **giỏ hàng hiện sẵn ở cột bên phải** — không phải bấm
   biểu tượng giỏ mới thấy.
6. Thanh điều khiển phải **nằm trên** mọi cửa sổ khác, kể cả game toàn màn hình.
7. Bấm nút **–** ở góc phải thanh: thanh thu về một **dải hẹp bám mép phải**.
   Bấm vào dải đó thì mở lại. Dải phải vẫn nằm trên các cửa sổ khác.
8. Đăng xuất trong lúc đang thu gọn: phải quay về màn hình khoá **phủ kín**,
   không phải một dải hẹp.
9. Chưa đăng nhập: **không** có nút ẩn nào — lớp khoá không được thu gọn.

### 7b. Mở khoá khi mất mạng

Đây là đường vào DUY NHẤT không cần máy chủ. Không thử được thì đừng giao máy
cho khách.

1. **Rút dây mạng** khỏi máy trạm. Đợi một phút.
2. Thử đăng nhập bằng tài khoản hội viên: phải báo lỗi kết nối. Thử "Đăng nhập
   quản trị": cũng phải báo lỗi. Máy vẫn khoá.
3. Bấm **Mở khoá kỹ thuật**, gõ PIN **sai**: báo "PIN không đúng", máy vẫn khoá,
   và mỗi lần sai phải chậm khoảng một giây.
4. Gõ PIN **đúng**: màn hình mở ra, cửa sổ thu nhỏ, dùng được desktop. Không có
   phiên nào được mở và không có tiền nào bị trừ.
5. Trong lúc đang bảo trì, dừng dịch vụ nền: máy **không được tự tắt** (lớp
   chống phá đứng yên khi đang bảo trì).
6. Đăng nhập một hội viên: chế độ bảo trì phải tắt, máy quay về hoạt động bình
   thường.
7. Máy **chưa đặt PIN**: nút "Mở khoá kỹ thuật" phải **không hiện**. Đặt bằng
   `vnet-client.exe --set-pin 246810` (cần quyền quản trị vì tệp nằm trong
   Program Files) rồi khởi động lại giao diện — nút phải hiện ra.
8. Mở `config.json` bằng Notepad: **không được thấy PIN dạng chữ thường**, chỉ
   thấy chuỗi bắt đầu bằng `pbkdf2-sha256$`.

### 7c. Vào bằng tài khoản nhân viên khi mất mạng

Đường này tự có, không phải cấu hình: mỗi lần một tài khoản nhân viên đăng nhập
thành công lúc còn mạng, máy trạm ghi lại tên và băm mật khẩu.

1. Khi **còn mạng**, bấm "Đăng nhập quản trị" và đăng nhập bằng một tài khoản
   nhân viên. Phải vào được bình thường.
2. Đăng xuất, **rút dây mạng**, rồi bấm "Đăng nhập quản trị" với **đúng tài
   khoản đó**: máy phải mở ra kèm thông báo "Không có mạng — đã mở khoá máy
   bằng tài khoản lưu sẵn".
3. Kiểm trang quản trị sau khi có mạng lại: **không được có phiên nào** mở ra
   trong lúc đó, và không có tiền nào bị trừ.
4. Vẫn mất mạng, thử **sai mật khẩu**: bị từ chối, máy vẫn khoá, mỗi lần sai
   chậm khoảng một giây.
5. Thử một tài khoản **hội viên** (chưa từng đăng nhập kiểu quản trị): phải bị
   từ chối. Hội viên không bao giờ được lưu — lưu được nghĩa là khách chỉ cần
   rút dây mạng là tự mở máy.
6. Đổi mật khẩu tài khoản đó trên máy chủ, đăng nhập **một lần** khi còn mạng,
   rồi rút mạng: **mật khẩu mới** dùng được, **mật khẩu cũ** không.
7. Mở `%AppData%\..\Roaming\VNET\offline-staff.json` (Windows) bằng Notepad:
   chỉ được thấy tên tài khoản và chuỗi băm, **không có mật khẩu**.

### 7d. Tài khoản mặc định của bản cài

Máy **vừa cài xong, chưa từng nối được máy chủ lần nào** vẫn phải vào được.

1. Cài lên một máy sạch, **không cắm mạng**. Đăng nhập `admin` / `admin`:
   máy phải mở ra kèm thông báo mở khoá bằng tài khoản mặc định.
2. Cắm mạng, đăng nhập bằng một tài khoản **nhân viên thật**: phải hiện cảnh
   báo màu vàng "Máy này còn mở được bằng tài khoản mặc định", và cảnh báo đó
   **không tự tắt** — phải bấm mới mất.
3. Đăng xuất, rút mạng, thử lại `admin` / `admin`: giờ phải **bị từ chối**. Cửa
   mặc định đã đóng sau lần đăng nhập thật đầu tiên.
4. Thử lại tài khoản nhân viên vừa dùng ở bước 2: phải mở được.
5. Đăng nhập bằng tài khoản **hội viên**: **không** được hiện cảnh báo này —
   khách không làm gì được với thông tin đó.

## 8. Lớp chống phá giờ chơi

**Không thử mục này trên máy đang có khách.** Nó kết thúc bằng việc tắt máy.

1. `services.msc` → dịch vụ **VNET Client Agent** phải ở chế độ Automatic, và
   trong tab *Recovery* cả ba lần hỏng đều là *Restart the Service*.
2. Tắt dịch vụ bằng `sc stop VNETClientAgent`: Windows phải **tự bật lại** sau
   khoảng 5 giây. Đây là lớp bảo vệ chính, không phải lớp canh trong giao diện.
3. Đặt dịch vụ sang **Disabled** rồi tắt nó. Giao diện sẽ thử bật lại mỗi 5 giây
   và thất bại; sau **một phút** máy phải **tự tắt**. Xem nhật ký giao diện:
   phải có các dòng `[guard] ... (n/12 lượt)` trước khi tắt.
4. Tạo tệp `maintenance.flag` trong thư mục cài đặt rồi lặp lại bước 3: máy
   **không được tắt**. Xoá tệp đi thì mới tắt lại như bước 3.

4b. **Tác vụ canh chừng** (lớp bịt lỗ `sc stop` mà SCM bỏ sót):
   - `taskschd.msc` → thư mục **VNET**: phải có hai tác vụ, cả hai chạy dưới
     tài khoản **SYSTEM**.
   - Chạy `sc stop VNETClient` (dừng SẠCH, khác với kill ở bước 2). SCM sẽ
     **không** bật lại. Tác vụ theo sự kiện phải bật lại trong vài giây; nếu nó
     không đăng ký được thì tác vụ theo nhịp bật lại trong vòng một phút.
   - Tắt tác vụ theo sự kiện trong Task Scheduler rồi lặp lại: phải vẫn bật lại
     trong vòng một phút.
   - Xem Task Scheduler → History của tác vụ theo sự kiện: nó chỉ được chạy khi
     **dịch vụ VNET** dừng, không phải mỗi lần bất kỳ dịch vụ nào đổi trạng thái.
   - Gỡ cài đặt: cả hai tác vụ phải biến mất khỏi `taskschd.msc`.
5. Tắt máy bình thường bằng nút Start → Shut down: **không** được có dòng
   `[guard]` nào trong nhật ký. Dịch vụ dừng trước giao diện lúc tắt máy là
   chuyện thường, đọc nhầm thành phá hoại là sai.
6. Bật máy lên và bấm giờ: trong **90 giây** đầu lớp canh phải im lặng kể cả khi
   dịch vụ chưa kịp chạy.
7. Chạy `vnet-client.exe` từ một thư mục bất kỳ khi **chưa cài dịch vụ**: không
   bao giờ được tắt máy.
8. Giết tiến trình giao diện trong Task Manager: dịch vụ phải bật lại nó trong
   khoảng 5 giây, và máy quay về màn hình khoá.

---

## 9. Giám sát máy trạm từ trang quản trị

Cần một máy trạm THẬT đang đăng nhập và nối WebSocket, và một trình duyệt mở
trang **Máy** trên trang quản trị. Ba chức năng này chỉ chạy ở tiến trình giao
diện (session của người dùng), không phải dịch vụ nền.

1. **Chụp màn hình**: ở trang Máy, mở dropdown "Khác" của máy đó → **Chụp màn
   hình**. Trong vài giây một hộp thoại hiện ảnh màn hình hiện tại của máy khách.
   - Máy đang tắt/không nối: bấm phải hiện cảnh báo "chưa kết nối", không phải lỗi đỏ.
   - Ảnh phải là màn hình desktop của KHÁCH, không phải màn hình khoá VNET.
2. **Xem ứng dụng**: dropdown "Khác" → **Xem ứng dụng**. Bảng hiện danh sách ứng
   dụng đang chạy, gộp theo tên, sắp theo RAM giảm dần. Chrome nhiều tiến trình
   con phải gộp thành MỘT dòng với tổng RAM, không phải 30 dòng.
3. **Tắt ứng dụng**: trong bảng trên, bấm **Tắt** ở một ứng dụng vô hại (ví dụ
   notepad đang mở). Xác nhận → ứng dụng tắt, bảng tự nạp lại và dòng đó biến mất.
   - Thử tắt một tiến trình trong danh sách cấm (không hiện nút, hoặc nếu gọi
     thẳng API thì `killed = -1`): `vnet-client`, `csrss`, `winlogon`, `lsass`,
     `services`, `wininit`, `smss`, `svchost`. Máy KHÔNG được sập, lớp khoá VNET
     KHÔNG được tắt.
4. Nhật ký kiểm toán: mỗi lần chụp/xem/tắt phải có một dòng `remote_screenshot`
   / `remote_process-list` / `remote_process-kill` trong bảng `audit_logs`, kèm
   ai bấm và máy nào.

---

## Ghi lại kết quả

Mỗi mục ghi **đạt / sai** kèm một câu mô tả thứ nhìn thấy. Mục nào sai thì chụp
màn hình hoặc chép nguyên văn thông báo lỗi — mô tả "không chạy" không đủ để
tìm ra nguyên nhân.
