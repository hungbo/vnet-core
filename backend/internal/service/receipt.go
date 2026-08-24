package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"time"

	"github.com/vnet/core/internal/model"
	"gorm.io/gorm"
)

// In hoá đơn và phiếu chế biến.
//
// Hai loại giấy khác hẳn nhau và không được lẫn:
//
//   - Hoá đơn khách: đủ món, đơn giá, giảm giá, tổng tiền — in ở quầy.
//   - Phiếu chế biến: chỉ tên món và số lượng, KHÔNG có giá — in ở bếp/quầy pha
//     chế. Bảng product_printer_mappings quyết định món nào ra máy in nào.
//
// Bảng mappings đã tồn tại từ lâu nhưng chưa có gì đọc hay ghi nó.

type ReceiptService struct {
	db    *gorm.DB
	audit *AuditService
}

func NewReceiptService(db *gorm.DB, audit *AuditService) *ReceiptService {
	return &ReceiptService{db: db, audit: audit}
}

// PrintJobResult mô tả một lần in tới một máy.
type PrintJobResult struct {
	PrinterID   string   `json:"printer_id"`
	PrinterName string   `json:"printer_name"`
	Items       []string `json:"items,omitempty"`
	Bytes       int      `json:"bytes"`
	Error       string   `json:"error,omitempty"`
}

// PrintResult là kết quả tổng hợp. Unrouted liệt kê món không có máy in nào
// nhận — im lặng bỏ qua chúng sẽ khiến bếp không bao giờ biết có món đó.
type PrintResult struct {
	Jobs     []PrintJobResult `json:"jobs"`
	Unrouted []string         `json:"unrouted,omitempty"`
}

// Failed cho biết có job nào hỏng không, để handler chọn mã trạng thái.
func (r *PrintResult) Failed() bool {
	for _, j := range r.Jobs {
		if j.Error != "" {
			return true
		}
	}
	return false
}

type shopInfo struct {
	Name    string
	Address string
	Phone   string

	// Ba trường dưới đây thuộc nhóm cài đặt "invoice". Trước đây hoá đơn dùng
	// chuỗi cứng "HOA DON THANH TOAN" / "Cam on quy khach!" và không in mã số
	// thuế, nên tab Hoá đơn trong trang Cài đặt lưu được mà không đổi gì trên
	// tờ giấy khách cầm.
	Title   string
	Footer  string
	TaxCode string
}

const (
	defaultReceiptTitle  = "HOA DON THANH TOAN"
	defaultReceiptFooter = "Cam on quy khach!"
)

func (s *ReceiptService) shopInfo() shopInfo {
	info := shopInfo{Name: "VNET", Title: defaultReceiptTitle, Footer: defaultReceiptFooter}

	general := settingsGroup(s.db, "general")
	if v := general["store_name"]; v != "" {
		info.Name = v
	}
	info.Address = general["store_address"]
	info.Phone = general["store_phone"]

	invoice := settingsGroup(s.db, "invoice")
	if v := invoice["invoice_title"]; v != "" {
		info.Title = v
	}
	if v := invoice["invoice_footer"]; v != "" {
		info.Footer = v
	}
	info.TaxCode = invoice["tax_code"]
	return info
}

// --- hoá đơn khách ---------------------------------------------------------

// PrintOrder in hoá đơn cho khách. printerID rỗng thì dùng máy in mặc định.
func (s *ReceiptService) PrintOrder(orderID, printerID, actorID string) (*PrintResult, error) {
	order, items, err := s.loadOrder(orderID)
	if err != nil {
		return nil, err
	}

	printer, err := s.pickReceiptPrinter(printerID)
	if err != nil {
		return nil, err
	}

	doc := s.renderReceipt(order, items, printer)
	res := &PrintResult{Jobs: []PrintJobResult{s.send(printer, doc)}}
	s.logPrint("print_receipt", orderID, actorID, res)
	return res, nil
}

// PreviewOrder trả về đúng nội dung sẽ ra giấy, dạng chữ. Cho phép kiểm hoá đơn
// mà không cần máy in — và để nhân viên xem trước khi tốn giấy.
func (s *ReceiptService) PreviewOrder(orderID, printerID string) (string, error) {
	order, items, err := s.loadOrder(orderID)
	if err != nil {
		return "", err
	}
	printer, err := s.pickReceiptPrinter(printerID)
	if err != nil {
		// Chưa khai máy in nào thì vẫn xem trước được, dùng khổ giấy 58mm.
		printer = &model.PrinterConfig{Name: "(chưa khai máy in)", CharsPerLine: 32}
	}
	return s.renderReceipt(order, items, printer).PlainText(), nil
}

