package service

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/utils"
)

var cotTongBaoCao = []string{"money_in", "sales", "sale_count", "topups", "topup_count", "combos", "combo_count",
	"refunds", "paid_by_balance", "session_fees", "session_count", "minutes", "discount", "line_count"}

func TestReportFilter_Validate(t *testing.T) {
	assert.NoError(t, ReportFilter{}.Validate())
	assert.NoError(t, ReportFilter{Kind: "topup", GroupBy: "product", EmployeeID: "none"}.Validate())
	assert.Error(t, ReportFilter{Kind: "fnet_import"}.Validate())
	assert.Error(t, ReportFilter{GroupBy: "hour"}.Validate())
	assert.Error(t, ReportFilter{MachineID: "MAY01"}.Validate())
	assert.Error(t, ReportFilter{EmployeeID: "casang"}.Validate())
}

// Tiền nạp do nhân viên thu phải nằm trong cùng tập dòng với đơn bán hàng:
// bản cũ của "Theo nhân viên" chỉ đọc bảng orders nên ca nạp 2,9 triệu chỉ hiện
// 3,1 triệu tiền món. Lọc theo nhân viên áp cho cả thẻ tổng lẫn bảng.
func TestReportService_Overview_TungDongTheoNhanVien(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewReportService(db)
	nv := "11111111-1111-1111-1111-111111111111"
	from := time.Date(2026, 10, 30, 0, 0, 0, 0, utils.VietnamLocation())
	to := from.AddDate(0, 0, 1)

	mock.ExpectQuery(`(?s)SELECT COALESCE\(SUM\(d\.money_in\), 0\) AS money_in,.* FROM \(SELECT o\.id AS ref_id.*FROM orders o.*UNION ALL.*FROM member_transactions t.*WHERE t\.transaction_type IN \('topup', 'refund', 'combo_purchase'\).*UNION ALL.*FROM machine_sessions s.*\) AS d LEFT JOIN members m ON m\.id = d\.member_id LEFT JOIN machines mc ON mc\.id = d\.machine_id WHERE d\.at >= \$1 AND d\.at < \$2 AND d\.employee_id = \$3$`).
		WithArgs(from, to, nv).
		WillReturnRows(sqlmock.NewRows(cotTongBaoCao).
			AddRow(6047000, 3102000, 43, 2945000, 82, 0, 0, 0, 0, 0, 0, 0, 0, 125))
	mock.ExpectQuery(`(?s)SELECT d\.ref_id AS id, d\.kind, d\.at.* AS note FROM \(SELECT .*\) AS d .* WHERE d\.at >= \$1 AND d\.at < \$2 AND d\.employee_id = \$3 ORDER BY d\.money_in asc, d\.at DESC LIMIT \$4 OFFSET \$5$`).
		WithArgs(from, to, nv, 50, 50).
		WillReturnRows(sqlmock.NewRows([]string{"id", "kind", "at", "code", "employee_name", "amount", "money_in"}).
			AddRow("t1", "topup", from, "", "casang", 50000, 50000))

	kq, err := svc.Overview(ReportFilter{DateFrom: "2026-10-30", DateTo: "2026-10-30", EmployeeID: nv},
		pagination.Params{Page: 2, PageSize: 50, Sort: "money_in", Order: "asc", Sorted: true})

	require.NoError(t, err)
	assert.Equal(t, int64(6047000), kq.Totals.MoneyIn)
	assert.Equal(t, int64(2945000), kq.Totals.Topups)
	assert.Equal(t, int64(125), kq.Total)
	lines := kq.Rows.([]ReportLine)
	require.Len(t, lines, 1)
	assert.Equal(t, "casang", lines[0].EmployeeName)
	assert.Equal(t, int64(50000), lines[0].MoneyIn)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// "Không có nhân viên" là các dòng khách tự làm ở máy trạm và tiền giờ.
func TestReportService_Overview_NhomTheoNhanVien(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewReportService(db)

	mock.ExpectQuery(`SELECT COALESCE\(SUM\(d\.money_in\), 0\) AS money_in`).
		WithArgs("topup").
		WillReturnRows(sqlmock.NewRows(cotTongBaoCao))
	mock.ExpectQuery(`(?s)SELECT COALESCE\(d\.employee_id::text, ''\) AS key, COALESCE\(MAX\(d\.employee_name\), ''\) AS label, COALESCE\(SUM\(d\.money_in\), 0\) AS money_in,.* WHERE d\.kind = \$1 AND d\.employee_id IS NULL GROUP BY COALESCE\(d\.employee_id::text, ''\) ORDER BY money_in DESC, sales DESC, label$`).
		WithArgs("topup").
		WillReturnRows(sqlmock.NewRows([]string{"key", "label", "money_in", "topups"}).AddRow("", "", 1000, 1000))

	kq, err := svc.Overview(ReportFilter{Kind: "topup", EmployeeID: "none", GroupBy: "employee"}, pagination.Params{Page: 1, PageSize: 20})

	require.NoError(t, err)
	rows := kq.Rows.([]ReportGroupRow)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(1000), rows[0].MoneyIn)
	assert.Equal(t, int64(1), kq.Total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Ngày xếp theo thời gian, không theo tiền.
func TestReportService_Overview_NhomTheoNgay(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewReportService(db)

	mock.ExpectQuery(`SELECT COALESCE\(SUM\(d\.money_in\), 0\) AS money_in`).WillReturnRows(sqlmock.NewRows(cotTongBaoCao))
	mock.ExpectQuery(`(?s)SELECT TO_CHAR\(d\.at, 'YYYY-MM-DD'\) AS key, '' AS label, .* GROUP BY TO_CHAR\(d\.at, 'YYYY-MM-DD'\) ORDER BY key$`).
		WillReturnRows(sqlmock.NewRows([]string{"key", "label", "money_in"}))

	_, err := svc.Overview(ReportFilter{GroupBy: "day"}, pagination.Params{Page: 1, PageSize: 20})

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Lọc một món rồi xem theo món thì chỉ đếm đúng món ấy, không đếm các món
// khác nằm chung đơn.
func TestReportService_Overview_NhomTheoMon(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewReportService(db)
	mon := "22222222-2222-2222-2222-222222222222"

	mock.ExpectQuery(`SELECT COALESCE\(SUM\(d\.money_in\), 0\) AS money_in`).
		WithArgs(mon).
		WillReturnRows(sqlmock.NewRows(cotTongBaoCao))
	mock.ExpectQuery(`(?s)SELECT oi\.product_id::text AS key, MAX\(oi\.product_name\) AS label, SUM\(oi\.quantity\) AS quantity, .* JOIN order_items oi ON oi\.order_id = d\.ref_id WHERE \(d\.kind = 'sale' AND EXISTS \(SELECT 1 FROM order_items oi WHERE oi\.order_id = d\.ref_id AND oi\.product_id = \$1\)\) AND d\.kind = 'sale' AND oi\.product_id = \$2 GROUP BY "oi"\."product_id" ORDER BY sales DESC, label$`).
		WithArgs(mon, mon).
		WillReturnRows(sqlmock.NewRows([]string{"key", "label", "quantity", "sales", "sale_count"}).AddRow(mon, "Sting", 5, 75000, 4))

	kq, err := svc.Overview(ReportFilter{ProductID: mon, GroupBy: "product"}, pagination.Params{Page: 1, PageSize: 20})

	require.NoError(t, err)
	rows := kq.Rows.([]ReportGroupRow)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(5), rows[0].Quantity)
	assert.Equal(t, int64(75000), rows[0].Sales)
	assert.NoError(t, mock.ExpectationsWereMet())
}
