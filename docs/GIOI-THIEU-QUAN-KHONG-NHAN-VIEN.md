# VNET — Phần mềm cho tiệm net không cần nhân viên đứng quầy

Bạn mở tiệm net nhưng không muốn (hoặc không đủ tiền) thuê người ngồi quầy cả ngày. Khách tự vào, tự chơi, tự trả tiền; bạn chỉ cần biết quán đang chạy thế nào và can thiệp khi có chuyện — từ điện thoại, ở bất cứ đâu.

VNET được xây theo đúng giả định đó: **mọi việc lặp đi lặp lại đều do máy làm, con người chỉ xuất hiện khi thực sự cần.**

---

## Một ngày ở quán không có ai đứng quầy

1. **Khách ngồi vào máy.** Màn hình khoá hiện sẵn. Khách gõ tài khoản + mật khẩu, hoặc quét mã QR trên điện thoại. Phiên chơi mở, đồng hồ bắt đầu chạy. Không ai phải bấm gì ở quầy.
2. **Tiền trừ theo từng phút.** Đồng hồ trên máy đếm ngược tới đúng lúc hết tiền, khách nhìn thấy trước chứ không bị cắt bất ngờ.
3. **Hết tiền, máy tự khoá.** Phiên kết thúc, màn hình khoá phủ lên với dòng "Hết số dư — vui lòng nạp thêm", máy chuyển về trạng thái sẵn sàng cho người tiếp theo.
4. **Khách muốn chơi tiếp** thì nạp thẻ cào của quán, hoặc mua gói giờ đã có sẵn tài khoản. Chi tiết ở mục *Nạp tiền không cần quầy* bên dưới.
5. **Khách đứng dậy đi về** mà quên đăng xuất? Đồng hồ vẫn trừ tới khi hết tiền hoặc bạn khoá máy từ xa. Bấm **Đăng xuất** là trả máy và dừng trừ tiền. Máy mất điện hay khởi động lại giữa chừng thì phiên được chốt đúng lúc máy tắt, phần đã lỡ trừ thêm được hoàn lại; bật lên là màn hình khoá, khách đăng nhập lại là chơi tiếp.
6. **Nửa đêm**, khách vị thành niên chạm giờ giới nghiêm — hệ thống tự trả máy, không đợi ai nhắc.

Bạn ở nhà, mở điện thoại, thấy sơ đồ máy nào đang có người, doanh thu hôm nay bao nhiêu, tin nhắn nào khách vừa gửi.

---

## Hệ thống tự làm thay nhân viên

| Việc | Nhân viên trước đây | VNET |
|---|---|---|
| Mở máy cho khách | Bấm mở ở quầy | Khách tự đăng nhập, phiên tự mở |
| Tính tiền | Nhìn đồng hồ, ghi sổ | Trừ theo phút, tự động, mỗi phút một lượt |
| Hết tiền | Ra tận máy nhắc | Tự đóng phiên, tự khoá màn hình |
| Máy hỏng, treo | Đi kiểm tra | Tự báo *Ngoại tuyến* sau 45 giây mất tín hiệu |
| Đặt chỗ mà không đến | Gọi điện hỏi | Tự đánh vắng sau 15 phút, tự trả máy |
| Trẻ em chơi khuya | Đuổi về | Giới nghiêm theo ngày trong tuần + trần giờ/ngày, tự trả máy |
| Xếp hạng thành viên | Tính tay cuối tháng | Tự xếp theo tổng chi tiêu, 15 phút một lần |
| Chống nạp trùng | "Ủa em nạp rồi mà" | Khoá đơn khi xử lý, hai thiết bị cùng duyệt vẫn chỉ cộng một lần |

---

## Chủ quán làm gì — từ điện thoại

Trang quản trị chạy trên trình duyệt, không cài gì, mở được trên điện thoại. Mọi thay đổi (số dư, đơn mới, tin nhắn, máy on/off) đẩy thẳng lên màn hình, không cần tải lại.

**Điều khiển từng máy như đang đứng cạnh nó:**

- Gửi một dòng tin nhắn hiện lên màn hình khách
- Chụp màn hình để xem khách đang gặp gì
- Xem danh sách ứng dụng đang chạy, tắt cái đang treo
- Khoá / mở khoá máy kèm lý do khách đọc được
- Chặn một ứng dụng cụ thể trên một máy
- Khởi động lại, tắt máy (và kết thúc phiên, chỉ khi lệnh đã chắc chắn tới nơi)

**Nói chuyện với khách:** ô *Hỗ trợ* trên máy trạm là kênh chat thẳng tới bạn. Khách hỏi, bạn trả lời từ điện thoại.

**Nhìn tổng thể:** báo cáo doanh thu ngày/tháng, theo máy, theo hội viên; đánh giá sao của khách; nhật ký mọi thao tác ai làm gì lúc nào.

