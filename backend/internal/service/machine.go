package service

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
)

type MachineService struct {
	db       *gorm.DB
	hub      *hub.Hub
	audit    *AuditService
	sessions *SessionService
}

func NewMachineService(db *gorm.DB, wsHub *hub.Hub, audit *AuditService) *MachineService {
	return &MachineService{db: db, hub: wsHub, audit: audit}
}

// WithSessions nối dịch vụ phiên vào. Không có nó, tắt máy từ xa không chốt được
// tiền — xem ghi chú trong RemoteAction.
func (s *MachineService) WithSessions(sess *SessionService) *MachineService {
	s.sessions = sess
	return s
}

type CreateMachineRequest struct {
	MachineCode string  `json:"machine_code" binding:"required"`
	GroupID     *string `json:"group_id"`
	CPUName     string  `json:"cpu_name"`
	RAMGB       int     `json:"ram_gb"`
	GPUName     string  `json:"gpu_name"`
	StorageGB   int     `json:"storage_gb"`
	OSInfo      string  `json:"os_info"`
}

type UpdateMachineRequest struct {
	GroupID   *string `json:"group_id"`
	CPUName   *string `json:"cpu_name"`
	RAMGB     *int    `json:"ram_gb"`
	GPUName   *string `json:"gpu_name"`
	StorageGB *int    `json:"storage_gb"`
	OSInfo    *string `json:"os_info"`
	// Ba trạng thái, không hơn.
	//
	// omitempty vì đây là bản vá từng phần: bỏ trống nghĩa là không đổi trạng
	// thái, và oneof chỉ chạy khi có giá trị. Không có ràng buộc này thì
	// {"status":"abcxyz"} ghi thẳng vào cột: máy đó rơi ra ngoài mọi phép so
	// sánh — không bị chặn mở phiên vì nó không phải "in_use", và trang quản
	// trị hiện chuỗi thô vì không có nhãn nào khớp.
	//
	// "maintenance" từng nằm trong danh sách nhưng chưa bao giờ là trạng thái
	// thật: không dòng code nào đặt hay đọc nó, chỉ có một nhãn trên giao diện.
	Status *string `json:"status" binding:"omitempty,oneof=offline available in_use"`
}

type HeartbeatRequest struct {
	CPUTemp float64 `json:"cpu_temp"`
	GPUTemp float64 `json:"gpu_temp"`
	IP      string  `json:"ip"`
	MAC     string  `json:"mac"`
	// Máy trạm vẫn luôn gửi ba chỉ số này nhưng DTO cũ không có nên bị bỏ đi,
	// dù bảng lịch sử phần cứng có sẵn cột để lưu.
	CPUUsage  float64 `json:"cpu_usage"`
	RAMUsage  float64 `json:"ram_usage"`
	DiskUsage float64 `json:"disk_usage"`
	Uptime    int64   `json:"uptime"`

	// Cấu hình máy do máy trạm tự khai. Bốn cột này trước đây chỉ nhập tay ở
	// trang quản trị, trong khi máy trạm đọc được chính xác hơn người gõ.
	// Đều là tuỳ chọn: gói tin không khai thì giữ nguyên giá trị đang có.
	CPUName   string `json:"cpu_name"`
	GPUName   string `json:"gpu_name"`
	RAMGB     int    `json:"ram_gb"`
	StorageGB int    `json:"storage_gb"`
}

type CreateMachineGroupRequest struct {
	Name         string `json:"name" binding:"required"`
	Description  string `json:"description"`
	Color        string `json:"color"`
	PricePerHour int64  `json:"price_per_hour"`
	SortOrder    int    `json:"sort_order"`
}

type UpdateMachineGroupRequest struct {
	Name         *string `json:"name"`
	Description  *string `json:"description"`
	Color        *string `json:"color"`
	PricePerHour *int64  `json:"price_per_hour"`
	SortOrder    *int    `json:"sort_order"`
}

