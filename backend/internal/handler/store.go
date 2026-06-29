package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/response"
)

type StoreHandler struct {
	svc *service.StoreService
}

func NewStoreHandler(svc *service.StoreService) *StoreHandler {
	_ = model.Store{}
	return &StoreHandler{svc: svc}
}

// List
// @Summary      List Stores
// @Description  Get paginated list of stores
// @Tags         Stores
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page      query  int    false  "Page number"
// @Param        page_size  query  int    false  "Page size"
// @Param        sort      query  string false  "Sort field"
// @Param        order     query  string false  "Sort order (asc/desc)"
// @Param        search    query  string false  "Search keyword"
// @Success      200       {object}  response.Response{data=response.PaginatedData{data=[]model.Store}}
// @Failure      500       {object}  response.Response
// @Router       /api/stores [get]
func (h *StoreHandler) List(c *gin.Context) {
	params := pagination.GetParams(c)
	stores, total, page, pageSize, err := h.svc.List(*params)
	if err != nil {
		response.InternalError(c, "Failed to fetch stores")
		return
	}
	response.Paginated(c, stores, total, page, pageSize)
}

// GetByID
// @Summary      Get Store by ID
// @Description  Get a single store by its ID
// @Tags         Stores
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string  true  "Store ID"
// @Success      200   {object}  response.Response{data=model.Store}
// @Failure      404   {object}  response.Response
// @Router       /api/stores/{id} [get]
func (h *StoreHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	store, err := h.svc.GetByID(id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, store)
}

// Create
// @Summary      Create Store
// @Description  Create a new store
// @Tags         Stores
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  service.CreateStoreRequest  true  "Store data"
// @Success      201   {object}  response.Response{data=model.Store}
// @Failure      400   {object}  response.Response
// @Router       /api/stores [post]
func (h *StoreHandler) Create(c *gin.Context) {
	var req service.CreateStoreRequest
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

// Update
// @Summary      Update Store
// @Description  Update an existing store
// @Tags         Stores
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string                     true  "Store ID"
// @Param        body  body  service.UpdateStoreRequest  true  "Store update data"
// @Success      200   {object}  response.Response{data=model.Store}
// @Failure      400   {object}  response.Response
// @Failure      404   {object}  response.Response
// @Router       /api/stores/{id} [put]
func (h *StoreHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req service.UpdateStoreRequest
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

// Delete
// @Summary      Delete Store
// @Description  Delete a store by its ID
// @Tags         Stores
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string  true  "Store ID"
// @Success      200   {object}  response.Response
// @Failure      400   {object}  response.Response
// @Failure      404   {object}  response.Response
// @Router       /api/stores/{id} [delete]
func (h *StoreHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.Delete(id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}
