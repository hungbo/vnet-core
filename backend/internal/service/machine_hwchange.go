package service

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
)

// Nhật ký thay đổi phần cứng và tổng quan Boot / bảo mật (roadmap M1, M2).
//
// Máy trạm gửi số đo trong MỌI nhịp tim; Heartbeat so với số đã lưu và ghi một dòng
// MachineHardwareChange cho mỗi khác biệt. Việc so sánh nằm trong hàm thuần hwDiff để
// kiểm thử mà không cần cơ sở dữ liệu.

// hwChange là một khác biệt tìm được; chưa có id, thời gian hay máy.
type hwChange struct {
	Component string
	Action    string // changed | added | removed
	Old, New  string
}

// hwChangeMax: trần số dòng ghi cho MỘT nhịp tim. Máy cắm cả hub USB hay đổi cả bộ
// linh kiện cùng lúc vẫn chỉ là một sự kiện đối với người đọc, và máy lạ không được
// nhồi hàng trăm dòng vào database.
const hwChangeMax = 20

// peripheralComponent đổi nhãn tiếng Việt máy trạm gửi ("Màn hình: DELL …") về tên thành
// phần trong nhật ký. Nhãn lạ (máy trạm bản mới thêm loại) vẫn ghi, dưới "other".
var peripheralComponent = map[string]string{
	"Màn hình": "monitor",
	"Bàn phím": "keyboard",
	"Chuột":    "mouse",
	"Âm thanh": "sound",
}

func splitPeripheral(item string) (component, name string) {
	label, name, ok := strings.Cut(item, ": ")
	if !ok {
		return "other", item
	}
	if c, found := peripheralComponent[label]; found {
		return c, name
	}
	return "other", item
}

// hwDiff so số đo trong nhịp tim với máy đã lưu. peripherals là danh sách đã làm sạch.
//
// Một giá trị chỉ tính là "đổi" khi CẢ HAI vế có: cũ rỗng là lần đầu thấy giá trị đó
// (máy trạm bản mới bắt đầu báo Secure Boot, hoặc máy chưa từng khai RAM), mới rỗng là
// máy trạm không đọc được lần này. Cả hai đều không phải người ta đổi linh kiện.
func hwDiff(m *model.Machine, req HeartbeatRequest, peripherals []string) []hwChange {
	var out []hwChange
	changed := func(component, old, now string) {
		if old != "" && now != "" && old != now {
			out = append(out, hwChange{Component: component, Action: "changed", Old: old, New: now})
		}
	}
	changed("cpu", m.CPUName, truncateRunes(req.CPUName, 100))
	changed("gpu", m.GPUName, truncateRunes(req.GPUName, 100))
	if m.RAMGB > 0 && req.RAMGB > 0 {
		changed("ram", fmt.Sprintf("%d GB", m.RAMGB), fmt.Sprintf("%d GB", req.RAMGB))
	}
	if m.StorageGB > 0 && req.StorageGB > 0 {
		changed("storage", fmt.Sprintf("%d GB", m.StorageGB), fmt.Sprintf("%d GB", req.StorageGB))
	}
	changed("boot_mode", m.BootMode, req.BootMode)
	changed("secure_boot", m.SecureBoot, req.SecureBoot)
	changed("tpm", m.TPM, req.TPM)
	changed("hvci", m.HVCI, req.HVCI)
	changed("virtualization", m.Virtualization, req.Virtualization)
	changed("bios", m.BIOSVersion, truncateRunes(req.BIOSVersion, 64))

	// Màn hình chỉ ghi khi TẦN SỐ QUÉT TỤT: game đổi độ phân giải rồi trả lại mỗi lần chạy,
	// ghi mọi thay đổi thì tràn nhật ký; còn màn 280 Hz rơi xuống 60 Hz (cáp, driver, hoặc có
	// người đổi) thì khách chơi dở mà quán không biết.
	if oldHz, newHz := displayHz(m.DisplayMode), displayHz(req.DisplayMode); oldHz > 0 && newHz > 0 && newHz < oldHz {
		out = append(out, hwChange{Component: "display", Action: "changed", Old: m.DisplayMode, New: req.DisplayMode})
	}

	// Ngoại vi: so như hai túi (có thể trùng tên — hai màn hình cùng hãng). Danh sách cũ
	// rỗng là lần đầu máy khai, danh sách mới rỗng không tới đây (Heartbeat bỏ qua).
	if len(m.Peripherals) > 0 && len(peripherals) > 0 {
		have := map[string]int{}
		for _, p := range m.Peripherals {
			have[p]++
		}
		for _, p := range peripherals {
			if have[p] > 0 {
				have[p]--
				continue
			}
			c, name := splitPeripheral(p)
			out = append(out, hwChange{Component: c, Action: "added", New: truncateRunes(name, 200)})
		}
		// Duyệt theo danh sách cũ để thứ tự ổn định (map thì không).
		for _, p := range m.Peripherals {
			if have[p] > 0 {
				have[p]--
				c, name := splitPeripheral(p)
				out = append(out, hwChange{Component: c, Action: "removed", Old: truncateRunes(name, 200)})
			}
		}
	}
	if len(out) > hwChangeMax {
		out = out[:hwChangeMax]
	}
	return out
}