type CreateMachineAssetRequest struct {
	MachineID string `json:"machine_id" binding:"required"`
	AssetType string `json:"asset_type" binding:"required"`
	Brand     string `json:"brand"`
	Model     string `json:"model"`
	Serial    string `json:"serial"`
	Status    string `json:"status"`
	Notes     string `json:"notes"`
	// Ảnh chụp lúc kiểm tra. Trường này có trong model từ đầu nhưng request
	// không hề nhận nó, nên chưa bao giờ lưu được ảnh nào.
	CheckPhotos []string `json:"check_photos"`
}

type UpdateMachineAssetRequest struct {
	AssetType   *string   `json:"asset_type"`
	Brand       *string   `json:"brand"`
	Model       *string   `json:"model"`
	Serial      *string   `json:"serial"`
	Status      *string   `json:"status"`
	Notes       *string   `json:"notes"`
	CheckPhotos *[]string `json:"check_photos"`
}

func (s *MachineService) List(params pagination.Params) (*pagination.Result, error) {
	var machines []model.Machine
	query := s.db.Model(&model.Machine{})
	var total int64
	if err := query.Model(&model.Machine{}).Count(&total).Error; err != nil {
		return nil, err
	}
	if err := pagination.Apply(query, &params).Find(&machines).Error; err != nil {
		return nil, err
	}
	return pagination.NewResult(machines, total, &params), nil
}

func (s *MachineService) GetByID(id string) (*model.Machine, error) {
	var machine model.Machine
	if err := s.db.Where("id = ?", id).First(&machine).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("machine not found")
		}
		return nil, err
	}
	return &machine, nil
}

func (s *MachineService) GetByCode(code string) (*model.Machine, error) {
	var machine model.Machine
	if err := s.db.Where("machine_code = ?", code).First(&machine).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("machine not found")
		}
		return nil, err
	}
	return &machine, nil
}

// CreateResult kèm khoá máy trạm ở dạng THÔ. Đây là lần duy nhất khoá xuất
// hiện — máy chủ chỉ lưu băm. Ghi vào cấu hình máy trạm ngay.
type CreateMachineResult struct {
	*model.Machine
	AgentToken string `json:"agent_token"`
}

func (s *MachineService) Create(req *CreateMachineRequest) (*CreateMachineResult, error) {
	machine := model.Machine{
		MachineCode: req.MachineCode,
		GroupID:     req.GroupID,
		CPUName:     req.CPUName,
		RAMGB:       req.RAMGB,
		GPUName:     req.GPUName,
		StorageGB:   req.StorageGB,
		OSInfo:      req.OSInfo,
		Status:      "offline",
		IsActive:    true,
		// KHÔNG cấp khoá tự động. Mỗi máy một khoá riêng nghĩa là quán 50 máy
		// phải chép tay 50 chuỗi, và chép nhầm một ký tự thì triệu chứng là máy
		// vẫn chạy nhưng trang quản trị chỉ nói "Ngoại tuyến" — không một chữ
		// nào cho biết vì sao. Máy trạm cắm vào là chạy.
		//
		// Ai muốn siết thì bấm "Cấp khoá" ở trang Máy: từ lúc đó máy chủ bắt
		// buộc khoá cho đúng máy đó, còn những máy khác vẫn không cần.
	}
	if err := s.db.Create(&machine).Error; err != nil {
		return nil, err
	}
	_ = s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "machine",
		EntityID:   machine.ID,
		Metadata:   map[string]interface{}{"machine_code": machine.MachineCode},
	})
	return &CreateMachineResult{Machine: &machine}, nil
}

