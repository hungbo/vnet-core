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
	"github.com/vnet/core/pkg/utils"
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
	// IsActive là công tắc tạm ngừng máy, và là ĐƯỜNG THAY CHO VIỆC XOÁ.
	//
	// Máy đã có phiên chơi, đơn hàng hay lịch đặt thì không xoá được nữa (xem
	// Delete bên dưới): lịch sử phải giữ lại. Không có công tắc này thì quản trị
	// viên muốn ngừng một máy hỏng chỉ còn cách xoá — đúng thao tác nguy hiểm
	// nhất trong service này.
	//
	// StartSession lọc "is_active = true" (session.go), nên đặt false là chặn
	// mở phiên mới ngay lập tức. List KHÔNG lọc cột này, nên máy đã tắt vẫn hiện
	// trên trang quản trị để bật lại được.
	//
	// Con trỏ vì đây là bản vá từng phần: bỏ trống nghĩa là không đổi.
	IsActive *bool `json:"is_active"`
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

	// Ô tìm kiếm trên trang Máy ghi "Tìm mã máy / nhóm" nhưng hàm này bỏ qua
	// hẳn tham số search: gõ gì cũng ra đủ danh sách máy. Quán vài trăm máy thì
	// đó là ô nhập duy nhất để tìm một máy cụ thể.
	//
	// Nhóm tra bằng truy vấn con chứ không join: join làm hỏng Count ở dưới khi
	// một máy khớp cả hai vế. Nhóm xoá mềm không được tính.
	if params.Search != "" {
		search := "%" + params.Search + "%"
		query = query.Where(
			"unaccent(machine_code) ILIKE unaccent(?) OR group_id IN (SELECT id FROM machine_groups WHERE unaccent(name) ILIKE unaccent(?) AND deleted_at IS NULL)",
			search, search,
		)
	}

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
			return nil, errors.New("không tìm thấy máy")
		}
		return nil, err
	}
	return &machine, nil
}

func (s *MachineService) GetByCode(code string) (*model.Machine, error) {
	var machine model.Machine
	if err := s.db.Where("machine_code = ?", code).First(&machine).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy máy")
		}
		return nil, err
	}
	return &machine, nil
}

// nhomRongThanhNil biến con trỏ trỏ vào chuỗi rỗng thành nil.
//
// group_id là cột uuid cho phép rỗng. Bỏ chọn nhóm trên giao diện gửi xuống ""
// chứ không phải null, mà PostgreSQL từ chối "" cho kiểu uuid — lỗi thô
// `invalid input syntax for type uuid: ""` lọt thẳng ra người dùng ở cả ba
// đường: tạo một máy, sửa máy, và tạo máy hàng loạt. Không nhóm thì là NULL.
func nhomRongThanhNil(id *string) *string {
	if id == nil || strings.TrimSpace(*id) == "" {
		return nil
	}
	return id
}

