## Thiết kế Database

### Quy tắc chung

- **Soft delete**: Tất cả bảng chính có `deleted_at TIMESTAMPTZ`. Query mặc định filter `WHERE deleted_at IS NULL`.
- **store_id**: Thêm từ Phase 1 để tránh migrate sau này.
- **Index**: Tất cả trường foreign key và trường hay query đều có index.
- **Kiểu dữ liệu**: Dùng `BIGINT` cho tiền (đơn vị VNĐ, lưu số nguyên, tránh float).
- **Timestamps**: Dùng `TIMESTAMPTZ` (timezone-aware).

### ERD Overview

```
users ──┬── user_roles ─── roles ─── role_permissions ──── permissions
         │
         ├── shifts ──── cash_handovers
         ├── members ──┬── member_transactions
         │              ├── member_attendance
         │              ├── machine_sessions
         │              ├── orders
         │              ├── combo_purchases
         │              ├── lucky_spins
         │              └── member_tiers
         │
         ├── machine_bookings
         └── chat_participants

machines ──┬── machine_groups ─── machine_prices
           ├── machine_assets
           ├── machine_sessions
           ├── orders
           ├── service_feedback
           └── chat_participants

categories ──── products ──┬── product_options
                            ├── product_materials
                            ├── product_printer_mapping
                            └── order_items

orders ──── order_items ──── payments

materials ──── stock_transactions ──── suppliers / warehouses

chat_conversations ──── chat_participants ──── chat_messages

promotions ──── promotion_conditions ──── promotion_rewards

stores ──── machines / members / users / printer_configs
```

### CREATE TABLE scripts

#### stores (multi-store)

```sql
CREATE TABLE stores (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name      VARCHAR(200) NOT NULL,
    code      VARCHAR(20) UNIQUE,
    address   TEXT,
    phone     VARCHAR(20),
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now(),
    deleted_at TIMESTAMPTZ
);
```

#### users & auth

```sql
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username      VARCHAR(50) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    full_name     VARCHAR(100),
    email         VARCHAR(100),
    phone         VARCHAR(20),
    avatar_url    TEXT,
    store_id      UUID REFERENCES stores(id),
    is_active     BOOLEAN DEFAULT true,
    last_login_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ DEFAULT now(),
    updated_at    TIMESTAMPTZ DEFAULT now(),
    deleted_at    TIMESTAMPTZ
);

CREATE TABLE roles (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(50) NOT NULL UNIQUE,
    description TEXT,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TYPE permission_module AS ENUM ('members', 'machines', 'orders', 'reports', 'settings', 'client');

CREATE TABLE permissions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code        VARCHAR(100) NOT NULL UNIQUE,
    name        VARCHAR(100),
    module      permission_module,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id),
    role_id UUID NOT NULL REFERENCES roles(id),
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE role_permissions (
    role_id       UUID NOT NULL REFERENCES roles(id),
    permission_id UUID NOT NULL REFERENCES permissions(id),
    PRIMARY KEY (role_id, permission_id)
);
```

#### members

