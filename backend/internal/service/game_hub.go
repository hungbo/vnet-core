package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"

	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/model"
	"gorm.io/gorm"
)

// Quán nối Hub (hoặc một master) ngay trên trang Game, không cần sửa config.env hay khởi động lại:
// địa chỉ, key và thư mục game lưu trong nhóm cài đặt "games". GAME_ROLE trong môi trường vẫn thắng —
// đặt rồi thì trang chỉ hiện, không đổi được.

const (
	// GameHubURLKey: địa chỉ vnet-server của Hub hoặc master (GAME_UPSTREAM_URL).
	GameHubURLKey = "hub_url"
	// GameHubKeyKey: key đọc catalog (GAME_CATALOG_KEY). Bí mật: isSecretSetting giấu khỏi GET /api/settings.
	GameHubKeyKey = "hub_key"
	// GameRootKey: thư mục game trên MÁY CHỦ (GAME_ROOT), khác client_root là đường dẫn máy trạm thấy.
	GameRootKey = "game_root"
)

// ErrGameHubEnv: vai đã đặt bằng biến môi trường nên không đổi ở trang quản trị.
var ErrGameHubEnv = errors.New("máy chủ này đặt vai cập nhật game bằng GAME_ROLE trong config.env, không đổi được ở trang Game")

// GameHubRequest là nội dung PUT /api/games/hub.
type GameHubRequest struct {
	URL string `json:"hub_url"`
	// Key trống nghĩa là giữ key đã lưu, và chỉ được khi địa chỉ không đổi: key không được tự gửi tới một địa chỉ mới.
	Key string `json:"hub_key"`
	// Root trống thì giữ thư mục đã lưu, rồi tới GAME_ROOT của môi trường.
	Root string `json:"game_root"`
}

// effectiveGameConfig: cấu hình có hiệu lực. Có GAME_ROLE thì lấy nguyên môi trường. Không có thì quán đã
// nối Hub từ trang Game chạy vai quán, các thông số còn lại (cổng, chu kỳ, thư mục engine) vẫn từ môi trường.
// Thiếu một trong ba giá trị hoặc giá trị đã lưu không còn hợp lệ là không có vai: không đoán.
func effectiveGameConfig(base config.GameConfig, st map[string]string) config.GameConfig {
	if base.Role != config.GameRoleOff {
		return base
	}
	root := st[GameRootKey]
	if root == "" {
		root = base.Root
	}
	if st[GameHubURLKey] == "" || st[GameHubKeyKey] == "" || root == "" {
		return base
	}
	c := base
	c.Role = config.GameRoleCafe
	c.UpstreamURL = strings.TrimRight(st[GameHubURLKey], "/")
	c.CatalogKey = st[GameHubKeyKey]
	c.Root = root
	if c.Validate() != nil {
		return base
	}
	return c
}

// conf là cấu hình đang chạy. Mọi chỗ đọc cấu hình game đều đi qua đây: nó đổi khi quán nối hay ngắt Hub.
func (s *GameService) conf() config.GameConfig {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	if s.live != nil {
		return *s.live
	}
	return s.cfg
}

func (s *GameService) setConf(c config.GameConfig) {
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()
	if c == s.cfg {
		s.live = nil
		return
	}
	s.live = &c
}

func (s *GameService) setSupervised(v bool) {
	s.cfgMu.Lock()
	s.supervised = v
	s.cfgMu.Unlock()
}

func (s *GameService) effectiveConf() config.GameConfig {
	if s.cfg.Role != config.GameRoleOff {
		return s.cfg
	}
	return effectiveGameConfig(s.cfg, settingsGroup(s.db, GamesGroup))
}

// Run chạy cho tới khi ctx kết thúc. Không làm gì khi chưa có vai (GAME_ROLE trống và quán chưa nối Hub);
// nối Hub ở trang Game thì bật vai quán, đổi hay ngắt thì dừng hẳn vai cũ rồi mới bật lại theo cấu hình mới,
// nên không lúc nào hai cấu hình chạy chung.
func (s *GameService) Run(ctx context.Context) {
	s.setSupervised(true)
	defer s.setSupervised(false)
	for ctx.Err() == nil {
		s.setConf(s.effectiveConf())
		stop := func() {}
		var done chan struct{}
		if s.conf().Role != config.GameRoleOff {
			var runCtx context.Context
			runCtx, stop = context.WithCancel(ctx)
			done = make(chan struct{})
			go func() {
				defer close(done)
				s.runRole(runCtx)
			}()
		}
		acks := s.waitReconf(ctx)
		stop()
		if done != nil {
			<-done
			if ctx.Err() == nil {
				s.recoverDownloading() // lượt tải bị cắt vì đổi cấu hình: dọn như sau một lần khởi động lại
			}
		}
		// Bên yêu cầu (SetHub, ClearHub) chờ tới đây để thấy vai mới ngay sau khi lưu.
		s.setConf(s.effectiveConf())
		for _, a := range acks {
			close(a)
		}
	}
}

