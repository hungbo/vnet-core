package service

import (
	"fmt"
	"sort"
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
	TotalTopup int64  `json:"total_topup"`
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

// phuongThucNoiBo là các cách trừ tiền NỘI BỘ — số dư, tiền thưởng, thẻ của
// chức năng đã gỡ — khớp reservedPaymentCodes. Mọi mã khác là tiền khách đưa
// vào quán: tiền mặt, chuyển khoản, ví điện tử hay phương thức quán tự thêm ở
// Cài đặt.
//
// Bản cũ chỉ đếm payment_method = 'cash', nên đơn hàng và gói cước trả bằng
// chuyển khoản hay ví điện tử biến mất khỏi doanh thu dù tiền đã vào tài khoản.
const phuongThucNoiBo = "('balance', 'bonus_balance', 'gift_card', 'topup_card')"

// ngayDonHang là lúc đơn mang tiền vào: lúc thanh toán, không phải lúc khách
// bấm đặt — đơn đặt 23:50 trả 00:10 là tiền của ngày hôm sau, giống cách trang
// Ca làm việc đối soát két theo giờ trả. Đơn thiếu completed_at thì lấy
// created_at.
const ngayDonHang = "COALESCE(orders.completed_at, orders.created_at)"

// locKhoangNgay lọc cột thời gian col theo date_from/date_to (YYYY-MM-DD).
//
// Mọi mốc ngày neo theo giờ Việt Nam. time.Parse cho ra nửa đêm UTC — đúng
// 07:00 giờ ta — nên lọc "hôm nay → hôm nay" sẽ cắt mất ca đêm 00:00–07:00
// (giờ đông khách nhất của quán net) và cộng nhầm 7 tiếng đầu của ngày kế tiếp.
//
// Mốc cuối là "trước nửa đêm hôm sau", không phải "<= 23:59:59": thời gian lưu
// tới micro giây, nên tiền thu lúc 23:59:59.6 trước đây lọt khỏi cả hai ngày.
func locKhoangNgay(q *gorm.DB, col, dateFrom, dateTo string) *gorm.DB {
	loc := utils.VietnamLocation()
	if t, err := time.ParseInLocation("2006-01-02", dateFrom, loc); err == nil {
		q = q.Where(col+" >= ?", utils.StartOfDay(t))
	}
	if t, err := time.ParseInLocation("2006-01-02", dateTo, loc); err == nil {
		q = q.Where(col+" < ?", utils.StartOfDay(t).AddDate(0, 0, 1))
	}
	return q
}

// MonthRange đổi cách gọi cũ ?year=&month= của báo cáo tháng thành khoảng
// ngày. Thiếu tháng (hoặc tháng sai) là cả năm; thiếu năm là không lọc.
//
// Bản cũ dựng time.Date(year, 0, 1) khi thiếu tháng — tháng 0 là tháng 12 năm
// TRƯỚC — nên báo cáo một năm cộng thêm tiền nạp của tháng 12 năm trước.
func MonthRange(year, month int) (dateFrom, dateTo string) {
	if year <= 0 {
		return "", ""
	}
	if month < 1 || month > 12 {
		return fmt.Sprintf("%04d-01-01", year), fmt.Sprintf("%04d-12-31", year)
	}
	dau := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	return dau.Format("2006-01-02"), dau.AddDate(0, 1, -1).Format("2006-01-02")
}

type kyDoanhThu struct {
	Ky          string
	TotalOrders int64
	Revenue     int64
	Discount    int64
}

// doanhThuTheoKy là phần chung của báo cáo ngày và báo cáo tháng; dinhDang là
// khuôn TO_CHAR của kỳ ('YYYY-MM-DD' hoặc 'YYYY-MM').
//
// Doanh thu ghi nhận theo mô hình TIỀN VÀO QUÁN: tính đúng một lần, tại lúc
// tiền thật sự đi từ tay khách vào quầy. Khách tiêu từ ví sau đó KHÔNG tính
// lại — số tiền ấy đã được ghi nhận từ lúc nạp.
//
// Tính vào doanh thu:
//   - đơn bán hàng đã hoàn tất, trả bằng tiền khách đưa (không phải số dư);
//   - topup: tiền nạp ví, mọi phương thức. Đơn NẠP TIỀN (order_type = 'topup')
//     khi thanh toán cũng sinh đúng một dòng topup, nên đơn ấy không đếm thêm;
//   - combo_purchase trả bằng tiền khách đưa. Sổ ghi nó là số âm (số dư không
//     đổi, tiền mặt mới là thứ đổi) nên phải lật dấu;
//   - refund: trả lại tiền cho khách, số âm.
//
// Loại trừ có chủ đích:
//   - session_fee, order_payment, combo_purchase trả bằng số dư: khách tiêu
//     tiền đã nạp, đã tính rồi.
//   - topup_bonus, attendance_bonus, lucky_spin_balance, lucky_spin_bonus:
//     quán TẶNG số dư, không có đồng nào đi vào két.
//   - booking_deposit, deposit_refund: tiền di chuyển trong ví, không ra vào quán.
//     Cọc MẤT do khách không đến (lịch no_show) cũng không cộng thêm: đồng
//     cọc ấy đã nằm trong doanh thu từ lần nạp ví sinh ra nó — tính đúng một
//     lần ở đó; cộng lúc mất cọc nữa là đếm hai lần.
//   - adjustment: điều chỉnh kiểm kê kho, không phải tiền khách.
//   - refund_bonus: thu lại số dư tặng, cũng không có tiền thật đi ra.
//
// "Số đơn hàng" chỉ đếm đơn bán hàng đã hoàn tất, kể cả đơn trả bằng số dư.
// Bản cũ cộng thêm số dòng nạp/hoàn/gói cước vào đây, nên con số không khớp
// với trang Đơn hàng.
func (s *ReportService) doanhThuTheoKy(dateFrom, dateTo, dinhDang string) ([]kyDoanhThu, error) {
	var donHang []kyDoanhThu
	orderQuery := s.db.Model(&model.Order{}).
		Select("TO_CHAR("+ngayDonHang+", '"+dinhDang+"') as ky, COUNT(*) as total_orders, "+
			"COALESCE(SUM(final_amount) FILTER (WHERE payment_method NOT IN "+phuongThucNoiBo+"), 0) as revenue, "+
			"COALESCE(SUM(discount_amount), 0) as discount").
		Where("status = ? AND order_type <> ?", "completed", model.OrderTypeTopup)
	orderQuery = locKhoangNgay(orderQuery, ngayDonHang, dateFrom, dateTo)
	if err := orderQuery.Group("ky").Find(&donHang).Error; err != nil {
		return nil, err
	}

	var soVi []kyDoanhThu
	txQuery := s.db.Table("member_transactions").
		Select("TO_CHAR(created_at, '" + dinhDang + "') as ky, " +
			"COALESCE(SUM(CASE WHEN transaction_type = 'combo_purchase' THEN -amount ELSE amount END), 0) as revenue").
		// Ngoặc ngoài là bắt buộc: AND bám chặt hơn OR, nên thiếu nó thì bộ lọc
		// ngày ghép vào sau chỉ áp cho vế cuối, và báo cáo một ngày sẽ cộng cả
		// tiền nạp của mọi ngày khác.
		Where("(transaction_type IN ('topup', 'refund') OR (transaction_type = 'combo_purchase' AND payment_method NOT IN " + phuongThucNoiBo + "))")
	txQuery = locKhoangNgay(txQuery, "created_at", dateFrom, dateTo)
	if err := txQuery.Group("ky").Find(&soVi).Error; err != nil {
		return nil, err
	}

	theoKy := make(map[string]*kyDoanhThu)
	for i := range donHang {
		theoKy[donHang[i].Ky] = &donHang[i]
	}
	for _, tx := range soVi {
		if existing, ok := theoKy[tx.Ky]; ok {
			existing.Revenue += tx.Revenue
		} else {
			theoKy[tx.Ky] = &kyDoanhThu{Ky: tx.Ky, Revenue: tx.Revenue}
		}
	}

	results := make([]kyDoanhThu, 0, len(theoKy))
	for _, r := range theoKy {
		results = append(results, *r)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Ky < results[j].Ky })
	return results, nil
}

