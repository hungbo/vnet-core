package service

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/pkg/utils"
)

func expectDemSai(mock sqlmock.Sqlmock, bang string, n int) {
	mock.ExpectQuery(`UPDATE ` + bang + ` SET failed_logins = failed_logins \+ 1 WHERE id = \$1 RETURNING failed_logins`).
		WithArgs("id1").
		WillReturnRows(sqlmock.NewRows([]string{"failed_logins"}).AddRow(n))
}

// Lần sai thứ 1 và 2 chỉ đếm; lần thứ 3 khoá 1 phút, thứ 4 khoá 5 phút.
func TestKhoaDangNhap_KhoaTamTheoBac(t *testing.T) {
	now := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		n    int
		khoa time.Duration
	}{{1, 0}, {2, 0}, {3, time.Minute}, {4, 5 * time.Minute}}
	for _, c := range cases {
		db, mock := newMockDB(t)
		k := khoaDangNhap{bang: "members", id: "id1", isActive: true}
		expectDemSai(mock, "members", c.n)
		if c.khoa > 0 {
			mock.ExpectBegin()
			mock.ExpectExec(`UPDATE "members" SET "locked_until"=\$1 WHERE id = \$2`).
				WithArgs(now.Add(c.khoa), "id1").
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
		}

		err := k.sauKhiSai(db, now)

		if c.khoa == 0 {
			assert.ErrorIs(t, err, ErrBadCredentials, "lần sai %d", c.n)
		} else {
			var l *LoginLockedError
			require.ErrorAs(t, err, &l, "lần sai %d", c.n)
			assert.False(t, l.Permanent)
			assert.Equal(t, now.Add(c.khoa), l.Until)
		}
		assert.NoError(t, mock.ExpectationsWereMet(), "lần sai %d", c.n)
	}
}

// Lần sai thứ 5 khoá hẳn: tắt tài khoản, quản lý phải kích hoạt lại.
func TestKhoaDangNhap_LanThuNamKhoaHan(t *testing.T) {
	db, mock := newMockDB(t)
	k := khoaDangNhap{bang: "users", id: "id1", isActive: true}
	expectDemSai(mock, "users", 5)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET "is_active"=\$1,"locked_until"=\$2 WHERE id = \$3`).
		WithArgs(false, nil, "id1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := k.sauKhiSai(db, time.Now())

	var l *LoginLockedError
	require.ErrorAs(t, err, &l)
	assert.True(t, l.Permanent)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Chủ quán (siêu quyền "*") không bao giờ bị khoá hẳn — chỉ khoá tạm 5 phút.
func TestKhoaDangNhap_SieuQuyenKhongKhoaHan(t *testing.T) {
	now := time.Now()
	db, mock := newMockDB(t)
	k := khoaDangNhap{bang: "users", id: "id1", isActive: true, sieuQuyen: true}
	expectDemSai(mock, "users", 7)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET "locked_until"=\$1 WHERE id = \$2`).
		WithArgs(now.Add(5*time.Minute), "id1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := k.sauKhiSai(db, now)

	var l *LoginLockedError
	require.ErrorAs(t, err, &l)
	assert.False(t, l.Permanent)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Đang khoá tạm thì từ chối luôn, kể cả khi mật khẩu đúng, và không đếm thêm.
// Bị tắt sau 5 lần sai thì báo khoá hẳn. Quản lý kích hoạt lại thì bộ đếm về 0.
func TestKhoaDangNhap_TruocKhiKiem(t *testing.T) {
	now := time.Now()
	sau := now.Add(30 * time.Second)

	db, mock := newMockDB(t)
	err := khoaDangNhap{bang: "members", id: "id1", isActive: true, failed: 3, lockedUntil: &sau}.truocKhiKiem(db, now)
	var l *LoginLockedError
	require.ErrorAs(t, err, &l)
	assert.Contains(t, err.Error(), "30 giây")

	err = khoaDangNhap{bang: "members", id: "id1", isActive: false, failed: 5}.truocKhiKiem(db, now)
	require.ErrorAs(t, err, &l)
	assert.True(t, l.Permanent)

	// Tắt vì lý do khác (không phải do gõ sai): để người gọi báo như cũ.
	assert.NoError(t, khoaDangNhap{bang: "members", id: "id1", isActive: false}.truocKhiKiem(db, now))

	truoc := now.Add(-time.Second)
	assert.NoError(t, khoaDangNhap{bang: "members", id: "id1", isActive: true, failed: 3, lockedUntil: &truoc}.truocKhiKiem(db, now),
		"hết hạn khoá tạm thì cho thử lại")
	assert.NoError(t, mock.ExpectationsWereMet(), "các trường hợp trên không được ghi gì")

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "members" SET "failed_logins"=\$1,"locked_until"=\$2 WHERE id = \$3`).
		WithArgs(0, nil, "id1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	assert.NoError(t, khoaDangNhap{bang: "members", id: "id1", isActive: true, failed: 5}.truocKhiKiem(db, now))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Đi trọn đường MemberLogin: tài khoản đang khoá tạm thì mật khẩu đúng cũng bị
// từ chối, không so mật khẩu và không đếm thêm.
func TestAuthService_MemberLogin_DangKhoaTam(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewAuthService(db, newTestJWT(), NewAuditService(db))
	hash, _ := utils.HashPassword("dung-mat-khau")
	sau := time.Now().Add(4 * time.Minute)

	mock.ExpectQuery(`SELECT \* FROM "members" WHERE username = \$1`).
		WithArgs("khach", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password_hash", "is_active", "failed_logins", "locked_until"}).
			AddRow("id1", "khach", hash, true, 4, sau))

	_, err := svc.MemberLogin(&MemberLoginRequest{Username: "khach", Password: "dung-mat-khau"})

	assert.True(t, IsLoginLocked(err))
	assert.Contains(t, err.Error(), "phút")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Nhân viên gõ sai lần thứ 3: thông báo nói rõ bị khoá 1 phút.
func TestAuthService_Login_SaiLanThuBaKhoaMotPhut(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewAuthService(db, newTestJWT(), NewAuditService(db))
	hash, _ := utils.HashPassword("dung-mat-khau")

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE username = \$1`).
		WithArgs("quay1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password_hash", "is_active", "failed_logins"}).
			AddRow("id1", "quay1", hash, true, 2))
	mock.ExpectQuery(`SELECT \* FROM "user_roles"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "role_id"}))
	expectDemSai(mock, "users", 3)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET "locked_until"=\$1 WHERE id = \$2`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	_, err := svc.Login(&LoginRequest{Username: "quay1", Password: "sai"})

	assert.True(t, IsLoginLocked(err))
	assert.Contains(t, err.Error(), "1 phút")
	assert.NoError(t, mock.ExpectationsWereMet())
}
