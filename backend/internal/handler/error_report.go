package handler

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/response"
	"gorm.io/gorm"
)

type ErrorReportHandler struct {
	svc *service.ErrorReportService
}

func NewErrorReportHandler(svc *service.ErrorReportService) *ErrorReportHandler {
	return &ErrorReportHandler{svc: svc}
}

// Report
// @Summary      Trang quản trị gửi lỗi JS của chính nó
// @Tags         ErrorReports
// @Accept       json
// @Produce      json
// @Param        body  body      service.ErrorReportInput  true  "Lỗi"
// @Success      200   {object}  response.Response
// @Router       /api/error-reports [post]
// @Security     BearerAuth
func (h *ErrorReportHandler) Report(c *gin.Context) {
	var req service.ErrorReportInput
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	req.Source = service.ErrorSourceAdmin
	req.Reporter = c.GetString(middleware.ContextKeyUsername)
	if err := h.svc.Record(req); err != nil {
		response.InternalError(c, "Không ghi được lỗi", err)
		return
	}
	response.Success(c, nil)
}

// ReportFromMachine
// @Summary      Máy trạm gửi lỗi: crash lần chạy trước, lỗi giao diện, không mở được game
// @Tags         ErrorReports
// @Accept       json
// @Produce      json
// @Param        code  path      string                             true  "Mã máy"
// @Param        body  body      service.MachineErrorReportRequest  true  "Tối đa 20 lỗi mỗi lần"
// @Success      200   {object}  response.Response
// @Router       /api/machines/by-code/{code}/errors [post]
func (h *ErrorReportHandler) ReportFromMachine(c *gin.Context) {
	var req service.MachineErrorReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	if err := h.svc.RecordMachineErrors(c.Param("code"), req.Events); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// List
// @Summary      Nhật ký lỗi, gộp theo loại, gặp gần nhất lên đầu
// @Tags         ErrorReports
// @Produce      json
// @Param        source     query     string  false  "server | admin | client"
// @Param        status     query     string  false  "open (mặc định) | resolved | all"
// @Param        page       query     int     false  "Trang"
// @Param        page_size  query     int     false  "Số dòng mỗi trang"
// @Success      200        {object}  response.Response{data=response.PaginatedData{items=[]model.ErrorEvent}}
// @Router       /api/error-reports [get]
// @Security     BearerAuth
func (h *ErrorReportHandler) List(c *gin.Context) {
	params := pagination.GetParams(c)
	var f service.ErrorListParams
	_ = c.ShouldBindQuery(&f)
	items, total, page, pageSize, err := h.svc.List(*params, f)
	if err != nil {
		response.InternalError(c, "Không tải được nhật ký lỗi", err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

// Resolve
// @Summary      Đánh dấu lỗi đã sửa (gặp lại sẽ tự mở lại)
// @Tags         ErrorReports
// @Produce      json
// @Param        id   path      string  true  "ID"
// @Success      200  {object}  response.Response
// @Router       /api/error-reports/{id}/resolve [put]
// @Security     BearerAuth
func (h *ErrorReportHandler) Resolve(c *gin.Context) {
	if err := h.svc.Resolve(c.Param("id")); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "Không tìm thấy lỗi")
			return
		}
		response.InternalError(c, "Không cập nhật được", err)
		return
	}
	response.Success(c, nil)
}
