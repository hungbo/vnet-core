package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/utils"

	"gorm.io/gorm"
)

// Bộ máy khuyến mãi: quyết định một đơn hàng được giảm bao nhiêu tiền.
//
// Bảng promotion_conditions và promotion_rewards đã tồn tại và lưu được dữ liệu
// từ lâu, nhưng chưa có gì đọc chúng — nên mọi khuyến mãi tạo ra đều nằm im.
//
// Nguyên tắc quan trọng nhất ở đây: **điều kiện không hiểu được thì không áp
// khuyến mãi**. Bỏ qua một khoá lạ sẽ biến "giảm 20% cho nhóm VIP" thành giảm
// cho tất cả mọi người — sai theo hướng mất tiền. Vì vậy Create/Update chặn
// khoá lạ ngay lúc lưu, và bộ đánh giá vẫn phòng thủ lần nữa lúc chạy.

// Khoá điều kiện được hỗ trợ.
const (
	CondMinAmount       = "min_amount"       // tổng tiền hàng >= giá trị
	CondMinQuantity     = "min_quantity"     // tổng số lượng món >= giá trị
	CondMemberGroup     = "member_group"     // hội viên thuộc một trong các nhóm
	CondDayOfWeek       = "day_of_week"      // thứ trong tuần (0 = Chủ nhật)
	CondTimeRange       = "time_range"       // {"from":"18:00","to":"22:00"}
	CondProductCategory = "product_category" // đơn có ít nhất một món thuộc danh mục
)

// Loại thưởng áp dụng được cho đơn hàng.
const (
	RewardDiscountPercent = "discount_percent" // {"percent":10,"max_discount":50000}
	RewardDiscountAmount  = "discount_amount"  // {"amount":20000}
)

// SupportedConditionKeys và SupportedOrderRewardTypes được handler dùng để báo
// lỗi có ích khi người dùng gõ sai khoá.
var SupportedConditionKeys = []string{
	CondMinAmount, CondMinQuantity, CondMemberGroup,
	CondDayOfWeek, CondTimeRange, CondProductCategory,
}

// Loại thưởng của vòng quay may mắn nằm ở bảng khác (lucky_spin_rewards) nên
// không liệt kê ở đây; một khuyến mãi không có thưởng giảm giá đơn giản là
// không bao giờ áp vào đơn hàng.
var SupportedOrderRewardTypes = []string{
	RewardDiscountPercent, RewardDiscountAmount,
}

// PromotionContext là toàn bộ dữ kiện bộ máy cần để xét một đơn.
type PromotionContext struct {
	MemberID    string
	Amount      int64 // tổng tiền hàng trước khi giảm
	Quantity    int   // tổng số lượng món
	CategoryIDs []string
	At          time.Time
}

// AppliedPromotion là kết quả: khuyến mãi nào, giảm bao nhiêu.
type AppliedPromotion struct {
	PromotionID string `json:"promotion_id"`
	Name        string `json:"name"`
	Discount    int64  `json:"discount"`
}

