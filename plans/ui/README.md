## Admin UI Structure

### Vue Router Routes

```
/login                    → LoginPage.vue
/dashboard                → DashboardPage.vue
/members                  → MemberList.vue
/members/new              → MemberForm.vue
/members/:id              → MemberDetail.vue (tabs: info, transactions, sessions, combos)
/machines                 → MachineGrid.vue
/machines/:id             → MachineDetail.vue (tabs: info, sessions, assets, hardware, remote)
/machines/:id/remote      → RemoteControlPage.vue
/machine-groups           → MachineGroupList.vue
/combos                   → ComboList.vue
/combos/new               → ComboForm.vue
/combos/:id               → ComboDetail.vue
/bookings                 → BookingList.vue (calendar view or table)
/products                 → ProductList.vue
/categories               → CategoryTree.vue
/orders                   → OrderList.vue (real-time updates)
/orders/:id               → OrderDetail.vue
/orders/new               → POSPage.vue (POS interface)
/inventory                → InventoryLayout.vue
/inventory/materials      → MaterialList.vue
/inventory/stock          → StockTransactionList.vue
/inventory/count          → InventoryCountPage.vue
/inventory/suppliers      → SupplierList.vue
/promotions               → PromotionList.vue
/promotions/lucky-spin    → LuckySpinConfig.vue
/reports                  → ReportLayout.vue
/reports/daily            → DailyRevenueReport.vue
/reports/by-member        → MemberRevenueReport.vue
/reports/by-machine       → MachineRevenueReport.vue
/reports/by-employee      → EmployeeRevenueReport.vue
/reports/top-products     → TopProductReport.vue
/reports/inventory        → InventoryReport.vue
/shifts                   → ShiftList.vue
/stores                   → StoreList.vue
/chat                     → ChatPage.vue
/audit                    → AuditLogList.vue
/backups                  → BackupList.vue
/settings                 → SettingsPage.vue (tabs: general, pricing, curfew, printer, einvoice, website)
/settings/website-blocking → WebsiteBlockingPage.vue
/curfew                   → CurfewPolicyList.vue
/einvoice                 → EInvoiceConfigList.vue
/printer                  → PrinterConfigList.vue
```

### Màn hình chi tiết

#### DashboardPage.vue — Trang tổng quan admin

```
┌──────────────────────────────────────────────────────────┐
│ Thống kê hôm nay (4 StatCard)                            │
│ ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────┐ │
│ │ Doanh thu  │ │ Máy đang   │ │ Hội viên  │ │ Đơn    │ │
│ │ 5.200.000  │ │ dùng       │ │ mới hôm   │ │ hàng   │ │
│ │           │ │ 12/20      │ │ nay: 3    │ │ chờ: 5 │ │
│ └────────────┘ └────────────┘ └────────────┘ └────────┘ │
│                                                          │
│ Biểu đồ doanh thu 7 ngày (Chart.js — line/bar)           │
│ ┌──────────────────────────────────────────────────────┐ │
│ │  ▆                                                    │ │
│ │  █▆    ▅                                              │ │
│ │  ██▆  ▅█    ▄▅    ▃▄                                 │ │
│ │  ███  ██▆  ▄██    ██▄  ▅▆                           │ │
│ │ 17 18 19 20 21 22 23 24                               │ │
│ └──────────────────────────────────────────────────────┘ │
│                                                          │
│ Machine Status Grid                                      │
│ ┌────┐ ┌────┐ ┌────┐ ┌────┐ ┌────┐                    │
│ │ M01│ │ M02│ │ M03│ │ M04│ │ M05│                    │
│ │ 🟢  │ │ 🟢 │ │ 🔴 │ │ 🟡 │ │ 🟢 │                    │
│ └────┘ └────┘ └────┘ └────┘ └────┘                    │
│   (🟢 trống  🔴 đang dùng  🟡 bảo trì)                 │
│                                                          │
│ Ca hiện tại: Admin A (08:00 - ⏳)  [Đóng ca]             │
│ Top 5 sản phẩm bán chạy (bảng nhỏ)                       │
│   Trà sữa    25 ly    375.000                            │
│   Coca       20 lon   300.000                            │
│ ─────────────────────────────────────                    │
│ Chat widget (góc dưới phải) — unread badge               │
└──────────────────────────────────────────────────────────┘
```

Pinia store gọi:
```
dashboard.store.ts
  Actions: fetchDailyStats, fetchRevenueChart, fetchMachineStatus, fetchTopProducts
  State: dailyStats, revenueChart[], machineStatusMap, topProducts[], currentShift
```

#### MemberDetail.vue — 4 tabs