func (s *ReportService) DailyRevenue(dateFrom, dateTo string) ([]DailyRevenueRow, error) {
	rows, err := s.doanhThuTheoKy(dateFrom, dateTo, "YYYY-MM-DD")
	if err != nil {
		return nil, err
	}
	results := make([]DailyRevenueRow, len(rows))
	for i, r := range rows {
		results[i] = DailyRevenueRow{Date: r.Ky, TotalOrders: r.TotalOrders, Revenue: r.Revenue, Discount: r.Discount}
	}
	return results, nil
}

// MonthlyRevenue nhận khoảng ngày giống mọi báo cáo khác. Trang Báo cáo chỉ có
// một ô chọn khoảng ngày; bản cũ lấy năm/tháng từ NGÀY ĐẦU của khoảng ấy, nên
// chọn 01/09 → 17/10 chỉ ra mỗi tháng 9.
func (s *ReportService) MonthlyRevenue(dateFrom, dateTo string) ([]MonthlyRevenueRow, error) {
	rows, err := s.doanhThuTheoKy(dateFrom, dateTo, "YYYY-MM")
	if err != nil {
		return nil, err
	}
	results := make([]MonthlyRevenueRow, len(rows))
	for i, r := range rows {
		results[i] = MonthlyRevenueRow{Month: r.Ky, TotalOrders: r.TotalOrders, Revenue: r.Revenue, Discount: r.Discount}
	}
	return results, nil
}

