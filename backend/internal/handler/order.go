package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/response"
)

type OrderHandler struct {
	svc *service.OrderService
}

func NewOrderHandler(svc *service.OrderService) *OrderHandler {
	return &OrderHandler{svc: svc}
}

// @Summary List orders
// @Description Get a paginated list of orders for the current store
// @Tags Orders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200 {object} response.Response{data=response.PaginatedData{data=[]service.OrderResponse}}
// @Failure 500 {object} response.Response
// @Router /orders [get]
func (h *OrderHandler) List(c *gin.Context) {
	params := pagination.GetParams(c)
	result, total, page, pageSize, err := h.svc.List(*params)
	if err != nil {
		response.InternalError(c, "Failed to fetch orders")
		return
	}
	response.Paginated(c, result, total, page, pageSize)
}

// @Summary Get order by ID
// @Description Get a single order by its ID
// @Tags Orders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Order ID"
// @Success 200 {object} response.Response{data=service.OrderResponse}
// @Failure 404 {object} response.Response
// @Router /orders/{id} [get]
func (h *OrderHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	order, err := h.svc.GetByID(id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, order)
}

// @Summary Create a new order
// @Description Create a new order for the current store
// @Tags Orders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body service.CreateOrderRequest true "Order data"
// @Success 201 {object} response.Response{data=service.OrderResponse}
// @Failure 400 {object} response.Response
// @Router /orders [post]
func (h *OrderHandler) Create(c *gin.Context) {
	var req service.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	userID := middleware.GetUserID(c)
	result, err := h.svc.Create(req, userID, "")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, result)
}

// @Summary Update an order
// @Description Update an existing order by ID
// @Tags Orders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Order ID"
// @Param body body service.CreateOrderRequest true "Order update data"
// @Success 200 {object} response.Response{data=service.OrderResponse}
// @Failure 400 {object} response.Response
// @Router /orders/{id} [put]
func (h *OrderHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req service.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	result, err := h.svc.Update(id, req)
	if err != nil {
		handleCreateError(c, err)
		return
	}
	response.Success(c, result)
}

// @Summary Delete an order
// @Description Delete an order by ID
// @Tags Orders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Order ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Router /orders/{id} [delete]
func (h *OrderHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.Delete(id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// @Summary Batch delete orders
// @Description Delete multiple orders by IDs
// @Tags Orders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "Batch delete request"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Router /orders/batch-delete [delete]
func (h *OrderHandler) BatchDelete(c *gin.Context) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		handleValidationError(c, err)
		return
	}
	if err := h.svc.BatchDelete(body.IDs); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// @Summary Create topup order
// @Description Create a topup order that admin will approve
// @Tags Orders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body service.CreateTopupOrderRequest true "Topup data"
// @Success 201 {object} response.Response{data=service.OrderResponse}
// @Failure 400 {object} response.Response
// @Router /orders/topup-request [post]
func (h *OrderHandler) CreateTopup(c *gin.Context) {
	var req service.CreateTopupOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	userID := middleware.GetUserID(c)
	result, err := h.svc.CreateTopupOrder(req, userID, "")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, result)
}

// @Summary Update order status
// @Description Update the status of an order
// @Tags Orders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Order ID"
// @Param body body service.UpdateStatusRequest true "Status update data"
// @Success 200 {object} response.Response{data=service.OrderResponse}
// @Failure 400 {object} response.Response
// @Router /orders/{id}/status [patch]
func (h *OrderHandler) UpdateStatus(c *gin.Context) {
	id := c.Param("id")
	userID := middleware.GetUserID(c)
	var req service.UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	result, err := h.svc.UpdateStatus(id, userID, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

// @Summary Split an order
// @Description Split an order into multiple orders
// @Tags Orders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Order ID"
// @Param body body service.SplitOrderRequest true "Split order data"
// @Success 201 {object} response.Response{data=[]service.OrderResponse}
// @Failure 400 {object} response.Response
// @Router /orders/{id}/split [post]
func (h *OrderHandler) Split(c *gin.Context) {
	id := c.Param("id")
	var req service.SplitOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	result, err := h.svc.Split(id, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, result)
}

// @Summary Pay an order
// @Description Process payment for an order
// @Tags Orders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Order ID"
// @Param body body service.PayRequest true "Payment data"
// @Success 200 {object} response.Response{data=service.PaymentResponse}
// @Failure 400 {object} response.Response
// @Router /orders/{id}/pay [post]
func (h *OrderHandler) Pay(c *gin.Context) {
	id := c.Param("id")
	var req service.PayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	result, err := h.svc.Pay(id, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}
