package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/response"
)

type WebsiteBlockHandler struct {
	svc *service.WebsiteBlockService
}

func NewWebsiteBlockHandler(svc *service.WebsiteBlockService) *WebsiteBlockHandler {
	return &WebsiteBlockHandler{svc: svc}
}

// ListRules
// @Summary      Danh sách luật chặn website
// @Tags         WebsiteBlocking
// @Produce      json
// @Success      200  {object}  response.Response
// @Router       /api/website-rules [get]
func (h *WebsiteBlockHandler) ListRules(c *gin.Context) {
	var req service.RuleListRequest
	_ = c.ShouldBindQuery(&req)
	res, err := h.svc.ListRules(&req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Paginated(c, res.Items, res.Total, res.Page, res.PageSize)
}

// GetRule
// @Summary      Chi tiết luật kèm lịch và phạm vi máy
// @Tags         WebsiteBlocking
// @Produce      json
// @Param        id  path  string  true  "Rule ID"
// @Success      200  {object}  response.Response
// @Router       /api/website-rules/{id} [get]
func (h *WebsiteBlockHandler) GetRule(c *gin.Context) {
	res, err := h.svc.GetRule(c.Param("id"))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, res)
}

// CreateRule
// @Summary      Thêm luật chặn website
// @Tags         WebsiteBlocking
// @Accept       json
// @Produce      json
// @Success      201  {object}  response.Response
// @Router       /api/website-rules [post]
func (h *WebsiteBlockHandler) CreateRule(c *gin.Context) {
	var req service.CreateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	res, err := h.svc.CreateRule(&req, middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, res)
}

// UpdateRule
// @Summary      Sửa luật chặn website
// @Tags         WebsiteBlocking
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "Rule ID"
// @Success      200  {object}  response.Response
// @Router       /api/website-rules/{id} [put]
func (h *WebsiteBlockHandler) UpdateRule(c *gin.Context) {
	var req service.UpdateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	res, err := h.svc.UpdateRule(c.Param("id"), &req, middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, res)
}

// DeleteRule
// @Summary      Xoá luật chặn website
// @Tags         WebsiteBlocking
// @Produce      json
// @Param        id  path  string  true  "Rule ID"
// @Success      200  {object}  response.Response
// @Failure 409 {object} response.Response "còn dữ liệu phụ thuộc"
// @Router       /api/website-rules/{id} [delete]
func (h *WebsiteBlockHandler) DeleteRule(c *gin.Context) {
	if err := h.svc.DeleteRule(c.Param("id"), middleware.GetUserID(c)); err != nil {
		handleDeleteError(c, err)
		return
	}
	response.Success(c, nil)
}

// SetSchedules
// @Summary      Đặt lịch áp dụng cho luật (thay toàn bộ)
// @Description  Không có lịch nào nghĩa là luật áp dụng 24/7.
// @Tags         WebsiteBlocking
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "Rule ID"
// @Success      200  {object}  response.Response
// @Router       /api/website-rules/{id}/schedules [put]
func (h *WebsiteBlockHandler) SetSchedules(c *gin.Context) {
	var req service.SetSchedulesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.svc.SetSchedules(c.Param("id"), &req, middleware.GetUserID(c)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// SetGroups
// @Summary      Gán luật cho nhóm máy (thay toàn bộ)
// @Description  Danh sách rỗng nghĩa là áp cho mọi máy.
// @Tags         WebsiteBlocking
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "Rule ID"
// @Success      200  {object}  response.Response
// @Router       /api/website-rules/{id}/groups [put]
func (h *WebsiteBlockHandler) SetGroups(c *gin.Context) {
	var req service.SetGroupsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.svc.SetGroups(c.Param("id"), &req, middleware.GetUserID(c)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// EffectiveForMachine
// @Summary      Danh sách tên miền máy trạm phải chặn ngay lúc này
// @Description  Máy trạm gọi định kỳ. Xác thực bằng khoá máy trạm như heartbeat.
// @Tags         WebsiteBlocking
// @Produce      json
// @Param        code  path  string  true  "Machine code"
// @Success      200  {object}  response.Response
// @Failure      401  {object}  response.Response
// @Router       /api/machines/by-code/{code}/blocklist [get]
func (h *WebsiteBlockHandler) EffectiveForMachine(c *gin.Context) {
	code := c.Param("code")
	res, err := h.svc.EffectiveFor(code, time.Now())
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, res)
}

// ReportViolation
// @Summary      Máy trạm báo một lần truy cập bị chặn
// @Tags         WebsiteBlocking
// @Accept       json
// @Produce      json
// @Param        code  path  string  true  "Machine code"
// @Success      200  {object}  response.Response
// @Failure      401  {object}  response.Response
// @Router       /api/machines/by-code/{code}/blocklist/violations [post]
func (h *WebsiteBlockHandler) ReportViolation(c *gin.Context) {
	code := c.Param("code")
	var req service.ReportViolationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	if err := h.svc.ReportViolation(code, &req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// ListViolations
// @Summary      Nhật ký các lần truy cập bị chặn
// @Tags         WebsiteBlocking
// @Produce      json
// @Success      200  {object}  response.Response
// @Router       /api/website-violations [get]
func (h *WebsiteBlockHandler) ListViolations(c *gin.Context) {
	var req service.ViolationListRequest
	_ = c.ShouldBindQuery(&req)
	res, err := h.svc.ListViolations(&req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Paginated(c, res.Items, res.Total, res.Page, res.PageSize)
}