// IssueAgentToken cấp lại khoá cho một máy. Khoá cũ mất hiệu lực ngay — dùng
// khi máy trạm bị thay hoặc nghi khoá lộ.
func (s *MachineService) IssueAgentToken(id, actorID string) (string, error) {
	var machine model.Machine
	if err := s.db.First(&machine, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", errors.New("không tìm thấy máy")
		}
		return "", err
	}

	token, err := randomString(cardSecretLen * 2)
	if err != nil {
		return "", err
	}
	now := time.Now()
	if err := s.db.Model(&machine).Updates(map[string]interface{}{
		"agent_token": hashSecret(token), "agent_token_issued_at": &now,
	}).Error; err != nil {
		return "", err
	}

	var actor *string
	if actorID != "" {
		actor = &actorID
	}
	_ = s.audit.Log(&LogAuditRequest{
		Action:     "issue_agent_token",
		EntityType: "machine",
		EntityID:   machine.ID,
		UserID:     actor,
		Metadata:   map[string]interface{}{"machine_code": machine.MachineCode},
	})
	return token, nil
}

var (
	ErrMachineUnknown    = errors.New("không có máy nào mang mã này")
	ErrAgentTokenInvalid = errors.New("khoá máy trạm không đúng")
)

// VerifyAgentToken kiểm khoá kèm theo báo cáo của máy trạm.
//
// Khoá là TUỲ CHỌN. Máy chưa được cấp khoá thì chỉ cần mã máy có thật là qua —
// cắm máy vào là chạy, không phải chép chuỗi bí mật cho từng máy một.
//
// Cấp khoá cho một máy (nút "Cấp khoá" ở trang Máy) là bật ràng buộc cho riêng
// máy đó: từ lúc ấy báo cáo thiếu khoá hoặc sai khoá đều bị từ chối. Đánh đổi
// phải nói rõ: không có khoá thì bất kỳ ai trong mạng đoán được mã máy đều gửi
// được báo cáo giả và nhận được lệnh điều khiển dành cho máy đó — mà trong quán
// net, khách ngồi ngay trên cùng mạng ấy.
func (s *MachineService) VerifyAgentToken(machineCode, token string) error {
	var machine model.Machine
	if err := s.db.Select("machine_code, agent_token").
		Where("machine_code = ?", machineCode).First(&machine).Error; err != nil {
		return ErrMachineUnknown
	}
	if machine.AgentToken == "" {
		return nil
	}
	if !secretMatches(machine.AgentToken, token) {
		return ErrAgentTokenInvalid
	}
	return nil
}

func (s *MachineService) Update(id string, req *UpdateMachineRequest) (*model.Machine, error) {
	var machine model.Machine
	if err := s.db.Where("id = ?", id).First(&machine).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("machine not found")
		}
		return nil, err
	}
	updates := map[string]interface{}{}
	if req.GroupID != nil {
		updates["group_id"] = *req.GroupID
	}
	if req.CPUName != nil {
		updates["cpu_name"] = *req.CPUName
	}
	if req.RAMGB != nil {
		updates["ram_gb"] = *req.RAMGB
	}
	if req.GPUName != nil {
		updates["gpu_name"] = *req.GPUName
	}
	if req.StorageGB != nil {
		updates["storage_gb"] = *req.StorageGB
	}
	if req.OSInfo != nil {
		updates["os_info"] = *req.OSInfo
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if len(updates) > 0 {
		updates["updated_at"] = time.Now()
		if err := s.db.Model(&machine).Updates(updates).Error; err != nil {
			return nil, err
		}
		_ = s.audit.Log(&LogAuditRequest{
			Action:     "update",
			EntityType: "machine",
			EntityID:   machine.ID,
			Metadata:   map[string]interface{}{"machine_code": machine.MachineCode},
		})
	}
	return &machine, nil
}

func (s *MachineService) Delete(id string) error {
	var machine model.Machine
	if err := s.db.Where("id = ?", id).First(&machine).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("machine not found")
		}
		return err
	}
	if err := s.db.Delete(&machine).Error; err != nil {
		return err
	}
	_ = s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "machine",
		EntityID:   machine.ID,
		Metadata:   map[string]interface{}{"machine_code": machine.MachineCode},
	})
	return nil
}

