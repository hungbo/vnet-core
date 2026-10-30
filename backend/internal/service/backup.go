package service

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/hub"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
)

type BackupService struct {
	db    *gorm.DB
	cfg   *config.DatabaseConfig
	dir   string
	audit *AuditService
	hub   *hub.Hub
}

// NewBackupService needs the database settings because pg_dump is a separate
// process: it cannot inherit the connection the application already holds.
func NewBackupService(db *gorm.DB, cfg *config.DatabaseConfig, dir string, audit *AuditService) *BackupService {
	if dir == "" {
		dir = "backups"
	}
	return &BackupService{db: db, cfg: cfg, dir: dir, audit: audit}
}

// WithHub để báo trang Sao lưu khi pg_dump chạy nền xong. Yêu cầu tạo đã trả
// về từ lúc bản sao lưu còn "running", nên middleware DataChanged không biết
// lúc nó thành "completed" hay "failed".
func (s *BackupService) WithHub(h *hub.Hub) *BackupService {
	s.hub = h
	return s
}

type CreateBackupRequest struct {
	Notes     string  `json:"notes"`
	CreatedBy *string `json:"created_by"`
}

func (s *BackupService) List(params pagination.Params) ([]model.BackupLog, int64, int, int, error) {
	query := s.db.Model(&model.BackupLog{})

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	// backup_logs không có cột created_at — mốc thời gian của nó là started_at.
	// Mặc định của pagination là created_at nên phải chỉ định, nếu không truy
	// vấn đổ lỗi "column does not exist".
	if params.Sort == "" || params.Sort == pagination.DefaultSort {
		params.Sort = "started_at"
	}

	var backups []model.BackupLog
	if err := pagination.Apply(query, &params).Find(&backups).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	return backups, total, params.Page, params.PageSize, nil
}

func (s *BackupService) Create(req *CreateBackupRequest) (*model.BackupLog, error) {
	backup := model.BackupLog{
		FileName:  "backup_" + time.Now().Format("20060102_150405") + ".sql",
		Status:    "running",
		Notes:     req.Notes,
		CreatedBy: req.CreatedBy,
		StartedAt: time.Now(),
	}

	if err := s.db.Create(&backup).Error; err != nil {
		return nil, err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "backup_log",
		EntityID:   backup.ID,
		Metadata:   map[string]interface{}{"status": backup.Status},
	})

	go s.runPgDump(&backup)

	return &backup, nil
}

func (s *BackupService) Restore(id string) error {
	var backup model.BackupLog
	if err := s.db.Where("id = ?", id).First(&backup).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("không tìm thấy bản sao lưu")
		}
		return err
	}

	if backup.FilePath == "" {
		return errors.New("bản sao lưu này chưa có tệp trên đĩa")
	}

	if _, err := os.Stat(backup.FilePath); err != nil {
		return errors.New("không tìm thấy tệp sao lưu: " + backup.FilePath)
	}

	// Bản cũ truyền đường dẫn tệp vào --dbname, tức là lấy tên tệp làm tên
	// database — lệnh đó không thể nào chạy đúng. Tệp phải là tham số vị trí,
	// còn thông tin kết nối phải khai báo riêng.
	cmd := exec.Command("pg_restore",
		"--no-owner",
		"--clean",
		"--if-exists",
		"-h", s.cfg.Host,
		"-p", strconv.Itoa(s.cfg.Port),
		"-U", s.cfg.User,
		"-d", s.cfg.Name,
		backup.FilePath,
	)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+s.cfg.Password)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("phục hồi thất bại: %s", string(output))
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "restore",
		EntityType: "backup_log",
		EntityID:   id,
		Metadata:   map[string]interface{}{"backup_id": id},
	})

	return nil
}

// Delete xoá cả dòng nhật ký lẫn tệp .sql trên đĩa.
//
// Không có thao tác này thì mỗi lần bấm "Tạo sao lưu" là thêm một tệp vài trăm
// KB đến vài trăm MB nằm lại vĩnh viễn — đĩa của máy chủ quán đầy dần mà không
// ai có đường nào dọn ngoài việc SSH vào xoá tay.
func (s *BackupService) Delete(id string) error {
	var backup model.BackupLog
	if err := s.db.Where("id = ?", id).First(&backup).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("không tìm thấy bản sao lưu")
		}
		return err
	}

	// Bản sao lưu đang chạy chưa ghi xong tệp: xoá lúc này để lại một tệp cụt
	// mà không dòng nào trỏ tới.
	if backup.Status == "running" {
		return errors.New("bản sao lưu đang chạy — hãy đợi nó xong rồi mới xoá")
	}

	if err := s.removeFile(&backup); err != nil {
		return err
	}

	if err := s.db.Delete(&model.BackupLog{}, "id = ?", id).Error; err != nil {
		return err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "backup_log",
		EntityID:   id,
		Metadata:   map[string]interface{}{"file_name": backup.FileName},
	})

	return nil
}