```sql
CREATE TABLE member_tiers (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name              VARCHAR(50) NOT NULL,
    min_spent         BIGINT DEFAULT 0,
    discount_percent  DECIMAL(5,2) DEFAULT 0,
    created_at        TIMESTAMPTZ DEFAULT now()
);

CREATE TYPE member_role AS ENUM ('member', 'combo', 'admin');

CREATE TABLE members (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code               VARCHAR(20) UNIQUE,
    full_name          VARCHAR(100) NOT NULL,
    phone              VARCHAR(20),
    email              VARCHAR(100),
    password_hash      VARCHAR(255),           -- NULL với combo member (chỉ QR)
    role               member_role DEFAULT 'member',
    id_card_number     VARCHAR(20),
    id_card_image_url  TEXT,
    avatar_url         TEXT,
    date_of_birth      DATE,
    balance            BIGINT DEFAULT 0,       -- tiền thật (tính doanh thu)
    bonus_balance      BIGINT DEFAULT 0,       -- tiền khuyến mãi (không tính doanh thu)
    total_spent        BIGINT DEFAULT 0,       -- chỉ tính từ balance chính
    total_played_hours INTEGER DEFAULT 0,
    tier_id            UUID REFERENCES member_tiers(id),
    store_id           UUID REFERENCES stores(id),
    notes              TEXT,
    parent_consent_file_url TEXT,
    is_active          BOOLEAN DEFAULT true,
    last_visit_at      TIMESTAMPTZ,
    created_at         TIMESTAMPTZ DEFAULT now(),
    updated_at         TIMESTAMPTZ DEFAULT now(),
    deleted_at         TIMESTAMPTZ
);

CREATE INDEX idx_members_phone ON members(phone) WHERE deleted_at IS NULL;
CREATE INDEX idx_members_code ON members(code) WHERE deleted_at IS NULL;
CREATE INDEX idx_members_store ON members(store_id);
CREATE INDEX idx_members_tier ON members(tier_id);

CREATE TYPE transaction_type AS ENUM (
    'topup', 'game_time', 'service_order', 'refund',
    'gift_card', 'combo_purchase', 'promotion_bonus'
);

CREATE TYPE payment_method AS ENUM (
    'cash', 'qr_momo', 'qr_bank', 'bank_transfer',
    'member_balance', 'gift_card'
);

CREATE TYPE payment_status AS ENUM ('pending', 'completed', 'failed', 'refunded');

CREATE TABLE member_transactions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    member_id        UUID NOT NULL REFERENCES members(id),
    transaction_type transaction_type NOT NULL,
    amount           BIGINT NOT NULL,
    balance_before   BIGINT NOT NULL,       -- main balance trước
    balance_after    BIGINT NOT NULL,        -- main balance sau
    bonus_before     BIGINT DEFAULT 0,       -- bonus balance trước
    bonus_after      BIGINT DEFAULT 0,       -- bonus balance sau
    payment_method   payment_method,
    reference_id     UUID,
    description      TEXT,
    store_id         UUID REFERENCES stores(id),
    created_by       UUID REFERENCES users(id),
    created_at       TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_member_tx_member ON member_transactions(member_id);
CREATE INDEX idx_member_tx_date ON member_transactions(created_at);
CREATE INDEX idx_member_tx_store ON member_transactions(store_id);

CREATE TABLE member_attendance (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    member_id      UUID NOT NULL REFERENCES members(id),
    checkin_at     TIMESTAMPTZ DEFAULT now(),
    reward_claimed BOOLEAN DEFAULT false,
    store_id       UUID REFERENCES stores(id)
);
```

#### machines & assets

```sql
CREATE TYPE machine_status AS ENUM ('offline', 'available', 'in_use', 'maintenance');
CREATE TYPE asset_type AS ENUM ('mouse', 'keyboard', 'headset', 'chair', 'monitor');
CREATE TYPE asset_status AS ENUM ('good', 'damaged', 'missing', 'repaired');
CREATE TYPE session_combo_type AS ENUM ('fixed_slot', 'prepaid');

CREATE TABLE machine_groups (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(50) NOT NULL,
    description TEXT,
    color       VARCHAR(7),
    store_id    UUID REFERENCES stores(id),
    sort_order  INTEGER DEFAULT 0,
    created_at  TIMESTAMPTZ DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE TABLE machines (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_code    VARCHAR(20) NOT NULL UNIQUE,
    group_id        UUID REFERENCES machine_groups(id),
    store_id        UUID REFERENCES stores(id),
    status          machine_status DEFAULT 'offline',
    cpu_name        VARCHAR(100),
    ram_gb          INTEGER,
    gpu_name        VARCHAR(100),
    storage_gb      INTEGER,
    ip_address      VARCHAR(45),
    mac_address     VARCHAR(17),
    os_info         VARCHAR(100),
    cpu_temp        DECIMAL(5,1),
    gpu_temp        DECIMAL(5,1),
    last_heartbeat  TIMESTAMPTZ,
    is_active       BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_machines_group ON machines(group_id);
CREATE INDEX idx_machines_status ON machines(status);
CREATE INDEX idx_machines_store ON machines(store_id);

CREATE TABLE machine_assets (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id  UUID NOT NULL REFERENCES machines(id),
    asset_type  asset_type NOT NULL,
    brand       VARCHAR(100),
    model       VARCHAR(100),
    serial      VARCHAR(100),
    status      asset_status DEFAULT 'good',
    notes       TEXT,
    checked_by  UUID REFERENCES users(id),
    checked_at  TIMESTAMPTZ,
    check_photos JSONB[],  -- ["url1.jpg", "url2.jpg", ...]
    created_at  TIMESTAMPTZ DEFAULT now(),
    updated_at  TIMESTAMPTZ DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX idx_machine_assets_machine ON machine_assets(machine_id);

CREATE TABLE machine_prices (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_group_id UUID REFERENCES machine_groups(id),
    member_tier_id   UUID REFERENCES member_tiers(id),
    price_per_hour   BIGINT NOT NULL,
    min_duration     INTEGER DEFAULT 1,
    effective_from   DATE NOT NULL,
    effective_to     DATE,
    created_at       TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE time_based_pricings (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_group_id UUID REFERENCES machine_groups(id),
    day_of_week      INTEGER,
    start_time       TIME NOT NULL,
    end_time         TIME NOT NULL,
    price_per_hour   BIGINT NOT NULL,
    is_active        BOOLEAN DEFAULT true,
    created_at       TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE machine_sessions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id       UUID NOT NULL REFERENCES machines(id),
    member_id        UUID REFERENCES members(id),
    combo_type       session_combo_type,     -- NULL = tính tiền giờ thường
    combo_id         UUID REFERENCES combos(id),
    slot_end         TIMESTAMPTZ,            -- fixed_slot: auto end tại đây, không cần timer
    remaining_minutes INTEGER,               -- prepaid: remaining sau session này
    started_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at         TIMESTAMPTZ,
    duration_minutes INTEGER,
    total_cost       BIGINT,
    is_overnight     BOOLEAN DEFAULT false,
    store_id         UUID REFERENCES stores(id),
    is_active        BOOLEAN DEFAULT true,
    created_at       TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_machine_sessions_machine ON machine_sessions(machine_id);
CREATE INDEX idx_machine_sessions_member ON machine_sessions(member_id);
CREATE INDEX idx_machine_sessions_active ON machine_sessions(is_active) WHERE is_active = true;
```