// truncateRunes cắt theo KÝ TỰ chứ không theo byte: varchar(100) của PostgreSQL
// đếm ký tự, và cắt giữa một ký tự nhiều byte sẽ tạo chuỗi UTF-8 hỏng.
func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

func (s *MachineService) Heartbeat(id string, req HeartbeatRequest) error {
	cpuTemp, gpuTemp := req.CPUTemp, req.GPUTemp
	var machine model.Machine
	if err := s.db.Where("id = ?", id).First(&machine).Error; err != nil {
		return err
	}
	now := time.Now()
	updates := map[string]interface{}{
		"cpu_temp":       cpuTemp,
		"gpu_temp":       gpuTemp,
		"last_heartbeat": now,
		"updated_at":     now,
	}
	// Chỉ ghi đè khi gói tin thật sự khai. Trước đây ghi vô điều kiện, mà vòng
	// giám sát 60 giây gửi gói không có ip/mac — cứ mỗi phút địa chỉ IP và MAC
	// của máy lại bị xoá trắng rồi nhịp tim sau mới ghi lại. Trên trang Máy nó
	// hiện ra thành hai cột nhấp nháy lúc có lúc không.
	if req.IP != "" {
		updates["ip_address"] = req.IP
	}
	if req.MAC != "" {
		updates["mac_address"] = req.MAC
	}
	// Cấu hình máy tự khai thắng số nhập tay: nó đo được, còn người thì gõ nhầm.
	// Cắt đúng bề rộng cột — tên CPU đầy đủ của Intel dài hơn 100 ký tự là
	// chuyện thường, và PostgreSQL từ chối cả câu lệnh chứ không cắt hộ.
	if req.CPUName != "" {
		updates["cpu_name"] = truncateRunes(req.CPUName, 100)
	}
	if req.GPUName != "" {
		updates["gpu_name"] = truncateRunes(req.GPUName, 100)
	}
	if req.RAMGB > 0 {
		updates["ram_gb"] = req.RAMGB
	}
	if req.StorageGB > 0 {
		updates["storage_gb"] = req.StorageGB
	}
	if machine.Status == "offline" {
		updates["status"] = "available"
	}
	// Suy ra thời điểm khởi động từ uptime. Chỉ là hiệu của hai giá trị đồng
	// hồ khác nhau nhưng uptime là KHOẢNG (không phải mốc thời gian) nên không
	// dính lệch đồng hồ — chỉ lệch vài giây do độ trễ mạng, vô hại với ngưỡng
	// hai phút mà lớp đóng phiên dùng. uptime chỉ giảm khi reboot, nên mốc này
	// đứng yên suốt một phiên máy và nhảy về hiện tại mỗi lần bật lại.
	if req.Uptime > 0 {
		bootedAt := now.Add(-time.Duration(req.Uptime) * time.Second)
		updates["booted_at"] = bootedAt
	}
	if err := s.db.Model(&machine).Updates(updates).Error; err != nil {
		return err
	}
	_ = s.audit.Log(&LogAuditRequest{
		Action:     "heartbeat",
		EntityType: "machine",
		EntityID:   id,
		Metadata:   map[string]interface{}{"machine_code": machine.MachineCode},
	})
	snapshot := model.MachineHardwareSnapshot{
		MachineID: id,
		CPUTemp:   cpuTemp,
		GPUTemp:   gpuTemp,
		CPUUsage:  req.CPUUsage,
		RAMUsage:  req.RAMUsage,
		DiskUsage: req.DiskUsage,
		Uptime:    req.Uptime,
	}
	s.db.Create(&snapshot)
	return nil
}