```
┌─ Member: MEM-001 ────────────────────────────────────────┐
│ [Sửa] [Nạp tiền] [Refund] [Xóa mềm]                      │
│                                                           │
│ Info | Transactions | Sessions | Combos                    │
│───────────────────────────────────────────────────────────│
│ Tab INFO:                                                  │
│ ┌──────────────┬────────────────────────────────────┐    │
│ │ Avatar       │ Full name: Nguyễn Văn A             │    │
│ │              │ Code: MEM-001                       │    │
│ │   [img]      │ Phone: 0901234567                   │    │
│ │              │ Tier: 🥇 Vàng (giảm 10%)            │    │
│ └──────────────┼────────────────────────────────────┘    │
│                │ Balance: 150.000 / Bonus: 20.000        │
│                │ Tổng đã chi (all-time): 12.500.000      │
│                │ Lần chơi cuối: 23/06 19:00 - 22:00     │
│                │ Ngày tạo: 01/01/2026                    │
│                └─────────────────────────────────────────│
│──────────────────────────────────────────────────────────│
│ Tab TRANSACTIONS:                                         │
│ Filter: loại ▼ | date range [__~__]                     │
│ ┌──────┬──────────┬──────────┬────────┬────────┬───────┐│
│ │ Ngày │ Loại     │ Số tiền  │ Balance│ Bonus  │ Admin ││
│ │      │          │          │ sau   │ sau    │       ││
│ ├──────┼──────────┼──────────┼────────┼────────┼───────┤│
│ │24/06 │ topup    │ +100.000 │150.000│ 20.000 │Admin A││
│ │23/06 │ game_time│ -30.000  │ 50.000│ 20.000 │ —     ││
│ │23/06 │ promotion│ +20.000  │ 80.000│ 20.000 │ —     ││
│ └──────┴──────────┴──────────┴────────┴────────┴───────┘│
│ Payment method chip (cash/qr_momo/member_balance...)     │
│──────────────────────────────────────────────────────────│
│ Tab SESSIONS:                                             │
│ Filter: date range [__~__]                               │
│ ┌──────┬──────┬──────┬──────┬──────┬───────┬──────────┐ │
│ │ Ngày │ Máy  │ Giờ  │ Phút │ Tiền │ Combo │ Trạng   │ │
│ ├──────┼──────┼──────┼──────┼──────┼───────┼──────────┤ │
│ │23/06 │M03   │19-22 │ 180  │45.000│ —     │✅ Ended │ │
│ │22/06 │M05   │14-17 │ 180  │ 0    │SC-001 │✅ Ended │ │
│ └──────┴──────┴──────┴──────┴──────┴───────┴──────────┘ │
│──────────────────────────────────────────────────────────│
│ Tab COMBOS:                                               │
│ ┌──────┬──────────┬──────────┬───────┬────────┬───────┐ │
│ │ Combo│ Mua lúc  │ Hết hạn  │ Còn   │ Trạng  │       │ │
│ │      │          │          │ (phút)│ thái   │       │ │
│ ├──────┼──────────┼──────────┼───────┼────────┼───────┤ │
│ │Sáng  │01/06     │30/06     │ —     │✅ Active│       │ │
│ │30h   │15/05     │14/06     │ 45    │⏳ Hết  │       │ │
│ └──────┴──────────┴──────────┴───────┴────────┴───────┘ │
│ Nếu chưa activate → button [Activate combo]              │
└──────────────────────────────────────────────────────────┘
```

#### MachineDetail.vue — 5 tabs