func (s *ReceiptService) renderReceipt(order *model.Order, items []model.OrderItem, p *model.PrinterConfig) *escposDoc {
	shop := s.shopInfo()
	d := newESCPOSDoc(p.CharsPerLine, p.Encoding)
	d.init(p.CodePage)

	d.align(1)
	d.bold(true)
	d.double(true)
	d.line(shop.Name)
	d.double(false)
	d.bold(false)
	if shop.Address != "" {
		d.wrap(shop.Address, 0)
	}
	if shop.Phone != "" {
		d.line("DT: " + shop.Phone)
	}
	if shop.TaxCode != "" {
		d.line("MST: " + shop.TaxCode)
	}
	d.feed(1)
	d.bold(true)
	d.line(shop.Title)
	d.bold(false)
	d.align(0)
	d.separator()

	d.twoCol("So HD:", order.OrderCode)
	d.twoCol("Ngay:", order.CreatedAt.Format("02/01/2006 15:04"))
	if order.TableNumber != "" {
		d.twoCol("Ban:", order.TableNumber)
	}
	d.separator()

	for _, it := range items {
		d.line(it.ProductName)
		for _, opt := range decodeOrderOptions(it.Options) {
			d.wrap("+ "+opt, 2)
		}
		if it.Note != "" {
			d.wrap("* "+it.Note, 2)
		}
		d.twoCol(
			fmt.Sprintf("  %d x %s", it.Quantity, formatMoney(it.UnitPrice)),
			formatMoney(it.Subtotal),
		)
	}

	d.separator()
	d.twoCol("Tam tinh", formatMoney(order.TotalAmount))
	if order.DiscountAmount > 0 {
		label := "Giam gia"
		if name := s.promotionName(order.PromotionID); name != "" {
			label = "Giam gia (" + name + ")"
		}
		d.twoCol(label, "-"+formatMoney(order.DiscountAmount))
	}
	d.bold(true)
	d.double(true)
	d.twoCol("TONG", formatMoney(order.FinalAmount))
	d.double(false)
	d.bold(false)
	if order.PaymentMethod != "" {
		d.twoCol("Thanh toan", paymentLabel(order.PaymentMethod))
	}

	d.feed(1)
	d.align(1)
	// Chân hoá đơn có thể dài (wifi, giờ mở cửa, lời cảm ơn nhiều dòng) nên
	// dùng wrap chứ không line — chuỗi dài hơn khổ giấy sẽ bị cắt cụt.
	d.wrap(shop.Footer, 0)
	d.align(0)
	d.cut()
	return d
}

// --- phiếu chế biến --------------------------------------------------------

// PrintStations in phiếu cho từng bộ phận theo bảng phân luồng.
func (s *ReceiptService) PrintStations(orderID, actorID string) (*PrintResult, error) {
	order, items, err := s.loadOrder(orderID)
	if err != nil {
		return nil, err
	}

	productIDs := make([]string, 0, len(items))
	for _, it := range items {
		productIDs = append(productIDs, it.ProductID)
	}

	var mappings []model.ProductPrinterMapping
	if len(productIDs) > 0 {
		s.db.Where("product_id IN ?", productIDs).Find(&mappings)
	}
	byProduct := map[string][]string{}
	printerIDs := map[string]bool{}
	for _, m := range mappings {
		byProduct[m.ProductID] = append(byProduct[m.ProductID], m.PrinterID)
		printerIDs[m.PrinterID] = true
	}

	res := &PrintResult{}

	// Món không có máy in nào nhận: báo ra, không nuốt.
	grouped := map[string][]model.OrderItem{}
	for _, it := range items {
		targets := byProduct[it.ProductID]
		if len(targets) == 0 {
			res.Unrouted = append(res.Unrouted, it.ProductName)
			continue
		}
		for _, pid := range targets {
			grouped[pid] = append(grouped[pid], it)
		}
	}
	if len(grouped) == 0 {
		return res, nil
	}

	ids := make([]string, 0, len(grouped))
	for pid := range grouped {
		ids = append(ids, pid)
	}
	sort.Strings(ids) // thứ tự ổn định để kiểm chứng lặp lại được

	var printers []model.PrinterConfig
	s.db.Where("id IN ?", ids).Find(&printers)
	byID := map[string]model.PrinterConfig{}
	for _, p := range printers {
		byID[p.ID] = p
	}

	for _, pid := range ids {
		p, ok := byID[pid]
		if !ok {
			// Bảng phân luồng trỏ tới máy in đã xoá.
			res.Jobs = append(res.Jobs, PrintJobResult{
				PrinterID: pid,
				Error:     "máy in không còn tồn tại",
			})
			continue
		}
		doc := s.renderStationTicket(order, grouped[pid], &p)
		job := s.send(&p, doc)
		for _, it := range grouped[pid] {
			job.Items = append(job.Items, it.ProductName)
		}
		res.Jobs = append(res.Jobs, job)
	}

	s.logPrint("print_stations", orderID, actorID, res)
	return res, nil
}

