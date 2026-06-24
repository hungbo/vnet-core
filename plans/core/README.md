# VNET Core — Quản lý tính tiền & Hội viên

## Auth & Phân quyền
- Đăng nhập/đăng xuất (Admin, Nhân viên)
- Phân quyền vai trò (Owner, Manager, Staff)
- JWT token + Refresh token
- QR code login máy trạm
- **Admin login trên client**: username/password → session `is_admin=true`, không tạo machine_sessions, không tính giờ. Ẩn nút Nạp tiền + Order.
- Phân quyền đến từng chức năng (members.create, reports.view...)

## Quản lý Hội viên
- CRUD hội viên (tạo tay, qua CCCD, qua số điện thoại)
- Phân hạng hội viên (Vàng, Bạc, Đồng...) dựa trên tổng chi tiêu
- Xác thực CCCD để check tuổi (minor policy)
- Nạp tiền (cash, QR Momo, chuyển khoản) — admin thực hiện
- **Nạp tiền từ client**: hội viên chọn mệnh giá (config: `topup_presets`), gửi yêu cầu qua chat → admin xác nhận → cập nhật số dư
- Hoàn tiền
- Lịch sử giao dịch (có phân trang)
- Điểm danh tích lũy / quà tặng
- Quà tặng lần đầu nạp
- Dọn dẹp hội viên (soft delete các hội viên inactive lâu ngày)

## Bonus Balance & Payment Priority
- `members.balance` = tiền thật → tính doanh thu, xuất hóa đơn
- `members.bonus_balance` = tiền KM → không tính doanh thu
- Khi thanh toán, ưu tiên trừ bonus_balance trước
- `promotion_bonus` → vào bonus_balance; `topup` → vào balance

## Quản lý Máy trạm & Thiết bị ngoại vi
- Nhóm máy (VIP, Thường, Thi đấu...)
- CRUD máy trạm
- Theo dõi trạng thái (trống/đang chơi/bảo trì)
- Đặt giữ máy từ quầy thu ngân (booking + deposit)
- Theo dõi nhiệt độ CPU/GPU
- **Asset Management**: Chuột, Bàn phím, Tai nghe, Ghế gắn với từng máy

## Tính tiền giờ
- Auto đếm giờ khi login máy trạm
- Giá theo nhóm máy + khung giờ
- Grace period (5-10 phút ân hạn)
- Combo: `fixed_slot` (vé khung giờ, auto end theo slot) hoặc `prepaid` (trừ dần remaining)
- Thẻ nạp tiền (chính/phụ)
- Khách vãng lai (không cần đăng nhập)
- Làm tròn giờ: config trong system_settings
- Pricing engine: machine_prices → time_based_pricings override → grace period → rounding → discount

## Booking & Deposit
- Khách gọi điện/online đặt máy trước
- Đặt cọc (deposit) khi booking, ghi deposit_transaction_id
- Auto-cancel nếu khách không đến sau X phút
- Check-in / No-show / Cancel flow

## Minor Policy & Giờ giới nghiêm
- Xác thực tuổi từ CCCD
- Giới hạn giờ chơi cho khách dưới 18 tuổi
- Tự động lock máy khi đến giờ giới nghiêm
- Cấu hình curfew theo ngày trong tuần
- Override cho giải đấu đêm (ghi audit log)

## Combo Member — Tự động tạo khi mua combo

Khi admin tạo combo tại quầy → hệ thống tự động tạo member mới:

```
1. Admin chọn combo "Sáng chiều" (member_prefix = 'SANGCHIEU', member_count = 0)
2. Server gen: code = 'SANGCHIEU-001', member_count++
3. Tạo members: { code, full_name = "SANGCHIEU-001", role = 'combo', password_hash = NULL }
4. Tạo combo_purchases gắn với member_id
5. In QR cho khách (hoặc gửi SMS)
```

Combo member:
- `role = 'combo'` → `balance = 0`, không thể nạp tiền
- Có thể order đồ ăn (trả cash/QR tại quầy)
- Hết combo → popup thông báo → auto logout
- Không cleanup (chỉ set `is_active = false` sau 30 ngày nếu cần)

## Combo — 2 loại

### `fixed_slot`
- Vé vào cửa theo khung giờ (VD: 7h-12h sáng)
- Mua 1 lần, dùng được nhiều ngày trong hạn (validity_days)
- Không giới hạn số lần vào trong tháng, mỗi lần trong khung giờ
- Có thể kèm product tặng (combo_items item_type='product')
- Đổi máy thoải mái trong khung giờ

### `prepaid`
- Gói giờ chơi trừ dần (VD: 30h trong 30 ngày)
- remaining_minutes giảm dần mỗi lần chơi
- Có thể kèm product tặng
- Đổi máy thoải mái (current_session_id quản lý session active)

## Bảng tin & Thông báo
- Bảng tin phòng máy (thông báo events, giải đấu)
- Gửi thông báo đến máy trạm
- Quay số trúng thưởng

## Báo cáo & Thống kê
- Doanh thu theo ngày/tuần/tháng
- Doanh thu theo nhân viên / máy / nhóm máy
- Báo cáo hội viên (nạp/chơi)
- Nhật ký hệ thống + giao dịch

## Khuyến mãi
- Chính sách giá theo nhóm người dùng (roles)
- Khuyến mãi theo giờ (time_discount)
- Combo kèm service tặng
- Khuyến mãi lần đầu nạp (first_topup)
- Lucky spin: quay số trúng thưởng

## Transaction & Concurrency

| Operation | Query |
|---|---|
| Trừ tiền hội viên | `SELECT balance FROM members WHERE id = ? FOR UPDATE` |
| Cộng/trừ tồn kho | `SELECT current_stock FROM materials WHERE id = ? FOR UPDATE` |
| Tạo machine session | `SELECT status FROM machines WHERE id = ? FOR UPDATE` |
| Kết ca | `SELECT SUM(amount) FROM payments WHERE shift_id = ? FOR UPDATE` |
| Nạp tiền + bonus cùng lúc | Transaction bao gồm cả 2 operations |

**Nguyên tắc:** Luôn lock row trước khi đọc giá trị hiện tại, update trong cùng transaction, commit sau khi hoàn tất.
