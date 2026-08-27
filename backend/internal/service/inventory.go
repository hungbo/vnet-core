package service

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ValidUnits = map[string]string{
	"cai":   "Cái",
	"hop":   "Hộp",
	"chai":  "Chai",
	"lon":   "Lon",
	"kg":    "Kg",
	"lit":   "Lít",
	"goi":   "Gói",
	"bich":  "Bịch",
	"thung": "Thùng",
	"ly":    "Ly",
	"coc":   "Cốc",
	"dia":   "Đĩa",
	"to":    "Tô",
	"phan":  "Phần",
	"suat":  "Suất",
	"mieng": "Miếng",
	"vien":  "Viên",
	"cay":   "Cây",
	"que":   "Que",
}

type InventoryService struct {
	db    *gorm.DB
	audit *AuditService
	hub   *hub.Hub
}

func NewInventoryService(db *gorm.DB, audit *AuditService) *InventoryService {
	return &InventoryService{db: db, audit: audit}
}

// WithHub nối hub WebSocket vào để nhập/xuất/điều chỉnh kho báo tồn kho đổi ngay lúc nó
// đổi. Không nối thì mọi thứ vẫn chạy đúng, chỉ là thực đơn trên máy trạm và
// bảng sản phẩm trên trang quản trị giữ số cũ tới khi tự tải lại.
func (s *InventoryService) WithHub(h *hub.Hub) *InventoryService {
	s.hub = h
	return s
}

type SupplierResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Address   string `json:"address"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at"`
}