func (s *MachineService) Create(req *CreateMachineRequest) (*model.Machine, error) {
	machine := model.Machine{
		MachineCode: req.MachineCode,
		GroupID:     nhomRongThanhNil(req.GroupID),
		CPUName:     req.CPUName,
		RAMGB:       req.RAMGB,
		GPUName:     req.GPUName,
		StorageGB:   req.StorageGB,
		OSInfo:      req.OSInfo,
		Status:      "offline",
		IsActive:    true,
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
	return &machine, nil
}

// machineCodeMaxLen khớp varchar(20) của Machine.MachineCode. Kiểm ở đây để một
// tiền tố quá dài bị chặn với thông điệp tiếng Việt, thay vì để PostgreSQL cắt
// ngang giữa lô bằng lỗi thô.
const machineCodeMaxLen = 20

// maxBatchMachines chặn một lần bấm nhầm sinh ra hàng chục nghìn dòng. Quán lớn
// nhất cũng dưới vài trăm máy.
const maxBatchMachines = 500

// BatchCreateMachinesRequest sinh một dải máy theo tiền tố và khoảng số:
// Prefix="PC-", From=1, To=50, Digits=2  ->  PC-01 ... PC-50.
//
// Digits là số chữ số tối thiểu, đệm 0 vào trước. Để 0 thì không đệm (PC-1).
type BatchCreateMachinesRequest struct {
	Prefix  string  `json:"prefix" binding:"required"`
	From    int     `json:"from" binding:"min=0"`
	To      int     `json:"to" binding:"min=0"`
	Digits  int     `json:"digits" binding:"min=0,max=10"`
	GroupID *string `json:"group_id"`

	// Cấu hình áp chung cho cả lô. Máy trạm sẽ ghi đè bằng số đo thật ở nhịp
	// tim đầu tiên, nên đây chỉ là giá trị tạm để nhìn cho có.
	CPUName   string `json:"cpu_name"`
	RAMGB     int    `json:"ram_gb"`
	GPUName   string `json:"gpu_name"`
	StorageGB int    `json:"storage_gb"`
	OSInfo    string `json:"os_info"`

	// DryRun chỉ kiểm rồi trả kết quả, không ghi gì. Giao diện gọi trước khi
	// tạo thật để báo trước có tạo được hay không.
	DryRun bool `json:"dry_run"`
}

// BatchCreateResult mô tả một lô: sẽ tạo những mã nào, mã nào vướng, và vướng vì
// lý do gì.
type BatchCreateResult struct {
	Codes []string `json:"codes"`
	// Conflicts là mã đã có máy đang dùng.
	Conflicts []string `json:"conflicts"`
	// Deleted là mã thuộc về máy ĐÃ XOÁ. Tách riêng khỏi Conflicts vì người
	// dùng không thấy chúng ở đâu trên màn hình: machine_code có unique index
	// thường (không phải partial theo deleted_at), nên máy xoá mềm vẫn giữ mã
	// và vẫn chặn mã đó. Gộp chung hai loại thì thông điệp thành "PC-05 đã tồn
	// tại" trong khi danh sách máy không có PC-05 nào.
	Deleted []string `json:"deleted"`
	Created int      `json:"created"`
	DryRun  bool     `json:"dry_run"`
}

// OK cho biết lô này tạo được hay không.
func (r *BatchCreateResult) OK() bool {
	return len(r.Conflicts) == 0 && len(r.Deleted) == 0
}

// MoTaVuong gom mã vướng thành một câu đọc được. Cắt bớt khi quá dài — liệt kê
// 200 mã trong một toast thì không ai đọc.
func (r *BatchCreateResult) MoTaVuong() string {
	var phan []string
	if n := len(r.Conflicts); n > 0 {
		phan = append(phan, fmt.Sprintf("%d mã đã có máy: %s", n, gomMa(r.Conflicts)))
	}
	if n := len(r.Deleted); n > 0 {
		phan = append(phan, fmt.Sprintf("%d mã thuộc máy đã xoá: %s", n, gomMa(r.Deleted)))
	}
	return strings.Join(phan, "; ")
}

func gomMa(ds []string) string {
	const toiDa = 10
	if len(ds) <= toiDa {
		return strings.Join(ds, ", ")
	}
	return fmt.Sprintf("%s… và %d mã khác", strings.Join(ds[:toiDa], ", "), len(ds)-toiDa)
}

// BatchCreateMachines tạo cả dải máy trong MỘT giao dịch.
//
// Toàn bộ hoặc không có gì: vướng dù chỉ một mã thì không máy nào được tạo. Tạo
// nửa vời rồi bắt người dùng tự dò xem thiếu máy nào là cách chắc chắn để có một
// dãy máy thủng lỗ chỗ mà không ai biết.
func (s *MachineService) BatchCreateMachines(req *BatchCreateMachinesRequest) (*BatchCreateResult, error) {
	prefix := strings.TrimSpace(req.Prefix)
	if prefix == "" {
		return nil, errors.New("thiếu tiền tố mã máy")
	}
	if req.To < req.From {
		return nil, errors.New("số cuối phải lớn hơn hoặc bằng số đầu")
	}
	if n := req.To - req.From + 1; n > maxBatchMachines {
		return nil, fmt.Errorf("một lần tạo tối đa %d máy, đang yêu cầu %d", maxBatchMachines, n)
	}

	codes := make([]string, 0, req.To-req.From+1)
	for n := req.From; n <= req.To; n++ {
		code := fmt.Sprintf("%s%0*d", prefix, req.Digits, n)
		if len(code) > machineCodeMaxLen {
			return nil, fmt.Errorf("mã %q dài %d ký tự, vượt trần %d — hãy rút ngắn tiền tố", code, len(code), machineCodeMaxLen)
		}
		codes = append(codes, code)
	}

	// Không cần chống trùng trong chính lô: cùng một tiền tố, hai số khác nhau
	// luôn cho hai mã khác nhau, kể cả khi đệm 0.
	//
	// Unscoped: máy xoá mềm vẫn giữ mã và vẫn chặn mã đó.
	var daCo []model.Machine
	if err := s.db.Unscoped().
		Select("machine_code", "deleted_at").
		Where("machine_code IN ?", codes).
		Find(&daCo).Error; err != nil {
		return nil, err
	}

	res := &BatchCreateResult{Codes: codes, DryRun: req.DryRun}
	for _, m := range daCo {
		if m.DeletedAt.Valid {
			res.Deleted = append(res.Deleted, m.MachineCode)
		} else {
			res.Conflicts = append(res.Conflicts, m.MachineCode)
		}
	}
	sort.Strings(res.Conflicts)
	sort.Strings(res.Deleted)

	if req.DryRun || !res.OK() {
		return res, nil
	}

	machines := make([]model.Machine, 0, len(codes))
	for _, code := range codes {
		machines = append(machines, model.Machine{
			MachineCode: code,
			GroupID:     nhomRongThanhNil(req.GroupID),
			CPUName:     req.CPUName,
			RAMGB:       req.RAMGB,
			GPUName:     req.GPUName,
			StorageGB:   req.StorageGB,
			OSInfo:      req.OSInfo,
			Status:      "offline",
			IsActive:    true,
		})
	}
	if err := s.db.Create(&machines).Error; err != nil {
		return nil, err
	}
	res.Created = len(machines)

	// Một dòng nhật ký cho cả lô, không phải mỗi máy một dòng: người đọc nhật ký
	// muốn biết "ai tạo dải PC-01..PC-50 lúc nào", không muốn cuộn qua 50 dòng.
	_ = s.audit.Log(&LogAuditRequest{
		Action:     "batch_create",
		EntityType: "machine",
		Metadata: map[string]interface{}{
			"prefix": prefix,
			"from":   req.From,
			"to":     req.To,
			"count":  len(machines),
		},
	})
	return res, nil
}

func (s *MachineService) Update(id string, req *UpdateMachineRequest) (*model.Machine, error) {
	var machine model.Machine
	if err := s.db.Where("id = ?", id).First(&machine).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy máy")
		}
		return nil, err
	}
	updates := map[string]interface{}{}
	if req.GroupID != nil {
		// Gán nil chứ không phải "": GORM ghi NULL, còn "" thì PostgreSQL từ
		// chối. Bỏ nhóm của một máy là thao tác hợp lệ.
		if g := nhomRongThanhNil(req.GroupID); g == nil {
			updates["group_id"] = nil
		} else {
			updates["group_id"] = *g
		}
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
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
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

// Delete xoá một máy.
//
// Máy đã từng chạy là máy có lịch sử: phiên chơi, đơn hàng gọi tại máy, lịch
// đặt, đánh giá dịch vụ. Toàn bộ những thứ đó là chứng từ và phải giữ — chính
// vì thế mà mười một chỗ trong service này phải dùng Unscoped() để còn hiện
// được mã máy của một máy đã xoá. Đường đúng cho máy hỏng hay máy tạm ngừng là
// tắt is_active (UpdateMachineRequest.IsActive), StartSession sẽ từ chối mở
// phiên trên nó ngay.
//
// machine_hardware_snapshots và website_blocking_violations KHÔNG chặn: đó là
// telemetry và nhật ký, không phải chứng từ. Bảng snapshot cũng là bảng lớn
// nhất hệ thống và đã có job dọn định kỳ riêng.
func (s *MachineService) Delete(id string) error {
	var machine model.Machine
	if err := s.db.Where("id = ?", id).First(&machine).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("không tìm thấy máy")
		}
		return err
	}
	if err := kiemTraPhuThuoc(s.db, id, []phuThuoc{
		{Bang: &model.MachineSession{}, Cot: "machine_id", Nhan: "phiên chơi"},
		{Bang: &model.Order{}, Cot: "machine_id", Nhan: "đơn hàng"},
		{Bang: &model.MachineBooking{}, Cot: "machine_id", Nhan: "lịch đặt máy"},
		{Bang: &model.ServiceFeedback{}, Cot: "machine_id", Nhan: "đánh giá dịch vụ"},
	}, "hãy tắt hoạt động máy thay vì xoá"); err != nil {
		return err
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		// Tài sản gắn máy (bàn phím, chuột, tai nghe) là con SỞ HỮU: không còn
		// máy thì không còn chỗ để gắn.
		if err := tx.Where("machine_id = ?", id).Delete(&model.MachineAsset{}).Error; err != nil {
			return err
		}
		return tx.Delete(&machine).Error
	}); err != nil {
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
	// Ba lệnh giám sát. Máy trạm làm xong thì báo NGƯỢC dữ liệu lên bằng HTTP
	// (route by-code) chứ không qua WebSocket: readPump của hub
	// giới hạn 4 KB chiều lên, mà ảnh chụp màn hình cỡ vài trăm KB.
	"screenshot":   "chụp màn hình máy",
	"process-list": "xem tiến trình đang chạy",
	"process-kill": "tắt một tiến trình",
}

