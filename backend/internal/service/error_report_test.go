package service

import (
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/pkg/pagination"
)

// Cùng một lỗi trên hai hội viên / hai đơn khác nhau phải dồn về một dòng.
func TestErrorFingerprint_IgnoresIDsAndNumbers(t *testing.T) {
	a := errorFingerprint("server", "error", "GET /api/members/:id", "không tìm thấy hội viên 3f9a1c2e-0000-4000-8000-000000000001 sau 30s")
	b := errorFingerprint("server", "error", "GET /api/members/:id", "không tìm thấy hội viên 7b2d4e6f-1111-4111-8111-111111111111 sau 45s")
	assert.Equal(t, a, b)

	// Giá trị người dùng gõ nằm trong ngoặc kép (PostgreSQL, %q của Go).
	assert.Equal(t,
		errorFingerprint("server", "error", "GET /api/audit-logs", `ERROR: invalid input syntax for type uuid: "khong-phai-uuid" (SQLSTATE 22P02)`),
		errorFingerprint("server", "error", "GET /api/audit-logs", `ERROR: invalid input syntax for type uuid: "uuid-sai-khac" (SQLSTATE 22P02)`))
	// Ngoặc đơn là tên thuộc tính trong lỗi JS: khác tên là khác lỗi.
	assert.NotEqual(t,
		errorFingerprint("admin", "error", "/vnet/members", "Cannot read properties of undefined (reading 'balance')"),
		errorFingerprint("admin", "error", "/vnet/members", "Cannot read properties of undefined (reading 'name')"))

	assert.NotEqual(t, a, errorFingerprint("admin", "error", "GET /api/members/:id", "không tìm thấy hội viên"), "khác nguồn là khác lỗi")
	assert.NotEqual(t, a, errorFingerprint("server", "error", "GET /api/orders/:id", "không tìm thấy hội viên"), "khác vị trí là khác lỗi")
}

// Gặp lại: tăng count, thay mẫu gần nhất, và mở lại (resolved_at = NULL từ
// dòng chèn vào) nếu dev đã đánh dấu đã sửa.
func TestErrorReport_Record_UpsertsByFingerprint(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewErrorReportService(db)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "error_events" .* ON CONFLICT \("fingerprint"\) DO UPDATE SET "message"="excluded"."message","detail"="excluded"."detail","reporter"="excluded"."reporter","request_id"="excluded"."request_id","version"="excluded"."version","last_seen_at"="excluded"."last_seen_at","resolved_at"="excluded"."resolved_at","count"="error_events"."count" \+ 1 RETURNING "id"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("00000000-0000-0000-0000-000000000001"))
	mock.ExpectCommit()

	err := svc.Record(ErrorReportInput{Source: ErrorSourceServer, Message: "pq: deadlock detected", Location: "GET /api/members"})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestErrorReport_Record_RejectsEmptyMessage(t *testing.T) {
	db, _ := newMockDB(t)
	err := NewErrorReportService(db).Record(ErrorReportInput{Source: ErrorSourceAdmin, Message: "   "})
	assert.Error(t, err)
}

// Máy trạm gửi gì cũng phải vừa cột: một stack 1 MB không được làm hỏng câu
// INSERT (PostgreSQL không tự cắt varchar), và cắt theo ký tự, không theo byte.
func TestErrorReport_Record_TruncatesByRune(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewErrorReportService(db)

	longLoc := strings.Repeat("ă", 400)
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "error_events"`).
		WithArgs(sqlmock.AnyArg(), "client", "crash", strings.Repeat("ă", 300), "panic: boom",
			sqlmock.AnyArg(), "MAY01", "", "", int64(1), nil, anyTime{}, anyTime{}).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("00000000-0000-0000-0000-000000000001"))
	mock.ExpectCommit()

	err := svc.Record(ErrorReportInput{Source: "client", Kind: "crash", Location: longLoc, Message: "panic: boom", Detail: strings.Repeat("x", 20000), Reporter: "MAY01"})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Route by-code để mở: mã máy bịa thì không ghi gì.
func TestErrorReport_RecordMachineErrors_UnknownMachine(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewErrorReportService(db)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "machines" WHERE machine_code = \$1`).
		WithArgs("KHONG-CO").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	err := svc.RecordMachineErrors("KHONG-CO", []ErrorReportInput{{Message: "x"}})
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestErrorReport_RecordMachineErrors_TagsSourceAndMachine(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewErrorReportService(db)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "machines"`).
		WithArgs("MAY01").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "error_events"`).
		WithArgs(sqlmock.AnyArg(), "client", "error", "game", "không mở được LMHT",
			"", "MAY01", "", "1.0.13", int64(1), nil, anyTime{}, anyTime{}).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("00000000-0000-0000-0000-000000000001"))
	mock.ExpectCommit()

	err := svc.RecordMachineErrors("MAY01", []ErrorReportInput{{Location: "game", Message: "không mở được LMHT", Version: "1.0.13", Source: "server", Reporter: "gia-mao"}})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestErrorReport_List_DefaultsToOpenNewestFirst(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewErrorReportService(db)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "error_events" WHERE source = \$1 AND resolved_at IS NULL`).
		WithArgs("client").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT \* FROM "error_events" WHERE source = \$1 AND resolved_at IS NULL ORDER BY last_seen_at desc LIMIT \$2`).
		WithArgs("client", 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "source", "message", "count"}).AddRow("e1", "client", "boom", 3))

	items, total, page, size, err := svc.List(pagination.Params{Page: 1, PageSize: 20}, ErrorListParams{Source: "client"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, 1, page)
	assert.Equal(t, 20, size)
	require.Len(t, items, 1)
	assert.Equal(t, int64(3), items[0].Count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestErrorReport_List_SortedByColumn(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewErrorReportService(db)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "error_events"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(25))
	mock.ExpectQuery(`SELECT \* FROM "error_events" ORDER BY count asc, last_seen_at desc LIMIT \$1 OFFSET \$2`).
		WithArgs(20, 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "source", "message", "count"}).AddRow("e1", "client", "boom", 3))

	_, _, _, _, err := svc.List(
		pagination.Params{Page: 2, PageSize: 20, Sort: "count", Order: "asc", Sorted: true},
		ErrorListParams{Status: "all"},
	)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestErrorReport_Resolve(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewErrorReportService(db)
	id := "3f9a1c2e-0000-4000-8000-000000000001"

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "error_events" SET "resolved_at"=\$1 WHERE id = \$2`).
		WithArgs(anyTime{}, id).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	require.NoError(t, svc.Resolve(id))

	assert.Error(t, svc.Resolve("khong-phai-uuid"), "id sai dạng không chạm database")
	assert.NoError(t, mock.ExpectationsWereMet())
}
