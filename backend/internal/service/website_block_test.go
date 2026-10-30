package service

import (
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tên miền dài hơn cột varchar(500) từng để PostgreSQL tự báo lỗi, và câu
// "value too long for type character varying(500) (SQLSTATE 22001)" lọt nguyên
// văn ra màn hình nhân viên. Chặn phải xảy ra TRƯỚC khi chạm cơ sở dữ liệu.
func TestWebsiteBlockService_CreateRule_ChanTenMienQuaDai(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewWebsiteBlockService(db, NewAuditService(db))

	_, err := svc.CreateRule(&CreateRuleRequest{
		Pattern: strings.Repeat("a", 520) + ".com",
	}, "user-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "quá dài")
	// Không có câu lệnh nào được gửi xuống cơ sở dữ liệu.
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Ô nhập nhận cả URL đầy đủ — phần đường dẫn bị cắt khi chuẩn hoá — nên phép đo
// độ dài phải tính trên tên miền SAU chuẩn hoá, không phải chuỗi thô.
func TestWebsiteBlockService_CreateRule_UrlDaiNhungTenMienNganVanHopLe(t *testing.T) {
	db, _ := newMockDB(t)
	svc := NewWebsiteBlockService(db, NewAuditService(db))

	_, err := svc.CreateRule(&CreateRuleRequest{
		Pattern: "https://facebook.com/" + strings.Repeat("x", 600),
	}, "user-1")

	// Không mô tả câu INSERT nên lệnh tạo sẽ hỏng ở tầng sqlmock; điều cần
	// khẳng định là nó KHÔNG bị chặn vì lý do độ dài.
	if err != nil {
		assert.NotContains(t, err.Error(), "quá dài")
		assert.NotContains(t, err.Error(), "không hợp lệ")
	}
}

// Bấm sắp xếp cột Máy: mã máy không có trong bảng vi phạm nên sắp bằng truy vấn con, mới nhất
// trước làm tiêu chí phụ.
func TestWebsiteBlockService_ListViolations_SapTheoCot(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewWebsiteBlockService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "website_blocking_violations"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT \* FROM "website_blocking_violations" ORDER BY \(SELECT machine_code FROM machines WHERE machines\.id = website_blocking_violations\.machine_id\) asc, blocked_at desc LIMIT \$1`).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_id", "domain"}).AddRow("v1", "m1", "facebook.com"))
	mock.ExpectQuery(`FROM "machines" WHERE id IN \(\$1\)`).
		WithArgs("m1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code"}).AddRow("m1", "M-001"))

	res, err := svc.ListViolations(&ViolationListRequest{Sort: "machine_code", Order: "asc"})
	require.NoError(t, err)
	rows := res.Items.([]ViolationRow)
	require.Len(t, rows, 1)
	assert.Equal(t, "M-001", rows[0].MachineCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}
