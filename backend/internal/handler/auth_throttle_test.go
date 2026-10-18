package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/utils"
	"gorm.io/gorm"
)

// --- bộ đếm -----------------------------------------------------------------

func newClockedThrottle() (*loginThrottle, *time.Time) {
	now := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	th := newLoginThrottle()
	th.now = func() time.Time { return now }
	return th, &now
}

func TestLoginThrottle_ChanSauMuoiLuotVaTuHetSauCuaSo(t *testing.T) {
	th, now := newClockedThrottle()

	for i := 0; i < loginMaxAttempts; i++ {
		_, ok := th.begin("admin|10.0.0.5")
		require.True(t, ok, "lượt %d phải được thử", i+1)
		*now = now.Add(time.Second)
	}

	wait, ok := th.begin("admin|10.0.0.5")
	assert.False(t, ok, "lượt thứ 11 trong cửa sổ phải bị chặn")
	// Lượt cũ nhất là 10 giây trước, nên còn phải chờ cửa sổ trừ 10 giây.
	assert.Equal(t, loginWindow-10*time.Second, wait)

	*now = now.Add(wait)
	_, ok = th.begin("admin|10.0.0.5")
	assert.True(t, ok, "hết thời gian chờ thì phải thử lại được")
}

// Bị chặn rồi mà vẫn bấm tiếp không được làm dài thời gian chờ — nếu không, kẻ
// dò chỉ cần giữ nút là khoá chủ quán mãi mãi.
func TestLoginThrottle_LuotBiChanKhongKeoDaiThoiGianCho(t *testing.T) {
	th, now := newClockedThrottle()
	for i := 0; i < loginMaxAttempts; i++ {
		th.begin("admin|10.0.0.5")
	}
	first, _ := th.begin("admin|10.0.0.5")

	*now = now.Add(4 * time.Minute)
	for i := 0; i < 50; i++ {
		th.begin("admin|10.0.0.5")
	}
	later, ok := th.begin("admin|10.0.0.5")

	assert.False(t, ok)
	assert.Equal(t, first-4*time.Minute, later)
}

func TestLoginThrottle_MoiKhoaMotBoDem(t *testing.T) {
	th, _ := newClockedThrottle()
	for i := 0; i < loginMaxAttempts; i++ {
		th.begin("admin|10.0.0.5")
	}

	_, ok := th.begin("admin|10.0.0.6")
	assert.True(t, ok, "cùng tên nhưng IP khác không bị liên luỵ")
	_, ok = th.begin("staff|10.0.0.5")
	assert.True(t, ok, "cùng IP nhưng tên khác không bị liên luỵ")
}

func TestLoginThrottle_ClearXoaBoDem(t *testing.T) {
	th, _ := newClockedThrottle()
	for i := 0; i < loginMaxAttempts; i++ {
		th.begin("admin|10.0.0.5")
	}

	th.clear("admin|10.0.0.5")

	_, ok := th.begin("admin|10.0.0.5")
	assert.True(t, ok)
}

// Bản đồ không được phình mãi theo số tên mà kẻ dò đã thử.
func TestLoginThrottle_DonKhoaDaIm(t *testing.T) {
	th, now := newClockedThrottle()
	for i := 0; i < 100; i++ {
		th.begin("ten" + strconv.Itoa(i) + "|10.0.0.5")
	}
	require.Len(t, th.attempts, 100)

	*now = now.Add(loginWindow + 2*time.Minute)
	th.begin("moi|10.0.0.5")

	assert.Len(t, th.attempts, 1, "chỉ còn khoá vừa thử")
}

// Khoá không được lớn theo độ dài tên: tên do người gọi chọn, mà khoá nằm lại cả
// cửa sổ 5 phút. Đo được trên bản giữ nguyên tên: 1500 lượt với tên 1 MB làm máy
// chủ giữ ~2 GB.
func TestLoginKey_CoDoDaiCoDinhVaVanGopHoaThuong(t *testing.T) {
	assert.Equal(t, loginKey("admin", "10.0.0.5"), loginKey("  ADMIN ", "10.0.0.5"), "hoa/thường và khoảng trắng vẫn chung một khoá")
	assert.NotEqual(t, loginKey("admin", "10.0.0.5"), loginKey("admin", "10.0.0.6"))
	assert.NotEqual(t, loginKey("admin", "10.0.0.5"), loginKey("staff", "10.0.0.5"))

	long := loginKey(strings.Repeat("a", 1<<20), "10.0.0.5")
	assert.LessOrEqual(t, len(long), 128, "tên 1 MB vẫn ra khoá ngắn")
}