#### billing & combos

```sql
CREATE TYPE combo_type AS ENUM ('fixed_slot', 'prepaid');
CREATE TYPE combo_item_type AS ENUM ('time', 'product');
CREATE TYPE topup_card_status AS ENUM ('active', 'sold', 'expired');
CREATE TYPE gift_card_status AS ENUM ('active', 'used', 'expired');

CREATE TABLE combos (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name             VARCHAR(100) NOT NULL,
    description      TEXT,
    type             combo_type NOT NULL,
    slot_start       TIME,                   -- fixed_slot: 07:00
    slot_end         TIME,                   -- fixed_slot: 12:00
    apply_days       INTEGER[],              -- fixed_slot: [1,2,3,4,5,6,7]
    total_minutes    INTEGER,                -- prepaid: tổng số phút
    validity_days    INTEGER,                -- số ngày hiệu lực (cả 2 loại)
    price            BIGINT NOT NULL,
    member_prefix    VARCHAR(20),               -- auto-gen từ name: "Sáng chiều" → "SANGCHIEU"
    member_count     INTEGER DEFAULT 0,         -- counter tăng dần theo prefix
    is_active        BOOLEAN DEFAULT true,
    created_at       TIMESTAMPTZ DEFAULT now(),
    deleted_at       TIMESTAMPTZ
);

CREATE TABLE combo_items (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    combo_id    UUID NOT NULL REFERENCES combos(id),
    item_type   combo_item_type NOT NULL,
    item_id     UUID,                    -- NULL nếu 'time', product_id nếu 'product'
    quantity    INTEGER DEFAULT 1,       -- product: số lượng; time: NULL
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE combo_purchases (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    combo_id            UUID NOT NULL REFERENCES combos(id),
    member_id           UUID NOT NULL REFERENCES members(id),
    price               BIGINT NOT NULL,
    payment_method      payment_method,
    activated           BOOLEAN DEFAULT false,
    activated_at        TIMESTAMPTZ,
    current_session_id  UUID REFERENCES machine_sessions(id),
    remaining_minutes   INTEGER,            -- prepaid: còn lại bao nhiêu phút
    expires_at          TIMESTAMPTZ,
    created_at          TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE topup_cards (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code        VARCHAR(50) NOT NULL UNIQUE,
    pin         VARCHAR(20),
    face_value  BIGINT NOT NULL,
    bonus_value BIGINT DEFAULT 0,
    status      topup_card_status DEFAULT 'active',
    sold_to     UUID REFERENCES members(id),
    sold_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE TABLE gift_cards (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code            VARCHAR(50) NOT NULL UNIQUE,
    balance         BIGINT DEFAULT 0,
    initial_balance BIGINT NOT NULL,
    status          gift_card_status DEFAULT 'active',
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE TABLE gift_card_transactions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    gift_card_id   UUID NOT NULL REFERENCES gift_cards(id),
    amount         BIGINT NOT NULL,
    balance_before BIGINT,
    balance_after  BIGINT,
    order_id       UUID REFERENCES orders(id),
    created_at     TIMESTAMPTZ DEFAULT now()
);
```