```
┌─ Machine: M03 ───────────────────────────────────────────┐
│ [Sửa] [Xóa mềm]           Status: 🟢 Đang dùng           │
│                                                           │
│ Info | Sessions | Assets | Hardware | Remote              │
│──────────────────────────────────────────────────────────│
│ Tab INFO:                                                  │
│   Code: M03                 Group: VIP                    │
│   IP: 192.168.1.103        MAC: AA:BB:CC:DD:EE:FF       │
│   Specs: i7-13700 / RTX 4070 / 32GB RAM / 512GB SSD     │
│   Vị trí: Tầng 1 - Khu VIP                               │
│   Store: Chi nhánh trung tâm                              │
│   Ngày tạo: 01/01/2026            Lần heartbeat cuối: 5s │
│──────────────────────────────────────────────────────────│
│ Tab SESSIONS:                                              │
│ ┌──────┬────────┬─────────┬──────┬───────┬───────┬──────┐│
│ │ Giờ  │ Member │ Thời    │ Tiền │ Combo │ Trạng│ Note ││
│ │      │        │ lượng   │      │       │ thái  │      ││
│ ├──────┼────────┼─────────┼──────┼───────┼───────┼──────┤│
│ │19-22│ MEM-001│ 180ph   │45.000│ —     │🔴Ended│      ││
│ │14-17│ SC-001 │ 180ph   │ 0    │ Sáng  │✅Ended│ combo││
│ └──────┴────────┴─────────┴──────┴───────┴───────┴──────┘│
│──────────────────────────────────────────────────────────│
│ Tab ASSETS:                                                │
│ [Thêm tài sản]                                            │
│ ┌──────────┬────────┬──────────┬────────┬────────┬─────┐ │
│ │ Loại     │ Hãng   │ Model    │ Serial │ Trạng  │ Check│
│ ├──────────┼────────┼──────────┼────────┼────────┼─────┤ │
│ │ 🖱 Mouse │ Logitech│G502     │ L123…  │ ✅ Good │ 23/06│
│ │ ⌨️KB    │ Corsair│K70      │ C456…  │ ❌ Damg │ 23/06│
│ └──────────┴────────┴──────────┴────────┴────────┴─────┘ │
│ Click asset → popup sửa + check-in form (status, notes,   │
│ photos)                                                    │
│──────────────────────────────────────────────────────────│
│ Tab HARDWARE:                                              │
│ ┌──────────────────────────────────────────────────────┐  │
│ │ CPU Temp: 65°C ───────────────▅▇██▇▆▅──── 24h      │  │
│ │ GPU Temp: 72°C ───▄▆███▆▄▃───▆▇██▇▆▅────           │  │
│ │ GPU Usage: 88% ▄▆███████▆▄▃▄▆███████▆▄▃───          │  │
│ │ RAM Usage: 62% ▃▄▆▇████▇▆▅▄▃▄▆▇████▇▆▅▄────         │  │
│ └──────────────────────────────────────────────────────┘  │
│ Switch: 1h / 6h / 24h / 7d                                │
│                                                           │
│ Alerts:                                                    │
│ ┌──────┬──────────┬────────┬──────────┬────────────────┐ │
│ │ Giờ  │ Loại     │ Value  │ Ngưỡng  │ Action         │ │
│ ├──────┼──────────┼────────┼──────────┼────────────────┤ │
│ │08:15 │cpu_temp  │ 92°C   │ 85°C     │ [Resolved]    │ │
│ └──────┴──────────┴────────┴──────────┴────────────────┘ │
│──────────────────────────────────────────────────────────│
│ Tab REMOTE:                                               │
│ ┌──────────────────────────────────────────────────────┐  │
│ │ [🔌 Shutdown] [🔄 Restart] [🔒 Lock] [💬 Message]  │  │
│ │ [🗙 Kill] [🚫 Block App] [⚡ Execute] [📷 Screenshot]│  │
│ ├──────────────────────────────────────────────────────┤  │
│ │ Screenshot preview:                                   │  │
│ │ ┌──────────────────────────────────────────────────┐ │  │
│ │ │  [Live Stream ▶]    [Refresh]                    │ │  │
│ │ │  ┌────────────────┐ DESKTOP-ABC                   │ │  │
│ │ │  │                │ Valorant đang chạy            │ │  │
│ │ │  │   img preview  │ CPU: 45%  RAM: 1.2GB         │ │  │
│ │ │  │                │                               │ │  │
│ │ │  └────────────────┘                               │ │  │
│ │ └──────────────────────────────────────────────────┘ │  │
│ ├──────────────────────────────────────────────────────┤  │
│ │ Process list (live):                                  │  │
│ │ ┌──────┬─────────────┬──────┬──────┬────────────────┐│  │
│ │ │ PID  │ Name        │ CPU  │ RAM  │ Action         ││  │
│ │ ├──────┼─────────────┼──────┼──────┼────────────────┤│  │
│ │ │ 1234 │ VALORANT.exe│ 45%  │1.2GB │ [Kill]         ││  │
│ │ │ 5678 │ chrome.exe  │ 12%  │ 800MB│ [Kill]         ││  │
│ │ └──────┴─────────────┴──────┴──────┴────────────────┘│  │
│ └──────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────┘
```

#### BookingList.vue — Calendar + Table view

```
┌──────────────────────────────────────────────────────────┐
│ [📅 Calendar] [📋 Table]  Filter: máy ▼ | ngày [____]  │
│──────────────────────────────────────────────────────────│
│ Calendar View (theo timeline):                            │
│ ┌──┬──────────────────────────────────────────────────┐  │
│ │09│ ─── M03: Tuấn (19-22h) ─── 🟢 Checked-in         │  │
│ │10│                                                    │  │
│ │  │ ┌──────────────────────────────────────────────┐  │  │
│ │  │ │ Deposit: 50.000 | Tạo: 20/06 08:00         │  │  │
│ │  │ │ [Check-in] [Cancel] [Mark no-show]          │  │  │
│ │  │ └──────────────────────────────────────────────┘  │  │
│ │14│ ─── M05: Nam (14-17h) ─── 🟡 Pending             │  │
│ │  │ ┌──────────────────────────────────────────────┐  │  │
│ │  │ │ Khách lẻ | 0901234567 | Không cọc           │  │  │
│ │  │ │ [Check-in] [Cancel] [Mark no-show]          │  │  │
│ │  │ └──────────────────────────────────────────────┘  │  │
│ │15│                                                    │  │
│ │17│ ─── M08: Khách lẻ (17-19h) ─── 🔴 No-show         │  │
│ │19│ ─── M03: Hùng (21-23h) ─── ⚫ Cancelled           │  │
│ └──┴──────────────────────────────────────────────────┘  │
│──────────────────────────────────────────────────────────│
│ Table View:                                               │
│ ┌──────┬──────┬──────────┬───────┬────────┬────────┬───┐│
│ │ Máy  │ Giờ  │ Khách    │ Cọc   │ Trạng  │ Tạo    │   ││
│ │      │      │          │       │ thái   │ lúc    │   ││
│ ├──────┼──────┼──────────┼───────┼────────┼────────┼───┤│
│ │ M03  │19-22 │ Tuấn     │ 50k   │ 🟢 CI   │ 20/06 │ 📝││
│ │ M05  │14-17 │ Nam      │ 0     │ 🟡 Pnd  │ 22/06 │ 📝││
│ └──────┴──────┴──────────┴───────┴────────┴────────┴───┘│
│ Click row → booking detail popup + actions               │
└──────────────────────────────────────────────────────────┘
```

