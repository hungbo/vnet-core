package router

import (
	"os"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/handler"
	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/pkg/jwt"
	"gorm.io/gorm"
)

func Register(r *gin.Engine, db *gorm.DB, jwtManager *jwt.Manager, wsHub *hub.Hub, cfg *config.Config) {
	r.Use(middleware.StoreContext())

	h := handler.NewHandlers(db, jwtManager, wsHub, cfg)

	uploadsDir := cfg.Server.UploadDir
	if _, err := os.Stat(uploadsDir); os.IsNotExist(err) {
		os.MkdirAll(uploadsDir, 0755)
	}
	r.Static("/uploads", uploadsDir)

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

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
		}

		protected := api.Group("")
		protected.Use(middleware.AuthRequired(jwtManager))
		{
			protected.GET("/ws/client", wsHub.HandleWS)
			protected.POST("/upload", h.Upload.Upload)
			protectedRoute := protected.Group("/route")
			{
				protectedRoute.GET("/getUserRoutes", h.Route.GetUserRoutes)
			}
			protectedAuth := protected.Group("/auth")
			{
				protectedAuth.GET("/me", h.Auth.Me)
				protectedAuth.PUT("/change-password", h.Auth.ChangePassword)
				protectedAuth.GET("/permissions", h.Auth.GetPermissions)
			}

			members := protected.Group("/members")
			{
				members.GET("", h.Member.List)
				members.GET("/:id", h.Member.GetByID)
				members.POST("", h.Member.Create)
				members.PUT("/:id", h.Member.Update)
				members.DELETE("/:id", h.Member.Delete)
				members.POST("/:id/reset-password", h.Member.ResetPassword)
				members.POST("/:id/topup", h.Member.Topup)
				members.POST("/:id/refund", h.Member.Refund)
				members.GET("/:id/transactions", h.Member.GetTransactions)
				members.GET("/:id/sessions", h.Member.GetSessions)
				members.GET("/:id/combos", h.Member.GetCombos)
			}

			memberGroups := protected.Group("/member-groups")
			{
				memberGroups.GET("", h.Member.ListGroups)
				memberGroups.POST("", h.Member.CreateGroup)
				memberGroups.PUT("/:id", h.Member.UpdateGroup)
				memberGroups.DELETE("/:id", h.Member.DeleteGroup)
			}

			machines := protected.Group("/machines")
			{
				machines.GET("", h.Machine.List)
				machines.GET("/by-code/:code", h.Machine.GetByCode)
				machines.GET("/:id", h.Machine.GetByID)
				machines.POST("", h.Machine.Create)
				machines.PUT("/:id", h.Machine.Update)
				machines.DELETE("/:id", h.Machine.Delete)
				machines.POST("/:id/heartbeat", h.Machine.Heartbeat)
				machines.GET("/:id/hardware", h.Machine.GetHardware)
				machines.POST("/:id/remote/:action", h.Machine.RemoteAction)
			}

			machineGroups := protected.Group("/machine-groups")
			{
				machineGroups.GET("", h.Machine.ListGroups)
				machineGroups.POST("", h.Machine.CreateGroup)
				machineGroups.PUT("/:id", h.Machine.UpdateGroup)
				machineGroups.DELETE("/:id", h.Machine.DeleteGroup)
			}

			machinePrices := protected.Group("/machine-prices")
			{
				machinePrices.GET("", h.Machine.ListPrices)
				machinePrices.POST("", h.Machine.CreatePrice)
				machinePrices.PUT("/:id", h.Machine.UpdatePrice)
				machinePrices.DELETE("/:id", h.Machine.DeletePrice)
			}

			machineAssets := protected.Group("/machine-assets")
			{
				machineAssets.GET("", h.Machine.ListAssets)
				machineAssets.POST("", h.Machine.CreateAsset)
				machineAssets.PUT("/:id", h.Machine.UpdateAsset)
				machineAssets.DELETE("/:id", h.Machine.DeleteAsset)
			}

			sessions := protected.Group("/sessions")
			{
				sessions.GET("/active", h.Session.ListActive)
				sessions.GET("/me", h.Session.GetMySession)
				sessions.POST("/start", h.Session.Start)
				sessions.POST("/:id/end", h.Session.End)
				sessions.GET("/:id", h.Session.Get)
				sessions.POST("/:id/switch-machine", h.Session.SwitchMachine)
				sessions.GET("/calculate-cost", h.Session.CalculateCost)
			}

			combos := protected.Group("/combos")
			{
				combos.GET("", h.Combo.List)
				combos.GET("/:id", h.Combo.GetByID)
				combos.POST("", h.Combo.Create)
				combos.PUT("/:id", h.Combo.Update)
				combos.DELETE("/:id", h.Combo.Delete)
				combos.POST("/:id/purchase", h.Combo.Purchase)
				combos.POST("/:id/activate", h.Combo.Activate)
			}

			bookings := protected.Group("/bookings")
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

			promotions := protected.Group("/promotions")
			{
				promotions.GET("", h.Promotion.List)
				promotions.GET("/:id", h.Promotion.GetByID)
				promotions.POST("", h.Promotion.Create)
				promotions.PUT("/:id", h.Promotion.Update)
				promotions.DELETE("/:id", h.Promotion.Delete)
			}

			luckySpin := protected.Group("/lucky-spin")
			{
				luckySpin.GET("/rewards", h.Promotion.GetLuckySpinRewards)
				luckySpin.POST("/spin", h.Promotion.Spin)
			}

			curfew := protected.Group("/curfew")
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
				categories.POST("", h.Category.Create)
				categories.PUT("/:id", h.Category.Update)
				categories.DELETE("/:id", h.Category.Delete)
			}

			products := protected.Group("/products")
			{
				products.GET("", h.Product.List)
				products.GET("/:id", h.Product.GetByID)
				products.POST("", h.Product.Create)
				products.PUT("/:id", h.Product.Update)
				products.DELETE("/:id", h.Product.Delete)
				products.GET("/:id/ingredients", h.Inventory.ListProductIngredients)
				products.POST("/:id/ingredients", h.Inventory.CreateProductIngredient)
				products.PUT("/:id/ingredients/:ingredientId", h.Inventory.UpdateProductIngredient)
				products.DELETE("/:id/ingredients/:ingredientId", h.Inventory.DeleteProductIngredient)
			}

			orders := protected.Group("/orders")
			{
				orders.GET("", h.Order.List)
				orders.GET("/:id", h.Order.GetByID)
				orders.POST("", h.Order.Create)
				orders.PUT("/:id", h.Order.Update)
				orders.DELETE("/:id", h.Order.Delete)
				orders.POST("/:id/status", h.Order.UpdateStatus)
				orders.POST("/:id/split", h.Order.Split)
				orders.POST("/:id/pay", h.Order.Pay)
			}

			printers := protected.Group("/printers")
			{
				printers.GET("", h.Printer.List)
				printers.GET("/:id", h.Printer.GetByID)
				printers.POST("", h.Printer.Create)
				printers.PUT("/:id", h.Printer.Update)
				printers.DELETE("/:id", h.Printer.Delete)
				printers.POST("/:id/test", h.Printer.TestPrint)
			}

			units := protected.Group("/units")
			{
				units.GET("", h.Inventory.ListUnits)
			}

			stockTransactions := protected.Group("/stock-transactions")
			{
				stockTransactions.GET("", h.Inventory.ListStockTransactions)
				stockTransactions.POST("", h.Inventory.CreateStockTransaction)
			}

			suppliers := protected.Group("/suppliers")
			{
				suppliers.GET("", h.Inventory.ListSuppliers)
				suppliers.POST("", h.Inventory.CreateSupplier)
				suppliers.PUT("/:id", h.Inventory.UpdateSupplier)
				suppliers.DELETE("/:id", h.Inventory.DeleteSupplier)
			}

			warehouses := protected.Group("/warehouses")
			{
				warehouses.GET("", h.Inventory.ListWarehouses)
				warehouses.POST("", h.Inventory.CreateWarehouse)
				warehouses.PUT("/:id", h.Inventory.UpdateWarehouse)
				warehouses.DELETE("/:id", h.Inventory.DeleteWarehouse)
			}

			shifts := protected.Group("/shifts")
			{
				shifts.GET("", h.Shift.List)
				shifts.GET("/:id", h.Shift.GetByID)
				shifts.POST("/open", h.Shift.OpenShift)
				shifts.POST("/:id/close", h.Shift.CloseShift)
				shifts.POST("/:id/handover", h.Shift.Handover)
			}

			protected.GET("/transactions", h.Report.ListTransactions)

			reports := protected.Group("/reports")
			{
				reports.GET("/daily-revenue", h.Report.DailyRevenue)
				reports.GET("/monthly-revenue", h.Report.MonthlyRevenue)
				reports.GET("/by-member", h.Report.ByMember)
				reports.GET("/by-machine", h.Report.ByMachine)
				reports.GET("/by-employee", h.Report.ByEmployee)
				reports.GET("/top-products", h.Report.TopProducts)
				reports.GET("/promotion-usage", h.Report.PromotionUsage)
			}

			stores := protected.Group("/stores")
			{
				stores.GET("", h.Store.List)
				stores.GET("/:id", h.Store.GetByID)
				stores.POST("", h.Store.Create)
				stores.PUT("/:id", h.Store.Update)
				stores.DELETE("/:id", h.Store.Delete)
			}

			settings := protected.Group("/settings")
			{
				settings.GET("", h.Settings.List)
				settings.GET("/:group", h.Settings.GetByGroup)
				settings.PUT("/:group", h.Settings.Update)
			}

			auditLogs := protected.Group("/audit-logs")
			{
				auditLogs.GET("", h.Audit.List)
				auditLogs.GET("/:id", h.Audit.GetByID)
			}

			backups := protected.Group("/backups")
			{
				backups.GET("", h.Backup.List)
				backups.POST("", h.Backup.Create)
				backups.POST("/:id/restore", h.Backup.Restore)
			}

			chat := protected.Group("/chat")
			{
				chat.GET("/rooms", h.Chat.ListRooms)
				chat.POST("/rooms", h.Chat.CreateRoom)
				chat.DELETE("/rooms", h.Chat.DeleteAllRooms)
				chat.GET("/rooms/:id/messages", h.Chat.GetMessages)
				chat.DELETE("/rooms/:id", h.Chat.DeleteRoom)
				chat.PUT("/rooms/:id/read", h.Chat.MarkRoomMessagesRead)
				chat.POST("/messages", h.Chat.SendMessage)
				chat.PUT("/messages/:id/deliver", h.Chat.MarkMessageDelivered)
				chat.PUT("/messages/:id/read", h.Chat.MarkMessageRead)
				chat.POST("/topup-request", h.Chat.RequestTopup)
			}

			notifications := protected.Group("/notifications")
			{
				notifications.GET("", h.Notification.List)
				notifications.GET("/unread-count", h.Notification.UnreadCount)
				notifications.PUT("/:id/read", h.Notification.MarkRead)
				notifications.PUT("/read-all", h.Notification.MarkAllRead)
			}

			systemManage := protected.Group("/systemManage")
			{
				systemManage.GET("/getUserList", h.SystemManage.GetUserList)
				systemManage.POST("/addUser", h.SystemManage.AddUser)
				systemManage.POST("/updateUser", h.SystemManage.UpdateUser)
				systemManage.DELETE("/deleteUser", h.SystemManage.DeleteUser)
				systemManage.DELETE("/batchDeleteUser", h.SystemManage.BatchDeleteUser)

				systemManage.GET("/getRoleList", h.SystemManage.GetRoleList)
				systemManage.GET("/getAllRoles", h.SystemManage.GetAllRoles)

				systemManage.GET("/getMenuList/v2", h.SystemManage.GetMenuList)
				systemManage.GET("/getAllPages", h.SystemManage.GetAllPages)
				systemManage.GET("/getMenuTree", h.SystemManage.GetMenuTree)
			}
		}
	}
}
