package service

import (
	"sort"
	"time"

	"github.com/vnet/core/internal/model"
	"gorm.io/gorm"
)

type ReportService struct {
	db *gorm.DB
}

func NewReportService(db *gorm.DB) *ReportService {
	return &ReportService{db: db}
}

type DailyRevenueRow struct {
	Date        string `json:"date"`
	TotalOrders int64  `json:"total_orders"`
	Revenue     int64  `json:"revenue"`
	Discount    int64  `json:"discount"`
}

type MonthlyRevenueRow struct {
	Month       string `json:"month"`
	TotalOrders int64  `json:"total_orders"`
	Revenue     int64  `json:"revenue"`
	Discount    int64  `json:"discount"`
}

type ByMemberRow struct {
	MemberID   string `json:"member_id"`
	MemberName string `json:"member_name"`
	TotalSpent int64  `json:"total_spent"`
	VisitCount int64  `json:"visit_count"`
}

type ByMachineRow struct {
	MachineID   string `json:"machine_id"`
	MachineName string `json:"machine_name"`
	TotalSales  int64  `json:"total_sales"`
	UsageHours  int64  `json:"usage_hours"`
}

type ByEmployeeRow struct {
	EmployeeID   string `json:"employee_id"`
	EmployeeName string `json:"employee_name"`
	OrdersTaken  int64  `json:"orders_taken"`
	TotalSales   int64  `json:"total_sales"`
}

type TopProductRow struct {
	ProductID   string `json:"product_id"`
	ProductName string `json:"product_name"`
	Quantity    int64  `json:"quantity"`
	TotalSales  int64  `json:"total_sales"`
}

type PromotionUsageRow struct {
	PromotionID   string `json:"promotion_id"`
	PromotionName string `json:"promotion_name"`
	UsageCount    int64  `json:"usage_count"`
	DiscountGiven int64  `json:"discount_given"`
}

type TransactionLogRow struct {
	ID              string  `json:"id"`
	MemberID        string  `json:"member_id"`
	MemberName      string  `json:"member_name"`
	MemberUsername  string  `json:"member_username"`
	TransactionType string  `json:"transaction_type"`
	Amount          int64   `json:"amount"`
	BalanceBefore   int64   `json:"balance_before"`
	BalanceAfter    int64   `json:"balance_after"`
	BonusBefore     int64   `json:"bonus_before"`
	BonusAfter      int64   `json:"bonus_after"`
	PaymentMethod   string  `json:"payment_method"`
	Description     string  `json:"description"`
	CreatedBy       *string `json:"created_by"`
	CreatedByName   string  `json:"created_by_name"`
	CreatedAt       string  `json:"created_at"`
}

type TransactionListParams struct {
	Page            int    `form:"page"`
	PageSize        int    `form:"page_size"`
	DateFrom        string `form:"date_from"`
	DateTo          string `form:"date_to"`
	TransactionType string `form:"transaction_type"`
	Search          string `form:"search"`
}

type ReportParams struct {
	DateFrom string `form:"date_from"`
	DateTo   string `form:"date_to"`
	Year     int    `form:"year"`
	Month    int    `form:"month"`
	Limit    int    `form:"limit"`
}

type txRevenueRow struct {
	Date   string
	Amount int64
	Count  int64
}

