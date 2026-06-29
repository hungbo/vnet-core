package main

import (
	"fmt"
	"log"

	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/database"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func main() {
	cfg := config.Load()
	db := database.Init(&cfg.Database)

	if err := runMigrations(db); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	log.Println("Migrations completed")

	if err := seed(db); err != nil {
		log.Fatalf("Seed failed: %v", err)
	}
	log.Println("Seed completed")
}

func runMigrations(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.Store{}, &model.User{}, &model.Role{}, &model.Permission{},
		&model.UserRole{}, &model.RolePermission{}, &model.Member{}, &model.MemberGroup{},
		&model.Machine{}, &model.MachineGroup{}, &model.Category{}, &model.Product{},
		&model.Order{}, &model.OrderItem{}, &model.Payment{},
		&model.MachineSession{}, &model.Combo{}, &model.ComboPurchase{},
		&model.MachineBooking{}, &model.Promotion{}, &model.PromotionCondition{}, &model.PromotionReward{},
		&model.SystemSetting{}, &model.AuditLog{},
		&model.MachinePrice{}, &model.TimeBasedPricing{}, &model.MachineAsset{}, &model.MachineHardwareSnapshot{},
		&model.ComboItem{}, &model.Shift{}, &model.CashHandover{},
		&model.ProductOptionGroup{}, &model.ProductOption{}, &model.ProductIngredient{},
		&model.PrinterConfig{}, &model.ProductPrinterMapping{},
		&model.Supplier{}, &model.Warehouse{}, &model.StockTransaction{}, &model.InventoryCount{},
		&model.Notification{}, &model.NotificationRecipient{}, &model.MemberNotification{}, &model.BackupLog{},
		&model.EInvoiceConfig{}, &model.EInvoice{},
		&model.ChatRoom{}, &model.ChatParticipant{}, &model.ChatMessage{}, &model.ServiceFeedback{},
		&model.AppUpdate{}, &model.WebsiteBlockingRule{}, &model.WebsiteRuleMapping{},
		&model.WebsiteBlockingSchedule{}, &model.WebsiteBlockingViolation{},
		&model.CurfewPolicy{}, &model.LuckySpinReward{}, &model.LuckySpinLog{},
		&model.TopupCard{}, &model.GiftCard{}, &model.GiftCardTransaction{},
		&model.MemberTransaction{}, &model.MemberAttendance{},
	)
}

func seed(db *gorm.DB) error {
	store := model.Store{Name: "Main Store", Code: "MAIN", Address: "Default", IsActive: true}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Where("code = ?", "MAIN").FirstOrCreate(&store).Error; err != nil {
		return err
	}

	hash, _ := utils.HashPassword("admin123")

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

	perms := []model.Permission{
		{Code: "*", Name: "Full access", Module: "all"},
		{Code: "members.view", Name: "Xem hội viên", Module: "members"},
		{Code: "members.create", Name: "Tạo hội viên", Module: "members"},
		{Code: "members.topup", Name: "Nạp tiền", Module: "members"},
		{Code: "machines.view", Name: "Xem máy", Module: "machines"},
		{Code: "orders.view", Name: "Xem đơn hàng", Module: "orders"},
		{Code: "orders.create", Name: "Tạo đơn hàng", Module: "orders"},
		{Code: "orders.pay", Name: "Thanh toán", Module: "orders"},
		{Code: "reports.view", Name: "Xem báo cáo", Module: "reports"},
		{Code: "settings.edit", Name: "Sửa cài đặt", Module: "settings"},
		{Code: "client.admin", Name: "Admin client", Module: "client"},
	}
	for _, p := range perms {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Where("code = ?", p.Code).FirstOrCreate(&p).Error; err != nil {
			return err
		}
	}

	admin := model.User{
		Username:     "admin",
		PasswordHash: hash,
		FullName:     "Admin",
		StoreID:      &store.ID,
		IsActive:     true,
	}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Where("username = ?", "admin").FirstOrCreate(&admin).Error; err != nil {
		return err
	}

	manager := model.User{
		Username:     "manager",
		PasswordHash: hash,
		FullName:     "Manager",
		StoreID:      &store.ID,
		IsActive:     true,
	}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Where("username = ?", "manager").FirstOrCreate(&manager).Error; err != nil {
		return err
	}

	staff := model.User{
		Username:     "staff",
		PasswordHash: hash,
		FullName:     "Staff",
		StoreID:      &store.ID,
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
	for _, role := range []model.Role{adminRole} {
		db.Model(&role).Association("Permissions").Replace(&allPerms)
	}

	var memberPerms []model.Permission
	db.Where("code IN ?", []string{"members.view", "machines.view", "orders.view", "client.admin"}).Find(&memberPerms)
	db.Model(&staffRole).Association("Permissions").Replace(&memberPerms)

	settings := []model.SystemSetting{
		{GroupName: "billing", Key: "rounding_mode", Value: `{"mode": "round_up", "interval_minutes": 60}`, Description: "Cách làm tròn giờ chơi"},
		{GroupName: "billing", Key: "grace_period_minutes", Value: `{"value": 10}`, Description: "Số phút ân hạn"},
		{GroupName: "billing", Key: "min_billing_unit", Value: `{"minutes": 30}`, Description: "Đơn vị tính tiền tối thiểu"},
		{GroupName: "billing", Key: "no_show_cancel_minutes", Value: `{"value": 30}`, Description: "Tự động hủy booking"},
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

	fmt.Println("Seed data completed!")
	fmt.Println("  Admin: admin / admin123")
	fmt.Println("  Manager: manager / admin123")
	fmt.Println("  Staff: staff / admin123")
	return nil
}
