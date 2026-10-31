package service

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"image/png"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/jwt"
	"github.com/vnet/core/pkg/utils"
	"gorm.io/gorm"
)

// Xác thực 2 bước cho tài khoản nhân viên: mã 6 số đổi mỗi 30 giây trên ứng dụng
// xác thực (Google Authenticator, Authy…), theo chuẩn TOTP.
//
// Từng nhân viên tự bật hoặc tắt cho tài khoản mình ở menu tài khoản. Chủ quán bật
// "Bắt buộc xác thực 2 bước" ở Cài đặt > Tính năng (two_factor_required, mặc định
// tắt) thì ai chưa cài phải quét QR ngay ở lần đăng nhập tới, và không ai tự tắt được.
//
// Chỉ trang quản trị hỏi mã. Máy trạm không có bước đó, nên nhân viên phải qua 2
// bước bị chặn ở máy trạm (ClientLogin) — không thì token máy trạm là lối vòng.

const (
	TwoFactorRequiredKey   = "two_factor_required"
	twoFactorChallengeTTL  = 5 * time.Minute
	twoFactorIssuer        = "VNET"
	twoFactorRecoveryCount = 10
)

var (
	ErrTwoFactorRequired = errors.New("quán đang bắt buộc xác thực 2 bước — không tắt được")
	ErrBadTOTP           = errors.New("mã xác thực không đúng hoặc đã dùng")
	ErrTwoFactorWebOnly  = errors.New("tài khoản phải xác thực 2 bước nên chỉ đăng nhập ở trang quản trị, không đăng nhập ở máy trạm")
	errTwoFactorExpired  = errors.New("phiên nhập mã đã hết hạn — đăng nhập lại")
)

var totpOpts = totp.ValidateOpts{Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1}

// twoFactorRequired: chủ quán bắt buộc mọi nhân viên có 2 bước.
func twoFactorRequired(db *gorm.DB) bool {
	return settingBool(settingsGroup(db, FeaturesGroup), TwoFactorRequiredKey, false)
}

// daBatHaiBuoc: tài khoản đã cài 2 bước, đăng nhập phải qua bước nhập mã.
func daBatHaiBuoc(u *model.User) bool {
	return u.TOTPEnabledAt != nil && u.TOTPSecret != ""
}

// khoaCuaNhanVien dựng trạng thái khoá đăng nhập (login_lock.go) của một nhân viên
// đã Preload Roles.Permissions. Chủ quán (quyền "*") không bị khoá hẳn.
func khoaCuaNhanVien(u *model.User) khoaDangNhap {
	sieuQuyen := false
	for _, role := range u.Roles {
		for _, perm := range role.Permissions {
			if perm.Code == "*" {
				sieuQuyen = true
			}
		}
	}
	return khoaDangNhap{bang: "users", id: u.ID, isActive: u.IsActive,
		failed: u.FailedLogins, lockedUntil: u.LockedUntil, sieuQuyen: sieuQuyen}
}

// kiemMaTOTP so mã 6 số với khoá, lệch tối đa một bước 30 giây mỗi phía (đồng hồ
// điện thoại chạy lệch). Trả bước của mã khớp, hoặc -1.
func kiemMaTOTP(secret, code string, now time.Time) int64 {
	code = strings.TrimSpace(code)
	if secret == "" || len(code) != 6 {
		return -1
	}
	step := now.Unix() / 30
	for _, d := range []int64{0, -1, 1} {
		want, err := totp.GenerateCodeCustom(secret, time.Unix((step+d)*30, 0), totpOpts)
		if err == nil && subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 {
			return step + d
		}
	}
	return -1
}

// taoMaDuPhong sinh các mã dự phòng dạng "abcd-efgh" (40 bit ngẫu nhiên mỗi mã) và
// chuỗi băm để lưu. Mã chỉ hiện cho người dùng đúng một lần, lúc bật.
func taoMaDuPhong() ([]string, string, error) {
	codes := make([]string, 0, twoFactorRecoveryCount)
	hashes := make([]string, 0, twoFactorRecoveryCount)
	for i := 0; i < twoFactorRecoveryCount; i++ {
		b := make([]byte, 5)
		if _, err := rand.Read(b); err != nil {
			return nil, "", err
		}
		raw := strings.ToLower(base32.StdEncoding.EncodeToString(b))
		codes = append(codes, raw[:4]+"-"+raw[4:])
		hashes = append(hashes, bamMaDuPhong(raw))
	}
	return codes, strings.Join(hashes, ","), nil
}

// bamMaDuPhong bỏ khoảng trắng, gạch nối và chữ hoa trước khi băm: người dùng gõ
// "ABCD EFGH" hay "abcd-efgh" đều là một mã.
func bamMaDuPhong(code string) string {
	c := strings.ToLower(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(code)))
	sum := sha256.Sum256([]byte(c))
	return hex.EncodeToString(sum[:])
}

func demMaDuPhong(stored string) int {
	if stored == "" {
		return 0
	}
	return len(strings.Split(stored, ","))
}