#### products & orders

```sql
CREATE TABLE categories (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(100) NOT NULL,
    parent_id  UUID REFERENCES categories(id),
    icon       VARCHAR(50),
    printer_id UUID REFERENCES printer_configs(id),
    sort_order INTEGER DEFAULT 0,
    is_active  BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE products (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id UUID NOT NULL REFERENCES categories(id),
    name        VARCHAR(200) NOT NULL,
    description TEXT,
    price       BIGINT NOT NULL,
    image_url   TEXT,
    is_active   BOOLEAN DEFAULT true,
    has_stock   BOOLEAN DEFAULT false,
    sort_order  INTEGER DEFAULT 0,
    created_at  TIMESTAMPTZ DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX idx_products_category ON products(category_id);

CREATE TABLE product_option_groups (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(100) NOT NULL,
    is_required BOOLEAN DEFAULT true,
    max_select  INTEGER DEFAULT 1,
    sort_order  INTEGER DEFAULT 0
);

CREATE TABLE product_options (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id     UUID NOT NULL REFERENCES product_option_groups(id),
    product_id   UUID NOT NULL REFERENCES products(id),
    name         VARCHAR(100) NOT NULL,
    price_adjust BIGINT DEFAULT 0,
    sort_order   INTEGER DEFAULT 0
);

CREATE TYPE order_status AS ENUM (
    'pending', 'confirmed', 'preparing', 'ready',
    'served', 'completed', 'cancelled', 'refunded'
);

CREATE TYPE order_type AS ENUM (
    'dine_in', 'takeaway', 'machine_order'
);

CREATE TABLE orders (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_code      VARCHAR(20) NOT NULL UNIQUE,
    order_type      order_type NOT NULL,
    status          order_status DEFAULT 'pending',
    member_id       UUID REFERENCES members(id),
    machine_id      UUID REFERENCES machines(id),
    store_id        UUID REFERENCES stores(id),
    table_number    VARCHAR(10),
    total_amount    BIGINT NOT NULL,
    discount_amount BIGINT DEFAULT 0,
    final_amount    BIGINT NOT NULL,
    note            TEXT,
    created_by      UUID REFERENCES users(id),
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_orders_member ON orders(member_id);
CREATE INDEX idx_orders_machine ON orders(machine_id);
CREATE INDEX idx_orders_store ON orders(store_id);
CREATE INDEX idx_orders_created ON orders(created_at);
CREATE INDEX idx_orders_status ON orders(status);

CREATE TABLE order_items (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id     UUID NOT NULL REFERENCES orders(id),
    product_id   UUID NOT NULL REFERENCES products(id),
    product_name VARCHAR(200) NOT NULL,
    quantity     INTEGER NOT NULL DEFAULT 1,
    unit_price   BIGINT NOT NULL,
    options      JSONB,
    subtotal     BIGINT NOT NULL,
    store_id     UUID REFERENCES stores(id),
    status       order_status DEFAULT 'pending',
    note         TEXT,
    created_at   TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_order_items_order ON order_items(order_id);

-- Split payment: 1 order có thể có nhiều payments
CREATE TABLE payments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id        UUID NOT NULL REFERENCES orders(id),
    payment_method  payment_method NOT NULL,
    amount          BIGINT NOT NULL,
    reference_code  VARCHAR(100),
    status          payment_status DEFAULT 'pending',
    paid_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_payments_order ON payments(order_id);
```

#### printer routing

```sql
CREATE TYPE printer_type AS ENUM ('receipt', 'kitchen', 'bar');

CREATE TABLE printer_configs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         VARCHAR(100) NOT NULL,
    printer_type printer_type NOT NULL,
    ip_address   VARCHAR(45),
    port         INTEGER DEFAULT 9100,
    is_default   BOOLEAN DEFAULT false,
    store_id     UUID REFERENCES stores(id),
    created_at   TIMESTAMPTZ DEFAULT now(),
    deleted_at   TIMESTAMPTZ
);

CREATE TABLE product_printer_mapping (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id  UUID NOT NULL REFERENCES products(id),
    printer_id  UUID NOT NULL REFERENCES printer_configs(id),
    created_at  TIMESTAMPTZ DEFAULT now()
);
```

