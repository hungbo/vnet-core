package service

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/pkg/jwt"
	"github.com/vnet/core/pkg/utils"
	"gorm.io/gorm"
)

func newTestJWT() *jwt.Manager {
	return jwt.New("test-secret", 1*time.Hour, 7*24*time.Hour, "vnet-test")
}

func TestAuthService_Login_Success(t *testing.T) {
	db, mock := newMockDB(t)
	jwtMgr := newTestJWT()
	svc := NewAuthService(db, jwtMgr, NewAuditService(db))

	hash, _ := utils.HashPassword("password123")

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE username = \$1 AND "users"\."deleted_at" IS NULL ORDER BY "users"\."id" LIMIT \$2`).
		WithArgs("admin", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password_hash", "is_active", "full_name"}).
			AddRow("u1", "admin", hash, true, "Admin User"))

	mock.ExpectQuery(`SELECT \* FROM "user_roles" WHERE "user_roles"."user_id" = \$1`).
		WithArgs("u1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "role_id"}))

	result, err := svc.Login(&LoginRequest{Username: "admin", Password: "password123"})
	require.NoError(t, err)
	assert.NotEmpty(t, result.AccessToken)
	assert.NotEmpty(t, result.RefreshToken)
	assert.Equal(t, "admin", result.User.Username)
	assert.Equal(t, "Admin User", result.User.FullName)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Một ô đăng nhập cho cả hai loại tài khoản: TÊN quyết định đường đi, không
// phải người dùng chọn. Tên có trong bảng users thì đi đường nhân viên và
// KHÔNG mở phiên — nhân viên vào máy để trông, không phải để chơi.
func TestAuthService_ClientLogin_TenNhanVienThiDiDuongNhanVien(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewAuthService(db, newTestJWT(), NewAuditService(db))

	hash, _ := utils.HashPassword("password123")
	dongUser := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"id", "username", "password_hash", "is_active", "full_name"}).
			AddRow("u1", "quanly", hash, true, "Quản lý")
	}

	// Lượt tra đầu chỉ để biết tên này thuộc bảng nào.
	mock.ExpectQuery(`SELECT \* FROM "users" WHERE username = \$1`).
		WithArgs("quanly", 1).WillReturnRows(dongUser())
	// Rồi mới gọi đúng luồng đăng nhập nhân viên có sẵn.
	mock.ExpectQuery(`SELECT \* FROM "users" WHERE username = \$1`).
		WithArgs("quanly", 1).WillReturnRows(dongUser())
	mock.ExpectQuery(`SELECT \* FROM "user_roles" WHERE "user_roles"."user_id" = \$1`).
		WithArgs("u1").WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "role_id"}))

	res, err := svc.ClientLogin(&MemberLoginRequest{
		Username: "quanly", Password: "password123", MachineCode: "PC-01",
	})
	require.NoError(t, err)
	assert.Equal(t, "staff", res.Kind)
	assert.Equal(t, "quanly", res.User.Username)
	assert.Empty(t, res.SessionID, "nhân viên vào máy thì không được mở phiên tính tiền")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Sai mật khẩu của một tài khoản nhân viên KHÔNG được rơi xuống đường hội viên.
// Rơi xuống thì cùng một cái tên sẽ thành người này hay người kia tuỳ mật khẩu
// gõ vào — và bảng members có thể có một tài khoản trùng tên.
func TestAuthService_ClientLogin_SaiMatKhauNhanVienThiDungLai(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewAuthService(db, newTestJWT(), NewAuditService(db))

	hash, _ := utils.HashPassword("password123")
	dongUser := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"id", "username", "password_hash", "is_active", "full_name"}).
			AddRow("u1", "quanly", hash, true, "Quản lý")
	}
	mock.ExpectQuery(`SELECT \* FROM "users" WHERE username = \$1`).
		WithArgs("quanly", 1).WillReturnRows(dongUser())
	mock.ExpectQuery(`SELECT \* FROM "users" WHERE username = \$1`).
		WithArgs("quanly", 1).WillReturnRows(dongUser())

	_, err := svc.ClientLogin(&MemberLoginRequest{Username: "quanly", Password: "sai-mat-khau"})
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet(), "không được đi tiếp sang tra bảng members")
}

// Tên không có trong bảng users thì tra sang hội viên.
func TestAuthService_ClientLogin_TenLaThiDiDuongHoiVien(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewAuthService(db, newTestJWT(), NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE username = \$1`).
		WithArgs("khach", 1).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`SELECT \* FROM "members" WHERE \(username = \$1 AND is_active = \$2\)`).
		WithArgs("khach", true, 1).WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, err := svc.ClientLogin(&MemberLoginRequest{Username: "khach", Password: "bat-ky"})
	assert.Error(t, err, "không có ở cả hai bảng thì phải báo sai tài khoản")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthService_Login_InvalidPassword(t *testing.T) {
	db, mock := newMockDB(t)
	jwtMgr := newTestJWT()
	svc := NewAuthService(db, jwtMgr, NewAuditService(db))

	hash, _ := utils.HashPassword("correctpassword")

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE username = \$1 AND "users"\."deleted_at" IS NULL ORDER BY "users"\."id" LIMIT \$2`).
		WithArgs("admin", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password_hash", "is_active"}).
			AddRow("u1", "admin", hash, true))

	mock.ExpectQuery(`SELECT \* FROM "user_roles" WHERE "user_roles"."user_id" = \$1`).
		WithArgs("u1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "role_id"}))

	_, err := svc.Login(&LoginRequest{Username: "admin", Password: "wrongpassword"})
	assert.Error(t, err)
	assert.Equal(t, "invalid username or password", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthService_Login_UserNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	jwtMgr := newTestJWT()
	svc := NewAuthService(db, jwtMgr, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE username = \$1 AND "users"\."deleted_at" IS NULL ORDER BY "users"\."id" LIMIT \$2`).
		WithArgs("unknown", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.Login(&LoginRequest{Username: "unknown", Password: "pw"})
	assert.Error(t, err)
	assert.Equal(t, "invalid username or password", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthService_Login_DisabledAccount(t *testing.T) {
	db, mock := newMockDB(t)
	jwtMgr := newTestJWT()
	svc := NewAuthService(db, jwtMgr, NewAuditService(db))

	hash, _ := utils.HashPassword("password123")

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE username = \$1 AND "users"\."deleted_at" IS NULL ORDER BY "users"\."id" LIMIT \$2`).
		WithArgs("disabled", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password_hash", "is_active"}).
			AddRow("u1", "disabled", hash, false))

	mock.ExpectQuery(`SELECT \* FROM "user_roles" WHERE "user_roles"."user_id" = \$1`).
		WithArgs("u1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "role_id"}))

	_, err := svc.Login(&LoginRequest{Username: "disabled", Password: "password123"})
	assert.Error(t, err)
	assert.Equal(t, "account is disabled", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthService_GetCurrentUser_Success(t *testing.T) {
	db, mock := newMockDB(t)
	jwtMgr := newTestJWT()
	svc := NewAuthService(db, jwtMgr, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1 AND "users"\."deleted_at" IS NULL ORDER BY "users"\."id" LIMIT \$2`).
		WithArgs("u1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "full_name"}).AddRow("u1", "admin", "Admin User"))

	mock.ExpectQuery(`SELECT \* FROM "user_roles" WHERE "user_roles"."user_id" = \$1`).
		WithArgs("u1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "role_id"}))

	result, err := svc.GetCurrentUser("u1")
	require.NoError(t, err)
	assert.Equal(t, "admin", result.Username)
	assert.Equal(t, "Admin User", result.FullName)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthService_ChangePassword_Success(t *testing.T) {
	db, mock := newMockDB(t)
	jwtMgr := newTestJWT()
	svc := NewAuthService(db, jwtMgr, NewAuditService(db))

	oldHash, _ := utils.HashPassword("oldpass")

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1 AND "users"\."deleted_at" IS NULL ORDER BY "users"\."id" LIMIT \$2`).
		WithArgs("u1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "password_hash"}).AddRow("u1", oldHash))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.ChangePassword("u1", false, &ChangePasswordRequest{
		OldPassword: "oldpass",
		NewPassword: "newpass123",
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthService_ChangePassword_WrongOldPassword(t *testing.T) {
	db, mock := newMockDB(t)
	jwtMgr := newTestJWT()
	svc := NewAuthService(db, jwtMgr, NewAuditService(db))

	oldHash, _ := utils.HashPassword("actualoldpass")

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1 AND "users"\."deleted_at" IS NULL ORDER BY "users"\."id" LIMIT \$2`).
		WithArgs("u1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "password_hash"}).AddRow("u1", oldHash))

	err := svc.ChangePassword("u1", false, &ChangePasswordRequest{
		OldPassword: "wrongold",
		NewPassword: "newpass123",
	})
	assert.Error(t, err)
	assert.Equal(t, "mật khẩu hiện tại không đúng", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthService_GetPermissions(t *testing.T) {
	db, mock := newMockDB(t)
	jwtMgr := newTestJWT()
	svc := NewAuthService(db, jwtMgr, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "permissions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name"}).
			AddRow("p1", "order.create", "Create Order").
			AddRow("p2", "order.delete", "Delete Order"))

	permissions, err := svc.GetPermissions()
	require.NoError(t, err)
	assert.Len(t, permissions, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthService_QRLogin_Success(t *testing.T) {
	db, mock := newMockDB(t)
	jwtMgr := newTestJWT()
	svc := NewAuthService(db, jwtMgr, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "members" WHERE \(id = \$1 AND is_active = \$2\) AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$3`).
		WithArgs("m1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "full_name", "is_active"}).
			AddRow("m1", "member001", "Test Member", true))

	result, err := svc.QRLogin(&QRLoginRequest{QRCode: "m1"})
	require.NoError(t, err)
	assert.NotEmpty(t, result.AccessToken)
	assert.NotEmpty(t, result.RefreshToken)
	assert.Equal(t, "member", result.User.Role)
	assert.Equal(t, "member001", result.User.Username)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthService_QRLogin_MemberNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	jwtMgr := newTestJWT()
	svc := NewAuthService(db, jwtMgr, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "members" WHERE \(id = \$1 AND is_active = \$2\) AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$3`).
		WithArgs("nonexistent", true, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.QRLogin(&QRLoginRequest{QRCode: "nonexistent"})
	assert.Error(t, err)
	assert.Equal(t, "invalid or inactive member QR code", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthService_RefreshToken_Success(t *testing.T) {
	db, mock := newMockDB(t)
	jwtMgr := newTestJWT()
	svc := NewAuthService(db, jwtMgr, NewAuditService(db))

	refreshToken, err := jwtMgr.GenerateRefreshToken("u1", jwt.KindStaff)
	require.NoError(t, err)

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE \(id = \$1 AND is_active = \$2\) AND "users"\."deleted_at" IS NULL ORDER BY "users"\."id" LIMIT \$3`).
		WithArgs("u1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "full_name"}).AddRow("u1", "admin", "Admin User"))

	mock.ExpectQuery(`SELECT \* FROM "user_roles" WHERE "user_roles"."user_id" = \$1`).
		WithArgs("u1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "role_id"}))

	result, err := svc.RefreshToken(&RefreshRequest{RefreshToken: refreshToken})
	require.NoError(t, err)
	assert.NotEmpty(t, result.AccessToken)
	assert.Equal(t, "admin", result.User.Username)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthService_RefreshToken_Invalid(t *testing.T) {
	db, mock := newMockDB(t)
	jwtMgr := newTestJWT()
	svc := NewAuthService(db, jwtMgr, NewAuditService(db))

	_, err := svc.RefreshToken(&RefreshRequest{RefreshToken: "invalid-token"})
	assert.Error(t, err)
	assert.Equal(t, "invalid or expired refresh token", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

// A member's credentials live in members, not users. Looking them up in users
// could only ever fail, so changing a PIN from the client always errored.
func TestAuthService_ChangePassword_MemberUsesMembersTable(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewAuthService(db, newTestJWT(), NewAuditService(db))

	hash, err := utils.HashPassword("oldpass")
	require.NoError(t, err)

	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 AND "members"\."deleted_at" IS NULL ORDER BY "members"\."id" LIMIT \$2`).
		WithArgs("mem-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "password_hash"}).AddRow("mem-1", hash))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "members" SET`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = svc.ChangePassword("mem-1", true, &ChangePasswordRequest{
		OldPassword: "oldpass",
		NewPassword: "newpass",
	})

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthService_ChangePassword_MemberRejectsWrongCurrent(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewAuthService(db, newTestJWT(), NewAuditService(db))

	hash, err := utils.HashPassword("oldpass")
	require.NoError(t, err)

	mock.ExpectQuery(`SELECT \* FROM "members"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "password_hash"}).AddRow("mem-1", hash))

	err = svc.ChangePassword("mem-1", true, &ChangePasswordRequest{
		OldPassword: "wrong",
		NewPassword: "newpass",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "mật khẩu hiện tại không đúng")
}