// dungMa kiểm mã người dùng gõ (mã 6 số hoặc mã dự phòng) và ghi lại để mã đó
// không dùng lại được. UPDATE có điều kiện: hai lượt gửi cùng một mã cùng lúc thì
// chỉ một lượt qua.
func (s *AuthService) dungMa(u *model.User, code string, now time.Time) (bool, error) {
	if step := kiemMaTOTP(u.TOTPSecret, code, now); step >= 0 {
		res := s.db.Model(&model.User{}).Where("id = ? AND totp_last_step < ?", u.ID, step).
			UpdateColumn("totp_last_step", step)
		return res.RowsAffected == 1, res.Error
	}
	h := bamMaDuPhong(code)
	con := []string{}
	found := false
	for _, x := range strings.Split(u.TOTPRecovery, ",") {
		if !found && x != "" && subtle.ConstantTimeCompare([]byte(x), []byte(h)) == 1 {
			found = true
			continue
		}
		if x != "" {
			con = append(con, x)
		}
	}
	if !found {
		return false, nil
	}
	res := s.db.Model(&model.User{}).Where("id = ? AND totp_recovery = ?", u.ID, u.TOTPRecovery).
		UpdateColumn("totp_recovery", strings.Join(con, ","))
	return res.RowsAffected == 1, res.Error
}

type LoginTOTPRequest struct {
	Challenge string `json:"challenge" binding:"required"`
	Code      string `json:"code" binding:"required"`
}

// TwoFactorChallengeUsername đọc tên đăng nhập của vé, để handler chặn dò mã theo
// tên + IP như bước mật khẩu.
func (s *AuthService) TwoFactorChallengeUsername(challenge string) (string, error) {
	claims, err := s.jwtManager.ValidateToken(challenge)
	if err != nil || claims.TokenType != jwt.TypeTwoFactor {
		return "", errTwoFactorExpired
	}
	var row struct{ Username string }
	if err := s.db.Table("users").Select("username").Where("id = ? AND deleted_at IS NULL", claims.UserID).Take(&row).Error; err != nil {
		return "", errTwoFactorExpired
	}
	return row.Username, nil
}

// LoginTOTP là bước hai của đăng nhập trang quản trị: vé từ Login + mã 6 số (hoặc
// một mã dự phòng). Gõ sai tính vào cùng bộ đếm khoá với gõ sai mật khẩu.
func (s *AuthService) LoginTOTP(req *LoginTOTPRequest) (*LoginResponse, error) {
	claims, err := s.jwtManager.ValidateToken(req.Challenge)
	if err != nil || claims.TokenType != jwt.TypeTwoFactor {
		return nil, errTwoFactorExpired
	}
	var user model.User
	if err := s.db.Where("id = ?", claims.UserID).Preload("Roles").Preload("Roles.Permissions").First(&user).Error; err != nil {
		return nil, errTwoFactorExpired
	}
	khoa := khoaCuaNhanVien(&user)
	now := time.Now()
	if err := khoa.truocKhiKiem(s.db, now); err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, errors.New("tài khoản đã bị khoá — liên hệ quản lý")
	}
	var recovery []string
	switch {
	case daBatHaiBuoc(&user):
		ok, err := s.dungMa(&user, req.Code, now)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, saiMa(khoa, s.db, now)
		}
	case user.TOTPPending != "":
		// Quán bắt buộc 2 bước mà tài khoản chưa cài: Login vừa trả mã QR, mã đúng là cài xong.
		codes, err := s.batHaiBuoc(&user, req.Code, now)
		if errors.Is(err, ErrBadTOTP) {
			return nil, saiMa(khoa, s.db, now)
		}
		if err != nil {
			return nil, err
		}
		recovery = codes
	default:
		// Quản lý vừa gỡ 2 bước của tài khoản trong lúc đang nhập mã.
		return nil, errTwoFactorExpired
	}
	khoa.sauKhiDung(s.db)
	res, err := s.capPhienNhanVien(&user, utils.CheckPassword(DefaultSeedPassword, user.PasswordHash))
	if err != nil {
		return nil, err
	}
	res.RecoveryCodes = recovery
	return res, nil
}

// saiMa tính một lượt sai mã vào bộ đếm khoá như sai mật khẩu.
func saiMa(khoa khoaDangNhap, db *gorm.DB, now time.Time) error {
	if err := khoa.sauKhiSai(db, now); !errors.Is(err, ErrBadCredentials) {
		return err
	}
	return ErrBadTOTP
}

type TwoFactorStatus struct {
	// Required: chủ quán bắt buộc 2 bước — không tự tắt được.
	Required     bool       `json:"required"`
	Enabled      bool       `json:"enabled"`
	EnabledAt    *time.Time `json:"enabled_at,omitempty"`
	RecoveryLeft int        `json:"recovery_left"`
}

func (s *AuthService) TwoFactorStatus(userID string) (*TwoFactorStatus, error) {
	var user model.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}
	return &TwoFactorStatus{
		Required:     twoFactorRequired(s.db),
		Enabled:      user.TOTPEnabledAt != nil,
		EnabledAt:    user.TOTPEnabledAt,
		RecoveryLeft: demMaDuPhong(user.TOTPRecovery),
	}, nil
}