func (s *ReportService) txRevenueQuery(dateFrom, dateTo string, dateExpr string) ([]txRevenueRow, error) {
	var results []txRevenueRow
	query := s.db.Table("member_transactions").
		Select(dateExpr+" as date, COALESCE(SUM(amount), 0) as amount, COUNT(*) as count").
		Where("transaction_type IN ('topup', 'session_fee')")

	if dateFrom != "" {
		if t, err := time.Parse("2006-01-02", dateFrom); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if dateTo != "" {
		if t, err := time.Parse("2006-01-02", dateTo); err == nil {
			query = query.Where("created_at <= ?", t.Add(24*time.Hour))
		}
	}

	query = query.Group("date").Order("date asc")
	if err := query.Find(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

func (s *ReportService) DailyRevenue(dateFrom, dateTo string) ([]DailyRevenueRow, error) {
	var orderResults []DailyRevenueRow
	orderQuery := s.db.Model(&model.Order{}).
		Select("DATE(created_at) as date, COUNT(*) as total_orders, COALESCE(SUM(final_amount), 0) as revenue, COALESCE(SUM(discount_amount), 0) as discount").
		Where("status = ?", "completed")

	if dateFrom != "" {
		if t, err := time.Parse("2006-01-02", dateFrom); err == nil {
			orderQuery = orderQuery.Where("created_at >= ?", t)
		}
	}
	if dateTo != "" {
		if t, err := time.Parse("2006-01-02", dateTo); err == nil {
			orderQuery = orderQuery.Where("created_at <= ?", t.Add(24*time.Hour))
		}
	}
	orderQuery = orderQuery.Group("DATE(created_at)").Order("date asc")
	if err := orderQuery.Find(&orderResults).Error; err != nil {
		return nil, err
	}

	txResults, err := s.txRevenueQuery(dateFrom, dateTo, "DATE(created_at)")
	if err != nil {
		return nil, err
	}

	dateMap := make(map[string]*DailyRevenueRow)
	for i := range orderResults {
		dateMap[orderResults[i].Date] = &orderResults[i]
	}
	for _, tx := range txResults {
		if existing, ok := dateMap[tx.Date]; ok {
			existing.Revenue += tx.Amount
			existing.TotalOrders += tx.Count
		} else {
			dateMap[tx.Date] = &DailyRevenueRow{Date: tx.Date, TotalOrders: tx.Count, Revenue: tx.Amount}
		}
	}

	results := make([]DailyRevenueRow, 0, len(dateMap))
	for _, r := range dateMap {
		results = append(results, *r)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Date < results[j].Date })
	return results, nil
}

func (s *ReportService) MonthlyRevenue(year, month int) ([]MonthlyRevenueRow, error) {
	var orderResults []MonthlyRevenueRow
	orderQuery := s.db.Model(&model.Order{}).
		Select("TO_CHAR(created_at, 'YYYY-MM') as month, COUNT(*) as total_orders, COALESCE(SUM(final_amount), 0) as revenue, COALESCE(SUM(discount_amount), 0) as discount").
		Where("status = ?", "completed")

	if year > 0 {
		orderQuery = orderQuery.Where("EXTRACT(YEAR FROM created_at) = ?", year)
	}
	if month > 0 {
		orderQuery = orderQuery.Where("EXTRACT(MONTH FROM created_at) = ?", month)
	}
	orderQuery = orderQuery.Group("month").Order("month asc")
	if err := orderQuery.Find(&orderResults).Error; err != nil {
		return nil, err
	}

	dateFrom, dateTo := "", ""
	if year > 0 {
		dateFrom = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		if month > 0 {
			dateTo = time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		} else {
			dateTo = time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		}
	}

	txResults, err := s.txRevenueQuery(dateFrom, dateTo, "TO_CHAR(created_at, 'YYYY-MM')")
	if err != nil {
		return nil, err
	}

	monthMap := make(map[string]*MonthlyRevenueRow)
	for i := range orderResults {
		monthMap[orderResults[i].Month] = &orderResults[i]
	}
	for _, tx := range txResults {
		if existing, ok := monthMap[tx.Date]; ok {
			existing.Revenue += tx.Amount
			existing.TotalOrders += tx.Count
		} else {
			monthMap[tx.Date] = &MonthlyRevenueRow{Month: tx.Date, TotalOrders: tx.Count, Revenue: tx.Amount}
		}
	}

	results := make([]MonthlyRevenueRow, 0, len(monthMap))
	for _, r := range monthMap {
		results = append(results, *r)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Month < results[j].Month })
	return results, nil
}

type txByMemberRow struct {
	MemberID string
	Amount   int64
	Count    int64
}

func (s *ReportService) ByMember(params ReportParams) ([]ByMemberRow, error) {
	var orderResults []ByMemberRow
	orderQuery := s.db.Model(&model.Order{}).
		Select("member_id, COUNT(*) as visit_count, COALESCE(SUM(final_amount), 0) as total_spent").
		Where("member_id IS NOT NULL AND status = ?", "completed")

	if params.DateFrom != "" {
		if t, err := time.Parse("2006-01-02", params.DateFrom); err == nil {
			orderQuery = orderQuery.Where("created_at >= ?", t)
		}
	}
	if params.DateTo != "" {
		if t, err := time.Parse("2006-01-02", params.DateTo); err == nil {
			orderQuery = orderQuery.Where("created_at <= ?", t.Add(24*time.Hour))
		}
	}
	orderQuery = orderQuery.Group("member_id").Order("total_spent desc")
	if params.Limit > 0 {
		orderQuery = orderQuery.Limit(params.Limit)
	}
	if err := orderQuery.Find(&orderResults).Error; err != nil {
		return nil, err
	}

	var txResults []txByMemberRow
	txQuery := s.db.Table("member_transactions").
		Select("member_id, COALESCE(SUM(amount), 0) as amount, COUNT(*) as count").
		Where("transaction_type IN ('topup', 'session_fee') AND member_id IS NOT NULL")

	if params.DateFrom != "" {
		if t, err := time.Parse("2006-01-02", params.DateFrom); err == nil {
			txQuery = txQuery.Where("created_at >= ?", t)
		}
	}
	if params.DateTo != "" {
		if t, err := time.Parse("2006-01-02", params.DateTo); err == nil {
			txQuery = txQuery.Where("created_at <= ?", t.Add(24*time.Hour))
		}
	}
	txQuery = txQuery.Group("member_id")
	if err := txQuery.Find(&txResults).Error; err != nil {
		return nil, err
	}

	memberMap := make(map[string]*ByMemberRow)
	for i := range orderResults {
		memberMap[orderResults[i].MemberID] = &orderResults[i]
	}
	for _, tx := range txResults {
		if existing, ok := memberMap[tx.MemberID]; ok {
			existing.TotalSpent += tx.Amount
			existing.VisitCount += tx.Count
		} else {
			memberMap[tx.MemberID] = &ByMemberRow{MemberID: tx.MemberID, TotalSpent: tx.Amount, VisitCount: tx.Count}
		}
	}

	results := make([]ByMemberRow, 0, len(memberMap))
	for _, r := range memberMap {
		results = append(results, *r)
	}

	for i := range results {
		var member model.Member
		if err := s.db.Select("full_name").Where("id = ?", results[i].MemberID).First(&member).Error; err == nil {
			results[i].MemberName = member.FullName
		}
	}

	sort.Slice(results, func(i, j int) bool { return results[i].TotalSpent > results[j].TotalSpent })
	if params.Limit > 0 && len(results) > params.Limit {
		results = results[:params.Limit]
	}
	return results, nil
}

func (s *ReportService) ByMachine(params ReportParams) ([]ByMachineRow, error) {
	var results []ByMachineRow

	query := s.db.Model(&model.MachineSession{}).
		Select("machine_id, COUNT(*) as usage_hours, COALESCE(SUM(total_cost), 0) as total_sales").
		Where("ended_at IS NOT NULL")

	if params.DateFrom != "" {
		if t, err := time.Parse("2006-01-02", params.DateFrom); err == nil {
			query = query.Where("started_at >= ?", t)
		}
	}
	if params.DateTo != "" {
		if t, err := time.Parse("2006-01-02", params.DateTo); err == nil {
			query = query.Where("started_at <= ?", t.Add(24*time.Hour))
		}
	}
	query = query.Group("machine_id").Order("total_sales desc")

	if params.Limit > 0 {
		query = query.Limit(params.Limit)
	}

	if err := query.Find(&results).Error; err != nil {
		return nil, err
	}

	for i := range results {
		var machine model.Machine
		if err := s.db.Select("machine_code").Where("id = ?", results[i].MachineID).First(&machine).Error; err == nil {
			results[i].MachineName = machine.MachineCode
		}
	}

	return results, nil
}

func (s *ReportService) ByEmployee(params ReportParams) ([]ByEmployeeRow, error) {
	var results []ByEmployeeRow

	query := s.db.Model(&model.Order{}).
		Select("created_by as employee_id, COUNT(*) as orders_taken, COALESCE(SUM(final_amount), 0) as total_sales").
		Where("created_by IS NOT NULL AND status = ?", "completed")

	if params.DateFrom != "" {
		if t, err := time.Parse("2006-01-02", params.DateFrom); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if params.DateTo != "" {
		if t, err := time.Parse("2006-01-02", params.DateTo); err == nil {
			query = query.Where("created_at <= ?", t.Add(24*time.Hour))
		}
	}
	query = query.Group("created_by").Order("total_sales desc")

	if params.Limit > 0 {
		query = query.Limit(params.Limit)
	}

	if err := query.Find(&results).Error; err != nil {
		return nil, err
	}

	for i := range results {
		var user model.User
		if err := s.db.Select("full_name").Where("id = ?", results[i].EmployeeID).First(&user).Error; err == nil {
			results[i].EmployeeName = user.FullName
		}
	}

	return results, nil
}

func (s *ReportService) TopProducts(params ReportParams) ([]TopProductRow, error) {
	var results []TopProductRow

	query := s.db.Model(&model.OrderItem{}).
		Select("product_id, product_name, SUM(quantity) as quantity, COALESCE(SUM(subtotal), 0) as total_sales")

	if params.DateFrom != "" {
		if t, err := time.Parse("2006-01-02", params.DateFrom); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if params.DateTo != "" {
		if t, err := time.Parse("2006-01-02", params.DateTo); err == nil {
			query = query.Where("created_at <= ?", t.Add(24*time.Hour))
		}
	}
	query = query.Group("product_id, product_name").Order("total_sales desc")

	if params.Limit > 0 {
		query = query.Limit(params.Limit)
	}

	if err := query.Find(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}

func (s *ReportService) PromotionUsage(params ReportParams) ([]PromotionUsageRow, error) {
	var results []PromotionUsageRow

	query := s.db.Model(&model.Order{}).
		Select("COALESCE(promotion_id, '') as promotion_id, COUNT(*) as usage_count, COALESCE(SUM(discount_amount), 0) as discount_given").
		Where("discount_amount > 0 AND status = ?", "completed")

	if params.DateFrom != "" {
		if t, err := time.Parse("2006-01-02", params.DateFrom); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if params.DateTo != "" {
		if t, err := time.Parse("2006-01-02", params.DateTo); err == nil {
			query = query.Where("created_at <= ?", t.Add(24*time.Hour))
		}
	}
	query = query.Group("promotion_id").Order("usage_count desc")

	if params.Limit > 0 {
		query = query.Limit(params.Limit)
	}

	if err := query.Find(&results).Error; err != nil {
		return nil, err
	}

	for i := range results {
		var promo model.Promotion
		if err := 		s.db.Select("name").Where("id = ?", results[i].PromotionID).First(&promo).Error; err == nil {
			results[i].PromotionName = promo.Name
		}
	}

	return results, nil
}

func (s *ReportService) ListTransactions(params *TransactionListParams) ([]TransactionLogRow, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 || params.PageSize > 100 {
		params.PageSize = 20
	}

	query := s.db.Table("member_transactions").
		Select(`member_transactions.id, member_transactions.member_id, 
			COALESCE(members.full_name, '') as member_name,
			COALESCE(members.username, '') as member_username,
			member_transactions.transaction_type, member_transactions.amount,
			member_transactions.balance_before, member_transactions.balance_after,
			member_transactions.bonus_before, member_transactions.bonus_after,
			member_transactions.payment_method, member_transactions.description,
			member_transactions.created_by, COALESCE(users.full_name, '') as created_by_name,
			TO_CHAR(member_transactions.created_at, 'YYYY-MM-DD HH24:MI:SS') as created_at`).
		Joins("LEFT JOIN members ON members.id = member_transactions.member_id").
		Joins("LEFT JOIN users ON users.id = member_transactions.created_by::uuid")

	if params.DateFrom != "" {
		if t, err := time.Parse("2006-01-02", params.DateFrom); err == nil {
			query = query.Where("member_transactions.created_at >= ?", t)
		}
	}
	if params.DateTo != "" {
		if t, err := time.Parse("2006-01-02", params.DateTo); err == nil {
			query = query.Where("member_transactions.created_at <= ?", t.Add(24*time.Hour))
		}
	}
	if params.TransactionType != "" {
		query = query.Where("member_transactions.transaction_type = ?", params.TransactionType)
	}
	if params.Search != "" {
		search := "%" + params.Search + "%"
		query = query.Where("members.full_name ILIKE ? OR members.phone ILIKE ? OR members.username ILIKE ?", search, search, search)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var results []TransactionLogRow
	offset := (params.Page - 1) * params.PageSize
	if err := query.Offset(offset).Limit(params.PageSize).Order("member_transactions.created_at DESC").Find(&results).Error; err != nil {
		return nil, 0, err
	}

	return results, total, nil
}
