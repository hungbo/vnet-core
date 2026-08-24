package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/response"
)

type InventoryCountHandler struct {
	svc *service.InventoryCountService
}

func NewInventoryCountHandler(svc *service.InventoryCountService) *InventoryCountHandler {
	return &InventoryCountHandler{svc: svc}
}

// List
// @Summary      Danh sách phiên kiểm kê
// @Tags         InventoryCount
// @Produce      json
// @Success      200  {object}  response.Response
// @Router       /api/inventory-counts [get]
func (h *InventoryCountHandler) List(c *gin.Context) {
	var req service.CountListRequest
	_ = c.ShouldBindQuery(&req)
	res, err := h.svc.List(&req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Paginated(c, res.Items, res.Total, res.Page, res.PageSize)
}

// GetByID
// @Summary      Chi tiết phiên kiểm kê kèm các dòng đếm
// @Tags         InventoryCount
// @Produce      json
// @Param        id  path  string  true  "Session ID"
// @Success      200  {object}  response.Response
// @Router       /api/inventory-counts/{id} [get]
func (h *InventoryCountHandler) GetByID(c *gin.Context) {
	res, err := h.svc.GetByID(c.Param("id"))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, res)
}

// Open
// @Summary      Mở phiên kiểm kê
// @Tags         InventoryCount
// @Accept       json
// @Produce      json
// @Success      201  {object}  response.Response
// @Router       /api/inventory-counts [post]
func (h *InventoryCountHandler) Open(c *gin.Context) {
	var req service.OpenCountRequest
	_ = c.ShouldBindJSON(&req)
	res, err := h.svc.Open(&req, middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, res)
}

// SetLine
// @Summary      Ghi số đếm thực của một mặt hàng
// @Description  Gọi lại cùng sản phẩm sẽ ghi đè dòng cũ.
// @Tags         InventoryCount
// @Accept       json
// @Produce      json
// @Param        id    path  string                     true  "Session ID"
// @Param        body  body  service.CountLineRequest   true  "Sản phẩm và số đếm"
// @Success      200  {object}  response.Response
// @Router       /api/inventory-counts/{id}/lines [post]
func (h *InventoryCountHandler) SetLine(c *gin.Context) {
	var req service.CountLineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	res, err := h.svc.SetLine(c.Param("id"), &req, middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, res)
}

// RemoveLine
// @Summary      Bỏ một dòng đếm khỏi phiên
// @Tags         InventoryCount
// @Produce      json
// @Param        id          path  string  true  "Session ID"
// @Param        product_id  path  string  true  "Product ID"
// @Success      200  {object}  response.Response
// @Router       /api/inventory-counts/{id}/lines/{product_id} [delete]
func (h *InventoryCountHandler) RemoveLine(c *gin.Context) {
	if err := h.svc.RemoveLine(c.Param("id"), c.Param("product_id")); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// Commit
// @Summary      Chốt phiên kiểm kê và điều chỉnh tồn kho
// @Tags         InventoryCount
// @Produce      json
// @Param        id  path  string  true  "Session ID"
// @Success      200  {object}  response.Response
// @Router       /api/inventory-counts/{id}/commit [post]
func (h *InventoryCountHandler) Commit(c *gin.Context) {
	res, err := h.svc.Commit(c.Param("id"), middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, res)
}

// Cancel
// @Summary      Huỷ phiên kiểm kê, không đụng tồn kho
// @Tags         InventoryCount
// @Produce      json
// @Param        id  path  string  true  "Session ID"
// @Success      200  {object}  response.Response
// @Router       /api/inventory-counts/{id}/cancel [post]
func (h *InventoryCountHandler) Cancel(c *gin.Context) {
	if err := h.svc.Cancel(c.Param("id"), middleware.GetUserID(c)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}
