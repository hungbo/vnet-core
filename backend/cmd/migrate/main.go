package main

import (
	"log"

	"github.com/vnet/core/internal/authz"
	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/database"
	"github.com/vnet/core/internal/model"
	"gorm.io/gorm"
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

	// Bỏ trạng thái máy "maintenance". Nó chưa bao giờ được code đặt hay đọc —
	// chỉ là một lựa chọn trên giao diện — nhưng quán nào đã lỡ đặt tay qua API
	// thì cột đang giữ một giá trị mà từ nay API từ chối, và máy đó không khớp
	// nhãn nào trên trang quản trị. Đưa về offline: heartbeat kế tiếp sẽ tự
	// chuyển sang available.
	{name: "drop_machine_status_maintenance", sql: "UPDATE machines SET status = 'offline' WHERE status = 'maintenance'"},

	// Bỏ khoá máy trạm (agent token). Cơ chế này là tuỳ chọn, thực tế không máy
	// nào bật, và đã được gỡ khỏi toàn bộ mã. Xoá hai cột cho sạch — GORM
	// AutoMigrate không tự xoá cột nên phải làm tay.
	{name: "drop_machine_agent_token", sql: "ALTER TABLE machines DROP COLUMN IF EXISTS agent_token, DROP COLUMN IF EXISTS agent_token_issued_at"},

	// Một đơn hàng chỉ được đánh giá một lần. Chỉ mục PHẦN: đánh giá không gắn
	// đơn (nhận xét chung về dịch vụ) thì gửi bao nhiêu lần cũng được.
	{name: "feedback_one_per_order", sql: "CREATE UNIQUE INDEX IF NOT EXISTS idx_feedback_order ON service_feedbacks (order_id) WHERE order_id IS NOT NULL"},

	// Tính lại total_spent theo đúng nghĩa "tiền khách đã tiêu tại quán": tiền
	// giờ chơi + đơn hàng đã hoàn tất + gói cước đã mua.
	//
	// Từ nay ba chỗ đó đều tự cộng, nhưng dữ liệu CŨ chỉ có tiền gói cước — cột
	// này quyết định hạng hội viên, nên không tính lại thì khách cũ vĩnh viễn
	// đứng ở hạng thấp hơn khách mới tiêu cùng số tiền. Chạy một lần, cộng từ
	// chính sổ giao dịch và bảng đơn hàng nên kết quả kiểm chứng được.
	// Thu hồi client.admin khỏi vai trò staff. Quyền này gác toàn bộ back-office
	// (quản lý người dùng, sao lưu, nhật ký, gửi thông báo); cấp cho nhân viên
	// quầy nghĩa là họ tạo được tài khoản owner cho chính mình.
	{name: "revoke_client_admin_from_staff", sql: `
		DELETE FROM role_permissions rp
		USING roles r, permissions p
		WHERE rp.role_id = r.id AND rp.permission_id = p.id
		  AND r.name = 'staff' AND p.code = 'client.admin'`},

	{name: "backfill_member_total_spent", sql: `
		UPDATE members m SET total_spent =
			COALESCE((SELECT SUM(ABS(tx.amount)) FROM member_transactions tx
				WHERE tx.member_id = m.id
				  AND tx.transaction_type IN ('session_fee', 'combo_purchase')), 0)
			+ COALESCE((SELECT SUM(o.final_amount) FROM orders o
				WHERE o.member_id = m.id
				  AND o.status = 'completed'
				  AND o.order_type <> 'topup'
				  AND o.deleted_at IS NULL), 0)`},

	// Xoá mềm + unique index = tên bị cháy vĩnh viễn: xoá sản phẩm "Mì tôm" rồi
	// tạo lại đúng tên đó bị chặn bởi một dòng mà người dùng không còn nhìn thấy
	// ở đâu cả. Chuyển sang unique MỘT PHẦN, chỉ ràng buộc trên bản ghi còn sống.
	//
	// Giữ nguyên TÊN index để AutoMigrate không dựng lại bản đầy đủ: GORM chỉ
	// kiểm tra index có tồn tại theo tên hay không, không so định nghĩa.
	//
	// Bốn bảng CỐ Ý không đổi:
	//   - orders.order_code: mã đơn sinh tuần tự và tra cứu Unscoped, trùng mã
	//     giữa đơn sống và đơn đã xoá là hỏng sổ sách.
	//   - topup_cards.code, gift_cards.code, gift_cards.serial: seri thẻ không
	//     bao giờ được cấp lại, nếu không một thẻ đã huỷ có thể nạp lại lần hai.
	{name: "partial_unique_categories_name", sql: `
		DO $$ BEGIN
			DROP INDEX IF EXISTS idx_categories_name;
			CREATE UNIQUE INDEX idx_categories_name ON categories (name) WHERE deleted_at IS NULL;
		END $$`},
	{name: "partial_unique_combos_name", sql: `
		DO $$ BEGIN
			DROP INDEX IF EXISTS idx_combos_name;
			CREATE UNIQUE INDEX idx_combos_name ON combos (name) WHERE deleted_at IS NULL;
		END $$`},
	{name: "partial_unique_machine_groups_name", sql: `
		DO $$ BEGIN
			DROP INDEX IF EXISTS idx_machine_groups_name;
			CREATE UNIQUE INDEX idx_machine_groups_name ON machine_groups (name) WHERE deleted_at IS NULL;
		END $$`},
	{name: "partial_unique_machines_code", sql: `
		DO $$ BEGIN
			DROP INDEX IF EXISTS idx_machines_machine_code;
			CREATE UNIQUE INDEX idx_machines_machine_code ON machines (machine_code) WHERE deleted_at IS NULL;
		END $$`},
	{name: "partial_unique_products_name", sql: `
		DO $$ BEGIN
			DROP INDEX IF EXISTS idx_products_name;
			CREATE UNIQUE INDEX idx_products_name ON products (name) WHERE deleted_at IS NULL;
		END $$`},
	{name: "partial_unique_members_username", sql: `
		DO $$ BEGIN
			DROP INDEX IF EXISTS idx_members_username;
			CREATE UNIQUE INDEX idx_members_username ON members (username) WHERE deleted_at IS NULL;
		END $$`},
	{name: "partial_unique_users_username", sql: `
		DO $$ BEGIN
			DROP INDEX IF EXISTS idx_users_username;
			CREATE UNIQUE INDEX idx_users_username ON users (username) WHERE deleted_at IS NULL;
		END $$`},
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

	dongBoQuyen(db)
}

