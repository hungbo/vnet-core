package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/service"
)

// newSeedServer dựng /seed/... thật trên một http.Server có WriteTimeout ngắn, phát một tệp `size` byte.
func newSeedServer(t *testing.T, writeTimeout time.Duration, size int) (url string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "Game"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "Game", "big.pak"), make([]byte, size), 0o644))

	db, mock := newTestDB(t)
	mock.MatchExpectationsInOrder(false)
	mock.ExpectQuery(`SELECT count\(\*\) FROM "games"`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	svc := service.NewGameService(db, config.GameConfig{Role: config.GameRoleMaster, CatalogKey: "0123456789abcdef"})
	r := gin.New()
	r.GET("/seed/:token/*filepath", NewGameHandler(svc).Seed(root))

	srv := httptest.NewUnstartedServer(r)
	srv.Config.WriteTimeout = writeTimeout
	srv.Start()
	t.Cleanup(srv.Close)
	return srv.URL + "/seed/" + svc.SeedToken() + "/Game/big.pak"
}

// readSlowly đọc thân phản hồi `chunk` byte mỗi `every`, như một quán WAN chậm.
func readSlowly(t *testing.T, url string, chunk int, every time.Duration) (int, error) {
	t.Helper()
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	buf := make([]byte, chunk)
	total := 0
	for {
		n, err := resp.Body.Read(buf)
		total += n
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return total, err
		}
		time.Sleep(every)
	}
}

// WriteTimeout của server là 30 giây cho cả phản hồi: một quán đọc chậm bị cắt giữa tệp. Ở đây
// WriteTimeout 300 ms, client mất ~2 giây mới đọc hết 32 MB — vẫn phải nhận đủ.
func TestSeed_SlowClientIsNotCutByServerWriteTimeout(t *testing.T) {
	const size = 32 << 20
	url := newSeedServer(t, 300*time.Millisecond, size)

	got, err := readSlowly(t, url, 256<<10, 10*time.Millisecond)
	require.NoError(t, err, "phản hồi bị cắt sau %d/%d byte", got, size)
	assert.Equal(t, size, got)
}

// Gia hạn theo tiến độ không được thành vô hạn: kết nối đứng im vẫn bị cắt.
func TestSeed_StuckClientIsStillCut(t *testing.T) {
	orig := seedWriteIdle
	seedWriteIdle = 400 * time.Millisecond
	t.Cleanup(func() { seedWriteIdle = orig })
	const size = 32 << 20
	url := newSeedServer(t, 30*time.Second, size)

	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	first := make([]byte, 1024)
	_, err = io.ReadFull(resp.Body, first)
	require.NoError(t, err)
	time.Sleep(2 * time.Second) // không đọc nữa: bộ đệm đầy, ghi bị chặn quá seedWriteIdle

	rest, err := io.Copy(io.Discard, resp.Body)
	assert.Error(t, err, "server phải bỏ kết nối đứng im")
	assert.Less(t, rest+int64(len(first)), int64(size))
}

// id không phải UUID: 404 sạch, không chạm DB (PostgreSQL sẽ ném lỗi thô về kiểu uuid).
func TestGameHandler_IdKhongPhaiUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, c := range []struct{ role, method, path string }{
		{config.GameRoleMaster, "PUT", "/games/abc"},
		{config.GameRoleMaster, "DELETE", "/games/abc"},
		{config.GameRoleMaster, "POST", "/games/abc/publish"},
		{config.GameRoleCafe, "PUT", "/games/abc"},
		{config.GameRoleCafe, "POST", "/games/abc/sync"},
	} {
		db, mock := newTestDB(t)
		h := NewGameHandler(service.NewGameService(db, config.GameConfig{Role: c.role}))
		r := gin.New()
		r.PUT("/games/:id", h.Update)
		r.DELETE("/games/:id", h.Delete)
		r.POST("/games/:id/publish", h.Publish)
		r.POST("/games/:id/sync", h.Sync)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(c.method, c.path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		label := c.role + " " + c.method + " " + c.path
		assert.Equal(t, http.StatusNotFound, w.Code, label)
		assert.Contains(t, w.Body.String(), "không tìm thấy game", label)
		assert.NotContains(t, w.Body.String(), "SQLSTATE", label)
		assert.NoError(t, mock.ExpectationsWereMet(), label+": không được có truy vấn nào")
	}
}
