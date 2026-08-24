package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/hub"
)

// Dispatch used to write only notification_recipients, a table nothing reads,
// so the member-facing inbox stayed empty no matter what admins sent.
func TestNotificationAdminService_Dispatch_FillsMemberInbox(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewNotificationAdminService(db, hub.New(nil), NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "notifications" WHERE id = \$1 ORDER BY "notifications"\."id" LIMIT \$2`).
		WithArgs("n1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "content"}).
			AddRow("n1", "Khuyến mãi", "Giảm 20% hôm nay"))

	mock.ExpectQuery(`SELECT "id" FROM "members"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("mem-1").AddRow("mem-2"))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "notification_recipients"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("r1").AddRow("r2"))
	mock.ExpectQuery(`INSERT INTO "member_notifications"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("i1").AddRow("i2"))
	mock.ExpectCommit()

	count, err := svc.Dispatch("n1")

	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationAdminService_Dispatch_NoMembersIsNoop(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewNotificationAdminService(db, hub.New(nil), NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "notifications"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "content"}).AddRow("n1", "T", "B"))
	mock.ExpectQuery(`SELECT "id" FROM "members"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	count, err := svc.Dispatch("n1")

	require.NoError(t, err)
	assert.Equal(t, 0, count)
	assert.NoError(t, mock.ExpectationsWereMet())
}
