package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
)

func TestChatService_SendMessage_Success(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewChatService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "chat_conversations" WHERE id = \$1 ORDER BY "chat_conversations"."id" LIMIT \$2`).
		WithArgs("conv1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title"}).AddRow("conv1", "Test"))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "chat_messages"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("msg1"))
	mock.ExpectCommit()

	result, err := svc.SendMessage(&SendMessageRequest{
		ConversationID: "conv1",
		SenderType:     "staff",
		SenderID:       "u1",
		Message:        "Hello",
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello", result.Message)
	assert.Equal(t, "text", result.MessageType)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestChatService_SendMessage_ConversationNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewChatService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "chat_conversations" WHERE id = \$1 ORDER BY "chat_conversations"."id" LIMIT \$2`).
		WithArgs("nonexistent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.SendMessage(&SendMessageRequest{
		ConversationID: "nonexistent",
		SenderType:     "staff",
		SenderID:       "u1",
		Message:        "Hello",
	})
	assert.Error(t, err)
	assert.Equal(t, "conversation not found", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestChatService_CreateConversation(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewChatService(db, nil, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "chat_conversations"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectQuery(`INSERT INTO "chat_participants"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("part1"))
	mock.ExpectCommit()

	result, err := svc.CreateConversation(&CreateConversationRequest{
		Title:           "Support",
		ParticipantID:   "u1",
		ParticipantType: "staff",
	})
	require.NoError(t, err)
	assert.Equal(t, "Support", result.Title)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestChatService_GetMessages(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewChatService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "chat_messages" WHERE conversation_id = \$1`).
		WithArgs("conv1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT \* FROM "chat_messages" WHERE conversation_id = \$1 ORDER BY created_at desc LIMIT \$2`).
		WithArgs("conv1", 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "conversation_id", "sender_type", "sender_id", "message", "message_type", "created_at"}).
			AddRow("msg1", "conv1", "staff", "u1", "Hello", "text", testNow))

	result, total, page, pageSize, err := svc.GetMessages("conv1", pagination.Params{Page: 1, PageSize: 20, Sort: "created_at", Order: "desc"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, result, 1)
	assert.Equal(t, 1, page)
	assert.Equal(t, 20, pageSize)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestChatService_MarkRead(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewChatService(db, nil, NewAuditService(db))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "chat_participants" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.MarkRead("conv1", "u1")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
