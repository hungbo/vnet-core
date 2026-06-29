package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/service"
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
		response.InternalError(c, "Failed to fetch daily revenue")
		return
	}
	response.Success(c, results)
}

// @Summary Monthly revenue report
// @Description Get monthly revenue report
// @Tags Reports
// @Accept json
// @Produce json
// @Param year query int false "Year"
// @Param month query int false "Month"
// @Success 200 {object} response.Response{data=[]service.MonthlyRevenueRow}
// @Failure 500 {object} response.Response
// @Router /api/reports/monthly-revenue [get]
// @Security BearerAuth
func (h *ReportHandler) MonthlyRevenue(c *gin.Context) {
	year, _ := strconv.Atoi(c.Query("year"))
	month, _ := strconv.Atoi(c.Query("month"))
	results, err := h.svc.MonthlyRevenue(year, month)
	if err != nil {
		response.InternalError(c, "Failed to fetch monthly revenue")
		return
	}
	response.Success(c, results)
}

// @Summary Member report
// @Description Get report by member
// @Tags Reports
// @Accept json
// @Produce json
// @Param date_from query string false "Start date"
// @Param date_to query string false "End date"
// @Param store_id query string false "Store ID"
// @Param limit query int false "Limit"
// @Success 200 {object} response.Response{data=[]service.ByMemberRow}
// @Failure 500 {object} response.Response
// @Router /api/reports/by-member [get]
// @Security BearerAuth
func (h *ReportHandler) ByMember(c *gin.Context) {
	params := parseReportParams(c)
	results, err := h.svc.ByMember(params)
	if err != nil {
		response.InternalError(c, "Failed to fetch member report")
		return
	}
	response.Success(c, results)
}

// @Summary Machine report
// @Description Get report by machine
// @Tags Reports
// @Accept json
// @Produce json
// @Param date_from query string false "Start date"
// @Param date_to query string false "End date"
// @Param store_id query string false "Store ID"
// @Param limit query int false "Limit"
// @Success 200 {object} response.Response{data=[]service.ByMachineRow}
// @Failure 500 {object} response.Response
// @Router /api/reports/by-machine [get]
// @Security BearerAuth
func (h *ReportHandler) ByMachine(c *gin.Context) {
	params := parseReportParams(c)
	results, err := h.svc.ByMachine(params)
	if err != nil {
		response.InternalError(c, "Failed to fetch machine report")
		return
	}
	response.Success(c, results)
}

// @Summary Employee report
// @Description Get report by employee
// @Tags Reports
// @Accept json
// @Produce json
// @Param date_from query string false "Start date"
// @Param date_to query string false "End date"
// @Param store_id query string false "Store ID"
// @Param limit query int false "Limit"
// @Success 200 {object} response.Response{data=[]service.ByEmployeeRow}
// @Failure 500 {object} response.Response
// @Router /api/reports/by-employee [get]
// @Security BearerAuth
func (h *ReportHandler) ByEmployee(c *gin.Context) {
	params := parseReportParams(c)
	results, err := h.svc.ByEmployee(params)
	if err != nil {
		response.InternalError(c, "Failed to fetch employee report")
		return
	}
	response.Success(c, results)
}

// @Summary Top products report
// @Description Get top products report
// @Tags Reports
// @Accept json
// @Produce json
// @Param date_from query string false "Start date"
// @Param date_to query string false "End date"
// @Param store_id query string false "Store ID"
// @Param limit query int false "Limit"
// @Success 200 {object} response.Response{data=[]service.TopProductRow}
// @Failure 500 {object} response.Response
// @Router /api/reports/top-products [get]
// @Security BearerAuth
func (h *ReportHandler) TopProducts(c *gin.Context) {
	params := parseReportParams(c)
	results, err := h.svc.TopProducts(params)
	if err != nil {
		response.InternalError(c, "Failed to fetch top products")
		return
	}
	response.Success(c, results)
}

// @Summary Promotion usage report
// @Description Get promotion usage report
// @Tags Reports
// @Accept json
// @Produce json
// @Param date_from query string false "Start date"
// @Param date_to query string false "End date"
// @Param store_id query string false "Store ID"
// @Param limit query int false "Limit"
// @Success 200 {object} response.Response{data=[]service.PromotionUsageRow}
// @Failure 500 {object} response.Response
// @Router /api/reports/promotion-usage [get]
// @Security BearerAuth
func (h *ReportHandler) PromotionUsage(c *gin.Context) {
	params := parseReportParams(c)
	results, err := h.svc.PromotionUsage(params)
	if err != nil {
		response.InternalError(c, "Failed to fetch promotion usage")
		return
	}
	response.Success(c, results)
}

func parseReportParams(c *gin.Context) service.ReportParams {
	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 {
		limit = 20
	}
	return service.ReportParams{
		DateFrom: c.Query("date_from"),
		DateTo:   c.Query("date_to"),
		Year:     parseIntSafe(c.Query("year")),
		Month:    parseIntSafe(c.Query("month")),
		StoreID:  c.Query("store_id"),
		Limit:    limit,
	}
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
		StoreID:         middleware.GetStoreID(c),
	}

	results, total, err := h.svc.ListTransactions(params)
	if err != nil {
		response.InternalError(c, "Failed to fetch transactions")
		return
	}
	response.Paginated(c, results, total, params.Page, params.PageSize)
}

func parseIntSafe(s string) int {
	if s == "" {
		return 0
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return v
}