#### ShiftList.vue + ShiftReportView

```
┌──────────────────────────────────────────────────────────┐
│ [Mở ca]                              Ca hiện tại: đang mở│
│ ┌──────────┬───────┬───────┬───────┬───────┬───────────┐ │
│ │ NV       │ Giờ   │ Giờ   │ Đầu   │ Cuối  │ Trạng thái │
│ │          │ mở    │ đóng  │ ca    │ ca    │           │ │
│ ├──────────┼───────┼───────┼───────┼───────┼───────────┤ │
│ │ Admin A  │08:00  │17:00  │500k   │5.2tr  │✅ Đã đóng │ │
│ │ Admin B  │17:00  │ —     │500k   │ —     │⏳ Đang mở │ │
│ │ Admin A  │07/06  │15/06  │300k   │4.8tr  │✅ Đã đóng │ │
│ └──────────┴───────┴───────┴───────┴───────┴───────────┘ │
│                                                           │
│ Click ca đã đóng → ShiftReportView:                       │
│ ┌─ Báo cáo ca ─────────────────────────────────────────┐ │
│ │ Nhân viên: Admin A          Giờ: 08:00 → 17:00      │ │
│ │ Tổng doanh thu: 5.200.000                            │ │
│ │ ├─ Cash: 3.000.000                                   │ │
│ │ ├─ Member balance: 1.500.000                         │ │
│ │ ├─ QR Momo: 500.000                                  │ │
│ │ └─ Bonus used: 200.000 (KHÔNG tính doanh thu)        │ │
│ │                                                       │ │
│ │ Orders trong ca: 45                                  │ │
│ │ Top SP: Trà sữa (25), Coca (20)                      │ │
│ │                                                       │ │
│ │ Cash Handover lịch sử:                                │ │
│ │ ┌──────┬──────────┬────────┬──────────┬───────────┐  │ │
│ │ │ Giờ  │ Loại     │ Số tiền│ Lý do    │ Người tạo│  │ │
│ │ ├──────┼──────────┼────────┼──────────┼───────────┤  │ │
│ │ │10:00 │ cash_out │ 500k   │ Nạp quỹ │ Admin A   │  │ │
│ │ │12:00 │ cash_in  │ 300k   │ Thu tiền │ Admin A   │  │ │
│ │ └──────┴──────────┴────────┴──────────┴───────────┘  │ │
│ │                                                       │ │
│ │ Số dư đầu ca: 500.000                                  │ │
│ │ Số dư cuối ca: 5.200.000                               │ │
│ │ Kỳ vọng: 5.150.000                                     │ │
│ │ Chênh lệch: +50.000 (⚠️ cần kiểm tra)                  │ │
│ │                                                       │ │
│ │ [Bàn giao tiền] [In báo cáo ca] [Xuất Excel]          │ │
│ └───────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────┘
```

#### BackupList.vue

```
┌──────────────────────────────────────────────────────────┐
│ [Tạo backup mới]    Filter: status ▼                    │
│ ┌──────────────┬────────┬──────────┬────────┬──────────┐│
│ │ Tên file     │ Kích   │ Ngày tạo │ Trạng  │ Ghi chú  ││
│ │              │ thước  │          │ thái   │          ││
│ ├──────────────┼────────┼──────────┼────────┼──────────┤│
│ │vnet_20260624 │ 245MB  │ 24/06    │✅Xong  │ Auto     ││
│ │vnet_20260623 │ 240MB  │ 23/06    │✅Xong  │ Auto     ││
│ │vnet_20260624 │ —      │ 24/06    │⏳Đang.. │ Thủ công ││
│ ├──────────────┼────────┼──────────┼────────┼──────────┤│
│ │...           │        │          │        │          ││
│ └──────────────┴────────┴──────────┴────────┴──────────┘│
│ Click file → confirm [Restore] (modal: "Mất dữ liệu hiện │
│ tại? Nhập admin password để xác nhận")                   │
└──────────────────────────────────────────────────────────┘
```

