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
	ContextKeyStoreID     = "store_id"
	ContextKeyUsername    = "username"
	ContextKeyRole        = "role"
	ContextKeyRoleID      = "role_id"
	ContextKeyPermissions = "permissions"
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

		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyStoreID, claims.StoreID)
		c.Set(ContextKeyUsername, claims.Username)
		c.Set(ContextKeyRole, claims.Role)
		c.Set(ContextKeyRoleID, claims.RoleID)
		c.Set(ContextKeyPermissions, claims.Permissions)

		c.Next()
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

func StoreContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		storeID := c.GetHeader("X-Store-ID")
		if storeID != "" {
			c.Set("store_id", storeID)
		}

		if _, exists := c.Get("store_id"); !exists {
			c.Set("store_id", "")
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

func GetStoreID(c *gin.Context) string {
	if v, exists := c.Get(ContextKeyStoreID); exists {
		if id, ok := v.(string); ok {
			return id
		}
	}
	if v, exists := c.Get("store_id"); exists {
		if id, ok := v.(string); ok {
			return id
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
