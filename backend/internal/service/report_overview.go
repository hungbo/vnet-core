package service

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/vnet/core/pkg/pagination"
)

// ReportFilter là bộ lọc chung của trang Báo cáo. Mọi ô lọc áp cùng lúc cho
// thẻ tổng, bảng chi tiết và mọi cách "Xem theo", nên con số ở thẻ luôn khớp
// với tổng của bảng bên dưới.
type ReportFilter struct {
	DateFrom string `form:"date_from"`
	DateTo   string `form:"date_to"`
	// Kind: sale | topup | combo | refund | session; rỗng là mọi loại.
	Kind string `form:"kind"`
	// EmployeeID: mã nhân viên, hoặc "none" — dòng không do nhân viên nào làm
	// (khách tự trả bằng số dư ở máy trạm, tiền giờ).
	EmployeeID    string `form:"employee_id"`
	MemberID      string `form:"member_id"`
	MachineID     string `form:"machine_id"`
	PaymentMethod string `form:"payment_method"`
	ProductID     string `form:"product_id"`
	PromotionID   string `form:"promotion_id"`
	Search        string `form:"search"`
	// GroupBy: rỗng là từng dòng; còn lại là một khoá của nhomBaoCao hoặc "product".
	GroupBy string `form:"group_by"`
}

var loaiDongTien = map[string]bool{"sale": true, "topup": true, "combo": true, "refund": true, "session": true}

// Validate trả lỗi để hiện cho người dùng (400), trước khi giá trị nào chạm SQL.
func (f ReportFilter) Validate() error {
	if f.Kind != "" && !loaiDongTien[f.Kind] {
		return errors.New("kind chỉ nhận sale, topup, combo, refund hoặc session")
	}
	if _, ok := nhomBaoCao[f.GroupBy]; !ok && f.GroupBy != "" && f.GroupBy != "product" {
		return errors.New("group_by không hợp lệ")
	}
	for _, id := range []string{f.MemberID, f.MachineID, f.ProductID, f.PromotionID} {
		if id != "" && !laUUID(id) {
			return errors.New("mã lọc không hợp lệ")
		}
	}
	if f.EmployeeID != "" && f.EmployeeID != "none" && !laUUID(f.EmployeeID) {
		return errors.New("mã nhân viên không hợp lệ")
	}
	return nil
}

// ReportTotals là các thẻ tổng ở đầu trang, và cũng là các cột của mỗi nhóm
// khi "Xem theo". Luôn đúng: MoneyIn = Sales + Topups + Combos + Refunds −
// PaidByBalance.
type ReportTotals struct {
	// MoneyIn: tiền thật vào quán, cùng định nghĩa với doanhThuTheoKy (Doanh thu
	// ở Bảng điều khiển) — bán món và gói cước trả bằng tiền khách đưa, mọi
	// lần nạp, trừ tiền hoàn.
	MoneyIn    int64 `json:"money_in"`
	Sales      int64 `json:"sales"`
	SaleCount  int64 `json:"sale_count"`
	Topups     int64 `json:"topups"`
	TopupCount int64 `json:"topup_count"`
	Combos     int64 `json:"combos"`
	ComboCount int64 `json:"combo_count"`
	// Refunds là số âm.
	Refunds int64 `json:"refunds"`
	// PaidByBalance: phần Bán món và Gói cước trả bằng số dư — tiền ấy đã tính
	// lúc khách nạp, nên không vào MoneyIn lần nữa.
	PaidByBalance int64 `json:"paid_by_balance"`
	SessionFees   int64 `json:"session_fees"`
	SessionCount  int64 `json:"session_count"`
	Minutes       int64 `json:"minutes"`
	Discount      int64 `json:"discount"`
	LineCount     int64 `json:"line_count"`
}

// ReportGroupRow là một dòng khi "Xem theo". Key rỗng là nhóm "không có":
// khách vãng lai, dòng không gắn máy, không có nhân viên…
type ReportGroupRow struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	ReportTotals
	// Quantity chỉ có khi xem theo món.
	Quantity int64 `json:"quantity"`
}

// ReportLine là một dòng tiền ở bảng chi tiết.
type ReportLine struct {
	ID            string    `json:"id"`
	Kind          string    `json:"kind"`
	At            time.Time `json:"at"`
	Code          string    `json:"code"`
	MemberID      *string   `json:"member_id"`
	MemberName    string    `json:"member_name"`
	MachineID     *string   `json:"machine_id"`
	MachineCode   string    `json:"machine_code"`
	EmployeeID    *string   `json:"employee_id"`
	EmployeeName  string    `json:"employee_name"`
	PaymentMethod string    `json:"payment_method"`
	Amount        int64     `json:"amount"`
	MoneyIn       int64     `json:"money_in"`
	Discount      int64     `json:"discount"`
	Minutes       int64     `json:"minutes"`
	// Note: món trong đơn ("2 Sting, 1 Mì"), hoặc diễn giải của dòng sổ ví.
	Note string `json:"note"`
}

