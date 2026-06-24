# Business Logic Flows

## A. Machine Session Lifecycle

```
1. Member QR/PIN login tại máy trạm
   Server check:
   ├─ Có combo hợp lệ?
   │   ├─ fixed_slot: check giờ ∈ [slot_start, slot_end) + apply_days + expires_at
   │   │   → tạo session, slot_end = hôm nay lúc slot_end, không timer
   │   └─ prepaid: check remaining_minutes > 0 + expires_at
   │       → tạo session, current_session_id set, timer đếm ngược
   └─ Không combo → tính theo machine_prices + time_based_pricings, server timer

2. Trong khi chơi:
   ├─ Mỗi 60s: server tính tiền = rounded_hours * price_per_hour
   ├─ Check balance + bonus_balance >= current_cost
   └─ Nếu tổng = 0 → end session (force logout)

3. Logout / End session:
   ├─ combo (prepaid): remaining_minutes -= duration, current_session_id = NULL, total_cost = 0
   ├─ combo (fixed_slot): ended_at = min(now, slot_end), total_cost = 0
   └─ normal (member): balance -= total_cost, bonus_balance ưu tiên trừ trước
```

## B. Pricing Engine

```
0. IF session.combo_type IS NOT NULL:
     // Combo member — không tính tiền, bỏ qua balance
     total_cost = 0
     IF combo_type = 'fixed_slot':
       ended_at = slot_end (hoặc now nếu logout sớm)
       RETURN
     IF combo_type = 'prepaid':
       remaining_minutes -= duration
       RETURN

1. Xác định giá cơ bản:
   machine_prices WHERE machine_group_id = ? AND (member_tier_id = ? OR member_tier_id IS NULL)
   → Ưu tiên row có member_tier_id cụ thể, fallback row NULL

2. Override theo giờ:
   time_based_pricings WHERE machine_group_id = ? AND day_of_week = ? AND start_time <= now < end_time
   → Nếu khớp, dùng price_per_hour này thay vì machine_prices

3. Grace period:
   IF duration_minutes <= grace_period_minutes → total_cost = 0

4. Rounding:
   rounded_hours = CEIL(duration_minutes / interval_minutes) * (interval_minutes / 60)

5. Discount:
   base = rounded_hours * price_per_hour
   tier_discount = base * member_tier.discount_percent / 100
   promotion_discount = kiểm tra promotions type='time_discount' + condition match
   total = base - tier_discount - promotion_discount

6. Payment priority:
   IF bonus_balance >= total:
      bonus_balance -= total → ghi bonus_before/bonus_after
   ELSE:
      remaining = total - bonus_balance
      bonus_balance = 0
      balance -= remaining → ghi balance_before/balance_after

7. Ghi member_transactions: transaction_type = 'game_time', amount = total
```

## C. Combo Activation & Change Machine

```
fixed_slot:
  Mua → combo_purchases (activated=false)
  Login lần đầu → check slot/ngày/hạn OK → activated=true → tạo session
  Chuyển máy: trong khung giờ, login máy khác → gán session cho máy mới
  Hết khung giờ: auto end session (ended_at = slot_end)
  Product kèm: tự động tạo order (giá 0đ) khi activate lần đầu

prepaid:
  Mua → combo_purchases (activated=false, remaining_minutes=total_minutes)
  Login lần đầu → activated=true, current_session_id set
  Logout → remaining_minutes trừ, current_session_id = NULL
  Login máy khác → current_session_id mới, timer tiếp từ remaining
  Hết remaining hoặc expires_at → không dùng được
  Product kèm: tự động tạo order (giá 0đ) khi activate lần đầu

Anti-cheat:
  1 combo chỉ có 1 current_session_id active tại 1 thời điểm
  Nếu login máy khác mà current_session_id còn active → force end session cũ
```

## D. Shift Close

```
1. Kiểm tra tất cả orders trong ca đã 'completed'? Nếu chưa → cảnh báo
2. expected_total = SUM(payments.amount) WHERE shift_id = ?
3. Nhân viên nhập closing_balance (tiền mặt thực tế)
4. discrepancy = closing_balance - expected_total
   Ghi cash_handovers: handover_type = 'cash_in' | 'cash_out'
5. status = 'closed', không cho sửa orders trong ca đã đóng
```

## E. Member Tier Auto Update

```
Trigger: sau mỗi member_transactions (topup, game_time, combo_purchase...)

1. Tính total_spent mới: SELECT SUM(amount) FROM member_transactions
   WHERE member_id = ? AND balance_before != balance_after
2. SELECT id FROM member_tiers WHERE min_spent <= total_spent
   ORDER BY min_spent DESC LIMIT 1
3. IF tier_id != current member.tier_id:
   ├─ UPDATE members SET tier_id = new_tier_id
   ├─ INSERT notification: "Chúc mừng bạn lên hạng {tier_name}!"
   └─ INSERT member_transactions: promotion_bonus (nếu tier mới có bonus)
```

## F. Refund Flow