// waitReconf chờ tới khi ctx kết thúc hoặc có yêu cầu làm cho cấu hình có hiệu lực khác cấu hình đang chạy.
// Trả về các kênh xác nhận của những yêu cầu đã nhận, để Run đóng sau khi áp xong.
func (s *GameService) waitReconf(ctx context.Context) []chan struct{} {
	var acks []chan struct{}
	for {
		select {
		case <-ctx.Done():
			return acks
		case a := <-s.reconf:
			if s.effectiveConf() == s.conf() {
				close(a) // không có gì đổi: không dừng vai đang chạy
				continue
			}
			return append(acks, a)
		}
	}
}

// reconfigure báo vòng nền áp cấu hình mới và chờ tới khi xong. Vòng nền không chạy (hoặc dừng lâu hơn hạn
// chờ) thì trả về sớm: cấu hình đã nằm trong DB nên lần Run sau vẫn dùng đúng.
func (s *GameService) reconfigure() {
	s.cfgMu.RLock()
	supervised := s.supervised
	s.cfgMu.RUnlock()
	if !supervised {
		return
	}
	ack := make(chan struct{})
	select {
	case s.reconf <- ack:
	default:
		return
	}
	select {
	case <-ack:
	case <-time.After(15 * time.Second):
	}
}

// SetHub kiểm tra rồi lưu kết nối Hub của quán. Kiểm tra bằng cách đọc catalog thật với key này: key sai,
// hết hạn hay địa chỉ không phải Hub báo ngay lúc dán, không phải đợi vòng nền ghi vào nhật ký.
func (s *GameService) SetHub(ctx context.Context, req GameHubRequest) (*GameInfo, error) {
	if s.cfg.Role != config.GameRoleOff {
		return nil, ErrGameHubEnv
	}
	st := settingsGroup(s.db, GamesGroup)
	base, err := chuanHoaHubURL(req.URL)
	if err != nil {
		return nil, err
	}
	key := strings.TrimSpace(req.Key)
	if key == "" && base == strings.TrimRight(st[GameHubURLKey], "/") {
		key = st[GameHubKeyKey]
	}
	if key == "" {
		return nil, errors.New("chưa có key: dán key do Hub cấp")
	}
	if len(key) < 16 || strings.IndexFunc(key, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return nil, errors.New("key không hợp lệ: dán đúng chuỗi key, không có khoảng trắng")
	}
	root := strings.TrimSpace(req.Root)
	if root == "" {
		root = st[GameRootKey]
	}
	if root == "" {
		root = s.cfg.Root
	}
	// Lưu đường dẫn đã giải symlink: thư mục game sau này bị thay bằng một liên kết trỏ chỗ khác thì
	// vẫn ghi vào đúng chỗ đã kiểm.
	root, err = kiemTraGameRoot(root, s.cfg.RootAllowed)
	if err != nil {
		return nil, err
	}
	if err := s.checkHub(ctx, base, key); err != nil {
		return nil, err
	}
	if err := saveGameSettings(s.db, map[string]string{GameHubURLKey: base, GameHubKeyKey: key, GameRootKey: root}); err != nil {
		return nil, err
	}
	s.reconfigure()
	info := s.Info()
	return &info, nil
}

// ClearHub ngắt kết nối: xoá địa chỉ và key. Game đã tải và thư mục game giữ nguyên.
func (s *GameService) ClearHub() (*GameInfo, error) {
	if s.cfg.Role != config.GameRoleOff {
		return nil, ErrGameHubEnv
	}
	if err := saveGameSettings(s.db, map[string]string{GameHubURLKey: "", GameHubKeyKey: ""}); err != nil {
		return nil, err
	}
	s.reconfigure()
	info := s.Info()
	return &info, nil
}

