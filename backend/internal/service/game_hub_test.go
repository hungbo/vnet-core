package service

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/config"
)

const testHubKey = "VGK-ABCDE-FGHIJ-KLMNO-PQRST"

func TestEffectiveGameConfig(t *testing.T) {
	root := t.TempDir()
	base := config.GameConfig{ListenPort: 19999, PollInterval: 5 * time.Minute, StableFor: 10 * time.Minute, StallTimeout: 10 * time.Minute}
	full := map[string]string{GameHubURLKey: "http://hub.local:20800/", GameHubKeyKey: testHubKey, GameRootKey: root}

	t.Run("không cài gì thì không có vai", func(t *testing.T) {
		assert.Equal(t, base, effectiveGameConfig(base, nil))
	})
	t.Run("đủ ba giá trị thì chạy vai quán, giữ các thông số còn lại", func(t *testing.T) {
		got := effectiveGameConfig(base, full)
		assert.Equal(t, config.GameRoleCafe, got.Role)
		assert.Equal(t, "http://hub.local:20800", got.UpstreamURL, "bỏ dấu / cuối")
		assert.Equal(t, testHubKey, got.CatalogKey)
		assert.Equal(t, root, got.Root)
		assert.Equal(t, base.ListenPort, got.ListenPort)
		assert.Equal(t, base.PollInterval, got.PollInterval)
	})
	t.Run("thư mục lấy từ GAME_ROOT khi chưa lưu", func(t *testing.T) {
		b := base
		b.Root = root
		got := effectiveGameConfig(b, map[string]string{GameHubURLKey: "http://hub:1", GameHubKeyKey: testHubKey})
		assert.Equal(t, config.GameRoleCafe, got.Role)
		assert.Equal(t, root, got.Root)
	})
	t.Run("thiếu một giá trị là không có vai", func(t *testing.T) {
		for _, drop := range []string{GameHubURLKey, GameHubKeyKey, GameRootKey} {
			st := map[string]string{}
			for k, v := range full {
				if k != drop {
					st[k] = v
				}
			}
			assert.Equal(t, base, effectiveGameConfig(base, st), "thiếu "+drop)
		}
	})
	t.Run("giá trị đã lưu sai thì không có vai", func(t *testing.T) {
		short := map[string]string{GameHubURLKey: "http://hub:1", GameHubKeyKey: "ngan", GameRootKey: root}
		assert.Equal(t, base, effectiveGameConfig(base, short), "key ngắn hơn 16 ký tự")
		rel := map[string]string{GameHubURLKey: "http://hub:1", GameHubKeyKey: testHubKey, GameRootKey: "games"}
		assert.Equal(t, base, effectiveGameConfig(base, rel), "thư mục không tuyệt đối")
	})
	t.Run("GAME_ROLE trong môi trường luôn thắng", func(t *testing.T) {
		env := base
		env.Role, env.Root, env.UpstreamURL, env.CatalogKey = config.GameRoleCafe, root, "http://env:1", "envkey-envkey-envkey"
		assert.Equal(t, env, effectiveGameConfig(env, full))
		master := base
		master.Role = config.GameRoleMaster
		assert.Equal(t, master, effectiveGameConfig(master, full))
	})
}

func TestGameService_ConfDoiTheoCaiDat(t *testing.T) {
	svc := &GameService{cfg: config.GameConfig{}}
	assert.Equal(t, config.GameRoleOff, svc.conf().Role)
	live := config.GameConfig{Role: config.GameRoleCafe, Root: "/g", UpstreamURL: "http://hub", CatalogKey: testHubKey}
	svc.setConf(live)
	assert.Equal(t, live, svc.conf())
	svc.setConf(config.GameConfig{})
	assert.Equal(t, config.GameRoleOff, svc.conf().Role, "bằng cấu hình môi trường thì bỏ cấu hình riêng")
	assert.Nil(t, svc.live)
}

func TestChuanHoaHubURL(t *testing.T) {
	for in, want := range map[string]string{
		"http://192.168.1.10:20800":     "http://192.168.1.10:20800",
		"  https://hub.example.com/  ":  "https://hub.example.com",
		"https://hub.example.com/vnet/": "https://hub.example.com/vnet",
	} {
		got, err := chuanHoaHubURL(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got)
	}
	for _, bad := range []string{
		"", "   ", "hub.example.com", "ftp://hub", "file:///etc/passwd", "javascript:alert(1)",
		"http://", "http://user:pass@hub", "https://hub?x=1", "https://hub#a",
	} {
		_, err := chuanHoaHubURL(bad)
		assert.Error(t, err, bad)
	}
}

