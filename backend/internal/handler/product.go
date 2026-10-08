package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/jwt"
	"github.com/vnet/core/pkg/response"
)

type ProductHandler struct {
	svc *service.ProductService
}

func NewProductHandler(svc *service.ProductService) *ProductHandler {
	return &ProductHandler{svc: svc}
}

// @Summary List all products
// @Description Get a list of all products, optionally filtered by category
// @Tags Products
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param category_id query string false "Filter by category ID"
// @Param supplier_id query string false "Filter by supplier ID"
// @Param is_active query bool false "Filter by status (staff only; members always get active products)"
// @Success 200 {object} response.Response{data=[]service.ProductResponse}
// @Failure 500 {object} response.Response
// @Router /products [get]
func (h *ProductHandler) List(c *gin.Context) {
	var isRetail *bool
	if q := c.Query("is_retail"); q != "" {
		v := q == "true"
		isRetail = &v
	}
	// Máy trạm (token hội viên) chỉ thấy món đang bán: món ngưng bán phải biến
	// khỏi thực đơn mà không cần cập nhật phần mềm máy trạm.
	var isActive *bool
	if middleware.GetKind(c) != jwt.KindStaff {
		v := true
		isActive = &v
	} else if q := c.Query("is_active"); q != "" {
		v := q == "true"
		isActive = &v
	}
	search := c.Query("search")
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))

	result, err := h.svc.List(isRetail, isActive, c.Query("category_id"), c.Query("supplier_id"), search, page, pageSize)
	if err != nil {
		response.InternalError(c, "Failed to fetch products")
		return
	}

	response.Paginated(c, result.Items, result.Total, result.Page, result.PageSize)
}

// @Summary Get product by ID
// @Description Get a single product by its ID
// @Tags Products
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Product ID"
// @Success 200 {object} response.Response{data=service.ProductResponse}
// @Failure 404 {object} response.Response
// @Router /products/{id} [get]
func (h *ProductHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	result, err := h.svc.GetByID(id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}

	response.Success(c, result)
}

// @Summary Create a new product
// @Description Create a new product
// @Tags Products
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body service.CreateProductRequest true "Product data"
// @Success 201 {object} response.Response{data=service.ProductResponse}
// @Failure 400 {object} response.Response
// @Router /products [post]
func (h *ProductHandler) Create(c *gin.Context) {
	var req service.CreateProductRequest
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

// @Summary Update a product
// @Description Update an existing product by ID
// @Tags Products
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Product ID"
// @Param body body service.UpdateProductRequest true "Product update data"
// @Success 200 {object} response.Response{data=service.ProductResponse}
// @Failure 400 {object} response.Response
// @Router /products/{id} [put]
func (h *ProductHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req service.UpdateProductRequest
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

// @Summary Delete a product
// @Description Delete a product by ID
// @Tags Products
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Product ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 409 {object} response.Response "còn dữ liệu phụ thuộc"
// @Router /products/{id} [delete]
func (h *ProductHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.svc.Delete(id); err != nil {
		handleDeleteError(c, err)
		return
	}

	response.Success(c, nil)
}