#### AuditLogList.vue

```
┌──────────────────────────────────────────────────────────┐
│ Filter: action ▼ | entity_type ▼ | user ▼ | date range  │
│ ┌──────────┬──────────────┬──────────┬────────┬──────┐ │
│ │ Thời gian│ Hành động    │ Entity   │ Người  │ IP  │ │
│ │          │              │          │ dùng   │     │ │
│ ├──────────┼──────────────┼──────────┼────────┼──────┤ │
│ │24/06 08:│ member.topup │ MEM-001  │Admin A │127.…│ │
│ │24/06 07:│ machine.create│ M03     │Admin A │10.0.…│ │
│ │23/06 22:│ session.end   │ SES-001  │ System  │—    │ │
│ │23/06 19:│ curfew.enforce│ SES-002  │ System  │—    │ │
│ └──────────┴──────────────┴──────────┴────────┴──────┘ │
│                                                           │
│ Click row → modal JSON viewer:                            │
│ ┌──────────────────────────────────────────────────────┐ │
│ │ {                                                      │ │
│ │   "action": "member.topup",                           │ │
│ │   "entity_type": "member",                            │ │
│ │   "entity_id": "uuid",                                │ │
│ │   "user_id": "uuid",                                  │ │
│ │   "metadata": {                                       │ │
│ │     "amount": 100000,                                 │ │
│ │     "method": "cash",                                 │ │
│ │     "balance_before": 50000,                          │ │
│ │     "balance_after": 150000                           │ │
│ │   },                                                  │ │
│ │   "ip_address": "127.0.0.1",                          │ │
│ │   "created_at": "2026-06-24T08:15:00Z"               │ │
│ │ }                                                      │ │
│ └──────────────────────────────────────────────────────┘ │
│ [Copy JSON]                                               │
└──────────────────────────────────────────────────────────┘
```

#### SettingsPage.vue — 6 tabs

