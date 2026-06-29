package handler

import (
	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/jwt"
	"gorm.io/gorm"
)

type Handlers struct {
	Auth          *AuthHandler
	Route         *RouteHandler
	Member        *MemberHandler
	Machine       *MachineHandler
	Session       *SessionHandler
	Combo         *ComboHandler
	Booking       *BookingHandler
	Promotion     *PromotionHandler
	Curfew        *CurfewHandler
	Category      *CategoryHandler
	Product       *ProductHandler
	Order         *OrderHandler
	Printer       *PrinterHandler
	Inventory     *InventoryHandler
	Shift         *ShiftHandler
	Store         *StoreHandler
	Settings      *SettingsHandler
	Audit         *AuditHandler
	Backup        *BackupHandler
	Chat          *ChatHandler
	Report        *ReportHandler
	SystemManage  *SystemManageHandler
	Upload        *UploadHandler
	Notification  *NotificationHandler
}

func NewHandlers(db *gorm.DB, jwtManager *jwt.Manager, wsHub *hub.Hub, cfg *config.Config) *Handlers {
	auditSvc := service.NewAuditService(db)
	invSvc := service.NewInventoryService(db, auditSvc)
	chatSvc := service.NewChatService(db, wsHub, auditSvc)
	chatSvc.HubRoomSync()
	return &Handlers{Upload: NewUploadHandler(cfg.Server.UploadDir, cfg.Server.MaxFileSize),
		Auth:      NewAuthHandler(service.NewAuthService(db, jwtManager, auditSvc), service.NewSessionService(db, wsHub, auditSvc)),
		Route:     NewRouteHandler(service.NewRouteService()),
		Member:    NewMemberHandler(service.NewMemberService(db, auditSvc)),
		Machine:   NewMachineHandler(service.NewMachineService(db, wsHub, auditSvc)),
		Session:   NewSessionHandler(service.NewSessionService(db, wsHub, auditSvc)),
		Combo:     NewComboHandler(service.NewComboService(db, auditSvc)),
		Booking:   NewBookingHandler(service.NewBookingService(db, auditSvc)),
		Promotion: NewPromotionHandler(service.NewPromotionService(db, auditSvc)),
		Curfew:    NewCurfewHandler(service.NewCurfewService(db, auditSvc)),
		Category:  NewCategoryHandler(service.NewCategoryService(db, auditSvc)),
		Product:   NewProductHandler(service.NewProductService(db, auditSvc)),
		Order:     NewOrderHandler(service.NewOrderService(db, wsHub, auditSvc, invSvc)),
		Printer:   NewPrinterHandler(service.NewPrinterService(db, auditSvc)),
		Inventory: NewInventoryHandler(invSvc),
		Shift:     NewShiftHandler(service.NewShiftService(db, auditSvc)),
		Store:     NewStoreHandler(service.NewStoreService(db, auditSvc)),
		Settings:  NewSettingsHandler(service.NewSettingsService(db, auditSvc)),
		Audit:     NewAuditHandler(auditSvc),
		Backup:    NewBackupHandler(service.NewBackupService(db, auditSvc)),
		Chat:      NewChatHandler(chatSvc),
		Report:       NewReportHandler(service.NewReportService(db)),
		SystemManage: NewSystemManageHandler(service.NewSystemManageService(db, auditSvc)),
		Notification: NewNotificationHandler(service.NewNotificationService(db, auditSvc)),
	}
}
