package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ComboService struct {
	db     *gorm.DB
	audit  *AuditService
	hub    *hub.Hub
	orders *OrderService
}

func NewComboService(db *gorm.DB, audit *AuditService) *ComboService {
	return &ComboService{db: db, audit: audit}
}

// WithHub nối hub WebSocket vào để mua gói giờ báo số dư mới cho máy trạm
// ngay lúc nó đổi. Không nối thì mọi thứ vẫn chạy đúng, chỉ là màn hình khách
// giữ số cũ cho tới khi tự tải lại.
//
// Nối rời thay vì thêm tham số cho constructor: giữ nguyên chữ ký thì mọi test
// dựng service không phải sửa, và hub vẫn nil được trong test.
func (s *ComboService) WithHub(h *hub.Hub) *ComboService {
	s.hub = h
	return s
}

// WithOrders nối dịch vụ đơn hàng để món tặng kèm của gói ghi thành đơn 0đ và
// trừ kho bằng đúng đường của đơn bán. Bán gói có món tặng mà chưa nối thì hỏng.
func (s *ComboService) WithOrders(o *OrderService) *ComboService {
	s.orders = o
	return s
}

type ComboListRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Sort     string `form:"sort"`
	Order    string `form:"order"`
	Search   string `form:"search"`
}

type ComboResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Type         string `json:"type"`
	SlotStart    string `json:"slot_start"`
	SlotEnd      string `json:"slot_end"`
	ApplyDays    []int  `json:"apply_days"`
	TotalMinutes int    `json:"total_minutes"`
	ValidityDays int    `json:"validity_days"`
	Price        int64  `json:"price"`
	MemberPrefix string `json:"member_prefix"`
	MemberCount  int    `json:"member_count"`
	IsActive     bool   `json:"is_active"`
	CreatedAt    string `json:"created_at"`
	// Tên đọc cả sản phẩm đã xoá mềm, để gói vẫn hiện món nó đang trỏ tới.
	GiftProductID   string `json:"gift_product_id"`
	GiftProductName string `json:"gift_product_name"`
	GiftQuantity    int    `json:"gift_quantity"`
}

type CreateComboRequest struct {
	Name         string `json:"name" binding:"required"`
	Description  string `json:"description"`
	Type         string `json:"type" binding:"required,oneof=fixed_slot prepaid"`
	SlotStart    string `json:"slot_start"`
	SlotEnd      string `json:"slot_end"`
	ApplyDays    []int  `json:"apply_days"`
	TotalMinutes int    `json:"total_minutes"`
	ValidityDays int    `json:"validity_days"`
	// required và min=0 mâu thuẫn nhau; gói miễn phí là hợp lệ.
	Price int64 `json:"price" binding:"min=0"`
	// Con trỏ: không gửi = bật như trước nay; gửi false = tạo ở trạng thái tắt.
	IsActive *bool `json:"is_active"`
	// Món tặng kèm mặc định; bỏ trống = không tặng. Số lượng dưới 1 tính là 1.
	GiftProductID string `json:"gift_product_id"`
	GiftQuantity  int    `json:"gift_quantity"`
}

// Description/ValidityDays/Price là con trỏ: với kiểu thường, "" và 0 trùng với
// "không gửi" nên không xoá được mô tả, không đặt được giá 0 (gói miễn phí) hay
// hiệu lực 0 ngày dù lúc tạo thì cho phép.
type UpdateComboRequest struct {
	Name         string  `json:"name"`
	Description  *string `json:"description"`
	Type         string  `json:"type" binding:"omitempty,oneof=fixed_slot prepaid"`
	SlotStart    string  `json:"slot_start"`
	SlotEnd      string  `json:"slot_end"`
	ApplyDays    []int   `json:"apply_days"`
	TotalMinutes int     `json:"total_minutes"`
	ValidityDays *int    `json:"validity_days" binding:"omitempty,min=0"`
	Price        *int64  `json:"price" binding:"omitempty,min=0"`
	IsActive     *bool   `json:"is_active"`
	// Không gửi = giữ món tặng; "" = bỏ tặng. GiftQuantity chỉ đọc khi có
	// GiftProductID, không gửi thì giữ số lượng cũ.
	GiftProductID *string `json:"gift_product_id"`
	GiftQuantity  *int    `json:"gift_quantity"`
}

