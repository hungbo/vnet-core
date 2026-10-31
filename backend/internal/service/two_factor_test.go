package service

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/pkg/utils"
)

const testTOTPSecret = "JBSWY3DPEHPK3PXP"

func maTaiLuc(t *testing.T, at time.Time) string {
	t.Helper()
	code, err := totp.GenerateCodeCustom(testTOTPSecret, at, totpOpts)
	require.NoError(t, err)
	return code
}

// Đồng hồ điện thoại lệch một bước (30 giây) vẫn nhận; lệch hơn thì không.
func TestKiemMaTOTP_LechMotBuoc(t *testing.T) {
	now := time.Unix(1_793_000_000, 0)
	step := now.Unix() / 30

	assert.Equal(t, step, kiemMaTOTP(testTOTPSecret, maTaiLuc(t, now), now))
	assert.Equal(t, step-1, kiemMaTOTP(testTOTPSecret, maTaiLuc(t, now.Add(-30*time.Second)), now))
	assert.Equal(t, step+1, kiemMaTOTP(testTOTPSecret, " "+maTaiLuc(t, now.Add(30*time.Second))+" ", now))
	assert.Equal(t, int64(-1), kiemMaTOTP(testTOTPSecret, maTaiLuc(t, now.Add(-90*time.Second)), now))
	assert.Equal(t, int64(-1), kiemMaTOTP(testTOTPSecret, "12345", now))
	assert.Equal(t, int64(-1), kiemMaTOTP("", maTaiLuc(t, now), now))
}

func TestMaDuPhong_GoKieuNaoCungKhop(t *testing.T) {
	codes, hashes, err := taoMaDuPhong()
	require.NoError(t, err)
	require.Len(t, codes, twoFactorRecoveryCount)
	assert.Equal(t, twoFactorRecoveryCount, demMaDuPhong(hashes))
	assert.Regexp(t, `^[a-z2-7]{4}-[a-z2-7]{4}$`, codes[0])
	assert.Equal(t, bamMaDuPhong("abcd-efgh"), bamMaDuPhong(" ABCD EFGH "))
	assert.Contains(t, hashes, bamMaDuPhong(codes[3]))
}

func dongNhanVienHaiBuoc(hash string) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "username", "password_hash", "is_active", "totp_secret", "totp_enabled_at", "totp_last_step"}).
		AddRow("u1", "quanly", hash, true, testTOTPSecret, time.Now(), 0)
}

func expectTinhNangHaiBuoc(mock sqlmock.Sqlmock, bat bool) {
	rows := sqlmock.NewRows([]string{"group_name", "key", "value"})
	if bat {
		rows.AddRow(FeaturesGroup, TwoFactorFeatureKey, `"true"`)
	}
	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1`).WithArgs(FeaturesGroup).WillReturnRows(rows)
}

// Mật khẩu đúng mà tài khoản bật 2 bước: chỉ có vé, CHƯA có token nào.
func TestAuthService_Login_HaiBuocChiTraVe(t *testing.T) {
	db, mock := newMockDB(t)
	jwtMgr := newTestJWT()
	svc := NewAuthService(db, jwtMgr, nil)
	hash, _ := utils.HashPassword("password123")

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE username = \$1`).WithArgs("quanly", 1).WillReturnRows(dongNhanVienHaiBuoc(hash))
	mock.ExpectQuery(`SELECT \* FROM "user_roles"`).WithArgs("u1").WillReturnRows(sqlmock.NewRows([]string{"user_id", "role_id"}))
	expectTinhNangHaiBuoc(mock, true)

	res, err := svc.Login(&LoginRequest{Username: "quanly", Password: "password123"})
	require.NoError(t, err)
	assert.True(t, res.NeedTOTP)
	assert.Empty(t, res.AccessToken)
	assert.Empty(t, res.RefreshToken)
	assert.Nil(t, res.User)

	claims, err := jwtMgr.ValidateToken(res.Challenge)
	require.NoError(t, err)
	assert.Equal(t, "2fa", claims.TokenType)
	assert.Equal(t, "u1", claims.UserID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Chủ quán tắt tính năng: khoá đã cài vẫn giữ nhưng không ai bị hỏi mã.
func TestAuthService_Login_TatTinhNangThiKhongHoiMa(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewAuthService(db, newTestJWT(), nil)
	hash, _ := utils.HashPassword("password123")

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE username = \$1`).WithArgs("quanly", 1).WillReturnRows(dongNhanVienHaiBuoc(hash))
	mock.ExpectQuery(`SELECT \* FROM "user_roles"`).WithArgs("u1").WillReturnRows(sqlmock.NewRows([]string{"user_id", "role_id"}))
	expectTinhNangHaiBuoc(mock, false)

	res, err := svc.Login(&LoginRequest{Username: "quanly", Password: "password123"})
	require.NoError(t, err)
	assert.False(t, res.NeedTOTP)
	assert.NotEmpty(t, res.AccessToken)
}

// Máy trạm không có bước nhập mã: tài khoản bật 2 bước bị chặn ở đó, không thì
// token máy trạm là lối vòng qua 2 bước vào API quản trị.
func TestAuthService_ClientLogin_NhanVienHaiBuocBiChan(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewAuthService(db, newTestJWT(), nil)
	hash, _ := utils.HashPassword("password123")

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE username = \$1`).WithArgs("quanly", 1).WillReturnRows(dongNhanVienHaiBuoc(hash))
	mock.ExpectQuery(`SELECT \* FROM "users" WHERE username = \$1`).WithArgs("quanly", 1).WillReturnRows(dongNhanVienHaiBuoc(hash))
	mock.ExpectQuery(`SELECT \* FROM "user_roles"`).WithArgs("u1").WillReturnRows(sqlmock.NewRows([]string{"user_id", "role_id"}))
	expectTinhNangHaiBuoc(mock, true)

	_, err := svc.ClientLogin(&MemberLoginRequest{Username: "quanly", Password: "password123", MachineCode: "PC-01"})
	assert.ErrorIs(t, err, ErrTwoFactorWebOnly)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func expectBuocHaiDocNhanVien(mock sqlmock.Sqlmock, hash string) {
	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).WithArgs("u1", 1).WillReturnRows(dongNhanVienHaiBuoc(hash))
	mock.ExpectQuery(`SELECT \* FROM "user_roles"`).WithArgs("u1").WillReturnRows(sqlmock.NewRows([]string{"user_id", "role_id"}))
	expectTinhNangHaiBuoc(mock, true)
}