// wantsReport là những lệnh mà máy trạm sẽ trả dữ liệu về sau, nên cần một
// request_id để trang quản trị khớp câu trả lời với đúng lần bấm.
var wantsReport = map[string]bool{
	"screenshot":   true,
	"process-list": true,
	"process-kill": true,
}

// ErrMachineOffline được handler dùng để trả 409 thay vì 400.
var ErrMachineOffline = hub.ErrMachineOffline

// ScreenshotReport là dữ liệu máy trạm gửi về sau lệnh remote:screenshot.
type ScreenshotReport struct {
	RequestID string `json:"request_id"`
	Image     string `json:"image"` // data URI
}

// ProcessInfo là một dòng trong bảng tiến trình — đã gộp theo tên phía máy trạm.
type ProcessInfo struct {
	Name  string `json:"name"`
	Count int    `json:"count"`  // số tiến trình cùng tên
	RAMMB int64  `json:"ram_mb"` // tổng RAM, MB
}

// ProcessReport là dữ liệu máy trạm gửi về sau remote:process-list / process-kill.
type ProcessReport struct {
	RequestID string        `json:"request_id"`
	Processes []ProcessInfo `json:"processes"`
	// Killed chỉ có với process-kill: số tiến trình vừa tắt được. -1 nếu chỉ là
	// process-list (không phải lệnh tắt).
	Killed int `json:"killed"`
}

