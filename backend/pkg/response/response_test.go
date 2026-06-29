package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func performRequest(handler gin.HandlerFunc) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	handler(c)
	return w
}

func parseResponse(w *httptest.ResponseRecorder) Response {
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp
}

func TestSuccess(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		Success(c, gin.H{"key": "value"})
	})

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResponse(w)
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "success", resp.Message)
	assert.Equal(t, "value", resp.Data.(map[string]interface{})["key"])
}

func TestSuccessWithMessage(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		SuccessWithMessage(c, "custom message", "data-payload")
	})

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResponse(w)
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "custom message", resp.Message)
	assert.Equal(t, "data-payload", resp.Data)
}

func TestCreated(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		Created(c, gin.H{"id": "123"})
	})

	assert.Equal(t, http.StatusCreated, w.Code)
	resp := parseResponse(w)
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "created", resp.Message)
}

func TestPaginated(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		Paginated(c, []string{"a", "b"}, int64(10), 1, 20)
	})

	assert.Equal(t, http.StatusOK, w.Code)
	resp := parseResponse(w)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, float64(10), data["total"])
	assert.Equal(t, float64(1), data["page"])
	assert.Equal(t, float64(20), data["page_size"])
}

func TestBadRequest(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		BadRequest(c, "invalid input")
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
	resp := parseResponse(w)
	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Equal(t, "invalid input", resp.Message)
}

func TestUnauthorized(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		Unauthorized(c, "invalid credentials")
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	resp := parseResponse(w)
	assert.Equal(t, http.StatusUnauthorized, resp.Code)
	assert.Equal(t, "invalid credentials", resp.Message)
}

func TestForbidden(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		Forbidden(c, "no permission")
	})

	assert.Equal(t, http.StatusForbidden, w.Code)
	resp := parseResponse(w)
	assert.Equal(t, http.StatusForbidden, resp.Code)
	assert.Equal(t, "no permission", resp.Message)
}

func TestNotFound(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		NotFound(c, "resource not found")
	})

	assert.Equal(t, http.StatusNotFound, w.Code)
	resp := parseResponse(w)
	assert.Equal(t, http.StatusNotFound, resp.Code)
	assert.Equal(t, "resource not found", resp.Message)
}

func TestInternalError(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		InternalError(c, "internal server error")
	})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	resp := parseResponse(w)
	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "internal server error", resp.Message)
}

func TestTokenExpired(t *testing.T) {
	w := performRequest(TokenExpired)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	resp := parseResponse(w)
	assert.Equal(t, 9999, resp.Code)
	assert.Equal(t, "Token expired", resp.Message)
}

func TestForceLogout(t *testing.T) {
	w := performRequest(ForceLogout)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	resp := parseResponse(w)
	assert.Equal(t, 8888, resp.Code)
	assert.Equal(t, "Session expired, please login again", resp.Message)
}

func TestSessionExpired(t *testing.T) {
	w := performRequest(SessionExpired)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	resp := parseResponse(w)
	assert.Equal(t, 7777, resp.Code)
	assert.Equal(t, "Session expired", resp.Message)
}

func TestError(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		Error(c, http.StatusTeapot, "custom error")
	})

	assert.Equal(t, http.StatusTeapot, w.Code)
	resp := parseResponse(w)
	assert.Equal(t, http.StatusTeapot, resp.Code)
	assert.Equal(t, "custom error", resp.Message)
	assert.Nil(t, resp.Data)
}

func TestNilData(t *testing.T) {
	w := performRequest(func(c *gin.Context) {
		Success(c, nil)
	})

	resp := parseResponse(w)
	assert.Nil(t, resp.Data)
}