type TwoFactorSetupResponse struct {
	Secret string `json:"secret"`
	URL    string `json:"otpauth_url"`
	// QR: ảnh PNG dạng data URL, quét bằng ứng dụng xác thực.
	QR string `json:"qr"`
}

// TwoFactorSetup tạo khoá mới cho người đang đăng nhập và giữ ở TOTPPending: chưa
// có hiệu lực cho tới khi TwoFactorEnable nhận đúng một mã sinh từ khoá này.
func (s *AuthService) TwoFactorSetup(userID string) (*TwoFactorSetupResponse, error) {
	var user model.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}
	if user.TOTPEnabledAt != nil {
		return nil, errors.New("tài khoản đã bật xác thực 2 bước — tắt trước rồi cài lại")
	}
	return s.taoKhoaCho(&user)
}

// taoKhoaCho sinh khoá mới, ghi vào TOTPPending và trả mã QR để quét.
func (s *AuthService) taoKhoaCho(user *model.User) (*TwoFactorSetupResponse, error) {
	key, err := totp.Generate(totp.GenerateOpts{Issuer: twoFactorIssuer, AccountName: user.Username})
	if err != nil {
		return nil, err
	}
	img, err := key.Image(240, 240)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.User{}).Where("id = ?", user.ID).UpdateColumn("totp_pending", key.Secret()).Error; err != nil {
		return nil, err
	}
	return &TwoFactorSetupResponse{
		Secret: key.Secret(),
		URL:    key.URL(),
		QR:     "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()),
	}, nil
}

// TwoFactorEnable xác nhận khoá đang chờ bằng một mã từ ứng dụng, bật 2 bước và
// trả các mã dự phòng (chỉ lần này).
func (s *AuthService) TwoFactorEnable(userID, code string) ([]string, error) {
	var user model.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}
	if user.TOTPPending == "" {
		return nil, errors.New("chưa tạo mã QR — mở lại hộp Xác thực 2 bước")
	}
	return s.batHaiBuoc(&user, code, time.Now())
}

// batHaiBuoc nhận khoá đang chờ (TOTPPending) khi mã khớp, bật 2 bước và trả mã dự phòng.
func (s *AuthService) batHaiBuoc(user *model.User, code string, now time.Time) ([]string, error) {
	step := kiemMaTOTP(user.TOTPPending, code, now)
	if step < 0 {
		return nil, ErrBadTOTP
	}
	codes, hashes, err := taoMaDuPhong()
	if err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
		"totp_secret":     user.TOTPPending,
		"totp_pending":    "",
		"totp_enabled_at": now,
		"totp_last_step":  step,
		"totp_recovery":   hashes,
	}).Error; err != nil {
		return nil, err
	}
	s.ghiHaiBuoc("two_factor_enable", user.ID, user.ID, user.Username)
	return codes, nil
}

// TwoFactorDisable: người dùng tự tắt, phải gõ lại mật khẩu.
func (s *AuthService) TwoFactorDisable(userID, password string) error {
	if twoFactorRequired(s.db) {
		return ErrTwoFactorRequired
	}
	var user model.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return err
	}
	if !utils.CheckPassword(password, user.PasswordHash) {
		return errors.New("mật khẩu không đúng")
	}
	if err := s.tatHaiBuoc(user.ID); err != nil {
		return err
	}
	s.ghiHaiBuoc("two_factor_disable", user.ID, user.ID, user.Username)
	return nil
}

// ResetTwoFactor là nút "Tắt 2FA" ở trang Người dùng: nhân viên mất điện thoại và
// hết mã dự phòng thì quản lý tắt hộ, người đó đăng nhập bằng mật khẩu rồi bật lại.
func (s *AuthService) ResetTwoFactor(id, byUserID string) error {
	var row struct{ Username string }
	if err := s.db.Table("users").Select("username").Where("id = ? AND deleted_at IS NULL", id).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("không tìm thấy tài khoản")
		}
		return err
	}
	if err := s.tatHaiBuoc(id); err != nil {
		return err
	}
	s.ghiHaiBuoc("two_factor_reset", id, byUserID, row.Username)
	return nil
}

func (s *AuthService) tatHaiBuoc(id string) error {
	return s.db.Model(&model.User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"totp_secret":     "",
		"totp_pending":    "",
		"totp_enabled_at": nil,
		"totp_last_step":  0,
		"totp_recovery":   "",
	}).Error
}

// ghiHaiBuoc ghi nhật ký hoạt động và báo trang Người dùng (cột 2FA) tải lại.
func (s *AuthService) ghiHaiBuoc(action, id, byUserID, username string) {
	if s.audit != nil {
		s.audit.Log(&LogAuditRequest{
			Action:     action,
			EntityType: "user",
			EntityID:   id,
			UserID:     optionalUUID(byUserID),
			Metadata:   map[string]string{"username": username},
		})
	}
	s.hub.Changed("systemManage", nil)
}
