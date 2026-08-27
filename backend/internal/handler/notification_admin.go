package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/response"
)

type NotificationAdminHandler struct {
	svc *service.NotificationAdminService
}

func NewNotificationAdminHandler(svc *service.NotificationAdminService) *NotificationAdminHandler {
	return &NotificationAdminHandler{svc: svc}
}

// @Summary List admin notifications
// @Tags Admin Notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200 {object} response.Response{data=response.PaginatedData{items=[]service.NotificationResponse}}
// @Router /admin/notifications [get]
func (h *NotificationAdminHandler) List(c *gin.Context) {
	params := *pagination.GetParams(c)
	items, total, page, pageSize, err := h.svc.List(params)
	if err != nil {
		response.InternalError(c, "Failed to fetch notifications")
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

// @Summary Get notification by ID
// @Tags Admin Notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Notification ID"
// @Success 200 {object} response.Response{data=service.NotificationResponse}
// @Router /admin/notifications/{id} [get]
func (h *NotificationAdminHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	result, err := h.svc.GetByID(id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, result)
}

// @Summary Create notification
// @Tags Admin Notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body service.CreateNotificationRequest true "Notification data"
// @Success 201 {object} response.Response{data=service.NotificationResponse}
// @Router /admin/notifications [post]
func (h *NotificationAdminHandler) Create(c *gin.Context) {
	var req service.CreateNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	result, err := h.svc.Create(&req)
	if err != nil {
		handleCreateError(c, err)
		return
	}
	response.Created(c, result)
}

// @Summary Update notification
// @Tags Admin Notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Notification ID"
// @Param body body service.UpdateNotificationRequest true "Notification update data"
// @Success 200 {object} response.Response{data=service.NotificationResponse}
// @Router /admin/notifications/{id} [put]
func (h *NotificationAdminHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req service.UpdateNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	result, err := h.svc.Update(id, &req)
	if err != nil {
		handleCreateError(c, err)
		return
	}
	response.Success(c, result)
}

// @Summary Delete notification
// @Tags Admin Notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Notification ID"
// @Success 200 {object} response.Response
// @Failure 409 {object} response.Response "còn dữ liệu phụ thuộc"
// @Router /admin/notifications/{id} [delete]
func (h *NotificationAdminHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.Delete(id); err != nil {
		handleDeleteError(c, err)
		return
	}
	response.Success(c, nil)
}

// @Summary Dispatch notification to all members
// @Tags Admin Notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Notification ID"
// @Success 200 {object} response.Response{data=map[string]int}
// @Router /admin/notifications/{id}/dispatch [post]
func (h *NotificationAdminHandler) Dispatch(c *gin.Context) {
	id := c.Param("id")
	count, err := h.svc.Dispatch(id)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"dispatched": count})
}