// saveGameSettings ghi thẳng vào nhóm "games", không qua SettingsService: key là bí mật và đường API chung
// từ chối ghi các khoá này (kiemTraCaiDatGame).
func saveGameSettings(db *gorm.DB, kv map[string]string) error {
	for k, v := range kv {
		raw, err := json.Marshal(v)
		if err != nil {
			return err
		}
		var existing model.SystemSetting
		err = db.Where("group_name = ? AND key = ?", GamesGroup, k).First(&existing).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			err = db.Create(&model.SystemSetting{GroupName: GamesGroup, Key: k, Value: string(raw)}).Error
		case err == nil:
			err = db.Model(&existing).Update("value", string(raw)).Error
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// chuanHoaHubURL: chỉ nhận http hoặc https có tên máy, bỏ dấu "/" cuối.
func chuanHoaHubURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("chưa nhập địa chỉ Hub")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return "", errors.New("địa chỉ Hub phải dạng http://máy-chủ:cổng hoặc https://tên-miền")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("địa chỉ Hub không có dấu ? hay #")
	}
	return strings.TrimRight(u.String(), "/"), nil
}

// kiemTraGameRoot: thư mục game trên máy chủ phải là đường dẫn tuyệt đối đã có sẵn. Không tự tạo: gõ nhầm
// chữ ổ mà tự tạo thì thư mục game nằm nhầm ổ mà không ai hay.
//
// Đường dẫn này do người dùng trang quản trị gõ, rồi máy chủ GHI các game tải về vào đó, nên ngoài chuyện có
// sẵn còn phải an toàn: đã giải symlink (liên kết trỏ vào /etc cũng không lọt), không phải gốc ổ đĩa, không
// trùng/nằm trong/chứa thư mục hệ thống, không chứa thư mục cài máy chủ. allowed (GAME_ROOT_ALLOWED) khác rỗng
// thì còn phải nằm trong một thư mục của danh sách đó — quán muốn khoá cứng thì đặt biến này.
// Trả về đường dẫn đã giải symlink để lưu.
func kiemTraGameRoot(root, allowed string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", errors.New("chưa nhập thư mục game trên máy chủ")
	}
	if !filepath.IsAbs(root) {
		return "", errors.New(`thư mục game trên máy chủ phải là đường dẫn đầy đủ, ví dụ D:\Games`)
	}
	real, err := filepath.EvalSymlinks(filepath.Clean(root))
	if err != nil {
		return "", fmt.Errorf("thư mục game %q chưa có trên máy chủ này: tạo thư mục trước rồi dán lại", root)
	}
	if fi, err := os.Stat(real); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("thư mục game %q chưa có trên máy chủ này: tạo thư mục trước rồi dán lại", root)
	}
	if filepath.Dir(real) == real {
		return "", errors.New(`thư mục game không được là gốc ổ đĩa: chọn một thư mục riêng, ví dụ D:\Games`)
	}
	for _, p := range thuMucHeThong() {
		if trongHoacChua(real, p) {
			return "", fmt.Errorf("thư mục game không được trùng, nằm trong hay chứa thư mục hệ thống %s", p)
		}
	}
	for _, p := range thuMucCuaMay() {
		if trong(p, real) { // trùng hay chứa nơi cài máy chủ: game đặt tên trùng là đè vào đó
			return "", fmt.Errorf("thư mục game không được trùng hay chứa thư mục cài/chạy của máy chủ (%s)", p)
		}
	}
	if allowed != "" && !trongThuMucChoPhep(real, allowed) {
		return "", errors.New("thư mục game phải nằm trong các thư mục quản trị máy chủ cho phép (GAME_ROOT_ALLOWED)")
	}
	return real, nil
}

// thuMucHeThong: nơi game không bao giờ được ghi vào, kể cả nằm bên trong.
func thuMucHeThong() []string {
	var ds []string
	if runtime.GOOS == "windows" {
		for _, e := range []string{"SystemRoot", "ProgramFiles", "ProgramFiles(x86)", "ProgramW6432", "ProgramData"} {
			ds = append(ds, os.Getenv(e))
		}
	} else {
		ds = []string{"/bin", "/boot", "/dev", "/etc", "/lib", "/lib32", "/lib64", "/proc", "/root", "/run", "/sbin", "/sys", "/usr",
			"/System", "/Library", "/private/etc"}
	}
	return daGiaiDuongDan(ds)
}

// thuMucCuaMay: nơi cài và nơi chạy của chính máy chủ. Thư mục game nằm BÊN TRONG thì không sao, nhưng không
// được trùng hay chứa chúng: thư mục game con tên "vnet" sẽ đè lên nơi cài.
func thuMucCuaMay() []string {
	var ds []string
	if exe, err := os.Executable(); err == nil {
		ds = append(ds, filepath.Dir(exe))
	}
	if wd, err := os.Getwd(); err == nil {
		ds = append(ds, wd)
	}
	return daGiaiDuongDan(ds)
}