type tienTheoHoiVien struct {
	MemberID string
	Spent    int64
	Topup    int64
	Count    int64
}

// ByMember: mỗi hội viên đã tiêu bao nhiêu, đã nạp bao nhiêu và chơi mấy lượt.
//
// "Tổng chi tiêu" dùng đúng định nghĩa của cột total_spent (thứ quyết định
// hạng hội viên): tiền giờ, đơn bán hàng và gói cước — trả bằng gì cũng tính.
// Bản cũ cộng TIỀN NẠP vào chi tiêu rồi cộng tiền giờ — vốn là số âm trong sổ
// ví — nên khách chơi càng nhiều thì "chi tiêu" càng thấp, còn gói cước không
// được tính. Tiền nạp giờ là cột riêng.
//
// "Số lần" là số lượt chơi (phiên máy) trong khoảng ngày. Bản cũ đếm gộp số
// đơn, số lần nạp và số dòng tiền giờ.
func (s *ReportService) ByMember(params ReportParams) ([]ByMemberRow, error) {
	theoHoiVien := make(map[string]*ByMemberRow)
	dong := func(id string) *ByMemberRow {
		r, ok := theoHoiVien[id]
		if !ok {
			r = &ByMemberRow{MemberID: id}
			theoHoiVien[id] = r
		}
		return r
	}

	var donHang []tienTheoHoiVien
	orderQuery := s.db.Model(&model.Order{}).
		Select("member_id, COALESCE(SUM(final_amount), 0) as spent").
		Where("member_id IS NOT NULL AND status = ? AND order_type <> ?", "completed", model.OrderTypeTopup)
	orderQuery = locKhoangNgay(orderQuery, ngayDonHang, params.DateFrom, params.DateTo)
	if err := orderQuery.Group("member_id").Find(&donHang).Error; err != nil {
		return nil, err
	}
	for _, d := range donHang {
		dong(d.MemberID).TotalSpent += d.Spent
	}

	// Đơn trả bằng số dư cũng sinh dòng order_payment trong sổ ví; đã lấy từ
	// bảng orders ở trên nên không đọc loại đó ở đây.
	var soVi []tienTheoHoiVien
	txQuery := s.db.Table("member_transactions").
		Select("member_id, " +
			"COALESCE(SUM(CASE WHEN transaction_type = 'topup' THEN 0 ELSE -amount END), 0) as spent, " +
			"COALESCE(SUM(CASE WHEN transaction_type = 'topup' THEN amount ELSE 0 END), 0) as topup").
		Where("transaction_type IN ('session_fee', 'combo_purchase', 'topup')")
	txQuery = locKhoangNgay(txQuery, "created_at", params.DateFrom, params.DateTo)
	if err := txQuery.Group("member_id").Find(&soVi).Error; err != nil {
		return nil, err
	}
	for _, tx := range soVi {
		r := dong(tx.MemberID)
		r.TotalSpent += tx.Spent
		r.TotalTopup += tx.Topup
	}

	var luotChoi []tienTheoHoiVien
	sessionQuery := s.db.Model(&model.MachineSession{}).
		Select("member_id, COUNT(*) as count").
		Where("member_id IS NOT NULL")
	sessionQuery = locKhoangNgay(sessionQuery, "started_at", params.DateFrom, params.DateTo)
	if err := sessionQuery.Group("member_id").Find(&luotChoi).Error; err != nil {
		return nil, err
	}
	for _, l := range luotChoi {
		dong(l.MemberID).VisitCount += l.Count
	}

	results := make([]ByMemberRow, 0, len(theoHoiVien))
	for _, r := range theoHoiVien {
		results = append(results, *r)
	}
	sort.Slice(results, func(i, j int) bool {
		a, b := results[i], results[j]
		if a.TotalSpent != b.TotalSpent {
			return a.TotalSpent > b.TotalSpent
		}
		if a.TotalTopup != b.TotalTopup {
			return a.TotalTopup > b.TotalTopup
		}
		return a.MemberID < b.MemberID
	})
	// Cắt SAU khi gộp: bản cũ cắt danh sách đơn hàng trước rồi mới cộng tiền
	// nạp và tiền giờ, nên khách tiêu nhiều mà ít đặt món có thể rơi khỏi top.
	if params.Limit > 0 && len(results) > params.Limit {
		results = results[:params.Limit]
	}

	if len(results) > 0 {
		ids := make([]string, len(results))
		for i := range results {
			ids[i] = results[i].MemberID
		}
		// Unscoped: hội viên đã xoá vẫn phải hiện tên ở báo cáo cũ. Khách tạo
		// tài khoản ở máy trạm thường không có họ tên — khi đó hiện tên đăng
		// nhập thay vì một ô trống.
		var members []model.Member
		if err := s.db.Unscoped().Select("id, username, full_name").
			Where("id IN ?", ids).Find(&members).Error; err != nil {
			return nil, err
		}
		ten := make(map[string]string, len(members))
		for _, m := range members {
			ten[m.ID] = m.FullName
			if ten[m.ID] == "" {
				ten[m.ID] = m.Username
			}
		}
		for i := range results {
			results[i].MemberName = ten[results[i].MemberID]
		}
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
	query = locKhoangNgay(query, "started_at", params.DateFrom, params.DateTo)
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

// ByEmployee tính mỗi đơn bán hàng cho nhân viên đã THU TIỀN — updated_by, cột
// "Người thực hiện" ở trang Đơn hàng — không phải người tạo đơn. Phần lớn đơn
// do khách tự đặt từ máy trạm nên created_by là mã HỘI VIÊN: bản cũ in mỗi hội
// viên thành một "nhân viên" không tên, còn người phục vụ thì không được tính.
// Đơn cũ chưa ghi người thu thì lấy người tạo; JOIN users loại mã hội viên ra.
//
// Đơn nạp tiền không phải doanh số bán hàng nên không tính ở đây.
func (s *ReportService) ByEmployee(params ReportParams) ([]ByEmployeeRow, error) {
	var results []ByEmployeeRow

	query := s.db.Model(&model.Order{}).
		Select("users.id as employee_id, COALESCE(NULLIF(users.full_name, ''), users.username) as employee_name, "+
			"COUNT(*) as orders_taken, COALESCE(SUM(orders.final_amount), 0) as total_sales").
		Joins("JOIN users ON users.id = COALESCE(orders.updated_by, orders.created_by)").
		Where("orders.status = ? AND orders.order_type <> ?", "completed", model.OrderTypeTopup)
	query = locKhoangNgay(query, ngayDonHang, params.DateFrom, params.DateTo)
	query = query.Group("users.id, users.full_name, users.username").Order("total_sales desc")

	if params.Limit > 0 {
		query = query.Limit(params.Limit)
	}

	if err := query.Find(&results).Error; err != nil {
		return nil, err
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
	query = locKhoangNgay(query, ngayDonHang, params.DateFrom, params.DateTo)
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
	query = locKhoangNgay(query, ngayDonHang, params.DateFrom, params.DateTo)
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

	query = locKhoangNgay(query, "member_transactions.created_at", params.DateFrom, params.DateTo)
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
