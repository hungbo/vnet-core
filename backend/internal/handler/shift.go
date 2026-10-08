package handler

import (
	"errors"

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

// shiftActor lấy người gọi từ token: chỉ người mở ca hoặc chủ quán (quyền "*")
// mới được đóng ca và ghi thu/chi.
func shiftActor(c *gin.Context) service.ShiftActor {
	actor := service.ShiftActor{UserID: c.GetString(middleware.ContextKeyUserID)}
	if perms, ok := c.Get(middleware.ContextKeyPermissions); ok {
		if list, ok := perms.([]string); ok {
			for _, p := range list {
				if p == "*" {
					actor.IsOwner = true
					break
				}
			}
		}
	}
	return actor
}

func shiftError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrShiftNotFound):
		response.NotFound(c, err.Error())
	case errors.Is(err, service.ErrShiftForbidden):
		response.Forbidden(c, err.Error())
	default:
		response.BadRequest(c, err.Error())
	}
}

// @Summary List shifts
// @Description Get a paginated list of shifts
// @Tags Shifts
// @Accept json
// @Produce json
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Param search query string false "Staff name or username"
// @Param status query string false "open | closed"
// @Success 200 {object} response.Response{data=response.PaginatedData{data=[]service.ShiftResponse}}
// @Failure 500 {object} response.Response
// @Router /api/shifts [get]
// @Security BearerAuth
func (h *ShiftHandler) List(c *gin.Context) {
	params := pagination.GetParams(c)

	// Trang Ca làm việc gửi status từ ô lọc; trước đây bị bỏ qua nên lọc vô tác dụng.
	status := c.Query("status")
	if status != "" && status != "open" && status != "closed" {
		response.BadRequest(c, "trạng thái ca không hợp lệ")
		return
	}

	result, err := h.svc.List(params, status)
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
		if errors.Is(err, service.ErrShiftNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "Failed to fetch shift")
		return
	}

	response.Success(c, result)
}

// @Summary Open shift
// @Description Open a new shift. Only one shift may be open in the whole shop (one cash drawer).
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
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /api/shifts/{id}/close [post]
// @Security BearerAuth
func (h *ShiftHandler) CloseShift(c *gin.Context) {
	id := c.Param("id")

	var req service.CloseShiftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	result, err := h.svc.CloseShift(id, &req, shiftActor(c))
	if err != nil {
		shiftError(c, err)
		return
	}

	response.Success(c, result)
}

// @Summary Record cash in/out
// @Description Record a non-sale cash movement (cash_in / cash_out) on an open shift. Cash-out cannot exceed the drawer amount.
// @Tags Shifts
// @Accept json
// @Produce json
// @Param id path string true "Shift ID"
// @Param request body service.HandoverRequest true "Request body"
// @Success 201 {object} response.Response{data=service.CashHandoverResponse}
// @Failure 400 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /api/shifts/{id}/handover [post]
// @Security BearerAuth
func (h *ShiftHandler) Handover(c *gin.Context) {
	id := c.Param("id")

	var req service.HandoverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	result, err := h.svc.Handover(id, &req, shiftActor(c))
	if err != nil {
		shiftError(c, err)
		return
	}

	response.Created(c, result)
}