#### inventory

```sql
CREATE TABLE units (
    id   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) NOT NULL
);

CREATE TABLE suppliers (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name      VARCHAR(200) NOT NULL,
    phone     VARCHAR(20),
    email     VARCHAR(100),
    address   TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE materials (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name           VARCHAR(200) NOT NULL,
    unit_id        UUID NOT NULL REFERENCES units(id),
    current_stock  DECIMAL(12,3) DEFAULT 0,
    min_stock      DECIMAL(12,3) DEFAULT 0,
    price_per_unit BIGINT,
    is_active      BOOLEAN DEFAULT true,
    created_at     TIMESTAMPTZ DEFAULT now(),
    deleted_at     TIMESTAMPTZ
);

CREATE TABLE product_materials (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id  UUID NOT NULL REFERENCES products(id),
    material_id UUID NOT NULL REFERENCES materials(id),
    quantity    DECIMAL(12,3) NOT NULL,
    unit_id     UUID NOT NULL REFERENCES units(id),
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TYPE stock_tx_type AS ENUM (
    'purchase', 'return', 'transfer_in', 'transfer_out',
    'production_usage', 'production_return',
    'adjustment_add', 'adjustment_subtract', 'loss'
);

CREATE TABLE stock_transactions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    material_id      UUID NOT NULL REFERENCES materials(id),
    transaction_type stock_tx_type NOT NULL,
    quantity         DECIMAL(12,3) NOT NULL,
    unit_price       BIGINT,
    total_price      BIGINT,
    stock_before     DECIMAL(12,3),
    stock_after      DECIMAL(12,3),
    reference_id     UUID,
    supplier_id      UUID REFERENCES suppliers(id),
    warehouse_id     UUID REFERENCES warehouses(id),
    description      TEXT,
    store_id         UUID REFERENCES stores(id),
    created_by       UUID REFERENCES users(id),
    created_at       TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_stock_tx_material ON stock_transactions(material_id);
CREATE INDEX idx_stock_tx_date ON stock_transactions(created_at);
CREATE INDEX idx_stock_tx_store ON stock_transactions(store_id);

CREATE TABLE warehouses (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name      VARCHAR(100) NOT NULL,
    address   TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE inventory_counts (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    warehouse_id   UUID REFERENCES warehouses(id),
    material_id    UUID NOT NULL REFERENCES materials(id),
    expected_qty   DECIMAL(12,3),
    actual_qty     DECIMAL(12,3),
    difference_qty DECIMAL(12,3),
    counted_by     UUID REFERENCES users(id),
    counted_at     TIMESTAMPTZ DEFAULT now()
);
```

#### promotions

```sql
CREATE TYPE promotion_type AS ENUM ('auto_apply', 'voucher', 'loyalty');
CREATE TYPE reward_type AS ENUM ('balance', 'product', 'time', 'voucher');

CREATE TABLE promotions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(200) NOT NULL,
    description TEXT,
    type        promotion_type NOT NULL,
    priority    INTEGER DEFAULT 0,
    is_active   BOOLEAN DEFAULT true,
    valid_from  TIMESTAMPTZ,
    valid_to    TIMESTAMPTZ,
    created_at  TIMESTAMPTZ DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE TABLE promotion_conditions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    promotion_id    UUID NOT NULL REFERENCES promotions(id),
    condition_key   VARCHAR(50) NOT NULL,
    condition_value JSONB NOT NULL,
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE promotion_rewards (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    promotion_id UUID NOT NULL REFERENCES promotions(id),
    reward_type  reward_type NOT NULL,
    reward_value JSONB NOT NULL,
    created_at   TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE lucky_spin_rewards (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         VARCHAR(100) NOT NULL,
    reward_type  reward_type NOT NULL,
    reward_value JSONB NOT NULL,
    probability  DECIMAL(5,4) NOT NULL,
    max_per_day  INTEGER DEFAULT 0,
    is_active    BOOLEAN DEFAULT true,
    created_at   TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE lucky_spins (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    member_id UUID NOT NULL REFERENCES members(id),
    reward_id UUID REFERENCES lucky_spin_rewards(id),
    is_win    BOOLEAN DEFAULT false,
    spun_at   TIMESTAMPTZ DEFAULT now()
);
```

