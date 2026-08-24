package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Kiểm kê kho.
//
// Một phiên kiểm kê: mở phiên → đếm từng mặt hàng → xem lại → chốt. Chốt xong
// mới sinh phiếu điều chỉnh và sửa tồn kho.
//
// Quyết định quan trọng nhất nằm ở lúc chốt: **áp CHÊNH LỆCH, không đặt tồn kho
// bằng số đã đếm**. Giữa lúc đếm và lúc chốt, quán vẫn bán hàng. Đếm được 10
// trong khi sổ ghi 12 nghĩa là hụt 2; nếu sau đó bán thêm 1 (sổ còn 11) mà chốt
// bằng cách đặt tồn = 10 thì lần bán đó bị xoá mất. Áp chênh lệch −2 cho ra 9 —
// đúng: 10 đếm được trừ 1 đã bán.

type InventoryCountService struct {
	db    *gorm.DB
	audit *AuditService
}

func NewInventoryCountService(db *gorm.DB, audit *AuditService) *InventoryCountService {
	return &InventoryCountService{db: db, audit: audit}
}

const (
	CountStatusOpen      = "open"
	CountStatusCommitted = "committed"
	CountStatusCancelled = "cancelled"
)

// countCodeLockKey serialise việc sinh mã phiên, cùng lý do với mã đơn hàng.
const countCodeLockKey = 4713002

type OpenCountRequest struct {
	Note string `json:"note"`
}

type CountLineRequest struct {
	ProductID string  `json:"product_id" binding:"required"`
	ActualQty float64 `json:"actual_qty" binding:"min=0"`
	Note      string  `json:"note"`
}

type CountLineResponse struct {
	ID            string  `json:"id"`
	ProductID     string  `json:"product_id"`
	ProductName   string  `json:"product_name"`
	ExpectedQty   float64 `json:"expected_qty"`
	ActualQty     float64 `json:"actual_qty"`
	DifferenceQty float64 `json:"difference_qty"`
	CountedAt     string  `json:"counted_at"`
}

type CountSessionResponse struct {
	model.InventoryCountSession
	Lines      []CountLineResponse `json:"lines,omitempty"`
	LineCount  int                 `json:"line_count"`
	DiffCount  int                 `json:"diff_count"`
	TotalShort float64             `json:"total_short"`
	TotalOver  float64             `json:"total_over"`
}

func (s *InventoryCountService) Open(req *OpenCountRequest, actorID string) (*model.InventoryCountSession, error) {
	session := model.InventoryCountSession{
		Note:   req.Note,
		Status: CountStatusOpen,
	}
	if actorID != "" {
		session.OpenedBy = &actorID
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		tx.Exec("SELECT pg_advisory_xact_lock(?)", countCodeLockKey)
		var last model.InventoryCountSession
		next := 1
		if err := tx.Where("code LIKE ?", "KK-%").Order("code DESC").First(&last).Error; err == nil {
			var n int
			if _, err := fmt.Sscanf(last.Code, "KK-%d", &n); err == nil {
				next = n + 1
			}
		}
		session.Code = fmt.Sprintf("KK-%05d", next)
		return tx.Create(&session).Error
	})
	if err != nil {
		return nil, err
	}

	s.log("open_inventory_count", session.ID, actorID, map[string]interface{}{"code": session.Code})
	return &session, nil
}