// daGiaiDuongDan làm sạch và giải symlink từng đường dẫn (bỏ qua chỗ trống); chỗ không giải được giữ nguyên.
func daGiaiDuongDan(ds []string) []string {
	out := make([]string, 0, len(ds))
	for _, d := range ds {
		if d == "" {
			continue
		}
		if r, err := filepath.EvalSymlinks(d); err == nil {
			d = r
		}
		out = append(out, filepath.Clean(d))
	}
	return out
}

// trong báo child bằng parent hoặc nằm bên trong parent. Windows không phân biệt hoa thường: filepath.Rel
// đã so sánh theo kiểu đó ở đấy.
func trong(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// trongHoacChua: hai đường dẫn trùng nhau, hoặc cái này nằm trong cái kia.
func trongHoacChua(a, b string) bool { return trong(a, b) || trong(b, a) }

// trongThuMucChoPhep: dsach là danh sách đường dẫn theo kiểu PATH của hệ điều hành (Windows ngăn bằng ";").
// Mục không phải đường dẫn tuyệt đối bị bỏ qua, nên danh sách hỏng là không cho gì — không phải cho tất.
func trongThuMucChoPhep(real, dsach string) bool {
	for _, a := range filepath.SplitList(dsach) {
		a = strings.TrimSpace(a)
		if a == "" || !filepath.IsAbs(a) {
			continue
		}
		if r, err := filepath.EvalSymlinks(a); err == nil {
			a = r
		}
		if trong(real, filepath.Clean(a)) {
			return true
		}
	}
	return false
}

// checkHub đọc catalog bằng key để biết key dùng được. Trang web tự dựng vẫn trả 200 cho đường lạ (admin
// nhúng trả index.html), nên 200 chưa đủ: phải đúng dạng {code, ...} của VNET.
func (s *GameService) checkHub(ctx context.Context, base, key string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/game-catalog", nil)
	if err != nil {
		return err
	}
	req.Header.Set(GameCatalogKeyHeader, key)
	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("không kết nối được tới %s: %w", base, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var env struct {
		Code    *int   `json:"code"`
		Message string `json:"message"`
	}
	parsed := json.Unmarshal(body, &env) == nil && env.Code != nil
	// Nội dung phản hồi KHÔNG đưa ngược về người gọi: địa chỉ do họ gõ, nên một dịch vụ nội bộ nào đó trả
	// JSON có "code" và "message" là họ đọc được nó qua ô lỗi. Chi tiết chỉ nằm trong nhật ký máy chủ.
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return errors.New("key bị Hub từ chối: key sai, hết hạn hoặc đã bị thu hồi")
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		return fmt.Errorf("%s chuyển hướng sang nơi khác (%d): nhập thẳng địa chỉ cuối cùng của Hub", base, resp.StatusCode)
	case !parsed:
		return fmt.Errorf("%s không phải Hub hay master của VNET (trả %d, không đúng định dạng)", base, resp.StatusCode)
	case resp.StatusCode != http.StatusOK || *env.Code != 0:
		log.Printf("[game] Hub %s trả %d, mã %d: %s", base, resp.StatusCode, *env.Code, truncateRunes(env.Message, 200))
		return fmt.Errorf("Hub trả lỗi (HTTP %d, mã %d): xem nhật ký máy chủ để biết chi tiết", resp.StatusCode, *env.Code)
	}
	return nil
}

// keyHint: vài ký tự cuối để quản trị viên nhận ra key đang lưu, không đủ để đoán key.
func keyHint(key string) string {
	r := []rune(key)
	if len(r) < 16 {
		return ""
	}
	return "…" + string(r[len(r)-4:])
}

// noteSync ghi kết quả lần đọc catalog gần nhất để trang Game hiện tình trạng kết nối.
func (s *GameService) noteSync(err error) {
	msg := ""
	if err != nil {
		msg = truncateRunes(err.Error(), 300)
	}
	s.mu.Lock()
	changed := s.syncAt.IsZero() || s.syncErr != msg
	s.syncAt = time.Now()
	s.syncErr = msg
	s.mu.Unlock()
	if changed { // key hết hạn, mất mạng, hay vừa nối được: trang Game cập nhật tình trạng ngay
		s.baoDoi("")
	}
}
