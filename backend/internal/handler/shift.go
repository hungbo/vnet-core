package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/response"
)

type ShiftHandler struct {
	svc *service.ShiftService
}

func NewShiftHandler(svc *service.ShiftService) *ShiftHandler {
	return &ShiftHandler{svc: svc}
}

// @Summary List shifts
// @Description Get a paginated list of shifts
// @Tags Shifts
// @Accept json
// @Produce json
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200 {object} response.Response{data=response.PaginatedData{data=[]service.ShiftResponse}}
// @Failure 500 {object} response.Response
// @Router /api/shifts [get]
// @Security BearerAuth
func (h *ShiftHandler) List(c *gin.Context) {
	params := pagination.GetParams(c)

	result, err := h.svc.List(params)
	if err != nil {
		response.InternalError(c, "Failed to fetch shifts")
		return
	}

	response.Paginated(c, result.Items, result.Total, result.Page, result.PageSize)
}

// @Summary Get shift by ID
// @Description Get a shift by its ID
// @Tags Shifts
// @Accept json
// @Produce json
// @Param id path string true "Shift ID"
// @Success 200 {object} response.Response{data=service.ShiftResponse}
// @Failure 404 {object} response.Response
// @Router /api/shifts/{id} [get]
// @Security BearerAuth
func (h *ShiftHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	result, err := h.svc.GetByID(id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}

	response.Success(c, result)
}

// @Summary Open shift
// @Description Open a new shift
// @Tags Shifts
// @Accept json
// @Produce json
// @Param request body service.OpenShiftRequest true "Request body"
// @Success 201 {object} response.Response{data=service.ShiftResponse}
// @Failure 400 {object} response.Response
// @Router /api/shifts/open [post]
// @Security BearerAuth
func (h *ShiftHandler) OpenShift(c *gin.Context) {
	var req service.OpenShiftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	userID := c.GetString(middleware.ContextKeyUserID)

	result, err := h.svc.OpenShift(&req, userID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Created(c, result)
}

// @Summary Close shift
// @Description Close an existing shift
// @Tags Shifts
// @Accept json
// @Produce json
// @Param id path string true "Shift ID"
// @Param request body service.CloseShiftRequest true "Request body"
// @Success 200 {object} response.Response{data=service.ShiftResponse}
// @Failure 400 {object} response.Response
// @Router /api/shifts/{id}/close [post]
// @Security BearerAuth
func (h *ShiftHandler) CloseShift(c *gin.Context) {
	id := c.Param("id")

	var req service.CloseShiftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	result, err := h.svc.CloseShift(id, &req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, result)
}

// @Summary Handover shift
// @Description Handover a shift to another user
// @Tags Shifts
// @Accept json
// @Produce json
// @Param id path string true "Shift ID"
// @Param request body service.HandoverRequest true "Request body"
// @Success 201 {object} response.Response{data=service.ShiftResponse}
// @Failure 400 {object} response.Response
// @Router /api/shifts/{id}/handover [post]
// @Security BearerAuth
func (h *ShiftHandler) Handover(c *gin.Context) {
	id := c.Param("id")

	var req service.HandoverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	userID := c.GetString(middleware.ContextKeyUserID)

	result, err := h.svc.Handover(id, &req, userID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Created(c, result)
}