// removeFile xoá tệp của một bản sao lưu. Chỉ cho phép xoá tệp nằm trong thư
// mục sao lưu: FilePath đến từ database; nếu ai đó sửa được cột này thì một
// đường dẫn kiểu ../../etc sẽ khiến máy chủ tự xoá tệp hệ thống của chính nó.
func (s *BackupService) removeFile(backup *model.BackupLog) error {
	if backup.FilePath == "" {
		return nil
	}
	dir, err := filepath.Abs(s.dir)
	if err != nil {
		return err
	}
	path, err := filepath.Abs(backup.FilePath)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(path, dir+string(os.PathSeparator)) {
		return errors.New("tệp sao lưu nằm ngoài thư mục sao lưu")
	}
	// Tệp đã bị xoá tay từ trước không phải lỗi — dòng nhật ký vẫn phải đi.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// --- sao lưu tự động -------------------------------------------------------

// BackupGroup là nhóm cài đặt của lịch sao lưu tự động.
const BackupGroup = "backup"

// Mặc định khi quán chưa lưu cài đặt nào: mỗi giờ một bản, giữ 48 bản tự động
// (2 ngày). Mỗi bản cỡ vài chục MB; sao lưu mỗi giờ mà không dọn thì vài tháng
// là đầy ổ đĩa.
const (
	defaultAutoBackupEvery = 1
	defaultAutoBackupUnit  = "hour"
	defaultAutoBackupKeep  = 48
	// Ngắn hơn thế thì pg_dump trên database lớn chưa kịp xong đã tới lượt sau.
	minAutoBackupInterval = 5 * time.Minute
	// Bản "running" cũ hơn mức này coi như đã chết (máy chủ tắt giữa chừng): nó
	// không được chặn lịch mãi mãi.
	staleRunningBackup = 2 * time.Hour
)

var autoBackupUnits = map[string]time.Duration{
	"minute": time.Minute,
	"hour":   time.Hour,
	"day":    24 * time.Hour,
}

type autoBackupConfig struct {
	Enabled  bool
	Interval time.Duration
	// Keep: số bản tự động giữ lại; 0 là không tự xoá.
	Keep int64
}

// readAutoBackupConfig đọc nhóm cài đặt "backup". Khoá thiếu hoặc sai thì dùng
// mặc định — quán chưa từng mở trang Sao lưu vẫn được sao lưu mỗi giờ.
func readAutoBackupConfig(m map[string]string) autoBackupConfig {
	every := settingInt(m, "auto_every", defaultAutoBackupEvery)
	if every < 1 {
		every = defaultAutoBackupEvery
	}
	unit, ok := autoBackupUnits[m["auto_unit"]]
	if !ok {
		unit = autoBackupUnits[defaultAutoBackupUnit]
	}
	interval := time.Duration(every) * unit
	if interval < minAutoBackupInterval {
		interval = minAutoBackupInterval
	}
	keep := settingInt(m, "auto_keep", defaultAutoBackupKeep)
	if keep < 0 {
		keep = defaultAutoBackupKeep
	}
	return autoBackupConfig{Enabled: m["auto_enabled"] != "false", Interval: interval, Keep: keep}
}

// kiemTraCaiDatSaoLuu chặn giá trị lịch sao lưu vô nghĩa ngay lúc lưu, thay vì
// để readAutoBackupConfig lặng lẽ thay bằng mặc định.
func kiemTraCaiDatSaoLuu(in map[string]interface{}) error {
	if v, ok := in["auto_enabled"]; ok {
		if _, isBool := v.(bool); !isBool {
			return errors.New("auto_enabled phải là true hoặc false")
		}
	}
	every := float64(defaultAutoBackupEvery)
	if v, ok := in["auto_every"]; ok {
		n, isNum := asNumber(v)
		if !isNum || n < 1 || n != float64(int64(n)) {
			return errors.New("chu kỳ sao lưu phải là số nguyên từ 1 trở lên")
		}
		every = n
	}
	unit := autoBackupUnits[defaultAutoBackupUnit]
	if v, ok := in["auto_unit"]; ok {
		s, _ := v.(string)
		d, known := autoBackupUnits[s]
		if !known {
			return errors.New("đơn vị chu kỳ sao lưu chỉ nhận minute, hour hoặc day")
		}
		unit = d
	}
	if time.Duration(every)*unit < minAutoBackupInterval {
		return fmt.Errorf("chu kỳ sao lưu tối thiểu là %d phút", int(minAutoBackupInterval/time.Minute))
	}
	if v, ok := in["auto_keep"]; ok {
		n, isNum := asNumber(v)
		if !isNum || n < 0 || n != float64(int64(n)) {
			return errors.New("số bản giữ lại phải là số nguyên không âm (0 = không tự xoá)")
		}
	}
	return nil
}

// RunAuto được bộ lập lịch gọi mỗi phút: tới hạn thì tạo một bản sao lưu tự
// động rồi dọn các bản tự động vượt số bản giữ lại. pg_dump chạy ngay trong
// lượt này chứ không tách goroutine — job không chạy chồng lượt, nên hai bản
// tự động không bao giờ cùng lúc.
func (s *BackupService) RunAuto(now time.Time) error {
	cfg := readAutoBackupConfig(settingsGroup(s.db, BackupGroup))
	if !cfg.Enabled {
		return nil
	}
	due, err := s.autoBackupDue(now, cfg.Interval)
	if err != nil || !due {
		return err
	}
	backup := model.BackupLog{
		FileName:  "backup_" + now.Format("20060102_150405") + ".sql",
		Status:    "running",
		Notes:     "Tự động",
		Auto:      true,
		StartedAt: now,
	}
	if err := s.db.Create(&backup).Error; err != nil {
		return err
	}
	s.runPgDump(&backup)
	return s.pruneAuto(cfg.Keep)
}

// autoBackupDue: đã đủ một chu kỳ kể từ lần sao lưu gần nhất chưa. Tính cả bản
// tạo tay (vừa sao lưu tay thì chưa cần thêm) và cả bản hỏng (pg_dump hỏng thì
// thử lại sau một chu kỳ, không phải mỗi phút một dòng "Thất bại").
func (s *BackupService) autoBackupDue(now time.Time, interval time.Duration) (bool, error) {
	var running int64
	if err := s.db.Model(&model.BackupLog{}).
		Where("status = ? AND started_at > ?", "running", now.Add(-staleRunningBackup)).
		Count(&running).Error; err != nil {
		return false, err
	}
	if running > 0 {
		return false, nil
	}
	var last model.BackupLog
	err := s.db.Order("started_at DESC").First(&last).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return now.Sub(last.StartedAt) >= interval, nil
}

// pruneAuto xoá các bản tự động cũ hơn `keep` bản mới nhất, cả tệp lẫn dòng.
// Bản tạo tay không bao giờ bị đụng tới. keep = 0 là không tự xoá.
func (s *BackupService) pruneAuto(keep int64) error {
	if keep <= 0 {
		return nil
	}
	var old []model.BackupLog
	if err := s.db.Where("auto = ? AND status <> ?", true, "running").
		Order("started_at DESC").Offset(int(keep)).Find(&old).Error; err != nil {
		return err
	}
	for i := range old {
		if err := s.removeFile(&old[i]); err != nil {
			return err
		}
		if err := s.db.Delete(&model.BackupLog{}, "id = ?", old[i].ID).Error; err != nil {
			return err
		}
	}
	if len(old) > 0 {
		s.hub.Changed("backups", map[string]interface{}{"pruned": len(old)})
	}
	return nil
}

func (s *BackupService) runPgDump(backup *model.BackupLog) {
	// Chạy sau mọi lần Save bên dưới, kể cả nhánh hỏng sớm.
	defer s.hub.Changed("backups", map[string]interface{}{"id": backup.ID})

	now := time.Now()

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		backup.Status = "failed"
		backup.Notes = "không tạo được thư mục sao lưu: " + err.Error()
		backup.CompletedAt = &now
		s.db.Save(backup)
		return
	}

	backup.FilePath = filepath.Join(s.dir, backup.FileName)

	// Every connection parameter has to be passed explicitly; without them
	// pg_dump falls back to the OS user and a local socket, which is why every
	// backup used to fail.
	cmd := exec.Command("pg_dump",
		"-Fc",
		"-h", s.cfg.Host,
		"-p", strconv.Itoa(s.cfg.Port),
		"-U", s.cfg.User,
		"-d", s.cfg.Name,
		"-f", backup.FilePath,
	)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+s.cfg.Password)

	output, err := cmd.CombinedOutput()
	now = time.Now()
	backup.CompletedAt = &now

	if err != nil {
		backup.Status = "failed"
		// Lỗi KHÔNG đến từ chính pg_dump — thiếu lệnh, không có quyền, hết chỗ
		// trên đĩa — thì output rỗng. Bản cũ ghi nguyên chuỗi rỗng đó, nên
		// người trực chỉ thấy "Thất bại" mà không nơi nào nói vì sao.
		backup.Notes = strings.TrimSpace(string(output))
		if backup.Notes == "" {
			backup.Notes = err.Error()
		}
	} else {
		backup.Status = "completed"
		// The real size on disk, not the exit code the old code stored here.
		if info, statErr := os.Stat(backup.FilePath); statErr == nil {
			backup.FileSize = info.Size()
		}
	}

	s.db.Save(backup)
}