// SetLine ghi số đếm thực của một mặt hàng. Gọi lại cùng sản phẩm thì ghi đè —
// đếm lại là chuyện bình thường, không nên tạo hai dòng mâu thuẫn.
func (s *InventoryCountService) SetLine(sessionID string, req *CountLineRequest, actorID string) (*CountLineResponse, error) {
	if req.ActualQty < 0 {
		return nil, errors.New("số đếm không được âm")
	}

	var out CountLineResponse
	err := s.db.Transaction(func(tx *gorm.DB) error {
		session, err := s.loadOpenSession(tx, sessionID)
		if err != nil {
			return err
		}

		var product model.Product
		if err := tx.First(&product, "id = ?", req.ProductID).Error; err != nil {
			return errors.New("không tìm thấy sản phẩm")
		}
		if !product.HasStock {
			return fmt.Errorf("sản phẩm %s không theo dõi tồn kho", product.Name)
		}

		line := model.InventoryCount{
			SessionID:     session.ID,
			ProductID:     product.ID,
			ExpectedQty:   product.CurrentStock,
			ActualQty:     req.ActualQty,
			DifferenceQty: req.ActualQty - product.CurrentStock,
			CountedAt:     time.Now(),
		}
		if actorID != "" {
			line.CountedBy = &actorID
		}

		// Đếm lại thì ghi đè dòng cũ.
		if err := tx.Where("session_id = ? AND product_id = ?", session.ID, product.ID).
			Delete(&model.InventoryCount{}).Error; err != nil {
			return err
		}
		if err := tx.Create(&line).Error; err != nil {
			return err
		}

		out = CountLineResponse{
			ID: line.ID, ProductID: product.ID, ProductName: product.Name,
			ExpectedQty: line.ExpectedQty, ActualQty: line.ActualQty,
			DifferenceQty: line.DifferenceQty,
			CountedAt:     line.CountedAt.Format(time.RFC3339),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *InventoryCountService) RemoveLine(sessionID, productID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if _, err := s.loadOpenSession(tx, sessionID); err != nil {
			return err
		}
		return tx.Where("session_id = ? AND product_id = ?", sessionID, productID).
			Delete(&model.InventoryCount{}).Error
	})
}

// Commit chốt phiên: sinh phiếu điều chỉnh cho từng dòng lệch và sửa tồn kho.
func (s *InventoryCountService) Commit(sessionID, actorID string) (*CountSessionResponse, error) {
	var adjusted int
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Khoá phiên: hai người bấm chốt cùng lúc sẽ điều chỉnh kho hai lần.
		var session model.InventoryCountSession
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&session, "id = ?", sessionID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("không tìm thấy phiên kiểm kê")
			}
			return err
		}
		if session.Status != CountStatusOpen {
			return fmt.Errorf("phiên đã ở trạng thái %s", session.Status)
		}

		var lines []model.InventoryCount
		if err := tx.Where("session_id = ?", sessionID).Find(&lines).Error; err != nil {
			return err
		}
		if len(lines) == 0 {
			return errors.New("phiên chưa có dòng đếm nào")
		}

		for _, line := range lines {
			if line.DifferenceQty == 0 {
				continue
			}

			var product model.Product
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				First(&product, "id = ?", line.ProductID).Error; err != nil {
				return fmt.Errorf("không khoá được sản phẩm %s: %w", line.ProductID, err)
			}

			// Áp CHÊNH LỆCH chứ không đặt tồn = số đã đếm. Xem ghi chú đầu file.
			after := product.CurrentStock + line.DifferenceQty
			if after < 0 {
				after = 0
			}

			trans := model.StockTransaction{
				ProductID:       &product.ID,
				TransactionType: "adjustment",
				Quantity:        line.DifferenceQty,
				StockBefore:     product.CurrentStock,
				StockAfter:      after,
				ReferenceID:     &session.ID,
				Description: fmt.Sprintf("Kiểm kê %s: đếm %.3f, sổ %.3f",
					session.Code, line.ActualQty, line.ExpectedQty),
			}
			if actorID != "" {
				trans.CreatedBy = &actorID
			}
			if err := tx.Create(&trans).Error; err != nil {
				return err
			}
			if err := tx.Model(&product).Update("current_stock", after).Error; err != nil {
				return err
			}
			adjusted++
		}

		now := time.Now()
		updates := map[string]interface{}{
			"status": CountStatusCommitted, "committed_at": &now,
		}
		if actorID != "" {
			updates["committed_by"] = actorID
		}
		return tx.Model(&session).Updates(updates).Error
	})
	if err != nil {
		return nil, err
	}

	s.log("commit_inventory_count", sessionID, actorID, map[string]interface{}{
		"adjusted_products": adjusted,
	})
	return s.GetByID(sessionID)
}

