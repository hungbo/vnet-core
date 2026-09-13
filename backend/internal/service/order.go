package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OrderOption struct {
	OptionID string  `json:"option_id"`
	Name     string  `json:"name"`
	Price    int64   `json:"price"`
	Quantity float64 `json:"quantity"`
}

type OrderService struct {
	db    *gorm.DB
	hub   *hub.Hub
	audit *AuditService
	inv   *InventoryService
}

func NewOrderService(db *gorm.DB, hub *hub.Hub, audit *AuditService, inv *InventoryService) *OrderService {
	return &OrderService{db: db, hub: hub, audit: audit, inv: inv}
}

type CreateOrderRequest struct {
	MemberID    string             `json:"member_id"`
	MachineID   string             `json:"machine_id"`
	MachineCode string             `json:"machine_code"`
	TableNumber string             `json:"table_number"`
	Note        string             `json:"note"`
	Items       []OrderItemRequest `json:"items"`
}

type CreateTopupOrderRequest struct {
	MemberID    string `json:"member_id"`
	Amount      int64  `json:"amount"`
	MachineCode string `json:"machine_code"`
}

type OrderItemRequest struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
	Options   string `json:"options"`
	Note      string `json:"note"`
}

type UpdateStatusRequest struct {
	Status string `json:"status"`
}

type SplitOrderRequest struct {
	Items []SplitItem `json:"items"`
}

type SplitItem struct {
	OrderItemID string `json:"order_item_id"`
	NewQuantity int    `json:"new_quantity"`
}

type PayRequest struct {
	PaymentMethod string `json:"payment_method"`
	Amount        int64  `json:"amount"`
	ReferenceCode string `json:"reference_code"`
	// Chỉ dùng khi payment_method = "gift_card".
	CardSerial string `json:"card_serial"`
	CardSecret string `json:"card_secret"`
}

type OrderResponse struct {
	ID             string              `json:"id"`
	OrderCode      string              `json:"order_code"`
	Status         string              `json:"status"`
	OrderType      string              `json:"order_type,omitempty"`
	MemberID       *string             `json:"member_id"`
	MachineID      *string             `json:"machine_id"`
	TableNumber    string              `json:"table_number"`
	TotalAmount    int64               `json:"total_amount"`
	DiscountAmount int64               `json:"discount_amount"`
	FinalAmount    int64               `json:"final_amount"`
	PromotionID    *string             `json:"promotion_id"`
	PromotionName  string              `json:"promotion_name,omitempty"`
	PaymentMethod  string              `json:"payment_method,omitempty"`
	Note           string              `json:"note"`
	CreatedBy      *string             `json:"created_by"`
	UpdatedBy      *string             `json:"updated_by"`
	UpdatedByName  string              `json:"updated_by_name,omitempty"`
	MemberName     string              `json:"member_name,omitempty"`
	MemberUsername string              `json:"member_username,omitempty"`
	MachineCode    string              `json:"machine_code,omitempty"`
	CompletedAt    *time.Time          `json:"completed_at"`
	CreatedAt      time.Time           `json:"created_at"`
	Items          []OrderItemResponse `json:"items,omitempty"`
	Payments       []PaymentResponse   `json:"payments,omitempty"`
}

type OrderItemResponse struct {
	ID          string        `json:"id"`
	OrderID     string        `json:"order_id"`
	ProductID   string        `json:"product_id"`
	ProductName string        `json:"product_name"`
	Quantity    int           `json:"quantity"`
	UnitPrice   int64         `json:"unit_price"`
	Options     string        `json:"options"`
	OptionList  []OrderOption `json:"option_list,omitempty"`
	Subtotal    int64         `json:"subtotal"`
	Status      string        `json:"status"`
	Note        string        `json:"note"`
}