type PurchaseComboRequest struct {
	MemberID      string `json:"member_id"`
	CustomerName  string `json:"customer_name"`
	CustomerPhone string `json:"customer_phone"`
	PaymentMethod string `json:"payment_method" binding:"required"`
	// IdempotencyKey do phía gọi sinh ra. Gửi lại cùng một khoá nghĩa là cùng
	// MỘT ý định, không phải hai lần thao tác. Bỏ trống là không tham gia.
	IdempotencyKey string `json:"idempotency_key"`
	// Món tặng nhân viên chọn lúc bán. Không gửi = món mặc định của gói; "" =
	// khách không lấy. Số lượng luôn theo gói.
	GiftProductID *string `json:"gift_product_id"`
}

type ActivateComboRequest struct {
	MachineID string `json:"machine_id" binding:"required"`
}

// optionalClock turns a "HH:MM:SS" form value into a nullable column value.
// An empty string is not a valid SQL time, so a combo without a fixed slot has
// to store NULL rather than "".
func optionalClock(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func clockValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func (s *ComboService) List(req *ComboListRequest) (*pagination.Result, error) {
	p := &pagination.Params{
		Page:     req.Page,
		PageSize: req.PageSize,
		Sort:     req.Sort,
		Order:    req.Order,
		Search:   req.Search,
	}
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = 20
	}
	if p.Sort == "" {
		p.Sort = "created_at"
	}
	if p.Order == "" {
		p.Order = "desc"
	}

	var combos []model.Combo
	query := s.db
	if p.Search != "" {
		// Bỏ dấu khi so khớp: nhân viên gõ "goi gio vang" phải ra "Gói giờ vàng".
		// Cùng khuôn với ô tìm ở trang Hội viên, Nhóm máy và Giao dịch.
		query = query.Where("unaccent(name) ILIKE unaccent(?)", "%"+p.Search+"%")
	}

	var total int64
	query.Model(&model.Combo{}).Count(&total)

	if err := pagination.Apply(query, p).Find(&combos).Error; err != nil {
		return nil, err
	}

	items := make([]ComboResponse, len(combos))
	for i, c := range combos {
		items[i] = comboToResponse(c)
	}
	ganTenMonTang(s.db, items)

	return pagination.NewResult(items, total, p), nil
}

func (s *ComboService) GetByID(id string) (*ComboResponse, error) {
	var combo model.Combo
	if err := s.db.Where("id = ?", id).First(&combo).Error; err != nil {
		return nil, err
	}
	return s.comboResponse(combo), nil
}

// comboResponse dựng phản hồi cho MỘT gói, kèm tên món tặng.
func (s *ComboService) comboResponse(c model.Combo) *ComboResponse {
	items := []ComboResponse{comboToResponse(c)}
	ganTenMonTang(s.db, items)
	return &items[0]
}

// ganTenMonTang điền tên món tặng cho cả trang bằng một câu truy vấn. Unscoped:
// món đã xoá mềm vẫn hiện tên, giống mọi chỗ trang trí bản ghi cũ.
func ganTenMonTang(db *gorm.DB, items []ComboResponse) {
	var ids []string
	for _, it := range items {
		if it.GiftProductID != "" {
			ids = append(ids, it.GiftProductID)
		}
	}
	if len(ids) == 0 {
		return
	}
	var products []model.Product
	db.Unscoped().Select("id", "name").Where("id IN ?", ids).Find(&products)
	ten := make(map[string]string, len(products))
	for _, p := range products {
		ten[p.ID] = p.Name
	}
	for i := range items {
		items[i].GiftProductName = ten[items[i].GiftProductID]
	}
}

// timMonTang đọc một món dùng làm quà tặng kèm. Chỉ hàng bán lẻ đang bán: cùng
// điều kiện với món trong đơn bán, vì món tặng cũng thành một dòng đơn.
func timMonTang(db *gorm.DB, id string) (*model.Product, error) {
	var p model.Product
	if err := db.Where("id = ?", id).First(&p).Error; err != nil {
		return nil, errors.New("không tìm thấy món tặng kèm")
	}
	if !p.IsRetail || !p.IsActive {
		return nil, fmt.Errorf("món %s đang ngưng bán, hãy chọn món tặng khác", p.Name)
	}
	return &p, nil
}

