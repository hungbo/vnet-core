package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/vnet/core/internal/authz"
	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/database"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func main() {
	// Thực đơn mẫu nằm sau một cờ chứ không chạy mặc định: đó là dữ liệu để thử
	// nghiệm, không phải dữ liệu quán nào cũng muốn có trong database thật.
	withMenu := flag.Bool("menu", false, "tạo thêm thực đơn mẫu 100 món (kèm ảnh) để thử nghiệm")
	flag.Parse()

	cfg := config.Load()
	db := database.Init(&cfg.Database)

	if err := runMigrations(db); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	log.Println("Migrations completed")

	if err := seed(db); err != nil {
		log.Fatalf("Seed failed: %v", err)
	}

	if *withMenu {
		if err := seedMenu(db, cfg.Server.UploadDir); err != nil {
			log.Fatalf("Seed menu failed: %v", err)
		}
	}

	log.Println("Seed completed")
}

func runMigrations(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.User{}, &model.Role{}, &model.Permission{},
		&model.UserRole{}, &model.RolePermission{}, &model.Member{}, &model.MemberGroup{},
		&model.Machine{}, &model.MachineGroup{}, &model.Category{}, &model.Product{},
		&model.Order{}, &model.OrderItem{}, &model.Payment{},
		&model.MachineSession{}, &model.Combo{}, &model.ComboPurchase{},
		&model.MachineBooking{}, &model.Promotion{}, &model.PromotionCondition{}, &model.PromotionReward{},
		&model.SystemSetting{}, &model.AuditLog{},
		&model.MachineAsset{}, &model.MachineHardwareSnapshot{},
		&model.Shift{}, &model.CashHandover{},
		&model.ProductOptionGroup{}, &model.ProductOption{}, &model.ProductIngredient{},
		&model.PrinterConfig{}, &model.ProductPrinterMapping{},
		&model.Supplier{}, &model.StockTransaction{}, &model.InventoryCountSession{}, &model.InventoryCount{},
		&model.Notification{}, &model.NotificationRecipient{}, &model.MemberNotification{}, &model.BackupLog{},
		&model.EInvoiceConfig{}, &model.EInvoice{},
		&model.ChatRoom{}, &model.ChatParticipant{}, &model.ChatMessage{}, &model.ServiceFeedback{},
		&model.AppUpdate{}, &model.WebsiteBlockingRule{}, &model.WebsiteRuleMapping{},
		&model.WebsiteBlockingSchedule{}, &model.WebsiteBlockingViolation{},
		&model.CurfewPolicy{}, &model.LuckySpinReward{}, &model.LuckySpinLog{},
		&model.MemberTransaction{}, &model.MemberAttendance{},
	)
}

// seedPassword là mật khẩu ban đầu của một tài khoản mẫu: SEED_PASSWORD_<TÊN>
// (ví dụ SEED_PASSWORD_ADMIN) nếu có, rồi đến SEED_PASSWORD, cuối cùng mới là mật
// khẩu mặc định ai cũng biết. Bộ cài Windows đặt riêng một mật khẩu ngẫu nhiên
// cho từng tài khoản — chung một mật khẩu thì giao tài khoản staff cho thu ngân
// cũng là giao luôn chìa khoá admin. Qua biến môi trường chứ không qua tham số
// dòng lệnh vì tham số hiện ra cho mọi tiến trình khác trên máy.
//
// Chỉ có tác dụng với tài khoản được TẠO MỚI: tài khoản đã có giữ nguyên mật khẩu.
func seedPassword(username string) string {
	if p := os.Getenv("SEED_PASSWORD_" + strings.ToUpper(username)); p != "" {
		return p
	}
	if p := os.Getenv("SEED_PASSWORD"); p != "" {
		return p
	}
	return service.DefaultSeedPassword
}