func TestAuthService_LoginTOTP_MaDungThiCapPhien(t *testing.T) {
	db, mock := newMockDB(t)
	jwtMgr := newTestJWT()
	svc := NewAuthService(db, jwtMgr, nil)
	hash, _ := utils.HashPassword("password123")
	challenge, err := jwtMgr.GenerateTwoFactorToken("u1", time.Minute)
	require.NoError(t, err)

	expectBuocHaiDocNhanVien(mock, hash)
	// Ghi bước của mã vừa dùng, chỉ khi nó mới hơn bước đã ghi.
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET "totp_last_step"=\$1 WHERE \(id = \$2 AND totp_last_step < \$3\)`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	res, err := svc.LoginTOTP(&LoginTOTPRequest{Challenge: challenge, Code: maTaiLuc(t, time.Now())})
	require.NoError(t, err)
	assert.NotEmpty(t, res.AccessToken)
	assert.NotEmpty(t, res.RefreshToken)
	assert.Equal(t, "quanly", res.User.Username)
}

// Cùng một mã gửi lại (bước đã dùng): UPDATE không trúng dòng nào → coi như sai.
func TestAuthService_LoginTOTP_MaDaDungThiTuChoi(t *testing.T) {
	db, mock := newMockDB(t)
	jwtMgr := newTestJWT()
	svc := NewAuthService(db, jwtMgr, nil)
	hash, _ := utils.HashPassword("password123")
	challenge, _ := jwtMgr.GenerateTwoFactorToken("u1", time.Minute)

	expectBuocHaiDocNhanVien(mock, hash)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET "totp_last_step"`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	mock.ExpectQuery(`UPDATE users SET failed_logins = failed_logins \+ 1 WHERE id = \$1 RETURNING failed_logins`).
		WithArgs("u1").WillReturnRows(sqlmock.NewRows([]string{"failed_logins"}).AddRow(1))

	_, err := svc.LoginTOTP(&LoginTOTPRequest{Challenge: challenge, Code: maTaiLuc(t, time.Now())})
	assert.ErrorIs(t, err, ErrBadTOTP)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Mã dự phòng dùng được đúng một lần: dùng xong thì bị gạch khỏi danh sách.
func TestAuthService_LoginTOTP_MaDuPhong(t *testing.T) {
	db, mock := newMockDB(t)
	jwtMgr := newTestJWT()
	svc := NewAuthService(db, jwtMgr, nil)
	hash, _ := utils.HashPassword("password123")
	challenge, _ := jwtMgr.GenerateTwoFactorToken("u1", time.Minute)
	stored := bamMaDuPhong("aaaa-bbbb") + "," + bamMaDuPhong("cccc-dddd")

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).WithArgs("u1", 1).WillReturnRows(
		sqlmock.NewRows([]string{"id", "username", "password_hash", "is_active", "totp_secret", "totp_enabled_at", "totp_recovery"}).
			AddRow("u1", "quanly", hash, true, testTOTPSecret, time.Now(), stored))
	mock.ExpectQuery(`SELECT \* FROM "user_roles"`).WithArgs("u1").WillReturnRows(sqlmock.NewRows([]string{"user_id", "role_id"}))
	expectTinhNangHaiBuoc(mock, true)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET "totp_recovery"=\$1 WHERE \(id = \$2 AND totp_recovery = \$3\)`).
		WithArgs(bamMaDuPhong("aaaa-bbbb"), "u1", stored).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	res, err := svc.LoginTOTP(&LoginTOTPRequest{Challenge: challenge, Code: "CCCC DDDD"})
	require.NoError(t, err)
	assert.NotEmpty(t, res.AccessToken)
}

// Vé hết hạn hay vé không phải vé 2 bước (vd. refresh token) đều không qua.
func TestAuthService_LoginTOTP_VeKhongHopLe(t *testing.T) {
	db, _ := newMockDB(t)
	jwtMgr := newTestJWT()
	svc := NewAuthService(db, jwtMgr, nil)

	expired, _ := jwtMgr.GenerateTwoFactorToken("u1", -time.Minute)
	refresh, _ := jwtMgr.GenerateRefreshToken("u1", "staff")
	for _, ve := range []string{expired, refresh, "rác"} {
		_, err := svc.LoginTOTP(&LoginTOTPRequest{Challenge: ve, Code: "123456"})
		assert.ErrorIs(t, err, errTwoFactorExpired)
	}
}
