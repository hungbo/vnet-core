package main

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Đọc trước tệp của giao diện lúc mới bật máy.
//
// Máy trạm không ổ cứng đọc mọi tệp qua ổ đĩa mạng: đọc tuần tự từng khối lớn thì
// nhanh, nạp DLL từng mẩu nhỏ thì chậm. Đo lúc boot, giao diện mất 4,4 s dựng
// WebView2 rồi thêm ~9 s mới bắt đầu phân tích trang — các tiến trình WebView2
// khởi động lạnh từ ổ mạng. Bật lại giao diện khi máy đã chạy thì chưa tới 1 s
// (research/diskless-boot-speed.md mục 10).
//
// Dịch vụ nền chạy trước lúc Windows tự đăng nhập vài giây, nên nó đọc tuần tự
// (rồi bỏ đi) những tệp giao diện sắp cần, để chúng nằm sẵn trong bộ nhớ đệm của
// Windows khi giao diện chạy. Chỉ làm lúc mới bật máy: sau đó tệp đã nóng sẵn.
const (
	// Trần tổng số byte của một lượt: đủ cho exe và phần WebView2 nạp lúc dựng
	// trang, mà không kéo cả bản runtime (~789 MB) qua mạng đúng lúc boot.
	prefetchMaxBytes = 450 << 20
	// Mỗi lần đọc một khối lớn: ổ mạng chỉ nhanh khi đọc tuần tự.
	prefetchChunk = 1 << 20
	// Quá thời gian này thì thôi: cả phòng bật cùng lúc thì mạng chậm hẳn, đọc
	// tiếp khi giao diện đã lên chỉ tranh mạng với Windows và các máy khác.
	prefetchMaxTime = 15 * time.Second
)

// webView2First: những tệp trong thư mục phiên bản WebView2 sẽ đọc trước, theo
// thứ tự này; tệp nào không có thì bỏ qua. Không đọc tệp nào khác của runtime:
// chưa chắc chúng được nạp lúc dựng trang, mà cả phòng bật cùng lúc thì mỗi MB
// đọc thừa nhân lên theo số máy trên đường mạng của server
// (research/diskless-boot-speed.md mục 6).
//
// Tệp dữ liệu đứng trước: Chromium ánh xạ chúng như dữ liệu, dùng lại đúng các
// trang mà lượt đọc để trong bộ nhớ đệm. DLL/EXE thì Windows nạp như ảnh chương
// trình, đọc trang thẳng từ đĩa chứ không qua bộ nhớ đệm đó, nên đọc trước chúng
// chỉ có ích nếu ổ đĩa mạng có bộ đệm riêng (chưa kiểm chứng).
var webView2First = []string{
	"icudtl.dat", "v8_context_snapshot.bin", "resources.pak",
	"chrome_100_percent.pak", "msedge_100_percent.pak", // Edge đổi chrome_* thành msedge_*: tên nào có thì lấy
	"Locales/vi.pak", "Locales/en-US.pak",
	"msedgewebview2.exe", "msedge_elf.dll", "EBWebView/x64/EmbeddedBrowserWebView.dll",
	"msedge.dll", "libEGL.dll", "libGLESv2.dll", "d3dcompiler_47.dll",
}

// prefetchFile là một tệp sẽ đọc trước; size lấy lúc liệt kê.
type prefetchFile struct {
	path string
	size int64
}

// runBootPrefetch là một lượt đọc trước, chạy một lần từ runAgent.
func runBootPrefetch(ctx context.Context) {
	if !osIsWindows {
		log.Printf("[prefetch] bỏ qua: không phải Windows")
		return
	}
	// Giao diện bị bật lại giữa buổi thì tệp đã nóng từ lâu, còn dịch vụ khởi
	// động lại (cập nhật) thì đọc lại chỉ tốn mạng.
	if up := systemUptime(); up >= curtainMaxUptime {
		log.Printf("[prefetch] bỏ qua: máy đã chạy %ds", int(up.Seconds()))
		return
	}
	root := webView2Root()
	dir, ver := newestWebView2(root)
	if dir == "" {
		log.Printf("[prefetch] bỏ qua: không thấy WebView2 runtime trong %s", root)
		return
	}
	exe, _ := os.Executable()
	ctx, cancel := context.WithTimeout(ctx, prefetchMaxTime)
	defer cancel()
	start := time.Now()
	n, total := readAhead(ctx, prefetchPlan(exe, dir, prefetchMaxBytes))
	log.Printf("[prefetch] đọc trước %d tệp, %d MB trong %d ms (WebView2 %s)",
		n, total>>20, time.Since(start).Milliseconds(), ver)
	bootRec.docTruocXong(systemUptime(), total>>20, time.Since(start).Milliseconds())
}