```
┌─ Cài đặt ────────────────────────────────────────────────┐
│ General | Pricing | Curfew | Printer | E-Invoice | Website│
│──────────────────────────────────────────────────────────│
│ Tab GENERAL:                                               │
│   ┌────────────────────────────────────────────────────┐ │
│   │ Thông tin cửa hàng:                                │ │
│   │   Tên: [VNET Trung tâm              ]              │ │
│   │   Địa chỉ: [123 Nguyễn Huệ, Q1       ]             │ │
│   │   SĐT: [0901234567                    ]             │ │
│   │   Mã số thuế: [0123456789             ]             │ │
│   ├────────────────────────────────────────────────────┤ │
│   │ Cài đặt tính giờ:                                  │ │
│   │   Làm tròn: round_up ▼  Khoảng: [60] phút         │ │
│   │   Ân hạn: [10] phút                               │ │
│   │   Đơn vị tối thiểu: [30] phút                     │ │
│   ├────────────────────────────────────────────────────┤ │
│   │ Mệnh giá nạp (máy trạm):                           │ │
│   │   [5k] [10k] [20k] [50k] [100k] [200k] [+]        │ │
│   ├────────────────────────────────────────────────────┤ │
│   │ Tự động hủy no-show sau: [30] phút                │ │
│   └────────────────────────────────────────────────────┘ │
│                                                           │
│ Tab PRICING:                                               │
│   ┌────────────────────────────────────────────────────┐ │
│   │ Nhóm máy: VIP ▼                                    │ │
│   ├────────────────────────────────────────────────────┤ │
│   │ Giá cơ bản (machine_prices):                       │ │
│   │ ┌──────────┬─────────────┬────────┬──────────────┐│ │
│   │ │ Hạng     │ Giá/giờ     │ Áp dụng│ Action       ││ │
│   │ ├──────────┼─────────────┼────────┼──────────────┤│ │
│   │ │ Mọi hạng │ 15.000      │ ✅     │ [Sửa] [Xóa] ││ │
│   │ │ Vàng     │ 12.000      │ ✅     │ [Sửa] [Xóa] ││ │
│   │ │ Bạc      │ 13.000      │ ✅     │ [Sửa] [Xóa] ││ │
│   │ └──────────┴─────────────┴────────┴──────────────┘│ │
│   │ [Thêm giá]                                          │ │
│   ├────────────────────────────────────────────────────┤ │
│   │ Giá theo giờ (time_based_pricings):                │ │
│   │ ┌──────┬───────┬──────┬───────┬──────┬───────────┐│ │
│   │ │ Thứ  │ Giờ   │ Giờ  │ Giá   │ Group│ Action    ││ │
│   │ ├──────┼───────┼──────┼───────┼──────┼───────────┤│ │
│   │ │ T2-T5│ 08:00 │ 17:00│ 12.000│ VIP  │ [Sửa][Xóa]││ │
│   │ │ T6-CN│ 17:00 │ 24:00│ 20.000│ VIP  │ [Sửa][Xóa]││ │
│   │ └──────┴───────┴──────┴───────┴──────┴───────────┘│ │
│   │ [Thêm khung giờ]                                    │ │
│   └────────────────────────────────────────────────────┘ │
│                                                           │
│ Tab CURFEW:                                                │
│   ┌────────────────────────────────────────────────────┐ │
│   │ [Thêm chính sách]  [Override]                     │ │
│   │ ┌────────┬──────┬──────┬─────────┬──────┬────────┐│ │
│   │ │ Tên    │Giờ bđ│Giờ kt│Áp dụng  │Hiệu  │Action  ││ │
│   │ ├────────┼──────┼──────┼─────────┼──────┼────────┤│ │
│   │ │ Giờ học│22:00 │06:00 │T2-T6    │ ✅   │[Sửa]   ││ │
│   │ │ Cuối   │23:00 │05:00 │T7-CN    │ ✅   │[Override]││
│   │ └────────┴──────┴──────┴─────────┴──────┴────────┘│ │
│   └────────────────────────────────────────────────────┘ │
│                                                           │
│ Tab PRINTER:                                               │
│   ┌────────────────────────────────────────────────────┐ │
│   │ [Thêm máy in]         [Test tất cả]               │ │
│   │ ┌──────────┬───────┬──────┬─────────┬──────┬─────┐│ │
│   │ │ Tên      │ IP    │ Port │ Loại    │ Mặc  │Act  ││ │
│   │ ├──────────┼───────┼──────┼─────────┼──────┼─────┤│ │
│   │ │ Bếp      │10.0.0.2│9100 │ kitchen │ ✅   │[Sửa]││ │
│   │ │ Bar      │10.0.0.3│9100 │ bar     │ —    │[Sửa]││ │
│   │ │ Quầy     │10.0.0.4│9100 │ receipt │ ✅   │[Sửa]││ │
│   │ └──────────┴───────┴──────┴─────────┴──────┴─────┘│ │
│   └────────────────────────────────────────────────────┘ │
│                                                           │
│ Tab E-INVOICE:                                             │
│   ┌────────────────────────────────────────────────────┐ │
│   │ Tên công ty: [Công ty VNET               ]        │ │
│   │ Mã số thuế: [0123456789                  ]        │ │
│   │ Địa chỉ: [123 Nguyễn Huệ, Q1            ]        │ │
│   │ Serial pattern: [VNET/24E/  ] [_________]        │ │
│   │ [Test kết nối] [Lưu]                              │ │
│   └────────────────────────────────────────────────────┘ │
│                                                           │
│ Tab WEBSITE:                                               │
│   ┌────────────────────────────────────────────────────┐ │
│   │ Chế độ: [🔘 Blacklist] [○ Whitelist]             │ │
│   ├────────────────────────────────────────────────────┤ │
│   │ [Thêm rule]                                        │ │
│   │ ┌──────────────┬──────────┬───────────┬──────────┐│ │
│   │ │ Pattern      │ Category │ Lịch     │ Action   ││ │
│   │ ├──────────────┼──────────┼───────────┼──────────┤│ │
│   │ │*.facebook.com│ social   │ T2-T6 08-17│ [Sửa]  ││ │
│   │ │*.youtube.com │ video    │ T2-T6 08-17│ [Sửa]  ││ │
│   │ └──────────────┴──────────┴───────────┴──────────┘│ │
│   ├────────────────────────────────────────────────────┤ │
│   │ Lịch vi phạm:                                      │ │
│   │ ┌──────────┬───────┬──────────┬────────┬────────┐│ │
│   │ │ Giờ      │ Máy   │ Domain   │ Hành   │ Action ││ │
│   │ │          │       │          │ động   │        ││ │
│   │ ├──────────┼───────┼──────────┼────────┼────────┤│ │
│   │ │24/06 09:│ M03   │ facebook │ blocked│ 📝     ││ │
│   │ │24/06 10:│ M05   │ fb.com   │ allowed│ 📝     ││ │
│   │ └──────────┴───────┴──────────┴────────┴────────┘│ │
│   └────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────┘
```

### Component Tree — Shared

```
AppLayout.vue
├── AppSidebar.vue (collapsible nav, menu groups)
├── AppHeader.vue (user avatar, notification dropdown, machine status dot, global search)
├── ChatFloatingWidget.vue (right bottom corner)
└── <RouterView />

Shared components:
├── DataTable.vue
│   Props: columns[], data[], loading, pagination, selectable
│   Columns: { field, label, width, sortable, filterable, formatter(value) -> string }
│   Events: @sort, @filter, @page-change, @row-click
├── SearchBar.vue (v-model, placeholder, @search debounced 300ms)
├── FormDialog.vue (title, width, @confirm, confirmLoading, @close)
├── ConfirmDialog.vue (message, @confirm, @cancel)
├── StatusBadge.vue (variant: 'success' | 'warning' | 'danger' | 'info' | 'default')
├── StatCard.vue (label, value: number, icon: string, trend: 'up' | 'down' | 'flat')
├── MachineCard.vue (code, status: dot color, cpu_temp, gpu_temp, current_user)
├── FormField.vue (label, error: string, helper: string, required: boolean)
├── ImageUpload.vue (multiple, preview, maxSize, @upload)
├── CurrencyInput.vue (v-model: number, formatted: VND with commas)
├── EmptyState.vue (icon, title, description, actionLabel, @action)
├── LoadingSkeleton.vue (type: 'table' | 'card' | 'chart', rows)
├── DateRangePicker.vue (v-model: [Date, Date], presets: 'Hôm nay', '7 ngày', 'Tháng này')
└── NotificationBell.vue (unread count badge, dropdown list, mark-read)
```