func seed(db *gorm.DB) error {
	adminHash, _ := utils.HashPassword(seedPassword("admin"))
	managerHash, _ := utils.HashPassword(seedPassword("manager"))
	staffHash, _ := utils.HashPassword(seedPassword("staff"))

	adminRole := model.Role{}
	if err := db.Where("name = ?", "owner").FirstOrCreate(&adminRole, &model.Role{
		Name: "owner", Description: "Chủ — toàn quyền",
	}).Error; err != nil {
		return err
	}

	managerRole := model.Role{}
	if err := db.Where("name = ?", "manager").FirstOrCreate(&managerRole, &model.Role{
		Name: "manager", Description: "Quản lý",
	}).Error; err != nil {
		return err
	}

	staffRole := model.Role{}
	if err := db.Where("name = ?", "staff").FirstOrCreate(&staffRole, &model.Role{
		Name: "staff", Description: "Nhân viên",
	}).Error; err != nil {
		return err
	}

	// Danh mục quyền là một chỗ duy nhất: internal/authz. Router gác từng API
	// theo đúng những mã này, nên thêm quyền mới chỉ cần sửa danh mục.
	perms := make([]model.Permission, 0, len(authz.Catalog))
	for _, c := range authz.Catalog {
		perms = append(perms, model.Permission{Code: c.Code, Name: c.Name, Module: c.Module})
	}
	for _, p := range perms {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Where("code = ?", p.Code).FirstOrCreate(&p).Error; err != nil {
			return err
		}
	}

	admin := model.User{
		Username:     "admin",
		PasswordHash: adminHash,
		FullName:     "Admin",
		IsActive:     true,
	}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Where("username = ?", "admin").FirstOrCreate(&admin).Error; err != nil {
		return err
	}

	manager := model.User{
		Username:     "manager",
		PasswordHash: managerHash,
		FullName:     "Manager",
		IsActive:     true,
	}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Where("username = ?", "manager").FirstOrCreate(&manager).Error; err != nil {
		return err
	}

	staff := model.User{
		Username:     "staff",
		PasswordHash: staffHash,
		FullName:     "Staff",
		IsActive:     true,
	}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Where("username = ?", "staff").FirstOrCreate(&staff).Error; err != nil {
		return err
	}

	db.Model(&admin).Association("Roles").Replace(&[]model.Role{adminRole})
	db.Model(&manager).Association("Roles").Replace(&[]model.Role{managerRole})
	db.Model(&staff).Association("Roles").Replace(&[]model.Role{staffRole})

	var allPerms []model.Permission
	db.Find(&allPerms)
	db.Model(&adminRole).Association("Permissions").Replace(&allPerms)

	// Quản lý lo toàn bộ việc kinh doanh hằng ngày nhưng không chạm back-office
	// (tài khoản nhân viên, sao lưu, nhật ký, gửi thông báo hàng loạt). "*" cũng
	// phải loại: PermissionRequired coi nó là siêu quyền, giữ lại thì mọi giới
	// hạn phía trên thành vô nghĩa.
	var managerPerms []model.Permission
	db.Where("code IN ?", authz.ManagerCodes()).Find(&managerPerms)
	db.Model(&managerRole).Association("Permissions").Replace(&managerPerms)

	// Nhân viên quầy VẬN HÀNH chứ không CẤU HÌNH: mở/trả máy, bán hàng, nhận
	// đặt chỗ, kiểm kê, mở/đóng ca — nhưng không đổi giá, không sửa khuyến mãi,
	// không xoá dữ liệu nền, và tuyệt đối không chạm back-office (bản cũ cấp
	// client.admin cho staff, đủ để họ tự tạo tài khoản owner).
	var staffPerms []model.Permission
	db.Where("code IN ?", authz.StaffCodes()).Find(&staffPerms)
	db.Model(&staffRole).Association("Permissions").Replace(&staffPerms)

	settings := []model.SystemSetting{
		{GroupName: "topup", Key: "presets", Value: `{"values": [5000, 10000, 20000, 50000, 100000, 200000, 500000, 1000000]}`, Description: "Mệnh giá nạp tiền"},
	}
	for _, s := range settings {
		db.Clauses(clause.OnConflict{DoNothing: true}).Where("group_name = ? AND key = ?", s.GroupName, s.Key).FirstOrCreate(&s)
	}

	groups := []model.MemberGroup{
		{Name: "Đồng", MinSpent: 0, DiscountPercent: 0, IsDefault: true},
		{Name: "Bạc", MinSpent: 500000, DiscountPercent: 5},
		{Name: "Vàng", MinSpent: 2000000, DiscountPercent: 10},
	}
	for _, g := range groups {
		db.Clauses(clause.OnConflict{DoNothing: true}).Where("name = ?", g.Name).FirstOrCreate(&g)
	}

	machineGroups := []model.MachineGroup{
		{Name: "VIP", Color: "#FFD700", PricePerHour: 15000, SortOrder: 1},
		{Name: "Thường", Color: "#909399", PricePerHour: 5000, SortOrder: 2},
	}
	for _, mg := range machineGroups {
		db.Clauses(clause.OnConflict{DoNothing: true}).Where("name = ?", mg.Name).FirstOrCreate(&mg)
	}

	fmt.Println("Seed data completed!")
	// Mật khẩu tự đặt thì KHÔNG in ra: dòng này nằm trong install.log và nhật ký
	// của container, ai đọc được log là đọc được mật khẩu.
	for _, u := range []string{"admin", "manager", "staff"} {
		if p := seedPassword(u); p == service.DefaultSeedPassword {
			fmt.Printf("  %s / %s  (mật khẩu mặc định — đổi ngay sau khi đăng nhập)\n", u, p)
		} else {
			fmt.Printf("  %s  (mật khẩu đặt từ SEED_PASSWORD)\n", u)
		}
	}
	return nil
}
