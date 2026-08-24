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

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
)

type BackupService struct {
	db    *gorm.DB
	cfg   *config.DatabaseConfig
	dir   string
	audit *AuditService
}

// NewBackupService needs the database settings because pg_dump is a separate
// process: it cannot inherit the connection the application already holds.
func NewBackupService(db *gorm.DB, cfg *config.DatabaseConfig, dir string, audit *AuditService) *BackupService {
	if dir == "" {
		dir = "backups"
	}
	return &BackupService{db: db, cfg: cfg, dir: dir, audit: audit}
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
			return errors.New("backup not found")
		}
		return err
	}

	if backup.FilePath == "" {
		return errors.New("backup file not found")
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
			return errors.New("backup not found")
		}
		return err
	}

	// Bản sao lưu đang chạy chưa ghi xong tệp: xoá lúc này để lại một tệp cụt
	// mà không dòng nào trỏ tới.
	if backup.Status == "running" {
		return errors.New("cannot delete a backup that is still running")
	}

	// Chỉ cho phép xoá tệp nằm trong thư mục sao lưu. FilePath đến từ database;
	// nếu ai đó sửa được cột này thì một đường dẫn kiểu ../../etc sẽ khiến máy
	// chủ tự xoá tệp hệ thống của chính nó.
	if backup.FilePath != "" {
		dir, err := filepath.Abs(s.dir)
		if err != nil {
			return err
		}
		path, err := filepath.Abs(backup.FilePath)
		if err != nil {
			return err
		}
		if !strings.HasPrefix(path, dir+string(os.PathSeparator)) {
			return errors.New("backup file is outside the backup directory")
		}
		// Tệp đã bị xoá tay từ trước không phải lỗi — dòng nhật ký vẫn phải đi.
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
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

func (s *BackupService) runPgDump(backup *model.BackupLog) {
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
		backup.Notes = string(output)
	} else {
		backup.Status = "completed"
		// The real size on disk, not the exit code the old code stored here.
		if info, statErr := os.Stat(backup.FilePath); statErr == nil {
			backup.FileSize = info.Size()
		}
	}

	s.db.Save(backup)
}