func TestKiemTraGameRoot(t *testing.T) {
	dir := t.TempDir()
	got, err := kiemTraGameRoot(dir, "")
	assert.NoError(t, err)
	want, _ := filepath.EvalSymlinks(dir)
	assert.Equal(t, want, got, "trả về đường dẫn đã giải symlink để lưu")

	for name, root := range map[string]string{
		"rỗng":                  "",
		"không tuyệt đối":       "Games",
		"chưa có thì không tạo": filepath.Join(dir, "chua-co"),
	} {
		_, err := kiemTraGameRoot(root, "")
		assert.Error(t, err, name)
	}
	f := filepath.Join(dir, "tep.txt")
	require.NoError(t, os.WriteFile(f, []byte("x"), 0o600))
	_, err = kiemTraGameRoot(f, "")
	assert.Error(t, err, "là tệp, không phải thư mục")
	_, err = os.Stat(filepath.Join(dir, "chua-co"))
	assert.True(t, os.IsNotExist(err), "không được tạo thư mục")
}

// Thư mục game do người dùng trang quản trị gõ rồi máy chủ GHI vào đó: những chỗ nguy hiểm phải bị chặn.
func TestKiemTraGameRoot_ChanChoNguyHiem(t *testing.T) {
	t.Run("gốc ổ đĩa", func(t *testing.T) {
		_, err := kiemTraGameRoot(filepath.VolumeName(t.TempDir())+string(os.PathSeparator), "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "gốc ổ đĩa")
	})
	t.Run("thư mục hệ thống", func(t *testing.T) {
		sys := "/etc"
		if runtime.GOOS == "windows" {
			sys = os.Getenv("SystemRoot")
		}
		_, err := kiemTraGameRoot(sys, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "hệ thống")
	})
	t.Run("bên trong thư mục hệ thống", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			_, err := kiemTraGameRoot(filepath.Join(os.Getenv("SystemRoot"), "System32"), "")
			assert.Error(t, err)
			return
		}
		_, err := kiemTraGameRoot("/etc/ssh", "")
		if _, statErr := os.Stat("/etc/ssh"); statErr == nil {
			assert.Error(t, err)
		}
	})
	t.Run("symlink trỏ vào thư mục hệ thống", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("tạo symlink trên Windows cần quyền riêng")
		}
		link := filepath.Join(t.TempDir(), "games")
		require.NoError(t, os.Symlink("/etc", link))
		_, err := kiemTraGameRoot(link, "")
		assert.Error(t, err, "liên kết trông vô hại nhưng trỏ vào /etc")
	})
	t.Run("chứa thư mục chạy của máy chủ", func(t *testing.T) {
		wd, err := os.Getwd()
		require.NoError(t, err)
		for _, root := range []string{wd, filepath.Dir(wd)} {
			_, err := kiemTraGameRoot(root, "")
			require.Error(t, err, root)
			assert.Contains(t, err.Error(), "cài/chạy của máy chủ")
		}
	})
}

func TestKiemTraGameRoot_DanhSachChoPhep(t *testing.T) {
	base := t.TempDir()
	trong := filepath.Join(base, "Games")
	ngoai := t.TempDir()
	require.NoError(t, os.Mkdir(trong, 0o755))
	list := base + string(os.PathListSeparator) + "tuong-doi"

	_, err := kiemTraGameRoot(trong, list)
	assert.NoError(t, err, "nằm trong thư mục được phép")
	_, err = kiemTraGameRoot(base, list)
	assert.NoError(t, err, "chính thư mục được phép cũng được")
	_, err = kiemTraGameRoot(ngoai, list)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GAME_ROOT_ALLOWED")

	// Danh sách hỏng thì không cho gì, không phải cho tất.
	_, err = kiemTraGameRoot(trong, "tuong-doi"+string(os.PathListSeparator)+" ")
	assert.Error(t, err, "mục không tuyệt đối bị bỏ qua, hết mục hợp lệ thì chặn")
}

func TestKeyHint(t *testing.T) {
	assert.Equal(t, "…QRST", keyHint(testHubKey))
	assert.Equal(t, "", keyHint(""))
	assert.Equal(t, "", keyHint("ngan"), "key ngắn không để lộ ký tự nào")
	assert.NotContains(t, keyHint(testHubKey), "ABCDE")
}

