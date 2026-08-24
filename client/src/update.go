package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// version được gán lúc biên dịch bằng -X main.version=...
//
// Trước đây biến này KHÔNG tồn tại, nên cờ -X trong scripts/build-client.sh
// rơi vào hư không: mọi bản build đều không biết mình là phiên bản nào.
var version = "dev"

// Cập nhật máy khách.
//
// Máy chủ trả về phiên bản mới nhất kèm đường tải và BĂM SHA-256. Máy trạm tải
// về, kiểm băm, rồi mới chạy.
//
// Kiểm băm là bắt buộc, không phải cho chắc: tải một tệp thực thi rồi chạy mà
// không kiểm nghĩa là ai đứng giữa đường truyền cũng chạy được mã tuỳ ý trên
// toàn bộ máy trạm.

type UpdateInfo struct {
	HasUpdate  bool   `json:"has_update"`
	Version    string `json:"version"`
	FileURL    string `json:"file_url"`
	Checksum   string `json:"checksum"`
	FileSize   int64  `json:"file_size"`
	Changelog  string `json:"changelog"`
	IsRequired bool   `json:"is_required"`
	Current    string `json:"current"`
}

// platformTag khớp với trường platform mà quầy khai lúc công bố bản cập nhật.
func platformTag() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

func (a *App) GetVersion() string { return version }

// CheckUpdate hỏi máy chủ có bản mới hơn không.
func (a *App) CheckUpdate() (string, error) {
	info, err := checkUpdate(a.cfg)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(info)
	return string(b), nil
}

func checkUpdate(cfg *Config) (*UpdateInfo, error) {
	url := fmt.Sprintf("%s/api/machines/by-code/%s/app-update?platform=%s&current=%s",
		cfg.ServerURL, cfg.MachineCode, platformTag(), version)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Agent-Token", cfg.AgentToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("máy chủ trả %d", resp.StatusCode)
	}

	var envelope struct {
		Data UpdateInfo `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

// DownloadUpdate tải bản cập nhật về và kiểm băm. Trả về đường dẫn tệp.
//
// KHÔNG tự chạy tệp: cài đặt làm ứng dụng tự tắt giữa phiên chơi của khách, nên
// đó phải là hành động có người quyết định.
func (a *App) DownloadUpdate() (string, error) {
	info, err := checkUpdate(a.cfg)
	if err != nil {
		return "", err
	}
	if !info.HasUpdate {
		return "", fmt.Errorf("đang ở phiên bản mới nhất (%s)", version)
	}
	return downloadAndVerify(info)
}

func downloadAndVerify(info *UpdateInfo) (string, error) {
	if !isHexSHA256(info.Checksum) {
		// Máy chủ bắt buộc phải khai băm; thiếu băm thì dừng, không tải.
		return "", fmt.Errorf("bản cập nhật %s không kèm băm hợp lệ — không tải", info.Version)
	}

	resp, err := httpClient.Get(info.FileURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("tải bản cập nhật thất bại: máy chủ trả %d", resp.StatusCode)
	}

	dir := filepath.Join(os.TempDir(), "vnet-update")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dest := filepath.Join(dir, fmt.Sprintf("vnet-client-%s%s", info.Version, installerExt()))

	// Ghi ra tệp tạm rồi mới đổi tên: đứt mạng giữa chừng sẽ để lại một tệp
	// cụt mang đúng tên bản cài, và lần sau có người chạy nhầm nó.
	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}

	h := sha256.New()
	written, err := io.Copy(io.MultiWriter(f, h), resp.Body)
	f.Close()
	if err != nil {
		os.Remove(tmp)
		return "", err
	}

	if info.FileSize > 0 && written != info.FileSize {
		os.Remove(tmp)
		return "", fmt.Errorf("tệp tải về dài %d byte, khai báo %d", written, info.FileSize)
	}

	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, info.Checksum) {
		os.Remove(tmp)
		return "", fmt.Errorf("băm không khớp: tệp tải về %s, khai báo %s — đã xoá tệp", got, info.Checksum)
	}

	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return dest, nil
}

func isHexSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

func installerExt() string {
	if osIsWindows {
		return ".exe"
	}
	return ""
}
