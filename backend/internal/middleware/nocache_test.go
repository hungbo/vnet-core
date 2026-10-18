package middleware

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestNoCache_SetsNoStore(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(NoCache())
	router.GET("/members", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 0})
	})

	w := performRequest(router.ServeHTTP, "GET", "/members", nil)

	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
}
