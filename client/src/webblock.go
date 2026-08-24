package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Chặn website ở máy trạm.
//
// Cách chặn: ghi tên miền vào tệp hosts, trỏ về 127.0.0.1, rồi tự dựng một máy
// chủ HTTP nhỏ ngay tại 127.0.0.1:80.
//
// Dựng máy chủ đó không phải để cho vui: chỉ ghi hosts thì trình duyệt báo "không
// kết nối được" — khách không hiểu vì sao, và quầy KHÔNG BIẾT đã có ai thử vào.
// Có máy chủ nội bộ thì khách thấy trang giải thích, còn quầy nhận được báo cáo
// vi phạm kèm tên miền.
//
// Giới hạn: chỉ chặn được HTTP. Trình duyệt gọi thẳng HTTPS sẽ bị lỗi chứng chỉ
// hoặc lỗi kết nối chứ không thấy trang giải thích — chặn vẫn có tác dụng nhưng
// không báo cáo được. Chặn triệt để cần proxy hoặc can thiệp DNS.

const (
	hostsMarkerBegin = "# === VNET BLOCK BEGIN — do not edit inside this block ==="
	hostsMarkerEnd   = "# === VNET BLOCK END ==="
)

type blocklistPayload struct {
	Domains []string `json:"domains"`
}

type webBlocker struct {
	cfg *Config

	mu      sync.RWMutex
	blocked map[string]bool
}

func newWebBlocker(cfg *Config) *webBlocker {
	return &webBlocker{cfg: cfg, blocked: map[string]bool{}}
}

// run kéo danh sách chặn theo chu kỳ và áp vào tệp hosts.
func (w *webBlocker) run(ctx context.Context) {
	go w.serveBlockPage(ctx)

	ticker := time.NewTicker(w.cfg.BlocklistInterval)
	defer ticker.Stop()

	w.sync()
	for {
		select {
		case <-ctx.Done():
			// Gỡ danh sách khi thoát: để lại hosts đã sửa nghĩa là máy vẫn bị
			// chặn sau khi phần mềm đã tắt.
			if err := writeHostsBlock(nil); err != nil {
				log.Printf("[chặn web] không dọn được tệp hosts: %v", err)
			}
			return
		case <-ticker.C:
			w.sync()
		}
	}
}

