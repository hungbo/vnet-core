// @title           VNET Core API
// @version         1.0
// @description     Hệ thống Quản lý Phòng Game Toàn Diện
// @termsOfService  https://vnet.net/terms

// @contact.name   VNET Support
// @contact.email  support@vnet.net

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:20800
// @BasePath  /api

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/vnet/core/docs"
	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/database"
	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/internal/router"
	"github.com/vnet/core/internal/scheduler"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/jwt"
	"github.com/vnet/core/pkg/response"
)

func main() {
	// Lấy trước mọi thứ khác: chạy dưới dạng dịch vụ Windows thì đây là nơi
	// log được chuyển sang tệp, và SCM phải nhận được tín hiệu đã chạy trong
	// vòng 30 giây kể cả khi database còn đang khởi động.
	ctx, finished := lifetime()
	defer finished()

	cfg := config.Load()

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Refusing to start: %v", err)
	}

	if cfg.Server.Mode == config.ModeRelease {
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
		&model.User{}, &model.Role{}, &model.Permission{},
		&model.UserRole{}, &model.RolePermission{},
		&model.Member{}, &model.MemberGroup{}, &model.MemberTransaction{}, &model.MemberAttendance{},
		&model.Machine{}, &model.MachineGroup{},
		&model.MachinePrice{}, &model.TimeBasedPricing{},
		&model.MachineAsset{}, &model.MachineHardwareSnapshot{},
		&model.MachineSession{},
		&model.Combo{}, &model.ComboPurchase{},
		&model.MachineBooking{},
		&model.Promotion{}, &model.PromotionCondition{}, &model.PromotionReward{},
		&model.LuckySpinReward{}, &model.LuckySpinLog{},
		&model.CurfewPolicy{},
		&model.Category{},
		&model.Product{}, &model.ProductIngredient{}, &model.ProductOptionGroup{}, &model.ProductOption{},
		&model.Order{}, &model.OrderItem{}, &model.Payment{},
		&model.PrinterConfig{}, &model.ProductPrinterMapping{},
		&model.Supplier{}, &model.StockTransaction{}, &model.InventoryCountSession{}, &model.InventoryCount{},
		&model.Shift{}, &model.CashHandover{},
		&model.ChatRoom{}, &model.ChatParticipant{}, &model.ChatMessage{}, &model.ServiceFeedback{},
		&model.SystemSetting{}, &model.AuditLog{}, &model.ErrorEvent{},
		&model.Notification{}, &model.NotificationRecipient{},
		&model.MemberNotification{},
		&model.BackupLog{}, &model.EInvoiceConfig{}, &model.EInvoice{},
		&model.WebsiteBlockingRule{}, &model.WebsiteRuleMapping{},
		&model.WebsiteBlockingSchedule{}, &model.WebsiteBlockingViolation{},
		&model.AppUpdate{},
		&model.IdempotencyKey{},
		&model.Game{}, &model.GamePublish{},
	); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Database migrations completed")

	wsHub := hub.New(cfg.Server.AllowedOrigins)

	r := gin.New()
	// RequestID trước để Logger và Recovery đều thấy mã; Logger bọc ngoài
	// Recovery để yêu cầu bị panic vẫn có dòng log "500 ... id=...".
	r.Use(middleware.RequestID())
	r.Use(middleware.CORS(cfg.Server.AllowedOrigins))
	r.Use(middleware.Logger())
	errReports := service.NewErrorReportService(db)
	r.Use(middleware.Recovery(func(c *gin.Context, kind, message, detail string) {
		in := service.ErrorReportInput{
			Source:    service.ErrorSourceServer,
			Kind:      kind,
			Message:   message,
			Detail:    detail,
			Location:  c.Request.Method + " " + c.FullPath(),
			Reporter:  c.GetString(middleware.ContextKeyUsername),
			RequestID: c.GetString(response.RequestIDKey),
		}
		// Ghi ngoài luồng yêu cầu: lỗi 500 hay do chính database, chờ nó thì
		// yêu cầu nào đang lỗi cũng treo thêm đến hết thời gian chờ kết nối.
		go func() {
			if err := errReports.Record(in); err != nil {
				log.Printf("[error-report] không ghi được lỗi id=%s: %v", in.RequestID, err)
				return
			}
			// Yêu cầu lỗi 500 không phát "<chủ đề>:changed" (middleware chỉ phát khi
			// thành công), nên trang Nhật ký lỗi phải được báo ở đây. Trừ khi chính
			// danh sách lỗi hỏng: báo thì trang tải lại, lại hỏng, lại báo — vòng lặp.
			// Và tối đa một lần mỗi 15 giây: trang nào khác nghe "error-reports" mà
			// lúc tải lại cũng gặp lỗi 500 thì vòng lặp đó chậm lại thành một nhịp
			// thưa, không dội máy chủ.
			if in.Location != "GET /api/error-reports" && thuaBaoLoi.duocBao(time.Now()) {
				wsHub.Changed("error-reports", nil)
			}
		}()
	}))

	h := router.Register(r, db, jwtManager, wsHub, cfg)
	router.RegisterAdminUI(r, adminAssets())

	// Background work needs a lifetime of its own (ctx, from lifetime above),
	// which is why the server is built explicitly instead of using gin's r.Run:
	// jobs must be told to stop and be waited on before the process exits.
	auditSvc := service.NewAuditService(db)
	curfewSvc := service.NewCurfewService(db, auditSvc)
	sessionSvc := service.NewSessionService(db, wsHub, auditSvc).WithCurfew(curfewSvc).WithMinCharge()
	memberSvc := service.NewMemberService(db, auditSvc)

	// Phần đã chơi trước khi bản trừ-tiền-theo-phút được triển khai coi như đã
	// thu: nếu không, lượt tính đầu tiên trừ một cục toàn bộ thời gian và có thể
	// đá khách đang ngồi ra khỏi máy.
	if n, err := sessionSvc.BackfillChargedAmount(); err != nil {
		log.Printf("backfill charged_amount: %v", err)
	} else if n > 0 {
		log.Printf("đánh dấu %d phiên đang chạy là đã thu tới thời điểm triển khai", n)
	}

	sched := scheduler.New()
	scheduler.Register(sched, db, wsHub, sessionSvc, curfewSvc, memberSvc, cfg.Server.HardwareHistoryDays)
	sched.Start(ctx)

	// Cập nhật game chạy nền cho tới khi tắt máy chủ; không làm gì nếu GAME_ROLE trống.
	gameDone := make(chan struct{})
	go func() {
		defer close(gameDone)
		h.Game.Service().Run(ctx)
	}()

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		log.Printf("VNET Core server starting on %s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down…")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown: %v", err)
	}
	sched.Wait()
	// Đợi vòng cập nhật game dừng (đóng engine BitTorrent), nhưng không đợi mãi: một lần băm hay
	// kiểm tra hàng chục GB đang dở không huỷ được giữa chừng, và dịch vụ không được kẹt lúc tắt.
	select {
	case <-gameDone:
	case <-time.After(30 * time.Second):
		log.Println("Cập nhật game chưa dừng sau 30 giây, bỏ qua")
	}
	log.Println("Stopped")
}

// nhipThua cho qua tối đa một lần trong mỗi quãng (an toàn khi gọi từ nhiều goroutine).
type nhipThua struct {
	quang time.Duration
	luc   atomic.Int64 // UnixNano lần cuối cho qua; 0 = chưa lần nào
}

func (n *nhipThua) duocBao(now time.Time) bool {
	last := n.luc.Load()
	if last != 0 && now.Sub(time.Unix(0, last)) < n.quang {
		return false
	}
	return n.luc.CompareAndSwap(last, now.UnixNano())
}

// thuaBaoLoi: nhịp báo trang quản trị khi máy chủ ghi một lỗi 500.
var thuaBaoLoi = &nhipThua{quang: 15 * time.Second}