### Stack & Dependencies

```json
{
  "dependencies": {
    "vue": "^3.4",
    "vue-router": "^4.3",
    "pinia": "^2.1",
    "element-plus": "^2.7",
    "@element-plus/icons-vue": "^2.3",
    "axios": "^1.7",
    "dayjs": "^1.11",
    "chart.js": "^4.4",
    "vue-chartjs": "^5.3",
    "@vueuse/core": "^10.11",
    "@tanstack/vue-virtual": "^3.5"
  },
  "devDependencies": {
    "vite": "^5.4",
    "@vitejs/plugin-vue": "^5.0",
    "sass": "^1.77",
    "unplugin-auto-import": "^0.17",
    "unplugin-vue-components": "^0.27"
  }
}
```

### Plugins Layer

```
src/plugins/
├── axios.ts           # Axios instance + interceptor JWT refresh (401 → refresh → retry)
├── dayjs.ts           # Locale vi, format: DD/MM/YYYY HH:mm, relativeTime
├── websocket.ts       # useWebSocket wrapper — auto reconnect, heartbeat ping/pong
└── form-rules.ts      # Shared validation patterns (tái sử dụng cho nhiều form)
```

**form-rules.ts:**

```ts
export const rules = {
  required:   { required: true,                            message: 'Không được để trống' },
  phone:      { pattern: /^0\d{9,10}$/,                    message: 'SĐT gồm 10-11 số, bắt đầu 0' },
  email:      { type: 'email',                             message: 'Email không hợp lệ' },
  positiveInt:{ type: 'number', min: 0,                    message: 'Phải >= 0' },
  amount:     { type: 'number', min: 1000,                 message: 'Tối thiểu 1.000đ' },
  ipAddress:  { pattern: /^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$/, message: 'IP không hợp lệ' },
  macAddress: { pattern: /^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$/, message: 'MAC không hợp lệ' },
  port:       { type: 'number', min: 1, max: 65535,        message: 'Port 1-65535' },
  dateOrder:  { validator: (_r, v, cb) => v > form.start ? cb() : cb(new Error('Phải sau ngày bắt đầu')) },
}
```

**axios.ts — Interceptor pattern:**

```ts
import axios from 'axios'
import { useAuthStore } from '@/stores/auth.store'

const api = axios.create({ baseURL: '/api' })

api.interceptors.response.use(
  res => res,
  async err => {
    if (err.response?.status === 401) {
      const auth = useAuthStore()
      const ok = await auth.refreshToken()
      if (!ok) { auth.logout(); return }
      err.config.headers.Authorization = `Bearer ${auth.token}`
      return api(err.config)
    }
    return Promise.reject(err)
  }
)
```

### Element Plus Form Rules — Chi tiết từng màn

Tất cả form dùng `el-form` + `rules` prop của Element Plus. Không cần thêm thư viện validate nào.