func (s *InventoryCountService) Cancel(sessionID, actorID string) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		session, err := s.loadOpenSession(tx, sessionID)
		if err != nil {
			return err
		}
		return tx.Model(session).Update("status", CountStatusCancelled).Error
	})
	if err != nil {
		return err
	}
	s.log("cancel_inventory_count", sessionID, actorID, nil)
	return nil
}

func (s *InventoryCountService) GetByID(id string) (*CountSessionResponse, error) {
	var session model.InventoryCountSession
	if err := s.db.First(&session, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy phiên kiểm kê")
		}
		return nil, err
	}

	var lines []model.InventoryCount
	s.db.Where("session_id = ?", id).Order("counted_at asc").Find(&lines)

	names := map[string]string{}
	if len(lines) > 0 {
		ids := make([]string, 0, len(lines))
		for _, l := range lines {
			ids = append(ids, l.ProductID)
		}
		var products []struct {
			ID   string
			Name string
		}
		s.db.Model(&model.Product{}).Where("id IN ?", ids).Find(&products)
		for _, p := range products {
			names[p.ID] = p.Name
		}
	}

	res := &CountSessionResponse{InventoryCountSession: session}
	for _, l := range lines {
		res.Lines = append(res.Lines, CountLineResponse{
			ID: l.ID, ProductID: l.ProductID, ProductName: names[l.ProductID],
			ExpectedQty: l.ExpectedQty, ActualQty: l.ActualQty,
			DifferenceQty: l.DifferenceQty,
			CountedAt:     l.CountedAt.Format(time.RFC3339),
		})
		if l.DifferenceQty != 0 {
			res.DiffCount++
			if l.DifferenceQty < 0 {
				res.TotalShort += -l.DifferenceQty
			} else {
				res.TotalOver += l.DifferenceQty
			}
		}
	}
	res.LineCount = len(lines)
	return res, nil
}

type CountListRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Status   string `form:"status"`
}

func (s *InventoryCountService) List(req *CountListRequest) (*pagination.Result, error) {
	page, size := req.Page, req.PageSize
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}

	q := s.db.Model(&model.InventoryCountSession{})
	if req.Status != "" {
		q = q.Where("status = ?", req.Status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []model.InventoryCountSession
	if err := q.Order("opened_at desc").Offset((page - 1) * size).Limit(size).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	// Tổng hợp số dòng và số dòng lệch cho từng phiên, trong MỘT truy vấn.
	out := make([]CountSessionResponse, 0, len(rows))
	if len(rows) > 0 {
		ids := make([]string, 0, len(rows))
		for _, r := range rows {
			ids = append(ids, r.ID)
		}
		var stats []struct {
			SessionID string
			Lines     int
			Diffs     int
		}
		s.db.Model(&model.InventoryCount{}).
			Select("session_id, count(*) as lines, count(*) filter (where difference_qty <> 0) as diffs").
			Where("session_id IN ?", ids).Group("session_id").Find(&stats)
		byID := map[string]struct {
			SessionID string
			Lines     int
			Diffs     int
		}{}
		for _, st := range stats {
			byID[st.SessionID] = st
		}
		for _, r := range rows {
			item := CountSessionResponse{InventoryCountSession: r}
			if st, ok := byID[r.ID]; ok {
				item.LineCount, item.DiffCount = st.Lines, st.Diffs
			}
			out = append(out, item)
		}
	}

	return &pagination.Result{Items: out, Total: total, Page: page, PageSize: size}, nil
}

func (s *InventoryCountService) loadOpenSession(tx *gorm.DB, id string) (*model.InventoryCountSession, error) {
	var session model.InventoryCountSession
	if err := tx.First(&session, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy phiên kiểm kê")
		}
		return nil, err
	}
	if session.Status != CountStatusOpen {
		return nil, fmt.Errorf("phiên đã ở trạng thái %s, không sửa được", session.Status)
	}
	return &session, nil
}

func (s *InventoryCountService) log(action, id, actorID string, meta map[string]interface{}) {
	var actor *string
	if actorID != "" {
		actor = &actorID
	}
	s.audit.Log(&LogAuditRequest{
		Action: action, EntityType: "inventory_count", EntityID: id,
		UserID: actor, Metadata: meta,
	})
}
