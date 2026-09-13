package router

import (
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/handler"
	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/pkg/jwt"
	"github.com/vnet/core/pkg/response"
	"gorm.io/gorm"
)

// RegisterAdminUI serves the embedded admin build. The SPA uses HTML5 history
// routing, so any unmatched path that is not an API call falls back to
// index.html and lets the client router resolve it.
func RegisterAdminUI(r *gin.Engine, assets fs.FS) {
	if assets == nil {
		return
	}

	fileServer := http.FileServer(http.FS(assets))

	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path

		// API, uploads and docs must keep returning their own errors instead of
		// being swallowed by the SPA fallback.
		if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/uploads/") || strings.HasPrefix(p, "/swagger/") {
			response.NotFound(c, "endpoint not found")
			return
		}

		if f, err := assets.Open(strings.TrimPrefix(p, "/")); err == nil {
			f.Close()
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		index, err := assets.Open("index.html")
		if err != nil {
			response.NotFound(c, "admin UI not available")
			return
		}
		defer index.Close()

		data, err := io.ReadAll(index)
		if err != nil {
			response.InternalError(c, "failed to read admin UI")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})
}

func Register(r *gin.Engine, db *gorm.DB, jwtManager *jwt.Manager, wsHub *hub.Hub, cfg *config.Config) {
	h := handler.NewHandlers(db, jwtManager, wsHub, cfg)

	uploadsDir := cfg.Server.UploadDir
	if _, err := os.Stat(uploadsDir); os.IsNotExist(err) {
		os.MkdirAll(uploadsDir, 0755)
	}
	r.Static("/uploads", uploadsDir)

	// The API explorer documents every endpoint; it stays off in production.
	if cfg.Server.Mode != config.ModeRelease {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	api := r.Group("/api")
	// Phản hồi API không bao giờ được tầng trung gian dùng lại: thiếu header này
	// thì danh sách vừa đổi vẫn có thể trả về bản cũ.
	api.Use(middleware.NoCache())
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"code":    0,
				"message": "success",
				"data":    gin.H{"status": "ok"},
			})
		})

		route := api.Group("/route")
		{
			route.GET("/getConstantRoutes", h.Route.GetConstantRoutes)
		}

		auth := api.Group("/auth")
		{
			auth.POST("/login", h.Auth.Login)
			auth.POST("/refresh", h.Auth.Refresh)
			auth.POST("/qr-login", h.Auth.QRLogin)
			auth.POST("/member-login", h.Auth.MemberLogin)
			// Một ô đăng nhập duy nhất cho máy trạm: tên tài khoản quyết định
			// nhân viên hay hội viên. Hai route trên vẫn giữ cho trang quản trị
			// và cho những máy trạm bản cũ chưa cập nhật.
			auth.POST("/client-login", h.Auth.ClientLogin)
		}

		// Ba route máy trạm gọi mà không có tài khoản người dùng. Tất cả đều
		// nhận diện bằng mã máy trong URL (khoá máy trạm đã bỏ).
		api.POST("/machines/by-code/:code/heartbeat", h.Machine.HeartbeatByCode)
		api.GET("/machines/by-code/:code/blocklist", h.WebsiteBlock.EffectiveForMachine)
		api.POST("/machines/by-code/:code/blocklist/violations", h.WebsiteBlock.ReportViolation)
		api.GET("/machines/by-code/:code/app-update", h.AppUpdate.Latest)
		// Máy trạm báo kết quả lệnh giám sát về: ảnh chụp và danh sách tiến
		// trình. Đường lên bằng HTTP chứ không WebSocket vì readPump của hub
		// giới hạn 4 KB, còn ảnh cỡ vài trăm KB.
		api.POST("/machines/by-code/:code/screenshot", h.Machine.ReportScreenshot)
		api.POST("/machines/by-code/:code/processes", h.Machine.ReportProcesses)

		// WebSocket nằm NGOÀI nhóm protected: nó nhận thêm một cách vào thứ hai —
		// khoá riêng của máy trạm — để tiến trình nền giữ được kết nối kể cả khi
		// không có ai đăng nhập trên máy đó.
		api.GET("/ws/client",
			middleware.AuthOrAgent(jwtManager),
			wsHub.HandleWS)

		protected := api.Group("")
		protected.Use(middleware.AuthRequired(jwtManager))
		{
			protected.POST("/upload", middleware.StaffOnly(), middleware.PermissionRequired("upload.file"), h.Upload.Upload)
			protectedRoute := protected.Group("/route")
			{
				protectedRoute.GET("/getUserRoutes", middleware.StaffOnly(), h.Route.GetUserRoutes)
			}
			protectedAuth := protected.Group("/auth")
			{
				protectedAuth.GET("/me", h.Auth.Me)
				protectedAuth.PUT("/change-password", h.Auth.ChangePassword)
				protectedAuth.GET("/permissions", middleware.StaffOnly(), h.Auth.GetPermissions)
			}

			members := protected.Group("/members")
			{
				members.GET("", middleware.StaffOnly(), middleware.PermissionRequired("members.view"), h.Member.List)
				members.POST("/refresh-tiers", middleware.StaffOnly(), middleware.PermissionRequired("members.refresh_tiers"), h.Member.RefreshTiers)
				members.POST("", middleware.StaffOnly(), middleware.PermissionRequired("members.create"), h.Member.Create)
				members.PUT("/:id", middleware.StaffOnly(), middleware.PermissionRequired("members.update"), h.Member.Update)
				members.DELETE("/:id", middleware.StaffOnly(), middleware.PermissionRequired("members.delete"), h.Member.Delete)
				members.POST("/:id/reset-password", middleware.StaffOnly(), middleware.PermissionRequired("members.reset_password"), h.Member.ResetPassword)
				members.POST("/:id/topup", middleware.StaffOnly(), middleware.PermissionRequired("members.topup"), h.Member.Topup)
				members.POST("/:id/refund", middleware.StaffOnly(), middleware.PermissionRequired("members.topup"), h.Member.Refund)

				// The desktop client reads the signed-in member's own profile,
				// balance history and sessions through these.
				members.GET("/:id", middleware.SelfOrStaff("id"), h.Member.GetByID)
				members.GET("/:id/transactions", middleware.SelfOrStaff("id"), h.Member.GetTransactions)
				members.GET("/:id/sessions", middleware.SelfOrStaff("id"), h.Member.GetSessions)
				members.GET("/:id/combos", middleware.SelfOrStaff("id"), h.Member.GetCombos)
			}

			memberGroups := protected.Group("/member-groups", middleware.StaffOnly())
			{
				memberGroups.GET("", middleware.PermissionRequired("member_groups.view"), h.Member.ListGroups)
				memberGroups.POST("", middleware.PermissionRequired("member_groups.create"), h.Member.CreateGroup)
				memberGroups.PUT("/:id", middleware.PermissionRequired("member_groups.update"), h.Member.UpdateGroup)
				memberGroups.DELETE("/:id", middleware.PermissionRequired("member_groups.delete"), h.Member.DeleteGroup)
			}

			machines := protected.Group("/machines")
			{
				machines.GET("/by-code/:code", h.Machine.GetByCode)

				machines.GET("", middleware.StaffOnly(), middleware.PermissionRequired("machines.view"), h.Machine.List)
				machines.GET("/:id", middleware.StaffOnly(), middleware.PermissionRequired("machines.view"), h.Machine.GetByID)
				machines.POST("", middleware.StaffOnly(), middleware.PermissionRequired("machines.create"), h.Machine.Create)
				machines.POST("/batch", middleware.StaffOnly(), middleware.PermissionRequired("machines.create"), h.Machine.BatchCreate)
				machines.PUT("/:id", middleware.StaffOnly(), middleware.PermissionRequired("machines.update"), h.Machine.Update)
				machines.DELETE("/:id", middleware.StaffOnly(), middleware.PermissionRequired("machines.delete"), h.Machine.Delete)
				machines.POST("/:id/heartbeat", middleware.StaffOnly(), middleware.PermissionRequired("machines.update"), h.Machine.Heartbeat)
				machines.GET("/:id/hardware", middleware.StaffOnly(), middleware.PermissionRequired("machines.view"), h.Machine.GetHardware)
				machines.POST("/:id/remote/:action", middleware.StaffOnly(), middleware.PermissionRequired("machines.remote"), h.Machine.RemoteAction)
			}

			machineGroups := protected.Group("/machine-groups", middleware.StaffOnly())
			{
				machineGroups.GET("", middleware.PermissionRequired("machine_groups.view"), h.Machine.ListGroups)
				machineGroups.POST("", middleware.PermissionRequired("machine_groups.create"), h.Machine.CreateGroup)
				machineGroups.PUT("/:id", middleware.PermissionRequired("machine_groups.update"), h.Machine.UpdateGroup)
				machineGroups.DELETE("/:id", middleware.PermissionRequired("machine_groups.delete"), h.Machine.DeleteGroup)
			}

			machineAssets := protected.Group("/machine-assets", middleware.StaffOnly())
			{
				machineAssets.GET("", middleware.PermissionRequired("machine_assets.view"), h.Machine.ListAssets)
				machineAssets.POST("", middleware.PermissionRequired("machine_assets.create"), h.Machine.CreateAsset)
				machineAssets.PUT("/:id", middleware.PermissionRequired("machine_assets.update"), h.Machine.UpdateAsset)
				machineAssets.DELETE("/:id", middleware.PermissionRequired("machine_assets.delete"), h.Machine.DeleteAsset)
			}

			sessions := protected.Group("/sessions")
			{
				sessions.GET("/me", h.Session.GetMySession)

				sessions.GET("/active", middleware.StaffOnly(), middleware.PermissionRequired("sessions.view"), h.Session.ListActive)
				sessions.POST("/start", middleware.StaffOnly(), middleware.PermissionRequired("sessions.start"), h.Session.Start)
				sessions.POST("/:id/end", middleware.StaffOnly(), middleware.PermissionRequired("sessions.end"), h.Session.End)
				sessions.GET("/:id", middleware.StaffOnly(), middleware.PermissionRequired("sessions.view"), h.Session.Get)
				sessions.POST("/:id/switch-machine", middleware.StaffOnly(), middleware.PermissionRequired("sessions.switch"), h.Session.SwitchMachine)
				sessions.GET("/calculate-cost", middleware.StaffOnly(), middleware.PermissionRequired("sessions.view"), h.Session.CalculateCost)
			}

			combos := protected.Group("/combos", middleware.StaffOnly())
			{
				combos.GET("", middleware.PermissionRequired("combos.view"), h.Combo.List)
				combos.GET("/:id", middleware.PermissionRequired("combos.view"), h.Combo.GetByID)
				combos.POST("", middleware.PermissionRequired("combos.create"), h.Combo.Create)
				combos.PUT("/:id", middleware.PermissionRequired("combos.update"), h.Combo.Update)
				combos.DELETE("/:id", middleware.PermissionRequired("combos.delete"), h.Combo.Delete)
				combos.POST("/:id/purchase", middleware.PermissionRequired("combos.sell"), h.Combo.Purchase)
				combos.POST("/:id/activate", middleware.PermissionRequired("combos.sell"), h.Combo.Activate)
			}

			bookings := protected.Group("/bookings", middleware.StaffOnly())
			{
				bookings.GET("", middleware.PermissionRequired("bookings.view"), h.Booking.List)
				bookings.GET("/:id", middleware.PermissionRequired("bookings.view"), h.Booking.GetByID)
				bookings.POST("", middleware.PermissionRequired("bookings.create"), h.Booking.Create)
				bookings.PUT("/:id", middleware.PermissionRequired("bookings.update"), h.Booking.Update)
				bookings.DELETE("/:id", middleware.PermissionRequired("bookings.delete"), h.Booking.Delete)
				bookings.POST("/:id/check-in", middleware.PermissionRequired("bookings.checkin"), h.Booking.CheckIn)
				bookings.POST("/:id/cancel", middleware.PermissionRequired("bookings.update"), h.Booking.Cancel)
				bookings.POST("/:id/no-show", middleware.PermissionRequired("bookings.update"), h.Booking.NoShow)
			}

			promotions := protected.Group("/promotions", middleware.StaffOnly())
			{
				promotions.GET("", middleware.PermissionRequired("promotions.view"), h.Promotion.List)
				promotions.GET("/:id", middleware.PermissionRequired("promotions.view"), h.Promotion.GetByID)
				promotions.POST("", middleware.PermissionRequired("promotions.create"), h.Promotion.Create)
				promotions.PUT("/:id", middleware.PermissionRequired("promotions.update"), h.Promotion.Update)
				promotions.DELETE("/:id", middleware.PermissionRequired("promotions.delete"), h.Promotion.Delete)
			}

			luckySpin := protected.Group("/lucky-spin", middleware.StaffOnly())
			{
				luckySpin.GET("/rewards", middleware.PermissionRequired("lucky_spin.view"), h.Promotion.GetLuckySpinRewards)
				luckySpin.POST("/rewards", middleware.PermissionRequired("lucky_spin.create"), h.Promotion.CreateLuckySpinReward)
				luckySpin.PUT("/rewards/:id", middleware.PermissionRequired("lucky_spin.update"), h.Promotion.UpdateLuckySpinReward)
				luckySpin.DELETE("/rewards/:id", middleware.PermissionRequired("lucky_spin.delete"), h.Promotion.DeleteLuckySpinReward)
				luckySpin.POST("/spin", middleware.PermissionRequired("lucky_spin.spin"), h.Promotion.Spin)
			}

			curfew := protected.Group("/curfew", middleware.StaffOnly())
			{
				curfew.GET("", middleware.PermissionRequired("curfew.view"), h.Curfew.List)
				curfew.GET("/:id", middleware.PermissionRequired("curfew.view"), h.Curfew.GetByID)
				curfew.POST("", middleware.PermissionRequired("curfew.create"), h.Curfew.Create)
				curfew.PUT("/:id", middleware.PermissionRequired("curfew.update"), h.Curfew.Update)
				curfew.DELETE("/:id", middleware.PermissionRequired("curfew.delete"), h.Curfew.Delete)
				curfew.POST("/override", middleware.PermissionRequired("curfew.override"), h.Curfew.Override)
			}

			categories := protected.Group("/categories")
			{
				categories.GET("", h.Category.List)
				categories.GET("/:id", h.Category.GetByID)

				categories.POST("", middleware.StaffOnly(), middleware.PermissionRequired("categories.create"), h.Category.Create)
				categories.PUT("/:id", middleware.StaffOnly(), middleware.PermissionRequired("categories.update"), h.Category.Update)
				categories.DELETE("/:id", middleware.StaffOnly(), middleware.PermissionRequired("categories.delete"), h.Category.Delete)
			}

			products := protected.Group("/products")
			{
				products.GET("", h.Product.List)
				products.GET("/:id", h.Product.GetByID)

				products.POST("", middleware.StaffOnly(), middleware.PermissionRequired("products.create"), h.Product.Create)
				products.PUT("/:id", middleware.StaffOnly(), middleware.PermissionRequired("products.update"), h.Product.Update)
				products.DELETE("/:id", middleware.StaffOnly(), middleware.PermissionRequired("products.delete"), h.Product.Delete)
				products.GET("/:id/ingredients", middleware.StaffOnly(), middleware.PermissionRequired("products.view"), h.Inventory.ListProductIngredients)
				products.POST("/:id/ingredients", middleware.StaffOnly(), middleware.PermissionRequired("products.ingredients"), h.Inventory.CreateProductIngredient)
				products.PUT("/:id/ingredients/:ingredientId", middleware.StaffOnly(), middleware.PermissionRequired("products.ingredients"), h.Inventory.UpdateProductIngredient)
				products.DELETE("/:id/ingredients/:ingredientId", middleware.StaffOnly(), middleware.PermissionRequired("products.ingredients"), h.Inventory.DeleteProductIngredient)
			}

			orders := protected.Group("/orders")
			{
				orders.POST("", h.Order.Create)
				orders.POST("/topup-request", h.Order.CreateTopup)

				orders.GET("", middleware.StaffOnly(), middleware.PermissionRequired("orders.view"), h.Order.List)
				orders.GET("/:id", middleware.StaffOnly(), middleware.PermissionRequired("orders.view"), h.Order.GetByID)
				orders.PUT("/:id", middleware.StaffOnly(), middleware.PermissionRequired("orders.update"), h.Order.Update)
				orders.DELETE("/batch-delete", middleware.StaffOnly(), middleware.PermissionRequired("orders.delete"), h.Order.BatchDelete)
				orders.DELETE("/:id", middleware.StaffOnly(), middleware.PermissionRequired("orders.delete"), h.Order.Delete)
				orders.POST("/:id/status", middleware.StaffOnly(), middleware.PermissionRequired("orders.status"), h.Order.UpdateStatus)
				orders.POST("/:id/split", middleware.StaffOnly(), middleware.PermissionRequired("orders.split"), h.Order.Split)
				orders.POST("/:id/items/:itemId/status", middleware.StaffOnly(), middleware.PermissionRequired("orders.status"), h.Order.UpdateItemStatus)
				orders.POST("/:id/pay", middleware.StaffOnly(), middleware.PermissionRequired("orders.pay"), h.Order.Pay)
				// In hoá đơn và phiếu chế biến. StaffOnly: hội viên không được
				// bắt máy in ở quầy nhả giấy.
				orders.POST("/:id/print", middleware.StaffOnly(), middleware.PermissionRequired("orders.print"), h.Receipt.PrintReceipt)
				orders.POST("/:id/print-stations", middleware.StaffOnly(), middleware.PermissionRequired("orders.print"), h.Receipt.PrintStations)
				orders.GET("/:id/receipt-preview", middleware.StaffOnly(), middleware.PermissionRequired("orders.print"), h.Receipt.PreviewReceipt)
			}

			// Thẻ nạp: sinh/liệt kê/huỷ là việc của nhân viên. Riêng "nạp thẻ"
			// hội viên tự làm được từ máy trạm — handler ép member_id về chính
			// người gọi nên không nạp hộ tài khoản khác được.
			topupCards := protected.Group("/topup-cards")
			{
				topupCards.POST("/redeem", h.Card.RedeemTopupCard)
				topupCards.GET("", middleware.StaffOnly(), middleware.PermissionRequired("topup_cards.view"), h.Card.ListTopupCards)
				topupCards.POST("/generate", middleware.StaffOnly(), middleware.PermissionRequired("topup_cards.generate"), h.Card.GenerateTopupCards)
				topupCards.POST("/:id/cancel", middleware.StaffOnly(), middleware.PermissionRequired("topup_cards.cancel"), h.Card.CancelTopupCard)
				topupCards.POST("/:id/sell", middleware.StaffOnly(), middleware.PermissionRequired("topup_cards.sell"), h.Card.SellTopupCard)
			}

			giftCards := protected.Group("/gift-cards")
			{
				giftCards.POST("/check", h.Card.CheckGiftCard)
				giftCards.GET("", middleware.StaffOnly(), middleware.PermissionRequired("gift_cards.view"), h.Card.ListGiftCards)
				giftCards.POST("/generate", middleware.StaffOnly(), middleware.PermissionRequired("gift_cards.generate"), h.Card.GenerateGiftCards)
				giftCards.POST("/:id/cancel", middleware.StaffOnly(), middleware.PermissionRequired("gift_cards.cancel"), h.Card.CancelGiftCard)
			}

			// Điểm danh: hội viên tự làm được từ máy trạm; handler ép member_id
			// về chính người gọi nên không điểm danh hộ người khác được.
			appUpdates := protected.Group("/app-updates", middleware.StaffOnly())
			{
				appUpdates.GET("", middleware.PermissionRequired("app_updates.view"), h.AppUpdate.List)
				appUpdates.POST("", middleware.PermissionRequired("app_updates.create"), h.AppUpdate.Create)
				appUpdates.PUT("/:id/active", middleware.PermissionRequired("app_updates.update"), h.AppUpdate.SetActive)
				appUpdates.DELETE("/:id", middleware.PermissionRequired("app_updates.delete"), h.AppUpdate.Delete)
			}

			websiteRules := protected.Group("/website-rules", middleware.StaffOnly())
			{
				websiteRules.GET("", middleware.PermissionRequired("website_rules.view"), h.WebsiteBlock.ListRules)
				websiteRules.GET("/:id", middleware.PermissionRequired("website_rules.view"), h.WebsiteBlock.GetRule)
				websiteRules.POST("", middleware.PermissionRequired("website_rules.create"), h.WebsiteBlock.CreateRule)
				websiteRules.PUT("/:id", middleware.PermissionRequired("website_rules.update"), h.WebsiteBlock.UpdateRule)
				websiteRules.DELETE("/:id", middleware.PermissionRequired("website_rules.delete"), h.WebsiteBlock.DeleteRule)
				websiteRules.PUT("/:id/schedules", middleware.PermissionRequired("website_rules.update"), h.WebsiteBlock.SetSchedules)
				websiteRules.PUT("/:id/groups", middleware.PermissionRequired("website_rules.update"), h.WebsiteBlock.SetGroups)
			}
			protected.GET("/website-violations", middleware.StaffOnly(), middleware.PermissionRequired("website_rules.view"), h.WebsiteBlock.ListViolations)

			// Đánh giá: hội viên tự gửi được từ máy trạm; xem tổng hợp là việc
			// của nhân viên.
			// Bảng giá: máy tính tiền đã đọc hai bảng này từ lâu nhưng chưa có
			// đường ghi nào, nên chỉ chèn tay vào database mới có giá.
			machinePrices := protected.Group("/machine-prices", middleware.StaffOnly())
			{
				machinePrices.GET("", middleware.PermissionRequired("pricing.view"), h.Pricing.ListMachinePrices)
				machinePrices.POST("", middleware.PermissionRequired("pricing.create"), h.Pricing.CreateMachinePrice)
				machinePrices.PUT("/:id", middleware.PermissionRequired("pricing.update"), h.Pricing.UpdateMachinePrice)
				machinePrices.DELETE("/:id", middleware.PermissionRequired("pricing.delete"), h.Pricing.DeleteMachinePrice)
			}

			timePricing := protected.Group("/time-pricing", middleware.StaffOnly())
			{
				timePricing.GET("", middleware.PermissionRequired("pricing.view"), h.Pricing.ListTimePricing)
				timePricing.POST("", middleware.PermissionRequired("pricing.create"), h.Pricing.CreateTimePricing)
				timePricing.PUT("/:id", middleware.PermissionRequired("pricing.update"), h.Pricing.UpdateTimePricing)
				timePricing.DELETE("/:id", middleware.PermissionRequired("pricing.delete"), h.Pricing.DeleteTimePricing)
			}

			feedback := protected.Group("/feedback")
			{
				feedback.POST("", h.Feedback.Create)
				feedback.GET("", middleware.StaffOnly(), middleware.PermissionRequired("feedback.view"), h.Feedback.List)
				feedback.GET("/summary", middleware.StaffOnly(), middleware.PermissionRequired("feedback.view"), h.Feedback.Summary)
			}

			attendance := protected.Group("/attendance")
			{
				attendance.POST("/checkin", h.Attendance.Checkin)
				attendance.GET("/status", h.Attendance.Status)
				attendance.GET("", middleware.StaffOnly(), middleware.PermissionRequired("attendance.view"), h.Attendance.List)
			}

			counts := protected.Group("/inventory-counts", middleware.StaffOnly())
			{
				counts.GET("", middleware.PermissionRequired("inventory_counts.view"), h.InventoryCount.List)
				counts.GET("/:id", middleware.PermissionRequired("inventory_counts.view"), h.InventoryCount.GetByID)
				counts.POST("", middleware.PermissionRequired("inventory_counts.open"), h.InventoryCount.Open)
				counts.POST("/:id/lines", middleware.PermissionRequired("inventory_counts.count"), h.InventoryCount.SetLine)
				counts.DELETE("/:id/lines/:product_id", middleware.PermissionRequired("inventory_counts.count"), h.InventoryCount.RemoveLine)
				counts.POST("/:id/commit", middleware.PermissionRequired("inventory_counts.commit"), h.InventoryCount.Commit)
				counts.POST("/:id/cancel", middleware.PermissionRequired("inventory_counts.cancel"), h.InventoryCount.Cancel)
			}

			printers := protected.Group("/printers", middleware.StaffOnly())
			{
				printers.GET("", middleware.PermissionRequired("printers.view"), h.Printer.List)
				printers.GET("/:id", middleware.PermissionRequired("printers.view"), h.Printer.GetByID)
				printers.POST("", middleware.PermissionRequired("printers.create"), h.Printer.Create)
				printers.PUT("/:id", middleware.PermissionRequired("printers.update"), h.Printer.Update)
				printers.DELETE("/:id", middleware.PermissionRequired("printers.delete"), h.Printer.Delete)
				printers.POST("/:id/test", middleware.PermissionRequired("printers.test"), h.Printer.TestPrint)
				printers.GET("/:id/products", middleware.PermissionRequired("printers.view"), h.Printer.ListProducts)
				printers.PUT("/:id/products", middleware.PermissionRequired("printers.update"), h.Printer.SetProducts)
			}

			units := protected.Group("/units", middleware.StaffOnly())
			{
				units.GET("", middleware.PermissionRequired("units.view"), h.Inventory.ListUnits)
			}

			stockTransactions := protected.Group("/stock-transactions", middleware.StaffOnly())
			{
				stockTransactions.GET("", middleware.PermissionRequired("stock.view"), h.Inventory.ListStockTransactions)
				stockTransactions.POST("", middleware.PermissionRequired("stock.create"), h.Inventory.CreateStockTransaction)
			}

			suppliers := protected.Group("/suppliers", middleware.StaffOnly())
			{
				suppliers.GET("", middleware.PermissionRequired("suppliers.view"), h.Inventory.ListSuppliers)
				suppliers.POST("", middleware.PermissionRequired("suppliers.create"), h.Inventory.CreateSupplier)
				suppliers.PUT("/:id", middleware.PermissionRequired("suppliers.update"), h.Inventory.UpdateSupplier)
				suppliers.DELETE("/:id", middleware.PermissionRequired("suppliers.delete"), h.Inventory.DeleteSupplier)
			}

			shifts := protected.Group("/shifts", middleware.StaffOnly())
			{
				shifts.GET("", middleware.PermissionRequired("shifts.view"), h.Shift.List)
				shifts.GET("/:id", middleware.PermissionRequired("shifts.view"), h.Shift.GetByID)
				shifts.POST("/open", middleware.PermissionRequired("shifts.open"), h.Shift.OpenShift)
				shifts.POST("/:id/close", middleware.PermissionRequired("shifts.close"), h.Shift.CloseShift)
				shifts.POST("/:id/handover", middleware.PermissionRequired("shifts.handover"), h.Shift.Handover)
			}

			protected.GET("/transactions", middleware.StaffOnly(), middleware.PermissionRequired("transactions.view"), h.Report.ListTransactions)

			reports := protected.Group("/reports", middleware.StaffOnly(), middleware.PermissionRequired("reports.view"))
			{
				reports.GET("/daily-revenue", h.Report.DailyRevenue)
				reports.GET("/monthly-revenue", h.Report.MonthlyRevenue)
				reports.GET("/by-member", h.Report.ByMember)
				reports.GET("/by-machine", h.Report.ByMachine)
				reports.GET("/by-employee", h.Report.ByEmployee)
				reports.GET("/top-products", h.Report.TopProducts)
				reports.GET("/promotion-usage", h.Report.PromotionUsage)
			}

			settings := protected.Group("/settings")
			{
				settings.GET("/:group", h.Settings.GetByGroup)

				settings.GET("", middleware.StaffOnly(), middleware.PermissionRequired("settings.view"), h.Settings.List)
				settings.PUT("/:group", middleware.StaffOnly(), middleware.PermissionRequired("settings.edit"), h.Settings.Update)
			}

			auditLogs := protected.Group("/audit-logs", middleware.StaffOnly(), middleware.PermissionRequired("client.admin"))
			{
				auditLogs.GET("", middleware.PermissionRequired("audit.view"), h.Audit.List)
				auditLogs.GET("/:id", middleware.PermissionRequired("audit.view"), h.Audit.GetByID)
			}

			backups := protected.Group("/backups", middleware.StaffOnly(), middleware.PermissionRequired("client.admin"))
			{
				backups.GET("", middleware.PermissionRequired("backups.view"), h.Backup.List)
				backups.POST("", middleware.PermissionRequired("backups.create"), h.Backup.Create)
				backups.POST("/:id/restore", middleware.PermissionRequired("backups.restore"), h.Backup.Restore)
				backups.DELETE("/:id", middleware.PermissionRequired("backups.delete"), h.Backup.Delete)
			}

			chat := protected.Group("/chat")
			{
				chat.GET("/rooms", h.Chat.ListRooms)
				chat.POST("/rooms", h.Chat.CreateRoom)
				chat.DELETE("/rooms", middleware.StaffOnly(), middleware.PermissionRequired("chat.moderate"), h.Chat.DeleteAllRooms)
				chat.GET("/rooms/:id/messages", h.Chat.GetMessages)
				chat.DELETE("/rooms/:id", middleware.StaffOnly(), middleware.PermissionRequired("chat.moderate"), h.Chat.DeleteRoom)
				chat.PUT("/rooms/:id/read", h.Chat.MarkRoomMessagesRead)
				chat.POST("/messages", h.Chat.SendMessage)
				chat.PUT("/messages/:id/deliver", h.Chat.MarkMessageDelivered)
				chat.PUT("/messages/:id/read", h.Chat.MarkMessageRead)
				// chat.POST("/topup-request", h.Chat.RequestTopup)
			}

			notifications := protected.Group("/notifications")
			{
				notifications.GET("", h.Notification.List)
				notifications.GET("/unread-count", h.Notification.UnreadCount)
				notifications.PUT("/:id/read", h.Notification.MarkRead)
				notifications.PUT("/read-all", h.Notification.MarkAllRead)
			}

			adminNotifications := protected.Group("/admin/notifications", middleware.StaffOnly(), middleware.PermissionRequired("client.admin"))
			{
				adminNotifications.GET("", middleware.PermissionRequired("notifications.view"), h.NotificationAdmin.List)
				adminNotifications.GET("/:id", middleware.PermissionRequired("notifications.view"), h.NotificationAdmin.GetByID)
				adminNotifications.POST("", middleware.PermissionRequired("notifications.create"), h.NotificationAdmin.Create)
				adminNotifications.PUT("/:id", middleware.PermissionRequired("notifications.update"), h.NotificationAdmin.Update)
				adminNotifications.DELETE("/:id", middleware.PermissionRequired("notifications.delete"), h.NotificationAdmin.Delete)
				adminNotifications.POST("/:id/dispatch", middleware.PermissionRequired("notifications.dispatch"), h.NotificationAdmin.Dispatch)
			}

			systemManage := protected.Group("/systemManage", middleware.StaffOnly(), middleware.PermissionRequired("client.admin"))
			{
				systemManage.GET("/getUserList", middleware.PermissionRequired("system.users.view"), h.SystemManage.GetUserList)
				systemManage.POST("/addUser", middleware.PermissionRequired("system.users.create"), h.SystemManage.AddUser)
				systemManage.POST("/updateUser", middleware.PermissionRequired("system.users.update"), h.SystemManage.UpdateUser)
				systemManage.DELETE("/deleteUser", middleware.PermissionRequired("system.users.delete"), h.SystemManage.DeleteUser)
				systemManage.DELETE("/batchDeleteUser", middleware.PermissionRequired("system.users.delete"), h.SystemManage.BatchDeleteUser)

				systemManage.GET("/getRoleList", middleware.PermissionRequired("system.roles.view"), h.SystemManage.GetRoleList)
				systemManage.GET("/getAllRoles", middleware.PermissionRequired("system.roles.view"), h.SystemManage.GetAllRoles)
				systemManage.POST("/addRole", middleware.PermissionRequired("system.roles.create"), h.SystemManage.AddRole)
				systemManage.POST("/updateRole", middleware.PermissionRequired("system.roles.update"), h.SystemManage.UpdateRole)
				systemManage.DELETE("/deleteRole", middleware.PermissionRequired("system.roles.delete"), h.SystemManage.DeleteRole)
				systemManage.DELETE("/batchDeleteRole", middleware.PermissionRequired("system.roles.delete"), h.SystemManage.BatchDeleteRole)
				systemManage.GET("/getAllPermissions", middleware.PermissionRequired("system.roles.view"), h.SystemManage.GetAllPermissions)
				systemManage.GET("/getRolePermissions", middleware.PermissionRequired("system.roles.view"), h.SystemManage.GetRolePermissions)
				systemManage.POST("/updateRolePermissions", middleware.PermissionRequired("system.roles.permissions"), h.SystemManage.UpdateRolePermissions)

				systemManage.GET("/getMenuList/v2", middleware.PermissionRequired("system.menus.view"), h.SystemManage.GetMenuList)
				systemManage.GET("/getAllPages", middleware.PermissionRequired("system.menus.view"), h.SystemManage.GetAllPages)
				systemManage.GET("/getMenuTree", middleware.PermissionRequired("system.menus.view"), h.SystemManage.GetMenuTree)
			}
		}
	}
}
