package handler

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/jwt"
	"github.com/vnet/core/pkg/utils"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type testResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func newTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn: mockDB,
	}), &gorm.Config{})
	require.NoError(t, err)
	return db, mock
}

func setupAuthHandler(db *gorm.DB) (*AuthHandler, *jwt.Manager) {
	jwtMgr := jwt.New("test-secret", 1*time.Hour, 7*24*time.Hour, "vnet-test")
	authSvc := service.NewAuthService(db, jwtMgr, service.NewAuditService(db))
	sessionSvc := service.NewSessionService(db, nil, service.NewAuditService(db))
	return NewAuthHandler(db, authSvc, sessionSvc), jwtMgr
}

func TestAuthHandler_Login_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, mock := newTestDB(t)
	handler, _ := setupAuthHandler(db)

	hash, _ := utils.HashPassword("password123")

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE username = \$1 ORDER BY "users"."id" LIMIT \$2`).
		WithArgs("admin", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password_hash", "is_active", "full_name"}).
			AddRow("u1", "admin", hash, true, "Admin User"))

	mock.ExpectQuery(`SELECT \* FROM "user_roles" WHERE "user_roles"."user_id" = \$1`).
		WithArgs("u1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "role_id"}))

	router := gin.New()
	router.POST("/api/auth/login", handler.Login)

	body := `{"username":"admin","password":"password123"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	var resp testResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, 200, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthHandler_Login_InvalidCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, mock := newTestDB(t)
	handler, _ := setupAuthHandler(db)

	hash, _ := utils.HashPassword("correctpass")

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE username = \$1 ORDER BY "users"."id" LIMIT \$2`).
		WithArgs("admin", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password_hash", "is_active"}).
			AddRow("u1", "admin", hash, true))

	mock.ExpectQuery(`SELECT \* FROM "user_roles" WHERE "user_roles"."user_id" = \$1`).
		WithArgs("u1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "role_id"}))

	router := gin.New()
	router.POST("/api/auth/login", handler.Login)

	body := `{"username":"admin","password":"wrongpass"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	var resp testResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 401, w.Code)
	assert.Contains(t, resp.Message, "invalid username or password")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthHandler_Login_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, mock := newTestDB(t)
	handler, _ := setupAuthHandler(db)

	router := gin.New()
	router.POST("/api/auth/login", handler.Login)

	body := `not-json`
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	var resp testResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, resp.Message, "JSON")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthHandler_Me_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, mock := newTestDB(t)
	handler, jwtMgr := setupAuthHandler(db)

	token, _ := jwtMgr.GenerateAccessToken("u1", "admin", "admin", "", []string{})

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1 ORDER BY "users"."id" LIMIT \$2`).
		WithArgs("u1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "full_name"}).
			AddRow("u1", "admin", "Admin User"))

	mock.ExpectQuery(`SELECT \* FROM "user_roles" WHERE "user_roles"."user_id" = \$1`).
		WithArgs("u1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "role_id"}))

	router := gin.New()
	router.Use(middleware.AuthRequired(jwtMgr))
	router.GET("/api/auth/me", handler.Me)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	var resp testResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 200, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthHandler_Me_Unauthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, mock := newTestDB(t)
	handler, jwtMgr := setupAuthHandler(db)

	router := gin.New()
	router.Use(middleware.AuthRequired(jwtMgr))
	router.GET("/api/auth/me", handler.Me)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	router.ServeHTTP(w, req)

	var resp testResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 8888, resp.Code) // ForceLogout
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthHandler_ChangePassword_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, mock := newTestDB(t)
	handler, jwtMgr := setupAuthHandler(db)

	token, _ := jwtMgr.GenerateAccessToken("u1", "admin", "admin", "", []string{})
	oldHash, _ := utils.HashPassword("oldpass")

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1 ORDER BY "users"."id" LIMIT \$2`).
		WithArgs("u1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "password_hash"}).AddRow("u1", oldHash))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	router := gin.New()
	router.Use(middleware.AuthRequired(jwtMgr))
	router.POST("/api/auth/change-password", handler.ChangePassword)

	body := `{"old_password":"oldpass","new_password":"newpass123"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/auth/change-password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	var resp testResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, 0, resp.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthHandler_GetPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, mock := newTestDB(t)
	handler, _ := setupAuthHandler(db)

	mock.ExpectQuery(`SELECT \* FROM "permissions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name"}).
			AddRow("p1", "order.create", "Create Order").
			AddRow("p2", "order.delete", "Delete Order"))

	router := gin.New()
	router.GET("/api/auth/permissions", handler.GetPermissions)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/auth/permissions", nil)
	router.ServeHTTP(w, req)

	var resp testResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, 0, resp.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}