#### bonus balance & payment priority

**Nguyên tắc:**
- `members.balance` = tiền thật → tính doanh thu, xuất hóa đơn
- `members.bonus_balance` = tiền KM → không tính doanh thu, không xuất hóa đơn
- `bonus_balance` không thể rút/nạp thêm, chỉ tự động cộng từ promotion

**Flow thanh toán:**

```
amount = 100k

IF bonus_balance >= amount:
   bonus_balance -= amount
   ghi: amount, bonus_before=old, bonus_after=new
   main trước/sau không đổi
   → KHÔNG tính doanh thu

ELSE:
   remaining = amount - bonus_balance
   bonus_balance = 0
   balance -= remaining
   ghi: amount, bonus trước/sau, main trước/sau
   → chỉ remaining tính doanh thu
```

**Nguồn gốc bonus:**
- `promotion_bonus` transaction: chỉ vào `bonus_balance`
- `topup` transaction: chỉ vào `balance` (main), không bao giờ vào bonus
- `refund`: nếu refund tiền KM → vào `bonus_balance`; tiền thật → vào `balance`

**Báo cáo doanh thu:**
```sql
-- Doanh thu thực tế (chỉ main)
SELECT SUM(amount) FROM member_transactions
WHERE balance_before != balance_after
  AND transaction_type IN ('game_time', 'service_order', 'combo_purchase');

-- KM đã dùng
SELECT SUM(amount) FROM member_transactions
WHERE bonus_before != bonus_after;
```

**Hiển thị cho hội viên:**
- Client UI: `"Số dư: 150k (Chính 100k + KM 50k)"`
- Xếp hạng (`total_spent`): chỉ tính từ `balance`, không tính `bonus_balance`

#### booking & deposit

```sql
CREATE TYPE booking_status AS ENUM ('pending', 'confirmed', 'checked_in', 'completed', 'cancelled', 'no_show');

CREATE TABLE machine_bookings (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id      UUID NOT NULL REFERENCES machines(id),
    member_id       UUID REFERENCES members(id),
    customer_name   VARCHAR(100),
    customer_phone  VARCHAR(20),
    booked_from     TIMESTAMPTZ NOT NULL,
    booked_to       TIMESTAMPTZ NOT NULL,
    deposit_amount  BIGINT DEFAULT 0,
    deposit_transaction_id UUID REFERENCES member_transactions(id),
    status          booking_status DEFAULT 'pending',
    cancel_at       TIMESTAMPTZ,
    notes           TEXT,
    created_by      UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_machine_bookings_machine ON machine_bookings(machine_id);
CREATE INDEX idx_machine_bookings_status ON machine_bookings(status);
```

#### shifts & employees

```sql
CREATE TYPE shift_status AS ENUM ('open', 'closed');
CREATE TYPE handover_type AS ENUM ('cash_in', 'cash_out');

CREATE TABLE shifts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id),
    store_id        UUID REFERENCES stores(id),
    started_at      TIMESTAMPTZ NOT NULL,
    ended_at        TIMESTAMPTZ,
    status          shift_status DEFAULT 'open',
    opening_balance BIGINT DEFAULT 0,
    closing_balance BIGINT,
    expected_total  BIGINT,
    notes           TEXT,
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE cash_handovers (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shift_id      UUID NOT NULL REFERENCES shifts(id),
    amount        BIGINT NOT NULL,
    handover_type handover_type NOT NULL,
    reason        TEXT,
    created_by    UUID REFERENCES users(id),
    created_at    TIMESTAMPTZ DEFAULT now()
);
```

#### curfew & minor policy

```sql
CREATE TABLE curfew_policies (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    day_of_week      INTEGER NOT NULL,
    curfew_start     TIME NOT NULL,
    curfew_end       TIME NOT NULL,
    max_minor_hours  INTEGER DEFAULT 2,
    is_active        BOOLEAN DEFAULT true,
    store_id         UUID REFERENCES stores(id),
    override_by_admin UUID REFERENCES users(id),
    override_reason  TEXT,
    override_at      TIMESTAMPTZ,
    created_at       TIMESTAMPTZ DEFAULT now()
);
```

#### e-invoice

