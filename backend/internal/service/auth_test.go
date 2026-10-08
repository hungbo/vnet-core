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
	assert.Equal(t, "sai tên đăng nhập hoặc mật khẩu", err.Error())
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
	assert.Equal(t, "sai tên đăng nhập hoặc mật khẩu", err.Error())
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
	assert.Equal(t, "tài khoản đã bị khoá — liên hệ quản lý", err.Error())
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
	assert.Equal(t, "mã QR không hợp lệ hoặc hội viên đã bị khoá", err.Error())
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
	assert.Equal(t, "phiên đăng nhập đã hết hạn, hãy đăng nhập lại", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

// refreshRows dựng lại hai truy vấn của RefreshToken (tài khoản rồi vai trò) với mốc đổi mật khẩu cho trước.
func refreshRows(mock sqlmock.Sqlmock, changedAt interface{}) {
	mock.ExpectQuery(`SELECT \* FROM "users" WHERE \(id = \$1 AND is_active = \$2\) AND "users"\."deleted_at" IS NULL ORDER BY "users"\."id" LIMIT \$3`).
		WithArgs("u1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "full_name", "password_changed_at"}).AddRow("u1", "admin", "Admin User", changedAt))
	mock.ExpectQuery(`SELECT \* FROM "user_roles" WHERE "user_roles"."user_id" = \$1`).
		WithArgs("u1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "role_id"}))
}

// Đổi mật khẩu để đóng cửa với ai đã có token (vd đăng nhập bằng admin123 trước khi chủ quán đổi): đo được
// trên bản chưa có chốt này, refresh token cũ vẫn xin được access token mới sau khi đổi.
func TestAuthService_RefreshToken_TuChoiTokenCapTruocHoacCungGiayVoiLanDoiMatKhau(t *testing.T) {
	jwtMgr := newTestJWT()
	refreshToken, err := jwtMgr.GenerateRefreshToken("u1", jwt.KindStaff)
	require.NoError(t, err)
	claims, err := jwtMgr.ValidateToken(refreshToken)
	require.NoError(t, err)

	for ten, doiLuc := range map[string]time.Time{
		"đổi sau khi token được cấp": time.Now().Add(time.Minute),
		// IssuedAt chỉ chính xác tới giây: không phân biệt được trước/sau trong cùng giây, nên bỏ.
		"đổi trong cùng giây cấp": claims.IssuedAt.Time.Add(500 * time.Millisecond),
	} {
		db, mock := newMockDB(t)
		svc := NewAuthService(db, jwtMgr, NewAuditService(db))
		refreshRows(mock, doiLuc)

		_, err := svc.RefreshToken(&RefreshRequest{RefreshToken: refreshToken})

		require.Error(t, err, ten)
		assert.Equal(t, "phiên đăng nhập đã hết hạn, hãy đăng nhập lại", err.Error(), ten)
		assert.NoError(t, mock.ExpectationsWereMet(), ten)
	}
}

func TestAuthService_RefreshToken_TokenCapSauKhiDoiMatKhauVanDung(t *testing.T) {
	jwtMgr := newTestJWT()
	refreshToken, err := jwtMgr.GenerateRefreshToken("u1", jwt.KindStaff)
	require.NoError(t, err)
	claims, err := jwtMgr.ValidateToken(refreshToken)
	require.NoError(t, err)

	for ten, doiLuc := range map[string]time.Time{
		"đổi từ lâu":               time.Now().Add(-time.Hour),
		"đổi ở giây ngay trước đó": claims.IssuedAt.Time.Add(-500 * time.Millisecond),
	} {
		db, mock := newMockDB(t)
		svc := NewAuthService(db, jwtMgr, NewAuditService(db))
		refreshRows(mock, doiLuc)

		result, err := svc.RefreshToken(&RefreshRequest{RefreshToken: refreshToken})

		require.NoError(t, err, ten)
		assert.NotEmpty(t, result.AccessToken, ten)
		assert.NoError(t, mock.ExpectationsWereMet(), ten)
	}
}

func TestAuthService_ChangePassword_NhanVienGhiMocDoiMatKhau(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewAuthService(db, newTestJWT(), NewAuditService(db))
	oldHash, _ := utils.HashPassword("oldpass")

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1 AND "users"\."deleted_at" IS NULL ORDER BY "users"\."id" LIMIT \$2`).
		WithArgs("u1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "password_hash"}).AddRow("u1", oldHash))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET "password_changed_at"=\$1,"password_hash"=\$2,"updated_at"=\$3 WHERE`).
		WithArgs(anyTime{}, sqlmock.AnyArg(), anyTime{}, "u1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.ChangePassword("u1", false, &ChangePasswordRequest{OldPassword: "oldpass", NewPassword: "newpass123"})

	require.NoError(t, err)
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

// Đăng nhập bằng đúng mật khẩu mà cmd/seed đặt sẵn phải bật cờ buộc đổi mật khẩu,
// ở CẢ hai chỗ: ngoài cùng (script đọc) và trong user (giao diện gộp user vào
// state). Mật khẩu nào khác thì tắt.
func TestAuthService_Login_MatKhauMacDinhBatCoBuocDoi(t *testing.T) {
	for _, tc := range []struct {
		name, password string
		want           bool
	}{
		{"mật khẩu mặc định của seed", DefaultSeedPassword, true},
		{"mật khẩu tự đặt", "mat-khau-rieng-cua-quan", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := newMockDB(t)
			svc := NewAuthService(db, newTestJWT(), NewAuditService(db))
			hash, _ := utils.HashPassword(tc.password)

			mock.ExpectQuery(`SELECT \* FROM "users" WHERE username = \$1`).
				WithArgs("admin", 1).
				WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password_hash", "is_active"}).
					AddRow("u1", "admin", hash, true))
			mock.ExpectQuery(`SELECT \* FROM "user_roles"`).
				WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "role_id"}))

			result, err := svc.Login(&LoginRequest{Username: "admin", Password: tc.password})

			require.NoError(t, err)
			assert.Equal(t, tc.want, result.MustChangePassword)
			assert.Equal(t, tc.want, result.User.MustChangePassword)
		})
	}
}