// newestWebView2 tìm thư mục phiên bản WebView2 mới nhất trong root
// (…\EdgeWebView\Application). Phiên bản so theo từng số: "141.0.3537.57" mới
// hơn "141.0.3537.9". Như trình nạp WebView2, chỉ tính thư mục có
// msedgewebview2.exe. Không thấy thì trả về "".
func newestWebView2(root string) (dir, ver string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", ""
	}
	var best []int
	for _, e := range entries {
		// IsDir của ReadDir không tính liên kết/junction.
		v := parseDotted(e.Name())
		if !e.IsDir() || v == nil || (best != nil && slices.Compare(v, best) <= 0) {
			continue
		}
		p := filepath.Join(root, e.Name())
		if st, err := os.Lstat(filepath.Join(p, "msedgewebview2.exe")); err != nil || !st.Mode().IsRegular() {
			continue
		}
		dir, ver, best = p, e.Name(), v
	}
	return dir, ver
}

// parseDotted đọc "141.0.3537.57" thành [141 0 3537 57]; tên không phải phiên
// bản thì trả về nil.
func parseDotted(s string) []int {
	parts := strings.Split(s, ".")
	v := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		// Atoi nhận cả "+1" và "-1".
		if err != nil || strings.Trim(p, "0123456789") != "" {
			return nil
		}
		v[i] = n
	}
	return v
}

// prefetchPlan xếp các tệp sẽ đọc trước: webView2First trong dir, rồi exe của
// giao diện. Chỉ lấy tệp thường, không đi theo liên kết/junction, kể cả ở thư
// mục con. Dừng ở tệp đầu tiên làm tổng vượt maxBytes: tệp sau nhỏ hơn cũng
// không lấy, để không đọc tệp kém quan trọng thay cho tệp quan trọng hơn.
func prefetchPlan(exe, dir string, maxBytes int64) []prefetchFile {
	var plan []prefetchFile
	add := func(p string) {
		if st, err := os.Lstat(p); err == nil && st.Mode().IsRegular() {
			plan = append(plan, prefetchFile{p, st.Size()})
		}
	}
	for _, rel := range webView2First {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if plainDirChain(dir, filepath.Dir(p)) == nil {
			add(p)
		}
	}
	// exe đứng cuối: nó cũng là ảnh chương trình, và dịch vụ chạy từ chính tệp
	// này nên giao diện dùng chung những trang dịch vụ đã nạp.
	if exe != "" {
		add(exe)
	}

	var total int64
	for i, f := range plan {
		if total += f.size; total > maxBytes {
			return plan[:i]
		}
	}
	return plan
}

// readAhead đọc rồi bỏ từng tệp theo thứ tự, mỗi lần prefetchChunk byte và
// không quá cỡ lúc liệt kê. Tệp không đọc được thì lặng lẽ bỏ qua; ctx huỷ thì
// dừng. Trả về số tệp đã đọc hết và tổng số byte đã đọc.
//
// Không dùng io.Copy sang io.Discard: nó bỏ qua bộ đệm của mình và đọc từng mẩu
// 8 KB, đúng kiểu đọc mà ổ mạng chậm.
func readAhead(ctx context.Context, files []prefetchFile) (n int, total int64) {
	buf := make([]byte, prefetchChunk)
	for _, f := range files {
		if ctx.Err() != nil {
			break
		}
		in, err := os.Open(f.path)
		if err != nil {
			continue
		}
		r := io.LimitReader(in, f.size)
		for err == nil && ctx.Err() == nil {
			var k int
			k, err = r.Read(buf)
			total += int64(k)
		}
		in.Close()
		if err == io.EOF {
			n++
		}
	}
	return n, total
}
