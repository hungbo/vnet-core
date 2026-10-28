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
	WindowsKey        *WindowsKeyHandler
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
	InventoryCount    *InventoryCountHandler
	Attendance        *AttendanceHandler
	WebsiteBlock      *WebsiteBlockHandler
	AppUpdate         *AppUpdateHandler
	Game              *GameHandler
	Feedback          *FeedbackHandler
	Pricing           *PricingHandler
	Inventory         *InventoryHandler
	Shift             *ShiftHandler
	Settings          *SettingsHandler
	Audit             *AuditHandler
	ErrorReport       *ErrorReportHandler
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
	service.TheoDoiMayTatKhiConDon(db, wsHub)
	curfewSvc := service.NewCurfewService(db, auditSvc)
	// Session starts are gated by curfew, so both handlers share one instance.
	sessionSvc := service.NewSessionService(db, wsHub, auditSvc).WithCurfew(curfewSvc).WithMinCharge()
	// gameSvc dùng chung với handler Game (main.go chạy chính bản đó): nhịp tim máy trạm
	// lấy nguồn metadata Riot mặc định của quán từ nó.
	gameSvc := service.NewGameService(db, cfg.Game).WithHub(wsHub)
	// Một MachineService dùng chung: handler chặn website cũng cần nó để kiểm
	// khoá máy trạm, dựng hai bản là hai nguồn sự thật cho cùng một thứ.
	// Dựng SAU sessionSvc vì tắt máy / khởi động lại từ xa phải chốt được phiên.
	machineSvc := service.NewMachineService(db, wsHub, auditSvc).WithSessions(sessionSvc).WithRiotSource(gameSvc.RiotMetadataSource)
	// Bán combo có món tặng kèm ghi một đơn 0đ và trừ kho qua chính bản này.
	orderSvc := service.NewOrderService(db, wsHub, auditSvc, invSvc)
	return &Handlers{Upload: NewUploadHandler(cfg.Server.UploadDir, cfg.Server.MaxFileSize),
		Auth:              NewAuthHandler(service.NewAuthService(db, jwtManager, auditSvc).WithCurfew(curfewSvc).WithSessions(sessionSvc).WithHub(wsHub)),
		Route:             NewRouteHandler(service.NewRouteService()),
		Member:            NewMemberHandler(service.NewMemberService(db, auditSvc).WithHub(wsHub)),
		Machine:           NewMachineHandler(machineSvc).WithHub(wsHub),
		WindowsKey:        NewWindowsKeyHandler(service.NewWindowsKeyService(db, auditSvc).WithHub(wsHub)),
		Session:           NewSessionHandler(sessionSvc),
		Combo:             NewComboHandler(service.NewComboService(db, auditSvc).WithHub(wsHub).WithOrders(orderSvc)),
		Booking:           NewBookingHandler(service.NewBookingService(db, auditSvc).WithHub(wsHub)),
		Promotion:         NewPromotionHandler(service.NewPromotionService(db, auditSvc).WithHub(wsHub)),
		Curfew:            NewCurfewHandler(curfewSvc),
		Category:          NewCategoryHandler(service.NewCategoryService(db, auditSvc)),
		Product:           NewProductHandler(service.NewProductService(db, auditSvc).WithHub(wsHub)),
		Order:             NewOrderHandler(orderSvc),
		Printer:           NewPrinterHandler(service.NewPrinterService(db, auditSvc)),
		Receipt:           NewReceiptHandler(service.NewReceiptService(db, auditSvc)),
		InventoryCount:    NewInventoryCountHandler(service.NewInventoryCountService(db, auditSvc).WithHub(wsHub)),
		Attendance:        NewAttendanceHandler(service.NewAttendanceService(db, auditSvc).WithHub(wsHub)),
		WebsiteBlock:      NewWebsiteBlockHandler(service.NewWebsiteBlockService(db, auditSvc)),
		AppUpdate:         NewAppUpdateHandler(service.NewAppUpdateService(db, auditSvc)),
		Game:              NewGameHandler(gameSvc),
		Feedback:          NewFeedbackHandler(service.NewFeedbackService(db, auditSvc)),
		Pricing:           NewPricingHandler(service.NewPricingService(db, auditSvc)),
		Inventory:         NewInventoryHandler(invSvc),
		Shift:             NewShiftHandler(service.NewShiftService(db, auditSvc)),
		Settings:          NewSettingsHandler(service.NewSettingsService(db, auditSvc)),
		Audit:             NewAuditHandler(auditSvc),
		ErrorReport:       NewErrorReportHandler(service.NewErrorReportService(db)),
		Backup:            NewBackupHandler(service.NewBackupService(db, &cfg.Database, cfg.Server.BackupDir, auditSvc).WithHub(wsHub)),
		Chat:              NewChatHandler(chatSvc),
		Report:            NewReportHandler(service.NewReportService(db)),
		SystemManage:      NewSystemManageHandler(service.NewSystemManageService(db, auditSvc)),
		Notification:      NewNotificationHandler(service.NewNotificationService(db, auditSvc).WithHub(wsHub)),
		NotificationAdmin: NewNotificationAdminHandler(service.NewNotificationAdminService(db, wsHub, auditSvc)),
	}
}
