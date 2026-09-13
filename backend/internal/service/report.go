package service

import (
	"sort"
	"strings"
	"time"

	"github.com/vnet/core/internal/model"
	"gorm.io/gorm"

	"github.com/vnet/core/pkg/utils"
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
	MachineID string `json:"machine_id"`
	// Máy không có tên riêng, chỉ có mã (model.Machine). Trường cũ tên
	// machine_name khiến giao diện dựng hai cột "Máy" và "Tên máy" trong đó một
	// cột luôn trống.
	MachineCode  string  `json:"machine_code"`
	TotalSales   int64   `json:"total_sales"`
	UsageHours   float64 `json:"usage_hours"`
	SessionCount int64   `json:"session_count"`
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

// donHangThuTien lọc ra những đơn thật sự mang tiền vào quầy, dùng chung cho
// báo cáo ngày và báo cáo tháng.
//
//   - order_type = 'topup' là phiếu khách bấm nạp tiền từ máy trạm. Khi thanh
//     toán xong nó đồng thời sinh một dòng ví 'topup', nên đếm cả hai là đếm
//     một khoản tiền hai lần.
//   - payment_method = 'balance' là trả bằng số dư — tiền đã tính lúc nạp.
//   - payment_method = 'gift_card' là thẻ quà tặng do quán phát, không có tiền
//     vào lúc phát cũng như lúc dùng.
const donHangThuTien = "order_type <> 'topup' AND payment_method = 'cash'"

type txRevenueRow struct {
	Date   string
	Amount int64
	Count  int64
}

// Doanh thu ghi nhận theo mô hình TIỀN VÀO QUÁN: tính đúng một lần, tại lúc
// tiền thật sự đi từ tay khách vào quầy. Khách tiêu từ ví sau đó KHÔNG tính
// lại — số tiền ấy đã được ghi nhận từ lúc nạp.
//
// Bản cũ cộng thẳng SUM(amount) trên một danh sách loại giao dịch, mà sổ ví ghi
// khoản tiêu là số ÂM. Hệ quả: tiền giờ chơi và tiền bán gói cước — hai nguồn
// thu chính của quán — bị TRỪ khỏi doanh thu, còn tiền nạp thì bị đếm hai lần
// cùng với đơn hàng trả bằng số dư.
//
// Loại trừ có chủ đích:
//   - session_fee, order_payment, combo_purchase trả bằng số dư: khách tiêu
//     tiền đã nạp, đã tính rồi.
//   - topup_bonus, attendance_bonus, lucky_spin_balance, lucky_spin_bonus:
//     quán TẶNG số dư, không có đồng nào đi vào két.
//   - booking_deposit, deposit_refund: tiền di chuyển trong ví, không ra vào quán.
//   - adjustment: điều chỉnh kiểm kê kho, không phải tiền khách.
//   - refund_bonus: thu lại số dư tặng, cũng không có tiền thật đi ra.
//
// topup_card KHÔNG có mặt ở đây: tiền mua thẻ vào quán lúc BÁN, và khoản đó
// nay được đếm riêng bằng thuTienBanThe. Đếm cả lúc bán lẫn lúc nạp là đếm hai
// lần cùng một khoản.
func (s *ReportService) txRevenueQuery(dateFrom, dateTo string, dateExpr string) ([]txRevenueRow, error) {
	var results []txRevenueRow
	query := s.db.Table("member_transactions").
		// Gói cước trả bằng tiền mặt là tiền vào quầy, nhưng sổ ghi nó là số âm
		// (số dư không đổi, tiền mặt mới là thứ đổi) nên phải lật dấu.
		Select(dateExpr + " as date, COALESCE(SUM(CASE WHEN transaction_type = 'combo_purchase' THEN -amount ELSE amount END), 0) as amount, COUNT(*) as count").
		// Ngoặc ngoài là bắt buộc: AND bám chặt hơn OR, nên thiếu nó thì bộ lọc
		// ngày ghép vào sau chỉ áp cho vế cuối, và báo cáo một ngày sẽ cộng cả
		// tiền nạp của mọi ngày khác.
		Where("(transaction_type IN ('topup', 'refund') OR (transaction_type = 'combo_purchase' AND payment_method = 'cash'))")

	// Mọi mốc ngày của báo cáo neo theo giờ Việt Nam. Bản cũ dùng time.Parse —
	// tức nửa đêm UTC, đúng 07:00 giờ ta — nên lọc "hôm nay → hôm nay" cắt mất
	// toàn bộ ca đêm 00:00–07:00 (giờ đông khách nhất của quán net) và lại cộng
	// nhầm 7 tiếng đầu của ngày kế tiếp. Cùng khuôn với ListTransactions bên dưới.
	if dateFrom != "" {
		if t, err := time.ParseInLocation("2006-01-02", dateFrom, utils.VietnamLocation()); err == nil {
			query = query.Where("created_at >= ?", utils.StartOfDay(t))
		}
	}
	if dateTo != "" {
		if t, err := time.ParseInLocation("2006-01-02", dateTo, utils.VietnamLocation()); err == nil {
			query = query.Where("created_at <= ?", utils.EndOfDay(t))
		}
	}

	query = query.Group("date").Order("date asc")
	if err := query.Find(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

// thuTienBanThe cộng mệnh giá những thẻ nạp đã bán, theo NGÀY BÁN.
//
// Bán thẻ là lúc tiền đi từ tay khách vào quầy, nên đây mới là thời điểm ghi
// nhận. Thẻ đã bán mà khách chưa nạp vẫn tính — tiền đã vào két rồi. Thẻ phát
// làm quà (không qua màn hình Bán) không có sold_at nên không lọt vào đây.
//
// Chỉ đếm thẻ chưa bị huỷ: huỷ thẻ là hoàn lại giao dịch bán.
func (s *ReportService) thuTienBanThe(dateFrom, dateTo string, dateExpr string) ([]txRevenueRow, error) {
	var results []txRevenueRow
	query := s.db.Table("topup_cards").
		Select(strings.ReplaceAll(dateExpr, "created_at", "sold_at") + " as date, COALESCE(SUM(face_value), 0) as amount, COUNT(*) as count").
		Where("sold_at IS NOT NULL AND status <> 'cancelled' AND deleted_at IS NULL")

	if dateFrom != "" {
		if t, err := time.ParseInLocation("2006-01-02", dateFrom, utils.VietnamLocation()); err == nil {
			query = query.Where("sold_at >= ?", utils.StartOfDay(t))
		}
	}
	if dateTo != "" {
		if t, err := time.ParseInLocation("2006-01-02", dateTo, utils.VietnamLocation()); err == nil {
			query = query.Where("sold_at <= ?", utils.EndOfDay(t))
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
		Where("status = ?", "completed").
		Where(donHangThuTien)

	if dateFrom != "" {
		if t, err := time.ParseInLocation("2006-01-02", dateFrom, utils.VietnamLocation()); err == nil {
			orderQuery = orderQuery.Where("created_at >= ?", utils.StartOfDay(t))
		}
	}
	if dateTo != "" {
		if t, err := time.ParseInLocation("2006-01-02", dateTo, utils.VietnamLocation()); err == nil {
			orderQuery = orderQuery.Where("created_at <= ?", utils.EndOfDay(t))
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

	theResults, err := s.thuTienBanThe(dateFrom, dateTo, "DATE(created_at)")
	if err != nil {
		return nil, err
	}
	txResults = append(txResults, theResults...)

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
		Where("status = ?", "completed").
		Where(donHangThuTien)

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

	theResults, err := s.thuTienBanThe(dateFrom, dateTo, "TO_CHAR(created_at, 'YYYY-MM')")
	if err != nil {
		return nil, err
	}
	txResults = append(txResults, theResults...)

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
		if t, err := time.ParseInLocation("2006-01-02", params.DateFrom, utils.VietnamLocation()); err == nil {
			orderQuery = orderQuery.Where("created_at >= ?", utils.StartOfDay(t))
		}
	}
	if params.DateTo != "" {
		if t, err := time.ParseInLocation("2006-01-02", params.DateTo, utils.VietnamLocation()); err == nil {
			orderQuery = orderQuery.Where("created_at <= ?", utils.EndOfDay(t))
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
		if t, err := time.ParseInLocation("2006-01-02", params.DateFrom, utils.VietnamLocation()); err == nil {
			txQuery = txQuery.Where("created_at >= ?", utils.StartOfDay(t))
		}
	}
	if params.DateTo != "" {
		if t, err := time.ParseInLocation("2006-01-02", params.DateTo, utils.VietnamLocation()); err == nil {
			txQuery = txQuery.Where("created_at <= ?", utils.EndOfDay(t))
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
		// COUNT(*) đếm số phiên chứ không phải số giờ; giờ thực nằm ở duration_minutes.
		Select("machine_id, ROUND(COALESCE(SUM(duration_minutes), 0) / 60.0, 2) as usage_hours, " +
			"COALESCE(SUM(total_cost), 0) as total_sales, COUNT(*) as session_count").
		Where("ended_at IS NOT NULL")

	if params.DateFrom != "" {
		if t, err := time.ParseInLocation("2006-01-02", params.DateFrom, utils.VietnamLocation()); err == nil {
			query = query.Where("started_at >= ?", utils.StartOfDay(t))
		}
	}
	if params.DateTo != "" {
		if t, err := time.ParseInLocation("2006-01-02", params.DateTo, utils.VietnamLocation()); err == nil {
			query = query.Where("started_at <= ?", utils.EndOfDay(t))
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
		// Unscoped: máy đã gỡ khỏi danh sách vẫn phải hiện mã trong báo cáo cũ —
		// nếu không, doanh thu của nó thành một dòng không tên.
		if err := s.db.Unscoped().Select("machine_code").
			Where("id = ?", results[i].MachineID).First(&machine).Error; err == nil {
			results[i].MachineCode = machine.MachineCode
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
		if t, err := time.ParseInLocation("2006-01-02", params.DateFrom, utils.VietnamLocation()); err == nil {
			query = query.Where("created_at >= ?", utils.StartOfDay(t))
		}
	}
	if params.DateTo != "" {
		if t, err := time.ParseInLocation("2006-01-02", params.DateTo, utils.VietnamLocation()); err == nil {
			query = query.Where("created_at <= ?", utils.EndOfDay(t))
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

	// Chỉ đếm đơn ĐÃ HOÀN TẤT, giống mọi báo cáo doanh thu khác trong file này.
	// Bản cũ gộp cả đơn pending/confirmed/cancelled: một đơn nháp 999 gói mì và
	// mấy đơn đã huỷ đủ sức đẩy "món bán chạy" lên 15 triệu trong khi doanh thu
	// thật của ngày hôm đó là 216 nghìn — số dùng để quyết định nhập hàng.
	query := s.db.Model(&model.OrderItem{}).
		Select("order_items.product_id, order_items.product_name, SUM(order_items.quantity) as quantity, COALESCE(SUM(order_items.subtotal), 0) as total_sales").
		Joins("JOIN orders ON orders.id = order_items.order_id").
		Where("orders.status = ?", "completed")

	if params.DateFrom != "" {
		if t, err := time.ParseInLocation("2006-01-02", params.DateFrom, utils.VietnamLocation()); err == nil {
			query = query.Where("order_items.created_at >= ?", utils.StartOfDay(t))
		}
	}
	if params.DateTo != "" {
		if t, err := time.ParseInLocation("2006-01-02", params.DateTo, utils.VietnamLocation()); err == nil {
			query = query.Where("order_items.created_at <= ?", utils.EndOfDay(t))
		}
	}
	query = query.Group("order_items.product_id, order_items.product_name").Order("total_sales desc")

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
		// promotion_id là uuid: phải ép về text trước khi COALESCE với chuỗi rỗng.
		Select("COALESCE(promotion_id::text, '') as promotion_id, COUNT(*) as usage_count, COALESCE(SUM(discount_amount), 0) as discount_given").
		Where("discount_amount > 0 AND status = ?", "completed")

	if params.DateFrom != "" {
		if t, err := time.ParseInLocation("2006-01-02", params.DateFrom, utils.VietnamLocation()); err == nil {
			query = query.Where("created_at >= ?", utils.StartOfDay(t))
		}
	}
	if params.DateTo != "" {
		if t, err := time.ParseInLocation("2006-01-02", params.DateTo, utils.VietnamLocation()); err == nil {
			query = query.Where("created_at <= ?", utils.EndOfDay(t))
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
		if err := s.db.Select("name").Where("id = ?", results[i].PromotionID).First(&promo).Error; err == nil {
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

	// time.Parse cho ra mốc UTC, còn created_at lưu theo giờ Việt Nam (+7): lọc
	// "từ 13/09 đến 13/09" thành ">= 13/09 00:00 UTC" tức 07:00 giờ VN, nên mọi
	// giao dịch rạng sáng — ca đêm của quán net, đúng khung đông khách nhất —
	// biến mất khỏi chính ngày của nó, còn tối hôm trước lại lọt vào.
	// StartOfDay/EndOfDay đã neo sẵn vào Asia/Ho_Chi_Minh.
	if params.DateFrom != "" {
		if t, err := time.ParseInLocation("2006-01-02", params.DateFrom, utils.VietnamLocation()); err == nil {
			query = query.Where("member_transactions.created_at >= ?", utils.StartOfDay(t))
		}
	}
	if params.DateTo != "" {
		if t, err := time.ParseInLocation("2006-01-02", params.DateTo, utils.VietnamLocation()); err == nil {
			query = query.Where("member_transactions.created_at <= ?", utils.EndOfDay(t))
		}
	}
	if params.TransactionType != "" {
		query = query.Where("member_transactions.transaction_type = ?", params.TransactionType)
	}
	if params.Search != "" {
		search := "%" + params.Search + "%"
		// Bỏ dấu khi so khớp: nhân viên gõ "Tran Thi" phải ra "Trần Thị Bích",
		// giống ô tìm ở trang Hội viên.
		query = query.Where(
			"unaccent(members.full_name) ILIKE unaccent(?) OR members.phone ILIKE ? OR unaccent(members.username) ILIKE unaccent(?)",
			search, search, search,
		)
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