var displayHzRe = regexp.MustCompile(`(\d{1,3})Hz$`)

// displayHz lấy tần số quét từ "1920x1080 144Hz"; 0 nếu không đọc được.
func displayHz(mode string) int {
	m := displayHzRe.FindStringSubmatch(mode)
	if m == nil {
		return 0
	}
	n := 0
	for _, c := range m[1] {
		n = n*10 + int(c-'0')
	}
	return n
}

// hwSummary là câu ngắn cho thông báo trên trang quản trị: tối đa ba thay đổi đầu.
func hwSummary(changes []hwChange) string {
	parts := make([]string, 0, 3)
	for i, c := range changes {
		if i == 3 {
			break
		}
		switch c.Action {
		case "added":
			parts = append(parts, fmt.Sprintf("%s: + %s", c.Component, c.New))
		case "removed":
			parts = append(parts, fmt.Sprintf("%s: − %s", c.Component, c.Old))
		default:
			parts = append(parts, fmt.Sprintf("%s: %s → %s", c.Component, c.Old, c.New))
		}
	}
	s := strings.Join(parts, "; ")
	if len(changes) > 3 {
		s += fmt.Sprintf(" (+%d)", len(changes)-3)
	}
	return s
}

// HardwareChangeFilter lọc danh sách nhật ký.
type HardwareChangeFilter struct {
	MachineID      string
	Component      string
	Unacknowledged bool
}

