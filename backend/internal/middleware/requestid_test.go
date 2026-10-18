package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/pkg/response"
)

// newLoggedRouter dựng router theo đúng thứ tự trong cmd/server/main.go và gom
// mọi thứ gin ghi ra log vào một buffer.
func newLoggedRouter(t *testing.T) (*gin.Engine, *bytes.Buffer) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	var buf bytes.Buffer
	oldOut, oldErr := gin.DefaultWriter, gin.DefaultErrorWriter
	gin.DefaultWriter, gin.DefaultErrorWriter = &buf, &buf
	t.Cleanup(func() { gin.DefaultWriter, gin.DefaultErrorWriter = oldOut, oldErr })

	r := gin.New()
	r.Use(RequestID(), Logger(), Recovery(nil))
	return r, &buf
}

func TestRequestID_HeaderAndContext(t *testing.T) {
	r, _ := newLoggedRouter(t)
	var seen string
	r.GET("/x", func(c *gin.Context) {
		seen = c.GetString(response.RequestIDKey)
		c.Status(http.StatusOK)
	})

	w1 := performRequest(r.ServeHTTP, "GET", "/x", nil)
	id1 := w1.Header().Get("X-Request-ID")
	assert.Len(t, id1, 8)
	assert.Equal(t, id1, seen)

	w2 := performRequest(r.ServeHTTP, "GET", "/x", map[string]string{"X-Request-ID": "khach-tu-dat"})
	assert.NotEqual(t, id1, w2.Header().Get("X-Request-ID"))
	assert.NotEqual(t, "khach-tu-dat", w2.Header().Get("X-Request-ID"), "mã do server sinh, không lấy từ client")
}

func TestLogger_LogsCauseOfInternalError(t *testing.T) {
	r, buf := newLoggedRouter(t)
	r.GET("/members", func(c *gin.Context) {
		response.InternalError(c, "Failed to fetch members", errors.New("pq: deadlock detected"))
	})

	w := performRequest(r.ServeHTTP, "GET", "/members", nil)
	id := w.Header().Get("X-Request-ID")

	var body response.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "Failed to fetch members (mã lỗi: "+id+")", body.Message)
	assert.NotContains(t, body.Message, "deadlock", "lỗi gốc không gửi cho client")

	logged := buf.String()
	assert.Contains(t, logged, "ERROR ")
	assert.Contains(t, logged, "id="+id)
	assert.Contains(t, logged, "err: pq: deadlock detected")
}

func TestLogger_MasksTokenInQuery(t *testing.T) {
	r, buf := newLoggedRouter(t)
	r.GET("/ws", func(c *gin.Context) { c.Status(http.StatusOK) })

	performRequest(r.ServeHTTP, "GET", "/ws?type=admin&token=eyJhbGciOi.secret.sig&x=1", nil)
	performRequest(r.ServeHTTP, "GET", "/ws?token=eyJ2.secret2", nil)

	logged := buf.String()
	assert.NotContains(t, logged, "secret")
	assert.Contains(t, logged, "?type=admin&token=***&x=1")
	assert.Contains(t, logged, "?token=***")
}

func TestRecovery_PanicBecomesJSONWithRequestID(t *testing.T) {
	r, buf := newLoggedRouter(t)
	r.GET("/boom", func(c *gin.Context) { panic("boom") })

	w := performRequest(r.ServeHTTP, "GET", "/boom?token=secret", nil)
	id := w.Header().Get("X-Request-ID")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var body response.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "Lỗi hệ thống (mã lỗi: "+id+")", body.Message)

	logged := buf.String()
	assert.Contains(t, logged, "PANIC id="+id+" GET /boom: boom")
	assert.Contains(t, logged, "requestid_test.go", "phải có stack trace")
	assert.Contains(t, logged, "err: panic: boom", "dòng log truy cập vẫn có, kèm lỗi")
	assert.NotContains(t, logged, "secret")
}

// Mọi lỗi 500 phải tới nhật ký lỗi — panic kèm stack, lỗi handler kèm lỗi gốc;
// 4xx là lỗi của người dùng thì không.
func TestRecovery_ReportsServerErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldErr := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &bytes.Buffer{}
	t.Cleanup(func() { gin.DefaultErrorWriter = oldErr })

	type report struct{ kind, message, detail, id string }
	var got []report
	r := gin.New()
	r.Use(RequestID(), Recovery(func(c *gin.Context, kind, message, detail string) {
		got = append(got, report{kind, message, detail, c.GetString(response.RequestIDKey)})
	}))
	r.GET("/boom", func(c *gin.Context) { panic("boom") })
	r.GET("/db", func(c *gin.Context) {
		response.InternalError(c, "Failed to fetch members", errors.New("pq: deadlock detected"))
	})
	r.GET("/bad", func(c *gin.Context) { response.BadRequest(c, "số tiền phải lớn hơn 0") })

	w1 := performRequest(r.ServeHTTP, "GET", "/boom", nil)
	w2 := performRequest(r.ServeHTTP, "GET", "/db", nil)
	performRequest(r.ServeHTTP, "GET", "/bad", nil)

	require.Len(t, got, 2)
	assert.Equal(t, "panic", got[0].kind)
	assert.Equal(t, "boom", got[0].message)
	assert.Contains(t, got[0].detail, "requestid_test.go")
	assert.Equal(t, w1.Header().Get("X-Request-ID"), got[0].id)

	assert.Equal(t, "error", got[1].kind)
	assert.Equal(t, "pq: deadlock detected", got[1].message)
	assert.Equal(t, w2.Header().Get("X-Request-ID"), got[1].id)
}
