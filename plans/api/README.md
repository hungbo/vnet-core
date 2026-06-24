## API Design

### Conventions

**Base URL:** `/api`

**Response format:**
```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

**Paginated response:**
```json
{
  "code": 0,
  "data": {
    "items": [],
    "total": 100,
    "page": 1,
    "page_size": 20
  }
}
```

**Error response:**
```json
{
  "code": 40001,
  "message": "member not found"
}
```

**Pagination:** `?page=1&page_size=20&sort=-created_at` (prefix `-` = desc)

**Filter:** `?status=active` (exact), `?name__like=keyword` (text search), `?created_at__gte=2026-01-01` (range)

**Auth:** `Authorization: Bearer <jwt>`

### Endpoints

#### Auth
| Method | Path | Description |
|---|---|---|
| POST | `/api/auth/login` | Login username/password → trả dashboard theo role |
| POST | `/api/auth/refresh` | Refresh token |
| POST | `/api/auth/qr-login` | QR code scan → verify → trả dashboard theo role |
| GET | `/api/auth/me` | Current user info + permissions |
| PUT | `/api/auth/change-password` | Change password |
| GET | `/api/auth/permissions` | All permissions (list) |

**POST /api/auth/login:**
```json
// Request
{ "username": "admin", "password": "xxx", "machine_id": "uuid" }
// Response — server check members.role:
//   'member' → tạo session, tính giờ
//   'combo'  → tạo session, tính giờ combo
//   'admin'  → is_admin=true, không tạo session, không tính giờ
{
  "access_token": "...",
  "is_admin": false,
  "role": "member | combo | admin",
  "session_id": "uuid | null (nếu admin)",
  "user": { "id": "uuid", "full_name": "Nguyễn Văn A", "role": "member" }
}
```

**POST /api/auth/qr-login:**
```json
// Request (từ Agent machine)
{ "qr_code": "uuid", "machine_id": "uuid", "mac_address": "..." }
// Response — tương tự login, check role
{
  "is_admin": false,
  "role": "member | combo | admin",
  "session_id": "uuid | null",
  "member": { "id", "name", "balance", "bonus_balance", "role" }
}
```

#### Members
| Method | Path | Description |
|---|---|---|
| GET | `/api/members` | List (paginated, filterable) |
| POST | `/api/members` | Create |
| GET | `/api/members/:id` | Detail + current tier info |
| PUT | `/api/members/:id` | Update |
| DELETE | `/api/members/:id` | Soft delete |
| POST | `/api/members/:id/topup` | Nạp tiền (main balance) |
| POST | `/api/members/:id/refund` | Hoàn tiền |
| GET | `/api/members/:id/transactions` | Lịch sử giao dịch (paginated) |
| GET | `/api/members/:id/sessions` | Lịch sử session |
| GET | `/api/members/:id/combos` | Combo đã mua |

**POST /api/members:**
```json
// Request
{ "full_name": "Nguyễn Văn A", "phone": "0901234567", "id_card_number": "079201..." }
// Response
{ "id": "uuid", "code": "MEM-001", "balance": 0, "bonus_balance": 0, "tier": "Đồng" }
```

**POST /api/members/:id/topup:**
```json
// Request
{ "amount": 100000, "payment_method": "cash", "note": "..." }
// Response
{ "balance_before": 0, "balance_after": 100000, "transaction_id": "uuid" }
```

#### Member Tiers
| Method | Path | Description |
|---|---|---|
| GET | `/api/member-tiers` | List |
| POST | `/api/member-tiers` | Create |
| PUT | `/api/member-tiers/:id` | Update |
| DELETE | `/api/member-tiers/:id` | Delete |

#### Machines
| Method | Path | Description |
|---|---|---|
| GET | `/api/machines` | List (filterable by group_id, status, store_id) |
| POST | `/api/machines` | Create |
| GET | `/api/machines/:id` | Detail + current session + hardware |
| PUT | `/api/machines/:id` | Update |
| DELETE | `/api/machines/:id` | Soft delete |
| PUT | `/api/machines/:id/heartbeat` | Agent push heartbeat + hardware snapshot |
| GET | `/api/machines/:id/sessions` | Session history |
| GET | `/api/machines/:id/hardware` | Hardware snapshots (paginated) |
| GET | `/api/machines/:id/hardware/hourly` | Aggregated hourly |
| GET | `/api/machines/:id/hardware/alerts` | Alerts |
| PUT | `/api/machines/:id/hardware/alerts/:alertId/resolve` | Resolve alert |
| **Remote control** | | |
| POST | `/api/machines/:id/remote/shutdown` | Tắt máy |
| POST | `/api/machines/:id/remote/restart` | Khởi động lại |
| POST | `/api/machines/:id/remote/lock` | Khóa workstation |
| POST | `/api/machines/:id/remote/message` | Gửi popup thông báo |
| POST | `/api/machines/:id/remote/kill` | Kill process (theo name hoặc PID) |
| GET | `/api/machines/:id/remote/processes` | Danh sách process đang chạy |
| POST | `/api/machines/:id/remote/block-app` | Chặn/mở chặn ứng dụng |
| POST | `/api/machines/:id/remote/execute` | Chạy lệnh tùy ý |
| GET | `/api/machines/:id/remote/screenshot` | Chụp màn hình (base64) |
| WS | `/api/machines/:id/remote/stream` | Stream màn hình real-time (JPEG 1fps) |
| POST | `/api/machines/:id/remote/input` | Gửi mouse/keyboard input |

**PUT /api/machines/:id/heartbeat:**
```json
// Request (từ Agent, mỗi 60s)
{
  "cpu_temp": 65.2, "cpu_usage": 45.0,
  "gpu_temp": 72.1, "gpu_usage": 88.0,
  "ram_usage": 62.5,
  "disk_health": "good", "disk_usage": 55.0,
  "net_latency_ms": 2.3,
  "current_game": "Valorant",
  "top_processes": [{ "name": "VALORANT.exe", "cpu": 45, "ram": 1200 }]
}
```

#### Machine Groups
| Method | Path | Description |
|---|---|---|
| GET | `/api/machine-groups` | List |
| POST | `/api/machine-groups` | Create |
| PUT | `/api/machine-groups/:id` | Update |
| DELETE | `/api/machine-groups/:id` | Delete |

#### Machine Prices
| Method | Path | Description |
|---|---|---|
| GET | `/api/machine-prices` | List |
| POST | `/api/machine-prices` | Create |
| PUT | `/api/machine-prices/:id` | Update |
| DELETE | `/api/machine-prices/:id` | Delete |
| GET | `/api/time-based-pricings` | List |
| POST | `/api/time-based-pricings` | Create |
| PUT | `/api/time-based-pricings/:id` | Update |
| DELETE | `/api/time-based-pricings/:id` | Delete |

**GET /api/machine-prices:** `?machine_group_id=xxx&member_tier_id=yyy`

#### Machine Sessions
| Method | Path | Description |
|---|---|---|
| POST | `/api/sessions/start` | Start session (login máy) |
| GET | `/api/sessions/:id` | Detail |
| POST | `/api/sessions/:id/end` | End session (logout) |
| GET | `/api/sessions` | Active sessions (lọc by machine/member) |
| PUT | `/api/sessions/:id/change-machine` | Đổi máy |

**POST /api/sessions/start:**
```json
// Request
{
  "machine_id": "uuid",
  "member_id": "uuid | null",
  "combo_purchase_id": "uuid | null",
  "auth_method": "qr | pin | cccd"
}
// Response
{
  "session_id": "uuid",
  "combo_type": "fixed_slot | prepaid | null",
  "slot_end": "2026-06-24T12:00:00Z | null",
  "remaining_minutes": 300
}
```

**POST /api/sessions/:id/end:**
```json
// Request
{ "reason": "logout | timeout | balance_zero | curfew" }
// Response
{
  "duration_minutes": 65,
  "total_cost": 15000,
  "balance_deducted": 10000,
  "bonus_deducted": 5000,
  "remaining_minutes": 240
}
```

#### Machine Assets
| Method | Path | Description |
|---|---|---|
| GET | `/api/machines/:id/assets` | List |
| POST | `/api/machines/:id/assets` | Add asset |
| PUT | `/api/machines/:id/assets/:assetId` | Update |
| PUT | `/api/machines/:id/assets/:assetId/check` | Check-in (khi trả máy) |
| DELETE | `/api/machines/:id/assets/:assetId` | Soft delete |

**PUT /api/machines/:id/assets/:assetId/check:**
```json
// Request
{
  "status": "good | damaged | missing",
  "notes": "Bàn phím hư phím W",
  "check_photos": ["url1.jpg", "url2.jpg"]
}
```

#### Combos
| Method | Path | Description |
|---|---|---|
| GET | `/api/combos` | List (active, filter by type) |
| POST | `/api/combos` | Create |
| PUT | `/api/combos/:id` | Update |
| DELETE | `/api/combos/:id` | Soft delete |
| POST | `/api/combos/:id/purchase` | Member mua combo |
| GET | `/api/combos/purchases` | All purchases (admin) |
| GET | `/api/members/:id/combos` | Member's combos |
| POST | `/api/combos/purchases/:id/activate` | Activate (lần dùng đầu) |

**POST /api/combos (fixed_slot):**
```json
// Request
{
  "type": "fixed_slot",
  "name": "Sáng 7h-12h",
  "slot_start": "07:00", "slot_end": "12:00",
  "apply_days": [1,2,3,4,5,6,7],
  "validity_days": 30,
  "price": 500000,
  "items": [{ "item_type": "product", "item_id": "uuid", "quantity": 1 }]
}
```

**POST /api/combos (prepaid):**
```json
// Request
{
  "type": "prepaid",
  "name": "30h chơi",
  "total_minutes": 1800,
  "validity_days": 30,
  "price": 300000
}
```

**POST /api/combos/:id/purchase:**
```json
// Request
{ "member_id": "uuid", "payment_method": "cash | member_balance" }
// Response
{ "purchase_id": "uuid", "remaining_minutes": 1800, "expires_at": "..." }
```

**POST /api/combos/purchases/:id/activate:**
```json
// Request
{ "machine_id": "uuid" }
// Response
{ "session_id": "uuid", "combo_type": "fixed_slot", "slot_end": "..." }
```

#### Categories & Products
| Method | Path | Description |
|---|---|---|
| GET | `/api/categories` | Tree (nested) |
| POST | `/api/categories` | Create |
| PUT | `/api/categories/:id` | Update |
| DELETE | `/api/categories/:id` | Soft delete |
| GET | `/api/products` | List (filter by category_id) |
| POST | `/api/products` | Create |
| PUT | `/api/products/:id` | Update (incl. options, materials) |
| DELETE | `/api/products/:id` | Soft delete |
| POST | `/api/products/:id/options` | Add option |
| PUT | `/api/products/options/:id` | Update option |

#### Orders
| Method | Path | Description |
|---|---|---|
| POST | `/api/orders` | Create order |
| GET | `/api/orders` | List (filter by status, store, shift) |
| GET | `/api/orders/:id` | Detail + items + payments |
| PUT | `/api/orders/:id/status` | Update status (confirmed→preparing→ready→served) |
| PUT | `/api/orders/:id/items/:itemId/status` | Update item status |
| POST | `/api/orders/:id/split` | Split order |
| POST | `/api/orders/:id/pay` | Add payment to order |
| DELETE | `/api/orders/:id` | Cancel (soft) |

**POST /api/orders:**
```json
// Request
{
  "order_type": "machine_order | dine_in | takeaway",
  "member_id": "uuid | null",
  "machine_id": "uuid | null",
  "items": [
    { "product_id": "uuid", "quantity": 2, "options": [{ "group": "Kích cỡ", "option": "Lớn" }] }
  ],
  "note": "Ít đá"
}
// Response
{ "id": "uuid", "order_code": "ORD-20260624-001", "final_amount": 50000 }
```

**POST /api/orders/:id/pay (split payment):**
```json
// Request
{
  "payment_method": "member_balance | cash | qr_momo",
  "amount": 40000
}
// Mỗi lần gọi = 1 phần payment. Gọi nhiều lần cho tới khi final_amount = 0
// Response
{ "paid": 40000, "remaining": 10000 }
```

#### Payments
| Method | Path | Description |
|---|---|---|
| GET | `/api/orders/:id/payments` | List payments for order |
| POST | `/api/orders/:id/payments` | Add payment (alias of /pay) |
| POST | `/api/payments/:id/refund` | Refund payment |
| **Topup** | | |
| GET | `/api/settings/general/topup_presets` | Danh sách mệnh giá nạp (cho client) |
| POST | `/api/chat/topup-request` | Client gửi yêu cầu nạp |

#### Printer Configs
| Method | Path | Description |
|---|---|---|
| GET | `/api/printer-configs` | List |
| POST | `/api/printer-configs` | Add (ip, port, type) |
| PUT | `/api/printer-configs/:id` | Update |
| DELETE | `/api/printer-configs/:id` | Soft delete |
| POST | `/api/printer-configs/test` | Test print |

#### Inventory
| Method | Path | Description |
|---|---|---|
| GET | `/api/materials` | List (filter by warehouse, low stock) |
| POST | `/api/materials` | Create |
| PUT | `/api/materials/:id` | Update |
| DELETE | `/api/materials/:id` | Soft delete |
| POST | `/api/stock-transactions` | Nhập/xuất kho |
| GET | `/api/stock-transactions` | List (filter by material, type, date) |
| GET | `/api/materials/:id/transactions` | Stock history for 1 material |
| POST | `/api/inventory-counts` | Ghi nhận kiểm kho |
| POST | `/api/inventory-counts/chot-kho` | Chốt kho (cân bằng chênh lệch) |
| GET | `/api/suppliers` | List |
| POST | `/api/suppliers` | Create |
| PUT | `/api/suppliers/:id` | Update |
| GET | `/api/warehouses` | List |
| POST | `/api/warehouses` | Create |

**POST /api/stock-transactions:**
```json
// Request
{
  "material_id": "uuid",
  "transaction_type": "purchase | production_usage | adjustment_add | loss",
  "quantity": 10.5,
  "unit_price": 50000,
  "supplier_id": "uuid | null",
  "note": "Nhập lô hàng tháng 6"
}
// Response
{ "id": "uuid", "stock_before": 20, "stock_after": 30.5 }
```

#### Promotions
| Method | Path | Description |
|---|---|---|
| GET | `/api/promotions` | List |
| POST | `/api/promotions` | Create (incl. conditions & rewards) |
| PUT | `/api/promotions/:id` | Update |
| DELETE | `/api/promotions/:id` | Soft delete |
| GET | `/api/lucky-spin-rewards` | List |
| POST | `/api/lucky-spin-rewards` | Create |
| PUT | `/api/lucky-spin-rewards/:id` | Update |
| POST | `/api/lucky-spin/spin` | Member spin |
| GET | `/api/lucky-spin/history` | History (filter by member) |

**POST /api/lucky-spin/spin:**
```json
// Request
{ "member_id": "uuid" }
// Response
{ "is_win": true, "reward": { "type": "balance", "value": 20000 }, "bonus_added": 20000 }
```

#### Bookings
| Method | Path | Description |
|---|---|---|
| GET | `/api/bookings` | List (filter by status, date, machine) |
| POST | `/api/bookings` | Create |
| PUT | `/api/bookings/:id` | Update |
| POST | `/api/bookings/:id/cancel` | Cancel (refund deposit) |
| POST | `/api/bookings/:id/check-in` | Check-in (khách đến) |
| POST | `/api/bookings/:id/no-show` | Mark no-show (auto cron) |

**POST /api/bookings:**
```json
// Request
{
  "machine_id": "uuid",
  "member_id": "uuid | null",
  "customer_name": "Nguyễn Văn A",
  "customer_phone": "0901234567",
  "booked_from": "2026-06-24T19:00:00Z",
  "booked_to": "2026-06-24T22:00:00Z",
  "deposit_amount": 50000
}
// Response
{ "id": "uuid", "deposit_transaction_id": "uuid" }
```

#### Shifts
| Method | Path | Description |
|---|---|---|
| POST | `/api/shifts/open` | Mở ca |
| PUT | `/api/shifts/:id/close` | Đóng ca |
| GET | `/api/shifts` | List |
| GET | `/api/shifts/current` | Ca đang mở |
| POST | `/api/shifts/:id/cash-handover` | Ghi nhận bàn giao tiền |
| GET | `/api/shifts/:id/report` | Báo cáo ca |

**POST /api/shifts/open:**
```json
// Request
{ "user_id": "uuid", "opening_balance": 500000 }
```

**PUT /api/shifts/:id/close:**
```json
// Request
{ "closing_balance": 1500000, "notes": "..." }
// Response — nếu có discrepancy
{ "status": "closed", "expected": 1000000, "actual": 1500000, "discrepancy": 500000 }
```

#### Curfew
| Method | Path | Description |
|---|---|---|
| GET | `/api/curfew-policies` | List |
| POST | `/api/curfew-policies` | Create |
| PUT | `/api/curfew-policies/:id` | Update |
| DELETE | `/api/curfew-policies/:id` | Delete |
| POST | `/api/curfew-policies/:id/override` | Override (log audit) |

**POST /api/curfew-policies/:id/override:**
```json
// Request
{ "admin_id": "uuid", "reason": "Giải đấu đêm", "override_until": "2026-06-25T02:00:00Z" }
```

#### Website Blocking
| Method | Path | Description |
|---|---|---|
| GET | `/api/website-rules` | List (filter by category, is_active, rule_type) |
| POST | `/api/website-rules` | Create rule |
| PUT | `/api/website-rules/:id` | Update |
| DELETE | `/api/website-rules/:id` | Soft delete |
| GET | `/api/website-rules/active` | Rules active hiện tại (Agent sync) |
| POST | `/api/website-rule-mappings` | Assign rule to machine_group |
| DELETE | `/api/website-rule-mappings/:id` | Remove mapping |
| POST | `/api/website-schedules` | Create schedule for rule (day_of_week, start/end) |
| PUT | `/api/website-schedules/:id` | Update |
| DELETE | `/api/website-schedules/:id` | Delete |
| GET | `/api/website-violations` | List (filter by machine_id, domain, date) |
| POST | `/api/website-violations` | Agent report violation |
| PUT | `/api/settings/website-blocking/mode` | Set mode: 'blacklist' | 'whitelist' |

**POST /api/website-rules:**
```json
// Request
{
  "pattern": "*.facebook.com",
  "rule_type": "block",
  "category": "social",
  "description": "Chặn mạng xã hội giờ học"
}
```

**POST /api/website-schedules:**
```json
// Request
{
  "rule_id": "uuid",
  "day_of_week": [1,2,3,4,5],
  "start_time": "08:00",
  "end_time": "17:00"
}
```

#### E-Invoice
| Method | Path | Description |
|---|---|---|
| GET | `/api/einvoice-configs` | List |
| POST | `/api/einvoice-configs` | Create |
| PUT | `/api/einvoice-configs/:id` | Update |
| POST | `/api/einvoices/issue` | Issue invoice for order |
| GET | `/api/einvoices` | List (filter by order, date) |

#### System Settings
| Method | Path | Description |
|---|---|---|
| GET | `/api/settings` | All settings (grouped) |
| GET | `/api/settings/:group` | By group (billing, general...) |
| PUT | `/api/settings/:group/:key` | Update setting value |

#### Reports
| Method | Path | Description |
|---|---|---|
| GET | `/api/reports/daily-revenue` | Doanh thu theo ngày |
| GET | `/api/reports/monthly-revenue` | Doanh thu theo tháng |
| GET | `/api/reports/by-member` | Doanh thu theo hội viên |
| GET | `/api/reports/by-machine` | Doanh thu theo máy |
| GET | `/api/reports/by-employee` | Doanh thu theo nhân viên |
| GET | `/api/reports/top-products` | Top sản phẩm bán chạy |
| GET | `/api/reports/inventory` | Báo cáo tồn kho |
| GET | `/api/reports/promotion-usage` | Báo cáo KM đã dùng |

**GET /api/reports/daily-revenue:** `?from=2026-06-01&to=2026-06-24&store_id=uuid`
```json
// Response
{
  "total_revenue": 5000000,
  "total_bonus_used": 500000,
  "by_method": { "cash": 3000000, "member_balance": 2000000 },
  "daily": [
    { "date": "2026-06-24", "revenue": 1500000, "orders": 45, "avg_order": 33333 }
  ]
}
```

#### Audit Logs
| Method | Path | Description |
|---|---|---|
| GET | `/api/audit-logs` | List (filter by action, entity, user, date) |

#### Stores
| Method | Path | Description |
|---|---|---|
| GET | `/api/stores` | List |
| POST | `/api/stores` | Create |
| PUT | `/api/stores/:id` | Update |
| DELETE | `/api/stores/:id` | Soft delete |

#### Backups
| Method | Path | Description |
|---|---|---|
| POST | `/api/backups` | Create backup |
| GET | `/api/backups` | List |
| POST | `/api/backups/:id/restore` | Restore from backup |