func TestAuthHandler_Login_TenRatDaiKhongLamBoDemPhinh(t *testing.T) {
	db, mock := newTestDB(t)
	handler, _ := setupAuthHandler(db)
	router := newLoginRouter(handler)

	huge := strings.Repeat("x", 1<<20)
	mock.ExpectQuery(usersQuery).WithArgs(huge, 1).WillReturnError(gorm.ErrRecordNotFound)
	w := postAs(router, "/api/auth/login", "192.0.2.10:5555", `{"username":"`+huge+`","password":"doan-sai"}`)
	require.Equal(t, http.StatusUnauthorized, w.Code)

	require.Len(t, handler.throttle.attempts, 1, "lượt sai phải được đếm")
	for key := range handler.throttle.attempts {
		assert.LessOrEqual(t, len(key), 128, "khoá giữ trong bộ nhớ không được mang theo tên 1 MB")
	}
}

// Số khoá có trần. Kẻ dò đổi tên mỗi lượt không được làm bản đồ lớn mãi, và cũng không
// được dùng chính cách đó đẩy khoá đang chặn của mình ra ngoài để được thử tiếp.
func TestLoginThrottle_TranSoKhoaVaGiuKhoaDangChan(t *testing.T) {
	th, _ := newClockedThrottle()
	for i := 0; i < loginMaxAttempts; i++ {
		th.begin("admin|10.0.0.5")
	}
	_, ok := th.begin("admin|10.0.0.5")
	require.False(t, ok, "khoá đã chặn")

	for i := 0; i < 3*loginMaxKeys; i++ {
		th.begin("rac" + strconv.Itoa(i) + "|10.0.0.5")
	}

	assert.LessOrEqual(t, len(th.attempts), loginMaxKeys, "bản đồ không được vượt trần")
	_, ok = th.begin("admin|10.0.0.5")
	assert.False(t, ok, "khoá đang chặn không bị đẩy ra để kẻ dò được thử lại")
}

// 300 yêu cầu song song đều kiểm trước khi yêu cầu nào kịp ghi thì cả 300 lọt
// qua (đo được trên bản chưa có bộ chặn: 300/300 trả 401). begin phải vừa kiểm
// vừa ghi trong một lần giữ khoá.
func TestLoginThrottle_SongSongKhongVuotQuaGioiHan(t *testing.T) {
	th := newLoginThrottle()
	var allowed atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 300; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok := th.begin("admin|10.0.0.5"); ok {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(loginMaxAttempts), allowed.Load())
}

// Chỉ "sai tên hoặc mật khẩu" giữ lượt trong bộ đếm. Mật khẩu đúng mà bị từ chối
// vì lý do khác (khách hết tiền, máy chưa khai báo) thì xoá, để khách nạp tiền
// xong đăng nhập lại không dính 429.
func TestAuthHandler_SettleLogin_ChiGiuLuotKhiSaiThongTin(t *testing.T) {
	h := NewAuthHandler(nil)
	const key = "khach|10.0.0.9"

	for i := 0; i < loginMaxAttempts-1; i++ {
		h.throttle.begin(key)
	}
	h.settleLogin(key, service.ErrBadCredentials)
	_, ok := h.throttle.begin(key)
	require.True(t, ok)
	_, ok = h.throttle.begin(key)
	assert.False(t, ok, "sai thông tin thì lượt vẫn tính")

	h.settleLogin(key, errors.New("số dư không đủ"))
	_, ok = h.throttle.begin(key)
	assert.True(t, ok, "mật khẩu đúng nhưng bị từ chối vì lý do khác thì xoá bộ đếm")
}

// --- qua HTTP ---------------------------------------------------------------

const usersQuery = `SELECT \* FROM "users" WHERE username = \$1`

func postAs(router *gin.Engine, path, remoteAddr, body string, headers ...string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteAddr
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	router.ServeHTTP(w, req)
	return w
}

// expectUnknownStaff: tên không có trong users — mỗi lượt sai tốn đúng một truy vấn.
func expectUnknownStaff(mock sqlmock.Sqlmock, username string, times int) {
	for i := 0; i < times; i++ {
		mock.ExpectQuery(usersQuery).WithArgs(username, 1).WillReturnError(gorm.ErrRecordNotFound)
	}
}

