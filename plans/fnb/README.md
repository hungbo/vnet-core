# VNET F&B — Quản lý F&B

## Menu & Sản phẩm
- Danh mục sản phẩm (đồ ăn, thức uống, snack) — nested category
- Sản phẩm với tùy chọn (size, khẩu vị) — product_option_groups + product_options
- Combo (combo_items với item_type = 'time' | 'product')
- Nhóm mặt hàng
- Thứ tự hiển thị menu
- Printer routing (categories.printer_id fallback; product → printer mapping)

## POS (Bán hàng)
- Tạo đơn hàng (tại quán / mang đi / machine_order)
- Chọn bàn/số máy
- **Split payment**: 1 order → nhiều payments
- Thanh toán (cash, QR, tài khoản hội viên, thẻ quà tặng)
- In hóa đơn / in order cho bếp/bar (printer routing)
- POS page layout: Menu grid (trái) + Cart panel (phải) + Payment modal

## Order Management
- Trạng thái: pending → confirmed → preparing → ready → served → completed
- Hủy / Hoàn tiền
- Kitchen/Bar display real-time
- Lịch sử orders

## Quản lý Ca & Nhân viên
- Ca làm việc (open → close)
- Kết ca / bàn giao tiền (cash_handovers)
- Discrepancy tracking

## Kho hàng (Inventory)
- Nguyên vật liệu (materials) với đơn vị tính (units)
- Nhà cung cấp (suppliers)
- Nhập kho (purchase, return, transfer_in)
- Xuất kho (production_usage, tự động nếu có product_materials mapping)
- Kiểm kho / chốt kho (inventory_counts, cân bằng chênh lệch)
- Cảnh báo tồn kho tối thiểu (min_stock)
- Hàng hóa thất lạc (loss)
- Báo cáo tồn kho + điều chỉnh

## Thẻ quà tặng / Voucher
- Tạo/Sửa/Xóa thẻ quà tặng (gift_cards)
- Bán thẻ quà tặng
- Thanh toán bằng thẻ quà tặng
- Lịch sử giao dịch thẻ

## Báo cáo F&B
- Doanh số theo đơn hàng
- Doanh số theo danh mục/sản phẩm
- Doanh số theo giờ / bàn / máy
- Top products
- Báo cáo hoàn tiền, thuế, khuyến mãi

## Printer Routing

```sql
-- Mặc định: categories.printer_id (fallback)
-- Nếu product có mapping riêng → override
product_printer_mapping (product_id → printer_id)
```

Loại printer: 'receipt' | 'kitchen' | 'bar'
