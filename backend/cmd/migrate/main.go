package main

import (
	"log"

	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/database"
)

type migration struct {
	name string
	sql  string
}

var migrations = []migration{
	{name: "create_unaccent_extension", sql: "CREATE EXTENSION IF NOT EXISTS unaccent"},
	{name: "drop_products_category_id_not_null", sql: "ALTER TABLE products ALTER COLUMN category_id DROP NOT NULL"},
	{name: "drop_product_options_price_adjust", sql: "ALTER TABLE product_options DROP COLUMN IF EXISTS price_adjust"},
	{name: "drop_stock_transactions_warehouse_id", sql: "ALTER TABLE stock_transactions DROP COLUMN IF EXISTS warehouse_id"},
	{name: "drop_inventory_counts_warehouse_id", sql: "ALTER TABLE inventory_counts DROP COLUMN IF EXISTS warehouse_id"},

	// Time-of-day columns were created as timestamptz because gorm:"type:time"
	// names GORM's abstract time type, which the Postgres driver renders as a
	// timestamp. That made every value like "22:00:00" unstorable, so curfew
	// schedules, combos and time-of-day pricing could not be created at all.
	//
	// timestamptz -> time has no implicit cast, hence the explicit USING.
	{name: "combos_slot_start_to_time", sql: "ALTER TABLE combos ALTER COLUMN slot_start TYPE time USING slot_start::time"},
	{name: "combos_slot_end_to_time", sql: "ALTER TABLE combos ALTER COLUMN slot_end TYPE time USING slot_end::time"},
	{name: "curfew_start_to_time", sql: "ALTER TABLE curfew_policies ALTER COLUMN curfew_start TYPE time USING curfew_start::time"},
	{name: "curfew_end_to_time", sql: "ALTER TABLE curfew_policies ALTER COLUMN curfew_end TYPE time USING curfew_end::time"},
	{name: "time_pricing_start_to_time", sql: "ALTER TABLE time_based_pricings ALTER COLUMN start_time TYPE time USING start_time::time"},
	{name: "time_pricing_end_to_time", sql: "ALTER TABLE time_based_pricings ALTER COLUMN end_time TYPE time USING end_time::time"},
	{name: "website_schedule_start_to_time", sql: "ALTER TABLE website_blocking_schedules ALTER COLUMN start_time TYPE time USING start_time::time"},
	{name: "website_schedule_end_to_time", sql: "ALTER TABLE website_blocking_schedules ALTER COLUMN end_time TYPE time USING end_time::time"},

	// Mã bí mật của thẻ chuyển từ lưu thô sang lưu băm SHA-256 (64 ký tự hex).
	{name: "topup_card_pin_hash_width", sql: "ALTER TABLE topup_cards ALTER COLUMN pin TYPE varchar(64)"},
	{name: "gift_card_code_hash_width", sql: "ALTER TABLE gift_cards ALTER COLUMN code TYPE varchar(64)"},

	// Một hội viên chỉ điểm danh một lần mỗi ngày. Ràng buộc phải ở tầng
	// database: kiểm bằng câu lệnh SELECT rồi INSERT sẽ thua khi hai request
	// gửi cùng lúc.
	{name: "attendance_one_per_day", sql: "CREATE UNIQUE INDEX IF NOT EXISTS idx_attendance_member_day ON member_attendances (member_id, checkin_date)"},

	// Một đơn hàng chỉ được đánh giá một lần. Chỉ mục PHẦN: đánh giá không gắn
	// đơn (nhận xét chung về dịch vụ) thì gửi bao nhiêu lần cũng được.
	{name: "feedback_one_per_order", sql: "CREATE UNIQUE INDEX IF NOT EXISTS idx_feedback_order ON service_feedbacks (order_id) WHERE order_id IS NOT NULL"},
}

func main() {
	cfg := config.Load()
	db := database.Init(&cfg.Database)

	db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		name VARCHAR(255) PRIMARY KEY,
		applied_at TIMESTAMPTZ DEFAULT NOW()
	)`)

	for _, m := range migrations {
		var count int64
		db.Table("schema_migrations").Where("name = ?", m.name).Count(&count)
		if count > 0 {
			log.Printf("SKIP %s (already applied)", m.name)
			continue
		}
		if err := db.Exec(m.sql).Error; err != nil {
			log.Fatalf("FAIL %s: %v", m.name, err)
		}
		db.Exec("INSERT INTO schema_migrations (name) VALUES (?)", m.name)
		log.Printf("OK   %s", m.name)
	}
}
