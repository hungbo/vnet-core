package service

import (
	"errors"
	"os/exec"
	"time"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
)

type BackupService struct {
	db    *gorm.DB
	audit *AuditService
}

func NewBackupService(db *gorm.DB, audit *AuditService) *BackupService {
	return &BackupService{db: db, audit: audit}
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

	cmd := exec.Command("pg_restore", "--no-owner", "--dbname="+backup.FilePath)
	if err := cmd.Run(); err != nil {
		return err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "restore",
		EntityType: "backup_log",
		EntityID:   id,
		Metadata:   map[string]interface{}{"backup_id": id},
	})

	return nil
}

func (s *BackupService) runPgDump(backup *model.BackupLog) {
	backup.FilePath = "/tmp/" + backup.FileName
	cmd := exec.Command("pg_dump", "-Fc", "-f", backup.FilePath)

	output, err := cmd.CombinedOutput()
	now := time.Now()
	backup.CompletedAt = &now

	if err != nil {
		backup.Status = "failed"
		backup.Notes = string(output)
	} else {
		backup.Status = "completed"
		result := cmd.ProcessState
		if result != nil {
			backup.FileSize = int64(result.ExitCode())
		}
	}

	s.db.Save(backup)
}
