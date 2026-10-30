package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Bấm tiêu đề cột trên trang Đánh giá thì sắp theo cột đó trên toàn bộ đánh
// giá, mới nhất làm tiêu chí phụ; không bấm thì vẫn mới nhất trước.
func TestFeedbackService_List_SortedByColumn(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewFeedbackService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "service_feedbacks" WHERE rating >= \$1`).
		WithArgs(4).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	mock.ExpectQuery(`SELECT \* FROM "service_feedbacks" WHERE rating >= \$1 ORDER BY rating desc, created_at desc LIMIT \$2`).
		WithArgs(4, 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_id", "rating"}))

	res, err := svc.List(&FeedbackListRequest{MinRating: 4, Sort: "rating", Order: "desc"})
	require.NoError(t, err)
	assert.Equal(t, int64(3), res.Total)
	assert.NoError(t, mock.ExpectationsWereMet())
}
