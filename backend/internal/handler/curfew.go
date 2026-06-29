package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/response"
)

type CurfewHandler struct {
	svc *service.CurfewService
}

func NewCurfewHandler(svc *service.CurfewService) *CurfewHandler {
	return &CurfewHandler{svc: svc}
}

// List retrieves all curfew policies with pagination
// @Summary List curfew policies
// @Description Get a paginated list of curfew policies with optional filters
// @Tags Curfew
// @Accept json
// @Produce json
// @Param request query service.CurfewListRequest false "List query params"
// @Success 200 {object} response.Response{data=response.PaginatedData{data=[]service.CurfewResponse}}
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /curfew [get]
// @Security BearerAuth
func (h *CurfewHandler) List(c *gin.Context) {
	var req service.CurfewListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	result, err := h.svc.List(&req)
	if err != nil {
		response.InternalError(c, "Failed to fetch curfew policies")
		return
	}

	response.Success(c, result)
}

// GetByID retrieves a single curfew policy by ID
// @Summary Get curfew policy by ID
// @Description Get detailed information about a specific curfew policy
// @Tags Curfew
// @Accept json
// @Produce json
// @Param id path string true "Curfew policy ID"
// @Success 200 {object} response.Response{data=service.CurfewResponse}
// @Failure 404 {object} response.Response
// @Router /curfew/{id} [get]
// @Security BearerAuth
func (h *CurfewHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	result, err := h.svc.GetByID(id)
	if err != nil {
		response.NotFound(c, "Curfew policy not found")
		return
	}

	response.Success(c, result)
}

// Create creates a new curfew policy
// @Summary Create curfew policy
// @Description Create a new curfew policy
// @Tags Curfew
// @Accept json
// @Produce json
// @Param request body service.CreateCurfewRequest true "Curfew policy details"
// @Success 201 {object} response.Response{data=service.CurfewResponse}
// @Failure 400 {object} response.Response
// @Router /curfew [post]
// @Security BearerAuth
func (h *CurfewHandler) Create(c *gin.Context) {
	var req service.CreateCurfewRequest
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

// Update updates an existing curfew policy
// @Summary Update curfew policy
// @Description Update the details of an existing curfew policy
// @Tags Curfew
// @Accept json
// @Produce json
// @Param id path string true "Curfew policy ID"
// @Param request body service.UpdateCurfewRequest true "Curfew policy details"
// @Success 200 {object} response.Response{data=service.CurfewResponse}
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /curfew/{id} [put]
// @Security BearerAuth
func (h *CurfewHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req service.UpdateCurfewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	result, err := h.svc.Update(id, &req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, result)
}

// Delete deletes a curfew policy
// @Summary Delete curfew policy
// @Description Soft delete a curfew policy by ID
// @Tags Curfew
// @Accept json
// @Produce json
// @Param id path string true "Curfew policy ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Router /curfew/{id} [delete]
// @Security BearerAuth
func (h *CurfewHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.svc.Delete(id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// Override overrides a curfew policy for a member
// @Summary Override curfew
// @Description Override a curfew policy for a specific member
// @Tags Curfew
// @Accept json
// @Produce json
// @Param request body service.OverrideCurfewRequest true "Override details"
// @Success 200 {object} response.Response{data=service.CurfewResponse}
// @Failure 400 {object} response.Response
// @Router /curfew/override [post]
// @Security BearerAuth
func (h *CurfewHandler) Override(c *gin.Context) {
	var req service.OverrideCurfewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	adminID := c.GetString(middleware.ContextKeyUserID)

	result, err := h.svc.Override(&req, adminID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, result)
}