type CreateSupplierRequest struct {
	Name     string `json:"name" binding:"required"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Address  string `json:"address"`
	IsActive *bool  `json:"is_active"`
}

type UpdateSupplierRequest struct {
	Name     *string `json:"name"`
	Phone    *string `json:"phone"`
	Email    *string `json:"email"`
	Address  *string `json:"address"`
	IsActive *bool   `json:"is_active"`
}

type StockTransactionResponse struct {
	ID              string  `json:"id"`
	ProductID       *string `json:"product_id"`
	ProductName     string  `json:"product_name"`
	TransactionType string  `json:"transaction_type"`
	Quantity        float64 `json:"quantity"`
	UnitPrice       int64   `json:"unit_price"`
	TotalPrice      int64   `json:"total_price"`
	StockBefore     float64 `json:"stock_before"`
	StockAfter      float64 `json:"stock_after"`
	ReferenceID     *string `json:"reference_id"`
	SupplierID      *string `json:"supplier_id"`
	SupplierName    string  `json:"supplier_name"`
	Description     string  `json:"description"`
	CreatedBy       *string `json:"created_by"`
	CreatedByName   string  `json:"created_by_name"`
	CreatedAt       string  `json:"created_at"`
}

type CreateStockTransactionRequest struct {
	ProductID       *string `json:"product_id"`
	TransactionType string  `json:"transaction_type" binding:"required,oneof=inbound outbound adjustment"`
	// Với loại adjustment, đây là số tồn kho mục tiêu — chỉnh về 0 là hợp lệ.
	Quantity    float64 `json:"quantity"`
	UnitPrice   int64   `json:"unit_price"`
	TotalPrice  int64   `json:"total_price"`
	ReferenceID *string `json:"reference_id"`
	SupplierID  *string `json:"supplier_id"`
	Description string  `json:"description"`
}

type UnitResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ProductIngredientResponse struct {
	ID             string  `json:"id"`
	ProductID      string  `json:"product_id"`
	IngredientID   string  `json:"ingredient_id"`
	IngredientName string  `json:"ingredient_name"`
	UnitID         string  `json:"unit_id"`
	UnitName       string  `json:"unit_name"`
	Quantity       float64 `json:"quantity"`
	CreatedAt      string  `json:"created_at"`
}

type CreateProductIngredientRequest struct {
	IngredientID string  `json:"ingredient_id" binding:"required"`
	Quantity     float64 `json:"quantity" binding:"gt=0"`
	UnitID       string  `json:"unit_id"`
}

type UpdateProductIngredientRequest struct {
	Quantity *float64 `json:"quantity"`
}

type StockDeductionItem struct {
	IngredientID string
	Quantity     float64
}

func (s *InventoryService) ListSuppliers() ([]SupplierResponse, error) {
	var suppliers []model.Supplier
	if err := s.db.Order("name asc").Find(&suppliers).Error; err != nil {
		return nil, err
	}

	responses := make([]SupplierResponse, len(suppliers))
	for i, su := range suppliers {
		responses[i] = supplierToResponse(su)
	}
	return responses, nil
}

func (s *InventoryService) CreateSupplier(req *CreateSupplierRequest) (*SupplierResponse, error) {
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}

	supplier := model.Supplier{
		Name:     req.Name,
		Phone:    req.Phone,
		Email:    req.Email,
		Address:  req.Address,
		IsActive: active,
	}

	if err := s.db.Create(&supplier).Error; err != nil {
		return nil, err
	}

	result := supplierToResponse(supplier)

	s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "supplier",
		EntityID:   supplier.ID,
		Metadata:   map[string]interface{}{"name": req.Name},
	})

	return &result, nil
}

func (s *InventoryService) UpdateSupplier(id string, req *UpdateSupplierRequest) (*SupplierResponse, error) {
	var supplier model.Supplier
	if err := s.db.Where("id = ?", id).First(&supplier).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy nhà cung cấp")
		}
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Phone != nil {
		updates["phone"] = *req.Phone
	}
	if req.Email != nil {
		updates["email"] = *req.Email
	}
	if req.Address != nil {
		updates["address"] = *req.Address
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) > 0 {
		if err := s.db.Model(&supplier).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	s.db.First(&supplier, "id = ?", id)
	result := supplierToResponse(supplier)

	s.audit.Log(&LogAuditRequest{
		Action:     "update",
		EntityType: "supplier",
		EntityID:   supplier.ID,
		Metadata:   map[string]interface{}{"name": supplier.Name, "updates": updates},
	})

	return &result, nil
}

// DeleteSupplier xoá một nhà cung cấp.
//
// Phiếu nhập kho là chứng từ mua hàng, còn sản phẩm thì trỏ tới nhà cung cấp để
// biết đặt hàng ở đâu. Xoá mà bỏ lại cả hai thì trang quản trị hiện UUID thô ở
// chỗ đáng lẽ là tên nhà cung cấp.
func (s *InventoryService) DeleteSupplier(id string) error {
	var supplier model.Supplier
	if err := s.db.Where("id = ?", id).First(&supplier).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("không tìm thấy nhà cung cấp")
		}
		return err
	}

	if err := kiemTraPhuThuoc(s.db, id, []phuThuoc{
		{Bang: &model.Product{}, Cot: "supplier_id", Nhan: "sản phẩm"},
		{Bang: &model.StockTransaction{}, Cot: "supplier_id", Nhan: "phiếu nhập kho"},
	}, "hãy ngừng hợp tác (bỏ đang hoạt động) thay vì xoá"); err != nil {
		return err
	}

	now := time.Now()
	if err := s.db.Model(&supplier).Update("deleted_at", &now).Error; err != nil {
		return err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "supplier",
		EntityID:   supplier.ID,
		Metadata:   map[string]interface{}{"name": supplier.Name},
	})

	return nil
}

func (s *InventoryService) ListStockTransactions(params *pagination.Params) (*pagination.Result, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}
	params.Sort = "created_at"
	params.Order = "desc"

	var transactions []model.StockTransaction
	query := s.db

	var total int64
	query.Model(&model.StockTransaction{}).Count(&total)

	if err := pagination.Apply(query, params).Find(&transactions).Error; err != nil {
		return nil, err
	}

	items := make([]StockTransactionResponse, len(transactions))
	for i, t := range transactions {
		items[i] = stockTransactionToResponse(s.db, t)
	}

	return pagination.NewResult(items, total, params), nil
}

func (s *InventoryService) CreateStockTransaction(req *CreateStockTransactionRequest, createdBy string) (*StockTransactionResponse, error) {
	if req.ProductID == nil {
		return nil, errors.New("cần product_id")
	}

	var product model.Product
	if err := s.db.Where("id = ?", *req.ProductID).First(&product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy sản phẩm")
		}
		return nil, err
	}

	entityName := product.Name
	stockBefore := product.CurrentStock
	var stockAfter float64

	if req.TotalPrice == 0 && req.UnitPrice > 0 && req.Quantity > 0 {
		req.TotalPrice = int64(float64(req.UnitPrice) * req.Quantity)
	}

	switch req.TransactionType {
	case "inbound":
		stockAfter = stockBefore + req.Quantity
	case "outbound":
		stockAfter = stockBefore - req.Quantity
		if stockAfter < 0 {
			return nil, errors.New("tồn kho không đủ")
		}
	case "adjustment":
		stockAfter = req.Quantity
	default:
		return nil, fmt.Errorf("loại giao dịch không hợp lệ: %s", req.TransactionType)
	}

	tx := s.db.Begin()

	if err := tx.Model(&product).Update("current_stock", stockAfter).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	transaction := model.StockTransaction{
		ProductID:       req.ProductID,
		TransactionType: req.TransactionType,
		Quantity:        req.Quantity,
		UnitPrice:       req.UnitPrice,
		TotalPrice:      req.TotalPrice,
		StockBefore:     stockBefore,
		StockAfter:      stockAfter,
		ReferenceID:     req.ReferenceID,
		SupplierID:      req.SupplierID,
		Description:     req.Description,
		CreatedBy:       &createdBy,
	}

	if err := tx.Create(&transaction).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	phatTonKhoDoi(s.hub, *req.ProductID)

	s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "stock_transaction",
		EntityID:   transaction.ID,
		UserID:     optionalUUID(createdBy),
		Metadata:   map[string]interface{}{"type": transaction.TransactionType, "entity_name": entityName, "quantity": req.Quantity},
	})

	result := stockTransactionToResponse(s.db, transaction)
	return &result, nil
}

func ListUnits() []UnitResponse {
	keys := make([]string, 0, len(ValidUnits))
	for k := range ValidUnits {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	responses := make([]UnitResponse, len(keys))
	for i, k := range keys {
		responses[i] = UnitResponse{ID: k, Name: ValidUnits[k]}
	}
	return responses
}

func supplierToResponse(s model.Supplier) SupplierResponse {
	return SupplierResponse{
		ID:        s.ID,
		Name:      s.Name,
		Phone:     s.Phone,
		Email:     s.Email,
		Address:   s.Address,
		IsActive:  s.IsActive,
		CreatedAt: s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (s *InventoryService) ListProductIngredients(productID string) ([]ProductIngredientResponse, error) {
	var pms []model.ProductIngredient
	if err := s.db.Where("product_id = ?", productID).Find(&pms).Error; err != nil {
		return nil, err
	}

	items := make([]ProductIngredientResponse, len(pms))
	for i, pm := range pms {
		items[i] = productIngredientToResponse(s.db, pm)
	}
	return items, nil
}

func (s *InventoryService) CreateProductIngredient(productID string, req *CreateProductIngredientRequest) (*ProductIngredientResponse, error) {
	var ingredient model.Product
	if err := s.db.Where("id = ?", req.IngredientID).First(&ingredient).Error; err != nil {
		return nil, errors.New("không tìm thấy nguyên liệu")
	}

	pm := model.ProductIngredient{
		ProductID:    productID,
		IngredientID: req.IngredientID,
		Quantity:     req.Quantity,
		UnitID:       *ingredient.UnitID,
	}
	if err := s.db.Create(&pm).Error; err != nil {
		return nil, err
	}

	result := productIngredientToResponse(s.db, pm)
	s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "product_ingredient",
		EntityID:   pm.ID,
		Metadata:   map[string]interface{}{"product_id": productID, "ingredient_name": ingredient.Name, "quantity": req.Quantity},
	})
	return &result, nil
}

func (s *InventoryService) UpdateProductIngredient(id string, req *UpdateProductIngredientRequest) (*ProductIngredientResponse, error) {
	var pm model.ProductIngredient
	if err := s.db.Where("id = ?", id).First(&pm).Error; err != nil {
		return nil, errors.New("không tìm thấy nguyên liệu sản phẩm")
	}

	updates := map[string]interface{}{}
	if req.Quantity != nil {
		updates["quantity"] = *req.Quantity
	}

	if len(updates) > 0 {
		if err := s.db.Model(&pm).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	s.db.First(&pm, "id = ?", id)
	result := productIngredientToResponse(s.db, pm)
	s.audit.Log(&LogAuditRequest{
		Action:     "update",
		EntityType: "product_ingredient",
		EntityID:   pm.ID,
		Metadata:   map[string]interface{}{"updates": updates},
	})
	return &result, nil
}

// DeleteProductIngredient gỡ một nguyên liệu khỏi công thức của sản phẩm.
//
// Dòng công thức là con SỞ HỮU của sản phẩm, không bảng nào tham chiếu tới nó.
// Không cần kiểm phụ thuộc.
func (s *InventoryService) DeleteProductIngredient(id string) error {
	var pm model.ProductIngredient
	if err := s.db.Where("id = ?", id).First(&pm).Error; err != nil {
		return errors.New("không tìm thấy nguyên liệu sản phẩm")
	}

	if err := s.db.Delete(&pm).Error; err != nil {
		return err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "product_ingredient",
		EntityID:   id,
		Metadata:   map[string]interface{}{"product_id": pm.ProductID},
	})
	return nil
}

// DeductItemsStock deducts multiple ingredient products at once within a transaction, grouped by ingredient.
// Each ingredient product gets one StockTransaction row. Used by OrderService.
func (s *InventoryService) DeductItemsStock(tx *gorm.DB, items []StockDeductionItem, productID string, referenceID string, orderCode string) error {
	deductions := map[string]float64{}
	for _, it := range items {
		deductions[it.IngredientID] += it.Quantity
	}

	// Iterate in a stable order. Map iteration order is random, which makes
	// concurrent orders touching the same ingredients lock rows in opposing
	// orders and deadlock.
	ingredientIDs := make([]string, 0, len(deductions))
	for id := range deductions {
		ingredientIDs = append(ingredientIDs, id)
	}
	sort.Strings(ingredientIDs)

	for _, ingredientID := range ingredientIDs {
		totalQty := deductions[ingredientID]
		var ingredient model.Product
		// Khoá dòng nguyên liệu, cùng lý do và cùng cách với dòng sản phẩm ở
		// computeDeductions. Thiếu chỗ này thì hàng bán thẳng chống được hai đơn
		// cùng lúc, còn món chế biến thì không — hai đơn đọc cùng một mốc rồi
		// ghi đè nhau, và cả hai đều đi qua phép kiểm "đủ nguyên liệu".
		//
		// Thứ tự khoá theo ingredientIDs đã sort ở trên nên không sinh chu trình.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", ingredientID).First(&ingredient).Error; err != nil {
			return fmt.Errorf("không tìm thấy nguyên liệu %s", ingredientID)
		}

		if !ingredient.HasStock {
			continue
		}

		stockAfter := ingredient.CurrentStock - totalQty
		if stockAfter < 0 {
			return fmt.Errorf("nguyên liệu %s không đủ tồn kho", ingredient.Name)
		}

		transaction := model.StockTransaction{
			ProductID:       &ingredientID,
			TransactionType: "outbound",
			Quantity:        totalQty,
			StockBefore:     ingredient.CurrentStock,
			StockAfter:      stockAfter,
			ReferenceID:     &referenceID,
			Description:     fmt.Sprintf("Xuất kho theo đơn %s", orderCode),
		}
		if err := tx.Create(&transaction).Error; err != nil {
			return err
		}

		if err := tx.Model(&ingredient).Update("current_stock", stockAfter).Error; err != nil {
			return err
		}

		s.audit.Log(&LogAuditRequest{
			Action:     "create",
			EntityType: "stock_transaction",
			EntityID:   transaction.ID,
			Metadata:   map[string]interface{}{"type": "outbound", "ingredient_name": ingredient.Name, "quantity": totalQty},
		})
	}

	return nil
}

// RestoreItemsStock reverses a stock deduction — creates inbound transactions and adds stock back.
// Used by OrderService when a confirmed order is cancelled.
func (s *InventoryService) RestoreItemsStock(tx *gorm.DB, items []StockDeductionItem, productID string, referenceID string, orderCode string) error {
	deductions := map[string]float64{}
	for _, it := range items {
		deductions[it.IngredientID] += it.Quantity
	}

	// Iterate in a stable order. Map iteration order is random, which makes
	// concurrent orders touching the same ingredients lock rows in opposing
	// orders and deadlock.
	ingredientIDs := make([]string, 0, len(deductions))
	for id := range deductions {
		ingredientIDs = append(ingredientIDs, id)
	}
	sort.Strings(ingredientIDs)

	for _, ingredientID := range ingredientIDs {
		totalQty := deductions[ingredientID]
		var ingredient model.Product
		// Khoá dòng nguyên liệu, cùng lý do và cùng cách với dòng sản phẩm ở
		// computeDeductions. Thiếu chỗ này thì hàng bán thẳng chống được hai đơn
		// cùng lúc, còn món chế biến thì không — hai đơn đọc cùng một mốc rồi
		// ghi đè nhau, và cả hai đều đi qua phép kiểm "đủ nguyên liệu".
		//
		// Thứ tự khoá theo ingredientIDs đã sort ở trên nên không sinh chu trình.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", ingredientID).First(&ingredient).Error; err != nil {
			return fmt.Errorf("không tìm thấy nguyên liệu %s", ingredientID)
		}

		if !ingredient.HasStock {
			continue
		}

		stockAfter := ingredient.CurrentStock + totalQty

		transaction := model.StockTransaction{
			ProductID:       &ingredientID,
			TransactionType: "inbound",
			Quantity:        totalQty,
			StockBefore:     ingredient.CurrentStock,
			StockAfter:      stockAfter,
			ReferenceID:     &referenceID,
			Description:     fmt.Sprintf("Hoàn kho theo đơn %s", orderCode),
		}
		if err := tx.Create(&transaction).Error; err != nil {
			return err
		}

		if err := tx.Model(&ingredient).Update("current_stock", stockAfter).Error; err != nil {
			return err
		}

		s.audit.Log(&LogAuditRequest{
			Action:     "create",
			EntityType: "stock_transaction",
			EntityID:   transaction.ID,
			Metadata:   map[string]interface{}{"type": "inbound", "ingredient_name": ingredient.Name, "quantity": totalQty},
		})
	}

	return nil
}

// DeductStock creates outbound stock transactions within a given transaction.
// Used by OrderService when an order is created (same tx as the order).
func (s *InventoryService) DeductStock(tx *gorm.DB, productID string, quantity int, referenceID string) error {
	var pms []model.ProductIngredient
	if err := tx.Where("product_id = ?", productID).Find(&pms).Error; err != nil {
		return err
	}
	if len(pms) == 0 {
		return nil
	}

	for _, pm := range pms {
		if err := s.deductSingleIngredient(tx, pm.IngredientID, pm.Quantity*float64(quantity), productID, referenceID); err != nil {
			return err
		}
	}

	return nil
}

func (s *InventoryService) deductSingleIngredient(tx *gorm.DB, ingredientID string, deductQty float64, productID string, referenceID string) error {
	var ingredient model.Product
	if err := tx.Where("id = ?", ingredientID).First(&ingredient).Error; err != nil {
		return fmt.Errorf("không tìm thấy nguyên liệu %s", ingredientID)
	}

	if !ingredient.HasStock {
		return nil
	}

	stockAfter := ingredient.CurrentStock - deductQty
	if stockAfter < 0 {
		return fmt.Errorf("nguyên liệu %s không đủ tồn kho", ingredient.Name)
	}

	transaction := model.StockTransaction{
		ProductID:       &ingredientID,
		TransactionType: "outbound",
		Quantity:        deductQty,
		StockBefore:     ingredient.CurrentStock,
		StockAfter:      stockAfter,
		ReferenceID:     &referenceID,
		Description:     fmt.Sprintf("Deducted by order %s", referenceID),
	}
	if err := tx.Create(&transaction).Error; err != nil {
		return err
	}

	if err := tx.Model(&ingredient).Update("current_stock", stockAfter).Error; err != nil {
		return err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "stock_transaction",
		EntityID:   transaction.ID,
		Metadata:   map[string]interface{}{"type": "outbound", "ingredient_name": ingredient.Name, "quantity": deductQty},
	})
	return nil
}

func productIngredientToResponse(db *gorm.DB, pm model.ProductIngredient) ProductIngredientResponse {
	ingredientName := ""
	unitName := ""
	var ingredient model.Product
	if err := db.Where("id = ?", pm.IngredientID).First(&ingredient).Error; err == nil {
		ingredientName = ingredient.Name
		if ingredient.UnitID != nil {
			if u, ok := ValidUnits[*ingredient.UnitID]; ok {
				unitName = u
			}
		}
	}

	return ProductIngredientResponse{
		ID:             pm.ID,
		ProductID:      pm.ProductID,
		IngredientID:   pm.IngredientID,
		IngredientName: ingredientName,
		UnitName:       unitName,
		UnitID:         pm.UnitID,
		Quantity:       pm.Quantity,
		CreatedAt:      pm.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func stockTransactionToResponse(db *gorm.DB, t model.StockTransaction) StockTransactionResponse {
	productName := ""
	if t.ProductID != nil {
		var product model.Product
		if err := db.Where("id = ?", *t.ProductID).First(&product).Error; err == nil {
			productName = product.Name
		}
	}

	supplierName := ""
	if t.SupplierID != nil {
		var supplier model.Supplier
		if err := db.Where("id = ?", *t.SupplierID).First(&supplier).Error; err == nil {
			supplierName = supplier.Name
		}
	}

	createdByName := ""
	if t.CreatedBy != nil {
		var user model.User
		if err := db.Where("id = ?", *t.CreatedBy).First(&user).Error; err == nil {
			createdByName = user.FullName
		}
	}

	return StockTransactionResponse{
		ID:              t.ID,
		ProductID:       t.ProductID,
		ProductName:     productName,
		TransactionType: t.TransactionType,
		Quantity:        t.Quantity,
		UnitPrice:       t.UnitPrice,
		TotalPrice:      t.TotalPrice,
		StockBefore:     t.StockBefore,
		StockAfter:      t.StockAfter,
		ReferenceID:     t.ReferenceID,
		SupplierID:      t.SupplierID,
		SupplierName:    supplierName,
		Description:     t.Description,
		CreatedBy:       t.CreatedBy,
		CreatedByName:   createdByName,
		CreatedAt:       t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
