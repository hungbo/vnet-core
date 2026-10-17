package service

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/pkg/utils"
)

// Doanh thu là tiền khách đưa vào quán bằng MỌI phương thức trừ cách trừ nội
// bộ (số dư, tiền thưởng...). Bản cũ chỉ đếm 'cash', nên đơn và gói cước trả
// bằng chuyển khoản hay ví điện tử biến mất khỏi báo cáo.
func TestReportService_DailyRevenue(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewReportService(db)

	mock.ExpectQuery(`SELECT TO_CHAR\(COALESCE\(orders\.completed_at, orders\.created_at\), 'YYYY-MM-DD'\) as ky, COUNT\(\*\) as total_orders, COALESCE\(SUM\(final_amount\) FILTER \(WHERE payment_method NOT IN \('balance', 'bonus_balance', 'gift_card', 'topup_card'\)\), 0\) as revenue, COALESCE\(SUM\(discount_amount\), 0\) as discount FROM "orders" WHERE \(status = \$1 AND order_type <> \$2\) AND "orders"\."deleted_at" IS NULL GROUP BY "ky"`).
		WithArgs("completed", "topup").
		WillReturnRows(sqlmock.NewRows([]string{"ky", "total_orders", "revenue", "discount"}).
			AddRow("2026-06-25", int64(10), int64(500000), int64(50000)))

	mock.ExpectQuery(`SELECT TO_CHAR\(created_at, 'YYYY-MM-DD'\) as ky, COALESCE\(SUM\(CASE WHEN transaction_type = 'combo_purchase' THEN -amount ELSE amount END\), 0\) as revenue FROM "member_transactions" WHERE \(transaction_type IN \('topup', 'refund'\) OR \(transaction_type = 'combo_purchase' AND payment_method NOT IN \('balance', 'bonus_balance', 'gift_card', 'topup_card'\)\)\) GROUP BY "ky"`).
		WillReturnRows(sqlmock.NewRows([]string{"ky", "revenue"}).
			AddRow("2026-06-25", int64(100000)).
			AddRow("2026-06-24", int64(30000)))

	result, err := svc.DailyRevenue("", "")
	require.NoError(t, err)
	require.Len(t, result, 2)
	// Ngày chỉ có tiền nạp vẫn có dòng, và các ngày xếp tăng dần.
	assert.Equal(t, DailyRevenueRow{Date: "2026-06-24", Revenue: 30000}, result[0])
	// Tiền đơn hàng cộng tiền sổ ví của cùng ngày. Số đơn chỉ đếm đơn bán hàng:
	// bản cũ cộng thêm số dòng nạp/hoàn nên ra 12 trong khi trang Đơn hàng có 10.
	assert.Equal(t, int64(600000), result[1].Revenue)
	assert.Equal(t, int64(10), result[1].TotalOrders)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReportService_MonthlyRevenue(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewReportService(db)

	mock.ExpectQuery(`SELECT TO_CHAR\(COALESCE\(orders\.completed_at, orders\.created_at\), 'YYYY-MM'\) as ky, COUNT\(\*\) as total_orders, .* FROM "orders" WHERE \(status = \$1 AND order_type <> \$2\) AND "orders"\."deleted_at" IS NULL GROUP BY "ky"`).
		WithArgs("completed", "topup").
		WillReturnRows(sqlmock.NewRows([]string{"ky", "total_orders", "revenue", "discount"}).
			AddRow("2026-06", int64(50), int64(3000000), int64(100000)))

	mock.ExpectQuery(`SELECT TO_CHAR\(created_at, 'YYYY-MM'\) as ky, .* FROM "member_transactions" .* GROUP BY "ky"`).
		WillReturnRows(sqlmock.NewRows([]string{"ky", "revenue"}).
			AddRow("2026-06", int64(200000)))

	result, err := svc.MonthlyRevenue("", "")
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "2026-06", result[0].Month)
	assert.Equal(t, int64(3200000), result[0].Revenue)
	assert.Equal(t, int64(50), result[0].TotalOrders)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Cách gọi cũ ?year=&month= đổi thành khoảng ngày. Thiếu tháng là cả năm —
// bản cũ dựng tháng 0, tức tháng 12 năm trước.
func TestMonthRange(t *testing.T) {
	cases := []struct {
		year, month int
		from, to    string
	}{
		{2026, 9, "2026-09-01", "2026-09-30"},
		{2026, 2, "2026-02-01", "2026-02-28"},
		{2026, 12, "2026-12-01", "2026-12-31"},
		{2026, 0, "2026-01-01", "2026-12-31"},
		{0, 5, "", ""},
	}
	for _, c := range cases {
		from, to := MonthRange(c.year, c.month)
		assert.Equal(t, c.from, from, "%d/%d", c.month, c.year)
		assert.Equal(t, c.to, to, "%d/%d", c.month, c.year)
	}
}

// Chi tiêu của hội viên = đơn bán hàng + tiền giờ + gói cước (đúng định nghĩa
// total_spent). Bản cũ cộng tiền NẠP vào chi tiêu và cộng tiền giờ dưới dạng số
// âm, còn "Số lần" đếm gộp đơn, lần nạp và dòng tiền giờ.
func TestReportService_ByMember(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewReportService(db)

	mock.ExpectQuery(`SELECT member_id, COALESCE\(SUM\(final_amount\), 0\) as spent FROM "orders" WHERE \(member_id IS NOT NULL AND status = \$1 AND order_type <> \$2\) AND "orders"\."deleted_at" IS NULL GROUP BY "member_id"`).
		WithArgs("completed", "topup").
		WillReturnRows(sqlmock.NewRows([]string{"member_id", "spent"}).
			AddRow("mem1", int64(42000)))

	mock.ExpectQuery(`SELECT member_id, COALESCE\(SUM\(CASE WHEN transaction_type = 'topup' THEN 0 ELSE -amount END\), 0\) as spent, COALESCE\(SUM\(CASE WHEN transaction_type = 'topup' THEN amount ELSE 0 END\), 0\) as topup FROM "member_transactions" WHERE transaction_type IN \('session_fee', 'combo_purchase', 'topup'\) GROUP BY "member_id"`).
		WillReturnRows(sqlmock.NewRows([]string{"member_id", "spent", "topup"}).
			AddRow("mem1", int64(47500), int64(100000)).
			AddRow("mem2", int64(0), int64(50000)))

	mock.ExpectQuery(`SELECT member_id, COUNT\(\*\) as count FROM "machine_sessions" WHERE member_id IS NOT NULL GROUP BY "member_id"`).
		WillReturnRows(sqlmock.NewRows([]string{"member_id", "count"}).
			AddRow("mem1", int64(2)))

	// Unscoped: hội viên đã xoá vẫn hiện tên, nên KHÔNG lọc deleted_at.
	mock.ExpectQuery(`FROM "members" WHERE id IN \(\$1,\$2\)$`).
		WithArgs("mem1", "mem2").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "full_name"}).
			AddRow("mem1", "hoang01", "Lê Hoàng").
			AddRow("mem2", "nam02", ""))

	result, err := svc.ByMember(ReportParams{})
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, ByMemberRow{MemberID: "mem1", MemberName: "Lê Hoàng", TotalSpent: 89500, TotalTopup: 100000, VisitCount: 2}, result[0])
	// Không có họ tên thì hiện tên đăng nhập thay vì ô trống.
	assert.Equal(t, ByMemberRow{MemberID: "mem2", MemberName: "nam02", TotalTopup: 50000}, result[1])
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Giới hạn áp SAU khi gộp đủ các nguồn: bản cũ cắt top theo tiền đơn hàng rồi
// mới cộng tiền giờ, nên khách chơi nhiều mà ít gọi món rơi khỏi danh sách.
func TestReportService_ByMember_CatSauKhiGop(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewReportService(db)

	mock.ExpectQuery(`FROM "orders"`).
		WillReturnRows(sqlmock.NewRows([]string{"member_id", "spent"}).AddRow("memA", int64(50000)))
	mock.ExpectQuery(`FROM "member_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"member_id", "spent", "topup"}).AddRow("memB", int64(80000), int64(0)))
	mock.ExpectQuery(`FROM "machine_sessions"`).
		WillReturnRows(sqlmock.NewRows([]string{"member_id", "count"}))
	mock.ExpectQuery(`FROM "members" WHERE id IN \(\$1\)$`).
		WithArgs("memB").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "full_name"}).AddRow("memB", "b", "B"))

	result, err := svc.ByMember(ReportParams{Limit: 1})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "memB", result[0].MemberID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReportService_ByMachine(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewReportService(db)

	mock.ExpectQuery(`SELECT machine_id, ROUND\(COALESCE\(SUM\(duration_minutes\), 0\) / 60\.0, 2\) as usage_hours, COALESCE\(SUM\(total_cost\), 0\) as total_sales, COUNT\(\*\) as session_count FROM "machine_sessions" WHERE ended_at IS NOT NULL GROUP BY "machine_id" ORDER BY total_sales desc`).
		WillReturnRows(sqlmock.NewRows([]string{"machine_id", "usage_hours", "total_sales", "session_count"}).
			AddRow("m1", int64(10), int64(150000), int64(4)))

	// Unscoped: máy đã gỡ khỏi danh sách vẫn phải hiện mã trong báo cáo cũ, nên
	// truy vấn KHÔNG được lọc theo deleted_at.
	mock.ExpectQuery(`SELECT "machine_code" FROM "machines" WHERE id = \$1 ORDER BY "machines"\."id" LIMIT \$2`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"machine_code"}).AddRow("M-001"))

	result, err := svc.ByMachine(ReportParams{})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "M-001", result[0].MachineCode)
	assert.Equal(t, int64(4), result[0].SessionCount)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Đơn được tính cho nhân viên đã thu tiền (updated_by), người tạo chỉ là
// phương án dự phòng, và JOIN users loại mã hội viên: đơn khách tự đặt ở máy
// trạm có created_by là mã HỘI VIÊN, bản cũ in nó thành một nhân viên không tên.
func TestReportService_ByEmployee(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewReportService(db)

	mock.ExpectQuery(`SELECT users\.id as employee_id, COALESCE\(NULLIF\(users\.full_name, ''\), users\.username\) as employee_name, COUNT\(\*\) as orders_taken, COALESCE\(SUM\(orders\.final_amount\), 0\) as total_sales FROM "orders" JOIN users ON users\.id = COALESCE\(orders\.updated_by, orders\.created_by\) WHERE \(orders\.status = \$1 AND orders\.order_type <> \$2\) AND "orders"\."deleted_at" IS NULL GROUP BY users\.id, users\.full_name, users\.username ORDER BY total_sales desc`).
		WithArgs("completed", "topup").
		WillReturnRows(sqlmock.NewRows([]string{"employee_id", "employee_name", "orders_taken", "total_sales"}).
			AddRow("u1", "Staff A", int64(20), int64(1000000)))

	result, err := svc.ByEmployee(ReportParams{})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Staff A", result[0].EmployeeName)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReportService_TopProducts(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewReportService(db)

	// Câu lệnh đổi có chủ đích: món bán chạy chỉ đếm đơn đã hoàn tất, nên phải
	// nối sang bảng orders và lọc trạng thái.
	mock.ExpectQuery(`SELECT order_items\.product_id, order_items\.product_name, SUM\(order_items\.quantity\) as quantity, COALESCE\(SUM\(order_items\.subtotal\), 0\) as total_sales FROM "order_items" JOIN orders ON orders\.id = order_items\.order_id WHERE orders\.status = \$1 GROUP BY order_items\.product_id, order_items\.product_name ORDER BY total_sales desc`).
		WithArgs("completed").
		WillReturnRows(sqlmock.NewRows([]string{"product_id", "product_name", "quantity", "total_sales"}).
			AddRow("p1", "Pepsi", int64(100), int64(200000)))

	result, err := svc.TopProducts(ReportParams{})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Pepsi", result[0].ProductName)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReportService_PromotionUsage(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewReportService(db)

	mock.ExpectQuery(`SELECT COALESCE\(promotion_id::text, ''\) as promotion_id, COUNT\(\*\) as usage_count, COALESCE\(SUM\(discount_amount\), 0\) as discount_given FROM "orders" WHERE \(discount_amount > 0 AND status = \$1\) AND "orders"\."deleted_at" IS NULL GROUP BY "promotion_id" ORDER BY usage_count desc`).
		WithArgs("completed").
		WillReturnRows(sqlmock.NewRows([]string{"promotion_id", "usage_count", "discount_given"}).
			AddRow("promo1", int64(3), int64(15000)))

	mock.ExpectQuery(`SELECT "name" FROM "promotions" WHERE id = \$1 AND "promotions"\."deleted_at" IS NULL ORDER BY "promotions"\."id" LIMIT \$2`).
		WithArgs("promo1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("Happy Hour"))

	result, err := svc.PromotionUsage(ReportParams{})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Happy Hour", result[0].PromotionName)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Mốc ngày của báo cáo phải là nửa đêm GIỜ VIỆT NAM. Bản cũ dùng time.Parse —
// nửa đêm UTC, tức 07:00 giờ ta — nên lọc "hôm nay → hôm nay" cắt mất cả ca đêm
// 00:00–07:00 và cộng nhầm 7 tiếng đầu của ngày kế tiếp.
//
// Mốc cuối là "< nửa đêm hôm sau": "<= 23:59:59" để lọt tiền thu lúc
// 23:59:59.6 ra khỏi cả hai ngày. Đơn lọc theo lúc thanh toán.
func TestReportService_DailyRevenue_MocNgayTheoGioVietNam(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewReportService(db)

	from := time.Date(2026, 9, 13, 0, 0, 0, 0, utils.VietnamLocation())
	to := time.Date(2026, 9, 14, 0, 0, 0, 0, utils.VietnamLocation())

	mock.ExpectQuery(`FROM "orders" WHERE \(status = \$1 AND order_type <> \$2\) AND COALESCE\(orders\.completed_at, orders\.created_at\) >= \$3 AND COALESCE\(orders\.completed_at, orders\.created_at\) < \$4`).
		WithArgs("completed", "topup", from, to).
		WillReturnRows(sqlmock.NewRows([]string{"ky", "total_orders", "revenue", "discount"}))
	// Ngoặc bao quanh vế OR phải còn nguyên khi ghép thêm bộ lọc ngày. Mất nó
	// thì AND bám chặt hơn OR và báo cáo một ngày cộng cả tiền nạp mọi ngày.
	mock.ExpectQuery(`FROM "member_transactions" WHERE \(\(transaction_type IN \('topup', 'refund'\) OR \(transaction_type = 'combo_purchase' AND payment_method NOT IN \(.*\)\)\)\) AND created_at >= \$1 AND created_at < \$2`).
		WithArgs(from, to).
		WillReturnRows(sqlmock.NewRows([]string{"ky", "revenue"}))

	_, err := svc.DailyRevenue("2026-09-13", "2026-09-13")

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