func TestIsSecretSetting_NhomGames(t *testing.T) {
	for _, k := range []string{GameHubKeyKey, GameHubURLKey, GameRootKey} {
		assert.True(t, isSecretSetting(GamesGroup, k), k)
	}
	assert.False(t, isSecretSetting(GamesGroup, GameUpdateWindowKey))
	assert.False(t, isSecretSetting(GamesGroup, GameClientRootKey))
	assert.True(t, isSecretSetting(ClientGroup, LocalAdminHashKey), "khoá cũ vẫn giấu")
	assert.False(t, isSecretSetting("khac", GameHubKeyKey))
}

// Đường cài đặt chung không được ghi key Hub: nó bỏ qua bước thử key và bước kiểm quyền games.hub.
func TestSettingsUpdate_ChanGhiThangKhoaHub(t *testing.T) {
	for _, k := range []string{GameHubKeyKey, GameHubURLKey, GameRootKey} {
		db, mock := newMockDB(t)
		svc := NewSettingsService(db, NewAuditService(db))
		_, err := svc.Update(GamesGroup, map[string]interface{}{k: "x"})
		require.Error(t, err, k)
		assert.Contains(t, err.Error(), "/api/games/hub")
		assert.NoError(t, mock.ExpectationsWereMet(), "không được có truy vấn nào")
	}
}

// fakeHub dựng một "Hub" trả theo từng kiểu để thử checkHub.
func fakeHub(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func TestCheckHub(t *testing.T) {
	svc := &GameService{http: http.DefaultClient}
	ctx := context.Background()

	t.Run("key đúng", func(t *testing.T) {
		var got string
		srv := fakeHub(t, func(w http.ResponseWriter, r *http.Request) {
			got = r.Header.Get(GameCatalogKeyHeader)
			assert.Equal(t, "/api/game-catalog", r.URL.Path)
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":[]}`))
		})
		assert.NoError(t, svc.checkHub(ctx, srv.URL, testHubKey))
		assert.Equal(t, testHubKey, got, "key đi trong header, không nằm trên URL")
	})
	t.Run("key bị từ chối", func(t *testing.T) {
		srv := fakeHub(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":401,"message":"sai khoá catalog game"}`))
		})
		err := svc.checkHub(ctx, srv.URL, testHubKey)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "key bị Hub từ chối")
		assert.NotContains(t, err.Error(), "sai khoá catalog game", "không vọng lại lời của phía bên kia")
		assert.NotContains(t, err.Error(), testHubKey, "lỗi không được lộ key")
	})
	t.Run("địa chỉ là một trang web khác trả 200", func(t *testing.T) {
		srv := fakeHub(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`<!doctype html><title>admin</title>`))
		})
		err := svc.checkHub(ctx, srv.URL, testHubKey)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "không phải Hub")
	})
	t.Run("200 nhưng không có code", func(t *testing.T) {
		srv := fakeHub(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"ok":true}`)) })
		assert.Error(t, svc.checkHub(ctx, srv.URL, testHubKey))
	})
	t.Run("Hub báo lỗi", func(t *testing.T) {
		srv := fakeHub(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":500,"message":"hỏng"}`))
		})
		err := svc.checkHub(ctx, srv.URL, testHubKey)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "HTTP 500")
		assert.NotContains(t, err.Error(), "hỏng", "nội dung phản hồi chỉ vào nhật ký, không trả về người gọi")
	})
	t.Run("dịch vụ khác trả JSON có code và message không bị đọc qua ô lỗi", func(t *testing.T) {
		srv := fakeHub(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"code":7,"message":"mật-khẩu-của-dịch-vụ-nội-bộ"}`))
		})
		err := svc.checkHub(ctx, srv.URL, testHubKey)
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "mật-khẩu-của-dịch-vụ-nội-bộ")
	})
	t.Run("chuyển hướng", func(t *testing.T) {
		// Dùng CheckRedirect thật của dịch vụ, bỏ riêng phần chặn loopback (Hub giả nằm trên loopback).
		c := &http.Client{Timeout: 5 * time.Second, CheckRedirect: hubCheckRedirect}
		redir := &GameService{http: c}
		srv := fakeHub(t, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://noi-khac.example/api/game-catalog", http.StatusFound)
		})
		err := redir.checkHub(ctx, srv.URL, testHubKey)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "chuyển hướng")
	})
	t.Run("không kết nối được", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		url := srv.URL
		srv.Close()
		err := svc.checkHub(ctx, url, testHubKey)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "không kết nối được")
		assert.NotContains(t, err.Error(), testHubKey)
	})
}

func TestHubText(t *testing.T) {
	assert.Equal(t, "sai khoá catalog game", hubText([]byte(`{"code":401,"message":"sai khoá catalog game"}`)))
	assert.Equal(t, "<html>", hubText([]byte(`<html>`)), "không phải JSON thì giữ nguyên")
	assert.Equal(t, `{"code":1}`, hubText([]byte(`{"code":1}`)), "không có message thì giữ nguyên")
}

func TestGameService_NoteSync(t *testing.T) {
	svc := &GameService{}
	svc.noteSync(nil)
	assert.False(t, svc.syncAt.IsZero())
	assert.Empty(t, svc.syncErr)
	svc.noteSync(assert.AnError)
	assert.NotEmpty(t, svc.syncErr)
	svc.noteSync(nil)
	assert.Empty(t, svc.syncErr, "lần sau tốt thì xoá lỗi cũ")
}

func TestSetHub_CheDoMoiTruong(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{Role: config.GameRoleMaster})
	_, err := svc.SetHub(context.Background(), GameHubRequest{URL: "http://hub", Key: testHubKey})
	assert.ErrorIs(t, err, ErrGameHubEnv)
	_, err = svc.ClearHub()
	assert.ErrorIs(t, err, ErrGameHubEnv)
	assert.NoError(t, mock.ExpectationsWereMet(), "không được chạm DB")
}

func expectSettingsRead(mock sqlmock.Sqlmock, rows ...[]string) {
	r := sqlmock.NewRows([]string{"group_name", "key", "value"})
	for _, row := range rows {
		r.AddRow(row[0], row[1], row[2])
	}
	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1`).WithArgs(GamesGroup).WillReturnRows(r)
}