func (s *MachineService) GetHardwareHistory(id string, params pagination.Params) (*pagination.Result, error) {
	var snapshots []model.MachineHardwareSnapshot
	// Sắp theo thời gian, không phải theo id. Mặc định của pagination là "id
	// desc", mà id là UUID ngẫu nhiên — nghĩa là bảng "50 số đo gần nhất" trên
	// màn hình thật ra là 50 dòng bất kỳ, xếp lộn xộn. Với vài dòng thì trông
	// vẫn hợp lý, nên lỗi này không tự lộ ra bao giờ.
	params.Sort = "created_at"
	params.Order = "desc"
	query := s.db.Where("machine_id = ?", id)
	var total int64
	if err := query.Model(&model.MachineHardwareSnapshot{}).Count(&total).Error; err != nil {
		return nil, err
	}
	if err := pagination.Apply(query, &params).Find(&snapshots).Error; err != nil {
		return nil, err
	}
	return pagination.NewResult(snapshots, total, &params), nil
}

// Lệnh điều khiển từ xa được phép. Trước đây :action là chuỗi tự do: gõ gì
// cũng thành một sự kiện "remote:<gì đó>" gửi thẳng xuống máy trạm, và một lỗi
// gõ sẽ im lặng không làm gì thay vì báo lỗi.
//
// Cố ý KHÔNG có lệnh chạy câu lệnh tuỳ ý. Máy khách có sẵn App.ExecuteCommand
// chạy `cmd /C` với chuỗi bất kỳ; nối nó vào đây sẽ biến mọi tài khoản nhân
// viên thành quyền thực thi mã trên toàn bộ máy trạm.
var remoteActions = map[string]string{
	"lock":     "khoá máy",
	"unlock":   "mở khoá máy",
	"shutdown": "tắt máy",
	"restart":  "khởi động lại máy",
	"message":  "gửi thông báo lên máy",
	// App.BlockApp/UnblockApp đã có sẵn trong máy khách nhưng không lối vào ở
	// cả hai phía: viết rồi mà chưa từng gọi được. Chặn theo TÊN tiến trình,
	// không phải câu lệnh tuỳ ý — xem ghi chú về ExecuteCommand ở trên.
	"block-app":   "chặn ứng dụng trên máy",
	"unblock-app": "bỏ chặn ứng dụng trên máy",
}

// ErrMachineOffline được handler dùng để trả 409 thay vì 400.
var ErrMachineOffline = hub.ErrMachineOffline

// RemoteActionResult mang theo kết quả chốt tiền khi lệnh làm kết thúc phiên,
// để nhân viên thấy ngay đã thu bao nhiêu chứ không phải mở trang khác kiểm.
type RemoteActionResult struct {
	Action  string              `json:"action"`
	Session *EndSessionResponse `json:"session,omitempty"`
}

// endsSession liệt kê những lệnh làm khách rời máy. Tắt máy và khởi động lại
// đều chấm dứt lượt chơi; khoá màn hình hay gửi thông báo thì không.
var endsSession = map[string]bool{
	"shutdown": true,
	"restart":  true,
}

