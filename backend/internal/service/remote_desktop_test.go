package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/hub"
)

// Máy đã có mật khẩu thì dùng lại, không sinh mới: đổi mật khẩu mỗi nhịp tim
// là TightVNC trên máy trạm lệch với mật khẩu trình xem gửi lên.
func TestMachineService_VNCPassword_DungLaiMatKhauCu(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, hub.New(nil), NewAuditService(db))

	mock.ExpectQuery(`SELECT id, vnc_password FROM "machines" WHERE id = \$1`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "vnc_password"}).AddRow("m1", "abcd1234"))

	pw, err := svc.VNCPassword("m1")
	require.NoError(t, err)
	assert.Equal(t, "abcd1234", pw)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_VNCPassword_SinhMoiDung8KyTu(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, hub.New(nil), NewAuditService(db))

	mock.ExpectQuery(`SELECT id, vnc_password FROM "machines" WHERE id = \$1`).
		WithArgs("m1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "vnc_password"}).AddRow("m1", ""))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "machines" SET "vnc_password"=\$1,"updated_at"=\$2 WHERE \(id = \$3 AND \(vnc_password IS NULL OR vnc_password = ''\)\) AND "machines"."deleted_at" IS NULL`).
		WithArgs(sqlmock.AnyArg(), anyTime{}, "m1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	pw, err := svc.VNCPassword("m1")
	require.NoError(t, err)
	assert.Len(t, pw, 8)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMachineService_RemoteDesktopTarget(t *testing.T) {
	cases := []struct {
		name, status, ip string
		wantErr          error
	}{
		{"máy tắt", "offline", "192.168.1.10", ErrMachineOffline},
		{"chưa có IP", "available", "", ErrNoMachineIP},
		{"được", "in_use", "192.168.1.10", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := newMockDB(t)
			svc := NewMachineService(db, hub.New(nil), NewAuditService(db))
			mock.ExpectQuery(`SELECT id, machine_code, status, ip_address FROM "machines" WHERE id = \$1`).
				WithArgs("m1", 1).
				WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code", "status", "ip_address"}).
					AddRow("m1", "M-001", tc.status, tc.ip))

			m, err := svc.RemoteDesktopTarget("m1")
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "192.168.1.10", m.IPAddress)
		})
	}
}
