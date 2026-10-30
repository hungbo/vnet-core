package service

import (
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/pkg/pagination"
)

const (
	keyA = "ABCDE-FGHIJ-KLMNO-PQRST-UVWXY"
	keyB = "11111-22222-33333-44444-55555"
)

func TestNormalizeWindowsKey(t *testing.T) {
	cases := map[string]struct {
		in   string
		want string
		ok   bool
	}{
		"đúng dạng":             {keyA, keyA, true},
		"chữ thường, thừa cách": {"  abcde-fghij-klmno-pqrst-uvwxy ", keyA, true},
		"dán liền 25 ký tự":     {"abcdefghijklmnopqrstuvwxy", keyA, true},
		"thiếu một nhóm":        {"ABCDE-FGHIJ-KLMNO-PQRST", "ABCDE-FGHIJ-KLMNO-PQRST", false},
		"ký tự lạ":              {"ABCDE-FGHIJ-KLMNO-PQRST-UVWX!", "ABCDE-FGHIJ-KLMNO-PQRST-UVWX!", false},
		"rỗng":                  {"", "", false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, ok := NormalizeWindowsKey(c.in)
			assert.Equal(t, c.ok, ok)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestMaskWindowsKey(t *testing.T) {
	assert.Equal(t, "XXXXX-XXXXX-XXXXX-XXXXX-UVWXY", MaskWindowsKey(keyA))
	assert.NotContains(t, MaskWindowsKey(keyA), "ABCDE")
	assert.Equal(t, "", MaskWindowsKey("abc"))
}

func TestWindowsKeyRecord_Row(t *testing.T) {
	id := "k1"
	base := windowsKeyRecord{MachineID: "m1", MachineCode: "M-001", KeyID: &id, FullKey: keyA}

	r := base.row()
	assert.True(t, r.HasKey)
	assert.Equal(t, "unknown", r.Match, "máy chưa báo đuôi key")
	assert.NotContains(t, r.KeyMasked, "ABCDE")

	base.WinKeyTail = "UVWXY"
	assert.Equal(t, "match", base.row().Match)
	base.WinKeyTail = "ZZZZZ"
	assert.Equal(t, "mismatch", base.row().Match)

	none := windowsKeyRecord{MachineID: "m2", MachineCode: "M-002", WinKeyTail: "UVWXY"}.row()
	assert.False(t, none.HasKey)
	assert.Empty(t, none.Match, "chưa có key thì không có gì để so")
	assert.Empty(t, none.KeyMasked)
}

func TestParseImport(t *testing.T) {
	text := "# danh sách\r\n\r\nM-001 " + keyA + "\nm-002, abcdefghijklmnopqrstuvwxy\n" +
		keyB + "\tM-003\nM-004 KHONG-PHAI-KEY\nchỉ-một-token\nM-005 " + keyA + " thừa\n"
	got, err := parseImport(text)
	require.NoError(t, err)
	require.Len(t, got, 6)

	assert.Equal(t, parsedImport{line: 3, code: "M-001", key: keyA}, got[0])
	assert.Equal(t, parsedImport{line: 4, code: "m-002", key: keyA}, got[1], "dán liền 25 ký tự cũng được")
	assert.Equal(t, parsedImport{line: 5, code: "M-003", key: keyB}, got[2], "đảo thứ tự: key trước, mã máy sau")
	assert.Equal(t, "invalid_key", got[3].invalid)
	assert.Equal(t, "invalid_line", got[4].invalid)
	assert.Equal(t, "invalid_line", got[5].invalid, "ba token là dòng hỏng chứ không đoán")

	_, err = parseImport("\n# chỉ ghi chú\n")
	assert.ErrorIs(t, err, ErrWindowsKeyEmptyImport)

	var many []string
	for i := 0; i < windowsKeyImportMax+1; i++ {
		many = append(many, "M-001 "+keyA)
	}
	_, err = parseImport(strings.Join(many, "\n"))
	assert.ErrorIs(t, err, ErrWindowsKeyTooMany)
}

func importDB(t *testing.T) (*WindowsKeyService, sqlmock.Sqlmock) {
	db, mock := newMockDB(t)
	return NewWindowsKeyService(db, NewAuditService(db)), mock
}

// Xem trước: báo từng dòng nhưng không ghi gì (sqlmock báo lỗi nếu có INSERT).
func TestWindowsKeyService_Import_DryRun(t *testing.T) {
	svc, mock := importDB(t)
	mock.ExpectQuery(`SELECT id, machine_code FROM "machines" WHERE lower\(machine_code\) IN \(\$1,\$2,\$3\) AND deleted_at IS NULL`).
		WithArgs("m-001", "m-002", "m-999").
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code"}).AddRow("id1", "M-001").AddRow("id2", "M-002"))
	mock.ExpectQuery(`SELECT "machine_id" FROM "windows_keys" WHERE machine_id IN \(\$1,\$2\)`).
		WillReturnRows(sqlmock.NewRows([]string{"machine_id"}).AddRow("id2"))

	res, err := svc.Import("M-001 "+keyA+"\nM-002 "+keyB+"\nM-999 "+keyA+"\nM-001 "+keyB, false, true, "u1")
	require.NoError(t, err)
	assert.True(t, res.DryRun)
	assert.Equal(t, 0, res.Applied)
	var got []string
	for _, r := range res.Rows {
		got = append(got, r.MachineCode+":"+r.Status)
	}
	assert.Equal(t, []string{"M-001:ok", "M-002:exists", "M-999:unknown_machine", "M-001:duplicate"}, got)
	assert.NotContains(t, res.Rows[0].KeyMasked, "ABCDE", "kết quả trả về cũng chỉ có bản che")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Nhập thật với overwrite: máy đã có key được thay, ghi cả lô bằng MỘT câu INSERT ... ON CONFLICT.
func TestWindowsKeyService_Import_Apply(t *testing.T) {
	svc, mock := importDB(t)
	mock.ExpectQuery(`SELECT id, machine_code FROM "machines"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code"}).AddRow("id1", "M-001").AddRow("id2", "M-002"))
	mock.ExpectQuery(`SELECT "machine_id" FROM "windows_keys"`).
		WillReturnRows(sqlmock.NewRows([]string{"machine_id"}).AddRow("id2"))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "windows_keys" .* ON CONFLICT \("machine_id"\) DO UPDATE SET "key"="excluded"."key","edition"="excluded"."edition","note"="excluded"."note","updated_at"="excluded"."updated_at"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID).AddRow(testUUID))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "audit_logs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow("a1", time.Now()))
	mock.ExpectCommit()

	res, err := svc.Import("M-001 "+keyA+"\nM-002 "+keyB, true, false, "")
	require.NoError(t, err)
	assert.Equal(t, 2, res.Applied)
	assert.Equal(t, "ok", res.Rows[0].Status)
	assert.Equal(t, "replaced", res.Rows[1].Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Bấm sắp xếp cột Cập nhật: cột của bảng windows_keys đứng trước, mã máy làm tiêu chí phụ.
func TestWindowsKeyService_List_SapTheoCot(t *testing.T) {
	svc, mock := importDB(t)
	mock.ExpectQuery(`SELECT count\(\*\) FROM machines AS m LEFT JOIN windows_keys AS k ON k\.machine_id = m\.id WHERE m\.deleted_at IS NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`FROM machines AS m LEFT JOIN windows_keys AS k ON k\.machine_id = m\.id WHERE m\.deleted_at IS NULL ORDER BY k\.updated_at desc, m\.machine_code asc LIMIT \$1`).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{"machine_id", "machine_code"}).AddRow("m1", "M-001"))

	params := pagination.Params{Page: 1, PageSize: 20, Sort: "updated_at", Order: "desc", Sorted: true}
	res, err := svc.List(params, WindowsKeyFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), res.Total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWindowsKeyService_SetKey_Validation(t *testing.T) {
	svc, _ := importDB(t)
	assert.ErrorIs(t, svc.SetKey("m1", "khong-phai-key", "", "", "u1"), ErrWindowsKeyInvalid)
}

func TestWindowsKeyService_Reveal_GhiNhatKy(t *testing.T) {
	svc, mock := importDB(t)
	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code"}).AddRow("m1", "M-001"))
	mock.ExpectQuery(`SELECT \* FROM "windows_keys" WHERE machine_id = \$1`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_id", "key"}).AddRow("k1", "m1", keyA))
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "audit_logs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow("a1", time.Now()))
	mock.ExpectCommit()

	key, err := svc.RevealKey("m1", "")
	require.NoError(t, err)
	assert.Equal(t, keyA, key)
	assert.NoError(t, mock.ExpectationsWereMet(), "xem key phải để lại dòng nhật ký")
}