func TestSetHub_KhongTuGuiKeyCuToiDiaChiMoi(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{})
	expectSettingsRead(mock,
		[]string{GamesGroup, GameHubURLKey, `"http://hub-cu:1"`},
		[]string{GamesGroup, GameHubKeyKey, `"` + testHubKey + `"`})

	_, err := svc.SetHub(context.Background(), GameHubRequest{URL: "http://hub-moi:1", Key: "", Root: t.TempDir()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chưa có key")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetHub_KeyKhongHopLeVaThuMucSai(t *testing.T) {
	for name, req := range map[string]GameHubRequest{
		"key có khoảng trắng": {URL: "http://hub:1", Key: "VGK-ABCDE FGHIJ-KLMNO-PQRST", Root: t.TempDir()},
		"key quá ngắn":        {URL: "http://hub:1", Key: "abc", Root: t.TempDir()},
		"địa chỉ sai":         {URL: "hub", Key: testHubKey, Root: t.TempDir()},
		"thư mục chưa có":     {URL: "http://hub:1", Key: testHubKey, Root: filepath.Join(t.TempDir(), "x")},
	} {
		db, mock := newMockDB(t)
		svc := NewGameService(db, config.GameConfig{})
		expectSettingsRead(mock)
		_, err := svc.SetHub(context.Background(), req)
		assert.Error(t, err, name)
		assert.NoError(t, mock.ExpectationsWereMet(), name+": không được ghi DB")
	}
}

func TestSetHub_KeySaiKhongLuu(t *testing.T) {
	srv := fakeHub(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":401,"message":"sai khoá catalog game"}`))
	})
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{})
	svc.http = http.DefaultClient // Hub giả nằm trên loopback, client thật của dịch vụ chặn loopback
	expectSettingsRead(mock)

	_, err := svc.SetHub(context.Background(), GameHubRequest{URL: srv.URL, Key: testHubKey, Root: t.TempDir()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "key bị Hub từ chối")
	assert.NoError(t, mock.ExpectationsWereMet(), "key sai thì không ghi gì vào DB")
}

