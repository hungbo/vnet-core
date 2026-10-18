package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/vnet/core/pkg/jwt"
)

type testResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func performRequest(r http.HandlerFunc, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAuthRequired_NoToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jwtManager := jwt.New("test-secret", 1*time.Hour, 7*24*time.Hour, "test")

	router := gin.New()
	router.Use(AuthRequired(jwtManager))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := performRequest(router.ServeHTTP, "GET", "/test", nil)

	var resp testResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 8888, resp.Code)
}

func TestAuthRequired_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jwtManager := jwt.New("test-secret", 1*time.Hour, 7*24*time.Hour, "test")

	router := gin.New()
	router.Use(AuthRequired(jwtManager))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := performRequest(router.ServeHTTP, "GET", "/test", map[string]string{
		"Authorization": "Bearer invalid-token",
	})

	var resp testResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 9999, resp.Code)
}

func TestAuthRequired_ValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jwtManager := jwt.New("test-secret", 1*time.Hour, 7*24*time.Hour, "test")
	token, _ := jwtManager.GenerateAccessToken("u1", "admin", "admin", "r1", jwt.KindStaff, []string{"*"})

	router := gin.New()
	router.Use(AuthRequired(jwtManager))
	router.GET("/test", func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		username, _ := c.Get("username")
		role, _ := c.Get("role")
		c.JSON(http.StatusOK, gin.H{
			"user_id":  userID,
			"username": username,
			"role":     role,
		})
	})

	w := performRequest(router.ServeHTTP, "GET", "/test", map[string]string{
		"Authorization": "Bearer " + token,
	})

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	// Gin wraps response in response.Success format
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		// If it's a plain JSON response (not wrapped)
		assert.Equal(t, "u1", resp["user_id"])
	} else {
		assert.Equal(t, "u1", data["user_id"])
		assert.Equal(t, "admin", data["username"])
		assert.Equal(t, "admin", data["role"])
	}
}

func TestAuthRequired_NoBearerPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jwtManager := jwt.New("test-secret", 1*time.Hour, 7*24*time.Hour, "test")

	router := gin.New()
	router.Use(AuthRequired(jwtManager))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := performRequest(router.ServeHTTP, "GET", "/test", map[string]string{
		"Authorization": "Token justatoken",
	})

	var resp testResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 8888, resp.Code)
}

func TestAuthRequired_EmptyAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jwtManager := jwt.New("test-secret", 1*time.Hour, 7*24*time.Hour, "test")

	router := gin.New()
	router.Use(AuthRequired(jwtManager))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := performRequest(router.ServeHTTP, "GET", "/test", map[string]string{
		"Authorization": "",
	})

	var resp testResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 8888, resp.Code)
}

func TestPermissionRequired_HasPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		c.Set("permissions", []string{"order.create", "order.delete"})
	}, PermissionRequired("order.create"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := performRequest(router.ServeHTTP, "GET", "/test", nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPermissionRequired_MissingPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		c.Set("permissions", []string{"order.read"})
	}, PermissionRequired("order.delete"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := performRequest(router.ServeHTTP, "GET", "/test", nil)

	var resp testResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 403, resp.Code)
}

func TestPermissionRequired_WildcardPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		c.Set("permissions", []string{"*"})
	}, PermissionRequired("anything"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := performRequest(router.ServeHTTP, "GET", "/test", nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPermissionRequired_NoPermissionsData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/test", PermissionRequired("anything"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := performRequest(router.ServeHTTP, "GET", "/test", nil)

	var resp testResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 403, resp.Code)
}

// A refresh token is signed with the same key and claims type as an access
// token, so only the "typ" claim keeps it out of the protected routes.
func TestAuthRequired_RejectsRefreshToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jwtManager := jwt.New("test-secret", 1*time.Hour, 7*24*time.Hour, "test")
	refreshToken, _ := jwtManager.GenerateRefreshToken("u1", jwt.KindStaff)

	router := gin.New()
	router.Use(AuthRequired(jwtManager))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := performRequest(router.ServeHTTP, "GET", "/test", map[string]string{
		"Authorization": "Bearer " + refreshToken,
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp testResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 8888, resp.Code)
}

func staffOnlyRouter(t *testing.T, kind string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	jwtManager := jwt.New("test-secret", 1*time.Hour, 7*24*time.Hour, "test")
	token, _ := jwtManager.GenerateAccessToken("u1", "someone", "member", "", kind, []string{"member.access"})

	router := gin.New()
	router.Use(AuthRequired(jwtManager), StaffOnly())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	return performRequest(router.ServeHTTP, "GET", "/test", map[string]string{
		"Authorization": "Bearer " + token,
	})
}

func TestStaffOnly_RejectsMemberToken(t *testing.T) {
	w := staffOnlyRouter(t, jwt.KindMember)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestStaffOnly_AllowsStaffToken(t *testing.T) {
	w := staffOnlyRouter(t, jwt.KindStaff)
	assert.Equal(t, http.StatusOK, w.Code)
}

func selfOrStaffRequest(t *testing.T, kind, subjectID, requestedID string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	jwtManager := jwt.New("test-secret", 1*time.Hour, 7*24*time.Hour, "test")
	token, _ := jwtManager.GenerateAccessToken(subjectID, "someone", "member", "", kind, []string{"member.access"})

	router := gin.New()
	router.Use(AuthRequired(jwtManager))
	router.GET("/members/:id", SelfOrStaff("id"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	return performRequest(router.ServeHTTP, "GET", "/members/"+requestedID, map[string]string{
		"Authorization": "Bearer " + token,
	})
}

func TestSelfOrStaff_MemberCanReadOwnRecord(t *testing.T) {
	w := selfOrStaffRequest(t, jwt.KindMember, "member-1", "member-1")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSelfOrStaff_MemberCannotReadOtherRecord(t *testing.T) {
	w := selfOrStaffRequest(t, jwt.KindMember, "member-1", "member-2")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestSelfOrStaff_StaffCanReadAnyRecord(t *testing.T) {
	w := selfOrStaffRequest(t, jwt.KindStaff, "u1", "member-2")
	assert.Equal(t, http.StatusOK, w.Code)
}
