package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/response"
)

type InventoryHandler struct {
	svc *service.InventoryService
}

func NewInventoryHandler(svc *service.InventoryService) *InventoryHandler {
	return &InventoryHandler{svc: svc}
}

// @Summary List suppliers
// @Description Get a list of suppliers
// @Tags Suppliers
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]service.SupplierResponse}
// @Failure 500 {object} response.Response
// @Router /api/suppliers [get]
// @Security BearerAuth
func (h *InventoryHandler) ListSuppliers(c *gin.Context) {
	result, err := h.svc.ListSuppliers()
	if err != nil {
		response.InternalError(c, "Failed to fetch suppliers")
		return
	}

	response.Success(c, result)
}

// @Summary Create supplier
// @Description Create a new supplier
// @Tags Suppliers
// @Accept json
// @Produce json
// @Param request body service.CreateSupplierRequest true "Request body"
// @Success 201 {object} response.Response{data=service.SupplierResponse}
// @Failure 400 {object} response.Response
// @Router /api/suppliers [post]
// @Security BearerAuth
func (h *InventoryHandler) CreateSupplier(c *gin.Context) {
	var req service.CreateSupplierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	result, err := h.svc.CreateSupplier(&req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Created(c, result)
}

// @Summary Update supplier
// @Description Update an existing supplier
// @Tags Suppliers
// @Accept json
// @Produce json
// @Param id path string true "Supplier ID"
// @Param request body service.UpdateSupplierRequest true "Request body"
// @Success 200 {object} response.Response{data=service.SupplierResponse}
// @Failure 400 {object} response.Response
// @Router /api/suppliers/{id} [put]
// @Security BearerAuth
func (h *InventoryHandler) UpdateSupplier(c *gin.Context) {
	id := c.Param("id")

	var req service.UpdateSupplierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	result, err := h.svc.UpdateSupplier(id, &req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, result)
}

// @Summary Delete supplier
// @Description Delete a supplier
// @Tags Suppliers
// @Accept json
// @Produce json
// @Param id path string true "Supplier ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Router /api/suppliers/{id} [delete]
// @Security BearerAuth
func (h *InventoryHandler) DeleteSupplier(c *gin.Context) {
	id := c.Param("id")

	if err := h.svc.DeleteSupplier(id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// @Summary List warehouses
// @Description Get a list of warehouses
// @Tags Warehouses
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]service.WarehouseResponse}
// @Failure 500 {object} response.Response
// @Router /api/warehouses [get]
// @Security BearerAuth
func (h *InventoryHandler) ListWarehouses(c *gin.Context) {
	result, err := h.svc.ListWarehouses()
	if err != nil {
		response.InternalError(c, "Failed to fetch warehouses")
		return
	}

	response.Success(c, result)
}

// @Summary Create warehouse
// @Description Create a new warehouse
// @Tags Warehouses
// @Accept json
// @Produce json
// @Param request body service.CreateWarehouseRequest true "Request body"
// @Success 201 {object} response.Response{data=service.WarehouseResponse}
// @Failure 400 {object} response.Response
// @Router /api/warehouses [post]
// @Security BearerAuth
func (h *InventoryHandler) CreateWarehouse(c *gin.Context) {
	var req service.CreateWarehouseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	result, err := h.svc.CreateWarehouse(&req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Created(c, result)
}

// @Summary Update warehouse
// @Description Update an existing warehouse
// @Tags Warehouses
// @Accept json
// @Produce json
// @Param id path string true "Warehouse ID"
// @Param request body service.UpdateWarehouseRequest true "Request body"
// @Success 200 {object} response.Response{data=service.WarehouseResponse}
// @Failure 400 {object} response.Response
// @Router /api/warehouses/{id} [put]
// @Security BearerAuth
func (h *InventoryHandler) UpdateWarehouse(c *gin.Context) {
	id := c.Param("id")

	var req service.UpdateWarehouseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	result, err := h.svc.UpdateWarehouse(id, &req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, result)
}

// @Summary Delete warehouse
// @Description Delete a warehouse
// @Tags Warehouses
// @Accept json
// @Produce json
// @Param id path string true "Warehouse ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Router /api/warehouses/{id} [delete]
// @Security BearerAuth
func (h *InventoryHandler) DeleteWarehouse(c *gin.Context) {
	id := c.Param("id")

	if err := h.svc.DeleteWarehouse(id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// @Summary List stock transactions
// @Description Get a paginated list of stock transactions
// @Tags StockTransactions
// @Accept json
// @Produce json
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200 {object} response.Response{data=response.PaginatedData{data=[]service.StockTransactionResponse}}
// @Failure 500 {object} response.Response
// @Router /api/stock-transactions [get]
// @Security BearerAuth
func (h *InventoryHandler) ListStockTransactions(c *gin.Context) {
	params := pagination.GetParams(c)

	result, err := h.svc.ListStockTransactions(params)
	if err != nil {
		response.InternalError(c, "Failed to fetch stock transactions")
		return
	}

	response.Paginated(c, result.Items, result.Total, result.Page, result.PageSize)
}

// @Summary Create stock transaction
// @Description Create a new stock transaction
// @Tags StockTransactions
// @Accept json
// @Produce json
// @Param request body service.CreateStockTransactionRequest true "Request body"
// @Success 201 {object} response.Response{data=service.StockTransactionResponse}
// @Failure 400 {object} response.Response
// @Router /api/stock-transactions [post]
// @Security BearerAuth
func (h *InventoryHandler) CreateStockTransaction(c *gin.Context) {
	var req service.CreateStockTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	userID := middleware.GetUserID(c)

	result, err := h.svc.CreateStockTransaction(&req, userID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Created(c, result)
}

// @Summary List product ingredients
// @Description Get ingredients linked to a product
// @Tags Products
// @Accept json
// @Produce json
// @Param productId path string true "Product ID"
// @Success 200 {object} response.Response{data=[]service.ProductIngredientResponse}
// @Router /products/{productId}/ingredients [get]
// @Security BearerAuth
func (h *InventoryHandler) ListProductIngredients(c *gin.Context) {
	productID := c.Param("id")

	result, err := h.svc.ListProductIngredients(productID)
	if err != nil {
		response.InternalError(c, "Failed to fetch product ingredients")
		return
	}

	response.Success(c, result)
}

// @Summary Add ingredient to product
// @Description Link an ingredient to a product
// @Tags Products
// @Accept json
// @Produce json
// @Param productId path string true "Product ID"
// @Param request body service.CreateProductIngredientRequest true "Request body"
// @Success 201 {object} response.Response{data=service.ProductIngredientResponse}
// @Router /products/{productId}/ingredients [post]
// @Security BearerAuth
func (h *InventoryHandler) CreateProductIngredient(c *gin.Context) {
	productID := c.Param("id")

	var req service.CreateProductIngredientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	result, err := h.svc.CreateProductIngredient(productID, &req)
	if err != nil {
		handleCreateError(c, err)
		return
	}

	response.Created(c, result)
}

// @Summary Update product ingredient
// @Description Update quantity of a product-ingredient link
// @Tags Products
// @Accept json
// @Produce json
// @Param id path string true "ProductIngredient ID"
// @Param request body service.UpdateProductIngredientRequest true "Request body"
// @Success 200 {object} response.Response{data=service.ProductIngredientResponse}
// @Router /products/ingredients/{id} [put]
// @Security BearerAuth
func (h *InventoryHandler) UpdateProductIngredient(c *gin.Context) {
	id := c.Param("ingredientId")

	var req service.UpdateProductIngredientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	result, err := h.svc.UpdateProductIngredient(id, &req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, result)
}

// @Summary Remove ingredient from product
// @Description Delete a product-ingredient link
// @Tags Products
// @Accept json
// @Produce json
// @Param id path string true "ProductIngredient ID"
// @Success 200 {object} response.Response
// @Router /products/ingredients/{id} [delete]
// @Security BearerAuth
func (h *InventoryHandler) DeleteProductIngredient(c *gin.Context) {
	id := c.Param("ingredientId")

	if err := h.svc.DeleteProductIngredient(id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// @Summary List units
// @Description Get a list of measurement units
// @Tags Units
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]service.UnitResponse}
// @Failure 500 {object} response.Response
// @Router /api/units [get]
// @Security BearerAuth
func (h *InventoryHandler) ListUnits(c *gin.Context) {
	result := service.ListUnits()
	response.Success(c, result)
}