func TestSetHub_LuuKhiKeyDung(t *testing.T) {
	srv := fakeHub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(GameCatalogKeyHeader) != testHubKey {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":401,"message":"sai khoá catalog game"}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":[]}`))
	})
	root := t.TempDir()
	db, mock := newMockDB(t)
	mock.MatchExpectationsInOrder(false) // saveGameSettings duyệt map nên thứ tự ba khoá không cố định
	svc := NewGameService(db, config.GameConfig{})
	svc.http = http.DefaultClient // Hub giả nằm trên loopback, client thật của dịch vụ chặn loopback

	expectSettingsRead(mock) // đọc cài đặt hiện có
	for range 3 {
		mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1 AND key = \$2 ORDER BY "system_settings"\."id" LIMIT \$3`).
			WillReturnRows(sqlmock.NewRows([]string{"id", "group_name", "key", "value"}))
		mock.ExpectBegin()
		mock.ExpectQuery(`INSERT INTO "system_settings"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
		mock.ExpectCommit()
	}
	expectSettingsRead(mock,
		[]string{GamesGroup, GameHubURLKey, `"` + srv.URL + `"`},
		[]string{GamesGroup, GameHubKeyKey, `"` + testHubKey + `"`},
		[]string{GamesGroup, GameRootKey, `"` + strings.ReplaceAll(root, `\`, `\\`) + `"`}) // Info đọc lại

	info, err := svc.SetHub(context.Background(), GameHubRequest{URL: srv.URL + "/", Key: "  " + testHubKey + "\n", Root: root})
	require.NoError(t, err)
	assert.Equal(t, srv.URL, info.HubURL)
	assert.True(t, info.HasKey)
	assert.Equal(t, "…QRST", info.KeyHint)
	assert.NotContains(t, strings.Join([]string{info.HubURL, info.KeyHint, info.GameRoot}, " "), "ABCDE", "key không được trả ra")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Rào SSRF: những địa chỉ máy chủ không được nối tới khi gọi Hub.
func TestBlockedHubIP(t *testing.T) {
	for _, s := range []string{
		"127.0.0.1", "127.8.9.10", "::1", "0.0.0.0", "::", "0.1.2.3",
		"169.254.169.254", "169.254.0.1", "fe80::1", "fd00:ec2::254",
		"::ffff:127.0.0.1", "::ffff:169.254.169.254", // IPv4 bọc trong IPv6
		"224.0.0.1", "ff02::1",
	} {
		ip := net.ParseIP(s)
		require.NotNil(t, ip, s)
		assert.True(t, blockedHubIP(ip), s+" phải bị chặn")
	}
	for _, s := range []string{"8.8.8.8", "1.1.1.1", "203.0.113.5", "2606:4700::1111",
		"10.0.0.5", "192.168.5.10", "172.16.3.4"} { // dải riêng của LAN: Hub đặt trong mạng quán vẫn nối được
		ip := net.ParseIP(s)
		require.NotNil(t, ip, s)
		assert.False(t, blockedHubIP(ip), s+" không được bị chặn")
	}
}

// Đường đầy đủ: dán địa chỉ Hub trỏ vào chính máy chủ thì SetHub từ chối, không ghi DB, và phía bên kia
// không nhận được yêu cầu nào.
func TestSetHub_ChanDiaChiNoiBo(t *testing.T) {
	var hit bool
	srv := fakeHub(t, func(w http.ResponseWriter, r *http.Request) { hit = true })
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{}) // client thật, có rào
	expectSettingsRead(mock)

	_, err := svc.SetHub(context.Background(), GameHubRequest{URL: srv.URL, Key: testHubKey, Root: t.TempDir()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "địa chỉ nội bộ")
	assert.False(t, hit)
	assert.NoError(t, mock.ExpectationsWereMet(), "không được ghi DB")
}

// Tên miền trỏ vào máy này cũng không lọt: rào nằm ở lúc nối, nhìn vào địa chỉ IP thật.
func TestHubHTTPClient_ChanNoiBoLucNoi(t *testing.T) {
	var hit bool
	srv := fakeHub(t, func(w http.ResponseWriter, r *http.Request) { hit = true })

	c := newHubHTTPClient(5 * time.Second)
	for name, target := range map[string]string{
		"theo IP":        srv.URL,
		"theo localhost": strings.Replace(srv.URL, "127.0.0.1", "localhost", 1),
	} {
		resp, err := c.Get(target)
		if resp != nil {
			resp.Body.Close()
		}
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), "địa chỉ nội bộ", name)
	}
	assert.False(t, hit, "yêu cầu không được tới máy chủ đích")
}

func TestHubHTTPClient_KhongTheoChuyenHuong(t *testing.T) {
	c := newHubHTTPClient(time.Second)
	require.NotNil(t, c.CheckRedirect)
	assert.Equal(t, http.ErrUseLastResponse, c.CheckRedirect(nil, nil))
	assert.Nil(t, c.Transport.(*http.Transport).Proxy, "qua proxy thì địa chỉ được kiểm là của proxy")
}

// Thư mục game nguy hiểm thì SetHub từ chối TRƯỚC khi hỏi Hub hay ghi DB.
func TestSetHub_ThuMucNguyHiemKhongLuu(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewGameService(db, config.GameConfig{})
	expectSettingsRead(mock)
	wd, err := os.Getwd()
	require.NoError(t, err)

	_, err = svc.SetHub(context.Background(), GameHubRequest{URL: "http://hub.example:20800", Key: testHubKey, Root: wd})
	require.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet(), "không được ghi DB")
}
