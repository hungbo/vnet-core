package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportService_DailyRevenue(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewReportService(db)

	mock.ExpectQuery(`SELECT DATE\(created_at\) as date, COUNT\(\*\) as total_orders, COALESCE\(SUM\(final_amount\), 0\) as revenue, COALESCE\(SUM\(discount_amount\), 0\) as discount FROM "orders" WHERE status = \$1 GROUP BY DATE\(created_at\) ORDER BY date asc`).
		WithArgs("completed").
		WillReturnRows(sqlmock.NewRows([]string{"date", "total_orders", "revenue", "discount"}).
			AddRow("2026-06-25T00:00:00Z", int64(10), int64(500000), int64(50000)))

	result, err := svc.DailyRevenue("", "")
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, int64(500000), result[0].Revenue)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReportService_MonthlyRevenue(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewReportService(db)

	mock.ExpectQuery(`SELECT TO_CHAR\(created_at, 'YYYY-MM'\) as month, COUNT\(\*\) as total_orders, COALESCE\(SUM\(final_amount\), 0\) as revenue, COALESCE\(SUM\(discount_amount\), 0\) as discount FROM "orders" WHERE status = \$1 GROUP BY "month" ORDER BY month asc`).
		WithArgs("completed").
		WillReturnRows(sqlmock.NewRows([]string{"month", "total_orders", "revenue", "discount"}).
			AddRow("2026-06", int64(50), int64(3000000), int64(100000)))

	result, err := svc.MonthlyRevenue(0, 0)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, int64(3000000), result[0].Revenue)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReportService_ByMember(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewReportService(db)

	mock.ExpectQuery(`SELECT member_id, COUNT\(\*\) as visit_count, COALESCE\(SUM\(final_amount\), 0\) as total_spent FROM "orders" WHERE member_id IS NOT NULL AND status = \$1 GROUP BY "member_id" ORDER BY total_spent desc`).
		WithArgs("completed").
		WillReturnRows(sqlmock.NewRows([]string{"member_id", "visit_count", "total_spent"}).
			AddRow("mem1", int64(5), int64(200000)))

	mock.ExpectQuery(`SELECT "full_name" FROM "members" WHERE id = \$1 ORDER BY "members"."id" LIMIT \$2`).
		WithArgs("mem1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"full_name"}).AddRow("Test Member"))

	result, err := svc.ByMember(ReportParams{})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Test Member", result[0].MemberName)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReportService_ByMachine(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewReportService(db)

	mock.ExpectQuery(`SELECT machine_id, COUNT\(\*\) as usage_hours, COALESCE\(SUM\(total_cost\), 0\) as total_sales FROM "machine_sessions" WHERE ended_at IS NOT NULL GROUP BY "machine_id" ORDER BY total_sales desc`).
		WillReturnRows(sqlmock.NewRows([]string{"machine_id", "usage_hours", "total_sales"}).
			AddRow("m1", int64(10), int64(150000)))

	mock.ExpectQuery(`SELECT "machine_code" FROM "machines" WHERE id = \$1 ORDER BY "machines"."id" LIMIT \$2`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"machine_code"}).AddRow("M-001"))

	result, err := svc.ByMachine(ReportParams{})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "M-001", result[0].MachineName)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReportService_ByEmployee(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewReportService(db)

	mock.ExpectQuery(`SELECT created_by as employee_id, COUNT\(\*\) as orders_taken, COALESCE\(SUM\(final_amount\), 0\) as total_sales FROM "orders" WHERE created_by IS NOT NULL AND status = \$1 GROUP BY "created_by" ORDER BY total_sales desc`).
		WithArgs("completed").
		WillReturnRows(sqlmock.NewRows([]string{"employee_id", "orders_taken", "total_sales"}).
			AddRow("u1", int64(20), int64(1000000)))

	mock.ExpectQuery(`SELECT "full_name" FROM "users" WHERE id = \$1 ORDER BY "users"."id" LIMIT \$2`).
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

	mock.ExpectQuery(`SELECT product_id, product_name, SUM\(quantity\) as quantity, COALESCE\(SUM\(subtotal\), 0\) as total_sales FROM "order_items" GROUP BY product_id, product_name ORDER BY total_sales desc`).
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

	mock.ExpectQuery(`SELECT COALESCE\(promotion_id, ''\) as promotion_id, COUNT\(\*\) as usage_count, COALESCE\(SUM\(discount_amount\), 0\) as discount_given FROM "orders" WHERE discount_amount > 0 AND status = \$1 GROUP BY "promotion_id" ORDER BY usage_count desc`).
		WithArgs("completed").
		WillReturnRows(sqlmock.NewRows([]string{"promotion_id", "usage_count", "discount_given"}).
			AddRow("promo1", int64(3), int64(15000)))

	mock.ExpectQuery(`SELECT "name" FROM "promotions" WHERE id = \$1 ORDER BY "promotions"."id" LIMIT \$2`).
		WithArgs("promo1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("Happy Hour"))

	result, err := svc.PromotionUsage(ReportParams{})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Happy Hour", result[0].PromotionName)
	assert.NoError(t, mock.ExpectationsWereMet())
}