```sql
CREATE TYPE einvoice_status AS ENUM ('pending', 'sent', 'failed', 'cancelled');

CREATE TABLE e_invoice_configs (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider   VARCHAR(30) NOT NULL,
    api_key    TEXT,
    api_secret TEXT,
    endpoint   TEXT,
    is_active  BOOLEAN DEFAULT false,
    store_id   UUID REFERENCES stores(id),
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE e_invoices (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id       UUID NOT NULL REFERENCES orders(id),
    invoice_code   VARCHAR(50),
    invoice_number VARCHAR(50),
    provider       VARCHAR(30) NOT NULL,
    raw_request    JSONB,
    raw_response   JSONB,
    status         einvoice_status DEFAULT 'pending',
    created_at     TIMESTAMPTZ DEFAULT now()
);
```

#### chat

```sql
CREATE TYPE message_type AS ENUM ('text', 'image', 'file', 'system');

CREATE TYPE chat_participant_type AS ENUM (
    'admin', 'machine', 'member'
);

CREATE TABLE chat_conversations (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title      VARCHAR(200),
    is_group   BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE chat_participants (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id   UUID NOT NULL REFERENCES chat_conversations(id),
    participant_type  chat_participant_type NOT NULL,
    participant_id    UUID NOT NULL,
    last_read_at      TIMESTAMPTZ,
    joined_at         TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE chat_messages (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES chat_conversations(id),
    sender_type     chat_participant_type NOT NULL,
    sender_id       UUID NOT NULL,
    message         TEXT NOT NULL,
    message_type    message_type DEFAULT 'text',
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_chat_messages_conv ON chat_messages(conversation_id, created_at);

CREATE TABLE service_feedback (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id UUID NOT NULL REFERENCES machines(id),
    member_id  UUID REFERENCES members(id),
    order_id   UUID REFERENCES orders(id),
    rating     INTEGER CHECK (rating >= 1 AND rating <= 5),
    content    TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);
```

#### system & audit

```sql
CREATE TABLE system_settings (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_name  VARCHAR(50) NOT NULL,
    key         VARCHAR(100) NOT NULL,
    value       JSONB NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ DEFAULT now(),
    updated_at  TIMESTAMPTZ DEFAULT now(),
    UNIQUE (group_name, key)
);

-- Default settings
INSERT INTO system_settings (group_name, key, value, description) VALUES
('billing', 'rounding_mode', '{"mode": "round_up", "interval_minutes": 60}', 'Cách làm tròn giờ chơi'),
('billing', 'grace_period_minutes', '{"value": 10}', 'Số phút ân hạn trước khi tính thêm giờ'),
('billing', 'min_billing_unit', '{"minutes": 30}', 'Đơn vị tính tiền tối thiểu'),
('billing', 'no_show_cancel_minutes', '{"value": 30}', 'Tự động hủy booking sau X phút không đến'),
('topup', 'presets', '{"values": [5000, 10000, 20000, 50000, 100000, 200000, 500000, 1000000]}', 'Mệnh giá nạp tiền hiển thị trên máy trạm');

CREATE TABLE audit_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    action      VARCHAR(100) NOT NULL,
    entity_type VARCHAR(50),
    entity_id   UUID,
    user_id     UUID REFERENCES users(id),
    metadata    JSONB,
    ip_address  VARCHAR(45),
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_entity ON audit_logs(entity_type, entity_id);
CREATE INDEX idx_audit_logs_date ON audit_logs(created_at);

#### Audit Log Hook Strategy

Audit log được ghi tự động qua 2 lớp:

**1. GORM Hooks (tự động — cho CRUD cơ bản):**

```go
// GORM hooks auto-log mọi thay đổi
func (m *Member) AfterCreate(tx *gorm.DB) error {
    return audit.Log(tx, "member.created", "member", m.ID, nil)
}

func (m *Member) AfterUpdate(tx *gorm.DB) error {
    diff := buildDiff(m, tx.Statement.Model)
    return audit.Log(tx, "member.updated", "member", m.ID, diff)
}

func (m *Member) AfterDelete(tx *gorm.DB) error {
    return audit.Log(tx, "member.deleted", "member", m.ID, nil)
}
```

**2. Manual middleware — cho business transaction quan trọng:**

```go
// Dùng trong các handler đặc thù cần audit chi tiết
// Ví dụ: nạp tiền, thanh toán, kết ca, hủy đơn

