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
		// xác thực bằng khoá riêng của máy (X-Agent-Token), không phải mở tự do.
		api.POST("/machines/by-code/:code/heartbeat", h.Machine.HeartbeatByCode)
		api.GET("/machines/by-code/:code/blocklist", h.WebsiteBlock.EffectiveForMachine)
		api.POST("/machines/by-code/:code/blocklist/violations", h.WebsiteBlock.ReportViolation)
		api.GET("/machines/by-code/:code/app-update", h.AppUpdate.Latest)

		// WebSocket nằm NGOÀI nhóm protected: nó nhận thêm một cách vào thứ hai —
		// khoá riêng của máy trạm — để tiến trình nền giữ được kết nối kể cả khi
		// không có ai đăng nhập trên máy đó.
		api.GET("/ws/client",
			middleware.AuthOrAgent(jwtManager, h.Machine.VerifyAgentToken),
			wsHub.HandleWS)

		protected := api.Group("")
		protected.Use(middleware.AuthRequired(jwtManager))
		{
			protected.POST("/upload", middleware.StaffOnly(), h.Upload.Upload)
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
				members.GET("", middleware.StaffOnly(), h.Member.List)
				members.POST("/refresh-tiers", middleware.StaffOnly(), h.Member.RefreshTiers)
				members.POST("", middleware.StaffOnly(), middleware.PermissionRequired("members.create"), h.Member.Create)
				members.PUT("/:id", middleware.StaffOnly(), h.Member.Update)
				members.DELETE("/:id", middleware.StaffOnly(), h.Member.Delete)
				members.POST("/:id/reset-password", middleware.StaffOnly(), h.Member.ResetPassword)
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
				memberGroups.GET("", h.Member.ListGroups)
				memberGroups.POST("", h.Member.CreateGroup)
				memberGroups.PUT("/:id", h.Member.UpdateGroup)
				memberGroups.DELETE("/:id", h.Member.DeleteGroup)
			}

			machines := protected.Group("/machines")
			{
				machines.GET("/by-code/:code", h.Machine.GetByCode)

				machines.GET("", middleware.StaffOnly(), h.Machine.List)
				machines.GET("/:id", middleware.StaffOnly(), h.Machine.GetByID)
				machines.POST("", middleware.StaffOnly(), h.Machine.Create)
				machines.PUT("/:id", middleware.StaffOnly(), h.Machine.Update)
				machines.DELETE("/:id", middleware.StaffOnly(), h.Machine.Delete)
				machines.POST("/:id/heartbeat", middleware.StaffOnly(), h.Machine.Heartbeat)
				machines.POST("/:id/agent-token", middleware.StaffOnly(), h.Machine.IssueAgentToken)
				machines.GET("/:id/hardware", middleware.StaffOnly(), h.Machine.GetHardware)
				machines.POST("/:id/remote/:action", middleware.StaffOnly(), h.Machine.RemoteAction)
			}

			machineGroups := protected.Group("/machine-groups", middleware.StaffOnly())
			{
				machineGroups.GET("", h.Machine.ListGroups)
				machineGroups.POST("", h.Machine.CreateGroup)
				machineGroups.PUT("/:id", h.Machine.UpdateGroup)
				machineGroups.DELETE("/:id", h.Machine.DeleteGroup)
			}

			machineAssets := protected.Group("/machine-assets", middleware.StaffOnly())
			{
				machineAssets.GET("", h.Machine.ListAssets)
				machineAssets.POST("", h.Machine.CreateAsset)
				machineAssets.PUT("/:id", h.Machine.UpdateAsset)
				machineAssets.DELETE("/:id", h.Machine.DeleteAsset)
			}

			sessions := protected.Group("/sessions")
			{
				sessions.GET("/me", h.Session.GetMySession)

				sessions.GET("/active", middleware.StaffOnly(), h.Session.ListActive)
				sessions.POST("/start", middleware.StaffOnly(), h.Session.Start)
				sessions.POST("/:id/end", middleware.StaffOnly(), h.Session.End)
				sessions.GET("/:id", middleware.StaffOnly(), h.Session.Get)
				sessions.POST("/:id/switch-machine", middleware.StaffOnly(), h.Session.SwitchMachine)
				sessions.GET("/calculate-cost", middleware.StaffOnly(), h.Session.CalculateCost)
			}

			combos := protected.Group("/combos", middleware.StaffOnly())
			{
				combos.GET("", h.Combo.List)
				combos.GET("/:id", h.Combo.GetByID)
				combos.POST("", h.Combo.Create)
				combos.PUT("/:id", h.Combo.Update)
				combos.DELETE("/:id", h.Combo.Delete)
				combos.POST("/:id/purchase", h.Combo.Purchase)
				combos.POST("/:id/activate", h.Combo.Activate)
			}

			bookings := protected.Group("/bookings", middleware.StaffOnly())
			{
				bookings.GET("", h.Booking.List)
				bookings.GET("/:id", h.Booking.GetByID)
				bookings.POST("", h.Booking.Create)
				bookings.PUT("/:id", h.Booking.Update)
				bookings.DELETE("/:id", h.Booking.Delete)
				bookings.POST("/:id/check-in", h.Booking.CheckIn)
				bookings.POST("/:id/cancel", h.Booking.Cancel)
				bookings.POST("/:id/no-show", h.Booking.NoShow)
			}

			promotions := protected.Group("/promotions", middleware.StaffOnly())
			{
				promotions.GET("", h.Promotion.List)
				promotions.GET("/:id", h.Promotion.GetByID)
				promotions.POST("", h.Promotion.Create)
				promotions.PUT("/:id", h.Promotion.Update)
				promotions.DELETE("/:id", h.Promotion.Delete)
			}

			luckySpin := protected.Group("/lucky-spin", middleware.StaffOnly())
			{
				luckySpin.GET("/rewards", h.Promotion.GetLuckySpinRewards)
				luckySpin.POST("/rewards", h.Promotion.CreateLuckySpinReward)
				luckySpin.PUT("/rewards/:id", h.Promotion.UpdateLuckySpinReward)
				luckySpin.DELETE("/rewards/:id", h.Promotion.DeleteLuckySpinReward)
				luckySpin.POST("/spin", h.Promotion.Spin)
			}

			curfew := protected.Group("/curfew", middleware.StaffOnly())
			{
				curfew.GET("", h.Curfew.List)
				curfew.GET("/:id", h.Curfew.GetByID)
				curfew.POST("", h.Curfew.Create)
				curfew.PUT("/:id", h.Curfew.Update)
				curfew.DELETE("/:id", h.Curfew.Delete)
				curfew.POST("/override", h.Curfew.Override)
			}

			categories := protected.Group("/categories")
			{
				categories.GET("", h.Category.List)
				categories.GET("/:id", h.Category.GetByID)

				categories.POST("", middleware.StaffOnly(), h.Category.Create)
				categories.PUT("/:id", middleware.StaffOnly(), h.Category.Update)
				categories.DELETE("/:id", middleware.StaffOnly(), h.Category.Delete)
			}

			products := protected.Group("/products")
			{
				products.GET("", h.Product.List)
				products.GET("/:id", h.Product.GetByID)

				products.POST("", middleware.StaffOnly(), h.Product.Create)
				products.PUT("/:id", middleware.StaffOnly(), h.Product.Update)
				products.DELETE("/:id", middleware.StaffOnly(), h.Product.Delete)
				products.GET("/:id/ingredients", middleware.StaffOnly(), h.Inventory.ListProductIngredients)
				products.POST("/:id/ingredients", middleware.StaffOnly(), h.Inventory.CreateProductIngredient)
				products.PUT("/:id/ingredients/:ingredientId", middleware.StaffOnly(), h.Inventory.UpdateProductIngredient)
				products.DELETE("/:id/ingredients/:ingredientId", middleware.StaffOnly(), h.Inventory.DeleteProductIngredient)
			}

			orders := protected.Group("/orders")
			{
				orders.POST("", h.Order.Create)
				orders.POST("/topup-request", h.Order.CreateTopup)

				orders.GET("", middleware.StaffOnly(), h.Order.List)
				orders.GET("/:id", middleware.StaffOnly(), h.Order.GetByID)
				orders.PUT("/:id", middleware.StaffOnly(), h.Order.Update)
				orders.DELETE("/batch-delete", middleware.StaffOnly(), h.Order.BatchDelete)
				orders.DELETE("/:id", middleware.StaffOnly(), h.Order.Delete)
				orders.POST("/:id/status", middleware.StaffOnly(), h.Order.UpdateStatus)
				orders.POST("/:id/split", middleware.StaffOnly(), h.Order.Split)
				orders.POST("/:id/items/:itemId/status", middleware.StaffOnly(), h.Order.UpdateItemStatus)
				orders.POST("/:id/pay", middleware.StaffOnly(), h.Order.Pay)
				// In hoá đơn và phiếu chế biến. StaffOnly: hội viên không được
				// bắt máy in ở quầy nhả giấy.
				orders.POST("/:id/print", middleware.StaffOnly(), h.Receipt.PrintReceipt)
				orders.POST("/:id/print-stations", middleware.StaffOnly(), h.Receipt.PrintStations)
				orders.GET("/:id/receipt-preview", middleware.StaffOnly(), h.Receipt.PreviewReceipt)
			}

			// Thẻ nạp: sinh/liệt kê/huỷ là việc của nhân viên. Riêng "nạp thẻ"
			// hội viên tự làm được từ máy trạm — handler ép member_id về chính
			// người gọi nên không nạp hộ tài khoản khác được.
			topupCards := protected.Group("/topup-cards")
			{
				topupCards.POST("/redeem", h.Card.RedeemTopupCard)
				topupCards.GET("", middleware.StaffOnly(), h.Card.ListTopupCards)
				topupCards.POST("/generate", middleware.StaffOnly(), h.Card.GenerateTopupCards)
				topupCards.POST("/:id/cancel", middleware.StaffOnly(), h.Card.CancelTopupCard)
				topupCards.POST("/:id/sell", middleware.StaffOnly(), h.Card.SellTopupCard)
			}

			giftCards := protected.Group("/gift-cards")
			{
				giftCards.POST("/check", h.Card.CheckGiftCard)
				giftCards.GET("", middleware.StaffOnly(), h.Card.ListGiftCards)
				giftCards.POST("/generate", middleware.StaffOnly(), h.Card.GenerateGiftCards)
				giftCards.POST("/:id/cancel", middleware.StaffOnly(), h.Card.CancelGiftCard)
			}

			// Điểm danh: hội viên tự làm được từ máy trạm; handler ép member_id
			// về chính người gọi nên không điểm danh hộ người khác được.
			appUpdates := protected.Group("/app-updates", middleware.StaffOnly())
			{
				appUpdates.GET("", h.AppUpdate.List)
				appUpdates.POST("", h.AppUpdate.Create)
				appUpdates.PUT("/:id/active", h.AppUpdate.SetActive)
				appUpdates.DELETE("/:id", h.AppUpdate.Delete)
			}

			websiteRules := protected.Group("/website-rules", middleware.StaffOnly())
			{
				websiteRules.GET("", h.WebsiteBlock.ListRules)
				websiteRules.GET("/:id", h.WebsiteBlock.GetRule)
				websiteRules.POST("", h.WebsiteBlock.CreateRule)
				websiteRules.PUT("/:id", h.WebsiteBlock.UpdateRule)
				websiteRules.DELETE("/:id", h.WebsiteBlock.DeleteRule)
				websiteRules.PUT("/:id/schedules", h.WebsiteBlock.SetSchedules)
				websiteRules.PUT("/:id/groups", h.WebsiteBlock.SetGroups)
			}
			protected.GET("/website-violations", middleware.StaffOnly(), h.WebsiteBlock.ListViolations)

			// Đánh giá: hội viên tự gửi được từ máy trạm; xem tổng hợp là việc
			// của nhân viên.
			// Bảng giá: máy tính tiền đã đọc hai bảng này từ lâu nhưng chưa có
			// đường ghi nào, nên chỉ chèn tay vào database mới có giá.
			machinePrices := protected.Group("/machine-prices", middleware.StaffOnly())
			{
				machinePrices.GET("", h.Pricing.ListMachinePrices)
				machinePrices.POST("", h.Pricing.CreateMachinePrice)
				machinePrices.PUT("/:id", h.Pricing.UpdateMachinePrice)
				machinePrices.DELETE("/:id", h.Pricing.DeleteMachinePrice)
			}

			timePricing := protected.Group("/time-pricing", middleware.StaffOnly())
			{
				timePricing.GET("", h.Pricing.ListTimePricing)
				timePricing.POST("", h.Pricing.CreateTimePricing)
				timePricing.PUT("/:id", h.Pricing.UpdateTimePricing)
				timePricing.DELETE("/:id", h.Pricing.DeleteTimePricing)
			}

			feedback := protected.Group("/feedback")
			{
				feedback.POST("", h.Feedback.Create)
				feedback.GET("", middleware.StaffOnly(), h.Feedback.List)
				feedback.GET("/summary", middleware.StaffOnly(), h.Feedback.Summary)
			}

			attendance := protected.Group("/attendance")
			{
				attendance.POST("/checkin", h.Attendance.Checkin)
				attendance.GET("/status", h.Attendance.Status)
				attendance.GET("", middleware.StaffOnly(), h.Attendance.List)
			}

			counts := protected.Group("/inventory-counts", middleware.StaffOnly())
			{
				counts.GET("", h.InventoryCount.List)
				counts.GET("/:id", h.InventoryCount.GetByID)
				counts.POST("", h.InventoryCount.Open)
				counts.POST("/:id/lines", h.InventoryCount.SetLine)
				counts.DELETE("/:id/lines/:product_id", h.InventoryCount.RemoveLine)
				counts.POST("/:id/commit", h.InventoryCount.Commit)
				counts.POST("/:id/cancel", h.InventoryCount.Cancel)
			}

			printers := protected.Group("/printers", middleware.StaffOnly())
			{
				printers.GET("", h.Printer.List)
				printers.GET("/:id", h.Printer.GetByID)
				printers.POST("", h.Printer.Create)
				printers.PUT("/:id", h.Printer.Update)
				printers.DELETE("/:id", h.Printer.Delete)
				printers.POST("/:id/test", h.Printer.TestPrint)
				printers.GET("/:id/products", h.Printer.ListProducts)
				printers.PUT("/:id/products", h.Printer.SetProducts)
			}

			units := protected.Group("/units", middleware.StaffOnly())
			{
				units.GET("", h.Inventory.ListUnits)
			}

			stockTransactions := protected.Group("/stock-transactions", middleware.StaffOnly())
			{
				stockTransactions.GET("", h.Inventory.ListStockTransactions)
				stockTransactions.POST("", h.Inventory.CreateStockTransaction)
			}

			suppliers := protected.Group("/suppliers", middleware.StaffOnly())
			{
				suppliers.GET("", h.Inventory.ListSuppliers)
				suppliers.POST("", h.Inventory.CreateSupplier)
				suppliers.PUT("/:id", h.Inventory.UpdateSupplier)
				suppliers.DELETE("/:id", h.Inventory.DeleteSupplier)
			}

			shifts := protected.Group("/shifts", middleware.StaffOnly())
			{
				shifts.GET("", h.Shift.List)
				shifts.GET("/:id", h.Shift.GetByID)
				shifts.POST("/open", h.Shift.OpenShift)
				shifts.POST("/:id/close", h.Shift.CloseShift)
				shifts.POST("/:id/handover", h.Shift.Handover)
			}

			protected.GET("/transactions", middleware.StaffOnly(), h.Report.ListTransactions)

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

				settings.GET("", middleware.StaffOnly(), h.Settings.List)
				settings.PUT("/:group", middleware.StaffOnly(), middleware.PermissionRequired("settings.edit"), h.Settings.Update)
			}

			auditLogs := protected.Group("/audit-logs", middleware.StaffOnly(), middleware.PermissionRequired("client.admin"))
			{
				auditLogs.GET("", h.Audit.List)
				auditLogs.GET("/:id", h.Audit.GetByID)
			}

			backups := protected.Group("/backups", middleware.StaffOnly(), middleware.PermissionRequired("client.admin"))
			{
				backups.GET("", h.Backup.List)
				backups.POST("", h.Backup.Create)
				backups.POST("/:id/restore", h.Backup.Restore)
				backups.DELETE("/:id", h.Backup.Delete)
			}

			chat := protected.Group("/chat")
			{
				chat.GET("/rooms", h.Chat.ListRooms)
				chat.POST("/rooms", h.Chat.CreateRoom)
				chat.DELETE("/rooms", middleware.StaffOnly(), h.Chat.DeleteAllRooms)
				chat.GET("/rooms/:id/messages", h.Chat.GetMessages)
				chat.DELETE("/rooms/:id", middleware.StaffOnly(), h.Chat.DeleteRoom)
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
				adminNotifications.GET("", h.NotificationAdmin.List)
				adminNotifications.GET("/:id", h.NotificationAdmin.GetByID)
				adminNotifications.POST("", h.NotificationAdmin.Create)
				adminNotifications.PUT("/:id", h.NotificationAdmin.Update)
				adminNotifications.DELETE("/:id", h.NotificationAdmin.Delete)
				adminNotifications.POST("/:id/dispatch", h.NotificationAdmin.Dispatch)
			}

			systemManage := protected.Group("/systemManage", middleware.StaffOnly(), middleware.PermissionRequired("client.admin"))
			{
				systemManage.GET("/getUserList", h.SystemManage.GetUserList)
				systemManage.POST("/addUser", h.SystemManage.AddUser)
				systemManage.POST("/updateUser", h.SystemManage.UpdateUser)
				systemManage.DELETE("/deleteUser", h.SystemManage.DeleteUser)
				systemManage.DELETE("/batchDeleteUser", h.SystemManage.BatchDeleteUser)

				systemManage.GET("/getRoleList", h.SystemManage.GetRoleList)
				systemManage.GET("/getAllRoles", h.SystemManage.GetAllRoles)
				systemManage.POST("/addRole", h.SystemManage.AddRole)
				systemManage.POST("/updateRole", h.SystemManage.UpdateRole)
				systemManage.DELETE("/deleteRole", h.SystemManage.DeleteRole)
				systemManage.DELETE("/batchDeleteRole", h.SystemManage.BatchDeleteRole)
				systemManage.GET("/getAllPermissions", h.SystemManage.GetAllPermissions)
				systemManage.GET("/getRolePermissions", h.SystemManage.GetRolePermissions)
				systemManage.POST("/updateRolePermissions", h.SystemManage.UpdateRolePermissions)

				systemManage.GET("/getMenuList/v2", h.SystemManage.GetMenuList)
				systemManage.GET("/getAllPages", h.SystemManage.GetAllPages)
				systemManage.GET("/getMenuTree", h.SystemManage.GetMenuTree)
			}
		}
	}
}
