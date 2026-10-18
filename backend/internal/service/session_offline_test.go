package service

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// Máy ngủ cả đêm (Lightning Boot) thì last_heartbeat cũ hàng giờ. Khách bấm nút
// rồi đăng nhập trước nhịp tim đầu tiên: phiên vừa mở không được bị coi là phiên
// trên máy mất tín hiệu — câu lọc phải loại phiên mở sau mốc timeout.
func TestEndSessionsOnOfflineMachines_BoQuaPhienVuaMo(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSessionService(db, nil, NewAuditService(db))

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT machine_sessions.id AS session_id, machines.last_heartbeat AS last_heartbeat FROM "machine_sessions" `+
			`JOIN machines ON machines.id = machine_sessions.machine_id `+
			`WHERE machine_sessions.is_active = $1 AND machines.last_heartbeat IS NOT NULL AND machines.last_heartbeat < $2 AND machine_sessions.started_at < $3`)).
		WithArgs(true, anyTime{}, anyTime{}).
		WillReturnRows(sqlmock.NewRows([]string{"session_id", "last_heartbeat"}))

	n, err := svc.EndSessionsOnOfflineMachines(2 * time.Minute)
	require.NoError(t, err)
	require.Equal(t, 0, n)
	require.NoError(t, mock.ExpectationsWereMet())
}
