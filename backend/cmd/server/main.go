// @title           VNET Core API
// @version         1.0
// @description     Hệ thống Quản lý Phòng Game Toàn Diện
// @termsOfService  https://vnet.net/terms

// @contact.name   VNET Support
// @contact.email  support@vnet.net

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:8080
// @BasePath  /api

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	_ "github.com/vnet/core/docs"
	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/database"
	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/internal/router"
	"github.com/vnet/core/pkg/jwt"
)

func main() {
	cfg := config.Load()

	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	db := database.Init(&cfg.Database)

	jwtManager := jwt.New(
		cfg.JWT.Secret,
		cfg.JWT.AccessTokenTTL,
		cfg.JWT.RefreshTokenTTL,
		cfg.JWT.Issuer,
	)

	if err := db.AutoMigrate(
		&model.Store{},
		&model.User{}, &model.Role{}, &model.Permission{},
		&model.UserRole{}, &model.RolePermission{},
		&model.Member{}, &model.MemberGroup{}, &model.MemberTransaction{}, &model.MemberAttendance{},
		&model.Machine{}, &model.MachineGroup{}, &model.MachinePrice{}, &model.TimeBasedPricing{},
		&model.MachineAsset{}, &model.MachineHardwareSnapshot{},
		&model.MachineSession{},
		&model.Combo{}, &model.ComboItem{}, &model.ComboPurchase{},
		&model.TopupCard{}, &model.GiftCard{}, &model.GiftCardTransaction{},
		&model.MachineBooking{},
		&model.Promotion{}, &model.PromotionCondition{}, &model.PromotionReward{},
		&model.LuckySpinReward{}, &model.LuckySpinLog{},
		&model.CurfewPolicy{},
		&model.Category{},
		&model.Product{}, &model.ProductIngredient{}, &model.ProductOptionGroup{}, &model.ProductOption{},
		&model.Order{}, &model.OrderItem{}, &model.Payment{},
		&model.PrinterConfig{}, &model.ProductPrinterMapping{},
		&model.Supplier{}, &model.Warehouse{}, &model.StockTransaction{}, &model.InventoryCount{},
		&model.Shift{}, &model.CashHandover{},
		&model.ChatConversation{}, &model.ChatParticipant{}, &model.ChatMessage{}, &model.ServiceFeedback{},
		&model.SystemSetting{}, &model.AuditLog{},
		&model.Notification{}, &model.NotificationRecipient{},
		&model.MemberNotification{},
		&model.BackupLog{}, &model.EInvoiceConfig{}, &model.EInvoice{},
		&model.WebsiteBlockingRule{}, &model.WebsiteRuleMapping{},
		&model.WebsiteBlockingSchedule{}, &model.WebsiteBlockingViolation{},
		&model.AppUpdate{},
	); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Database migrations completed")

	wsHub := hub.New()

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORS(cfg.Server.AllowedOrigins))
	r.Use(middleware.Logger())

	router.Register(r, db, jwtManager, wsHub, cfg)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("VNET Core server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
