package service

import (
	"errors"
	"time"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/jwt"
	"github.com/vnet/core/pkg/utils"
	"gorm.io/gorm"
)

type AuthService struct {
	db          *gorm.DB
	jwtManager  *jwt.Manager
	audit       *AuditService
}

func NewAuthService(db *gorm.DB, jwtManager *jwt.Manager, audit *AuditService) *AuthService {
	return &AuthService{db: db, jwtManager: jwtManager, audit: audit}
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
			return nil, errors.New("invalid username or password")
		}
		return nil, err
	}

	if !user.IsActive {
		return nil, errors.New("account is disabled")
	}

	if !utils.CheckPassword(req.Password, user.PasswordHash) {
		return nil, errors.New("invalid username or password")
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

	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Username, roleName, roleID, permissions)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID)
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
		return nil, errors.New("invalid or inactive member QR code")
	}

	accessToken, err := s.jwtManager.GenerateAccessToken(member.ID, member.FullName, "member", "", []string{"member.access"})
	if err != nil {
		return nil, err
	}
	refreshToken, _ := s.jwtManager.GenerateRefreshToken(member.ID)

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

func (s *AuthService) MemberLogin(req *MemberLoginRequest) (*MemberLoginResponse, error) {
	var member model.Member
	if err := s.db.Where("username = ? AND is_active = ?", req.Username, true).First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid username or password")
		}
		return nil, err
	}

	if !utils.CheckPassword(req.Password, member.PasswordHash) {
		return nil, errors.New("invalid username or password")
	}

	role := member.Role
	if role == "" {
		role = "member"
	}

	accessToken, err := s.jwtManager.GenerateAccessToken(member.ID, member.Username, role, "", []string{"member.access"})
	if err != nil {
		return nil, err
	}
	refreshToken, _ := s.jwtManager.GenerateRefreshToken(member.ID)

	return &MemberLoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
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
		return nil, errors.New("invalid or expired refresh token")
	}

	var user model.User
	if err := s.db.Where("id = ? AND is_active = ?", claims.UserID, true).Preload("Roles").Preload("Roles.Permissions").First(&user).Error; err != nil {
		return nil, errors.New("user not found or inactive")
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

	accessToken, err := s.jwtManager.GenerateAccessToken(claims.UserID, user.Username, roleName, roleID, permissions)
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

func (s *AuthService) ChangePassword(userID string, req *ChangePasswordRequest) error {
	var user model.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return err
	}

	if !utils.CheckPassword(req.OldPassword, user.PasswordHash) {
		return errors.New("current password is incorrect")
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