type PaymentResponse struct {
	ID            string     `json:"id"`
	OrderID       string     `json:"order_id"`
	PaymentMethod string     `json:"payment_method"`
	Amount        int64      `json:"amount"`
	ReferenceCode string     `json:"reference_code"`
	Status        string     `json:"status"`
	PaidAt        *time.Time `json:"paid_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

func (s *OrderService) List(params pagination.Params) ([]OrderResponse, int64, int, int, error) {
	query := s.db.Model(&model.Order{})

	if params.OrderType != "" {
		query = query.Where("order_type = ?", params.OrderType)
	}

	if params.Search != "" {
		search := "%" + params.Search + "%"
		// Ghi chú đơn là tiếng Việt có dấu; nhân viên quầy gõ không dấu. Bỏ dấu
		// cả hai vế đúng như trang Sản phẩm và Hội viên đang làm.
		query = query.Where("unaccent(note) ILIKE unaccent(?)", search)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	params.Sort = "created_at"
	params.Order = "desc"

	var orders []model.Order
	if err := pagination.Apply(query, &params).Find(&orders).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	result := make([]OrderResponse, len(orders))
	var memberIDs, machineIDs, userIDs, promotionIDs []string
	for i := range orders {
		items := s.loadOrderItems(orders[i].ID)
		result[i] = toOrderResponse(&orders[i], items)
		if orders[i].MemberID != nil {
			memberIDs = append(memberIDs, *orders[i].MemberID)
		}
		if orders[i].MachineID != nil {
			machineIDs = append(machineIDs, *orders[i].MachineID)
		}
		if orders[i].UpdatedBy != nil {
			userIDs = append(userIDs, *orders[i].UpdatedBy)
		}
		if orders[i].PromotionID != nil {
			promotionIDs = append(promotionIDs, *orders[i].PromotionID)
		}
	}

	memberNames := s.batchLoadMemberNames(memberIDs)
	memberUsernames := s.batchLoadMemberUsernames(memberIDs)
	machineCodes := s.batchLoadMachineCodes(machineIDs)
	userNames := s.batchLoadUserNames(userIDs)
	promotionNames := s.batchLoadPromotionNames(promotionIDs)

	for i := range result {
		if orders[i].MemberID != nil {
			result[i].MemberName = memberNames[*orders[i].MemberID]
			result[i].MemberUsername = memberUsernames[*orders[i].MemberID]
		}
		if orders[i].MachineID != nil {
			result[i].MachineCode = machineCodes[*orders[i].MachineID]
		}
		if orders[i].UpdatedBy != nil {
			result[i].UpdatedByName = userNames[*orders[i].UpdatedBy]
		}
		if orders[i].PromotionID != nil {
			result[i].PromotionName = promotionNames[*orders[i].PromotionID]
		}
	}

	return result, total, params.Page, params.PageSize, nil
}

func (s *OrderService) GetByID(id string) (*OrderResponse, error) {
	var order model.Order
	if err := s.db.Where("id = ?", id).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy đơn hàng")
		}
		return nil, err
	}

	items := s.loadOrderItems(order.ID)
	payments := s.loadOrderPayments(order.ID)

	result := toOrderResponse(&order, items)
	result.Payments = payments

	if order.MemberID != nil {
		names := s.batchLoadMemberNames([]string{*order.MemberID})
		usernames := s.batchLoadMemberUsernames([]string{*order.MemberID})
		result.MemberName = names[*order.MemberID]
		result.MemberUsername = usernames[*order.MemberID]
	}
	if order.MachineID != nil {
		codes := s.batchLoadMachineCodes([]string{*order.MachineID})
		result.MachineCode = codes[*order.MachineID]
	}
	if order.UpdatedBy != nil {
		names := s.batchLoadUserNames([]string{*order.UpdatedBy})
		result.UpdatedByName = names[*order.UpdatedBy]
	}
	if order.PromotionID != nil {
		names := s.batchLoadPromotionNames([]string{*order.PromotionID})
		result.PromotionName = names[*order.PromotionID]
	}

	return &result, nil
}

func (s *OrderService) CreateTopupOrder(req CreateTopupOrderRequest, createdBy string) (*OrderResponse, error) {
	if req.Amount <= 0 {
		return nil, errors.New("số tiền phải lớn hơn 0")
	}

	order := model.Order{
		// OrderCode được gán bên trong giao dịch, xem GenerateOrderCode.
		Status:      "pending",
		OrderType:   model.OrderTypeTopup,
		TotalAmount: req.Amount,
		FinalAmount: req.Amount,
		CreatedBy:   &createdBy,
	}
	if req.MemberID != "" {
		order.MemberID = &req.MemberID
	}
	if req.MachineCode != "" {
		var machine model.Machine
		if err := s.db.Select("id").Where("machine_code = ?", req.MachineCode).First(&machine).Error; err == nil {
			order.MachineID = &machine.ID
		}
	}

	topupTx := s.db.Begin()
	order.OrderCode = s.GenerateOrderCode(topupTx)
	if err := topupTx.Create(&order).Error; err != nil {
		topupTx.Rollback()
		return nil, err
	}
	if err := topupTx.Commit().Error; err != nil {
		return nil, err
	}

	items := s.loadOrderItems(order.ID)
	result := toOrderResponse(&order, items)

	var uid *string
	if createdBy != "" {
		uid = &createdBy
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "create_topup_order",
		EntityType: "order",
		EntityID:   order.ID,
		UserID:     uid,
		Metadata: map[string]interface{}{
			"order_code": order.OrderCode,
			"amount":     req.Amount,
		},
	})

	// Đơn hàng là việc của quầy, không phải tin tức cho máy khách.
	s.hub.BroadcastToType(hub.Event{
		Type: "order:new",
		Data: map[string]interface{}{
			"order_id":     order.ID,
			"order_code":   order.OrderCode,
			"order_type":   model.OrderTypeTopup,
			"final_amount": order.FinalAmount,
		},
	}, hub.ClientTypeAdmin)

	return &result, nil
}

func (s *OrderService) Create(req CreateOrderRequest, createdBy string) (*OrderResponse, error) {
	if len(req.Items) == 0 {
		return nil, errors.New("đơn hàng phải có ít nhất một sản phẩm")
	}

	var totalAmount int64
	var orderItems []model.OrderItem
	// Dữ kiện cho bộ máy khuyến mãi, gom luôn trong vòng lặp này.
	var totalQuantity int
	var categoryIDs []string

	for _, item := range req.Items {
		if item.ProductID == "" {
			return nil, errors.New("thiếu mã sản phẩm")
		}
		if item.Quantity <= 0 {
			return nil, errors.New("số lượng phải lớn hơn 0")
		}

		var product model.Product
		if err := s.db.Where("id = ?", item.ProductID).First(&product).Error; err != nil {
			return nil, fmt.Errorf("không tìm thấy sản phẩm %s", item.ProductID)
		}

		if !product.IsRetail {
			return nil, fmt.Errorf("sản phẩm %s không phải hàng bán lẻ", product.Name)
		}

		totalQuantity += int(item.Quantity)
		if product.CategoryID != nil {
			categoryIDs = append(categoryIDs, *product.CategoryID)
		}

		// Parse options & compute price adjustment
		type parsedOption struct {
			opt   model.ProductOption
			qty   float64
			price int64
		}
		var enrichedOpts []OrderOption
		var parsedOpts []parsedOption
		optionPriceTotal := int64(0)
		if item.Options != "" {
			var rawOpts []struct {
				OptionID string  `json:"option_id"`
				Quantity float64 `json:"quantity"`
			}
			if err := json.Unmarshal([]byte(item.Options), &rawOpts); err == nil {
				for _, ro := range rawOpts {
					var opt model.ProductOption
					if err := s.db.Where("id = ? AND product_id = ?", ro.OptionID, item.ProductID).First(&opt).Error; err != nil {
						continue
					}
					qty := ro.Quantity
					if qty <= 0 {
						qty = 1
					}
					var price int64
					if opt.IngredientID != nil {
						var ing model.Product
						if err := s.db.Select("price").Where("id = ?", *opt.IngredientID).First(&ing).Error; err == nil {
							price = ing.Price
						}
					}
					optionPriceTotal += price * int64(qty)
					parsedOpts = append(parsedOpts, parsedOption{opt: opt, qty: qty, price: price})
					enrichedOpts = append(enrichedOpts, OrderOption{
						OptionID: ro.OptionID,
						Name:     opt.Name,
						Price:    price,
						Quantity: qty,
					})
				}
			}
		}

		baseTotal := product.Price * int64(item.Quantity)
		subtotal := baseTotal + optionPriceTotal
		unitPrice := product.Price
		if item.Quantity > 0 {
			unitPrice = subtotal / int64(item.Quantity)
		}
		totalAmount += subtotal

		options := "null"
		if len(enrichedOpts) > 0 {
			b, _ := json.Marshal(enrichedOpts)
			options = string(b)
		}
		orderItems = append(orderItems, model.OrderItem{
			ProductID:   item.ProductID,
			ProductName: product.Name,
			Quantity:    item.Quantity,
			UnitPrice:   unitPrice,
			Options:     options,
			Subtotal:    subtotal,
			Note:        item.Note,
			Status:      "pending",
		})

	}

	if req.MachineCode != "" && req.MachineID == "" {
		var machine model.Machine
		if err := s.db.Where("machine_code = ?", req.MachineCode).First(&machine).Error; err == nil {
			req.MachineID = machine.ID
		}
	}

	order := model.Order{
		// OrderCode được gán bên trong giao dịch, xem GenerateOrderCode.
		Status:      "pending",
		TableNumber: req.TableNumber,
		TotalAmount: totalAmount,
		FinalAmount: totalAmount,
		Note:        req.Note,
		CreatedBy:   &createdBy,
	}

	if req.MemberID != "" {
		order.MemberID = &req.MemberID
	}
	if req.MachineID != "" {
		order.MachineID = &req.MachineID
	}
	tx := s.db.Begin()

	// Sinh mã trong chính giao dịch chèn đơn để khoá tư vấn có tác dụng.
	order.OrderCode = s.GenerateOrderCode(tx)

	// Khuyến mãi được xét trong chính giao dịch, ngay trước khi ghi, để mức
	// giảm khớp với danh sách khuyến mãi tại thời điểm chốt đơn.
	appliedPromoName := ""
	if promo := BestPromotionForOrder(tx, PromotionContext{
		MemberID:    req.MemberID,
		Amount:      totalAmount,
		Quantity:    totalQuantity,
		CategoryIDs: categoryIDs,
		At:          time.Now(),
	}); promo != nil {
		order.PromotionID = &promo.PromotionID
		order.DiscountAmount = promo.Discount
		order.FinalAmount = totalAmount - promo.Discount
		appliedPromoName = promo.Name
	}

	if err := tx.Create(&order).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	for i := range orderItems {
		orderItems[i].OrderID = order.ID
		if err := tx.Create(&orderItems[i]).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	items := s.loadOrderItems(order.ID)
	result := toOrderResponse(&order, items)
	result.PromotionName = appliedPromoName
	var uid *string
	if createdBy != "" {
		uid = &createdBy
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "order",
		EntityID:   order.ID,
		UserID:     uid,
		Metadata: map[string]interface{}{
			"order_code":   order.OrderCode,
			"total_amount": order.TotalAmount,
			"final_amount": order.FinalAmount,
			"status":       order.Status,
		},
	})
	s.hub.BroadcastToType(hub.Event{
		Type: "order:new",
		Data: map[string]interface{}{
			"order_id":     order.ID,
			"order_code":   order.OrderCode,
			"order_type":   model.OrderTypeProduct,
			"final_amount": order.FinalAmount,
		},
	}, hub.ClientTypeAdmin)
	return &result, nil
}

// Update sửa phần "vỏ" của đơn: số bàn, ghi chú, hội viên, máy. KHÔNG sửa món —
// đổi món phải huỷ đơn rồi tạo lại để kho và khuyến mãi tính lại từ đầu.
func (s *OrderService) Update(id string, req CreateOrderRequest, updatedBy string) (*OrderResponse, error) {
	var order model.Order
	if err := s.db.Where("id = ?", id).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy đơn hàng")
		}
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.TableNumber != "" {
		updates["table_number"] = req.TableNumber
	}
	if req.Note != "" {
		updates["note"] = req.Note
	}
	if req.MemberID != "" {
		updates["member_id"] = req.MemberID
	}
	if req.MachineID != "" {
		updates["machine_id"] = req.MachineID
	}

	if len(updates) > 0 {
		// Bảng orders KHÔNG có cột updated_at (xem model.Order): gán vào đây làm
		// PostgreSQL trả 42703 và endpoint này chưa bao giờ chạy được — không ai
		// phát hiện vì trong admin không có nút nào gọi tới.
		if updatedBy != "" {
			updates["updated_by"] = updatedBy
		}
		if err := s.db.Model(&order).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	items := s.loadOrderItems(order.ID)
	result := toOrderResponse(&order, items)
	s.audit.Log(&LogAuditRequest{
		Action:     "update",
		EntityType: "order",
		EntityID:   id,
		UserID:     optionalUUID(updatedBy),
		Metadata: map[string]interface{}{
			"order_code":   order.OrderCode,
			"status":       order.Status,
			"total_amount": order.TotalAmount,
			"final_amount": order.FinalAmount,
		},
	})
	return &result, nil
}

// Delete xoá một đơn hàng.
//
// Chặn theo TRẠNG THÁI và CHỨNG TỪ TIỀN, không theo số bản ghi con: mọi đơn đều
// có order_items, nên chặn theo con thì không đơn nào xoá được bao giờ và chức
// năng dọn đơn rác biến mất. Cái cần bảo vệ là sổ sách.
//
// Đơn đã duyệt hoặc đã hoàn tất thì đã trừ kho và có thể đã thu tiền — đường
// đúng là HUỶ đơn (UpdateStatus "cancelled"), việc đó hoàn kho và hoàn tiền tử
// tế. Xoá thẳng thì tồn kho lệch và doanh thu mất một dòng mà không ai biết.
//
// Đơn pending/cancelled sạch chứng từ thì cho xoá, kéo theo order_items vì
// chúng là con SỞ HỮU.
func (s *OrderService) Delete(id string) error {
	var order model.Order
	if err := s.db.Where("id = ?", id).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("không tìm thấy đơn hàng")
		}
		return err
	}

	if order.Status == "confirmed" || order.Status == "completed" {
		return chanVi("đơn %s đã duyệt/đã hoàn tất, không xoá được — hãy dùng chức năng huỷ đơn", order.OrderCode)
	}

	if err := kiemTraPhuThuoc(s.db, id, []phuThuoc{
		{Bang: &model.Payment{}, Cot: "order_id", Nhan: "phiếu thanh toán"},
		{Bang: &model.EInvoice{}, Cot: "order_id", Nhan: "hoá đơn điện tử đã phát hành"},
		{Bang: &model.GiftCardTransaction{}, Cot: "order_id", Nhan: "giao dịch thẻ quà tặng"},
	}, "xoá sẽ làm lệch đối soát ca và báo cáo doanh thu — hãy dùng chức năng huỷ đơn"); err != nil {
		return err
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("order_id = ?", id).Delete(&model.OrderItem{}).Error; err != nil {
			return err
		}
		return tx.Delete(&order).Error
	}); err != nil {
		return err
	}
	phatDonDoi(s.hub, &order, "deleted")
	s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "order",
		EntityID:   id,
		Metadata: map[string]interface{}{
			"order_code":   order.OrderCode,
			"status":       order.Status,
			"total_amount": order.TotalAmount,
		},
	})
	return nil
}

// BatchDelete xoá nhiều đơn, lặp qua Delete.
//
// Bản cũ chạy một câu "WHERE id IN (...)" nên bỏ qua toàn bộ phần kiểm trạng
// thái và chứng từ — chọn cả trang rồi bấm xoá là quét sạch cả đơn đã thu tiền.
// Nó cũng không phát sự kiện WebSocket nào, nên thiết bị khác giữ danh sách cũ.
//
// Dừng ở đơn đầu tiên bị chặn; những đơn trước đó đã xoá thật và đã ghi audit.
func (s *OrderService) BatchDelete(ids []string) error {
	if len(ids) == 0 {
		return errors.New("chưa chọn bản ghi nào")
	}
	for _, id := range ids {
		if err := s.Delete(id); err != nil {
			return err
		}
	}
	return nil
}

// phatDonDoi báo cho trang quản trị rằng một đơn vừa đổi trạng thái.
//
// order:new chỉ bắn lúc đơn SINH RA. Duyệt hay huỷ thì không bắn gì, nên thiết
// bị thứ hai của cùng một nhân viên — điện thoại để cạnh máy tính — vẫn hiện nút
// "Duyệt" trên một đơn đã xử lý xong, cho tới khi có người tự tải lại trang.
//
// BroadcastToType tới mọi kết nối admin, tức mọi thiết bị của mọi nhân viên.
// Không gửi cho máy trạm: trạng thái đơn của người khác không phải việc của
// khách đang ngồi máy.
func phatDonDoi(h *hub.Hub, order *model.Order, status string) {
	if h == nil || order == nil {
		return
	}
	data := map[string]interface{}{
		"order_id":   order.ID,
		"order_code": order.OrderCode,
		"order_type": order.OrderType,
		"status":     status,
	}
	if order.MemberID != nil {
		data["member_id"] = *order.MemberID
	}
	h.BroadcastToType(hub.Event{Type: "order:updated", Data: data}, hub.ClientTypeAdmin)
}

var validOrderTransitions = map[string][]string{
	"pending":   {"confirmed", "cancelled"},
	"confirmed": {"completed", "cancelled"},
	"completed": {},
	"cancelled": {},
}

func contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}

// processTopupOrder cộng tiền cho hội viên rồi chốt đơn nạp.
//
// Toàn bộ việc đọc-kiểm-ghi nằm trong MỘT transaction với đơn hàng đã khoá.
// UpdateStatus kiểm tra trạng thái trên một bản đọc trần ở ngoài transaction,
// nên hai nhân viên bấm duyệt cùng lúc — hoặc cùng một người bấm trên hai thiết
// bị — đều lọt qua guard đó và cộng tiền hai lần. Khoá ở đây là chỗ duy nhất
// chặn được.
func (s *OrderService) processTopupOrder(order *model.Order, updatedBy string) (err error) {
	now := time.Now()
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			// Giá trị trả về phải được đặt tên: không có dòng này thì panic bị
			// nuốt và hàm trả nil, admin thấy "nạp thành công" trong khi không
			// đồng nào chuyển đi.
			err = fmt.Errorf("lỗi khi xử lý đơn nạp tiền: %v", r)
		}
	}()

	var locked model.Order
	if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", order.ID).First(&locked).Error; e != nil {
		tx.Rollback()
		return errors.New("không tìm thấy đơn hàng")
	}
	if locked.Status != "pending" {
		tx.Rollback()
		return errors.New("đơn đã được xử lý")
	}
	// CreateTopupOrderRequest không bắt buộc member_id, nên đơn có thể không gắn
	// hội viên. Trước đây chỗ này deref thẳng và panic.
	if locked.MemberID == nil || *locked.MemberID == "" {
		tx.Rollback()
		return errors.New("đơn nạp tiền không gắn hội viên")
	}

	var member model.Member
	if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", *locked.MemberID).First(&member).Error; e != nil {
		tx.Rollback()
		return errors.New("không tìm thấy hội viên")
	}

	balanceAfter := member.Balance + locked.FinalAmount
	transaction := model.MemberTransaction{
		MemberID:        member.ID,
		TransactionType: "topup",
		Amount:          locked.FinalAmount,
		BalanceBefore:   member.Balance,
		BalanceAfter:    balanceAfter,
		PaymentMethod:   locked.PaymentMethod,
		Description:     "Nạp tiền qua đơn hàng " + locked.OrderCode,
		CreatedBy:       &updatedBy,
	}
	if e := tx.Create(&transaction).Error; e != nil {
		tx.Rollback()
		return e
	}
	if e := tx.Model(&member).Updates(map[string]interface{}{
		"balance":    balanceAfter,
		"updated_at": time.Now(),
	}).Error; e != nil {
		tx.Rollback()
		return e
	}
	// Điều kiện status trong chính câu UPDATE: khoá ở trên đã đủ cho hai
	// transaction song song, câu này còn chặn cả trường hợp khoá bị bỏ qua (đọc
	// ngoài tx ở nơi khác trong tương lai).
	res := tx.Model(&model.Order{}).
		Where("id = ? AND status = ?", locked.ID, "pending").
		Updates(map[string]interface{}{
			"status":       "completed",
			"updated_by":   updatedBy,
			"completed_at": &now,
		})
	if res.Error != nil {
		tx.Rollback()
		return res.Error
	}
	if res.RowsAffected == 0 {
		tx.Rollback()
		return errors.New("đơn đã được xử lý")
	}
	if e := tx.Commit().Error; e != nil {
		return e
	}

	// Caller dựng response từ struct này nên phải đồng bộ lại sau khi ghi.
	order.Status = "completed"
	order.CompletedAt = &now

	// Mã máy tra một lần, dùng cho cả hai sự kiện dưới.
	machineCode := ""
	if locked.MachineID != nil {
		var machine model.Machine
		if e := s.db.Select("machine_code").Where("id = ?", *locked.MachineID).First(&machine).Error; e == nil {
			machineCode = machine.MachineCode
		}
	}

	// Đi qua helper chung: nó gửi balance:updated tới mọi thiết bị của hội viên
	// và member:updated tới mọi thiết bị quản trị.
	phatSoDuMoi(s.hub, *locked.MemberID, balanceAfter, member.BonusBalance)
	// Máy trạm xác thực bằng mã máy chứ không có tài khoản, nên SendToUser
	// không với tới nó — phải gửi riêng theo mã máy.
	if machineCode != "" {
		s.hub.SendToMachine(machineCode, hub.Event{
			Type: "balance:updated",
			Data: map[string]interface{}{
				"member_id": *locked.MemberID,
				"balance":   balanceAfter,
			},
		})
		s.hub.SendToMachine(machineCode, hub.Event{
			Type: "topup:confirmed",
			Data: map[string]interface{}{
				"amount":   locked.FinalAmount,
				"order_id": locked.ID,
			},
		})
	}
	return nil
}

// PaymentMethodBalance settles an order against the member's stored balance.
// Anything else is treated as money taken outside the system (cash, transfer),
// which is recorded but moves no balance.
const PaymentMethodBalance = "balance"

// PaymentMethodGiftCard trừ tiền từ thẻ quà tặng. Việc trừ nằm trong CHÍNH
// giao dịch chốt đơn: trừ thẻ mà đơn không chốt được là mất tiền của khách.
const PaymentMethodGiftCard = "gift_card"

// settleOrder performs the money side of completing an order and must be the
// only place that does so — Pay and the completed status transition both route
// through here, so the two can never drift apart again.
//
// It records a payment for the order's own FinalAmount (never a caller-supplied
// figure) and, when paying from balance, deducts it from the member inside the
// same transaction as the order update.
func (s *OrderService) settleOrder(tx *gorm.DB, order *model.Order, method, referenceCode, updatedBy string, now time.Time) error {
	if method == "" {
		method = order.PaymentMethod
	}
	if method == "" {
		method = "cash"
	}

	if method == PaymentMethodBalance {
		if order.MemberID == nil || *order.MemberID == "" {
			return errors.New("đơn hàng không gắn hội viên nên không thể trừ số dư")
		}

		var member model.Member
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", *order.MemberID).First(&member).Error; err != nil {
			return errors.New("không tìm thấy hội viên")
		}
		if member.Balance < order.FinalAmount {
			return errors.New("số dư không đủ để thanh toán đơn hàng")
		}

		balanceAfter := member.Balance - order.FinalAmount
		transaction := model.MemberTransaction{
			MemberID:        member.ID,
			TransactionType: "order_payment",
			Amount:          -order.FinalAmount,
			BalanceBefore:   member.Balance,
			BalanceAfter:    balanceAfter,
			PaymentMethod:   method,
			ReferenceID:     &order.ID,
			Description:     "Thanh toán đơn hàng " + order.OrderCode,
		}
		if updatedBy != "" {
			transaction.CreatedBy = &updatedBy
		}
		if err := tx.Create(&transaction).Error; err != nil {
			return err
		}
		if err := tx.Model(&member).Update("balance", balanceAfter).Error; err != nil {
			return err
		}
	}

	// Đơn của hội viên cộng vào total_spent bất kể trả bằng gì: hạng hội viên
	// phản ánh số tiền khách tiêu tại quán, không phải cách họ trả. Trước đây
	// chỉ tiền mua gói cước được cộng nên hạng gần như đứng yên.
	//
	// Đơn đã hoàn tất không huỷ được (bảng chuyển trạng thái cho "completed" đi
	// tới rỗng), nên không có đường nào phải trừ ngược lại.
	if order.MemberID != nil && *order.MemberID != "" && order.FinalAmount > 0 &&
		order.OrderType != model.OrderTypeTopup {
		if err := tx.Model(&model.Member{}).Where("id = ?", *order.MemberID).
			UpdateColumn("total_spent", gorm.Expr("total_spent + ?", order.FinalAmount)).Error; err != nil {
			return err
		}
	}

	payment := model.Payment{
		OrderID:       order.ID,
		PaymentMethod: method,
		Amount:        order.FinalAmount,
		ReferenceCode: referenceCode,
		Status:        "completed",
		PaidAt:        &now,
	}
	if err := tx.Create(&payment).Error; err != nil {
		return err
	}

	updates := map[string]interface{}{
		"status":         "completed",
		"payment_method": method,
		"completed_at":   &now,
	}
	// updated_by is a uuid column: an empty string is not a null uuid and
	// PostgreSQL rejects it outright. Pay has no acting user, so the column is
	// simply left untouched there.
	if updatedBy != "" {
		updates["updated_by"] = updatedBy
	}

	return tx.Model(order).Updates(updates).Error
}

// khoaDonHang khoá đơn trong transaction rồi xác nhận nó vẫn đúng trạng thái mà
// UpdateStatus đã đọc ở ngoài.
//
// UpdateStatus đọc đơn bằng một câu SELECT trần rồi mới quyết định làm gì. Hai
// nhân viên bấm cùng lúc — hay cùng một người bấm trên điện thoại và máy tính —
// đều đọc được trạng thái cũ và đều lọt qua validOrderTransitions. Transaction
// thứ hai phải dừng ở đây: nó chờ khoá, đọc lại và thấy trạng thái đã đổi.
func khoaDonHang(tx *gorm.DB, id, mongDoi string) error {
	var locked model.Order
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&locked).Error; err != nil {
		return errors.New("không tìm thấy đơn hàng")
	}
	if locked.Status != mongDoi {
		return errors.New("đơn đã được xử lý")
	}
	return nil
}

func (s *OrderService) UpdateStatus(id, updatedBy string, req UpdateStatusRequest) (*OrderResponse, error) {
	var order model.Order
	if err := s.db.Where("id = ?", id).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy đơn hàng")
		}
		return nil, err
	}

	if order.OrderType == model.OrderTypeTopup && order.Status == "pending" && req.Status == "completed" {
		req.Status = "confirmed"
	}

	if !contains(validOrderTransitions[order.Status], req.Status) {
		return nil, errors.New("không thể chuyển từ " + order.Status + " sang " + req.Status)
	}

	switch req.Status {
	case "confirmed":
		if order.OrderType == model.OrderTypeTopup {
			if err := s.processTopupOrder(&order, updatedBy); err != nil {
				return nil, err
			}
			req.Status = "completed"
			goto afterUpdate
		}
		tx := s.db.Begin()
		if err := khoaDonHang(tx, order.ID, order.Status); err != nil {
			tx.Rollback()
			return nil, err
		}
		if err := tx.Model(&order).Updates(map[string]interface{}{
			"status":     "confirmed",
			"updated_by": updatedBy,
		}).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		khoDaDoi, err := s.deductStockForOrder(tx, order.ID, order.OrderCode, updatedBy)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		if err := tx.Commit().Error; err != nil {
			return nil, err
		}
		phatTonKhoDoi(s.hub, khoDaDoi...)

	case "completed":
		now := time.Now()
		if order.OrderType == model.OrderTypeTopup {
			if err := s.processTopupOrder(&order, updatedBy); err != nil {
				return nil, err
			}
			goto afterUpdate
		}
		tx := s.db.Begin()
		if err := khoaDonHang(tx, order.ID, order.Status); err != nil {
			tx.Rollback()
			return nil, err
		}
		if err := s.settleOrder(tx, &order, order.PaymentMethod, "", updatedBy, now); err != nil {
			tx.Rollback()
			return nil, err
		}
		if err := tx.Commit().Error; err != nil {
			return nil, err
		}

	case "cancelled":
		// Nhớ trạng thái CŨ trước khi ghi: tx.Model(&order).Updates(...) của
		// GORM ghi ngược giá trị mới vào chính struct, nên ngay sau lệnh đó
		// order.Status đã là "cancelled". Điều kiện trả kho ở dưới so với
		// "confirmed" vì thế KHÔNG BAO GIỜ đúng — huỷ một đơn đã xác nhận không
		// hoàn lại tồn kho, hàng bốc hơi khỏi sổ. Dựng lại được trên hệ thống
		// thật: tồn 47 → xác nhận đơn 5 → 42 → huỷ đơn → vẫn 42.
		trangThaiTruoc := order.Status

		tx := s.db.Begin()
		// Không khoá thì hai lệnh huỷ song song đều thấy status "confirmed" và
		// đều gọi restoreStockForOrder — tồn kho được cộng lại hai lần, tức là
		// sinh hàng từ hư không.
		if err := khoaDonHang(tx, order.ID, trangThaiTruoc); err != nil {
			tx.Rollback()
			return nil, err
		}
		if err := tx.Model(&order).Updates(map[string]interface{}{
			"status":     "cancelled",
			"updated_by": updatedBy,
		}).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		var khoDaDoi []string
		if trangThaiTruoc == "confirmed" && order.OrderType != model.OrderTypeTopup {
			var err error
			khoDaDoi, err = s.restoreStockForOrder(tx, order.ID, order.OrderCode, updatedBy)
			if err != nil {
				tx.Rollback()
				return nil, err
			}
		}
		if err := tx.Commit().Error; err != nil {
			return nil, err
		}
		phatTonKhoDoi(s.hub, khoDaDoi...)
	}

afterUpdate:

	phatDonDoi(s.hub, &order, req.Status)

	items := s.loadOrderItems(order.ID)
	result := toOrderResponse(&order, items)
	s.audit.Log(&LogAuditRequest{
		Action:     "update_status",
		EntityType: "order",
		EntityID:   id,
		UserID:     optionalUUID(updatedBy),
		Metadata: map[string]interface{}{
			"order_code":   order.OrderCode,
			"status":       req.Status,
			"total_amount": order.TotalAmount,
			"final_amount": order.FinalAmount,
		},
	})
	return &result, nil
}

type itemDeductionSet struct {
	productID string
	deds      []StockDeductionItem
}

type itemStockUpdate struct {
	productID    string
	currentStock float64
	// Ba trường dưới chỉ để ghi sổ kho. Nguyên liệu của món chế biến đã có bút
	// toán "outbound" từ DeductItemsStock, còn hàng bán thẳng thì trước đây chỉ
	// sửa mỗi con số current_stock — sổ Giao dịch tồn kho trống trơn trong khi
	// tồn kho vẫn đổi, và kỳ kiểm kê không có gì để đối chiếu. Quán bán nước
	// đóng chai, mì gói… tức là phần lớn hàng hoá, rơi đúng vào nhánh này.
	tenSanPham string
	khoTruoc   float64
	soLuong    float64
}

// stockOp là chiều của một lượt tính kho.
//
// Hai chiều dùng CHUNG một phép phân nhánh trong computeDeductions, cố ý. Trước
// đây chiều hoàn có vòng lặp riêng với điều kiện riêng, và hai bên lệch nhau:
// chiều trừ bỏ qua cột thô của món có công thức còn chiều hoàn thì cộng vào —
// mỗi lần huỷ đơn là sinh tồn kho từ hư không.
type stockOp int

const (
	stockOpDeduct stockOp = iota
	stockOpRestore
)

// computeDeductions tính phần tồn kho phải trừ cho một đơn.
//
// Đọc bằng chính giao dịch sẽ ghi và khoá dòng sản phẩm: bản cũ đọc bằng kết
// nối ngoài giao dịch rồi mới ghi trong giao dịch, nên hai đơn cùng lúc đều
// thấy tồn kho cũ và bán vượt số hàng thực có.
//
// op quyết định chiều: trừ kho (thiếu hàng phải báo lỗi) hay hoàn kho.
func (s *OrderService) computeDeductions(tx *gorm.DB, items []model.OrderItem, op stockOp) ([]itemDeductionSet, []itemStockUpdate, error) {
	var deductions []itemDeductionSet
	var stockUpdates []itemStockUpdate

	db := tx
	if db == nil {
		db = s.db
	}

	// Phần đã hứa trừ trong CHÍNH lượt tính này, cộng dồn theo sản phẩm.
	//
	// Không có nó thì một sản phẩm nằm ở hai dòng order_items sẽ được tính hai
	// lần từ cùng một mốc tồn kho: lệnh UPDATE sau ghi đè lệnh trước nên trừ
	// thiếu, và phép kiểm "đủ hàng" cũng đọc mốc cũ nên cho qua cả khi tổng hai
	// dòng vượt tồn. Nhánh nguyên liệu không dính vì DeductItemsStock đã tự cộng
	// dồn theo IngredientID.
	daHua := map[string]float64{}

	for _, item := range items {
		var product model.Product
		q := db
		if tx != nil {
			q = tx.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := q.Where("id = ?", item.ProductID).First(&product).Error; err != nil {
			return nil, nil, fmt.Errorf("không tìm thấy sản phẩm %s", item.ProductID)
		}

		// Định lượng đọc TRƯỚC, và đọc bằng chính giao dịch đang ghi.
		//
		// Trước: đọc bằng s.db (kết nối khác trong pool) nên không thấy thứ mà
		// giao dịch này vừa ghi, và không cùng ảnh chụp với dòng products vừa
		// khoá. Lỗi đọc còn bị nuốt, mà từ giờ "không có định lượng" là một
		// nhánh rẽ chứ không còn vô hại — nuốt lỗi thành bán tự do.
		var pms []model.ProductIngredient
		if err := db.Where("product_id = ?", item.ProductID).Find(&pms).Error; err != nil {
			return nil, nil, fmt.Errorf("không đọc được định lượng sản phẩm %s: %w", product.Name, err)
		}

		// Kho của một sản phẩm là MỘT trong hai nguồn, không bao giờ cả hai.
		//
		// Đây đúng là quy ước mà loadProductStock dùng để hiển thị: món có công
		// thức thì tồn kho là min(tồn nguyên liệu / định mức), cột current_stock
		// thô bị bỏ qua hoàn toàn. Chỗ này trước đây lệch khỏi quy ước đó nên
		// sinh hai lỗi cùng lúc — trừ hai lần cho món có công thức, và điều kiện
		// CurrentStock > 0 che mất phép kiểm khi tồn đúng bằng 0.
		switch {
		case len(pms) > 0:
			// Món chế biến: trừ ở nguyên liệu. DeductItemsStock là nơi chặn khi
			// thiếu. KHÔNG đụng cột thô — quán nhập nguyên liệu chứ không nhập
			// thành phẩm, nên con số đó không ai nạp và không ai nhìn.
			deds := make([]StockDeductionItem, 0, len(pms))
			for _, pm := range pms {
				deds = append(deds, StockDeductionItem{
					IngredientID: pm.IngredientID,
					Quantity:     pm.Quantity * float64(item.Quantity),
				})
			}
			deductions = append(deductions, itemDeductionSet{productID: item.ProductID, deds: deds})

		case product.HasStock:
			// Hàng bán thẳng, kho tự quản.
			if op == stockOpRestore {
				conLai := product.CurrentStock + daHua[item.ProductID] + float64(item.Quantity)
				daHua[item.ProductID] += float64(item.Quantity)
				stockUpdates = append(stockUpdates, itemStockUpdate{
					productID:    item.ProductID,
					currentStock: conLai,
					tenSanPham:   product.Name,
					khoTruoc:     conLai - float64(item.Quantity),
					soLuong:      float64(item.Quantity),
				})
				break
			}

			conLai := product.CurrentStock - daHua[item.ProductID]
			newStock := conLai - float64(item.Quantity)
			if newStock < 0 {
				// Bản cũ lặng lẽ cắt về 0 và vẫn cho đơn đi tiếp, khiến sổ
				// sách khớp còn hàng trong kho thì không.
				return nil, nil, fmt.Errorf("sản phẩm %s không đủ tồn kho (còn %.2f, cần %d)",
					product.Name, conLai, item.Quantity)
			}
			daHua[item.ProductID] += float64(item.Quantity)
			stockUpdates = append(stockUpdates, itemStockUpdate{
				productID:    item.ProductID,
				currentStock: newStock,
				tenSanPham:   product.Name,
				khoTruoc:     conLai,
				soLuong:      float64(item.Quantity),
			})

		default:
			// Không định lượng, không bật has_stock: sản phẩm này không theo dõi
			// tồn kho, bán bao nhiêu cũng được. Giữ nguyên hành vi cũ.
		}

		// Nguyên liệu của tuỳ chọn nằm NGOÀI switch: topping trừ theo nguyên
		// liệu bất kể món gốc quản kho kiểu gì.
		if item.Options != "" && item.Options != "null" {
			var opts []OrderOption
			if err := json.Unmarshal([]byte(item.Options), &opts); err == nil {
				for _, opt := range opts {
					var po model.ProductOption
					if err := db.Where("id = ? AND product_id = ?", opt.OptionID, item.ProductID).First(&po).Error; err != nil {
						continue
					}
					if po.IngredientID != nil {
						deductions = append(deductions, itemDeductionSet{
							productID: item.ProductID,
							deds: []StockDeductionItem{{
								IngredientID: *po.IngredientID,
								Quantity:     po.Quantity * opt.Quantity * float64(item.Quantity),
							}},
						})
					}
				}
			}
		}
	}

	return deductions, stockUpdates, nil
}

// Trả về id của mọi sản phẩm có tồn kho vừa đổi — cả hàng bán thẳng lẫn nguyên
// liệu — để người gọi phát stock:changed SAU khi commit.
// ghiSoKhoHangBanThang tạo bút toán cho phần tồn kho của hàng bán thẳng.
//
// Nguyên liệu của món chế biến đã được DeductItemsStock ghi sổ; hàng bán thẳng
// thì trước đây chỉ đổi mỗi con số current_stock, nên trang "Giao dịch tồn kho"
// trống trơn dù kho vẫn chạy, và kỳ kiểm kê không có gì để truy. Bút toán ở đây
// dùng cùng kiểu "outbound"/"inbound" và cùng cách mô tả với nhánh nguyên liệu.
func ghiSoKhoHangBanThang(tx *gorm.DB, u itemStockUpdate, loai, orderID, orderCode, actorID string) error {
	if u.soLuong == 0 {
		return nil
	}
	productID := u.productID
	referenceID := orderID
	moTa := fmt.Sprintf("Xuất kho theo đơn %s", orderCode)
	if loai == "inbound" {
		moTa = fmt.Sprintf("Hoàn kho do huỷ đơn %s", orderCode)
	}
	bt := model.StockTransaction{
		ProductID:       &productID,
		TransactionType: loai,
		Quantity:        u.soLuong,
		StockBefore:     u.khoTruoc,
		StockAfter:      u.currentStock,
		ReferenceID:     &referenceID,
		Description:     moTa,
	}
	// Cột "Người thực hiện" của sổ kho: phiếu nhập/xuất thủ công có tên nhân
	// viên, còn bút toán sinh từ đơn hàng thì bỏ trống — sổ không trả lời được
	// "ai bán chỗ hàng này".
	if id := uuidRongThanhNil(&actorID); id != nil {
		bt.CreatedBy = id
	}
	return tx.Create(&bt).Error
}

func (s *OrderService) deductStockForOrder(tx *gorm.DB, orderID, orderCode, actorID string) ([]string, error) {
	items := s.loadOrderItems(orderID)
	deductions, stockUpdates, err := s.computeDeductions(tx, items, stockOpDeduct)
	if err != nil {
		return nil, err
	}

	var daDoi []string
	for _, d := range deductions {
		if err := s.inv.DeductItemsStock(tx, d.deds, d.productID, orderID, orderCode); err != nil {
			return nil, err
		}
		for _, ded := range d.deds {
			daDoi = append(daDoi, ded.IngredientID)
		}
	}

	for _, u := range stockUpdates {
		if err := tx.Model(&model.Product{}).Where("id = ?", u.productID).Update("current_stock", u.currentStock).Error; err != nil {
			return nil, err
		}
		if err := ghiSoKhoHangBanThang(tx, u, "outbound", orderID, orderCode, actorID); err != nil {
			return nil, err
		}
		daDoi = append(daDoi, u.productID)
	}

	return daDoi, nil
}

func (s *OrderService) restoreStockForOrder(tx *gorm.DB, orderID, orderCode, actorID string) ([]string, error) {
	items := s.loadOrderItems(orderID)
	deductions, stockUpdates, err := s.computeDeductions(tx, items, stockOpRestore)
	if err != nil {
		return nil, err
	}

	var daDoi []string
	for _, d := range deductions {
		if err := s.inv.RestoreItemsStock(tx, d.deds, d.productID, orderID, orderCode); err != nil {
			return nil, err
		}
		for _, ded := range d.deds {
			daDoi = append(daDoi, ded.IngredientID)
		}
	}

	// Áp thẳng stockUpdates thay vì lặp lại phép phân nhánh ở đây. Vòng lặp cũ
	// đọc lại products KHÔNG khoá dòng, nên hai lần huỷ đồng thời đọc cùng một
	// giá trị rồi ghi đè nhau — đúng lỗi mà chiều trừ đã chống bằng FOR UPDATE.
	for _, u := range stockUpdates {
		if err := tx.Model(&model.Product{}).Where("id = ?", u.productID).Update("current_stock", u.currentStock).Error; err != nil {
			return nil, err
		}
		if err := ghiSoKhoHangBanThang(tx, u, "inbound", orderID, orderCode, actorID); err != nil {
			return nil, err
		}
		daDoi = append(daDoi, u.productID)
	}

	return daDoi, nil
}

func (s *OrderService) Split(id string, req SplitOrderRequest) (*OrderResponse, error) {
	var originalOrder model.Order
	if err := s.db.Where("id = ?", id).First(&originalOrder).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy đơn hàng")
		}
		return nil, err
	}

	if len(req.Items) == 0 {
		return nil, errors.New("không có sản phẩm để tách")
	}

	tx := s.db.Begin()

	var newOrderItems []model.OrderItem
	var splitTotal int64

	for _, split := range req.Items {
		var origItem model.OrderItem
		if err := tx.Where("id = ? AND order_id = ?", split.OrderItemID, id).First(&origItem).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("không tìm thấy sản phẩm trong đơn %s", split.OrderItemID)
		}

		if split.NewQuantity <= 0 || split.NewQuantity >= origItem.Quantity {
			tx.Rollback()
			return nil, errors.New("số lượng tách không hợp lệ")
		}

		newQty := split.NewQuantity
		remainingQty := origItem.Quantity - newQty

		newSubtotal := origItem.UnitPrice * int64(newQty)
		remainingSubtotal := origItem.UnitPrice * int64(remainingQty)

		if err := tx.Model(&origItem).Updates(map[string]interface{}{
			"quantity": remainingQty,
			"subtotal": remainingSubtotal,
		}).Error; err != nil {
			tx.Rollback()
			return nil, err
		}

		splitTotal += newSubtotal

		newOrderItems = append(newOrderItems, model.OrderItem{
			ProductID:   origItem.ProductID,
			ProductName: origItem.ProductName,
			Quantity:    newQty,
			UnitPrice:   origItem.UnitPrice,
			Options:     origItem.Options,
			Subtotal:    newSubtotal,
			Status:      "pending",
			Note:        origItem.Note,
		})
	}

	orderCode := s.GenerateOrderCode(tx)

	newOrder := model.Order{
		OrderCode:   orderCode,
		Status:      "pending",
		MemberID:    originalOrder.MemberID,
		MachineID:   originalOrder.MachineID,
		TableNumber: originalOrder.TableNumber,
		TotalAmount: splitTotal,
		FinalAmount: splitTotal,
		Note:        "Split from order " + originalOrder.OrderCode,
		CreatedBy:   originalOrder.CreatedBy,
	}

	if err := tx.Create(&newOrder).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	for i := range newOrderItems {
		newOrderItems[i].OrderID = newOrder.ID
		if err := tx.Create(&newOrderItems[i]).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	var remainingTotal int64
	tx.Model(&model.OrderItem{}).Where("order_id = ?", id).Select("COALESCE(SUM(subtotal), 0)").Scan(&remainingTotal)
	tx.Model(&model.Order{}).Where("id = ?", id).Updates(map[string]interface{}{
		"total_amount": remainingTotal,
		"final_amount": remainingTotal,
	})

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	items := s.loadOrderItems(newOrder.ID)
	result := toOrderResponse(&newOrder, items)
	s.audit.Log(&LogAuditRequest{
		Action:     "split",
		EntityType: "order",
		EntityID:   id,
		Metadata: map[string]interface{}{
			"order_code":     originalOrder.OrderCode,
			"new_order_code": orderCode,
			"new_order_id":   newOrder.ID,
			"split_amount":   splitTotal,
		},
	})
	return &result, nil
}

func (s *OrderService) Pay(id string, req PayRequest) (*OrderResponse, error) {
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var order model.Order
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&order).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy đơn hàng")
		}
		return nil, err
	}

	if order.Status == "completed" {
		tx.Rollback()
		return nil, errors.New("đơn hàng đã hoàn tất")
	}
	if order.Status == "cancelled" {
		tx.Rollback()
		return nil, errors.New("đơn hàng đã bị huỷ")
	}

	// The caller states what it is paying; a mismatch means the client and the
	// server disagree about the price, which must never settle silently.
	if req.Amount != order.FinalAmount {
		tx.Rollback()
		return nil, fmt.Errorf("số tiền thanh toán (%d) không khớp với giá trị đơn hàng (%d)", req.Amount, order.FinalAmount)
	}

	// Paying straight from pending skips the confirm step, so the stock it
	// would have deducted has to be deducted here.
	var khoDaDoi []string
	if order.Status == "pending" {
		var err error
		// Pay không nhận người thực hiện (settleOrder ngay dưới cũng truyền ""),
		// nên bút toán kho ở nhánh này chưa có tên nhân viên.
		khoDaDoi, err = s.deductStockForOrder(tx, order.ID, order.OrderCode, "")
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if req.PaymentMethod == PaymentMethodGiftCard {
		if req.CardSerial == "" || req.CardSecret == "" {
			tx.Rollback()
			return nil, errors.New("thiếu seri hoặc mã thẻ quà tặng")
		}
		if err := spendGiftCard(tx, req.CardSerial, req.CardSecret, order.FinalAmount, order.ID); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	now := time.Now()
	if err := s.settleOrder(tx, &order, req.PaymentMethod, req.ReferenceCode, "", now); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	phatTonKhoDoi(s.hub, khoDaDoi...)
	phatDonDoi(s.hub, &order, "completed")

	items := s.loadOrderItems(order.ID)
	result := toOrderResponse(&order, items)
	if payments, err := s.GetPayments(order.ID); err == nil {
		result.Payments = payments
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "pay",
		EntityType: "order",
		EntityID:   id,
		Metadata: map[string]interface{}{
			"order_code":     order.OrderCode,
			"amount":         order.FinalAmount,
			"payment_method": order.PaymentMethod,
			"status":         "completed",
		},
	})
	return &result, nil
}

func (s *OrderService) GetPayments(id string) ([]PaymentResponse, error) {
	var payments []model.Payment
	if err := s.db.Where("order_id = ?", id).Find(&payments).Error; err != nil {
		return nil, err
	}

	result := make([]PaymentResponse, len(payments))
	for i := range payments {
		result[i] = toPaymentResponse(&payments[i])
	}

	return result, nil
}

// orderCodeLockKey là khoá tư vấn dùng riêng cho việc sinh mã đơn. Giá trị chỉ
// cần ổn định và không đụng khoá nào khác trong ứng dụng.
const orderCodeLockKey = 4713001

// GenerateOrderCode sinh mã đơn kế tiếp bên trong giao dịch đang mở.
//
// Trước đây hàm này đọc mã lớn nhất rồi cộng một mà không khoá gì cả, nên hai
// đơn đặt cùng lúc sinh ra cùng một mã và đơn thứ hai vỡ ở ràng buộc duy nhất
// (trả 500 cho người dùng). Khoá tư vấn ở cấp giao dịch nối tiếp hoá đoạn này
// và tự nhả khi commit.
func (s *OrderService) GenerateOrderCode(tx *gorm.DB) string {
	db := tx
	if db == nil {
		db = s.db
	} else {
		db.Exec("SELECT pg_advisory_xact_lock(?)", orderCodeLockKey)
	}

	// Unscoped: chỉ mục duy nhất idx_orders_order_code phủ CẢ dòng đã xoá mềm,
	// nên bỏ qua chúng khi tìm mã lớn nhất sẽ cấp lại một mã đã tồn tại. Hậu quả
	// thật: quầy xoá một đơn gõ nhầm, và đơn kế tiếp báo lỗi trùng khoá — không
	// bán hàng được nữa cho tới khi có người sửa database.
	var lastOrder model.Order
	db.Unscoped().Where("order_code LIKE ?", "ORD-%").Order("order_code DESC").First(&lastOrder)
	seq := int64(1)
	if lastOrder.OrderCode != "" {
		parts := strings.Split(lastOrder.OrderCode, "-")
		if len(parts) == 2 {
			if n, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
				seq = n + 1
			}
		}
	}
	return utils.GenerateCode("ORD", seq, 5)
}

func (s *OrderService) loadOrderItems(orderID string) []model.OrderItem {
	var items []model.OrderItem
	s.db.Where("order_id = ?", orderID).Find(&items)
	return items
}

func (s *OrderService) loadOrderPayments(orderID string) []PaymentResponse {
	var payments []model.Payment
	s.db.Where("order_id = ?", orderID).Find(&payments)
	result := make([]PaymentResponse, len(payments))
	for i := range payments {
		result[i] = toPaymentResponse(&payments[i])
	}
	return result
}

func (s *OrderService) batchLoadMemberNames(ids []string) map[string]string {
	result := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return result
	}
	var members []struct {
		ID       string
		FullName string
		Username string
	}
	s.db.Model(&model.Member{}).Where("id IN ?", ids).Select("id, full_name, username").Find(&members)
	for _, m := range members {
		n := m.FullName
		if n == "" {
			n = m.Username
		}
		result[m.ID] = n
	}
	return result
}

func (s *OrderService) batchLoadMemberUsernames(ids []string) map[string]string {
	result := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return result
	}
	var members []struct {
		ID       string
		Username string
	}
	s.db.Model(&model.Member{}).Where("id IN ?", ids).Select("id, username").Find(&members)
	for _, m := range members {
		result[m.ID] = m.Username
	}
	return result
}

func (s *OrderService) batchLoadPromotionNames(ids []string) map[string]string {
	result := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return result
	}
	var promos []struct {
		ID   string
		Name string
	}
	s.db.Model(&model.Promotion{}).Where("id IN ?", ids).Find(&promos)
	for _, p := range promos {
		result[p.ID] = p.Name
	}
	return result
}

func (s *OrderService) batchLoadMachineCodes(ids []string) map[string]string {
	result := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return result
	}
	var machines []struct {
		ID          string
		MachineCode string
	}
	// Unscoped: đơn hàng cũ vẫn phải hiện mã máy sau khi máy bị gỡ khỏi danh sách.
	s.db.Unscoped().Model(&model.Machine{}).Where("id IN ?", ids).Find(&machines)
	for _, m := range machines {
		result[m.ID] = m.MachineCode
	}
	return result
}

func (s *OrderService) batchLoadUserNames(ids []string) map[string]string {
	result := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return result
	}
	var users []struct {
		ID       string
		FullName string
	}
	s.db.Model(&model.User{}).Where("id IN ?", ids).Find(&users)
	for _, u := range users {
		result[u.ID] = u.FullName
	}
	return result
}

func toOrderResponse(o *model.Order, items []model.OrderItem) OrderResponse {
	itemResponses := make([]OrderItemResponse, len(items))
	for i := range items {
		itemResponses[i] = toOrderItemResponse(&items[i])
	}
	return OrderResponse{
		ID:             o.ID,
		OrderCode:      o.OrderCode,
		Status:         o.Status,
		OrderType:      o.OrderType,
		MemberID:       o.MemberID,
		MachineID:      o.MachineID,
		TableNumber:    o.TableNumber,
		TotalAmount:    o.TotalAmount,
		DiscountAmount: o.DiscountAmount,
		FinalAmount:    o.FinalAmount,
		PaymentMethod:  o.PaymentMethod,
		PromotionID:    o.PromotionID,
		Note:           o.Note,
		CreatedBy:      o.CreatedBy,
		UpdatedBy:      o.UpdatedBy,
		CompletedAt:    o.CompletedAt,
		CreatedAt:      o.CreatedAt,
		Items:          itemResponses,
	}
}

func toPaymentResponse(p *model.Payment) PaymentResponse {
	return PaymentResponse{
		ID:            p.ID,
		OrderID:       p.OrderID,
		PaymentMethod: p.PaymentMethod,
		Amount:        p.Amount,
		ReferenceCode: p.ReferenceCode,
		Status:        p.Status,
		PaidAt:        p.PaidAt,
		CreatedAt:     p.CreatedAt,
	}
}

func toOrderItemResponse(it *model.OrderItem) OrderItemResponse {
	var optList []OrderOption
	if it.Options != "" && it.Options != "null" {
		json.Unmarshal([]byte(it.Options), &optList)
	}
	return OrderItemResponse{
		ID:          it.ID,
		OrderID:     it.OrderID,
		ProductID:   it.ProductID,
		ProductName: it.ProductName,
		Quantity:    it.Quantity,
		UnitPrice:   it.UnitPrice,
		Options:     it.Options,
		OptionList:  optList,
		Subtotal:    it.Subtotal,
		Status:      it.Status,
		Note:        it.Note,
	}
}

// --- luồng chế biến ----------------------------------------------------------
//
// OrderItem.Status khai từ đầu với mặc định "pending" nhưng KHÔNG nơi nào đổi:
// bếp in được phiếu chế biến mà không có cách nào báo lại món đã xong, nên quầy
// phải chạy xuống hỏi. Bốn trạng thái dưới đây là vòng đời một món.

var orderItemStatuses = map[string]bool{
	"pending":   true, // chờ làm
	"preparing": true, // đang làm
	"ready":     true, // xong, chờ mang ra
	"served":    true, // đã phục vụ
}

type UpdateItemStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

// UpdateItemStatus đổi trạng thái một món trong đơn.
func (s *OrderService) UpdateItemStatus(orderID, itemID, status, actorID string) (*OrderItemResponse, error) {
	if !orderItemStatuses[status] {
		return nil, fmt.Errorf("trạng thái món không hợp lệ: %s", status)
	}

	var item model.OrderItem
	if err := s.db.Where("id = ? AND order_id = ?", itemID, orderID).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy món trong đơn")
		}
		return nil, err
	}

	var order model.Order
	if err := s.db.Where("id = ?", orderID).First(&order).Error; err != nil {
		return nil, errors.New("không tìm thấy đơn hàng")
	}
	// Đơn đã huỷ thì món không còn gì để làm; cho đổi sẽ sinh phiếu bếp cho đơn
	// khách đã bỏ.
	if order.Status == "cancelled" {
		return nil, errors.New("đơn đã huỷ, không đổi được trạng thái món")
	}

	if err := s.db.Model(&item).Update("status", status).Error; err != nil {
		return nil, err
	}
	item.Status = status

	s.audit.Log(&LogAuditRequest{
		Action:     "update_item_status",
		EntityType: "order_item",
		EntityID:   itemID,
		UserID:     optionalUUID(actorID),
		Metadata: map[string]interface{}{
			"order_id":     orderID,
			"order_code":   order.OrderCode,
			"product_name": item.ProductName,
			"status":       status,
		},
	})

	// Quầy cần biết ngay món nào vừa xong để mang ra, không phải bấm làm mới.
	// Chỉ quầy — máy khách không có màn hình nào hiện thứ này.
	if s.hub != nil {
		s.hub.BroadcastToType(hub.Event{
			Type: "order:item_status",
			Data: map[string]interface{}{
				"order_id":     orderID,
				"order_code":   order.OrderCode,
				"item_id":      itemID,
				"product_name": item.ProductName,
				"status":       status,
			},
		}, hub.ClientTypeAdmin)
	}

	resp := toOrderItemResponse(&item)
	return &resp, nil
}
