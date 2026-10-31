package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/jwt"
	"github.com/vnet/core/pkg/response"
)

type AuthHandler struct {
	svc      *service.AuthService
	throttle *loginThrottle
}

func NewAuthHandler(svc *service.AuthService) *AuthHandler {
	// Giữ tham chiếu cho Swagger sinh được kiểu trả về của /auth/permissions.
	_ = model.Permission{}
	return &AuthHandler{svc: svc, throttle: newLoginThrottle()}
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
// @Failure      429   {object}  response.Response  "Sai quá nhiều lần — kèm header Retry-After"
// @Router       /api/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req service.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	key, ok := h.allowLogin(c, req.Username)
	if !ok {
		return
	}
	result, err := h.svc.Login(&req)
	// Mật khẩu đúng nhưng còn bước mã 2 bước: chưa xong, nên KHÔNG xoá bộ chặn — không thì
	// sai mã vài lần rồi gõ lại mật khẩu là có lượt dò mã mới, dò được vô hạn. Nhật ký ghi ở
	// LoginTOTP, khi biết kết quả cuối.
	if err == nil && result.NeedTOTP {
		response.Success(c, result)
		return
	}
	h.settleLogin(key, err)
	userID := ""
	if err == nil && result.User != nil {
		userID = result.User.ID
	}
	h.svc.LogLogin(req.Username, userID, c.ClientIP(), err)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}

	response.Success(c, result)
}

// LoginTOTP
// @Summary      Đăng nhập bước 2: mã xác thực
// @Description  Gửi vé (challenge) mà /auth/login trả khi need_totp = true, kèm mã 6 số từ ứng dụng xác thực hoặc một mã dự phòng. Vé sống 5 phút. Gõ sai tính vào cùng bộ đếm khoá với sai mật khẩu.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body  service.LoginTOTPRequest  true  "Vé và mã"
// @Success      200   {object}  response.Response{data=service.LoginResponse}
// @Failure      401   {object}  response.Response
// @Failure      429   {object}  response.Response  "Sai quá nhiều lần — kèm header Retry-After"
// @Router       /api/auth/login/totp [post]
func (h *AuthHandler) LoginTOTP(c *gin.Context) {
	var req service.LoginTOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	username, err := h.svc.TwoFactorChallengeUsername(req.Challenge)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}
	key, ok := h.allowLogin(c, username)
	if !ok {
		return
	}
	result, err := h.svc.LoginTOTP(&req)
	h.settleLogin(key, err)
	userID := ""
	if err == nil && result.User != nil {
		userID = result.User.ID
	}
	h.svc.LogLogin(username, userID, c.ClientIP(), err)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}

	response.Success(c, result)
}

// TwoFactorStatus
// @Summary      Trạng thái xác thực 2 bước của tài khoản đang đăng nhập
// @Tags         Auth
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=service.TwoFactorStatus}
// @Router       /api/auth/2fa [get]
func (h *AuthHandler) TwoFactorStatus(c *gin.Context) {
	status, err := h.svc.TwoFactorStatus(middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, status)
}

// TwoFactorSetup
// @Summary      Tạo mã QR để bật xác thực 2 bước
// @Description  Khoá mới chỉ có hiệu lực sau khi /auth/2fa/enable nhận đúng một mã sinh từ nó.
// @Tags         Auth
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=service.TwoFactorSetupResponse}
// @Failure      400  {object}  response.Response
// @Router       /api/auth/2fa/setup [post]
func (h *AuthHandler) TwoFactorSetup(c *gin.Context) {
	res, err := h.svc.TwoFactorSetup(middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, res)
}

type TwoFactorCodeRequest struct {
	Code string `json:"code" binding:"required"`
}

// TwoFactorEnable
// @Summary      Bật xác thực 2 bước
// @Description  Xác nhận bằng một mã từ ứng dụng; trả 10 mã dự phòng, chỉ hiện lần này.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  TwoFactorCodeRequest  true  "Mã 6 số"
// @Success      200   {object}  response.Response{data=[]string}
// @Failure      400   {object}  response.Response
// @Router       /api/auth/2fa/enable [post]
func (h *AuthHandler) TwoFactorEnable(c *gin.Context) {
	var req TwoFactorCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	codes, err := h.svc.TwoFactorEnable(middleware.GetUserID(c), req.Code)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, codes)
}

type TwoFactorDisableRequest struct {
	Password string `json:"password" binding:"required"`
}

