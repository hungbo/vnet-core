package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/utils"
	"gorm.io/gorm"
)

type OrderOption struct {
	OptionID string  `json:"option_id"`
	Name     string  `json:"name"`
	Price    int64   `json:"price"`
	Quantity float64 `json:"quantity"`
}

type OrderService struct {
	db    *gorm.DB
	audit *AuditService
	inv   *InventoryService
}

func NewOrderService(db *gorm.DB, audit *AuditService, inv *InventoryService) *OrderService {
	return &OrderService{db: db, audit: audit, inv: inv}
}

type CreateOrderRequest struct {
	MemberID    string             `json:"member_id"`
	MachineID   string             `json:"machine_id"`
	MachineCode string             `json:"machine_code"`
	TableNumber string             `json:"table_number"`
	Note        string             `json:"note"`
	Items       []OrderItemRequest `json:"items"`
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
}

type OrderResponse struct {
	ID             string              `json:"id"`
	OrderCode      string              `json:"order_code"`
	Status         string              `json:"status"`
	MemberID       *string             `json:"member_id"`
	MachineID      *string             `json:"machine_id"`
	StoreID        *string             `json:"store_id"`
	TableNumber    string              `json:"table_number"`
	TotalAmount    int64               `json:"total_amount"`
	DiscountAmount int64               `json:"discount_amount"`
	FinalAmount    int64               `json:"final_amount"`
	Note           string              `json:"note"`
	CreatedBy      *string             `json:"created_by"`
	UpdatedBy      *string             `json:"updated_by"`
	UpdatedByName  string              `json:"updated_by_name,omitempty"`
	MemberName     string              `json:"member_name,omitempty"`
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

func (s *OrderService) List(params pagination.Params, storeID string) ([]OrderResponse, int64, int, int, error) {
	query := s.db.Model(&model.Order{}).Where("store_id = ? AND deleted_at IS NULL", storeID)

	if params.Search != "" {
		search := "%" + params.Search + "%"
		query = query.Where("note ILIKE ?", search)
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
	var memberIDs, machineIDs, userIDs []string
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
	}

	memberNames := s.batchLoadMemberNames(memberIDs)
	machineCodes := s.batchLoadMachineCodes(machineIDs)
	userNames := s.batchLoadUserNames(userIDs)

	for i := range result {
		if orders[i].MemberID != nil {
			result[i].MemberName = memberNames[*orders[i].MemberID]
		}
		if orders[i].MachineID != nil {
			result[i].MachineCode = machineCodes[*orders[i].MachineID]
		}
		if orders[i].UpdatedBy != nil {
			result[i].UpdatedByName = userNames[*orders[i].UpdatedBy]
		}
	}

	return result, total, params.Page, params.PageSize, nil
}

func (s *OrderService) GetByID(id string) (*OrderResponse, error) {
	var order model.Order
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&order).Error; err != nil {
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
		result.MemberName = names[*order.MemberID]
	}
	if order.MachineID != nil {
		codes := s.batchLoadMachineCodes([]string{*order.MachineID})
		result.MachineCode = codes[*order.MachineID]
	}
	if order.UpdatedBy != nil {
		names := s.batchLoadUserNames([]string{*order.UpdatedBy})
		result.UpdatedByName = names[*order.UpdatedBy]
	}

	return &result, nil
}

func (s *OrderService) Create(req CreateOrderRequest, createdBy string, storeID string) (*OrderResponse, error) {
	if len(req.Items) == 0 {
		return nil, errors.New("đơn hàng phải có ít nhất một sản phẩm")
	}

	orderCode := s.GenerateOrderCode()

	var totalAmount int64
	var orderItems []model.OrderItem

	for _, item := range req.Items {
		if item.ProductID == "" {
			return nil, errors.New("thiếu mã sản phẩm")
		}
		if item.Quantity <= 0 {
			return nil, errors.New("số lượng phải lớn hơn 0")
		}

		var product model.Product
		if err := s.db.Where("id = ? AND deleted_at IS NULL", item.ProductID).First(&product).Error; err != nil {
			return nil, fmt.Errorf("không tìm thấy sản phẩm %s", item.ProductID)
		}

		if !product.IsRetail {
			return nil, fmt.Errorf("sản phẩm %s không phải hàng bán lẻ", product.Name)
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
		OrderCode:   orderCode,
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
	if storeID != "" {
		order.StoreID = &storeID
	}

	tx := s.db.Begin()

	if err := tx.Create(&order).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	for i := range orderItems {
		orderItems[i].OrderID = order.ID
		orderItems[i].StoreID = order.StoreID
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
	return &result, nil
}

func (s *OrderService) Update(id string, req CreateOrderRequest) (*OrderResponse, error) {
	var order model.Order
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&order).Error; err != nil {
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
		updates["updated_at"] = time.Now()
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
		Metadata: map[string]interface{}{
			"order_code":   order.OrderCode,
			"status":       order.Status,
			"total_amount": order.TotalAmount,
			"final_amount": order.FinalAmount,
		},
	})
	return &result, nil
}

func (s *OrderService) Delete(id string) error {
	var order model.Order
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("không tìm thấy đơn hàng")
		}
		return err
	}
	if err := s.db.Delete(&order).Error; err != nil {
		return err
	}
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