func (s *ComboService) Create(req *CreateComboRequest) (*ComboResponse, error) {
	if req.Type == "fixed_slot" && (req.SlotStart == "" || req.SlotEnd == "") {
		return nil, errors.New("slot_start and slot_end are required for fixed_slot type")
	}
	if req.Type == "prepaid" && req.TotalMinutes <= 0 {
		return nil, errors.New("total_minutes must be > 0 for prepaid type")
	}

	prefix := generateMemberPrefix(req.Name)

	combo := model.Combo{
		Name:         req.Name,
		Description:  req.Description,
		Type:         req.Type,
		SlotStart:    optionalClock(req.SlotStart),
		SlotEnd:      optionalClock(req.SlotEnd),
		ApplyDays:    req.ApplyDays,
		TotalMinutes: req.TotalMinutes,
		ValidityDays: req.ValidityDays,
		Price:        req.Price,
		MemberPrefix: prefix,
		MemberCount:  0,
		IsActive:     true,
	}
	if req.GiftProductID != "" {
		mon, err := timMonTang(s.db, req.GiftProductID)
		if err != nil {
			return nil, err
		}
		combo.GiftProductID = &mon.ID
		combo.GiftQuantity = max(req.GiftQuantity, 1)
	}

	// is_active có `default:true`, GORM thay false bằng default khi INSERT —
	// ghi lại false trong cùng giao dịch (cùng cách với promotion.go).
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&combo).Error; err != nil {
			return err
		}
		if req.IsActive != nil && !*req.IsActive {
			if err := tx.Model(&combo).Update("is_active", false).Error; err != nil {
				return err
			}
			combo.IsActive = false
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "combo",
		EntityID:   combo.ID,
		Metadata: map[string]interface{}{
			"name":  combo.Name,
			"type":  combo.Type,
			"price": combo.Price,
		},
	})

	return s.comboResponse(combo), nil
}

func (s *ComboService) Update(id string, req *UpdateComboRequest) (*ComboResponse, error) {
	var combo model.Combo
	if err := s.db.Where("id = ?", id).First(&combo).Error; err != nil {
		return nil, err
	}

	if req.Type == "fixed_slot" && req.SlotStart == "" && req.SlotEnd == "" && (clockValue(combo.SlotStart) == "" || clockValue(combo.SlotEnd) == "") {
		return nil, errors.New("slot_start and slot_end are required for fixed_slot type")
	}
	if req.Type == "prepaid" && req.TotalMinutes <= 0 && combo.TotalMinutes <= 0 {
		return nil, errors.New("total_minutes must be > 0 for prepaid type")
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
		updates["member_prefix"] = generateMemberPrefix(req.Name)
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Type != "" {
		updates["type"] = req.Type
	}
	if req.SlotStart != "" {
		updates["slot_start"] = req.SlotStart
	}
	if req.SlotEnd != "" {
		updates["slot_end"] = req.SlotEnd
	}
	if req.ApplyDays != nil {
		updates["apply_days"] = req.ApplyDays
	}
	if req.TotalMinutes > 0 {
		updates["total_minutes"] = req.TotalMinutes
	}
	if req.ValidityDays != nil {
		updates["validity_days"] = *req.ValidityDays
	}
	if req.Price != nil {
		updates["price"] = *req.Price
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.GiftProductID != nil {
		if *req.GiftProductID == "" {
			updates["gift_product_id"] = nil
			updates["gift_quantity"] = 0
		} else {
			mon, err := timMonTang(s.db, *req.GiftProductID)
			if err != nil {
				return nil, err
			}
			soLuong := combo.GiftQuantity
			if req.GiftQuantity != nil {
				soLuong = *req.GiftQuantity
			}
			updates["gift_product_id"] = mon.ID
			updates["gift_quantity"] = max(soLuong, 1)
		}
	}

	if err := s.db.Model(&combo).Updates(updates).Error; err != nil {
		return nil, err
	}

	s.db.First(&combo, "id = ?", id)

	s.audit.Log(&LogAuditRequest{
		Action:     "update",
		EntityType: "combo",
		EntityID:   id,
		Metadata:   updates,
	})

	return s.comboResponse(combo), nil
}

// Delete xoá một gói combo.
//
// Lượt mua là chứng từ tiền và có thể còn phút chưa dùng; phiên chơi đã mở bằng
// combo này là lịch sử. Cả hai phải giữ, nên combo đã bán thì chỉ tắt được.
func (s *ComboService) Delete(id string) error {
	var combo model.Combo
	if err := s.db.Where("id = ?", id).First(&combo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("không tìm thấy combo")
		}
		return err
	}
	if err := kiemTraPhuThuoc(s.db, id, []phuThuoc{
		{Bang: &model.ComboPurchase{}, Cot: "combo_id", Nhan: "lượt mua combo"},
		{Bang: &model.MachineSession{}, Cot: "combo_id", Nhan: "phiên chơi dùng combo này"},
	}, "hãy tắt combo (bỏ đang bán) thay vì xoá"); err != nil {
		return err
	}
	now := time.Now()
	if err := s.db.Model(&combo).Update("deleted_at", &now).Error; err != nil {
		return err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "delete",
		EntityType: "combo",
		EntityID:   id,
		Metadata: map[string]interface{}{
			"name": combo.Name,
		},
	})

	return nil
}

