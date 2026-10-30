package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/response"
)

type ReportHandler struct {
	svc *service.ReportService
}

func NewReportHandler(svc *service.ReportService) *ReportHandler {
	return &ReportHandler{svc: svc}
}

// @Summary Daily revenue report
// @Description Get daily revenue report
// @Tags Reports
// @Accept json
// @Produce json
// @Param date_from query string false "Start date"
// @Param date_to query string false "End date"
// @Success 200 {object} response.Response{data=[]service.DailyRevenueRow}
// @Failure 500 {object} response.Response
// @Router /api/reports/daily-revenue [get]
// @Security BearerAuth
func (h *ReportHandler) DailyRevenue(c *gin.Context) {
	dateFrom := c.Query("date_from")
	dateTo := c.Query("date_to")
	results, err := h.svc.DailyRevenue(dateFrom, dateTo)
	if err != nil {
		response.InternalError(c, "Failed to fetch daily revenue", err)
		return
	}
	response.Success(c, results)
}

// @Summary Report overview
// @Description Mọi khoản tiền (bán món, nạp tiền, gói cước, hoàn tiền, tiền giờ) theo một bộ lọc chung: thẻ tổng, cộng bảng từng dòng (phân trang) hoặc bảng nhóm theo group_by
// @Tags Reports
// @Produce json
// @Param date_from query string false "Từ ngày, YYYY-MM-DD (giờ Việt Nam)"
// @Param date_to query string false "Đến ngày, YYYY-MM-DD (giờ Việt Nam)"
// @Param kind query string false "sale | topup | combo | refund | session"
// @Param employee_id query string false "Mã nhân viên, hoặc none = không có nhân viên"
// @Param member_id query string false "Mã hội viên"
// @Param machine_id query string false "Mã máy"
// @Param payment_method query string false "Mã phương thức thanh toán"
// @Param product_id query string false "Chỉ đơn có món này"
// @Param promotion_id query string false "Mã khuyến mãi"
// @Param search query string false "Mã đơn, hội viên, SĐT, máy, nhân viên, tên món, diễn giải"
// @Param group_by query string false "Rỗng = từng dòng; day | month | employee | member | machine | payment_method | kind | product | promotion"
// @Param page query int false "Trang (chỉ khi xem từng dòng)"
// @Param page_size query int false "Số dòng mỗi trang"
// @Param sort query string false "at, kind, code, member_name, machine_code, employee_name, payment_method, amount, money_in, discount, minutes"
// @Param order query string false "asc | desc"
// @Success 200 {object} response.Response{data=service.ReportOverview}
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /api/reports [get]
// @Security BearerAuth
func (h *ReportHandler) Overview(c *gin.Context) {
	var f service.ReportFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		handleValidationError(c, err)
		return
	}
	if err := f.Validate(); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := h.svc.Overview(f, *pagination.GetParams(c))
	if err != nil {
		response.InternalError(c, "Failed to fetch report", err)
		return
	}
	response.Success(c, result)
}

// @Summary Staff list for the report filter
// @Description Nhân viên cho ô lọc của trang Báo cáo — người xem báo cáo chưa chắc có quyền xem trang Tài khoản
// @Tags Reports
// @Produce json
// @Success 200 {object} response.Response{data=[]service.ReportEmployee}
// @Failure 500 {object} response.Response
// @Router /api/reports/employees [get]
// @Security BearerAuth
func (h *ReportHandler) Employees(c *gin.Context) {
	list, err := h.svc.Employees()
	if err != nil {
		response.InternalError(c, "Failed to fetch staff", err)
		return
	}
	response.Success(c, list)
}

// @Summary List transactions
// @Description Get paginated list of all member transactions
// @Tags Reports
// @Accept json
// @Produce json
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Param date_from query string false "Start date"
// @Param date_to query string false "End date"
// @Param transaction_type query string false "Filter by type (topup, session_fee, refund, etc)"
// @Param search query string false "Search by member name/phone/username"
// @Param sort query string false "Sort column (created_at, member_username, transaction_type, amount, ...)"
// @Param order query string false "Sort order (asc/desc)"
// @Success 200 {object} response.Response{data=response.PaginatedData{data=[]service.TransactionLogRow}}
// @Failure 500 {object} response.Response
// @Router /api/transactions [get]
// @Security BearerAuth
func (h *ReportHandler) ListTransactions(c *gin.Context) {
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))

	params := &service.TransactionListParams{
		Page:            page,
		PageSize:        pageSize,
		DateFrom:        c.Query("date_from"),
		DateTo:          c.Query("date_to"),
		TransactionType: c.Query("transaction_type"),
		Search:          c.Query("search"),
		Sort:            c.Query("sort"),
		Order:           c.Query("order"),
	}

	results, total, err := h.svc.ListTransactions(params)
	if err != nil {
		response.InternalError(c, "Failed to fetch transactions", err)
		return
	}
	response.Paginated(c, results, total, params.Page, params.PageSize)
}