// ReportScreenshot nhận ảnh máy trạm gửi lên rồi đẩy tới mọi trang quản trị.
//
// Không lưu xuống đĩa: ảnh đi thẳng qua WebSocket tới admin rồi thôi. Lưu lại
// nghĩa là một kho ảnh màn hình khách nằm trên ổ cứng, kèm chính sách dọn rác
// và một câu hỏi pháp lý — không đáng, vì nhân viên chỉ cần nhìn một lần.
func (s *MachineService) ReportScreenshot(machineCode string, req *ScreenshotReport) error {
	var machine model.Machine
	if err := s.db.Select("id, machine_code").Where("machine_code = ?", machineCode).
		First(&machine).Error; err != nil {
		return errors.New("không tìm thấy máy")
	}
	s.hub.BroadcastToType(hub.Event{
		Type: "machine:screenshot",
		Data: map[string]interface{}{
			"request_id":   req.RequestID,
			"machine_id":   machine.ID,
			"machine_code": machine.MachineCode,
			"image":        req.Image,
		},
	}, hub.ClientTypeAdmin)
	return nil
}

// ReportProcesses nhận danh sách tiến trình máy trạm gửi lên rồi đẩy tới admin.
func (s *MachineService) ReportProcesses(machineCode string, req *ProcessReport) error {
	var machine model.Machine
	if err := s.db.Select("id, machine_code").Where("machine_code = ?", machineCode).
		First(&machine).Error; err != nil {
		return errors.New("không tìm thấy máy")
	}
	s.hub.BroadcastToType(hub.Event{
		Type: "machine:processes",
		Data: map[string]interface{}{
			"request_id":   req.RequestID,
			"machine_id":   machine.ID,
			"machine_code": machine.MachineCode,
			"processes":    req.Processes,
			"killed":       req.Killed,
		},
	}, hub.ClientTypeAdmin)
	return nil
}

