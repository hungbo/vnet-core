package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/jwt"
	"github.com/vnet/core/pkg/utils"
	"gorm.io/gorm"
)

type AuthService struct {
	db         *gorm.DB
	jwtManager *jwt.Manager
	audit      *AuditService
	curfew     *CurfewService
	sessions   *SessionService
}

func NewAuthService(db *gorm.DB, jwtManager *jwt.Manager, audit *AuditService) *AuthService {
	return &AuthService{db: db, jwtManager: jwtManager, audit: audit}
}

// WithCurfew nối dịch vụ giới nghiêm vào. Không có nó, MemberLogin vẫn chặn
// được máy lạ và khách hết tiền, nhưng KHÔNG chặn được trẻ vị thành niên ngồi
// máy quá giờ cấm.
func (s *AuthService) WithCurfew(c *CurfewService) *AuthService {
	s.curfew = c
	return s
}

// WithSessions nối dịch vụ phiên vào. Đăng nhập trên máy trạm PHẢI mở được
// phiên, nếu không khách ngồi máy mà không có gì tính tiền.
func (s *AuthService) WithSessions(sess *SessionService) *AuthService {
	s.sessions = sess
	return s
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	User         *UserResponse `json:"user"`
}

type UserResponse struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	FullName    string   `json:"full_name"`
	Email       string   `json:"email"`
	Phone       string   `json:"phone"`
	AvatarURL   string   `json:"avatar_url"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}

func (s *AuthService) Login(req *LoginRequest) (*LoginResponse, error) {
	var user model.User
	if err := s.db.Where("username = ?", req.Username).Preload("Roles").Preload("Roles.Permissions").First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("sai tên đăng nhập hoặc mật khẩu")
		}
		return nil, err
	}

	if !user.IsActive {
		return nil, errors.New("tài khoản đã bị khoá — liên hệ quản lý")
	}

	if !utils.CheckPassword(req.Password, user.PasswordHash) {
		return nil, errors.New("sai tên đăng nhập hoặc mật khẩu")
	}

	var permissions []string
	var roleName string
	var roleID string

	if len(user.Roles) > 0 {
		roleName = user.Roles[0].Name
		roleID = user.Roles[0].ID
		for _, role := range user.Roles {
			for _, perm := range role.Permissions {
				permissions = append(permissions, perm.Code)
			}
		}
	}

	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Username, roleName, roleID, jwt.KindStaff, permissions)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID, jwt.KindStaff)
	if err != nil {
		return nil, err
	}

	s.db.Model(&user).Update("last_login_at", time.Now())

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: &UserResponse{
			ID:          user.ID,
			Username:    user.Username,
			FullName:    user.FullName,
			Email:       user.Email,
			Phone:       user.Phone,
			AvatarURL:   user.AvatarURL,
			Role:        roleName,
			Permissions: permissions,
		},
	}, nil
}

type QRLoginRequest struct {
	QRCode string `json:"qr_code" binding:"required"`
}

func (s *AuthService) QRLogin(req *QRLoginRequest) (*LoginResponse, error) {
	var member model.Member
	if err := s.db.Where("id = ? AND is_active = ?", req.QRCode, true).First(&member).Error; err != nil {
		return nil, errors.New("mã QR không hợp lệ hoặc hội viên đã bị khoá")
	}

	accessToken, err := s.jwtManager.GenerateAccessToken(member.ID, member.FullName, "member", "", jwt.KindMember, []string{"member.access"})
	if err != nil {
		return nil, err
	}
	refreshToken, err := s.jwtManager.GenerateRefreshToken(member.ID, jwt.KindMember)
	if err != nil {
		return nil, err
	}

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: &UserResponse{
			ID:       member.ID,
			Username: member.Username,
			FullName: member.FullName,
			Role:     "member",
		},
	}, nil
}

type MemberLoginRequest struct {
	Username    string `json:"username" binding:"required"`
	Password    string `json:"password" binding:"required"`
	MachineCode string `json:"machine_code"`
}

type MemberLoginResponse struct {
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	User         *UserResponse `json:"user"`
	SessionID    string        `json:"session_id,omitempty"`
}

// ClientLoginResponse thêm đúng một trường so với đăng nhập hội viên: ai vừa vào.
type ClientLoginResponse struct {
	*MemberLoginResponse
	// "staff" hoặc "member". Máy trạm cần biết để dựng đúng màn hình: nhân viên
	// vào máy thì không mở phiên và không tính tiền.
	Kind string `json:"kind"`
}

// ClientLogin là MỘT ô đăng nhập cho cả nhân viên lẫn hội viên.
//
// Máy trạm trước đây có hai đường riêng và một nút "Đăng nhập quản trị" nằm
// khuất bên dưới. Nhân viên phải nhớ mình là loại tài khoản nào trước khi gõ,
// mà gõ nhầm ô thì thông báo là "sai mật khẩu" — sai chỗ để đi tìm.
//
// Tên tài khoản quyết định đường đi, không phải người dùng chọn:
//
//   - có trong bảng users  → đăng nhập nhân viên, KHÔNG mở phiên, không tính tiền
//   - còn lại              → đăng nhập hội viên, mở phiên và bắt đầu tính tiền
//
// Trùng tên giữa hai bảng thì NHÂN VIÊN thắng, và tài khoản đó không bao giờ
// đăng nhập được kiểu hội viên nữa. Đó là chủ ý: một cái tên chỉ được là một
// thứ, còn hơn là tuỳ mật khẩu gõ vào mà thành người này hay người kia.
func (s *AuthService) ClientLogin(req *MemberLoginRequest) (*ClientLoginResponse, error) {
	var user model.User
	err := s.db.Where("username = ?", req.Username).First(&user).Error

	if err == nil {
		res, err := s.Login(&LoginRequest{Username: req.Username, Password: req.Password})
		if err != nil {
			return nil, err
		}
		return &ClientLoginResponse{
			MemberLoginResponse: &MemberLoginResponse{
				AccessToken:  res.AccessToken,
				RefreshToken: res.RefreshToken,
				User:         res.User,
			},
			Kind: "staff",
		}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	res, err := s.MemberLogin(req)
	if err != nil {
		return nil, err
	}
	return &ClientLoginResponse{MemberLoginResponse: res, Kind: "member"}, nil
}

// machineForLogin tra máy theo mã máy khách tự khai trong phần Cài đặt.
//
// Mã máy KHÔNG phải bí mật, nên đây không phải xác thực — nhưng nó chặn được
// trường hợp một máy chưa hề khai báo trong danh sách vẫn cho khách ngồi: máy
// đó không thuộc nhóm nào, không có giá, không ai nhìn thấy trên trang quản trị,
// và mọi phiên trên đó là doanh thu quán không bao giờ thu được.
func (s *AuthService) machineForLogin(code string) (*model.Machine, error) {
	if strings.TrimSpace(code) == "" {
		return nil, errors.New("máy chưa khai mã máy — mở Cài đặt trên máy khách và điền Mã máy")
	}
	var machine model.Machine
	if err := s.db.Where("machine_code = ?", code).First(&machine).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("máy %q chưa có trong danh sách máy — nhờ nhân viên thêm máy này trước", code)
		}
		return nil, err
	}
	if !machine.IsActive {
		return nil, fmt.Errorf("máy %q đang bị khoá", code)
	}
	return &machine, nil
}

// openSessionForLogin trả về mã phiên đang chạy của khách trên đúng máy này,
// mở phiên mới nếu chưa có.
func (s *AuthService) openSessionForLogin(member *model.Member, machine *model.Machine, role string) (string, error) {
	// Tài khoản quản trị đăng nhập trên máy trạm là để kiểm tra máy, không phải
	// để chơi — không mở phiên tính tiền cho họ.
	if role == "admin" || machine == nil {
		return "", nil
	}

	// Đóng trước phiên còn sót từ TRƯỚC lần reboot của máy này. Trên đĩa đóng
	// băng, reboot đưa máy về màn hình khoá nhưng phiên cũ vẫn sống trên máy
	// chủ; người ngồi vào bây giờ là lượt mới, phải là phiên mới. Không đóng
	// thì khách khác bị chặn "máy đang bận", còn cùng một khách thì bị nối lại
	// phiên cũ tính từ trước reboot.
	if s.sessions != nil {
		if n, err := s.sessions.EndSessionsStaleAfterReboot(machine.ID); err != nil {
			return "", err
		} else if n > 0 {
			// Phiên cũ vừa đóng, máy vừa được giải phóng — xuống dưới mở phiên
			// mới. KHÔNG nối lại phiên nào.
			session, err := s.sessions.StartSession(&StartRequest{MachineID: machine.ID, MemberID: member.ID})
			if err != nil {
				return "", err
			}
			return session.ID, nil
		}
	}

	// Khách đang giữa phiên mà GIAO DIỆN khởi động lại (không phải máy reboot)
	// thì phải vào lại được: tiền đã trả rồi, chặn ở đây là nhốt khách ngoài
	// chính máy họ đang thuê. Phân biệt với reboot ở trên bằng booted_at.
	var existing model.MachineSession
	err := s.db.Where("member_id = ? AND machine_id = ? AND is_active = ?", member.ID, machine.ID, true).
		First(&existing).Error
	if err == nil {
		return existing.ID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}

	if s.sessions == nil {
		// Không nối SessionService thì ít nhất cũng phải kiểm đủ điều kiện, chứ
		// không được lặng lẽ cho vào.
		return "", CheckMemberMayPlay(s.db, s.curfew, member, false)
	}

	session, err := s.sessions.StartSession(&StartRequest{MachineID: machine.ID, MemberID: member.ID})
	if err != nil {
		return "", err
	}
	return session.ID, nil
}

func (s *AuthService) MemberLogin(req *MemberLoginRequest) (*MemberLoginResponse, error) {
	var member model.Member
	if err := s.db.Where("username = ? AND is_active = ?", req.Username, true).First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("sai tên đăng nhập hoặc mật khẩu")
		}
		return nil, err
	}

	if !utils.CheckPassword(req.Password, member.PasswordHash) {
		return nil, errors.New("sai tên đăng nhập hoặc mật khẩu")
	}

	// Đây là màn hình khoá mà KHÁCH tự gõ mật khẩu vào — cửa vào máy thật sự,
	// chứ không phải /sessions/start (đường nhân viên mở máy từ trang quản trị).
	// Trước đây machine_code được máy khách gửi lên rồi bị bỏ qua hoàn toàn, và
	// không có một phép kiểm nào: cài máy khách với mã máy bịa ra cũng vào được,
	// khách số dư 0 cũng vào được và dùng máy mà không phiên nào tính tiền.
	machine, err := s.machineForLogin(req.MachineCode)
	if err != nil {
		return nil, err
	}

	role := member.Role
	if role == "" {
		role = "member"
	}

	// Mở phiên NGAY trong lượt đăng nhập, và để lỗi nổi lên.
	//
	// Bản cũ có mở phiên nhưng nuốt mọi lỗi bằng log.Printf rồi vẫn trả 200:
	// máy không có trong danh sách, khách số dư 0, khách đang nợ, trẻ vị thành
	// niên trong giờ cấm — tất cả đều "đăng nhập thành công" rồi ngồi máy mà
	// không phiên nào tính tiền. Dòng log đó nằm trong container, không ai đọc.
	sessionID, err := s.openSessionForLogin(&member, machine, role)
	if err != nil {
		return nil, err
	}

	accessToken, err := s.jwtManager.GenerateAccessToken(member.ID, member.Username, role, "", jwt.KindMember, []string{"member.access"})
	if err != nil {
		return nil, err
	}
	refreshToken, err := s.jwtManager.GenerateRefreshToken(member.ID, jwt.KindMember)
	if err != nil {
		return nil, err
	}

	return &MemberLoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		SessionID:    sessionID,
		User: &UserResponse{
			ID:       member.ID,
			Username: member.Username,
			FullName: member.FullName,
			Role:     role,
		},
	}, nil
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (s *AuthService) RefreshToken(req *RefreshRequest) (*LoginResponse, error) {
	claims, err := s.jwtManager.ValidateToken(req.RefreshToken)
	if err != nil {
		return nil, errors.New("phiên đăng nhập đã hết hạn, hãy đăng nhập lại")
	}
	if claims.TokenType != jwt.TypeRefresh {
		return nil, errors.New("phiên đăng nhập đã hết hạn, hãy đăng nhập lại")
	}

	var user model.User
	if err := s.db.Where("id = ? AND is_active = ?", claims.UserID, true).Preload("Roles").Preload("Roles.Permissions").First(&user).Error; err != nil {
		return nil, errors.New("không tìm thấy tài khoản hoặc tài khoản đã bị khoá")
	}

	var permissions []string
	var roleName string
	var roleID string
	if len(user.Roles) > 0 {
		roleName = user.Roles[0].Name
		roleID = user.Roles[0].ID
		for _, role := range user.Roles {
			for _, perm := range role.Permissions {
				permissions = append(permissions, perm.Code)
			}
		}
	}

	accessToken, err := s.jwtManager.GenerateAccessToken(claims.UserID, user.Username, roleName, roleID, jwt.KindStaff, permissions)
	if err != nil {
		return nil, err
	}

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: req.RefreshToken,
		User: &UserResponse{
			ID:          user.ID,
			Username:    user.Username,
			FullName:    user.FullName,
			Role:        roleName,
			Permissions: permissions,
		},
	}, nil
}

// GetCurrentMember trả hồ sơ của hội viên đang đăng nhập.
//
// Nhân viên nằm ở bảng users, hội viên nằm ở bảng members. GetCurrentUser chỉ
// tra bảng users, nên /auth/me gọi bằng token hội viên trước nay LUÔN thất bại —
// đúng cái bẫy đã làm hỏng chức năng đổi PIN từ máy trạm.
func (s *AuthService) GetCurrentMember(memberID string) (*UserResponse, error) {
	var member model.Member
	if err := s.db.Where("id = ?", memberID).First(&member).Error; err != nil {
		return nil, err
	}
	role := member.Role
	if role == "" {
		role = "member"
	}
	return &UserResponse{
		ID:          member.ID,
		Username:    member.Username,
		FullName:    member.FullName,
		Email:       member.Email,
		Phone:       member.Phone,
		AvatarURL:   member.AvatarURL,
		Role:        role,
		Permissions: []string{"member.access"},
	}, nil
}

func (s *AuthService) GetCurrentUser(userID string) (*UserResponse, error) {
	var user model.User
	if err := s.db.Where("id = ?", userID).Preload("Roles").Preload("Roles.Permissions").First(&user).Error; err != nil {
		return nil, err
	}

	var permissions []string
	var roleName string
	if len(user.Roles) > 0 {
		roleName = user.Roles[0].Name
		for _, role := range user.Roles {
			for _, perm := range role.Permissions {
				permissions = append(permissions, perm.Code)
			}
		}
	}

	return &UserResponse{
		ID:          user.ID,
		Username:    user.Username,
		FullName:    user.FullName,
		Email:       user.Email,
		Phone:       user.Phone,
		AvatarURL:   user.AvatarURL,
		Role:        roleName,
		Permissions: permissions,
	}, nil
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// ChangePassword updates the password of whoever the token belongs to. Staff
// live in users and members live in members: looking a member up in users can
// only ever fail, which is why changing a PIN from the client never worked.
func (s *AuthService) ChangePassword(userID string, isMember bool, req *ChangePasswordRequest) error {
	if isMember {
		return s.changeMemberPassword(userID, req)
	}
	return s.changeStaffPassword(userID, req)
}

func (s *AuthService) changeStaffPassword(userID string, req *ChangePasswordRequest) error {
	var user model.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return errors.New("không tìm thấy tài khoản")
	}

	if !utils.CheckPassword(req.OldPassword, user.PasswordHash) {
		return errors.New("mật khẩu hiện tại không đúng")
	}

	hash, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		return err
	}
	if err := s.db.Model(&user).Update("password_hash", hash).Error; err != nil {
		return err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "change_password",
		EntityType: "user",
		EntityID:   userID,
		UserID:     &userID,
	})
	return nil
}

func (s *AuthService) changeMemberPassword(memberID string, req *ChangePasswordRequest) error {
	var member model.Member
	if err := s.db.Where("id = ?", memberID).First(&member).Error; err != nil {
		return errors.New("không tìm thấy hội viên")
	}

	if !utils.CheckPassword(req.OldPassword, member.PasswordHash) {
		return errors.New("mật khẩu hiện tại không đúng")
	}

	hash, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		return err
	}
	if err := s.db.Model(&member).Update("password_hash", hash).Error; err != nil {
		return err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "change_password",
		EntityType: "member",
		EntityID:   memberID,
	})
	return nil
}

func (s *AuthService) GetPermissions() ([]model.Permission, error) {
	var permissions []model.Permission
	if err := s.db.Find(&permissions).Error; err != nil {
		return nil, err
	}
	return permissions, nil
}
