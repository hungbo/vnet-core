package handler

import (
	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/jwt"
	"gorm.io/gorm"
)

type Handlers struct {
	Auth              *AuthHandler
	Route             *RouteHandler
	Member            *MemberHandler
	Machine           *MachineHandler
	Session           *SessionHandler
	Combo             *ComboHandler
	Booking           *BookingHandler
	Promotion         *PromotionHandler
	Curfew            *CurfewHandler
	Category          *CategoryHandler
	Product           *ProductHandler
	Order             *OrderHandler
	Printer           *PrinterHandler
	Receipt           *ReceiptHandler
	Card              *CardHandler
	InventoryCount    *InventoryCountHandler
	Attendance        *AttendanceHandler
	WebsiteBlock      *WebsiteBlockHandler
	AppUpdate         *AppUpdateHandler
	Feedback          *FeedbackHandler
	Pricing           *PricingHandler
	Inventory         *InventoryHandler
	Shift             *ShiftHandler
	Settings          *SettingsHandler
	Audit             *AuditHandler
	Backup            *BackupHandler
	Chat              *ChatHandler
	Report            *ReportHandler
	SystemManage      *SystemManageHandler
	Upload            *UploadHandler
	Notification      *NotificationHandler
	NotificationAdmin *NotificationAdminHandler
}

func NewHandlers(db *gorm.DB, jwtManager *jwt.Manager, wsHub *hub.Hub, cfg *config.Config) *Handlers {
	auditSvc := service.NewAuditService(db)
	invSvc := service.NewInventoryService(db, auditSvc).WithHub(wsHub)
	chatSvc := service.NewChatService(db, wsHub, auditSvc)
	chatSvc.HubRoomSync()
	curfewSvc := service.NewCurfewService(db, auditSvc)
	// Session starts are gated by curfew, so both handlers share one instance.
	sessionSvc := service.NewSessionService(db, wsHub, auditSvc).WithCurfew(curfewSvc)
	// Một MachineService dùng chung: handler chặn website cũng cần nó để kiểm
	// khoá máy trạm, dựng hai bản là hai nguồn sự thật cho cùng một thứ.
	// Dựng SAU sessionSvc vì tắt máy / khởi động lại từ xa phải chốt được phiên.
	machineSvc := service.NewMachineService(db, wsHub, auditSvc).WithSessions(sessionSvc)
	return &Handlers{Upload: NewUploadHandler(cfg.Server.UploadDir, cfg.Server.MaxFileSize),
		Auth:              NewAuthHandler(service.NewAuthService(db, jwtManager, auditSvc).WithCurfew(curfewSvc).WithSessions(sessionSvc)),
		Route:             NewRouteHandler(service.NewRouteService()),
		Member:            NewMemberHandler(service.NewMemberService(db, auditSvc).WithHub(wsHub)),
		Machine:           NewMachineHandler(machineSvc),
		Session:           NewSessionHandler(sessionSvc),
		Combo:             NewComboHandler(service.NewComboService(db, auditSvc).WithHub(wsHub)),
		Booking:           NewBookingHandler(service.NewBookingService(db, auditSvc).WithHub(wsHub)),
		Promotion:         NewPromotionHandler(service.NewPromotionService(db, auditSvc).WithHub(wsHub)),
		Curfew:            NewCurfewHandler(curfewSvc),
		Category:          NewCategoryHandler(service.NewCategoryService(db, auditSvc)),
		Product:           NewProductHandler(service.NewProductService(db, auditSvc).WithHub(wsHub)),
		Order:             NewOrderHandler(service.NewOrderService(db, wsHub, auditSvc, invSvc)),
		Printer:           NewPrinterHandler(service.NewPrinterService(db, auditSvc)),
		Receipt:           NewReceiptHandler(service.NewReceiptService(db, auditSvc)),
		Card:              NewCardHandler(service.NewCardService(db, auditSvc).WithHub(wsHub)),
		InventoryCount:    NewInventoryCountHandler(service.NewInventoryCountService(db, auditSvc).WithHub(wsHub)),
		Attendance:        NewAttendanceHandler(service.NewAttendanceService(db, auditSvc)),
		WebsiteBlock:      NewWebsiteBlockHandler(service.NewWebsiteBlockService(db, auditSvc)),
		AppUpdate:         NewAppUpdateHandler(service.NewAppUpdateService(db, auditSvc)),
		Feedback:          NewFeedbackHandler(service.NewFeedbackService(db, auditSvc)),
		Pricing:           NewPricingHandler(service.NewPricingService(db, auditSvc)),
		Inventory:         NewInventoryHandler(invSvc),
		Shift:             NewShiftHandler(service.NewShiftService(db, auditSvc)),
		Settings:          NewSettingsHandler(service.NewSettingsService(db, auditSvc)),
		Audit:             NewAuditHandler(auditSvc),
		Backup:            NewBackupHandler(service.NewBackupService(db, &cfg.Database, cfg.Server.BackupDir, auditSvc)),
		Chat:              NewChatHandler(chatSvc),
		Report:            NewReportHandler(service.NewReportService(db)),
		SystemManage:      NewSystemManageHandler(service.NewSystemManageService(db, auditSvc)),
		Notification:      NewNotificationHandler(service.NewNotificationService(db, auditSvc).WithHub(wsHub)),
		NotificationAdmin: NewNotificationAdminHandler(service.NewNotificationAdminService(db, wsHub, auditSvc)),
	}
}