// RemoteActionResult mang theo kết quả chốt tiền khi lệnh làm kết thúc phiên,
// để nhân viên thấy ngay đã thu bao nhiêu chứ không phải mở trang khác kiểm.
type RemoteActionResult struct {
	Action  string              `json:"action"`
	Session *EndSessionResponse `json:"session,omitempty"`
	// RequestID chỉ có với các lệnh giám sát. Trang quản trị giữ nó rồi khớp
	// với sự kiện machine:screenshot / machine:processes bay về sau đó, để biết
	// ảnh nào là của lần bấm nào — quan trọng khi mở nhiều máy cùng lúc.
	RequestID string `json:"request_id,omitempty"`
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

	// Với lệnh giám sát: sinh request_id để máy trạm echo lại khi báo dữ liệu
	// về. Nuốt lỗi rand là chấp nhận được — request_id rỗng chỉ làm mất khả năng
	// khớp nhiều máy cùng lúc, không làm hỏng lệnh.
	requestID := ""
	if wantsReport[action] {
		requestID, _ = utils.GenerateRandomToken(8)
	}

	// request_id nằm NGANG HÀNG với payload, không lồng vào trong: payload là
	// thứ máy trạm đọc field theo tên (reason/title/process...), chèn vào đó là
	// làm lệch mọi lệnh cũ.
	data := map[string]interface{}{
		"machine_id":   id,
		"machine_code": machine.MachineCode,
		"action":       action,
		"payload":      payload,
	}
	if requestID != "" {
		data["request_id"] = requestID
	}

	event := hub.Event{Type: "remote:" + action, Data: data}

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

	result := &RemoteActionResult{Action: action, RequestID: requestID}

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

// ListGroups lọc theo tên nhóm khi có từ khoá.
//
// Trang quản trị vẫn gửi ?search= từ trước nhưng hàm này bỏ qua hẳn: gõ gì vào
// ô tìm kiếm cũng ra đủ danh sách, không lỗi, không dấu hiệu gì. Bỏ dấu khi so
// khớp cho giống ô tìm hội viên và nhóm hội viên.
func (s *MachineService) ListGroups(search string) ([]model.MachineGroup, error) {
	query := s.db.Order("sort_order asc")
	if search != "" {
		query = query.Where("unaccent(name) ILIKE unaccent(?)", "%"+search+"%")
	}

	var groups []model.MachineGroup
	if err := query.Find(&groups).Error; err != nil {
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
			return nil, errors.New("không tìm thấy nhóm máy")
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

// DeleteGroup xoá một nhóm máy.
//
// Nhóm máy không có cột is_active nên không có đường "tạm ngừng" — nhưng mọi
// phụ thuộc ở đây đều SỬA ĐƯỢC: chuyển máy sang nhóm khác, xoá dòng giá. Thông
// điệp phải nói ra điều đó, chặn mà không chỉ lối thoát là bịt kín.
func (s *MachineService) DeleteGroup(id string) error {
	var group model.MachineGroup
	if err := s.db.Where("id = ?", id).First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("không tìm thấy nhóm máy")
		}
		return err
	}
	if err := kiemTraPhuThuoc(s.db, id, []phuThuoc{
		{Bang: &model.Machine{}, Cot: "group_id", Nhan: "máy"},
		{Bang: &model.MachinePrice{}, Cot: "machine_group_id", Nhan: "dòng bảng giá theo hạng",
			GoiY: "hãy xoá chúng trong hộp thoại Bảng giá của nhóm trước"},
		{Bang: &model.TimeBasedPricing{}, Cot: "machine_group_id", Nhan: "khung giá theo giờ",
			GoiY: "hãy xoá chúng trong hộp thoại Bảng giá của nhóm trước"},
		{Bang: &model.WebsiteRuleMapping{}, Cot: "machine_group_id", Nhan: "luật chặn web gán cho nhóm",
			GoiY: "hãy gỡ nhóm này khỏi luật chặn web trước"},
	}, "hãy chuyển chúng sang nhóm khác trước"); err != nil {
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
			return nil, errors.New("không tìm thấy thiết bị của máy")
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

// DeleteAsset xoá một tài sản gắn với máy (bàn phím, chuột, tai nghe…).
//
// Đã rà internal/model: không bảng nào tham chiếu machine_assets, nó là lá của
// cây quan hệ. Không cần kiểm phụ thuộc.
func (s *MachineService) DeleteAsset(id string) error {
	var asset model.MachineAsset
	if err := s.db.Where("id = ?", id).First(&asset).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("không tìm thấy thiết bị của máy")
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
