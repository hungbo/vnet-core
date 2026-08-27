package service

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Gửi lại đúng một khoá idempotency phải bị từ chối, và phải bị từ chối trước
// khi chạm vào số dư.
//
// Nạp tiền tại quầy không có trạng thái nào để mà kiểm: nạp hai lần cùng số tiền
// cho cùng khách là chuyện hoàn toàn hợp lệ. Chỉ phía gọi mới biết hai lần bấm
// là một ý định hay hai, nên nó gửi kèm khoá; khoá trùng là cùng một ý định.
func TestMemberService_Topup_RejectsDuplicateIdempotencyKey(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMemberService(db, NewAuditService(db))

	mock.ExpectBegin()
	// Khoá chính trùng: chèn hỏng, transaction bị rút lại, không đồng nào chuyển.
	mock.ExpectQuery(`INSERT INTO "idempotency_keys"`).
		WillReturnError(errors.New(`duplicate key value violates unique constraint "idempotency_keys_pkey"`))
	mock.ExpectRollback()

	_, err := svc.Topup("mem-1", &TopupRequest{
		Amount:         100000,
		PaymentMethod:  "cash",
		IdempotencyKey: "k-1",
	}, "user-1")

	require.ErrorIs(t, err, ErrDuplicateRequest)
	// Không có SELECT ... FOR UPDATE trên members, không có INSERT
	// member_transactions: mọi kỳ vọng đã khai đều khớp và không có gì khác chạy.
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Khoá rỗng nghĩa là phía gọi không tham gia — máy trạm Wails và các lần gọi API
// cũ không gửi khoá, và chúng vẫn phải nạp được như trước.
func TestMemberService_Topup_WithoutIdempotencyKeySkipsClaim(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMemberService(db, NewAuditService(db))

	mock.ExpectBegin()
	// Không có INSERT "idempotency_keys" ở đây: đi thẳng vào việc khoá hội viên.
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$2 FOR UPDATE`).
		WithArgs("mem-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance", "bonus_balance"}).
			AddRow("mem-1", int64(20000), int64(0)))
	mock.ExpectQuery(`INSERT INTO "member_transactions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("tx-1"))
	mock.ExpectExec(`UPDATE "members" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// GetByID sau khi commit.
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance"}).AddRow("mem-1", int64(120000)))

	_, err := svc.Topup("mem-1", &TopupRequest{Amount: 100000, PaymentMethod: "cash"}, "user-1")

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
