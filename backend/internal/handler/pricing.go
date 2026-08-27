package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/response"
)

type PricingHandler struct {
	svc *service.PricingService
}

func NewPricingHandler(svc *service.PricingService) *PricingHandler {
	return &PricingHandler{svc: svc}
}

// ListMachinePrices
// @Summary      Bảng giá theo hạng hội viên
// @Tags         Pricing
// @Produce      json
// @Param        machine_group_id  query  string  false  "Lọc theo nhóm máy"
// @Success      200  {object}  response.Response
// @Router       /api/machine-prices [get]
func (h *PricingHandler) ListMachinePrices(c *gin.Context) {
	res, err := h.svc.ListMachinePrices(c.Query("machine_group_id"))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, res)
}

// CreateMachinePrice
// @Summary      Thêm giá cho một hạng hội viên
// @Tags         Pricing
// @Accept       json
// @Produce      json
// @Success      201  {object}  response.Response
// @Router       /api/machine-prices [post]
func (h *PricingHandler) CreateMachinePrice(c *gin.Context) {
	var req service.MachinePriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	res, err := h.svc.CreateMachinePrice(&req, middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, res)
}

// UpdateMachinePrice
// @Summary      Sửa một dòng giá theo hạng
// @Tags         Pricing
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "Price ID"
// @Success      200  {object}  response.Response
// @Router       /api/machine-prices/{id} [put]
func (h *PricingHandler) UpdateMachinePrice(c *gin.Context) {
	var req service.MachinePriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	res, err := h.svc.UpdateMachinePrice(c.Param("id"), &req, middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, res)
}

// DeleteMachinePrice
// @Summary      Xoá một dòng giá theo hạng
// @Tags         Pricing
// @Produce      json
// @Param        id  path  string  true  "Price ID"
// @Success      200  {object}  response.Response
// @Failure 409 {object} response.Response "còn dữ liệu phụ thuộc"
// @Router       /api/machine-prices/{id} [delete]
func (h *PricingHandler) DeleteMachinePrice(c *gin.Context) {
	if err := h.svc.DeleteMachinePrice(c.Param("id"), middleware.GetUserID(c)); err != nil {
		handleDeleteError(c, err)
		return
	}
	response.Success(c, nil)
}

// ListTimePricing
// @Summary      Bảng giá theo khung giờ
// @Tags         Pricing
// @Produce      json
// @Param        machine_group_id  query  string  false  "Lọc theo nhóm máy"
// @Success      200  {object}  response.Response
// @Router       /api/time-pricing [get]
func (h *PricingHandler) ListTimePricing(c *gin.Context) {
	res, err := h.svc.ListTimePricing(c.Query("machine_group_id"))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, res)
}

// CreateTimePricing
// @Summary      Thêm khung giá cao/thấp điểm
// @Description  Khung chồng nhau trong cùng ngày bị từ chối: máy tính tiền không phân định được giá nào thắng.
// @Tags         Pricing
// @Accept       json
// @Produce      json
// @Success      201  {object}  response.Response
// @Router       /api/time-pricing [post]
func (h *PricingHandler) CreateTimePricing(c *gin.Context) {
	var req service.TimePricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	res, err := h.svc.CreateTimePricing(&req, middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, res)
}

// UpdateTimePricing
// @Summary      Sửa một khung giá
// @Tags         Pricing
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "Pricing ID"
// @Success      200  {object}  response.Response
// @Router       /api/time-pricing/{id} [put]
func (h *PricingHandler) UpdateTimePricing(c *gin.Context) {
	var req service.TimePricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	res, err := h.svc.UpdateTimePricing(c.Param("id"), &req, middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, res)
}

// DeleteTimePricing
// @Summary      Xoá một khung giá
// @Tags         Pricing
// @Produce      json
// @Param        id  path  string  true  "Pricing ID"
// @Success      200  {object}  response.Response
// @Failure 409 {object} response.Response "còn dữ liệu phụ thuộc"
// @Router       /api/time-pricing/{id} [delete]
func (h *PricingHandler) DeleteTimePricing(c *gin.Context) {
	if err := h.svc.DeleteTimePricing(c.Param("id"), middleware.GetUserID(c)); err != nil {
		handleDeleteError(c, err)
		return
	}
	response.Success(c, nil)
}