func (s *ComboService) Purchase(comboID string, req *PurchaseComboRequest, userID string) (*ComboPurchaseResponse, error) {
	var combo model.Combo
	if err := s.db.Where("id = ? AND is_active = ?", comboID, true).First(&combo).Error; err != nil {
		return nil, errors.New("không tìm thấy gói cước hoặc gói đã ngừng bán")
	}
	if req.PaymentMethod != PaymentMethodBalance {
		if err := kiemTraPhuongThucThanhToan(s.db, req.PaymentMethod); err != nil {
			return nil, err
		}
	}

	// Món tặng: món nhân viên chọn lúc bán, không chọn thì món mặc định của gói.
	var monTang *model.OrderItem
	if combo.GiftQuantity > 0 {
		monID := ""
		if combo.GiftProductID != nil {
			monID = *combo.GiftProductID
		}
		if req.GiftProductID != nil {
			monID = *req.GiftProductID
		}
		if monID != "" {
			mon, err := timMonTang(s.db, monID)
			if err != nil {
				return nil, err
			}
			monTang = &model.OrderItem{
				ProductID:   mon.ID,
				ProductName: mon.Name,
				Quantity:    combo.GiftQuantity,
				Options:     "null",
				// Món đưa tận tay ở quầy lúc bán gói, không qua bếp.
				Status: "served",
			}
			// Kiểm tồn kho TRƯỚC khi tạo tài khoản khách: hết hàng thì dừng ở
			// đây, không để lại một tài khoản trống không có gói. Phép kiểm thật
			// vẫn chạy lại dưới khoá trong giao dịch bán.
			if _, _, err := s.orders.computeDeductions(nil, []model.OrderItem{*monTang}, stockOpDeduct); err != nil {
				return nil, err
			}
		}
	}

	memberID := req.MemberID
	var generatedPassword string

	if memberID == "" {
		// Lấy theo tên gói HIỆN TẠI chứ không theo member_prefix đã lưu: gói tạo
		// trước khi đổi quy tắc vẫn giữ tiền tố kiểu cũ ("GAMIN") trong cột đó.
		prefix := generateMemberPrefix(combo.Name)

		tx := s.db.Begin()

		var lockedCombo model.Combo
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedCombo, "id = ?", comboID).Error; err != nil {
			tx.Rollback()
			return nil, errors.New("không tìm thấy gói cước")
		}

		// Bỏ qua số đã có người dùng: hai gói khác tên vẫn có thể ra cùng tiền
		// tố ("Gói 1" số 1 và "Gói" số 11 đều là "Goi11"), và hội viên đã xoá
		// mềm vẫn giữ tên trong chỉ mục unique.
		newCount := lockedCombo.MemberCount
		var memberCode string
		for {
			newCount++
			memberCode = fmt.Sprintf("%s%d", prefix, newCount)
			var taken int64
			if err := tx.Unscoped().Model(&model.Member{}).Where("username = ?", memberCode).Count(&taken).Error; err != nil {
				tx.Rollback()
				return nil, err
			}
			if taken == 0 {
				break
			}
		}

		randomPass, err := sixDigitPassword()
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		generatedPassword = randomPass
		passHash, _ := utils.HashPassword(randomPass)

		member := model.Member{
			Username:     memberCode,
			PasswordHash: passHash,
			FullName:     req.CustomerName,
			Phone:        req.CustomerPhone,
			// Tài khoản sinh ra theo gói: trang Hội viên lọc riêng nhóm này, và
			// máy trạm hiện nhãn "Combo" theo role.
			Role:     MemberRoleCombo,
			IsActive: true,
		}
		if err := tx.Create(&member).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		memberID = member.ID

		if err := tx.Model(&lockedCombo).Update("member_count", newCount).Error; err != nil {
			tx.Rollback()
			return nil, err
		}

		if err := tx.Commit().Error; err != nil {
			return nil, err
		}

		combo.MemberCount = newCount
	}

	var expiresAt *time.Time
	if combo.ValidityDays > 0 {
		t := time.Now().AddDate(0, 0, combo.ValidityDays)
		expiresAt = &t
	}

	purchase := model.ComboPurchase{
		ComboID:          comboID,
		MemberID:         memberID,
		Price:            combo.Price,
		PaymentMethod:    req.PaymentMethod,
		Activated:        false,
		RemainingMinutes: combo.TotalMinutes,
		ExpiresAt:        expiresAt,
	}

	// Purchase and payment are one unit of work: a combo must never exist
	// without the matching money movement, and an unpayable purchase must not
	// be created at all.
	payTx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			payTx.Rollback()
			panic(r)
		}
	}()

	if err := giuKhoaIdempotency(payTx, "combo.purchase", req.IdempotencyKey); err != nil {
		payTx.Rollback()
		return nil, err
	}

	var member model.Member
	if err := payTx.Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&member, "id = ?", memberID).Error; err != nil {
		payTx.Rollback()
		return nil, errors.New("không tìm thấy hội viên")
	}

	balanceAfter := member.Balance
	if req.PaymentMethod == PaymentMethodBalance {
		if member.Balance < combo.Price {
			payTx.Rollback()
			return nil, errors.New("số dư không đủ để mua gói cước này")
		}
		balanceAfter = member.Balance - combo.Price
	}

	if err := payTx.Create(&purchase).Error; err != nil {
		payTx.Rollback()
		return nil, err
	}

	trans := model.MemberTransaction{
		MemberID:        memberID,
		TransactionType: "combo_purchase",
		Amount:          -combo.Price,
		BalanceBefore:   member.Balance,
		BalanceAfter:    balanceAfter,
		PaymentMethod:   req.PaymentMethod,
		ReferenceID:     &purchase.ID,
		Description:     fmt.Sprintf("Purchase combo: %s", combo.Name),
		CreatedAt:       time.Now(),
	}
	if userID != "" {
		trans.CreatedBy = &userID
	}
	if err := payTx.Create(&trans).Error; err != nil {
		payTx.Rollback()
		return nil, err
	}

	updates := map[string]interface{}{
		"total_spent": member.TotalSpent + combo.Price,
	}
	if req.PaymentMethod == PaymentMethodBalance {
		updates["balance"] = balanceAfter
	}
	if err := payTx.Model(&member).Updates(updates).Error; err != nil {
		payTx.Rollback()
		return nil, err
	}

	// Món tặng ghi trong cùng giao dịch: thiếu hàng thì không bán gói, không
	// có chuyện thu tiền gói rồi mới phát hiện không có nước để tặng.
	var donTang *model.Order
	var khoDaDoi []string
	if monTang != nil {
		var err error
		donTang, khoDaDoi, err = s.orders.taoDonTangKem(payTx, memberID, *monTang, req.PaymentMethod,
			"Tặng kèm combo "+combo.Name, userID)
		if err != nil {
			payTx.Rollback()
			return nil, err
		}
	}

	if err := payTx.Commit().Error; err != nil {
		return nil, err
	}

	// Chỉ khi trả bằng số dư mới có gì để báo; trả tiền mặt thì số dư đứng yên.
	if req.PaymentMethod == PaymentMethodBalance {
		phatSoDuMoi(s.hub, memberID, balanceAfter, member.BonusBalance)
	}
	if donTang != nil {
		phatTonKhoDoi(s.hub, khoDaDoi...)
		// Đơn sinh từ route /combos nên middleware chỉ báo "combos"; trang Đơn
		// hàng phải được báo riêng.
		s.hub.Changed("orders", nil)
	}

	auditMetadata := map[string]interface{}{
		"combo_name":     combo.Name,
		"amount":         combo.Price,
		"payment_method": req.PaymentMethod,
		"member_id":      memberID,
	}
	if req.MemberID == "" {
		auditMetadata["new_member"] = true
	}
	if donTang != nil {
		auditMetadata["gift_product"] = monTang.ProductName
		auditMetadata["gift_quantity"] = monTang.Quantity
		auditMetadata["gift_order_code"] = donTang.OrderCode
	}
	var uid *string
	if userID != "" {
		uid = &userID
	}
	s.audit.Log(&LogAuditRequest{
		Action:     "purchase",
		EntityType: "combo",
		EntityID:   purchase.ID,
		UserID:     uid,
		Metadata:   auditMetadata,
	})

	result := purchaseToResponseWithPassword(purchase, combo.Name, generatedPassword)
	result.MemberUsername = member.Username
	if donTang != nil {
		result.GiftProductName = monTang.ProductName
		result.GiftQuantity = monTang.Quantity
		result.GiftOrderCode = donTang.OrderCode
	}
	return &result, nil
}

