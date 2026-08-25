package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/database"
	"github.com/vnet/core/pkg/jwt"
	"github.com/vnet/core/pkg/response"
	"gorm.io/gorm"
)

const (
	ContextKeyUserID      = "user_id"
	ContextKeyUsername    = "username"
	ContextKeyRole        = "role"
	ContextKeyRoleID      = "role_id"
	ContextKeyPermissions = "permissions"
	ContextKeyKind        = "kind"
)

func AuthRequired(jwtManager *jwt.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := extractToken(c)
		if tokenString == "" {
			response.ForceLogout(c)
			c.Abort()
			return
		}

		claims, err := jwtManager.ValidateToken(tokenString)
		if err != nil {
			response.TokenExpired(c)
			c.Abort()
			return
		}

		// A refresh token is signed with the same key and the same claims type,
		// so it only differs by this field. Without the check it would work as
		// a long-lived access token.
		if claims.TokenType != jwt.TypeAccess {
			response.ForceLogout(c)
			c.Abort()
			return
		}

		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyUsername, claims.Username)
		c.Set(ContextKeyRole, claims.Role)
		c.Set(ContextKeyRoleID, claims.RoleID)
		c.Set(ContextKeyPermissions, claims.Permissions)
		c.Set(ContextKeyKind, claims.Kind)

		c.Next()
	}
}

// AuthOrAgent cho phép MỘT TRONG HAI cách vào: token người dùng như thường, hoặc
// chỉ mã máy (tiến trình nền của máy trạm, không có tài khoản người).
//
// Đây là thứ vá lỗ hổng lớn nhất của máy trạm: WebSocket trước đây chỉ nối được
// SAU KHI khách đăng nhập. Máy trống thì không có kết nối nào, nên nhân viên
// KHÔNG tắt hay khoá được máy trống — đúng lúc cần nhất. Tiến trình nền chạy như
// dịch vụ Windows giữ kết nối 24/7 chỉ bằng mã máy.
//
// Kết nối kiểu này KHÔNG có user_id và KHÔNG có role_id, nên hub xếp nó là client
// thường chứ không phải quản trị, và mọi middleware phân quyền phía sau đều từ
// chối nó. Khoá máy trạm đã bỏ: nhận diện chỉ bằng mã máy trong URL.
func AuthOrAgent(jwtManager *jwt.Manager) gin.HandlerFunc {
	authRequired := AuthRequired(jwtManager)

	return func(c *gin.Context) {
		machineCode := c.Query("machine_code")

		// Ưu tiên token người dùng. Giao diện máy trạm gửi kèm CẢ mã máy, nên
		// nếu chỉ nhìn mã máy thì kết nối của khách bị xếp nhầm là tiến trình
		// nền và mất danh tính người dùng — phòng chat sẽ không vào được.
		if extractToken(c) != "" || machineCode == "" {
			authRequired(c)
			return
		}

		c.Set(ContextKeyKind, "agent")
		c.Next()
	}
}

// StaffOnly rejects tokens issued by the member-facing login flows. The
// permission list alone is not enough to gate on: member tokens carry
// "member.access" and every admin route would otherwise accept them.
func StaffOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if GetKind(c) != jwt.KindStaff {
			response.Forbidden(c, "Staff access required")
			c.Abort()
			return
		}

		c.Next()
	}
}

// SelfOrStaff allows staff through and restricts members to their own record,
// comparing the given route param against the token subject.
func SelfOrStaff(param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if GetKind(c) == jwt.KindStaff {
			c.Next()
			return
		}

		id := c.Param(param)
		if id != "" && id == GetUserID(c) {
			c.Next()
			return
		}

		response.Forbidden(c, "Access denied")
		c.Abort()
	}
}

func PermissionRequired(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		perms, exists := c.Get(ContextKeyPermissions)
		if !exists {
			response.Forbidden(c, "No permissions data")
			c.Abort()
			return
		}

		permList, ok := perms.([]string)
		if !ok {
			response.Forbidden(c, "Invalid permissions data")
			c.Abort()
			return
		}

		hasPermission := false
		for _, p := range permList {
			if p == permission || p == "*" {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			response.Forbidden(c, "Insufficient permissions")
			c.Abort()
			return
		}

		c.Next()
	}
}

func GetUserID(c *gin.Context) string {
	if v, exists := c.Get(ContextKeyUserID); exists {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

func GetKind(c *gin.Context) string {
	if v, exists := c.Get(ContextKeyKind); exists {
		if kind, ok := v.(string); ok {
			return kind
		}
	}
	return ""
}

func GetDB(c *gin.Context) *gorm.DB {
	return database.GetDB().WithContext(c.Request.Context())
}

func extractToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		token := c.Query("token")
		if token != "" {
			return token
		}
		return ""
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return ""
	}

	return parts[1]
}