// BestPromotionForOrder chọn khuyến mãi tốt nhất cho đơn, hoặc nil nếu không có.
// Là hàm cấp gói chứ không phải phương thức: OrderService gọi được ngay bằng
// chính giao dịch đang mở, không cần nối thêm phụ thuộc.
//
// Chỉ một khuyến mãi được áp: bảng orders có đúng một cột promotion_id, nên
// cộng dồn nhiều khuyến mãi sẽ không ghi lại được cái nào đã áp. Thứ tự ưu
// tiên: priority cao hơn thắng; bằng nhau thì số tiền giảm lớn hơn thắng.
func BestPromotionForOrder(tx *gorm.DB, pctx PromotionContext) *AppliedPromotion {
	if tx == nil {
		return nil
	}
	if pctx.At.IsZero() {
		pctx.At = time.Now()
	}

	var promos []model.Promotion
	err := tx.Where("is_active = ?", true).
		Where("valid_from IS NULL OR valid_from <= ?", pctx.At).
		Where("valid_to IS NULL OR valid_to >= ?", pctx.At).
		Order("priority DESC").
		Find(&promos).Error
	if err != nil || len(promos) == 0 {
		return nil
	}

	// Nạp điều kiện và thưởng của tất cả khuyến mãi trong hai truy vấn, thay vì
	// hai truy vấn cho mỗi khuyến mãi. Model Promotion không khai báo quan hệ
	// nên không Preload được — phần còn lại của service cũng nạp tay như vậy.
	ids := make([]string, len(promos))
	for i := range promos {
		ids[i] = promos[i].ID
	}
	var allConds []model.PromotionCondition
	var allRewards []model.PromotionReward
	tx.Where("promotion_id IN ?", ids).Find(&allConds)
	tx.Where("promotion_id IN ?", ids).Find(&allRewards)
	condsBy := map[string][]model.PromotionCondition{}
	for _, c := range allConds {
		condsBy[c.PromotionID] = append(condsBy[c.PromotionID], c)
	}
	rewardsBy := map[string][]model.PromotionReward{}
	for _, r := range allRewards {
		rewardsBy[r.PromotionID] = append(rewardsBy[r.PromotionID], r)
	}

	// Nhóm hội viên chỉ tra khi có điều kiện cần tới, và chỉ tra một lần.
	groupLoaded := false
	memberGroup := ""
	lookupGroup := func() string {
		if !groupLoaded {
			groupLoaded = true
			if pctx.MemberID != "" {
				var m model.Member
				if err := tx.Select("group_id").First(&m, "id = ?", pctx.MemberID).Error; err == nil && m.GroupID != nil {
					memberGroup = *m.GroupID
				}
			}
		}
		return memberGroup
	}

	var best *AppliedPromotion
	bestPriority := 0
	for i := range promos {
		p := &promos[i]
		if best != nil && p.Priority < bestPriority {
			// Danh sách đã sắp theo priority giảm dần: không còn ứng viên nào
			// thắng được nữa.
			break
		}
		if !conditionsPass(condsBy[p.ID], pctx, lookupGroup) {
			continue
		}
		discount := discountFromRewards(rewardsBy[p.ID], pctx.Amount)
		if discount <= 0 {
			continue
		}
		if discount > pctx.Amount {
			discount = pctx.Amount
		}
		if best == nil || discount > best.Discount {
			best = &AppliedPromotion{PromotionID: p.ID, Name: p.Name, Discount: discount}
			bestPriority = p.Priority
		}
	}
	return best
}

// EvaluatePromotion xét lại ĐÚNG MỘT khuyến mãi cho một đơn, hoặc nil nếu đơn
// không còn đủ điều kiện.
//
// Dùng khi tách đơn: khuyến mãi đã được chốt lúc tạo đơn, nên ở đây KHÔNG lọc
// is_active / valid_from / valid_to — quán tắt khuyến mãi sau khi khách đã đặt
// món không được làm mất phần giảm khách đã được hứa. Nhưng điều kiện theo giỏ
// hàng (tiền tối thiểu, số lượng, danh mục) thì phải xét lại cho từng nửa: tách
// một đơn 200k "giảm 20k cho đơn từ 150k" thành hai đơn 100k mà vẫn giữ giảm
// trên cả hai là cho không tiền.
func EvaluatePromotion(tx *gorm.DB, promotionID string, pctx PromotionContext) *AppliedPromotion {
	if tx == nil || promotionID == "" {
		return nil
	}
	var p model.Promotion
	if err := tx.Where("id = ?", promotionID).First(&p).Error; err != nil {
		return nil
	}
	var conds []model.PromotionCondition
	var rewards []model.PromotionReward
	tx.Where("promotion_id = ?", promotionID).Find(&conds)
	tx.Where("promotion_id = ?", promotionID).Find(&rewards)

	lookupGroup := func() string {
		if pctx.MemberID == "" {
			return ""
		}
		var m model.Member
		if err := tx.Select("group_id").First(&m, "id = ?", pctx.MemberID).Error; err == nil && m.GroupID != nil {
			return *m.GroupID
		}
		return ""
	}
	if !conditionsPass(conds, pctx, lookupGroup) {
		return nil
	}
	discount := discountFromRewards(rewards, pctx.Amount)
	if discount <= 0 {
		return nil
	}
	if discount > pctx.Amount {
		discount = pctx.Amount
	}
	return &AppliedPromotion{PromotionID: p.ID, Name: p.Name, Discount: discount}
}

// conditionsPass: mọi điều kiện phải đúng (AND). Nhiều giá trị trong cùng một
// điều kiện là OR — "thứ 7 hoặc chủ nhật" là một điều kiện, không phải hai.
func conditionsPass(conds []model.PromotionCondition, pctx PromotionContext, memberGroup func() string) bool {
	for _, c := range conds {
		var raw interface{}
		if err := json.Unmarshal([]byte(c.ConditionValue), &raw); err != nil {
			return false
		}
		if !conditionPasses(c.ConditionKey, raw, pctx, memberGroup) {
			return false
		}
	}
	return true
}

