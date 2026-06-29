package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type PaginatedData struct {
	Items    interface{} `json:"items"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

func SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: message,
		Data:    data,
	})
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Response{
		Code:    0,
		Message: "created",
		Data:    data,
	})
}

func Paginated(c *gin.Context, items interface{}, total int64, page, pageSize int) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: PaginatedData{
			Items:    items,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	})
}

func Error(c *gin.Context, httpCode int, message string) {
	c.JSON(httpCode, Response{
		Code:    httpCode,
		Message: message,
		Data:    nil,
	})
}

func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, message)
}

func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, message)
}

func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, message)
}

// TokenExpired returns 401 with code 9999, matching admin's VITE_SERVICE_EXPIRED_TOKEN_CODES
func TokenExpired(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, Response{
		Code:    9999,
		Message: "Token expired",
		Data:    nil,
	})
}

// ForceLogout returns 401 with code 8888, matching admin's VITE_SERVICE_LOGOUT_CODES
func ForceLogout(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, Response{
		Code:    8888,
		Message: "Session expired, please login again",
		Data:    nil,
	})
}

// SessionExpired returns 401 with code 7777, matching admin's VITE_SERVICE_MODAL_LOGOUT_CODES
func SessionExpired(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, Response{
		Code:    7777,
		Message: "Session expired",
		Data:    nil,
	})
}

func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, message)
}

func InternalError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, message)
}
