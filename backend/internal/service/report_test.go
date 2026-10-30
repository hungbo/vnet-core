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

// Bấm tiêu đề cột trên trang Giao dịch thì sắp theo cột đó trên toàn bộ sổ,
// mới nhất làm tiêu chí phụ. Truy vấn JOIN nên cột phải ghi rõ tên bảng.
func TestReportService_ListTransactions_SortedByColumn(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewReportService(db)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "member_transactions" LEFT JOIN members`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`FROM "member_transactions" LEFT JOIN members .* ` +
		`ORDER BY members\.username asc, member_transactions\.created_at DESC LIMIT \$1$`).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "member_username"}).AddRow("t1", "kane"))

	rows, total, err := svc.ListTransactions(&TransactionListParams{Sort: "member_username", Order: "asc"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}