```
Refund tiền thật (từ balance):
  POST /api/members/:id/refund
  { "amount": 50000, "original_transaction_id": "uuid", "reason": "..." }
  → balance += amount, member_transactions: transaction_type='refund'

Refund tiền KM (từ bonus_balance):
  → bonus_balance += amount
  → member_transactions: bonus_before/bonus_after, không ảnh hưởng balance

Refund giao dịch hỗn hợp:
  amount = 100k (80k main + 20k bonus)
  → hoàn 80k vào balance, 20k vào bonus_balance
```

## G. Booking & Deposit

```
Create booking:
  deposit_amount > 0 → tạo member_transactions (trừ balance)
  → deposit_transaction_id = transaction_id, status = 'pending'

Check-in (khách đến):
  status = 'checked_in' → deposit giữ nguyên

No-show (auto cron, sau cancel_at):
  status = 'no_show' → deposit không hoàn (ghi doanh thu)

Cancel (khách hủy trước):
  status = 'cancelled' → hoàn deposit (nếu có), ghi refund transaction
```

## H. Topup Flow (Client → Admin)

```
Client (máy trạm)                Server                      Admin Web
   │                               │                            │
   ├─ Click nút 💳 Nạp tiền        │                            │
   ├─ Chọn mệnh giá (VD: 100k)    │                            │
   ├─ [Gửi yêu cầu] ────────────► │                            │
   │                               ├─ Tạo chat message:        │
   │                               │  "M03 - A yêu cầu nạp 100k"│
   │                               ├─ Push notification ──────►│
   │                               │                   ┌───────┴────┐
   │                               │                   │ Admin thấy │
   │                               │                   │ notification│
   │                               │                   │ → [Xác nhận]│
   │                               │◄──────────────────│ hoặc sửa số │
   │                               │                   │ tiền thực tế│
   │                               ├─ Validate amount > 0       │
   │                               ├─ BEGIN TX                  │
   │                               │  UPDATE members SET balance += amount
   │                               │  INSERT member_transactions (topup)
   │                               ├─ COMMIT                    │
   │                               ├─ Chat: "✅ Đã nạp 100k"   │
   │◄──── chat message ────────────┤                            │
   │ "✅ Đã nạp 100k. Số dư: 200k"│                            │
```

## I. Order Fulfillment (Kitchen/Bar)

```
1. Order created → status = 'pending'
2. FOR EACH order_item:
     ├─ Tìm printer: product_printer_mapping.product_id
     │   └─ Nếu không có → fallback categories.printer_id
     ├─ Gửi print job tới printer (raw TCP: IP:9100)
     └─ order_item.status = 'confirmed'
3. Kitchen/Bar nhận được print → bắt đầu làm
4. Staff click "Đang làm" → PUT status = 'preparing'
5. Staff click "Hoàn thành" → PUT status = 'ready'
6. Staff mang ra bàn/máy → PUT status = 'served'
7. IF tất cả items = 'served' → order.status = 'completed'

Printer routing priority:
  product_printer_mapping (specific product) ▶
  categories.printer_id (category default) ▶
  printer_configs WHERE is_default = true (global default)
```

## J. Inventory Auto-Deduction

```
Trigger: order_item.status → 'served'

1. SELECT pm.material_id, pm.quantity, pm.unit_id
   FROM product_materials pm
   WHERE pm.product_id = ?

2. FOR EACH material:
     BEGIN TX
       SELECT current_stock FROM materials WHERE id = ? FOR UPDATE
       new_stock = current_stock - (quantity * order_item.qty)
       IF new_stock < 0 → cảnh báo (cho phép âm hoặc từ chối tùy config)
       UPDATE materials SET current_stock = new_stock
       INSERT stock_transactions (
         material_id, transaction_type='production_usage',
         quantity = -(quantity * order_item.qty),
         stock_before, stock_after,
         reference_id = order_id
       )
     COMMIT
     IF new_stock < min_stock → INSERT notification "Vật liệu X sắp hết"

3. Nếu product không có product_materials mapping → skip (không track inventory)
```

## K. Gift Card Lifecycle

```
1. Admin tạo gift card:
   INSERT gift_cards (code, initial_balance, expires_at)

2. Bán gift card cho member:
   BEGIN TX
     member.balance -= price (hoặc cash)
     gift_card.status = 'active'
     INSERT member_transactions (type='gift_card', amount=-price)
     INSERT gift_card_transactions (gift_card_id, amount=initial_balance)
   COMMIT

3. Sử dụng gift card để thanh toán order:
   POST /api/orders/:id/pay { payment_method: 'gift_card', gift_card_id }
   BEGIN TX
     SELECT balance FROM gift_cards WHERE id = ? FOR UPDATE
     IF balance < amount → error
     UPDATE gift_cards SET balance -= amount
     IF balance = 0 → gift_cards.status = 'used'
     INSERT gift_card_transactions (amount, balance_before, balance_after, order_id)
     INSERT payments (payment_method='gift_card', amount)
   COMMIT

4. Kiểm tra hạn (cron daily):
   UPDATE gift_cards SET status = 'expired' WHERE expires_at < now() AND status = 'active'
```

