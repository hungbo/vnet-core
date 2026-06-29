package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/response"
)

type BackupHandler struct {
	svc *service.BackupService
}

func NewBackupHandler(svc *service.BackupService) *BackupHandler {
	_ = model.BackupLog{}
	return &BackupHandler{svc: svc}
}

// @Summary List backups
// @Description Get a paginated list of database backups
// @Tags Backups
// @Accept json
// @Produce json
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200 {object} response.Response{data=response.PaginatedData{data=[]model.BackupLog}}
// @Failure 500 {object} response.Response
// @Router /api/backups [get]
// @Security BearerAuth
func (h *BackupHandler) List(c *gin.Context) {
	params := pagination.GetParams(c)
	backups, total, page, pageSize, err := h.svc.List(*params)
	if err != nil {
		response.InternalError(c, "Failed to fetch backups")
		return
	}
	response.Paginated(c, backups, total, page, pageSize)
}

// @Summary Create backup
// @Description Create a new database backup
// @Tags Backups
// @Accept json
// @Produce json
// @Param request body service.CreateBackupRequest true "Request body"
// @Success 201 {object} response.Response{data=model.BackupLog}
// @Failure 400 {object} response.Response
// @Router /api/backups [post]
// @Security BearerAuth
func (h *BackupHandler) Create(c *gin.Context) {
	var req service.CreateBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	result, err := h.svc.Create(&req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, result)
}

// @Summary Restore backup
// @Description Restore a database from a backup
// @Tags Backups
// @Accept json
// @Produce json
// @Param id path string true "Backup ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Router /api/backups/{id}/restore [post]
// @Security BearerAuth
func (h *BackupHandler) Restore(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.Restore(id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}