// TwoFactorDisable
// @Summary      Tự tắt xác thực 2 bước
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  TwoFactorDisableRequest  true  "Mật khẩu hiện tại"
// @Success      200   {object}  response.Response
// @Failure      400   {object}  response.Response
// @Router       /api/auth/2fa/disable [post]
func (h *AuthHandler) TwoFactorDisable(c *gin.Context) {
	var req TwoFactorDisableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	if err := h.svc.TwoFactorDisable(middleware.GetUserID(c), req.Password); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// ResetUserTwoFactor
// @Summary      Tắt xác thực 2 bước của một nhân viên
// @Description  Cho người mất điện thoại và hết mã dự phòng: người đó đăng nhập lại bằng mật khẩu rồi tự bật lại.
// @Tags         SystemManage
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  UnlockUserLoginRequest  true  "User ID"
// @Success      200   {object}  response.Response
// @Failure      400   {object}  response.Response
// @Router       /api/systemManage/resetUserTwoFactor [post]
func (h *AuthHandler) ResetUserTwoFactor(c *gin.Context) {
	var req UnlockUserLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	if err := h.svc.ResetTwoFactor(req.ID, middleware.GetUserID(c)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
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
// @Failure      429   {object}  response.Response  "Sai quá nhiều lần — kèm header Retry-After"
// @Router       /api/auth/member-login [post]
func (h *AuthHandler) MemberLogin(c *gin.Context) {
	var req service.MemberLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	key, ok := h.allowLogin(c, req.Username)
	if !ok {
		return
	}
	// Mở phiên nằm trong MemberLogin: đăng nhập được mà không mở được phiên là
	// khách ngồi máy không ai tính tiền, nên hai việc đó phải thành hoặc bại
	// cùng nhau.
	result, err := h.svc.MemberLogin(&req)
	h.settleLogin(key, err)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}

	response.Success(c, result)
}

// ClientLogin
// @Summary      Client Login (staff or member)
// @Description  Một ô đăng nhập cho cả nhân viên lẫn hội viên; tên tài khoản quyết định đường đi
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body  service.MemberLoginRequest  true  "Tài khoản, mật khẩu, mã máy"
// @Success      200   {object}  response.Response{data=service.ClientLoginResponse}
// @Failure      401   {object}  response.Response
// @Failure      429   {object}  response.Response  "Sai quá nhiều lần — kèm header Retry-After"
// @Router       /api/auth/client-login [post]
func (h *AuthHandler) ClientLogin(c *gin.Context) {
	var req service.MemberLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	// Cùng bộ đếm với /auth/login: tên nhân viên gõ ở đây đi thẳng vào Login,
	// nên chặn một cửa mà để cửa kia mở thì dò mật khẩu admin qua cửa còn lại.
	key, ok := h.allowLogin(c, req.Username)
	if !ok {
		return
	}
	result, err := h.svc.ClientLogin(&req)
	h.settleLogin(key, err)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
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

	// Hội viên và nhân viên nằm ở hai bảng khác nhau; tra nhầm bảng thì hội viên
	// gọi /auth/me luôn hỏng.
	var result *service.UserResponse
	var err error
	kind := "member"
	if middleware.GetKind(c) == jwt.KindStaff {
		kind = "staff"
		result, err = h.svc.GetCurrentUser(userID)
	} else {
		result, err = h.svc.GetCurrentMember(userID)
	}
	if err == nil && result != nil {
		result.Kind = kind
	}
	if err != nil {
		// 401 chứ KHÔNG phải 404: token hợp lệ về mặt chữ ký nhưng trỏ tới một
		// tài khoản không còn tồn tại (tài khoản bị xoá, hoặc database vừa được
		// phục hồi từ bản sao lưu khác). Đó là lỗi xác thực.
		//
		// Trả 404 làm admin không nhận ra: axios chỉ xử lý 401 và ba mã
		// 9999/8888/7777, nên 404 lọt qua như một lỗi thường và giao diện treo
		// vĩnh viễn ở màn hình chờ khởi động thay vì quay về trang đăng nhập.
		response.ForceLogout(c)
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

	isMember := middleware.GetKind(c) != jwt.KindStaff
	if err := h.svc.ChangePassword(userID, isMember, &req); err != nil {
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
		response.InternalError(c, "Failed to fetch permissions", err)
		return
	}

	response.Success(c, permissions)
}

// UnlockUserLoginRequest là nội dung POST /api/systemManage/unlockUser.
type UnlockUserLoginRequest struct {
	ID string `json:"id" binding:"required"`
}

// UnlockUser
// @Summary      Mở khoá đăng nhập nhân viên
// @Description  Xoá khoá tạm do nhập sai mật khẩu (bộ đếm về 0, bỏ mốc khoá, xoá bộ chặn theo tên). Tài khoản đã khoá hẳn thì kích hoạt lại ở Sửa.
// @Tags         SystemManage
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  UnlockUserLoginRequest  true  "ID nhân viên"
// @Success      200   {object}  response.Response
// @Failure      400   {object}  response.Response
// @Router       /api/systemManage/unlockUser [post]
func (h *AuthHandler) UnlockUser(c *gin.Context) {
	var req UnlockUserLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	username, err := h.svc.UnlockUserLogin(req.ID, middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	h.throttle.clearUser(username)
	response.Success(c, nil)
}

// UnlockMember
// @Summary      Mở khoá đăng nhập hội viên
// @Description  Xoá khoá tạm do nhập sai mật khẩu (bộ đếm về 0, bỏ mốc khoá, xoá bộ chặn theo tên). Tài khoản đã khoá hẳn thì kích hoạt lại ở Sửa.
// @Tags         Members
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "Member ID"
// @Success      200   {object}  response.Response
// @Failure      400   {object}  response.Response
// @Router       /api/members/{id}/unlock-login [post]
func (h *AuthHandler) UnlockMember(c *gin.Context) {
	username, err := h.svc.UnlockMemberLogin(c.Param("id"), middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	h.throttle.clearUser(username)
	response.Success(c, nil)
}