func (w *webBlocker) sync() {
	url := fmt.Sprintf("%s/api/machines/by-code/%s/blocklist",
		w.cfg.ServerURL, w.cfg.MachineCode)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return
	}
	req.Header.Set("X-Agent-Token", w.cfg.AgentToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("[chặn web] không lấy được danh sách: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		log.Printf("[chặn web] máy chủ trả %d — kiểm tra VNET_AGENT_TOKEN", resp.StatusCode)
		return
	}

	var envelope struct {
		Data blocklistPayload `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		log.Printf("[chặn web] danh sách không đọc được: %v", err)
		return
	}

	domains := envelope.Data.Domains
	next := make(map[string]bool, len(domains)*2)
	for _, d := range domains {
		next[d] = true
		next["www."+d] = true
	}

	w.mu.Lock()
	changed := !sameSet(w.blocked, next)
	w.blocked = next
	w.mu.Unlock()

	// Chỉ ghi lại hosts khi danh sách đổi: ghi mỗi chu kỳ là ghi đĩa vô ích và
	// dễ đụng độ với phần mềm khác cũng sửa tệp này.
	if !changed {
		return
	}
	if err := writeHostsBlock(domains); err != nil {
		log.Printf("[chặn web] không ghi được tệp hosts: %v", err)
		return
	}
	log.Printf("[chặn web] đã áp %d tên miền", len(domains))
}

func sameSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// serveBlockPage phục vụ trang giải thích tại 127.0.0.1:80 và báo vi phạm về
// quầy.
func (w *webBlocker) serveBlockPage(ctx context.Context) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		host := strings.ToLower(r.Host)
		if i := strings.Index(host, ":"); i >= 0 {
			host = host[:i]
		}

		w.mu.RLock()
		known := w.blocked[host]
		w.mu.RUnlock()
		if known {
			go w.reportViolation(host, r.URL.String())
		}

		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		rw.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(rw, blockPageHTML, host)
	})

	srv := &http.Server{
		Addr:              "127.0.0.1:80",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		// Cổng 80 bận là chuyện có thể xảy ra; chặn vẫn hoạt động, chỉ mất
		// trang giải thích và báo cáo vi phạm.
		log.Printf("[chặn web] không mở được trang thông báo tại :80: %v", err)
	}
}

func (w *webBlocker) reportViolation(domain, url string) {
	body, _ := json.Marshal(map[string]string{
		"domain": domain, "url": url, "process_name": "",
	})
	endpoint := fmt.Sprintf("%s/api/machines/by-code/%s/blocklist/violations",
		w.cfg.ServerURL, w.cfg.MachineCode)
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-Token", w.cfg.AgentToken)
	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

// writeHostsBlock thay phần của VNET trong tệp hosts, giữ nguyên phần còn lại.
//
// Ghi qua tệp tạm rồi đổi tên: mất điện giữa chừng khi ghi đè trực tiếp sẽ để
// lại tệp hosts cụt, và máy mất luôn khả năng phân giải tên miền.
func writeHostsBlock(domains []string) error {
	path := hostsPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	kept := stripHostsBlock(string(raw))

	var b strings.Builder
	b.WriteString(kept)
	if !strings.HasSuffix(kept, "\n") && kept != "" {
		b.WriteString("\n")
	}
	if len(domains) > 0 {
		sorted := append([]string(nil), domains...)
		sort.Strings(sorted)
		b.WriteString(hostsMarkerBegin)
		b.WriteString("\n")
		for _, d := range sorted {
			fmt.Fprintf(&b, "127.0.0.1\t%s\n127.0.0.1\twww.%s\n", d, d)
		}
		b.WriteString(hostsMarkerEnd)
		b.WriteString("\n")
	}

	tmp := filepath.Join(filepath.Dir(path), ".vnet-hosts.tmp")
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// stripHostsBlock gỡ đúng phần giữa hai mốc, giữ nguyên mọi dòng khác. Người
// dùng hoặc phần mềm khác có thể có mục riêng trong tệp này.
func stripHostsBlock(content string) string {
	begin := strings.Index(content, hostsMarkerBegin)
	if begin < 0 {
		return content
	}
	end := strings.Index(content[begin:], hostsMarkerEnd)
	if end < 0 {
		// Mốc mở mà không có mốc đóng: cắt từ mốc mở tới hết, còn hơn để lại
		// một khối dở dang rồi nối thêm khối mới vào sau.
		return strings.TrimRight(content[:begin], "\n")
	}
	after := begin + end + len(hostsMarkerEnd)
	rest := strings.TrimPrefix(content[after:], "\n")
	return strings.TrimRight(content[:begin], "\n") + "\n" + rest
}

func hostsPath() string {
	if p := os.Getenv("VNET_HOSTS_FILE"); p != "" {
		return p
	}
	if isWindows() {
		root := os.Getenv("SystemRoot")
		if root == "" {
			root = `C:\Windows`
		}
		return filepath.Join(root, "System32", "drivers", "etc", "hosts")
	}
	return "/etc/hosts"
}

var blockPageHTML = `<!doctype html>
<html lang="vi"><head><meta charset="utf-8">
<title>Trang bị chặn</title>
<style>
body{margin:0;height:100vh;display:flex;align-items:center;justify-content:center;
font-family:system-ui,-apple-system,"Segoe UI",sans-serif;background:#0f1021;color:#e8e9f3}
.box{text-align:center;padding:48px;max-width:520px}
h1{font-size:28px;margin:0 0 12px}
p{color:#a5a8c4;line-height:1.6;margin:0}
code{background:#1c1e3a;padding:2px 8px;border-radius:4px;color:#e8e9f3}
</style></head>
<body><div class="box">
<div style="font-size:64px;line-height:1;margin-bottom:20px">🚫</div>
<h1>Trang này bị chặn</h1>
<p><code>%s</code> nằm trong danh sách chặn của quán.<br>Liên hệ quầy nếu bạn cần truy cập.</p>
</div></body></html>`

// isWindows tách riêng để phần còn lại của tệp kiểm được trên máy phát triển.
func isWindows() bool { return osIsWindows }

// listenerCheck giữ net trong danh sách import ở mọi hệ điều hành.
var _ = net.JoinHostPort
