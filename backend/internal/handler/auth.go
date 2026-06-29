package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/response"
)

type AuthHandler struct {
	svc          *service.AuthService
	sessionSvc   *service.SessionService
}

func NewAuthHandler(svc *service.AuthService, sessionSvc *service.SessionService) *AuthHandler {
	_ = model.Permission{}
	return &AuthHandler{svc: svc, sessionSvc: sessionSvc}
}

// Login
// @Summary      Login
// @Description  Authenticate user with username and password
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body  service.LoginRequest  true  "Login credentials"
// @Success      200   {object}  response.Response{data=service.LoginResponse}
// @Failure      400   {object}  response.Response
// @Failure      401   {object}  response.Response
// @Router       /api/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req service.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	result, err := h.svc.Login(&req)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}

	response.Success(c, result)
}

// QRLogin
// @Summary      QR Login
// @Description  Authenticate user via QR code
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body  service.QRLoginRequest  true  "QR code data"
// @Success      200   {object}  response.Response{data=service.LoginResponse}
// @Failure      400   {object}  response.Response
// @Router       /api/auth/qr-login [post]
func (h *AuthHandler) QRLogin(c *gin.Context) {
	var req service.QRLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	result, err := h.svc.QRLogin(&req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, result)
}

// MemberLogin
// @Summary      Member Login
// @Description  Authenticate member with username and password
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body  service.MemberLoginRequest  true  "Member login credentials"
// @Success      200   {object}  response.Response{data=service.LoginResponse}
// @Failure      400   {object}  response.Response
// @Failure      401   {object}  response.Response
// @Router       /api/auth/member-login [post]
func (h *AuthHandler) MemberLogin(c *gin.Context) {
	var req service.MemberLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	result, err := h.svc.MemberLogin(&req)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}

	if req.MachineID != "" && result.User.Role != "admin" {
		session, err := h.sessionSvc.StartSession(&service.StartRequest{
			MachineID: req.MachineID,
			MemberID:  result.User.ID,
		})
		if err == nil && session != nil {
			result.SessionID = session.ID
		}
	}

	response.Success(c, result)
}

// Refresh
// @Summary      Refresh Token
// @Description  Refresh access token using refresh token
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body  service.RefreshRequest  true  "Refresh token"
// @Success      200   {object}  response.Response{data=service.LoginResponse}
// @Failure      400   {object}  response.Response
// @Failure      401   {object}  response.Response
// @Router       /api/auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req service.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	result, err := h.svc.RefreshToken(&req)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}

	response.Success(c, result)
}

// Me
// @Summary      Get Current User
// @Description  Get current authenticated user details
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200   {object}  response.Response{data=service.UserResponse}
// @Failure      401   {object}  response.Response
// @Failure      404   {object}  response.Response
// @Router       /api/auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	userID := middleware.GetUserID(c)

	result, err := h.svc.GetCurrentUser(userID)
	if err != nil {
		response.NotFound(c, "User not found")
		return
	}

	response.Success(c, result)
}

// ChangePassword
// @Summary      Change Password
// @Description  Change current user's password
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  service.ChangePasswordRequest  true  "Password change data"
// @Success      200   {object}  response.Response
// @Failure      400   {object}  response.Response
// @Failure      401   {object}  response.Response
// @Router       /api/auth/change-password [post]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req service.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	if err := h.svc.ChangePassword(userID, &req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// GetPermissions
// @Summary      Get Permissions
// @Description  Get all available permissions
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200   {object}  response.Response{data=[]model.Permission}
// @Failure      500   {object}  response.Response
// @Router       /api/auth/permissions [get]
func (h *AuthHandler) GetPermissions(c *gin.Context) {
	permissions, err := h.svc.GetPermissions()
	if err != nil {
		response.InternalError(c, "Failed to fetch permissions")
		return
	}

	response.Success(c, permissions)
}