func (s *MachineService) RemoteAction(id, action string, payload interface{}, actorID string) (*RemoteActionResult, error) {
	label, ok := remoteActions[action]
	if !ok {
		allowed := make([]string, 0, len(remoteActions))
		for a := range remoteActions {
			allowed = append(allowed, a)
		}
		sort.Strings(allowed)
		return nil, fmt.Errorf("lệnh %q không được hỗ trợ (chấp nhận: %s)", action, strings.Join(allowed, ", "))
	}

	var machine model.Machine
	if err := s.db.Select("id, machine_code").First(&machine, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy máy")
		}
		return nil, err
	}

	event := hub.Event{
		Type: "remote:" + action,
		Data: map[string]interface{}{
			"machine_id":   id,
			"machine_code": machine.MachineCode,
			"action":       action,
			"payload":      payload,
		},
	}

	err := s.hub.SendToMachine(machine.MachineCode, event)

	// Ghi nhật ký cả khi gửi hỏng: "ai đã cố tắt máy nào" là thông tin cần lưu
	// không kém "ai đã tắt được máy nào".
	var actor *string
	if actorID != "" {
		actor = &actorID
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "remote_" + action,
		EntityType: "machine",
		EntityID:   id,
		UserID:     actor,
		Metadata: map[string]interface{}{
			"machine_code": machine.MachineCode,
			"label":        label,
			"delivered":    err == nil,
			"error":        errText(err),
		},
	})

	if err != nil {
		return nil, err
	}

	result := &RemoteActionResult{Action: action}

	// Chốt phiên SAU khi lệnh đã tới được máy, không phải trước.
	//
	// Trước đây RemoteAction chỉ đẩy sự kiện WebSocket rồi ghi nhật ký, không
	// chạm gì tới phiên: tắt máy xong phiên vẫn is_active = true nên khách
	// không bị tính tiền, máy kẹt ở "in_use", và chính khách đó lần sau không
	// mở được máy nào vì tài khoản đang chơi ở một máy khác.
	//
	// Thứ tự này là chủ ý: lệnh không tới được máy (ErrMachineOffline) thì
	// không đụng vào phiên — không bao giờ tính tiền của ai khi không chắc
	// khách đã thật sự rời máy.
	if endsSession[action] {
		ended, err := s.endActiveSession(machine.ID)
		if err != nil {
			// KHÔNG nuốt lỗi này: máy đã nhận lệnh tắt nhưng tiền chưa chốt là
			// thứ nhân viên phải biết ngay, không phải một dòng log không ai đọc.
			return nil, fmt.Errorf("máy %s đã nhận lệnh %s nhưng chưa chốt được phiên: %w",
				machine.MachineCode, label, err)
		}
		result.Session = ended
	}

	return result, nil
}

// endActiveSession chốt phiên đang chạy trên máy. Máy trống thì bỏ qua im lặng —
// tắt một máy không có khách là chuyện bình thường.
func (s *MachineService) endActiveSession(machineID string) (*EndSessionResponse, error) {
	if s.sessions == nil {
		return nil, nil
	}
	var session model.MachineSession
	err := s.db.Select("id").Where("machine_id = ? AND is_active = ?", machineID, true).
		First(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.sessions.EndSession(session.ID)
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (s *MachineService) ListGroups() ([]model.MachineGroup, error) {
	var groups []model.MachineGroup
	if err := s.db.Order("sort_order asc").Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

func (s *MachineService) CreateGroup(req *CreateMachineGroupRequest) (*model.MachineGroup, error) {
	group := model.MachineGroup{
		Name:         req.Name,
		Description:  req.Description,
		Color:        req.Color,
		PricePerHour: req.PricePerHour,
		SortOrder:    req.SortOrder,
	}
	if err := s.db.Create(&group).Error; err != nil {
		return nil, err
	}
	_ = s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "machine_group",
		EntityID:   group.ID,
		Metadata:   map[string]interface{}{"name": group.Name},
	})
	return &group, nil
}

func (s *MachineService) UpdateGroup(id string, req *UpdateMachineGroupRequest) (*model.MachineGroup, error) {
	var group model.MachineGroup
	if err := s.db.Where("id = ?", id).First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("machine group not found")
		}
		return nil, err
	}
	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Color != nil {
		updates["color"] = *req.Color
	}
	if req.PricePerHour != nil {
		updates["price_per_hour"] = *req.PricePerHour
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	if len(updates) > 0 {
		if err := s.db.Model(&group).Updates(updates).Error; err != nil {
			return nil, err
		}
		_ = s.audit.Log(&LogAuditRequest{
			Action:     "update",
			EntityType: "machine_group",
			EntityID:   group.ID,
			Metadata:   map[string]interface{}{"name": group.Name},
		})
	}
	return &group, nil
}

