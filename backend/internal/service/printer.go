package service

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/vnet/core/internal/model"
	"gorm.io/gorm"
)

type PrinterService struct {
	db    *gorm.DB
	audit *AuditService
}

func NewPrinterService(db *gorm.DB, audit *AuditService) *PrinterService {
	return &PrinterService{db: db, audit: audit}
}

type PrinterResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	PrinterType string  `json:"printer_type"`
	IPAddress   string  `json:"ip_address"`
	Port        int     `json:"port"`
	IsDefault   bool    `json:"is_default"`
	CreatedAt   string  `json:"created_at"`
}

type CreatePrinterRequest struct {
	Name        string  `json:"name" binding:"required"`
	PrinterType string  `json:"printer_type" binding:"required"`
	IPAddress   string  `json:"ip_address"`
	Port        int     `json:"port"`
	IsDefault   bool    `json:"is_default"`
}

type UpdatePrinterRequest struct {
	Name        *string `json:"name"`
	PrinterType *string `json:"printer_type"`
	IPAddress   *string `json:"ip_address"`
	Port        *int    `json:"port"`
	IsDefault   *bool   `json:"is_default"`
}

func (s *PrinterService) List() ([]PrinterResponse, error) {
	var printers []model.PrinterConfig
	if err := s.db.Where("deleted_at IS NULL").Order("name asc").Find(&printers).Error; err != nil {
		return nil, err
	}

	responses := make([]PrinterResponse, len(printers))
	for i, p := range printers {
		responses[i] = printerToResponse(p)
	}
	return responses, nil
}

func (s *PrinterService) GetByID(id string) (*PrinterResponse, error) {
	var printer model.PrinterConfig
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&printer).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("printer not found")
		}
		return nil, err
	}

	result := printerToResponse(printer)
	return &result, nil
}

func (s *PrinterService) Create(req *CreatePrinterRequest) (*PrinterResponse, error) {
	printer := model.PrinterConfig{
		Name:        req.Name,
		PrinterType: req.PrinterType,
		IPAddress:   req.IPAddress,
		Port:        req.Port,
		IsDefault:   req.IsDefault,
	}

	if printer.Port == 0 {
		printer.Port = 9100
	}

	if printer.IsDefault {
		s.db.Model(&model.PrinterConfig{}).Where("is_default = ?", true).Update("is_default", false)
	}

	if err := s.db.Create(&printer).Error; err != nil {
		return nil, err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "printer_config",
		EntityID:   printer.ID,
		UserID:     nil,
		Metadata:   map[string]interface{}{"name": printer.Name},
		IPAddress:  "",
	})

	result := printerToResponse(printer)
	return &result, nil
}

func (s *PrinterService) Update(id string, req *UpdatePrinterRequest) (*PrinterResponse, error) {
	var printer model.PrinterConfig
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&printer).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("printer not found")
		}
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.PrinterType != nil {
		updates["printer_type"] = *req.PrinterType
	}
	if req.IPAddress != nil {
		updates["ip_address"] = *req.IPAddress
	}
	if req.Port != nil {
		updates["port"] = *req.Port
	}
	if req.IsDefault != nil {
		if *req.IsDefault {
			s.db.Model(&model.PrinterConfig{}).Where("is_default = ?", true).Update("is_default", false)
		}
		updates["is_default"] = *req.IsDefault
	}
	if len(updates) > 0 {
		if err := s.db.Model(&printer).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "update",
		EntityType: "printer_config",
		EntityID:   id,
		UserID:     nil,
		Metadata:   updates,
		IPAddress:  "",
	})

	s.db.First(&printer, "id = ?", id)
	result := printerToResponse(printer)
	return &result, nil
}

func (s *PrinterService) Delete(id string) error {
	var printer model.PrinterConfig
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&printer).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("printer not found")
		}
		return err
	}

	now := time.Now()
	if err := s.db.Model(&printer).Update("deleted_at", &now).Error; err != nil {
		return err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "printer_config",
		EntityID:   printer.ID,
		UserID:     nil,
		Metadata:   map[string]interface{}{"name": printer.Name},
		IPAddress:  "",
	})
	return nil
}

func (s *PrinterService) TestPrint(id string) error {
	var printer model.PrinterConfig
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&printer).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("printer not found")
		}
		return err
	}

	if printer.IPAddress == "" {
		return errors.New("printer has no IP address configured")
	}

	addr := fmt.Sprintf("%s:%d", printer.IPAddress, printer.Port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("cannot connect to printer at %s: %w", addr, err)
	}
	defer conn.Close()

	testData := append(
		[]byte("\x1b\x40\x1b\x61\x01"),
		[]byte("Test Print\n\nPrinter: "+printer.Name+"\nIP: "+printer.IPAddress+"\nPort: "+fmt.Sprintf("%d", printer.Port)+"\n\n\x1d\x56\x00")...,
	)

	if _, err := conn.Write(testData); err != nil {
		return fmt.Errorf("failed to send test data: %w", err)
	}

	return nil
}

func printerToResponse(p model.PrinterConfig) PrinterResponse {
	return PrinterResponse{
		ID:          p.ID,
		Name:        p.Name,
		PrinterType: p.PrinterType,
		IPAddress:   p.IPAddress,
		Port:        p.Port,
		IsDefault:   p.IsDefault,
		CreatedAt:   p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