---

## Nạp tiền không cần quầy

Đây là câu hỏi lớn nhất của quán không nhân viên, nên nói thẳng cái gì làm được hôm nay, cái gì chưa.

**Làm được ngay:**

- **Gói giờ kèm tài khoản.** Bán một gói (ví dụ 3 giờ tối, hoặc 300 phút trả trước), hệ thống tự tạo tài khoản kiểu `Goi3h1` + mật khẩu 6 số. Khách vãng lai không cần đăng ký gì, cầm tài khoản ngồi vào máy là chơi. Gói chỉ bắt đầu trừ giờ khi kích hoạt trên máy, mua sáng tối chơi không mất phút nào.
- **Nạp từ xa cho khách quen.** Khách chuyển khoản cho bạn, bạn bấm *Nạp* trên điện thoại, số dư trên máy khách đổi tức thì.

**Chưa có, nói trước để khỏi kỳ vọng sai:**

- Nút *Nạp tiền* trên máy trạm hiện chỉ **gửi yêu cầu**; vẫn cần một người bấm duyệt sau khi thấy tiền về. Chưa nối cổng thanh toán (VietQR, Momo) để tự xác nhận. Đây là hạng mục đang trong kế hoạch qua VNET Cloud Hub và app di động cho game thủ: khách chọn quán, chọn mệnh giá, quét QR, tiền tự cộng vào ví tại quán.
- Đồ ăn: khách tự đặt món trên máy, phiếu chạy ra máy in bếp, nhưng khâu xác nhận và thanh toán vẫn cần người. Quán không nhân viên nên tắt mục này hoặc thay bằng tủ mát tự phục vụ.

---

## Khi không ai trông quán, phần mềm phải khó bị qua mặt

- **Máy trạm là một dịch vụ Windows chạy nền 24/7**, kể cả khi chưa ai đăng nhập. Khách tắt giao diện thì dịch vụ tự dựng lại. Bạn vẫn khoá, tắt được máy trống.
- **Rút dây mạng không mở được máy.** Đăng nhập ngoại tuyến chỉ chấp nhận tài khoản nhân viên, không bao giờ chấp nhận hội viên.
- **Mã máy không sửa được từ giao diện**, chỉ nằm trong tệp cấu hình do bộ cài ghi. Khách không giả danh máy khác được.
- **Chặn website** theo danh sách tên miền, theo lịch, theo nhóm máy; có nhật ký vi phạm. (Chỉ chặn được HTTP; HTTPS thì trang lỗi kết nối chứ không hiện trang giải thích.)
- **Nợ có trần.** Trừ tiền không bao giờ bị từ chối để khách không bị cắt ngang, nhưng số dư âm quá mức bạn đặt thì không mở được phiên mới.
- **Nhật ký hoạt động** ghi mọi thao tác quản trị: ai nạp, ai hoàn, ai xoá, lúc nào, từ địa chỉ nào.
- **Sao lưu / khôi phục** cơ sở dữ liệu ngay trên trang quản trị.

---

## Cài đặt: một máy chủ, một tệp

- **Máy chủ** là *một* tệp thực thi, trang quản trị đã nhúng sẵn bên trong. Không cần nginx, không cần dựng web riêng. Có sẵn Docker Compose nếu bạn thích cách đó. Cần PostgreSQL.
- **Máy trạm** cài lên từng PC Windows bằng bộ cài, khai mã máy một lần là xong. Có cơ chế cập nhật phiên bản máy khách từ trang quản trị.
- **Quầy** không cần thiết bị: điện thoại của bạn là quầy.
- **Mã nguồn mở (AGPL-3.0)**, không phí bản quyền cho phần lõi. Dữ liệu nằm trên máy chủ của bạn.

---

## Điều bạn cần chấp nhận trước khi dùng

- Nạp tiền mặt tại máy vẫn cần một người bấm duyệt. Chưa có tự xác nhận chuyển khoản.
- Chốt ca và sao lưu không tự chạy, phải bấm tay (chốt ca không bắt buộc với quán không ca).
- Máy trạm chỉ có bản Windows.
- Bạn phải tự dựng máy chủ (hoặc thuê người dựng một lần); hướng dẫn từng bước có trong `docs/HUONG-DAN.md`.

---

## Sắp tới

**VNET Cloud Hub + app di động cho game thủ:** khách đăng ký bằng số điện thoại, xem số dư ở mọi quán dùng VNET, nạp từ xa qua Momo/VietQR và tiền tự cộng vào ví tại quán, nhận thông báo "còn 5 phút" trên điện thoại. Với quán không nhân viên, đây là mảnh ghép khép kín vòng tự phục vụ. Dự kiến ba gói theo tháng, gói cơ bản cho quán dưới 20 máy.