func (s *MachineService) DeleteGroup(id string) error {
	var group model.MachineGroup
	if err := s.db.Where("id = ?", id).First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("machine group not found")
		}
		return err
	}
	if err := s.db.Delete(&group).Error; err != nil {
		return err
	}
	_ = s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "machine_group",
		EntityID:   group.ID,
		Metadata:   map[string]interface{}{"name": group.Name},
	})
	return nil
}

func (s *MachineService) ListAssets(machineID string) ([]model.MachineAsset, error) {
	var assets []model.MachineAsset
	query := s.db
	if machineID != "" {
		query = query.Where("machine_id = ?", machineID)
	}
	if err := query.Find(&assets).Error; err != nil {
		return nil, err
	}
	return assets, nil
}

func (s *MachineService) CreateAsset(req *CreateMachineAssetRequest) (*model.MachineAsset, error) {
	asset := model.MachineAsset{
		MachineID: req.MachineID,
		AssetType: req.AssetType,
		Brand:     req.Brand,
		Model:     req.Model,
		Serial:    req.Serial,
		Status:    req.Status,
		Notes:     req.Notes,
	}
	if len(req.CheckPhotos) > 0 {
		asset.CheckPhotos = model.StringArray(req.CheckPhotos)
	}
	if asset.Status == "" {
		asset.Status = "good"
	}
	if err := s.db.Create(&asset).Error; err != nil {
		return nil, err
	}
	_ = s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "machine_asset",
		EntityID:   asset.ID,
		Metadata:   map[string]interface{}{"machine_id": asset.MachineID, "asset_type": asset.AssetType},
	})
	return &asset, nil
}

func (s *MachineService) UpdateAsset(id string, req *UpdateMachineAssetRequest, actorID string) (*model.MachineAsset, error) {
	var asset model.MachineAsset
	if err := s.db.Where("id = ?", id).First(&asset).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("machine asset not found")
		}
		return nil, err
	}
	updates := map[string]interface{}{}
	if req.AssetType != nil {
		updates["asset_type"] = *req.AssetType
	}
	if req.Brand != nil {
		updates["brand"] = *req.Brand
	}
	if req.Model != nil {
		updates["model"] = *req.Model
	}
	if req.Serial != nil {
		updates["serial"] = *req.Serial
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.Notes != nil {
		updates["notes"] = *req.Notes
	}
	if req.CheckPhotos != nil {
		updates["check_photos"] = model.StringArray(*req.CheckPhotos)
	}
	// Đổi tình trạng hoặc thêm ảnh nghĩa là vừa có người đi kiểm; ghi lại ai và
	// lúc nào, nếu không thì hai cột đó mãi mãi rỗng.
	if req.Status != nil || req.CheckPhotos != nil {
		now := time.Now()
		updates["checked_at"] = &now
		if actorID != "" {
			updates["checked_by"] = actorID
		}
	}
	if len(updates) > 0 {
		updates["updated_at"] = time.Now()
		if err := s.db.Model(&asset).Updates(updates).Error; err != nil {
			return nil, err
		}
		_ = s.audit.Log(&LogAuditRequest{
			Action:     "update",
			EntityType: "machine_asset",
			EntityID:   asset.ID,
			UserID:     optionalUUID(actorID),
			Metadata:   map[string]interface{}{"machine_id": asset.MachineID, "asset_type": asset.AssetType},
		})
	}
	return &asset, nil
}

func (s *MachineService) DeleteAsset(id string) error {
	var asset model.MachineAsset
	if err := s.db.Where("id = ?", id).First(&asset).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("machine asset not found")
		}
		return err
	}
	if err := s.db.Delete(&asset).Error; err != nil {
		return err
	}
	_ = s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "machine_asset",
		EntityID:   asset.ID,
		Metadata:   map[string]interface{}{"machine_id": asset.MachineID, "asset_type": asset.AssetType},
	})
	return nil
}