func (s *ComboService) Activate(purchaseID string, req *ActivateComboRequest) (*ComboPurchaseResponse, error) {
	var purchase model.ComboPurchase
	if err := s.db.Where("id = ?", purchaseID).First(&purchase).Error; err != nil {
		return nil, errors.New("không tìm thấy lượt mua")
	}

	if purchase.Activated {
		return nil, errors.New("lượt mua này đã được kích hoạt")
	}

	var combo model.Combo
	if err := s.db.First(&combo, "id = ?", purchase.ComboID).Error; err != nil {
		return nil, errors.New("không tìm thấy gói cước")
	}

	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	// Kiểm tra purchase.Activated ở trên chạy trên bản đọc ngoài transaction. Hai
	// lần kích hoạt song song đều thấy Activated=false và đều mở phiên chơi cho
	// cùng một gói combo. Khoá rồi đọc lại là chỗ duy nhất chặn được.
	var lockedPurchase model.ComboPurchase
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", purchaseID).First(&lockedPurchase).Error; err != nil {
		tx.Rollback()
		return nil, errors.New("không tìm thấy lượt mua")
	}
	if lockedPurchase.Activated {
		tx.Rollback()
		return nil, errors.New("lượt mua này đã được kích hoạt")
	}
	purchase = lockedPurchase

	var machine model.Machine
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND is_active = ?", req.MachineID, true).First(&machine).Error; err != nil {
		tx.Rollback()
		return nil, errors.New("không tìm thấy máy")
	}

	if machine.Status == "in_use" {
		tx.Rollback()
		return nil, errors.New("máy đang có người dùng")
	}

	now := time.Now()
	activatedAt := now
	var slotEnd *time.Time
	var remainingMinutes *int

	if slotClock := clockValue(combo.SlotEnd); slotClock != "" {
		parts := strings.Split(slotClock, ":")
		if len(parts) >= 2 {
			h, m := 0, 0
			fmt.Sscanf(parts[0], "%d", &h)
			fmt.Sscanf(parts[1], "%d", &m)
			endTime := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
			if endTime.Before(now) {
				endTime = endTime.Add(24 * time.Hour)
			}
			slotEnd = &endTime
		}
	}

	rm := combo.TotalMinutes
	remainingMinutes = &rm

	session := model.MachineSession{
		MachineID:        req.MachineID,
		MemberID:         &purchase.MemberID,
		ComboType:        combo.Type,
		ComboID:          &purchase.ID,
		SlotEnd:          slotEnd,
		RemainingMinutes: remainingMinutes,
		StartedAt:        now,
		IsActive:         true,
	}

	if err := tx.Create(&session).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	machine.Status = "in_use"
	if err := tx.Save(&machine).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	updates := map[string]interface{}{
		"activated":          true,
		"activated_at":       activatedAt,
		"current_session_id": session.ID,
	}
	if err := tx.Model(&purchase).Updates(updates).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	purchase.Activated = true
	purchase.ActivatedAt = &activatedAt
	purchase.CurrentSessionID = &session.ID

	// Kích hoạt combo mở phiên và giữ máy mà không qua SessionService.StartSession,
	// nên tự báo trang quản trị như nó: trang Phiên chơi / Máy trạm / Bảng điều
	// khiển nghe "sessions" và machine:status.
	s.hub.Changed("sessions", map[string]interface{}{"id": session.ID, "reason": "started"})
	s.hub.BroadcastToType(hub.Event{
		Type: "machine:status",
		Data: map[string]interface{}{
			"machine_id":   machine.ID,
			"machine_code": machine.MachineCode,
			"status":       "in_use",
		},
	}, hub.ClientTypeAdmin)

	s.audit.Log(&LogAuditRequest{
		Action:     "activate",
		EntityType: "combo_purchase",
		EntityID:   purchaseID,
		Metadata: map[string]interface{}{
			"combo_id":   combo.ID,
			"combo_name": combo.Name,
			"machine_id": req.MachineID,
			"session_id": session.ID,
		},
	})

	result := purchaseToResponse(purchase, combo.Name)
	return &result, nil
}