func newLoginRouter(h *AuthHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/auth/login", h.Login)
	router.POST("/api/auth/client-login", h.ClientLogin)
	router.POST("/api/auth/member-login", h.MemberLogin)
	return router
}

const wrongLogin = `{"username":"admin","password":"doan-sai"}`

func TestAuthHandler_Login_Chan429SauMuoiLanSai(t *testing.T) {
	db, mock := newTestDB(t)
	handler, _ := setupAuthHandler(db)
	router := newLoginRouter(handler)

	expectUnknownStaff(mock, "admin", loginMaxAttempts)
	for i := 0; i < loginMaxAttempts; i++ {
		w := postAs(router, "/api/auth/login", "192.0.2.10:5555", wrongLogin)
		require.Equal(t, http.StatusUnauthorized, w.Code, "lượt %d", i+1)
	}

	w := postAs(router, "/api/auth/login", "192.0.2.10:5555", wrongLogin)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	var resp testResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp.Message, "thử lại sau")
	retry, err := strconv.Atoi(w.Header().Get("Retry-After"))
	require.NoError(t, err, "phải có header Retry-After là số giây")
	assert.InDelta(t, loginWindow.Seconds(), retry, 5)
	// Lượt thứ 11 không được chạm database — sqlmock sẽ báo nếu nó truy vấn thêm.
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Chủ quán không bị khoá vì kẻ dò: khoá theo cả IP, nên từ máy khác vẫn đăng
// nhập được, và tên khác từ cùng máy kẻ dò cũng không bị liên luỵ.
func TestAuthHandler_Login_KeDoKhongKhoaDuocChuQuanOMayKhac(t *testing.T) {
	db, mock := newTestDB(t)
	handler, _ := setupAuthHandler(db)
	router := newLoginRouter(handler)
	hash, _ := utils.HashPassword("mat-khau-that")

	expectUnknownStaff(mock, "admin", loginMaxAttempts)
	for i := 0; i < loginMaxAttempts; i++ {
		postAs(router, "/api/auth/login", "192.0.2.10:5555", wrongLogin)
	}
	require.Equal(t, http.StatusTooManyRequests, postAs(router, "/api/auth/login", "192.0.2.10:5555", wrongLogin).Code)

	// Chủ quán, máy khác.
	mock.ExpectQuery(usersQuery).WithArgs("admin", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password_hash", "is_active"}).AddRow("u1", "admin", hash, true))
	mock.ExpectQuery(`SELECT \* FROM "user_roles"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "role_id"}))
	w := postAs(router, "/api/auth/login", "192.0.2.77:4000", `{"username":"admin","password":"mat-khau-that"}`)
	assert.Equal(t, http.StatusOK, w.Code)

	// Máy kẻ dò, tên khác.
	expectUnknownStaff(mock, "staff", 1)
	w = postAs(router, "/api/auth/login", "192.0.2.10:5555", `{"username":"staff","password":"x"}`)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// Gin mặc định tin X-Forwarded-For của bất kỳ ai; nếu khoá theo ClientIP thì kẻ
// dò đổi header mỗi lượt là có khoá mới và bộ chặn vô dụng.
func TestAuthHandler_Login_DoiXForwardedForKhongNeTranhDuoc(t *testing.T) {
	db, mock := newTestDB(t)
	handler, _ := setupAuthHandler(db)
	router := newLoginRouter(handler)

	expectUnknownStaff(mock, "admin", loginMaxAttempts)
	for i := 0; i < loginMaxAttempts; i++ {
		postAs(router, "/api/auth/login", "192.0.2.10:5555", wrongLogin, "X-Forwarded-For", "203.0.113."+strconv.Itoa(i+1))
	}

	w := postAs(router, "/api/auth/login", "192.0.2.10:5555", wrongLogin, "X-Forwarded-For", "203.0.113.200")

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

// /auth/client-login với tên nhân viên đi thẳng vào Login, nên hai cửa dùng
// chung một bộ đếm. Chặn một cửa mà để cửa kia mở thì dò mật khẩu admin qua cửa còn lại.
func TestAuthHandler_ClientLogin_ChungBoDemVoiLogin(t *testing.T) {
	db, mock := newTestDB(t)
	handler, _ := setupAuthHandler(db)
	router := newLoginRouter(handler)

	// 5 lượt qua /login (1 truy vấn) + 5 lượt qua /client-login (tên không phải
	// nhân viên nên tra thêm bảng members).
	expectUnknownStaff(mock, "admin", 5)
	for i := 0; i < 5; i++ {
		mock.ExpectQuery(usersQuery).WithArgs("admin", 1).WillReturnError(gorm.ErrRecordNotFound)
		mock.ExpectQuery(`SELECT \* FROM "members"`).WillReturnError(gorm.ErrRecordNotFound)
	}
	for i := 0; i < 5; i++ {
		require.Equal(t, http.StatusUnauthorized, postAs(router, "/api/auth/login", "192.0.2.10:5555", wrongLogin).Code)
	}
	for i := 0; i < 5; i++ {
		require.Equal(t, http.StatusUnauthorized,
			postAs(router, "/api/auth/client-login", "192.0.2.10:5555", `{"username":"admin","password":"doan-sai","machine_code":"M1"}`).Code)
	}

	assert.Equal(t, http.StatusTooManyRequests, postAs(router, "/api/auth/login", "192.0.2.10:5555", wrongLogin).Code)
	assert.Equal(t, http.StatusTooManyRequests,
		postAs(router, "/api/auth/client-login", "192.0.2.10:5555", `{"username":"admin","password":"x","machine_code":"M1"}`).Code)
	assert.Equal(t, http.StatusTooManyRequests,
		postAs(router, "/api/auth/member-login", "192.0.2.10:5555", `{"username":"admin","password":"x","machine_code":"M1"}`).Code)
}

// Đăng nhập đúng xoá những lần gõ sai trước đó.
func TestAuthHandler_Login_DangNhapDungXoaBoDem(t *testing.T) {
	db, mock := newTestDB(t)
	handler, _ := setupAuthHandler(db)
	router := newLoginRouter(handler)
	hash, _ := utils.HashPassword("mat-khau-that")

	expectUnknownStaff(mock, "admin", loginMaxAttempts-1)
	for i := 0; i < loginMaxAttempts-1; i++ {
		postAs(router, "/api/auth/login", "192.0.2.10:5555", wrongLogin)
	}
	mock.ExpectQuery(usersQuery).WithArgs("admin", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password_hash", "is_active"}).AddRow("u1", "admin", hash, true))
	mock.ExpectQuery(`SELECT \* FROM "user_roles"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "role_id"}))
	require.Equal(t, http.StatusOK,
		postAs(router, "/api/auth/login", "192.0.2.10:5555", `{"username":"admin","password":"mat-khau-that"}`).Code)

	// Bộ đếm về 0: lại được gõ sai đủ số lần.
	expectUnknownStaff(mock, "admin", loginMaxAttempts)
	for i := 0; i < loginMaxAttempts; i++ {
		w := postAs(router, "/api/auth/login", "192.0.2.10:5555", wrongLogin)
		require.Equal(t, http.StatusUnauthorized, w.Code, "lượt %d sau khi đăng nhập đúng", i+1)
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Cờ buộc đổi mật khẩu đi cùng phản hồi đăng nhập, đúng tên trường mà giao diện
// và script đọc.
func TestAuthHandler_Login_PhanHoiCoMustChangePassword(t *testing.T) {
	db, mock := newTestDB(t)
	handler, _ := setupAuthHandler(db)
	router := newLoginRouter(handler)
	hash, _ := utils.HashPassword(service.DefaultSeedPassword)

	mock.ExpectQuery(usersQuery).WithArgs("admin", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password_hash", "is_active"}).AddRow("u1", "admin", hash, true))
	mock.ExpectQuery(`SELECT \* FROM "user_roles"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "role_id"}))

	w := postAs(router, "/api/auth/login", "192.0.2.10:5555", `{"username":"admin","password":"`+service.DefaultSeedPassword+`"}`)

	require.Equal(t, http.StatusOK, w.Code)
	var raw struct {
		Data struct {
			MustChangePassword bool `json:"must_change_password"`
			User               struct {
				MustChangePassword bool `json:"must_change_password"`
			} `json:"user"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
	assert.True(t, raw.Data.MustChangePassword)
	assert.True(t, raw.Data.User.MustChangePassword)
}
