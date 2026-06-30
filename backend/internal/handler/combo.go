package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/response"
)

type ComboHandler struct {
	svc *service.ComboService
}

func NewComboHandler(svc *service.ComboService) *ComboHandler {
	return &ComboHandler{svc: svc}
}

// List retrieves all combos with pagination
// @Summary List combos
// @Description Get a paginated list of combos with optional search and sorting
// @Tags Combos
// @Accept json
// @Produce json
// @Param request query service.ComboListRequest false "List query params"
// @Success 200 {object} response.Response{data=response.PaginatedData{data=[]service.ComboResponse}}
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /combos [get]
// @Security BearerAuth
func (h *ComboHandler) List(c *gin.Context) {
	var req service.ComboListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	result, err := h.svc.List(&req)
	if err != nil {
		response.InternalError(c, "Failed to fetch combos")
		return
	}

	response.Success(c, result)
}

// GetByID retrieves a single combo by ID
// @Summary Get combo by ID
// @Description Get detailed information about a specific combo
// @Tags Combos
// @Accept json
// @Produce json
// @Param id path string true "Combo ID"
// @Success 200 {object} response.Response{data=service.ComboResponse}
// @Failure 404 {object} response.Response
// @Router /combos/{id} [get]
// @Security BearerAuth
func (h *ComboHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	result, err := h.svc.GetByID(id)
	if err != nil {
		response.NotFound(c, "Combo not found")
		return
	}

	response.Success(c, result)
}

// Create creates a new combo
// @Summary Create combo
// @Description Create a new combo with the provided details
// @Tags Combos
// @Accept json
// @Produce json
// @Param request body service.CreateComboRequest true "Combo details"
// @Success 201 {object} response.Response{data=service.ComboResponse}
// @Failure 400 {object} response.Response
// @Router /combos [post]
// @Security BearerAuth
func (h *ComboHandler) Create(c *gin.Context) {
	var req service.CreateComboRequest
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

// Update updates an existing combo
// @Summary Update combo
// @Description Update the details of an existing combo
// @Tags Combos
// @Accept json
// @Produce json
// @Param id path string true "Combo ID"
// @Param request body service.UpdateComboRequest true "Combo details"
// @Success 200 {object} response.Response{data=service.ComboResponse}
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /combos/{id} [put]
// @Security BearerAuth
func (h *ComboHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req service.UpdateComboRequest
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

// Delete deletes a combo
// @Summary Delete combo
// @Description Soft delete a combo by ID
// @Tags Combos
// @Accept json
// @Produce json
// @Param id path string true "Combo ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Router /combos/{id} [delete]
// @Security BearerAuth
func (h *ComboHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.svc.Delete(id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// Purchase purchases a combo for a member
// @Summary Purchase combo
// @Description Purchase a combo for a member or walk-in customer
// @Tags Combos
// @Accept json
// @Produce json
// @Param id path string true "Combo ID"
// @Param request body service.PurchaseComboRequest true "Purchase details"
// @Success 201 {object} response.Response{data=service.ComboPurchaseResponse}
// @Failure 400 {object} response.Response
// @Router /combos/{id}/purchase [post]
// @Security BearerAuth
func (h *ComboHandler) Purchase(c *gin.Context) {
	id := c.Param("id")

	var req service.PurchaseComboRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	userID := c.GetString(middleware.ContextKeyUserID)

	result, err := h.svc.Purchase(id, &req, "", userID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Created(c, result)
}

// Activate activates a purchased combo
// @Summary Activate combo
// @Description Activate a purchased combo on a specific machine
// @Tags Combos
// @Accept json
// @Produce json
// @Param id path string true "Purchase ID"
// @Param request body service.ActivateComboRequest true "Activation details"
// @Success 200 {object} response.Response{data=service.ComboPurchaseResponse}
// @Failure 400 {object} response.Response
// @Router /combos/{id}/activate [post]
// @Security BearerAuth
func (h *ComboHandler) Activate(c *gin.Context) {
	id := c.Param("id")

	var req service.ActivateComboRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	result, err := h.svc.Activate(id, &req, "")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, result)
}