// renderStationTicket cố ý KHÔNG in giá. Bếp cần biết làm gì, không cần biết
// khách trả bao nhiêu — và phiếu lọt ra ngoài thì cũng không lộ doanh thu.
func (s *ReceiptService) renderStationTicket(order *model.Order, items []model.OrderItem, p *model.PrinterConfig) *escposDoc {
	d := newESCPOSDoc(p.CharsPerLine, p.Encoding)
	d.init(p.CodePage)

	d.align(1)
	d.bold(true)
	d.double(true)
	d.line(p.Name)
	d.double(false)
	d.bold(false)
	d.align(0)
	d.separator()
	d.twoCol("So HD:", order.OrderCode)
	d.twoCol("Gio:", time.Now().Format("15:04"))
	if order.TableNumber != "" {
		d.bold(true)
		d.twoCol("BAN:", order.TableNumber)
		d.bold(false)
	}
	d.separator()

	for _, it := range items {
		d.bold(true)
		d.double(true)
		d.line(fmt.Sprintf("%d x %s", it.Quantity, it.ProductName))
		d.double(false)
		d.bold(false)
		for _, opt := range decodeOrderOptions(it.Options) {
			d.wrap("+ "+opt, 2)
		}
		if it.Note != "" {
			d.wrap("* "+it.Note, 2)
		}
	}

	d.separator()
	d.cut()
	return d
}

// --- gửi xuống máy in ------------------------------------------------------

func (s *ReceiptService) send(p *model.PrinterConfig, doc *escposDoc) PrintJobResult {
	job := PrintJobResult{PrinterID: p.ID, PrinterName: p.Name, Bytes: len(doc.Bytes())}
	if p.IPAddress == "" {
		job.Error = "máy in chưa khai địa chỉ IP"
		return job
	}
	addr := net.JoinHostPort(p.IPAddress, fmt.Sprintf("%d", p.Port))
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		job.Error = fmt.Sprintf("không kết nối được tới %s: %v", addr, err)
		return job
	}
	defer conn.Close()

	// Máy in nhiệt nhận chậm; không đặt hạn ghi thì một máy kẹt giấy sẽ treo
	// luôn request.
	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if _, err := conn.Write(doc.Bytes()); err != nil {
		job.Error = fmt.Sprintf("gửi dữ liệu thất bại: %v", err)
	}
	return job
}

// --- tiện ích --------------------------------------------------------------

func (s *ReceiptService) loadOrder(orderID string) (*model.Order, []model.OrderItem, error) {
	var order model.Order
	if err := s.db.First(&order, "id = ?", orderID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, errors.New("không tìm thấy đơn hàng")
		}
		return nil, nil, err
	}
	var items []model.OrderItem
	if err := s.db.Where("order_id = ?", orderID).Order("created_at asc").Find(&items).Error; err != nil {
		return nil, nil, err
	}
	if len(items) == 0 {
		return nil, nil, errors.New("đơn hàng không có món nào để in")
	}
	return &order, items, nil
}

func (s *ReceiptService) pickReceiptPrinter(printerID string) (*model.PrinterConfig, error) {
	var p model.PrinterConfig
	q := s.db.Order("is_default DESC, created_at ASC")
	if printerID != "" {
		q = q.Where("id = ?", printerID)
	}
	if err := q.First(&p).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if printerID != "" {
				return nil, errors.New("không tìm thấy máy in")
			}
			return nil, errors.New("chưa khai máy in nào")
		}
		return nil, err
	}
	return &p, nil
}

func (s *ReceiptService) promotionName(id *string) string {
	if id == nil || *id == "" {
		return ""
	}
	var p model.Promotion
	if err := s.db.Select("name").First(&p, "id = ?", *id).Error; err != nil {
		return ""
	}
	return p.Name
}

func (s *ReceiptService) logPrint(action, orderID, actorID string, res *PrintResult) {
	var actor *string
	if actorID != "" {
		actor = &actorID
	}
	jobs := make([]map[string]interface{}, 0, len(res.Jobs))
	for _, j := range res.Jobs {
		jobs = append(jobs, map[string]interface{}{
			"printer": j.PrinterName,
			"bytes":   j.Bytes,
			"error":   j.Error,
		})
	}
	s.audit.Log(&LogAuditRequest{
		Action:     action,
		EntityType: "order",
		EntityID:   orderID,
		UserID:     actor,
		Metadata: map[string]interface{}{
			"jobs":     jobs,
			"unrouted": res.Unrouted,
			"failed":   res.Failed(),
		},
	})
}

// decodeOrderOptions bóc phần tuỳ chọn món ra thành dòng đọc được. Cột options
// là jsonb tự do và có thể là chuỗi "null", nên mọi lỗi đều nuốt im lặng — một
// hoá đơn thiếu dòng topping vẫn hơn là không in được hoá đơn.
func decodeOrderOptions(raw string) []string {
	if raw == "" || raw == "null" {
		return nil
	}
	var opts []OrderOption
	if err := json.Unmarshal([]byte(raw), &opts); err != nil {
		return nil
	}
	out := make([]string, 0, len(opts))
	for _, o := range opts {
		if o.Name == "" {
			continue
		}
		line := o.Name
		if o.Quantity > 1 {
			line = fmt.Sprintf("%s x%g", o.Name, o.Quantity)
		}
		out = append(out, line)
	}
	return out
}

func paymentLabel(method string) string {
	switch method {
	case "cash":
		return "Tien mat"
	case "balance":
		return "Tru so du"
	case "transfer":
		return "Chuyen khoan"
	case "card":
		return "The"
	}
	return method
}
