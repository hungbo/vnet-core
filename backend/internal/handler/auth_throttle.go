package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/response"
)

const (
	// Mỗi cặp (tên đăng nhập, IP) được thử tối đa loginMaxAttempts lần trong
	// loginWindow. Khoá theo CẢ HAI: chỉ theo tên thì ai đó gõ sai mật khẩu của
	// admin từ máy khác là khoá luôn chủ quán; chỉ theo IP thì một máy dò được
	// mọi tài khoản. Cửa sổ ngắn nên khoá nhầm cũng tự hết sau vài phút.
	loginMaxAttempts = 10
	loginWindow      = 5 * time.Minute

	// Số khoá giữ tối đa: kẻ dò đổi tên mỗi lượt thì mỗi lượt là một khoá mới, và
	// mỗi khoá nằm lại ít nhất một cửa sổ. Khi đầy, khoá mới đẩy một khoá CHƯA bị
	// chặn ra ngoài (khoá đang chặn là thứ đáng giữ); chỉ khi toàn khoá đang chặn
	// mới để bản đồ lớn thêm — mỗi khoá như vậy đã tốn kẻ dò đủ 10 lượt.
	loginMaxKeys = 10000
)

// loginThrottle chặn dò mật khẩu trong bộ nhớ. Máy chủ là một tiến trình duy
// nhất nên không cần kho dùng chung; khởi động lại thì bộ đếm về 0, chấp nhận được.
type loginThrottle struct {
	mu        sync.Mutex
	attempts  map[string][]time.Time // mốc thời gian các lượt thử còn trong cửa sổ, cũ → mới
	lastSweep time.Time
	now       func() time.Time
}

func newLoginThrottle() *loginThrottle {
	return &loginThrottle{attempts: map[string][]time.Time{}, now: time.Now}
}

// begin ghi nhận một lượt thử NGAY TRƯỚC khi kiểm mật khẩu, hoặc từ chối kèm
// thời gian phải chờ. Ghi trước chứ không ghi sau khi thất bại: 300 yêu cầu song
// song đều kiểm "chưa đủ 10 lần" trước khi yêu cầu nào kịp ghi thì cả 300 lọt qua.
func (t *loginThrottle) begin(key string) (wait time.Duration, ok bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.now()
	t.sweep(now)

	recent, tracked := t.attempts[key]
	if !tracked && len(t.attempts) >= loginMaxKeys {
		for k, times := range t.attempts {
			if len(times) < loginMaxAttempts {
				delete(t.attempts, k)
				break
			}
		}
	}
	for len(recent) > 0 && now.Sub(recent[0]) >= loginWindow {
		recent = recent[1:]
	}
	if len(recent) >= loginMaxAttempts {
		t.attempts[key] = recent
		// Lượt bị từ chối KHÔNG được ghi: bị chặn rồi mà vẫn bấm tiếp không làm
		// thời gian chờ dài thêm.
		return recent[0].Add(loginWindow).Sub(now), false
	}
	t.attempts[key] = append(recent, now)
	return 0, true
}

// clear xoá bộ đếm của một khoá: đăng nhập đúng thì những lần gõ sai trước đó
// không còn tính.
func (t *loginThrottle) clear(key string) {
	t.mu.Lock()
	delete(t.attempts, key)
	t.mu.Unlock()
}

// sweep gỡ những khoá đã im quá một cửa sổ, nếu không bản đồ phình mãi theo số
// tên đăng nhập mà kẻ dò đã thử. Gọi khi đang giữ khoá.
func (t *loginThrottle) sweep(now time.Time) {
	if now.Sub(t.lastSweep) < time.Minute {
		return
	}
	t.lastSweep = now
	for key, times := range t.attempts {
		if len(times) == 0 || now.Sub(times[len(times)-1]) >= loginWindow {
			delete(t.attempts, key)
		}
	}
}

// loginKey là khoá của một cặp (tên đăng nhập, IP). Tên được BĂM chứ không giữ
// nguyên: nó do người gọi chọn và không bị giới hạn độ dài, mà khoá nằm lại trong
// bộ nhớ ít nhất một cửa sổ — mỗi tên dài vài MB là máy chủ phải giữ vài MB suốt
// 5 phút. Máy chủ quán còn chạy cả PostgreSQL nên hết bộ nhớ là hết cả quán.
func loginKey(username, ip string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(username))))
	return hex.EncodeToString(sum[:16]) + "|" + ip
}

// allowLogin trả khoá của lượt đăng nhập này và true nếu được thử. Hết lượt thì
// đã trả sẵn 429 kèm Retry-After, handler chỉ việc return.
//
// IP lấy từ RemoteIP (địa chỉ kết nối thật) chứ không phải ClientIP: gin mặc định
// tin X-Forwarded-For của mọi người gửi, kẻ dò chỉ cần đổi header mỗi lượt là có
// khoá mới.
func (h *AuthHandler) allowLogin(c *gin.Context, username string) (string, bool) {
	key := loginKey(username, c.RemoteIP())
	wait, ok := h.throttle.begin(key)
	if ok {
		return key, true
	}
	c.Header("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
	response.Error(c, http.StatusTooManyRequests,
		fmt.Sprintf("Đăng nhập sai quá nhiều lần — thử lại sau %d phút", int(math.Ceil(wait.Minutes()))))
	return key, false
}

// settleLogin chốt kết quả một lượt đã được allowLogin cho phép. Chỉ lỗi "sai tên
// hoặc mật khẩu" giữ lượt trong bộ đếm; mật khẩu đúng (kể cả khi sau đó bị từ
// chối vì lý do khác, như khách hết tiền) thì xoá, để khách nạp tiền xong đăng
// nhập lại không dính 429.
func (h *AuthHandler) settleLogin(key string, err error) {
	if !errors.Is(err, service.ErrBadCredentials) {
		h.throttle.clear(key)
	}
}
