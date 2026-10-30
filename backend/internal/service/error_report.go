package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrorReportService gom lỗi của cả hệ thống vào bảng error_events để dev biết
// mà sửa: lỗi 500 và panic của server, lỗi JS của trang quản trị, crash và lỗi
// của máy trạm.
//
// Trước đây lỗi chỉ còn trong server.log của từng quán, còn nhật ký máy trạm
// nằm trên ổ C: và mất sạch mỗi lần boot lại (máy không ổ cứng). Dev chỉ biết
// có lỗi khi khách gọi điện.
type ErrorReportService struct {
	db *gorm.DB
}

func NewErrorReportService(db *gorm.DB) *ErrorReportService {
	return &ErrorReportService{db: db}
}

const (
	ErrorSourceServer = "server"
	ErrorSourceAdmin  = "admin"
	ErrorSourceClient = "client"
)

// ErrorReportInput là một lần gặp lỗi. Source/Reporter/RequestID do server
// điền, không nhận từ thân yêu cầu.
type ErrorReportInput struct {
	Kind     string `json:"kind" binding:"omitempty,oneof=error panic crash"`
	Location string `json:"location"`
	Message  string `json:"message" binding:"required"`
	Detail   string `json:"detail"`
	Version  string `json:"version"`

	Source    string `json:"-"`
	Reporter  string `json:"-"`
	RequestID string `json:"-"`
}

type MachineErrorReportRequest struct {
	Events []ErrorReportInput `json:"events" binding:"required,min=1,max=20,dive"`
}

type ErrorListParams struct {
	Source string `form:"source"`
	// open (mặc định) | resolved | all
	Status string `form:"status"`
}

var (
	uuidTrongLoi     = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	soTrongLoi       = regexp.MustCompile(`\d+`)
	ngoacKepTrongLoi = regexp.MustCompile(`"[^"]*"`)
)

// errorFingerprint quyết định hai lần lỗi có phải CÙNG MỘT lỗi không. Bỏ uuid,
// con số và chuỗi trong ngoặc KÉP trước khi băm: "không tìm thấy hội viên 3f9a…"
// của hai hội viên là một lỗi, và PostgreSQL / Go (%q) đặt giá trị người dùng gõ
// vào ngoặc kép — `invalid input syntax for type uuid: "abc"`. Ngoặc ĐƠN giữ
// nguyên: lỗi JS dùng nó cho tên thuộc tính (reading 'balance'), khác tên là
// khác lỗi.
func errorFingerprint(source, kind, location, message string) string {
	chuan := func(s string) string {
		s = ngoacKepTrongLoi.ReplaceAllString(s, `"…"`)
		return soTrongLoi.ReplaceAllString(uuidTrongLoi.ReplaceAllString(s, "<id>"), "<n>")
	}
	sum := sha256.Sum256([]byte(source + "\x00" + kind + "\x00" + chuan(location) + "\x00" + chuan(message)))
	return hex.EncodeToString(sum[:])
}

// Record ghi một lần gặp lỗi: dòng mới nếu là lỗi lạ, còn không thì tăng Count,
// thay mẫu gần nhất và MỞ LẠI nếu dev đã đánh dấu đã sửa — sửa chưa đúng thì
// phải thấy lại.
func (s *ErrorReportService) Record(in ErrorReportInput) error {
	kind := in.Kind
	if kind == "" {
		kind = "error"
	}
	msg := truncateRunes(strings.TrimSpace(in.Message), 2000)
	if msg == "" {
		return errors.New("thiếu nội dung lỗi")
	}
	loc := truncateRunes(strings.TrimSpace(in.Location), 300)
	now := time.Now()

	ev := model.ErrorEvent{
		Fingerprint: errorFingerprint(in.Source, kind, loc, msg),
		Source:      in.Source,
		Kind:        kind,
		Location:    loc,
		Message:     msg,
		Detail:      truncateRunes(in.Detail, 16000),
		Reporter:    truncateRunes(in.Reporter, 100),
		RequestID:   truncateRunes(in.RequestID, 32),
		Version:     truncateRunes(in.Version, 50),
		Count:       1,
		FirstSeenAt: now,
		LastSeenAt:  now,
	}
	updates := append(
		clause.AssignmentColumns([]string{"message", "detail", "reporter", "request_id", "version", "last_seen_at", "resolved_at"}),
		clause.Assignment{Column: clause.Column{Name: "count"}, Value: gorm.Expr(`"error_events"."count" + 1`)},
	)
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "fingerprint"}},
		DoUpdates: updates,
	}).Create(&ev).Error
}

// RecordMachineErrors nhận lỗi do máy trạm gửi. Route by-code để mở (quy ước
// 31), nên ít nhất mã máy phải có thật — không thì ai trong LAN cũng đổ rác
// vào bảng được bằng một mã bịa.
func (s *ErrorReportService) RecordMachineErrors(code string, events []ErrorReportInput) error {
	var n int64
	if err := s.db.Model(&model.Machine{}).Where("machine_code = ?", code).Count(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		return errors.New("không tìm thấy máy")
	}
	for _, e := range events {
		e.Source = ErrorSourceClient
		e.Reporter = code
		if err := s.Record(e); err != nil {
			return err
		}
	}
	return nil
}

// List trả các loại lỗi, gặp gần nhất lên đầu. Mặc định chỉ lỗi chưa sửa.
func (s *ErrorReportService) List(params pagination.Params, f ErrorListParams) ([]model.ErrorEvent, int64, int, int, error) {
	q := s.db.Model(&model.ErrorEvent{})
	if f.Source != "" {
		q = q.Where("source = ?", f.Source)
	}
	switch f.Status {
	case "resolved":
		q = q.Where("resolved_at IS NOT NULL")
	case "all":
	default:
		q = q.Where("resolved_at IS NULL")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, 0, 0, err
	}
	// Bấm sắp xếp một cột trên trang Nhật ký lỗi thì theo cột đó; mặc định vẫn
	// gặp gần nhất lên đầu.
	order := pagination.OrderBy(&params, map[string]string{
		"source":       "source",
		"kind":         "kind",
		"message":      "message",
		"location":     "location",
		"count":        "count",
		"reporter":     "reporter",
		"last_seen_at": "last_seen_at",
	}, "last_seen_at desc")
	var items []model.ErrorEvent
	offset := (params.Page - 1) * params.PageSize
	if err := q.Offset(offset).Limit(params.PageSize).Order(order).Find(&items).Error; err != nil {
		return nil, 0, 0, 0, err
	}
	return items, total, params.Page, params.PageSize, nil
}

// Resolve đánh dấu đã sửa. Gặp lại thì Record tự mở lại.
func (s *ErrorReportService) Resolve(id string) error {
	if !laUUID(id) {
		return gorm.ErrRecordNotFound
	}
	res := s.db.Model(&model.ErrorEvent{}).Where("id = ?", id).Update("resolved_at", time.Now())
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