func comboToResponse(c model.Combo) ComboResponse {
	r := ComboResponse{
		ID:           c.ID,
		Name:         c.Name,
		Description:  c.Description,
		Type:         c.Type,
		SlotStart:    clockValue(c.SlotStart),
		SlotEnd:      clockValue(c.SlotEnd),
		ApplyDays:    c.ApplyDays,
		TotalMinutes: c.TotalMinutes,
		ValidityDays: c.ValidityDays,
		Price:        c.Price,
		MemberPrefix: c.MemberPrefix,
		MemberCount:  c.MemberCount,
		IsActive:     c.IsActive,
		CreatedAt:    c.CreatedAt.Format(time.RFC3339),
		GiftQuantity: c.GiftQuantity,
	}
	if c.GiftProductID != nil {
		r.GiftProductID = *c.GiftProductID
	}
	return r
}

type ComboPurchaseResponse struct {
	ID                string     `json:"id"`
	ComboID           string     `json:"combo_id"`
	ComboName         string     `json:"combo_name"`
	MemberID          string     `json:"member_id"`
	Price             int64      `json:"price"`
	PaymentMethod     string     `json:"payment_method"`
	Activated         bool       `json:"activated"`
	ActivatedAt       *time.Time `json:"activated_at"`
	CurrentSessionID  *string    `json:"current_session_id"`
	RemainingMinutes  int        `json:"remaining_minutes"`
	ExpiresAt         *time.Time `json:"expires_at"`
	CreatedAt         time.Time  `json:"created_at"`
	GeneratedPassword string     `json:"generated_password,omitempty"`
	// MemberUsername chỉ có khi vừa mua: mã tài khoản khách để quầy tra lại gói
	// trong "Combo đã mua" lúc kích hoạt.
	MemberUsername string `json:"member_username,omitempty"`
	// Món tặng kèm của lượt bán và mã đơn 0đ ghi nó; chỉ có khi vừa mua.
	GiftProductName string `json:"gift_product_name,omitempty"`
	GiftQuantity    int    `json:"gift_quantity,omitempty"`
	GiftOrderCode   string `json:"gift_order_code,omitempty"`
}

