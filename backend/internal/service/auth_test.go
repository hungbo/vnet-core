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

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE username = \$1 ORDER BY "users"."id" LIMIT \$2`).
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

func TestAuthService_Login_InvalidPassword(t *testing.T) {
	db, mock := newMockDB(t)
	jwtMgr := newTestJWT()
	svc := NewAuthService(db, jwtMgr, NewAuditService(db))

	hash, _ := utils.HashPassword("correctpassword")

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE username = \$1 ORDER BY "users"."id" LIMIT \$2`).
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

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE username = \$1 ORDER BY "users"."id" LIMIT \$2`).
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

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE username = \$1 ORDER BY "users"."id" LIMIT \$2`).
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

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1 ORDER BY "users"."id" LIMIT \$2`).
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

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1 ORDER BY "users"."id" LIMIT \$2`).
		WithArgs("u1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "password_hash"}).AddRow("u1", oldHash))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.ChangePassword("u1", &ChangePasswordRequest{
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

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1 ORDER BY "users"."id" LIMIT \$2`).
		WithArgs("u1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "password_hash"}).AddRow("u1", oldHash))

	err := svc.ChangePassword("u1", &ChangePasswordRequest{
		OldPassword: "wrongold",
		NewPassword: "newpass123",
	})
	assert.Error(t, err)
	assert.Equal(t, "current password is incorrect", err.Error())
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

	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 AND is_active = \$2 ORDER BY "members"."id" LIMIT \$3`).
		WithArgs("m1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "full_name", "is_active"}).
			AddRow("m1", "MEMBER-001", "Test Member", true))

	result, err := svc.QRLogin(&QRLoginRequest{QRCode: "m1"})
	require.NoError(t, err)
	assert.NotEmpty(t, result.AccessToken)
	assert.Equal(t, "member", result.User.Role)
	assert.Equal(t, "MEMBER-001", result.User.Username)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthService_QRLogin_MemberNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	jwtMgr := newTestJWT()
	svc := NewAuthService(db, jwtMgr, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "members" WHERE id = \$1 AND is_active = \$2 ORDER BY "members"."id" LIMIT \$3`).
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

	refreshToken, err := jwtMgr.GenerateRefreshToken("u1")
	require.NoError(t, err)

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1 AND is_active = \$2 ORDER BY "users"."id" LIMIT \$3`).
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
