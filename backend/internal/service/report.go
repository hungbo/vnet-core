package service

import (
	"sort"
	"time"

	"github.com/vnet/core/internal/model"
	"gorm.io/gorm"

	"github.com/vnet/core/pkg/pagination"
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
	// Cột người dùng bấm sắp xếp trên trang Giao dịch (xem transactionSortColumns).
	Sort  string `form:"sort"`
	Order string `form:"order"`
}

// transactionSortColumns: cột trang Giao dịch cho bấm sắp xếp (prop của cột →
// cột SQL). Truy vấn JOIN members/users nên phải ghi rõ tên bảng.
var transactionSortColumns = map[string]string{
	"created_at":       "member_transactions.created_at",
	"member_username":  "members.username",
	"transaction_type": "member_transactions.transaction_type",
	"amount":           "member_transactions.amount",
	"balance_before":   "member_transactions.balance_before",
	"balance_after":    "member_transactions.balance_after",
	"bonus_before":     "member_transactions.bonus_before",
	"bonus_after":      "member_transactions.bonus_after",
	"payment_method":   "member_transactions.payment_method",
	"description":      "member_transactions.description",
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
			member_transactions.created_by,
			COALESCE(NULLIF(users.full_name, ''), users.username, NULLIF(nguoi_hv.full_name, ''), nguoi_hv.username, '') as created_by_name,
			TO_CHAR(member_transactions.created_at, 'YYYY-MM-DD HH24:MI:SS') as created_at`).
		Joins("LEFT JOIN members ON members.id = member_transactions.member_id").
		// Người thực hiện: nhân viên chưa khai họ tên thì lấy tên đăng nhập — bản
		// cũ trả chuỗi rỗng và trang Giao dịch in nguyên mã uuid. Thao tác do
		// chính hội viên làm ở máy trạm (mua gói, nạp qua đơn) có created_by là mã
		// HỘI VIÊN, nên tra cả bảng members.
		Joins("LEFT JOIN users ON users.id = member_transactions.created_by::uuid").
		Joins("LEFT JOIN members nguoi_hv ON nguoi_hv.id = member_transactions.created_by::uuid")

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
	sorting := &pagination.Params{Sort: params.Sort, Order: params.Order, Sorted: params.Sort != ""}
	order := pagination.OrderBy(sorting, transactionSortColumns, "member_transactions.created_at DESC")
	if err := query.Offset(offset).Limit(params.PageSize).Order(order).Find(&results).Error; err != nil {
		return nil, 0, err
	}

	return results, total, nil
}