func conditionPasses(key string, raw interface{}, pctx PromotionContext, memberGroup func() string) bool {
	switch key {
	case CondMinAmount:
		n, ok := asNumber(raw)
		return ok && float64(pctx.Amount) >= n

	case CondMinQuantity:
		n, ok := asNumber(raw)
		return ok && float64(pctx.Quantity) >= n

	case CondMemberGroup:
		g := memberGroup()
		if g == "" {
			return false
		}
		return containsString(asStringSet(raw), g)

	case CondDayOfWeek:
		days := asNumberSet(raw)
		if len(days) == 0 {
			return false
		}
		// Thứ và giờ phải tính theo giờ quán (Việt Nam), không theo múi giờ của
		// máy chủ: máy chủ chạy UTC thì "thứ 7 từ 18:00" lệch 7 tiếng, khuyến
		// mãi tối thứ 7 bắt đầu từ 1 giờ sáng Chủ nhật.
		today := float64(pctx.At.In(utils.VietnamLocation()).Weekday())
		for _, d := range days {
			if d == today {
				return true
			}
		}
		return false

	case CondTimeRange:
		m, ok := raw.(map[string]interface{})
		if !ok {
			return false
		}
		from, _ := m["from"].(string)
		to, _ := m["to"].(string)
		if from == "" || to == "" {
			return false
		}
		// withinCurfew là so sánh khung giờ trong ngày có xử lý vắt qua nửa
		// đêm — đúng thứ cần ở đây, dù tên nó gắn với giới nghiêm.
		// Giờ quán, cùng lý do với day_of_week ở trên.
		return withinCurfew(pctx.At.In(utils.VietnamLocation()).Format("15:04"), normalizeClock(from), normalizeClock(to))

	case CondProductCategory:
		want := asStringSet(raw)
		for _, cat := range pctx.CategoryIDs {
			if containsString(want, cat) {
				return true
			}
		}
		return false
	}

	// Khoá lạ: không áp. Xem ghi chú đầu file.
	return false
}

// discountFromRewards cộng dồn các thưởng giảm giá của cùng một khuyến mãi.
// Thưởng không phải giảm giá (vòng quay may mắn…) bị bỏ qua.
func discountFromRewards(rewards []model.PromotionReward, amount int64) int64 {
	var total int64
	for _, r := range rewards {
		var v map[string]interface{}
		if err := json.Unmarshal([]byte(r.RewardValue), &v); err != nil {
			continue
		}
		switch r.RewardType {
		case RewardDiscountPercent:
			pct, ok := asNumber(v["percent"])
			if !ok || pct <= 0 {
				continue
			}
			if pct > 100 {
				pct = 100
			}
			d := int64(float64(amount) * pct / 100)
			if maxD, ok := asNumber(v["max_discount"]); ok && maxD > 0 && float64(d) > maxD {
				d = int64(maxD)
			}
			total += d
		case RewardDiscountAmount:
			if a, ok := asNumber(v["amount"]); ok && a > 0 {
				total += int64(a)
			}
		}
	}
	return total
}

// ValidatePromotionRules kiểm khoá điều kiện và loại thưởng lúc lưu, để lỗi gõ
// sai lộ ra ngay chứ không biến thành một khuyến mãi im lặng không bao giờ chạy.
func ValidatePromotionRules(condKeys []string, rewardTypes []string) error {
	for _, k := range condKeys {
		if k != "" && !containsString(SupportedConditionKeys, k) {
			return fmt.Errorf("điều kiện %q không được hỗ trợ (chấp nhận: %s)",
				k, strings.Join(SupportedConditionKeys, ", "))
		}
	}
	for _, t := range rewardTypes {
		if t != "" && !containsString(SupportedOrderRewardTypes, t) {
			return fmt.Errorf("loại thưởng %q không được hỗ trợ (chấp nhận: %s)",
				t, strings.Join(SupportedOrderRewardTypes, ", "))
		}
	}
	return nil
}

// --- tiện ích đọc jsonb ------------------------------------------------------
// condition_value là jsonb tự do, người dùng gõ tay; mọi hàm dưới đây đều phải
// chịu được kiểu sai mà không panic.

func asNumber(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case string:
		var f float64
		if _, err := fmt.Sscanf(n, "%g", &f); err == nil {
			return f, true
		}
	}
	return 0, false
}