// ListHardwareChanges trả nhật ký mới nhất trước, kèm mã máy.
func (s *MachineService) ListHardwareChanges(params pagination.Params, f HardwareChangeFilter) (*pagination.Result, error) {
	query := s.db.Table("machine_hardware_changes AS c").
		Joins("LEFT JOIN machines AS m ON m.id = c.machine_id")
	if f.MachineID != "" {
		query = query.Where("c.machine_id = ?", f.MachineID)
	}
	if f.Component != "" {
		query = query.Where("c.component = ?", f.Component)
	}
	if f.Unacknowledged {
		query = query.Where("c.acknowledged_at IS NULL")
	}
	if params.Search != "" {
		query = query.Where("unaccent(m.machine_code) ILIKE unaccent(?)", "%"+params.Search+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	// Chưa xác nhận lên đầu rồi mới tới thời gian: việc cần xem không trôi xuống trang sau
	// chỉ vì một máy khác vừa đổi.
	var rows []model.MachineHardwareChange
	err := query.Select("c.*, m.machine_code AS machine_code").
		Order("(c.acknowledged_at IS NOT NULL) asc, c.detected_at desc").
		Limit(params.PageSize).Offset((params.Page - 1) * params.PageSize).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return pagination.NewResult(rows, total, &params), nil
}

// AckHardwareChanges đánh dấu đã xem. ids rỗng và all=true thì xác nhận hết các dòng chưa
// xác nhận. Trả về số dòng vừa đổi; dòng đã xác nhận từ trước không bị ghi đè người xác nhận.
func (s *MachineService) AckHardwareChanges(ids []string, all bool, username string) (int64, error) {
	if len(ids) == 0 && !all {
		return 0, errors.New("chưa chọn thay đổi nào để xác nhận")
	}
	query := s.db.Model(&model.MachineHardwareChange{}).Where("acknowledged_at IS NULL")
	if !all {
		query = query.Where("id IN ?", ids)
	}
	res := query.Updates(map[string]interface{}{
		"acknowledged_at": time.Now(),
		"acknowledged_by": truncateRunes(username, 50),
	})
	return res.RowsAffected, res.Error
}

// MachineHardwareRow là một dòng của bảng tổng quan Boot / bảo mật.
type MachineHardwareRow struct {
	ID             string     `json:"id"`
	MachineCode    string     `json:"machine_code"`
	Status         string     `json:"status"`
	CPUName        string     `json:"cpu_name"`
	GPUName        string     `json:"gpu_name"`
	RAMGB          int        `json:"ram_gb"`
	StorageGB      int        `json:"storage_gb"`
	BootMode       string     `json:"boot_mode"`
	SecureBoot     string     `json:"secure_boot"`
	TPM            string     `json:"tpm"`
	HVCI           string     `json:"hvci"`
	Virtualization string     `json:"virtualization"`
	BIOSVersion    string     `json:"bios_version"`
	DisplayMode    string     `json:"display_mode"`
	LastHeartbeat  *time.Time `json:"last_heartbeat"`
	// Số thay đổi chưa xác nhận của máy này.
	PendingChanges int64 `json:"pending_changes"`
}

// MachineActivityRow là một dòng của bảng Hoạt động: một dòng mỗi máy.
type MachineActivityRow struct {
	ID            string     `json:"id"`
	MachineCode   string     `json:"machine_code"`
	Status        string     `json:"status"`
	LastHeartbeat *time.Time `json:"last_heartbeat"`
	ActiveApp     string     `json:"active_app"`
	CPUTemp       float64    `json:"cpu_temp"`
	GPUTemp       float64    `json:"gpu_temp"`
	CPUUsage      float64    `json:"cpu_usage"`
	RAMUsage      float64    `json:"ram_usage"`
	DiskUsage     float64    `json:"disk_usage"`
	NetDownBps    int64      `json:"net_down_bps"`
	NetUpBps      int64      `json:"net_up_bps"`
	PingMS        int        `json:"ping_ms"`
}

// MachineActivity trả số đo nhịp tim gần nhất của MỌI máy. Máy đang tắt vẫn có dòng (số cũ),
// trang tự che bằng trạng thái.
func (s *MachineService) MachineActivity() ([]MachineActivityRow, error) {
	var rows []MachineActivityRow
	err := s.db.Table("machines").
		Select(`id, machine_code, status, last_heartbeat, active_app, cpu_temp, gpu_temp,
			cpu_usage, ram_usage, disk_usage, net_down_bps, net_up_bps, ping_ms`).
		Where("deleted_at IS NULL").
		Order("machine_code asc").
		Scan(&rows).Error
	return rows, err
}

// HardwareOverview trả MỌI máy (không phân trang): trang so sánh cả phòng máy, và bộ lọc
// "máy nào chưa bật Secure Boot" phải thấy hết chứ không phải trang đang xem.
func (s *MachineService) HardwareOverview() ([]MachineHardwareRow, error) {
	var rows []MachineHardwareRow
	err := s.db.Table("machines AS m").
		Select(`m.id, m.machine_code, m.status, m.cpu_name, m.gpu_name, m.ram_gb, m.storage_gb,
			m.boot_mode, m.secure_boot, m.tpm, m.hvci, m.virtualization, m.bios_version,
			m.display_mode, m.last_heartbeat,
			(SELECT count(*) FROM machine_hardware_changes c
			 WHERE c.machine_id = m.id AND c.acknowledged_at IS NULL) AS pending_changes`).
		Where("m.deleted_at IS NULL").
		Order("m.machine_code asc").
		Scan(&rows).Error
	return rows, err
}

// sanitizeSecurity bỏ giá trị ngoài danh sách. Đường nhịp tim không có đăng nhập, nên
// những chuỗi này vào thẳng cột database và bộ lọc trên trang quản trị; máy trạm đã lọc
// ở phía nó (client security.go) nhưng máy chủ không được tin vào điều đó.
func (r *HeartbeatRequest) sanitizeSecurity() {
	oneOf := func(v string, allowed ...string) string {
		for _, a := range allowed {
			if v == a {
				return v
			}
		}
		return ""
	}
	r.BootMode = oneOf(r.BootMode, "uefi", "legacy")
	r.SecureBoot = oneOf(r.SecureBoot, "on", "off", "unsupported")
	r.HVCI = oneOf(r.HVCI, "on", "off")
	r.Virtualization = oneOf(r.Virtualization, "on", "off")
	if r.TPM != "none" && !tpmVersionRe.MatchString(r.TPM) {
		r.TPM = ""
	}
	r.WinLicense = oneOf(r.WinLicense, "licensed", "unlicensed", "grace", "notification")
	r.WinKeyTail = strings.ToUpper(strings.TrimSpace(r.WinKeyTail))
	if !winKeyTailRe.MatchString(r.WinKeyTail) {
		r.WinKeyTail = ""
	}
	r.BIOSVersion = truncateRunes(strings.TrimSpace(r.BIOSVersion), 64)
	if !displayModeRe.MatchString(r.DisplayMode) {
		r.DisplayMode = ""
	}
}

// addActivity thêm các cột của bảng Hoạt động khi nhịp tim có khai. Giá trị âm hoặc vô lý bị
// kẹp lại: đường nhịp tim không có đăng nhập, và cột này hiện thẳng lên trang quản trị.
func (r *HeartbeatRequest) addActivity(updates map[string]interface{}) {
	if r.ActiveApp != nil {
		updates["active_app"] = truncateRunes(strings.TrimSpace(*r.ActiveApp), 64)
	}
	clamp := func(v int64) int64 {
		// Trần 10 GB/s (80 Gbps) mỗi chiều: hơn mọi card mạng quán đang dùng.
		return min(max(v, 0), 10_000_000_000)
	}
	if r.NetDownBps != nil {
		updates["net_down_bps"] = clamp(*r.NetDownBps)
	}
	if r.NetUpBps != nil {
		updates["net_up_bps"] = clamp(*r.NetUpBps)
	}
	if r.PingMS != nil {
		updates["ping_ms"] = min(max(*r.PingMS, 0), 60000)
	}
}

var (
	tpmVersionRe  = regexp.MustCompile(`^\d{1,2}(\.\d{1,2})?$`)
	displayModeRe = regexp.MustCompile(`^\d{3,5}x\d{3,5} \d{1,3}Hz$`)
	winKeyTailRe  = regexp.MustCompile(`^[A-Z0-9]{5}$`)
)

// boTutHzLapLai: máy đang có một dòng "tụt tần số quét" chưa xác nhận thì không ghi thêm dòng
// mới. Game bật tắt chế độ 60 Hz mỗi lần chạy sẽ sinh một dòng mỗi phiên; để nhân viên thấy MỘT
// dòng cho tới khi họ xác nhận. Chỉ truy vấn khi có dòng màn hình, tức hiếm.
func (s *MachineService) boTutHzLapLai(machineID string, changes []hwChange) []hwChange {
	idx := -1
	for i, c := range changes {
		if c.Component == "display" {
			idx = i
			break
		}
	}
	if idx < 0 {
		return changes
	}
	var pending int64
	if err := s.db.Model(&model.MachineHardwareChange{}).
		Where("machine_id = ? AND component = ? AND acknowledged_at IS NULL", machineID, "display").
		Count(&pending).Error; err != nil || pending == 0 {
		return changes
	}
	return append(changes[:idx:idx], changes[idx+1:]...)
}

// ghiThayDoiPhanCung ghi các dòng nhật ký và báo trang quản trị. Lỗi ghi nhật ký chỉ được
// log: nhịp tim đã lưu xong, và máy chủ không được báo "lỗi" cho máy trạm chỉ vì một dòng
// nhật ký.
func (s *MachineService) ghiThayDoiPhanCung(machine *model.Machine, changes []hwChange, doi bool, now time.Time) {
	changes = s.boTutHzLapLai(machine.ID, changes)
	if len(changes) > 0 {
		rows := make([]model.MachineHardwareChange, len(changes))
		for i, c := range changes {
			rows[i] = model.MachineHardwareChange{
				MachineID: machine.ID, Component: c.Component, Action: c.Action,
				OldValue: c.Old, NewValue: c.New, DetectedAt: now,
			}
		}
		if err := s.db.Create(&rows).Error; err != nil {
			log.Printf("[phần cứng] không ghi được thay đổi của máy %s: %v", machine.MachineCode, err)
		} else if s.hub != nil {
			s.hub.BroadcastToType(hub.Event{
				Type: "machine:alert",
				Data: map[string]interface{}{
					"machine_id":   machine.ID,
					"machine_code": machine.MachineCode,
					"kind":         "hardware_change",
					"detail":       hwSummary(changes),
				},
			}, hub.ClientTypeAdmin)
		}
	}
	if (doi || len(changes) > 0) && s.hub != nil {
		s.hub.Changed("machine-hardware", map[string]interface{}{"id": machine.ID, "machine_code": machine.MachineCode})
	}
}
