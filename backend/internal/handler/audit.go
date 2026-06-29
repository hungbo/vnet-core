package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/response"
)

type AuditHandler struct {
	svc *service.AuditService
}

func NewAuditHandler(svc *service.AuditService) *AuditHandler {
	_ = model.AuditLog{}
	return &AuditHandler{svc: svc}
}

// @Summary List audit logs
// @Description Get a paginated list of audit logs
// @Tags AuditLogs
// @Accept json
// @Produce json
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200 {object} response.Response{data=response.PaginatedData{data=[]model.AuditLog}}
// @Failure 500 {object} response.Response
// @Router /api/audit-logs [get]
// @Security BearerAuth
func (h *AuditHandler) List(c *gin.Context) {
	params := pagination.GetParams(c)
	var filters service.AuditLogParams
	if err := c.ShouldBindQuery(&filters); err != nil {
		filters = service.AuditLogParams{}
	}

	logs, total, page, pageSize, err := h.svc.List(*params, filters)
	if err != nil {
		response.InternalError(c, "Failed to fetch audit logs")
		return
	}
	response.Paginated(c, logs, total, page, pageSize)
}

// @Summary Get audit log by ID
// @Description Get a single audit log entry
// @Tags AuditLogs
// @Accept json
// @Produce json
// @Param id path string true "Audit log ID"
// @Success 200 {object} response.Response{data=model.AuditLog}
// @Failure 404 {object} response.Response
// @Router /api/audit-logs/{id} [get]
// @Security BearerAuth
func (h *AuditHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	log, err := h.svc.GetByID(id)
	if err != nil {
		response.NotFound(c, "Audit log not found")
		return
	}
	response.Success(c, log)
}
