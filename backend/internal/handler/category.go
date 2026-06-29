package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/response"
)

type CategoryHandler struct {
	svc *service.CategoryService
}

func NewCategoryHandler(svc *service.CategoryService) *CategoryHandler {
	return &CategoryHandler{svc: svc}
}

// @Summary List all categories
// @Description Get a list of all product categories
// @Tags Categories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response{data=[]service.CategoryResponse}
// @Failure 500 {object} response.Response
// @Router /categories [get]
func (h *CategoryHandler) List(c *gin.Context) {
	result, err := h.svc.List()
	if err != nil {
		response.InternalError(c, "Failed to fetch categories")
		return
	}

	response.Success(c, result)
}

// @Summary Get category by ID
// @Description Get a single product category by its ID
// @Tags Categories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Category ID"
// @Success 200 {object} response.Response{data=service.CategoryResponse}
// @Failure 404 {object} response.Response
// @Router /categories/{id} [get]
func (h *CategoryHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	result, err := h.svc.GetByID(id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}

	response.Success(c, result)
}

// @Summary Create a new category
// @Description Create a new product category
// @Tags Categories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body service.CreateCategoryRequest true "Category data"
// @Success 201 {object} response.Response{data=service.CategoryResponse}
// @Failure 400 {object} response.Response
// @Router /categories [post]
func (h *CategoryHandler) Create(c *gin.Context) {
	var req service.CreateCategoryRequest
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

// @Summary Update a category
// @Description Update an existing product category by ID
// @Tags Categories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Category ID"
// @Param body body service.UpdateCategoryRequest true "Category update data"
// @Success 200 {object} response.Response{data=service.CategoryResponse}
// @Failure 400 {object} response.Response
// @Router /categories/{id} [put]
func (h *CategoryHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req service.UpdateCategoryRequest
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

// @Summary Delete a category
// @Description Delete a product category by ID
// @Tags Categories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Category ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Router /categories/{id} [delete]
func (h *CategoryHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.svc.Delete(id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, nil)
}