type ReportOverview struct {
	Totals ReportTotals `json:"totals"`
	// Rows: []ReportLine khi xem từng dòng, []ReportGroupRow khi "Xem theo".
	Rows     interface{} `json:"rows"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

// dongTienSQL gom mọi lần tiền đổi chủ về một khuôn chung, mỗi dòng một lần:
//
//   - sale: đơn bán hàng đã hoàn tất, tính lúc thanh toán. Nhân viên là người
//     THU TIỀN (updated_by — cột "Người thực hiện" ở trang Đơn hàng); đơn cũ
//     chưa ghi người thu thì lấy người tạo. JOIN users loại mã hội viên ra:
//     đơn khách tự đặt và tự trả ở máy trạm không có nhân viên.
//   - topup, refund, combo: từ sổ ví. Đơn NẠP TIỀN khi duyệt sinh đúng một
//     dòng topup do nhân viên duyệt ghi, nên không lấy đơn nạp từ bảng orders.
//   - session: tiền giờ của phiên đã kết thúc, tính theo giờ mở máy (giống
//     báo cáo theo máy cũ). Trừ vào số dư nên không vào money_in.
//
// money_in theo đúng quy tắc của doanhThuTheoKy. Bản cũ của "Theo nhân viên"
// chỉ cộng đơn bán hàng — kể cả đơn trả bằng số dư — và bỏ hẳn tiền nạp, thứ
// nhân viên quán net thu nhiều nhất, nên ca nạp 2,9 triệu chỉ hiện 3,1 triệu
// tiền món.
const dongTienSQL = `SELECT o.id AS ref_id, 'sale' AS kind, COALESCE(o.completed_at, o.created_at) AS at, o.order_code AS code,
	o.member_id, o.machine_id, u.id AS employee_id, COALESCE(NULLIF(u.full_name, ''), u.username) AS employee_name,
	o.payment_method, o.final_amount AS amount,
	CASE WHEN o.payment_method IN ` + phuongThucNoiBo + ` THEN 0 ELSE o.final_amount END AS money_in,
	o.discount_amount AS discount, o.promotion_id, 0 AS minutes, o.note
FROM orders o
LEFT JOIN users u ON u.id = COALESCE(o.updated_by, o.created_by)
WHERE o.deleted_at IS NULL AND o.status = 'completed' AND o.order_type <> 'topup'
UNION ALL
SELECT t.id, CASE WHEN t.transaction_type = 'combo_purchase' THEN 'combo' ELSE t.transaction_type END,
	t.created_at, '', t.member_id, NULL, u.id, COALESCE(NULLIF(u.full_name, ''), u.username),
	t.payment_method,
	CASE WHEN t.transaction_type = 'combo_purchase' THEN -t.amount ELSE t.amount END,
	CASE WHEN t.transaction_type <> 'combo_purchase' THEN t.amount
		WHEN t.payment_method IN ` + phuongThucNoiBo + ` THEN 0 ELSE -t.amount END,
	0, NULL, 0, t.description
FROM member_transactions t
LEFT JOIN users u ON u.id = t.created_by::uuid
WHERE t.transaction_type IN ('topup', 'refund', 'combo_purchase')
UNION ALL
SELECT s.id, 'session', s.started_at, '', s.member_id, s.machine_id, NULL, NULL,
	'', COALESCE(s.total_cost, 0), 0, 0, NULL, COALESCE(s.duration_minutes, 0), ''
FROM machine_sessions s
WHERE s.ended_at IS NOT NULL`

const tongBaoCaoSQL = `COALESCE(SUM(d.money_in), 0) AS money_in,
COALESCE(SUM(d.amount) FILTER (WHERE d.kind = 'sale'), 0) AS sales,
COUNT(*) FILTER (WHERE d.kind = 'sale') AS sale_count,
COALESCE(SUM(d.amount) FILTER (WHERE d.kind = 'topup'), 0) AS topups,
COUNT(*) FILTER (WHERE d.kind = 'topup') AS topup_count,
COALESCE(SUM(d.amount) FILTER (WHERE d.kind = 'combo'), 0) AS combos,
COUNT(*) FILTER (WHERE d.kind = 'combo') AS combo_count,
COALESCE(SUM(d.amount) FILTER (WHERE d.kind = 'refund'), 0) AS refunds,
COALESCE(SUM(d.amount) FILTER (WHERE d.kind IN ('sale', 'combo') AND d.payment_method IN ` + phuongThucNoiBo + `), 0) AS paid_by_balance,
COALESCE(SUM(d.amount) FILTER (WHERE d.kind = 'session'), 0) AS session_fees,
COUNT(*) FILTER (WHERE d.kind = 'session') AS session_count,
COALESCE(SUM(d.minutes), 0) AS minutes,
COALESCE(SUM(d.discount), 0) AS discount,
COUNT(*) AS line_count`

// nhomBaoCao: mỗi cách "Xem theo" là một khoá nhóm và một nhãn (gộp bằng MAX
// vì nhóm theo khoá). Ngày/tháng dùng TO_CHAR theo múi giờ phiên kết nối — giờ
// Việt Nam (DSN), giống doanhThuTheoKy.
var nhomBaoCao = map[string]struct{ khoa, nhan string }{
	"day":            {"TO_CHAR(d.at, 'YYYY-MM-DD')", "''"},
	"month":          {"TO_CHAR(d.at, 'YYYY-MM')", "''"},
	"employee":       {"COALESCE(d.employee_id::text, '')", "COALESCE(MAX(d.employee_name), '')"},
	"member":         {"COALESCE(d.member_id::text, '')", "COALESCE(MAX(COALESCE(NULLIF(m.username, ''), m.full_name)), '')"},
	"machine":        {"COALESCE(d.machine_id::text, '')", "COALESCE(MAX(mc.machine_code), '')"},
	"payment_method": {"d.payment_method", "''"},
	"kind":           {"d.kind", "''"},
	"promotion":      {"COALESCE(d.promotion_id::text, '')", "COALESCE(MAX(p.name), '')"},
}

// reportLineSortColumns: cột bảng chi tiết cho bấm sắp xếp.
var reportLineSortColumns = map[string]string{
	"at":             "d.at",
	"kind":           "d.kind",
	"code":           "d.code",
	"member_name":    "member_name",
	"machine_code":   "machine_code",
	"employee_name":  "employee_name",
	"payment_method": "d.payment_method",
	"amount":         "d.amount",
	"money_in":       "d.money_in",
	"discount":       "d.discount",
	"minutes":        "d.minutes",
}

// dongTien là tập dòng tiền đã áp mọi ô lọc. Mỗi truy vấn gọi lại để có một
// câu lệnh GORM mới.
func (s *ReportService) dongTien(f ReportFilter) *gorm.DB {
	q := s.db.Table("(" + dongTienSQL + ") AS d").
		// Không lọc deleted_at: hội viên hay máy đã xoá vẫn phải hiện tên ở
		// báo cáo cũ.
		Joins("LEFT JOIN members m ON m.id = d.member_id").
		Joins("LEFT JOIN machines mc ON mc.id = d.machine_id")
	q = locKhoangNgay(q, "d.at", f.DateFrom, f.DateTo)
	if f.Kind != "" {
		q = q.Where("d.kind = ?", f.Kind)
	}
	if f.EmployeeID == "none" {
		q = q.Where("d.employee_id IS NULL")
	} else if f.EmployeeID != "" {
		q = q.Where("d.employee_id = ?", f.EmployeeID)
	}
	if f.MemberID != "" {
		q = q.Where("d.member_id = ?", f.MemberID)
	}
	if f.MachineID != "" {
		q = q.Where("d.machine_id = ?", f.MachineID)
	}
	if f.PaymentMethod != "" {
		q = q.Where("d.payment_method = ?", f.PaymentMethod)
	}
	if f.ProductID != "" {
		q = q.Where("d.kind = 'sale' AND EXISTS (SELECT 1 FROM order_items oi WHERE oi.order_id = d.ref_id AND oi.product_id = ?)", f.ProductID)
	}
	if f.PromotionID != "" {
		q = q.Where("d.promotion_id = ?", f.PromotionID)
	}
	if tu := strings.TrimSpace(f.Search); tu != "" {
		mau := "%" + tu + "%"
		// Bỏ dấu khi so khớp, giống ô tìm ở trang Hội viên và Giao dịch. Tìm
		// được cả tên món trong đơn.
		q = q.Where(`(d.code ILIKE ? OR unaccent(COALESCE(d.note, '')) ILIKE unaccent(?)
			OR unaccent(COALESCE(d.employee_name, '')) ILIKE unaccent(?) OR mc.machine_code ILIKE ?
			OR unaccent(m.username) ILIKE unaccent(?) OR unaccent(m.full_name) ILIKE unaccent(?) OR m.phone ILIKE ?
			OR EXISTS (SELECT 1 FROM order_items oi WHERE oi.order_id = d.ref_id AND unaccent(oi.product_name) ILIKE unaccent(?)))`,
			mau, mau, mau, mau, mau, mau, mau, mau)
	}
	return q
}

// Overview trả thẻ tổng cùng bảng: từng dòng (phân trang, sắp xếp) khi
// GroupBy rỗng, còn lại là các nhóm (không phân trang).
func (s *ReportService) Overview(f ReportFilter, params pagination.Params) (*ReportOverview, error) {
	kq := &ReportOverview{}
	if err := s.dongTien(f).Select(tongBaoCaoSQL).Scan(&kq.Totals).Error; err != nil {
		return nil, err
	}

	switch f.GroupBy {
	case "":
		lines := []ReportLine{}
		order := pagination.OrderBy(&params, reportLineSortColumns, "d.at DESC")
		if err := s.dongTien(f).
			Select(`d.ref_id AS id, d.kind, d.at, d.code,
				d.member_id, COALESCE(NULLIF(m.username, ''), m.full_name, '') AS member_name,
				d.machine_id, COALESCE(mc.machine_code, '') AS machine_code,
				d.employee_id, COALESCE(d.employee_name, '') AS employee_name,
				d.payment_method, d.amount, d.money_in, d.discount, d.minutes,
				CASE WHEN d.kind = 'sale' THEN COALESCE((SELECT string_agg(oi.quantity || ' ' || oi.product_name, ', ' ORDER BY oi.created_at)
					FROM order_items oi WHERE oi.order_id = d.ref_id), '') ELSE COALESCE(d.note, '') END AS note`).
			Order(order).Offset((params.Page - 1) * params.PageSize).Limit(params.PageSize).
			Scan(&lines).Error; err != nil {
			return nil, err
		}
		kq.Rows, kq.Total, kq.Page, kq.PageSize = lines, kq.Totals.LineCount, params.Page, params.PageSize

	case "product":
		// Theo món chỉ có nghĩa với đơn bán hàng. Lọc theo một món thì chỉ đếm
		// đúng món ấy, không đếm các món khác nằm chung đơn.
		rows := []ReportGroupRow{}
		q := s.dongTien(f).Where("d.kind = 'sale'").Joins("JOIN order_items oi ON oi.order_id = d.ref_id")
		if f.ProductID != "" {
			q = q.Where("oi.product_id = ?", f.ProductID)
		}
		if err := q.Select(`oi.product_id::text AS key, MAX(oi.product_name) AS label,
				SUM(oi.quantity) AS quantity, SUM(oi.subtotal) AS sales, COUNT(DISTINCT d.ref_id) AS sale_count`).
			Group("oi.product_id").Order("sales DESC, label").Scan(&rows).Error; err != nil {
			return nil, err
		}
		kq.Rows, kq.Total = rows, int64(len(rows))

	default:
		nhom := nhomBaoCao[f.GroupBy]
		rows := []ReportGroupRow{}
		q := s.dongTien(f)
		if f.GroupBy == "promotion" {
			q = q.Joins("LEFT JOIN promotions p ON p.id = d.promotion_id").Where("d.discount > 0")
		}
		thuTu := "money_in DESC, sales DESC, label"
		if f.GroupBy == "day" || f.GroupBy == "month" {
			thuTu = "key"
		}
		if err := q.Select(nhom.khoa + " AS key, " + nhom.nhan + " AS label, " + tongBaoCaoSQL).
			Group(nhom.khoa).Order(thuTu).Scan(&rows).Error; err != nil {
			return nil, err
		}
		kq.Rows, kq.Total = rows, int64(len(rows))
	}
	return kq, nil
}

// ReportEmployee là một lựa chọn ở ô "Nhân viên".
type ReportEmployee struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Employees: danh sách cho ô lọc nhân viên. Người có quyền xem báo cáo chưa
// chắc có quyền xem trang Tài khoản, nên báo cáo tự trả danh sách này.
func (s *ReportService) Employees() ([]ReportEmployee, error) {
	list := []ReportEmployee{}
	err := s.db.Table("users").
		Select("id, COALESCE(NULLIF(full_name, ''), username) AS name").
		Where("deleted_at IS NULL").Order("name").Scan(&list).Error
	return list, err
}