// dongBoQuyen đưa danh mục quyền mới vào hệ thống đã chạy.
//
// Chạy được nhiều lần: chỉ THÊM, không bao giờ gỡ quyền ai đó đã tự cấp trên
// giao diện. Phần cấp quyền mặc định cho manager/staff chỉ chạy một lần (ghi
// vào schema_migrations) để lần sau quản trị viên sửa tay không bị ghi đè.
func dongBoQuyen(db *gorm.DB) {
	// 1. Bổ sung mã quyền còn thiếu.
	them := 0
	for _, c := range authz.Catalog {
		var n int64
		db.Model(&model.Permission{}).Where("code = ?", c.Code).Count(&n)
		if n > 0 {
			continue
		}
		if err := db.Create(&model.Permission{Code: c.Code, Name: c.Name, Module: c.Module}).Error; err != nil {
			log.Printf("FAIL thêm quyền %s: %v", c.Code, err)
			continue
		}
		them++
	}
	log.Printf("OK   danh mục quyền: thêm %d mã mới", them)

	// 2. Vai trò đang giữ "*" (chủ quán) được cấp toàn bộ mã: nó vốn đã qua mọi
	// cửa nhờ siêu quyền, nhưng để màn hình Phân quyền hiện đúng thực tế chứ
	// không phải 11/140 ô được tick.
	var chuQuan []model.Role
	db.Raw(`SELECT r.* FROM roles r
		JOIN role_permissions rp ON rp.role_id = r.id
		JOIN permissions p ON p.id = rp.permission_id
		WHERE p.code = ?`, authz.Wildcard).Scan(&chuQuan)
	for _, r := range chuQuan {
		var tatCa []model.Permission
		db.Find(&tatCa)
		for _, q := range tatCa {
			db.Exec(`INSERT INTO role_permissions (role_id, permission_id)
				VALUES (?, ?) ON CONFLICT DO NOTHING`, r.ID, q.ID)
		}
		log.Printf("OK   cấp toàn bộ %d quyền cho vai trò %s", len(tatCa), r.Name)
	}

	// 3. Cấp mặc định cho hai vai trò sẵn có — đúng một lần.
	const mốc = "grant_default_permissions_v2"
	var đã int64
	db.Table("schema_migrations").Where("name = ?", mốc).Count(&đã)
	if đã > 0 {
		log.Printf("SKIP %s (already applied)", mốc)
		return
	}
	for tên, mã := range map[string][]string{
		"manager": authz.ManagerCodes(),
		"staff":   authz.StaffCodes(),
	} {
		var vaiTro model.Role
		if err := db.Where("name = ?", tên).First(&vaiTro).Error; err != nil {
			continue
		}
		var quyen []model.Permission
		db.Where("code IN ?", mã).Find(&quyen)
		for _, q := range quyen {
			db.Exec(`INSERT INTO role_permissions (role_id, permission_id)
				VALUES (?, ?) ON CONFLICT DO NOTHING`, vaiTro.ID, q.ID)
		}
		log.Printf("OK   cấp %d quyền mặc định cho vai trò %s", len(quyen), tên)
	}
	db.Exec("INSERT INTO schema_migrations (name) VALUES (?)", mốc)
}
