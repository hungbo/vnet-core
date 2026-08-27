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
	ID          string `json:"id"`
	Name        string `json:"name"`
	PrinterType string `json:"printer_type"`
	IPAddress   string `json:"ip_address"`
	Port        int    `json:"port"`
	IsDefault   bool   `json:"is_default"`
	// Cấu hình in: xem escpos.go.
	CharsPerLine int    `json:"chars_per_line"`
	Encoding     string `json:"encoding"`
	CodePage     int    `json:"code_page"`
	CreatedAt    string `json:"created_at"`
}

type CreatePrinterRequest struct {
	Name         string `json:"name" binding:"required"`
	PrinterType  string `json:"printer_type" binding:"required"`
	IPAddress    string `json:"ip_address"`
	Port         int    `json:"port"`
	IsDefault    bool   `json:"is_default"`
	CharsPerLine int    `json:"chars_per_line"`
	Encoding     string `json:"encoding"`
	CodePage     int    `json:"code_page"`
}

type UpdatePrinterRequest struct {
	Name         *string `json:"name"`
	PrinterType  *string `json:"printer_type"`
	IPAddress    *string `json:"ip_address"`
	Port         *int    `json:"port"`
	IsDefault    *bool   `json:"is_default"`
	CharsPerLine *int    `json:"chars_per_line"`
	Encoding     *string `json:"encoding"`
	CodePage     *int    `json:"code_page"`
}

// SetProductsRequest thay toàn bộ danh sách món của một máy in.
type SetProductsRequest struct {
	ProductIDs []string `json:"product_ids"`
}

func (s *PrinterService) List() ([]PrinterResponse, error) {
	var printers []model.PrinterConfig
	if err := s.db.Order("name asc").Find(&printers).Error; err != nil {
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
	if err := s.db.Where("id = ?", id).First(&printer).Error; err != nil {
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
		Name:         req.Name,
		PrinterType:  req.PrinterType,
		IPAddress:    req.IPAddress,
		Port:         req.Port,
		IsDefault:    req.IsDefault,
		CharsPerLine: req.CharsPerLine,
		Encoding:     req.Encoding,
		CodePage:     req.CodePage,
	}

	if printer.Port == 0 {
		printer.Port = 9100
	}
	// 32 ký tự = giấy 58mm. Chọn khổ hẹp làm mặc định vì đặt rộng quá thì chữ
	// tràn dòng lung tung trên giấy hẹp, còn đặt hẹp trên giấy rộng chỉ hơi phí.
	if printer.CharsPerLine <= 0 {
		printer.CharsPerLine = 32
	}
	if printer.Encoding == "" {
		printer.Encoding = EncodingASCII
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
	if err := s.db.Where("id = ?", id).First(&printer).Error; err != nil {
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
	if req.CharsPerLine != nil {
		updates["chars_per_line"] = *req.CharsPerLine
	}
	if req.Encoding != nil {
		updates["encoding"] = *req.Encoding
	}
	if req.CodePage != nil {
		updates["code_page"] = *req.CodePage
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

// Delete xoá một cấu hình máy in.
//
// Danh mục trỏ tới máy in để biết in phiếu bếp ở đâu; xoá máy in mà bỏ lại
// danh mục nghĩa là đơn của danh mục đó in vào hư không. Ánh xạ sản phẩm–máy in
// thì ngược lại, là con SỞ HỮU nên dọn luôn.
func (s *PrinterService) Delete(id string) error {
	var printer model.PrinterConfig
	if err := s.db.Where("id = ?", id).First(&printer).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("printer not found")
		}
		return err
	}

	if printer.IsDefault {
		return chanVi("không xoá được máy in mặc định — hãy đặt máy in khác làm mặc định trước")
	}

	if err := kiemTraPhuThuoc(s.db, id, []phuThuoc{
		{Bang: &model.Category{}, Cot: "printer_id", Nhan: "danh mục đang in ở máy này"},
	}, "hãy gỡ máy in khỏi các danh mục đó trước"); err != nil {
		return err
	}

	now := time.Now()
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("printer_id = ?", id).Delete(&model.ProductPrinterMapping{}).Error; err != nil {
			return err
		}
		return tx.Model(&printer).Update("deleted_at", &now).Error
	}); err != nil {
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
	if err := s.db.Where("id = ?", id).First(&printer).Error; err != nil {
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
		CharsPerLine: p.CharsPerLine,
		Encoding:     p.Encoding,
		CodePage:     p.CodePage,
		CreatedAt:    p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// --- phân luồng món ra máy in ----------------------------------------------
//
// Bảng product_printer_mappings đã có từ lâu nhưng chưa có gì đọc hay ghi nó,
// nên phiếu chế biến không thể phân luồng được. Một món có thể ra nhiều máy in
// (ví dụ vừa ra bếp vừa ra quầy điều phối), nên đây là quan hệ nhiều-nhiều.

// ListProducts trả về danh sách mã sản phẩm được gán cho một máy in.
func (s *PrinterService) ListProducts(printerID string) ([]string, error) {
	var printer model.PrinterConfig
	if err := s.db.Where("id = ?", printerID).First(&printer).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy máy in")
		}
		return nil, err
	}
	var rows []model.ProductPrinterMapping
	if err := s.db.Where("printer_id = ?", printerID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ProductID)
	}
	return out, nil
}

// SetProducts thay toàn bộ danh sách món của một máy in.
func (s *PrinterService) SetProducts(printerID string, productIDs []string, actorID string) ([]string, error) {
	var printer model.PrinterConfig
	if err := s.db.Where("id = ?", printerID).First(&printer).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy máy in")
		}
		return nil, err
	}

	// Bỏ trùng và bỏ rỗng: gửi cùng một mã hai lần sẽ tạo hai phiếu cho cùng
	// một món.
	seen := map[string]bool{}
	clean := make([]string, 0, len(productIDs))
	for _, id := range productIDs {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		clean = append(clean, id)
	}

	if len(clean) > 0 {
		var count int64
		if err := s.db.Model(&model.Product{}).Where("id IN ?", clean).Count(&count).Error; err != nil {
			return nil, err
		}
		if int(count) != len(clean) {
			return nil, errors.New("có mã sản phẩm không tồn tại")
		}
	}

	tx := s.db.Begin()
	if err := tx.Where("printer_id = ?", printerID).Delete(&model.ProductPrinterMapping{}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	for _, id := range clean {
		if err := tx.Create(&model.ProductPrinterMapping{PrinterID: printerID, ProductID: id}).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	var actor *string
	if actorID != "" {
		actor = &actorID
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "set_printer_products",
		EntityType: "printer",
		EntityID:   printerID,
		UserID:     actor,
		Metadata: map[string]interface{}{
			"printer": printer.Name,
			"count":   len(clean),
		},
	})
	return clean, nil
}