// Handler:
func (h *MemberHandler) Topup(c *gin.Context) {
    // ... business logic ...
    h.audit.Log(c, "member.topup", "member", memberID, map[string]any{
        "amount":       req.Amount,
        "balance_before": before,
        "balance_after":  after,
    })
}
```

**Nguyên tắc:**
- CRUD cơ bản → GORM hooks tự ghi
- Giao dịch tài chính / quyền hạn → manual middleware (cần context chi tiết hơn)
- `audit.Log()` luôn được gọi trong cùng DB transaction với business logic chính

CREATE TYPE notification_type AS ENUM ('system_alert', 'promotion', 'topup', 'curfew', 'session', 'booking');

CREATE TABLE notifications (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type         notification_type NOT NULL,
    title        VARCHAR(200) NOT NULL,
    content      TEXT,
    reference_id UUID,
    created_at   TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE notification_recipients (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id UUID NOT NULL REFERENCES notifications(id),
    recipient_id    UUID NOT NULL,
    is_read         BOOLEAN DEFAULT false,
    read_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE TYPE app_platform AS ENUM ('windows', 'mac', 'linux', 'ios', 'android');

CREATE TABLE app_updates (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    version     VARCHAR(20) NOT NULL,
    platform    app_platform NOT NULL,
    file_url    TEXT NOT NULL,
    changelog   TEXT,
    is_required BOOLEAN DEFAULT false,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TYPE backup_status AS ENUM ('running', 'completed', 'failed');

CREATE TABLE backup_logs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    file_name    VARCHAR(255) NOT NULL,
    file_size    BIGINT,
    file_path    TEXT,
    status       backup_status DEFAULT 'running',
    notes        TEXT,
    created_by   UUID REFERENCES users(id),
    started_at   TIMESTAMPTZ DEFAULT now(),
    completed_at TIMESTAMPTZ
);

#### website blocking

```sql
CREATE TYPE website_rule_type AS ENUM ('block', 'allow');
CREATE TYPE website_category AS ENUM ('gaming', 'social', 'streaming', 'adult', 'p2p', 'custom');

CREATE TABLE website_rules (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pattern     VARCHAR(500) NOT NULL,
    rule_type   website_rule_type NOT NULL,
    category    website_category,
    description TEXT,
    is_active   BOOLEAN DEFAULT true,
    created_at  TIMESTAMPTZ DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX idx_website_rules_active ON website_rules(is_active) WHERE is_active = true;

CREATE TABLE website_rule_mappings (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id          UUID NOT NULL REFERENCES website_rules(id),
    machine_group_id UUID REFERENCES machine_groups(id),
    created_at       TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE website_schedules (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id     UUID NOT NULL REFERENCES website_rules(id),
    day_of_week INTEGER[],
    start_time  TIME,
    end_time    TIME,
    is_active   BOOLEAN DEFAULT true,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE website_violations (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_id   UUID NOT NULL REFERENCES machines(id),
    rule_id      UUID REFERENCES website_rules(id),
    domain       VARCHAR(500) NOT NULL,
    url          TEXT,
    process_name VARCHAR(200),
    blocked_at   TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_website_violations_machine ON website_violations(machine_id, blocked_at);
```

#### seeds

```sql
-- Admin mặc định cho client login
INSERT INTO members (code, full_name, role, password_hash, is_active)
VALUES ('ADMIN', 'admin', 'admin', '$2a$10$...', true);
-- username: admin, password: admin

-- Permissions (từng role)
INSERT INTO permissions (code, name, module) VALUES
('members.view',    'Xem hội viên',     'members'),
('members.create',  'Tạo hội viên',     'members'),
('members.topup',   'Nạp tiền',         'members'),
('machines.view',   'Xem máy',          'machines'),
('orders.view',     'Xem đơn hàng',     'orders'),
('orders.create',   'Tạo đơn hàng',     'orders'),
('orders.pay',      'Thanh toán',       'orders'),
('reports.view',    'Xem báo cáo',      'reports'),
('settings.edit',   'Sửa cài đặt',      'settings'),
('client.admin',    'Admin client',     'client');

-- Roles
INSERT INTO roles (name, description) VALUES
('owner',   'Chủ — toàn quyền'),
('manager', 'Quản lý'),
('staff',   'Nhân viên');

-- Owner: tất cả permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p WHERE r.name = 'owner';
```

