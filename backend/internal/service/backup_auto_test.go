package service

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/config"
)

// Quán chưa từng mở trang Sao lưu vẫn phải được sao lưu mỗi giờ, giữ 48 bản.
func TestReadAutoBackupConfig_MacDinhMoiGio(t *testing.T) {
	cfg := readAutoBackupConfig(map[string]string{})
	assert.True(t, cfg.Enabled)
	assert.Equal(t, time.Hour, cfg.Interval)
	assert.Equal(t, int64(48), cfg.Keep)
}

func TestReadAutoBackupConfig_TheoPhutNgayVaTat(t *testing.T) {
	cfg := readAutoBackupConfig(map[string]string{
		"auto_enabled": "false", "auto_every": "30", "auto_unit": "minute", "auto_keep": "0",
	})
	assert.False(t, cfg.Enabled)
	assert.Equal(t, 30*time.Minute, cfg.Interval)
	assert.Equal(t, int64(0), cfg.Keep)

	cfg = readAutoBackupConfig(map[string]string{"auto_every": "2", "auto_unit": "day"})
	assert.Equal(t, 48*time.Hour, cfg.Interval)

	// Dưới 5 phút thì kéo lên 5 phút; đơn vị lạ thì về giờ.
	cfg = readAutoBackupConfig(map[string]string{"auto_every": "1", "auto_unit": "minute"})
	assert.Equal(t, 5*time.Minute, cfg.Interval)
	cfg = readAutoBackupConfig(map[string]string{"auto_every": "3", "auto_unit": "week"})
	assert.Equal(t, 3*time.Hour, cfg.Interval)
}

func TestKiemTraCaiDatSaoLuu(t *testing.T) {
	ok := []map[string]interface{}{
		{"auto_enabled": true, "auto_every": float64(1), "auto_unit": "hour", "auto_keep": float64(48)},
		{"auto_every": float64(15), "auto_unit": "minute"},
		{"auto_every": "2", "auto_unit": "day", "auto_keep": float64(0)},
		{"auto_enabled": false},
	}
	for _, in := range ok {
		assert.NoError(t, kiemTraCaiDatSaoLuu(in), "%v", in)
	}
	bad := []map[string]interface{}{
		{"auto_every": float64(0)},
		{"auto_every": 1.5},
		{"auto_unit": "week"},
		{"auto_every": float64(1), "auto_unit": "minute"},
		{"auto_keep": float64(-1)},
		{"auto_enabled": "yes"},
	}
	for _, in := range bad {
		assert.Error(t, kiemTraCaiDatSaoLuu(in), "%v", in)
	}
}

func TestBackupService_AutoBackupDue(t *testing.T) {
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.Local)
	newSvc := func(t *testing.T) (*BackupService, sqlmock.Sqlmock) {
		db, mock := newMockDB(t)
		return NewBackupService(db, &config.DatabaseConfig{}, t.TempDir(), NewAuditService(db)), mock
	}
	expectRunning := func(mock sqlmock.Sqlmock, n int) {
		mock.ExpectQuery(`SELECT count\(\*\) FROM "backup_logs" WHERE status = \$1 AND started_at > \$2`).
			WithArgs("running", now.Add(-2*time.Hour)).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(n))
	}
	expectLast := func(mock sqlmock.Sqlmock, started *time.Time) {
		rows := sqlmock.NewRows([]string{"id", "status", "started_at"})
		if started != nil {
			rows.AddRow("b1", "completed", *started)
		}
		mock.ExpectQuery(`SELECT \* FROM "backup_logs" ORDER BY started_at DESC`).WillReturnRows(rows)
	}

	t.Run("đang có bản chạy thì đợi", func(t *testing.T) {
		svc, mock := newSvc(t)
		expectRunning(mock, 1)
		due, err := svc.autoBackupDue(now, time.Hour)
		require.NoError(t, err)
		assert.False(t, due)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("chưa có bản nào thì sao lưu ngay", func(t *testing.T) {
		svc, mock := newSvc(t)
		expectRunning(mock, 0)
		expectLast(mock, nil)
		due, err := svc.autoBackupDue(now, time.Hour)
		require.NoError(t, err)
		assert.True(t, due)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("bản gần nhất mới 30 phút thì chưa tới hạn", func(t *testing.T) {
		svc, mock := newSvc(t)
		expectRunning(mock, 0)
		last := now.Add(-30 * time.Minute)
		expectLast(mock, &last)
		due, err := svc.autoBackupDue(now, time.Hour)
		require.NoError(t, err)
		assert.False(t, due)
	})
	t.Run("bản gần nhất đủ 1 giờ thì tới hạn", func(t *testing.T) {
		svc, mock := newSvc(t)
		expectRunning(mock, 0)
		last := now.Add(-time.Hour)
		expectLast(mock, &last)
		due, err := svc.autoBackupDue(now, time.Hour)
		require.NoError(t, err)
		assert.True(t, due)
	})
}
