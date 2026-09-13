package service

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/pkg/utils"
)

func TestReportService_DailyRevenue(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewReportService(db)

	mock.ExpectQuery(`SELECT DATE\(created_at\) as date, COUNT\(\*\) as total_orders, COALESCE\(SUM\(final_amount\), 0\) as revenue, COALESCE\(SUM\(discount_amount\), 0\) as discount FROM "orders" WHERE status = \$1 AND \(order_type <> \'topup\' AND payment_method = \'cash\'\) AND "orders"\."deleted_at" IS NULL GROUP BY DATE\(created_at\) ORDER BY date asc`).
		WithArgs("completed").
		WillReturnRows(sqlmock.NewRows([]string{"date", "total_orders", "revenue", "discount"}).
			AddRow("2026-06-25T00:00:00Z", int64(10), int64(500000), int64(50000)))

	mock.ExpectQuery(`SELECT DATE\(created_at\) as date, COALESCE\(SUM\(CASE WHEN transaction_type = 'combo_purchase' THEN -amount ELSE amount END\), 0\) as amount, COUNT\(\*\) as count FROM "member_transactions" WHERE \(transaction_type IN \('topup', 'refund'\) OR \(transaction_type = 'combo_purchase' AND payment_method = 'cash'\)\) GROUP BY "date" ORDER BY date asc`).
		WillReturnRows(sqlmock.NewRows([]string{"date", "amount", "count"}).
			AddRow("2026-06-25T00:00:00Z", int64(100000), int64(2)))

	// Nguồn thu thứ ba: tiền bán thẻ nạp, tính theo NGÀY BÁN chứ không phải
	// ngày khách nạp thẻ.
	mock.ExpectQuery(`SELECT DATE\(sold_at\) as date, COALESCE\(SUM\(face_value\), 0\) as amount, COUNT\(\*\) as count FROM "topup_cards" WHERE sold_at IS NOT NULL AND status <> 'cancelled' AND deleted_at IS NULL GROUP BY "date" ORDER BY date asc`).
		WillReturnRows(sqlmock.NewRows([]string{"date", "amount", "count"}))

	result, err := svc.DailyRevenue("", "")
	require.NoError(t, err)
	assert.Len(t, result, 1)
	// Order revenue merged with member-transaction revenue for the same day.
	assert.Equal(t, int64(600000), result[0].Revenue)
	assert.Equal(t, int64(12), result[0].TotalOrders)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReportService_MonthlyRevenue(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewReportService(db)

	mock.ExpectQuery(`SELECT TO_CHAR\(created_at, 'YYYY-MM'\) as month, COUNT\(\*\) as total_orders, COALESCE\(SUM\(final_amount\), 0\) as revenue, COALESCE\(SUM\(discount_amount\), 0\) as discount FROM "orders" WHERE status = \$1 AND \(order_type <> \'topup\' AND payment_method = \'cash\'\) AND "orders"\."deleted_at" IS NULL GROUP BY "month" ORDER BY month asc`).
		WithArgs("completed").
		WillReturnRows(sqlmock.NewRows([]string{"month", "total_orders", "revenue", "discount"}).
			AddRow("2026-06", int64(50), int64(3000000), int64(100000)))

	mock.ExpectQuery(`SELECT TO_CHAR\(created_at, 'YYYY-MM'\) as date, COALESCE\(SUM\(CASE WHEN transaction_type = 'combo_purchase' THEN -amount ELSE amount END\), 0\) as amount, COUNT\(\*\) as count FROM "member_transactions" WHERE \(transaction_type IN \('topup', 'refund'\) OR \(transaction_type = 'combo_purchase' AND payment_method = 'cash'\)\) GROUP BY "date" ORDER BY date asc`).
		WillReturnRows(sqlmock.NewRows([]string{"date", "amount", "count"}).
			AddRow("2026-06", int64(200000), int64(5)))

	mock.ExpectQuery(`SELECT TO_CHAR\(sold_at, 'YYYY-MM'\) as date, COALESCE\(SUM\(face_value\), 0\) as amount, COUNT\(\*\) as count FROM "topup_cards"`).
		WillReturnRows(sqlmock.NewRows([]string{"date", "amount", "count"}))

	result, err := svc.MonthlyRevenue(0, 0)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, int64(3200000), result[0].Revenue)
	assert.Equal(t, int64(55), result[0].TotalOrders)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReportService_ByMember(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewReportService(db)

	mock.ExpectQuery(`SELECT member_id, COUNT\(\*\) as visit_count, COALESCE\(SUM\(final_amount\), 0\) as total_spent FROM "orders" WHERE \(member_id IS NOT NULL AND status = \$1\) AND "orders"\."deleted_at" IS NULL GROUP BY "member_id" ORDER BY total_spent desc`).
		WithArgs("completed").
		WillReturnRows(sqlmock.NewRows([]string{"member_id", "visit_count", "total_spent"}).
			AddRow("mem1", int64(5), int64(200000)))

	mock.ExpectQuery(`SELECT member_id, COALESCE\(SUM\(amount\), 0\) as amount, COUNT\(\*\) as count FROM "member_transactions" WHERE transaction_type IN \('topup', 'session_fee'\) AND member_id IS NOT NULL GROUP BY "member_id"`).
		WillReturnRows(sqlmock.NewRows([]string{"member_id", "amount", "count"}).
			AddRow("mem1", int64(50000), int64(3)))

	mock.ExpectQuery(`SELECT "full_name" FROM "members" WHERE id = \$1 AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$2`).
		WithArgs("mem1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"full_name"}).AddRow("Test Member"))

	result, err := svc.ByMember(ReportParams{})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Test Member", result[0].MemberName)
	assert.Equal(t, int64(250000), result[0].TotalSpent)
	assert.Equal(t, int64(8), result[0].VisitCount)
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

func TestReportService_ByEmployee(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewReportService(db)

	mock.ExpectQuery(`SELECT created_by as employee_id, COUNT\(\*\) as orders_taken, COALESCE\(SUM\(final_amount\), 0\) as total_sales FROM "orders" WHERE \(created_by IS NOT NULL AND status = \$1\) AND "orders"\."deleted_at" IS NULL GROUP BY "created_by" ORDER BY total_sales desc`).
		WithArgs("completed").
		WillReturnRows(sqlmock.NewRows([]string{"employee_id", "orders_taken", "total_sales"}).
			AddRow("u1", int64(20), int64(1000000)))

	mock.ExpectQuery(`SELECT "full_name" FROM "users" WHERE id = \$1 AND "users"\."deleted_at" IS NULL ORDER BY "users"\."id" LIMIT \$2`).
		WithArgs("u1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"full_name"}).AddRow("Staff A"))

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
func TestReportService_DailyRevenue_MocNgayTheoGioVietNam(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewReportService(db)

	from := time.Date(2026, 9, 13, 0, 0, 0, 0, utils.VietnamLocation())
	to := time.Date(2026, 9, 13, 23, 59, 59, 0, utils.VietnamLocation())

	mock.ExpectQuery(`SELECT DATE\(created_at\).* FROM "orders"`).
		WithArgs("completed", from, to).
		WillReturnRows(sqlmock.NewRows([]string{"date", "total_orders", "revenue", "discount"}))
	// Ngoặc bao quanh vế OR phải còn nguyên khi ghép thêm bộ lọc ngày. Mất nó
	// thì AND bám chặt hơn OR và báo cáo một ngày cộng cả tiền nạp mọi ngày.
	mock.ExpectQuery(`FROM "member_transactions" WHERE \(\(transaction_type IN \('topup', 'refund'\) OR \(transaction_type = 'combo_purchase' AND payment_method = 'cash'\)\)\) AND created_at >= \$1 AND created_at <= \$2`).
		WithArgs(from, to).
		WillReturnRows(sqlmock.NewRows([]string{"date", "amount", "count"}))
	// Tiền bán thẻ cũng phải neo theo cùng mốc giờ Việt Nam.
	mock.ExpectQuery(`FROM "topup_cards" WHERE \(sold_at IS NOT NULL AND status <> 'cancelled' AND deleted_at IS NULL\) AND sold_at >= \$1 AND sold_at <= \$2`).
		WithArgs(from, to).
		WillReturnRows(sqlmock.NewRows([]string{"date", "amount", "count"}))

	_, err := svc.DailyRevenue("2026-09-13", "2026-09-13")

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
