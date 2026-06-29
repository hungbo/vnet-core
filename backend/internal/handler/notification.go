package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/response"
)

type NotificationHandler struct {
	svc *service.NotificationService
}

func NewNotificationHandler(svc *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

func (h *NotificationHandler) List(c *gin.Context) {
	memberID := middleware.GetUserID(c)

	notifications, total, page, pageSize, err := h.svc.List(memberID, *pagination.GetParams(c))
	if err != nil {
		response.InternalError(c, "Failed to fetch notifications")
		return
	}
	response.Paginated(c, notifications, total, page, pageSize)
}

func (h *NotificationHandler) UnreadCount(c *gin.Context) {
	memberID := middleware.GetUserID(c)
	count, err := h.svc.UnreadCount(memberID)
	if err != nil {
		response.InternalError(c, "Failed to count notifications")
		return
	}
	response.Success(c, map[string]int64{"count": count})
}

func (h *NotificationHandler) MarkRead(c *gin.Context) {
	memberID := middleware.GetUserID(c)
	notificationID := c.Param("id")

	if err := h.svc.MarkRead(notificationID, memberID); err != nil {
		response.NotFound(c, "Notification not found")
		return
	}
	response.Success(c, nil)
}

func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	memberID := middleware.GetUserID(c)

	if err := h.svc.MarkAllRead(memberID); err != nil {
		response.InternalError(c, "Failed to mark notifications as read")
		return
	}
	response.Success(c, nil)
}
