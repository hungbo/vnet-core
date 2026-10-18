package middleware

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCORS_WildcardOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(CORS([]string{"*"}))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := performRequest(router.ServeHTTP, "GET", "/test", map[string]string{
		"Origin": "http://example.com",
	})

	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_AllowedOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(CORS([]string{"http://localhost:3000"}))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := performRequest(router.ServeHTTP, "GET", "/test", map[string]string{
		"Origin": "http://localhost:3000",
	})

	assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_DeniedOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(CORS([]string{"http://localhost:3000"}))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := performRequest(router.ServeHTTP, "GET", "/test", map[string]string{
		"Origin": "http://evil.com",
	})

	assert.Equal(t, "", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_OPTIONS_Preflight(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(CORS([]string{"*"}))
	router.OPTIONS("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := performRequest(router.ServeHTTP, "OPTIONS", "/test", map[string]string{
		"Origin": "http://example.com",
	})

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestCORS_SetsStandardHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(CORS([]string{"http://example.com"}))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := performRequest(router.ServeHTTP, "GET", "/test", map[string]string{
		"Origin": "http://example.com",
	})

	assert.Equal(t, "GET, POST, PUT, PATCH, DELETE, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Authorization")
	assert.Equal(t, "86400", w.Header().Get("Access-Control-Max-Age"))
}
