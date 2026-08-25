package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/shirou/gopsutil/v3/process"
)

// procInfo là một dòng bảng tiến trình đã GỘP theo tên. Trình duyệt đẻ ra hàng
// chục tiến trình con cùng tên; liệt kê thô thì bảng dài vô dụng và không nói
// được "Chrome đang ăn bao nhiêu RAM".
type procInfo struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
	RAMMB int64  `json:"ram_mb"`
}

// camTat là những tiến trình KHÔNG được phép tắt từ xa.
//
// Hai nhóm: lõi Windows (tắt là máy sập xanh) và chính máy trạm VNET (tắt lớp
// khoá là khách dùng máy chùa). superviseUI có bật lại giao diện nếu nó chết,
// nhưng chặn ở đây tránh cả cú nhấp nháy lẫn việc một nhân viên nghịch dại.
var camTat = map[string]bool{
	"csrss":       true,
	"winlogon":    true,
	"wininit":     true,
	"services":    true,
	"lsass":       true,
	"smss":        true,
	"svchost":     true,
	"system":      true,
	"vnet-client": true,
}

func chuanTenTienTrinh(name string) string {
	return strings.ToLower(strings.TrimSuffix(name, ".exe"))
}

// likeTienTrinh gộp mọi tiến trình theo tên, kèm tổng RAM.
func likeTienTrinh() []procInfo {
	procs, err := process.Processes()
	if err != nil {
		return nil
	}
	gop := map[string]*procInfo{}
	for _, p := range procs {
		name, err := p.Name()
		if err != nil || name == "" {
			continue
		}
		key := chuanTenTienTrinh(name)
		g := gop[key]
		if g == nil {
			g = &procInfo{Name: name}
			gop[key] = g
		}
		g.Count++
		if mem, err := p.MemoryInfo(); err == nil && mem != nil {
			g.RAMMB += int64(mem.RSS) / (1024 * 1024)
		}
	}

	out := make([]procInfo, 0, len(gop))
	for _, g := range gop {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RAMMB > out[j].RAMMB })
	return out
}

// tatTienTrinh tắt mọi tiến trình khớp tên, trả số tắt được. Bỏ qua danh sách
// cấm — trả -1 để người gọi biết đây là yêu cầu bị từ chối chứ không phải
// "không tìm thấy tiến trình nào".
func tatTienTrinh(name string) int {
	if camTat[chuanTenTienTrinh(name)] {
		return -1
	}
	procs, err := process.Processes()
	if err != nil {
		return 0
	}
	killed := 0
	for _, p := range procs {
		pname, err := p.Name()
		if err != nil {
			continue
		}
		if chuanTenTienTrinh(pname) == chuanTenTienTrinh(name) {
			if p.Kill() == nil {
				killed++
			}
		}
	}
	return killed
}

// baoCaoLen POST một payload JSON lên route by-code, xác thực bằng khoá máy —
// cùng đường mà heartbeat đang đi. path ví dụ "screenshot" hoặc "processes".
func baoCaoLen(cfg *Config, path string, body interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/api/machines/by-code/%s/%s", cfg.ServerURL, cfg.MachineCode, path)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-Token", cfg.AgentToken)
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("máy chủ trả %d", resp.StatusCode)
	}
	return nil
}