// Sai tên hay sai mật khẩu đều phải là ErrBadCredentials: bộ chặn dò mật khẩu ở
// handler đếm đúng lỗi này và không đếm gì khác.
func TestAuthService_Login_SaiThongTinLaErrBadCredentials(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewAuthService(db, newTestJWT(), NewAuditService(db))
	hash, _ := utils.HashPassword("dung-mat-khau")

	mock.ExpectQuery(`SELECT \* FROM "users"`).WithArgs("khong-co", 1).WillReturnError(gorm.ErrRecordNotFound)
	_, err := svc.Login(&LoginRequest{Username: "khong-co", Password: "x"})
	assert.ErrorIs(t, err, ErrBadCredentials)

	mock.ExpectQuery(`SELECT \* FROM "users"`).WithArgs("admin", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password_hash", "is_active"}).
			AddRow("u1", "admin", hash, true))
	mock.ExpectQuery(`SELECT \* FROM "user_roles"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "role_id"}))
	_, err = svc.Login(&LoginRequest{Username: "admin", Password: "sai"})
	assert.ErrorIs(t, err, ErrBadCredentials)

	// Hội viên cũng vậy: ClientLogin đi qua đây khi tên không có trong users.
	mock.ExpectQuery(`SELECT \* FROM "members"`).WillReturnError(gorm.ErrRecordNotFound)
	_, err = svc.MemberLogin(&MemberLoginRequest{Username: "khong-co", Password: "x"})
	assert.ErrorIs(t, err, ErrBadCredentials)
}

// Tải lại trang chỉ còn token, không còn mật khẩu vừa gõ — nên /auth/me phải tự
// biết tài khoản còn dùng mật khẩu mặc định, bằng cách so băm. Thiếu nhánh này
// thì F5 là hộp thoại buộc đổi mật khẩu biến mất.
func TestAuthService_GetCurrentUser_BaoMatKhauMacDinhTheoBam(t *testing.T) {
	for _, tc := range []struct {
		name, password string
		want           bool
	}{
		{"còn mật khẩu mặc định", DefaultSeedPassword, true},
		{"đã đổi mật khẩu", "mat-khau-moi-123", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := newMockDB(t)
			svc := NewAuthService(db, newTestJWT(), NewAuditService(db))
			hash, _ := utils.HashPassword(tc.password)

			mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).
				WithArgs("u1", 1).
				WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password_hash"}).AddRow("u1", "admin", hash))
			mock.ExpectQuery(`SELECT \* FROM "user_roles"`).
				WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "role_id"}))

			result, err := svc.GetCurrentUser("u1")

			require.NoError(t, err)
			assert.Equal(t, tc.want, result.MustChangePassword)
		})
	}
}

// Mật khẩu nhân viên chặt hơn mã PIN của hội viên: từ 8 ký tự, và không được là
// chính mật khẩu mặc định — nếu không, hộp thoại buộc đổi mật khẩu đổi "admin123"
// thành "admin123" là xong. Hội viên vẫn chỉ cần 6 (xem
// ChangePassword_MemberUsesMembersTable, mật khẩu mới 7 ký tự).
func TestAuthService_ChangePassword_NhanVienPhaiTu8KyTuVaKhacMacDinh(t *testing.T) {
	for _, tc := range []struct {
		name, newPassword, wantErr string
	}{
		{"7 ký tự", "abcdefg", "ít nhất 8 ký tự"},
		{"đúng mật khẩu mặc định", DefaultSeedPassword, "mật khẩu mặc định"},
		{"8 ký tự khác mặc định", "abcdefgh", ""},
		{"7 chữ có dấu dài hơn 8 byte vẫn là 7 ký tự", "mậtkhẩu", "ít nhất 8 ký tự"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := newMockDB(t)
			svc := NewAuthService(db, newTestJWT(), NewAuditService(db))
			oldHash, _ := utils.HashPassword(DefaultSeedPassword)

			mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).
				WithArgs("u1", 1).
				WillReturnRows(sqlmock.NewRows([]string{"id", "password_hash"}).AddRow("u1", oldHash))
			if tc.wantErr == "" {
				mock.ExpectBegin()
				mock.ExpectExec(`UPDATE "users" SET`).WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			}

			err := svc.ChangePassword("u1", false, &ChangePasswordRequest{
				OldPassword: DefaultSeedPassword,
				NewPassword: tc.newPassword,
			})

			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
