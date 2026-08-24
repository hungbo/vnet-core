package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/response"
)

type AppUpdateHandler struct {
	svc     *service.AppUpdateService
	machine *service.MachineService
}

func NewAppUpdateHandler(svc *service.AppUpdateService, machine *service.MachineService) *AppUpdateHandler {
	return &AppUpdateHandler{svc: svc, machine: machine}
}

// List
// @Summary      Danh sách bản cập nhật đã công bố
// @Tags         AppUpdates
// @Produce      json
// @Success      200  {object}  response.Response
// @Router       /api/app-updates [get]
func (h *AppUpdateHandler) List(c *gin.Context) {
	var req service.AppUpdateListRequest
	_ = c.ShouldBindQuery(&req)
	res, err := h.svc.List(&req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Paginated(c, res.Items, res.Total, res.Page, res.PageSize)
}

// Create
// @Summary      Công bố một bản cập nhật
// @Description  Băm SHA-256 là bắt buộc: máy trạm kiểm băm trước khi chạy tệp tải về.
// @Tags         AppUpdates
// @Accept       json
// @Produce      json
// @Success      201  {object}  response.Response
// @Router       /api/app-updates [post]
func (h *AppUpdateHandler) Create(c *gin.Context) {
	var req service.CreateAppUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	res, err := h.svc.Create(&req, middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, res)
}

type setActiveRequest struct {
	IsActive bool `json:"is_active"`
}

// SetActive
// @Summary      Bật/tắt một bản cập nhật
// @Tags         AppUpdates
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "Update ID"
// @Success      200  {object}  response.Response
// @Router       /api/app-updates/{id}/active [put]
func (h *AppUpdateHandler) SetActive(c *gin.Context) {
	var req setActiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.svc.SetActive(c.Param("id"), req.IsActive, middleware.GetUserID(c)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// Delete
// @Summary      Xoá một bản cập nhật
// @Tags         AppUpdates
// @Produce      json
// @Param        id  path  string  true  "Update ID"
// @Success      200  {object}  response.Response
// @Router       /api/app-updates/{id} [delete]
func (h *AppUpdateHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Param("id"), middleware.GetUserID(c)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// Latest
// @Summary      Máy trạm hỏi có bản mới hơn không
// @Description  Xác thực bằng khoá máy trạm như heartbeat.
// @Tags         AppUpdates
// @Produce      json
// @Param        code      path   string  true   "Machine code"
// @Param        platform  query  string  true   "windows-amd64"
// @Param        current   query  string  false  "Phiên bản đang chạy"
// @Success      200  {object}  response.Response
// @Failure      401  {object}  response.Response
// @Router       /api/machines/by-code/{code}/app-update [get]
func (h *AppUpdateHandler) Latest(c *gin.Context) {
	code := c.Param("code")
	if err := h.machine.VerifyAgentToken(code, agentToken(c)); err != nil {
		response.Unauthorized(c, err.Error())
		return
	}
	res, err := h.svc.Latest(c.Query("platform"), c.Query("current"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, res)
}
