package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/response"
)

type SettingsHandler struct {
	svc *service.SettingsService
}

func NewSettingsHandler(svc *service.SettingsService) *SettingsHandler {
	return &SettingsHandler{svc: svc}
}

// @Summary List settings
// @Description Get all settings
// @Tags Settings
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=service.SettingResponse}
// @Failure 500 {object} response.Response
// @Router /api/settings [get]
// @Security BearerAuth
func (h *SettingsHandler) List(c *gin.Context) {
	settings, err := h.svc.List()
	if err != nil {
		response.InternalError(c, "Failed to fetch settings")
		return
	}
	response.Success(c, settings)
}

// @Summary Get settings by group
// @Description Get settings grouped by name
// @Tags Settings
// @Accept json
// @Produce json
// @Param group path string true "Group name"
// @Success 200 {object} response.Response{data=service.SettingResponse}
// @Failure 404 {object} response.Response
// @Router /api/settings/{group} [get]
// @Security BearerAuth
func (h *SettingsHandler) GetByGroup(c *gin.Context) {
	groupName := c.Param("group")
	settings, err := h.svc.GetByGroup(groupName)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, settings)
}

// @Summary Update settings
// @Description Update settings by group
// @Tags Settings
// @Accept json
// @Produce json
// @Param group path string true "Group name"
// @Param request body map[string]interface{} true "Settings data"
// @Success 200 {object} response.Response{data=service.SettingResponse}
// @Failure 400 {object} response.Response
// @Router /api/settings/{group} [put]
// @Security BearerAuth
func (h *SettingsHandler) Update(c *gin.Context) {
	groupName := c.Param("group")
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	result, err := h.svc.Update(groupName, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}