func asStringSet(v interface{}) []string {
	switch x := v.(type) {
	case string:
		return []string{x}
	case []interface{}:
		out := make([]string, 0, len(x))
		for _, it := range x {
			if s, ok := it.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func asNumberSet(v interface{}) []float64 {
	if n, ok := asNumber(v); ok {
		return []float64{n}
	}
	if arr, ok := v.([]interface{}); ok {
		out := make([]float64, 0, len(arr))
		for _, it := range arr {
			if n, ok := asNumber(it); ok {
				out = append(out, n)
			}
		}
		return out
	}
	return nil
}

func containsString(set []string, want string) bool {
	for _, s := range set {
		if s == want {
			return true
		}
	}
	return false
}

// normalizeClock cắt "18:00:00" về "18:00" để so sánh chuỗi hoạt động đúng.
func normalizeClock(s string) string {
	if len(s) >= 5 {
		return s[:5]
	}
	return s
}

// validateRuleRequest gom khoá từ payload rồi giao cho ValidatePromotionRules,
// sau đó kiểm giá trị của từng dòng.
func validateRuleRequest(conds []CreatePromotionCondition, rewards []CreatePromotionReward) error {
	condKeys := make([]string, len(conds))
	for i, c := range conds {
		condKeys[i] = c.ConditionKey
	}
	rewardTypes := make([]string, len(rewards))
	for i, r := range rewards {
		rewardTypes[i] = r.RewardType
	}
	if err := ValidatePromotionRules(condKeys, rewardTypes); err != nil {
		return err
	}
	for _, c := range conds {
		if err := validateConditionValue(c.ConditionKey, c.ConditionValue); err != nil {
			return err
		}
	}
	for _, r := range rewards {
		if err := validateRewardValue(r.RewardType, r.RewardValue); err != nil {
			return err
		}
	}
	return nil
}

// validateConditionValue chặn giá trị mà bộ máy sẽ đọc ra "không áp" — lưu
// được nhưng không bao giờ chạy là lỗi khó thấy nhất với người tạo khuyến mãi.
func validateConditionValue(key string, value json.RawMessage) error {
	var raw interface{}
	if err := json.Unmarshal(value, &raw); err != nil {
		return fmt.Errorf("giá trị điều kiện %q không phải JSON hợp lệ", key)
	}
	switch key {
	case CondMinAmount, CondMinQuantity:
		if n, ok := asNumber(raw); !ok || n <= 0 {
			return fmt.Errorf("điều kiện %q phải là một số lớn hơn 0", key)
		}
	case CondDayOfWeek:
		days := asNumberSet(raw)
		if len(days) == 0 {
			return errors.New("điều kiện day_of_week phải là số 0–6 hoặc danh sách, VD [0,6] (0 = Chủ nhật)")
		}
		for _, d := range days {
			if d < 0 || d > 6 || d != float64(int(d)) {
				return errors.New("điều kiện day_of_week chỉ nhận số nguyên 0–6 (0 = Chủ nhật)")
			}
		}
	case CondTimeRange:
		m, _ := raw.(map[string]interface{})
		from, _ := m["from"].(string)
		to, _ := m["to"].(string)
		if !gioHopLe(from) || !gioHopLe(to) {
			return errors.New(`điều kiện time_range phải có dạng {"from":"18:00","to":"22:00"}`)
		}
	case CondMemberGroup, CondProductCategory:
		if len(asStringSet(raw)) == 0 {
			return fmt.Errorf("điều kiện %q phải là một mã hoặc danh sách mã", key)
		}
	}
	return nil
}

// validateRewardValue: thưởng giảm giá phải là object có đúng khoá bộ máy đọc.
// Gõ "10" vào ô giảm % lưu được nhưng discountFromRewards đọc ra 0 — khuyến
// mãi chết lặng lẽ.
func validateRewardValue(rewardType string, value json.RawMessage) error {
	var v map[string]interface{}
	if err := json.Unmarshal(value, &v); err != nil {
		switch rewardType {
		case RewardDiscountPercent:
			return errors.New(`thưởng giảm % phải có dạng {"percent":10,"max_discount":50000}`)
		default:
			return errors.New(`thưởng giảm tiền phải có dạng {"amount":20000}`)
		}
	}
	switch rewardType {
	case RewardDiscountPercent:
		pct, ok := asNumber(v["percent"])
		if !ok || pct <= 0 || pct > 100 {
			return errors.New("phần trăm giảm phải lớn hơn 0 và không quá 100")
		}
		if raw, has := v["max_discount"]; has && raw != nil {
			if maxD, ok := asNumber(raw); !ok || maxD <= 0 {
				return errors.New("mức giảm tối đa (max_discount) phải là số lớn hơn 0")
			}
		}
	case RewardDiscountAmount:
		if a, ok := asNumber(v["amount"]); !ok || a <= 0 {
			return errors.New("số tiền giảm phải là số lớn hơn 0")
		}
	}
	return nil
}

// gioHopLe nhận "HH:MM" hoặc "HH:MM:SS" — đúng hai dạng normalizeClock xử lý.
func gioHopLe(s string) bool {
	if _, err := time.Parse("15:04", s); err == nil {
		return true
	}
	_, err := time.Parse("15:04:05", s)
	return err == nil
}