## L. Lucky Spin

```
1. Member click spin:
   GET /api/lucky-spin/history?member_id=X&date=today
   Count >= max_per_day (từ lucky_spin_rewards) → từ chối

2. Random reward:
   SELECT * FROM lucky_spin_rewards WHERE is_active = true
   Random 0.0 - 1.0
   Duyệt rewards theo probability, cumulative sum
   → Xác định reward trúng (hoặc không trúng)

3. Apply reward:
   INSERT lucky_spins (member_id, reward_id? null nếu lose, is_win)

   IF win:
     CASE reward_type:
       'balance'  → bonus_balance += reward_value.amount
                      INSERT member_transactions (promotion_bonus)
       'product'  → Tự động tạo order (giá 0, ghi chú "Quà lucky spin")
       'time'     → INSERT combo_purchases (prepaid với total_minutes, giá 0)
       'voucher'  → INSERT gift_cards (code, balance, ghi chú "KM lucky spin")

4. Response cho client:
   { is_win: true/false, reward: { type, name, value } }
```

## M. Curfew Enforcement

```
1. Cron chạy mỗi 1 phút:
   SELECT ms.*, m.date_of_birth FROM machine_sessions ms
   JOIN members m ON ms.member_id = m.id
   WHERE ms.is_active = true AND m.date_of_birth IS NOT NULL

2. FOR EACH active session của minor (age < 18):
     cp = SELECT * FROM curfew_policies
          WHERE day_of_week = EXTRACT(DOW FROM now())
          AND curfew_start <= now::time < curfew_end
          AND is_active = true
     IF cp found:
       // Kiểm tra override
       IF cp.override_at IS NOT NULL AND cp.override_at > now
          → skip (đã được admin override cho giải đấu)
       ELSE:
          → Force end session (POST /api/sessions/:id/end { reason: 'curfew' })
          → Agent lock machine
          → INSERT audit_log: { action: 'curfew.enforce', entity_id: session_id }
          → INSERT notification: "Máy {code} bị khóa do giờ giới nghiêm"

3. Override (admin cho phép chơi qua giờ):
   POST /api/curfew-policies/:id/override
   { admin_id, reason: "Giải đấu đêm", override_until: "2026-06-25T02:00" }
   → UPDATE curfew_policies SET override_by_admin, override_reason, override_at
   → INSERT audit_log
```

## N. Promotion Application

```
Khi tính tiền (session end hoặc order create):

1. Lấy danh sách promotions active
   SELECT * FROM promotions
   WHERE is_active = true
   AND (valid_from IS NULL OR valid_from <= now)
   AND (valid_to IS NULL OR valid_to >= now)
   ORDER BY priority DESC

2. FOR EACH promotion:
     conditions_match = true
     FOR EACH promotion_condition:
       CASE condition_key:
         'min_amount'   → total >= condition_value.min_amount
         'member_tier'  → member.tier_id IN condition_value.tier_ids
         'day_of_week'  → EXTRACT(DOW) IN condition_value.days
         'hour_range'   → now::time BETWEEN start AND end
         'machine_group'→ session.machine_group_id IN condition_value.group_ids
         ELSE → false
       IF NOT match → conditions_match = false, break

     IF conditions_match:
       FOR EACH promotion_reward:
         CASE reward_type:
           'discount_percent' → discount = total * reward_value.percent / 100
           'discount_fixed'   → discount = reward_value.amount
           'bonus_balance'    → bonus_balance += reward_value.amount
                                INSERT member_transactions (promotion_bonus)
           'free_product'     → Tạo order_item với giá 0

3. Tổng hợp discount:
   total_discount = SUM(all_matched_discounts)
   final_amount = total - total_discount
   Ghi vào orders.discount_amount + orders.final_amount

4. Priority stack rules (config):
   - 'stackable': multiple discounts cộng dồn
   - 'best_only': chỉ lấy discount cao nhất
```

## O. Multi-store Operations (Phase 2)

```
1. Member cross-store:
   Member tạo ở store A → balance/combos dùng được ở store B
   Vì store_id trong members chỉ là "store đăng ký", không phải "store giới hạn"
   → Query: lọc theo store_id khi cần báo cáo, không giới hạn login

2. Pricing/Combo cross-store:
   combo_purchases.expires_at + remaining_minutes dùng được mọi store
   machine_prices + time_based_pricings theo từng store riêng

3. Reporting:
   Doanh thu store A: WHERE store_id = 'A'
   Doanh thu toàn hệ thống: không filter store_id

4. Agent quản lý theo store:
   machines.store_id → Agent chỉ sync với store của nó
   website_rules, printer_configs... có thể có store_id riêng

5. Admin:
   Owner role → thấy tất cả stores
   Manager role → chỉ thấy store được gán (user.store_id)
```