| Màn | Rules đặc thù |
|---|---|
| **LoginPage** | username required, password required min 4 |
| **MemberForm** | phone, email, date_of_birth (>= 1950, < today), id_card_number pattern |
| **TopupDialog** | amount > 0, bội số 1000 |
| **RefundDialog** | amount <= member.balance, reason required |
| **ProductForm** | price > 0, options: max_select >= 1, sort_order >= 0 |
| **ComboForm** | fixed_slot: slot_start < slot_end, apply_days tồn tại. prepaid: total_minutes > 0 |
| **MachineForm** | ip_address pattern, mac_address pattern, machine_code unique |
| **MachineGroupForm** | name required, color HEX (#xxx) |
| **BookingForm** | booked_to > booked_from, deposit_amount >= 0 |
| **ShiftCloseForm** | closing_balance >= 0, confirm dialog nếu discrepancy |
| **PricingForm** | price_per_hour > 0, effective_to > effective_from, min_duration >= 1 |
| **PrinterForm** | ip_address pattern, port 1-65535, printer_type required |
| **PromotionForm** | valid_to > valid_from, priority >= 0, reward_value phù hợp type |
| **WebsiteRuleForm** | pattern wildcard hợp lệ (\*.domain.com), schedule: start < end |
| **SettingsForm** | rounding interval > 0, grace_period >= 0, topup presets tất cả > 0 |
| **RoleForm** | permission list phải có ít nhất 1 permission |

### Pinia Stores

```
stores/
├── auth.store.ts
│   State: user, token, permissions[], loading
│   Getters: isAuthenticated, hasPermission(code)
│   Actions: login, logout, refreshToken, fetchMe

├── member.store.ts
│   State: list[], current, pagination, filters
│   Actions: fetchList, fetchDetail, create, update, topup, refund

├── machine.store.ts
│   State: list[], groups[], statusMap: Record<id, MachineStatus>, current
│   Actions: fetchList, fetchDetail, updateStatus, heartbeat
│   WebSocket listener: machine:status-change → update statusMap

├── session.store.ts
│   State: activeSessions[], current
│   Actions: startSession, endSession, changeMachine

├── order.store.ts
│   State: list[], current, pagination, filters
│   Actions: fetchList, fetchDetail, create, addPayment, updateStatus
│   WebSocket listener: order:new, order:status-change → push to list

├── combo.store.ts
│   State: combos[], purchases[], types
│   Actions: fetchCombos, create, purchase, activate

├── product.store.ts
│   State: categories: Tree[], products[], current
│   Actions: fetchTree, fetchProducts, create, update

├── promotion.store.ts
│   State: promotions[], luckySpinRewards[]
│   Actions: fetch, create, spin, fetchSpinHistory

├── chat.store.ts
│   State: conversations[], messages[], unreadCount, activeConversation
│   Actions: fetchConversations, sendMessage, markRead, processTopupRequest
│   WebSocket listener: chat:message, chat:typing, chat:topup-request

**Admin Chat — Xử lý topup request:**
```
Chat message từ máy trạm có dạng:
  "💳 M03 yêu cầu nạp 100k"

Admin click nút [Xác nhận] trong message:
  → processTopupRequest(conversationId, amount)
  → POST /api/members/:id/topup
  → chat.sendMessage("✅ Đã nạp 100k. Số dư: ...")
  → member.store.fetchDetail(id) (refresh số dư)

Nếu admin muốn nhập số tiền khác (VD: gamer ghi "nạp 200k" nhưng chỉ đưa 150k):
  → Admin nhập amount = 150000
  → processTopupRequest với amount đã sửa
```

├── hardware.store.ts
│   State: snapshots, alerts[], thresholds
│   Actions: fetchSnapshots, fetchAlerts, resolveAlert

├── shift.store.ts
│   State: currentShift, list
│   Actions: openShift, closeShift, fetchCurrent

├── store.store.ts
│   State: stores[], current
│   Actions: fetchList, create, update

└── settings.store.ts
│   State: settings (loaded once on app start, cached)
│   Actions: fetchSettings, updateSetting
```

### POS Page (Order Page) — Chi tiết

```
POSPage.vue layout:
┌──────────────────────────────┬──────────────────────────────┐
│  Left Panel (Menu)          │  Right Panel (Cart)         │
│                              │                              │
│  Category Tabs:             │  Order summary:              │
│  [Đồ uống] [Đồ ăn] [Snack]   │  - Machine: M03              │
│                              │  - Member: Nguyễn Văn A      │
│  Product Grid:              │  - Items:                    │
│  ┌────┐ ┌────┐ ┌────┐       │    Coca 2 x 15k = 30k       │
│  │Coca│ │Pepsi│ │Sting│      │    Snack 1 x 10k = 10k       │
│  │15k │ │15k │ │12k │       │                              │
│  └────┘ └────┘ └────┘       │  Total: 40k                 │
│                              │                              │
│  Click product → add 1 qty   │  Actions:                    │
│  Right click → choose qty    │  [Thanh toán] [Tách đơn]    │
│                              │                              │
│  Search: [_________]         │  Payment modal:              │
│                              │    Tổng: 40k                 │
│                              │    Member balance: 100k      │
│                              │    Trừ balance: [  40k  ]   │
│                              │    Cash: [  0  ]            │
│                              │    [Xác nhận]               │
└──────────────────────────────┴──────────────────────────────┘

**Mở máy khách vãng lai:**
```
POS toolbar: [🧑 Mở máy cho khách]
→ Popup: chọn máy (dropdown) + chọn thời gian (combo hoặc giờ thường)
→ Click "Mở máy"
→ Server tạo machine_sessions (member_id = NULL)
→ Client tự động unlock + vào Dashboard (dạng member, không cần login)
```

**Tạo combo member tại quầy:**
```
POS toolbar: [💠 Tạo combo + tạo hội viên]
→ Chọn combo (dropdown)
→ Nhập tên khách (optional) → in QR code cho khách
→ Server:
   1. Tạo member mới (role='combo', code = {combo.member_prefix}-{counter})
   2. Tạo combo_purchases gắn với member đó
   3. Trả về QR code / thông tin login
→ Khách dùng QR login máy trạm → vào Combo Dashboard
```

