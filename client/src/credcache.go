package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Bộ nhớ đệm tài khoản nhân viên, để còn vào được máy khi máy chủ không với tới.
//
// Cách làm: mỗi lần một tài khoản NHÂN VIÊN đăng nhập thành công qua máy chủ,
// máy trạm ghi lại tên và BĂM của mật khẩu. Mất mạng thì vẫn dùng đúng tài khoản
// đó để mở máy — không phải nhớ thêm mã nào.
//
// Ba điều kiện phải giữ, và mỗi cái đều là một cách làm hỏng nếu bỏ qua:
//
//  1. KHÔNG lưu mật khẩu trần. Băm bằng PBKDF2 như tài khoản quản trị máy trạm.
//  2. CHỈ lưu tài khoản nhân viên. Lưu cả hội viên nghĩa là khách rút dây mạng
//     rồi tự mở máy — chơi không mất tiền, đúng thứ lớp khoá sinh ra để chặn.
//  3. CÓ hạn dùng. Đuổi việc một nhân viên và đổi mật khẩu trên máy chủ mà bản
//     đệm sống mãi thì họ vẫn mở được mọi máy trong quán.

const (
	credCacheMax = 5                   // giữ mấy tài khoản gần nhất
	credCacheTTL = 30 * 24 * time.Hour // quá hạn thì không mở offline được nữa
)

type cachedCred struct {
	Username string    `json:"username"`
	Hash     string    `json:"hash"`
	Role     string    `json:"role"`
	SavedAt  time.Time `json:"saved_at"`
}

var errServerUnreachable = errors.New("không kết nối được máy chủ")

var errNoCachedCred = errors.New("máy này chưa từng có nhân viên nào đăng nhập, nên không mở offline được")

// credCachePath nằm trong thư mục dữ liệu của NGƯỜI DÙNG, không phải cạnh .exe.
//
// Giao diện chạy dưới tài khoản khách nên nó không ghi được vào Program Files —
// ghi ở đó thì bản đệm không bao giờ được cập nhật, và cả cơ chế này thành vô
// dụng đúng lúc cần.
func credCachePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, "VNET")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "offline-staff.json"), nil
}

func loadCredCache() []cachedCred {
	path, err := credCachePath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []cachedCred
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out
}

func saveCredCache(list []cachedCred) error {
	path, err := credCachePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// rememberStaff ghi lại một lần đăng nhập nhân viên thành công.
//
// Ghi đè bản cũ của cùng tài khoản: đổi mật khẩu trên máy chủ rồi đăng nhập một
// lần là bản đệm theo kịp, đúng như mong đợi.
func rememberStaff(username, password, role string) error {
	if role == "" || role == "member" {
		return nil
	}
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil
	}

	hash, err := hashPin(password)
	if err != nil {
		return err
	}

	list := loadCredCache()
	out := []cachedCred{{Username: username, Hash: hash, Role: role, SavedAt: time.Now()}}
	for _, c := range list {
		if strings.EqualFold(c.Username, username) {
			continue
		}
		out = append(out, c)
		if len(out) == credCacheMax {
			break
		}
	}
	return saveCredCache(out)
}

// verifyCachedStaff kiểm tài khoản nhân viên khi không có mạng.
func verifyCachedStaff(username, password string, now time.Time) (*cachedCred, error) {
	username = strings.TrimSpace(username)
	list := loadCredCache()

	if len(list) == 0 {
		return nil, errNoCachedCred
	}

	for i := range list {
		c := list[i]
		if !strings.EqualFold(c.Username, username) {
			continue
		}
		if now.Sub(c.SavedAt) > credCacheTTL {
			return nil, errors.New("bản lưu offline của tài khoản này đã quá hạn — cần kết nối máy chủ một lần")
		}
		if err := verifyPin(c.Hash, password); err != nil {
			return nil, errors.New("sai tài khoản hoặc mật khẩu")
		}
		return &c, nil
	}
	return nil, errors.New("sai tài khoản hoặc mật khẩu")
}