func purchaseToResponse(p model.ComboPurchase, comboName string) ComboPurchaseResponse {
	return ComboPurchaseResponse{
		ID:               p.ID,
		ComboID:          p.ComboID,
		ComboName:        comboName,
		MemberID:         p.MemberID,
		Price:            p.Price,
		PaymentMethod:    p.PaymentMethod,
		Activated:        p.Activated,
		ActivatedAt:      p.ActivatedAt,
		CurrentSessionID: p.CurrentSessionID,
		RemainingMinutes: p.RemainingMinutes,
		ExpiresAt:        p.ExpiresAt,
		CreatedAt:        p.CreatedAt,
	}
}

func purchaseToResponseWithPassword(p model.ComboPurchase, comboName, password string) ComboPurchaseResponse {
	r := purchaseToResponse(p, comboName)
	if password != "" {
		r.GeneratedPassword = password
	}
	return r
}

// generateMemberPrefix: tên gói viết liền, bỏ dấu, giữ hoa thường —
// "Gói Đêm 5h" -> "GoiDem5h". Tài khoản khách là tiền tố + số thứ tự.
func generateMemberPrefix(name string) string {
	var b strings.Builder
	for _, r := range foldVietnamese(name) {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	prefix := b.String()
	if prefix == "" {
		return "Goi"
	}
	// Vừa cột member_prefix varchar(20).
	if len(prefix) > 20 {
		prefix = prefix[:20]
	}
	return prefix
}

// sixDigitPassword: mật khẩu 6 chữ số để khách gõ được ngay tại máy.
func sixDigitPassword() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