func (s *OrderService) UpdateStatus(id, updatedBy string, req UpdateStatusRequest) (*OrderResponse, error) {
	var order model.Order
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy đơn hàng")
		}
		return nil, err
	}

	if !contains(validOrderTransitions[order.Status], req.Status) {
		return nil, errors.New("không thể chuyển từ " + order.Status + " sang " + req.Status)
	}

	switch req.Status {
	case "confirmed":
		tx := s.db.Begin()
		if err := tx.Model(&order).Updates(map[string]interface{}{
			"status":     "confirmed",
			"updated_by": updatedBy,
		}).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		if err := s.deductStockForOrder(tx, order.ID, order.OrderCode); err != nil {
			tx.Rollback()
			return nil, err
		}
		if err := tx.Commit().Error; err != nil {
			return nil, err
		}

	case "completed":
		now := time.Now()
		tx := s.db.Begin()
		payment := model.Payment{
			OrderID:       order.ID,
			PaymentMethod: "cash",
			Amount:        order.FinalAmount,
			Status:        "completed",
			PaidAt:        &now,
		}
		if err := tx.Create(&payment).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		if err := tx.Model(&order).Updates(map[string]interface{}{
			"status":       "completed",
			"updated_by":   updatedBy,
			"completed_at": &now,
		}).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		if err := tx.Commit().Error; err != nil {
			return nil, err
		}

	case "cancelled":
		tx := s.db.Begin()
		if err := tx.Model(&order).Updates(map[string]interface{}{
			"status":     "cancelled",
			"updated_by": updatedBy,
		}).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		if order.Status == "confirmed" {
			if err := s.restoreStockForOrder(tx, order.ID, order.OrderCode); err != nil {
				tx.Rollback()
				return nil, err
			}
		}
		if err := tx.Commit().Error; err != nil {
			return nil, err
		}
	}

	items := s.loadOrderItems(order.ID)
	result := toOrderResponse(&order, items)
	s.audit.Log(&LogAuditRequest{
		Action:     "update_status",
		EntityType: "order",
		EntityID:   id,
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
}

func (s *OrderService) computeDeductions(items []model.OrderItem) ([]itemDeductionSet, []itemStockUpdate, error) {
	var deductions []itemDeductionSet
	var stockUpdates []itemStockUpdate

	for _, item := range items {
		var product model.Product
		if err := s.db.Where("id = ? AND deleted_at IS NULL", item.ProductID).First(&product).Error; err != nil {
			return nil, nil, fmt.Errorf("không tìm thấy sản phẩm %s", item.ProductID)
		}

		// Self-managed stock
		if product.HasStock && product.CurrentStock > 0 {
			newStock := product.CurrentStock - float64(item.Quantity)
			if newStock < 0 {
				newStock = 0
			}
			stockUpdates = append(stockUpdates, itemStockUpdate{
				productID:    item.ProductID,
				currentStock: newStock,
			})
		}

		// BOM ingredients
		var pms []model.ProductIngredient
		s.db.Where("product_id = ?", item.ProductID).Find(&pms)
		if len(pms) > 0 {
			deds := make([]StockDeductionItem, 0, len(pms))
			for _, pm := range pms {
				deds = append(deds, StockDeductionItem{
					IngredientID: pm.IngredientID,
					Quantity:     pm.Quantity * float64(item.Quantity),
				})
			}
			deductions = append(deductions, itemDeductionSet{productID: item.ProductID, deds: deds})
		}

		// Option ingredients
		if item.Options != "" && item.Options != "null" {
			var opts []OrderOption
			if err := json.Unmarshal([]byte(item.Options), &opts); err == nil {
				for _, opt := range opts {
					var po model.ProductOption
					if err := s.db.Where("id = ? AND product_id = ?", opt.OptionID, item.ProductID).First(&po).Error; err != nil {
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

func (s *OrderService) deductStockForOrder(tx *gorm.DB, orderID string, orderCode string) error {
	items := s.loadOrderItems(orderID)
	deductions, stockUpdates, err := s.computeDeductions(items)
	if err != nil {
		return err
	}

	for _, d := range deductions {
		if err := s.inv.DeductItemsStock(tx, d.deds, d.productID, orderID, orderCode); err != nil {
			return err
		}
	}

	for _, u := range stockUpdates {
		if err := tx.Model(&model.Product{}).Where("id = ?", u.productID).Update("current_stock", u.currentStock).Error; err != nil {
			return err
		}
	}

	return nil
}

func (s *OrderService) restoreStockForOrder(tx *gorm.DB, orderID string, orderCode string) error {
	items := s.loadOrderItems(orderID)
	deductions, _, err := s.computeDeductions(items)
	if err != nil {
		return err
	}

	for _, d := range deductions {
		if err := s.inv.RestoreItemsStock(tx, d.deds, d.productID, orderID, orderCode); err != nil {
			return err
		}
	}

	for _, item := range items {
		var product model.Product
		if err := tx.Where("id = ? AND deleted_at IS NULL", item.ProductID).First(&product).Error; err != nil {
			return err
		}
		if product.HasStock {
			restored := product.CurrentStock + float64(item.Quantity)
			if err := tx.Model(&model.Product{}).Where("id = ?", item.ProductID).Update("current_stock", restored).Error; err != nil {
				return err
			}
		}
	}

	return nil
}


func (s *OrderService) Split(id string, req SplitOrderRequest) (*OrderResponse, error) {
	var originalOrder model.Order
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&originalOrder).Error; err != nil {
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

	orderCode := s.GenerateOrderCode()

	newOrder := model.Order{
		OrderCode:   orderCode,
		Status:      "pending",
		MemberID:    originalOrder.MemberID,
		MachineID:   originalOrder.MachineID,
		StoreID:     originalOrder.StoreID,
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
		newOrderItems[i].StoreID = newOrder.StoreID
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
	var order model.Order
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy đơn hàng")
		}
		return nil, err
	}

	if order.Status == "completed" {
		return nil, errors.New("đơn hàng đã hoàn tất")
	}

	now := time.Now()

	payment := model.Payment{
		OrderID:       order.ID,
		PaymentMethod: req.PaymentMethod,
		Amount:        req.Amount,
		ReferenceCode: req.ReferenceCode,
		Status:        "completed",
		PaidAt:        &now,
	}

	tx := s.db.Begin()

	if err := tx.Create(&payment).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Model(&order).Updates(map[string]interface{}{
		"status":       "completed",
		"completed_at": &now,
	}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	items := s.loadOrderItems(order.ID)
	payments := []model.Payment{payment}
	result := toOrderResponse(&order, items)
	result.Payments = make([]PaymentResponse, len(payments))
	for i := range payments {
		result.Payments[i] = toPaymentResponse(&payments[i])
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "pay",
		EntityType: "order",
		EntityID:   id,
		Metadata: map[string]interface{}{
			"order_code":     order.OrderCode,
			"amount":         req.Amount,
			"payment_method": req.PaymentMethod,
			"status":         "completed",
		},
	})
	return &result, nil
}

func (s *OrderService) GetPayments(id string) ([]PaymentResponse, error) {
	var payments []model.Payment
	if err := s.db.Where("order_id = ? AND deleted_at IS NULL", id).Find(&payments).Error; err != nil {
		return nil, err
	}

	result := make([]PaymentResponse, len(payments))
	for i := range payments {
		result[i] = toPaymentResponse(&payments[i])
	}

	return result, nil
}

func (s *OrderService) GenerateOrderCode() string {
	var lastOrder model.Order
	s.db.Where("order_code LIKE ?", "ORD-%").Order("order_code DESC").First(&lastOrder)
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
	s.db.Where("order_id = ? AND deleted_at IS NULL", orderID).Find(&payments)
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
	}
	s.db.Model(&model.Member{}).Where("id IN ? AND deleted_at IS NULL", ids).Find(&members)
	for _, m := range members {
		result[m.ID] = m.FullName
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
	s.db.Model(&model.Machine{}).Where("id IN ? AND deleted_at IS NULL", ids).Find(&machines)
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
	var optList []OrderOption
	if items[i].Options != "" && items[i].Options != "null" {
		json.Unmarshal([]byte(items[i].Options), &optList)
	}
	itemResponses[i] = OrderItemResponse{
		ID:          items[i].ID,
		OrderID:     items[i].OrderID,
		ProductID:   items[i].ProductID,
		ProductName: items[i].ProductName,
		Quantity:    items[i].Quantity,
		UnitPrice:   items[i].UnitPrice,
		Options:     items[i].Options,
		OptionList:  optList,
		Subtotal:    items[i].Subtotal,
		Status:      items[i].Status,
		Note:        items[i].Note,
	}
	}
	return OrderResponse{
		ID:             o.ID,
		OrderCode:      o.OrderCode,
		Status:         o.Status,
		MemberID:       o.MemberID,
		MachineID:      o.MachineID,
		StoreID:        o.StoreID,
		TableNumber:    o.TableNumber,
		TotalAmount:    o.TotalAmount,
		DiscountAmount: o.DiscountAmount,
		FinalAmount:    o.FinalAmount,
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
