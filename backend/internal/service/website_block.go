package service

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
)

// Chặn website.
//
// Máy chủ giữ luật, lịch áp dụng và phạm vi máy; máy trạm hỏi "bây giờ tôi phải
// chặn những tên miền nào" rồi tự cưỡng chế.
//
// Việc chia như vậy có chủ đích: máy trạm không cần biết luật, lịch hay nhóm
// máy — nó chỉ nhận một danh sách tên miền phẳng. Đổi lịch ở quầy là lần hỏi
// tiếp theo máy trạm đã áp đúng, không phải cài lại gì.

type WebsiteBlockService struct {
	db    *gorm.DB
	audit *AuditService
}

func NewWebsiteBlockService(db *gorm.DB, audit *AuditService) *WebsiteBlockService {
	return &WebsiteBlockService{db: db, audit: audit}
}

const (
	RuleTypeBlock = "block"
	RuleTypeAllow = "allow"
)

type CreateRuleRequest struct {
	Pattern     string `json:"pattern" binding:"required"`
	RuleType    string `json:"rule_type"`
	Category    string `json:"category"`
	Description string `json:"description"`
	IsActive    *bool  `json:"is_active"`
}

type UpdateRuleRequest struct {
	Pattern     *string `json:"pattern"`
	RuleType    *string `json:"rule_type"`
	Category    *string `json:"category"`
	Description *string `json:"description"`
	IsActive    *bool   `json:"is_active"`
}

// normalizeDomain gỡ giao thức, đường dẫn, cổng và tiền tố "*." để mọi luật quy
// về một dạng duy nhất. Không chuẩn hoá thì "https://Facebook.com/" và
// "facebook.com" thành hai luật khác nhau mà nhân viên tưởng là một.
func normalizeDomain(p string) string {
	d := strings.TrimSpace(strings.ToLower(p))
	d = strings.TrimPrefix(strings.TrimPrefix(d, "https://"), "http://")
	if i := strings.IndexAny(d, "/?#"); i >= 0 {
		d = d[:i]
	}
	if i := strings.Index(d, ":"); i >= 0 {
		d = d[:i]
	}
	d = strings.TrimPrefix(d, "*.")
	d = strings.TrimPrefix(d, "www.")
	return strings.Trim(d, ".")
}

func (s *WebsiteBlockService) CreateRule(req *CreateRuleRequest, actorID string) (*model.WebsiteBlockingRule, error) {
	pattern := normalizeDomain(req.Pattern)
	if pattern == "" || !strings.Contains(pattern, ".") {
		return nil, errors.New("tên miền không hợp lệ")
	}

	ruleType := req.RuleType
	if ruleType == "" {
		ruleType = RuleTypeBlock
	}
	if ruleType != RuleTypeBlock && ruleType != RuleTypeAllow {
		return nil, fmt.Errorf("loại luật %q không hợp lệ (chấp nhận: block, allow)", ruleType)
	}

	rule := model.WebsiteBlockingRule{
		Pattern:     pattern,
		RuleType:    ruleType,
		Category:    req.Category,
		Description: req.Description,
		IsActive:    req.IsActive == nil || *req.IsActive,
	}
	if err := s.db.Create(&rule).Error; err != nil {
		return nil, err
	}
	s.log("create_website_rule", rule.ID, actorID, map[string]interface{}{
		"pattern": rule.Pattern, "rule_type": rule.RuleType,
	})
	return &rule, nil
}

func (s *WebsiteBlockService) UpdateRule(id string, req *UpdateRuleRequest, actorID string) (*model.WebsiteBlockingRule, error) {
	var rule model.WebsiteBlockingRule
	if err := s.db.First(&rule, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy luật")
		}
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.Pattern != nil {
		p := normalizeDomain(*req.Pattern)
		if p == "" || !strings.Contains(p, ".") {
			return nil, errors.New("tên miền không hợp lệ")
		}
		updates["pattern"] = p
	}
	if req.RuleType != nil {
		if *req.RuleType != RuleTypeBlock && *req.RuleType != RuleTypeAllow {
			return nil, fmt.Errorf("loại luật %q không hợp lệ", *req.RuleType)
		}
		updates["rule_type"] = *req.RuleType
	}
	if req.Category != nil {
		updates["category"] = *req.Category
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if len(updates) > 0 {
		if err := s.db.Model(&rule).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	s.log("update_website_rule", id, actorID, updates)
	return &rule, nil
}

func (s *WebsiteBlockService) DeleteRule(id, actorID string) error {
	if err := s.db.Delete(&model.WebsiteBlockingRule{}, "id = ?", id).Error; err != nil {
		return err
	}
	s.log("delete_website_rule", id, actorID, nil)
	return nil
}

type RuleResponse struct {
	model.WebsiteBlockingRule
	Schedules     []model.WebsiteBlockingSchedule `json:"schedules"`
	MachineGroups []string                        `json:"machine_group_ids"`
}

func (s *WebsiteBlockService) GetRule(id string) (*RuleResponse, error) {
	var rule model.WebsiteBlockingRule
	if err := s.db.First(&rule, "id = ?", id).Error; err != nil {
		return nil, errors.New("không tìm thấy luật")
	}
	res := &RuleResponse{WebsiteBlockingRule: rule, Schedules: []model.WebsiteBlockingSchedule{}, MachineGroups: []string{}}
	s.db.Where("rule_id = ?", id).Order("start_time asc").Find(&res.Schedules)

	var maps []model.WebsiteRuleMapping
	s.db.Where("rule_id = ?", id).Find(&maps)
	for _, m := range maps {
		if m.MachineGroupID != nil {
			res.MachineGroups = append(res.MachineGroups, *m.MachineGroupID)
		}
	}
	return res, nil
}

type RuleListRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Search   string `form:"search"`
	RuleType string `form:"rule_type"`
	Category string `form:"category"`
}

func (s *WebsiteBlockService) ListRules(req *RuleListRequest) (*pagination.Result, error) {
	page, size := req.Page, req.PageSize
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 50
	}

	q := s.db.Model(&model.WebsiteBlockingRule{})
	if req.Search != "" {
		q = q.Where("pattern ILIKE ?", "%"+strings.ToLower(req.Search)+"%")
	}
	if req.RuleType != "" {
		q = q.Where("rule_type = ?", req.RuleType)
	}
	if req.Category != "" {
		q = q.Where("category = ?", req.Category)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	var rules []model.WebsiteBlockingRule
	if err := q.Order("pattern asc").Offset((page - 1) * size).Limit(size).
		Find(&rules).Error; err != nil {
		return nil, err
	}

	out := make([]RuleResponse, 0, len(rules))
	if len(rules) > 0 {
		ids := make([]string, 0, len(rules))
		for _, r := range rules {
			ids = append(ids, r.ID)
		}
		var scheds []model.WebsiteBlockingSchedule
		s.db.Where("rule_id IN ?", ids).Order("start_time asc").Find(&scheds)
		byRule := map[string][]model.WebsiteBlockingSchedule{}
		for _, sc := range scheds {
			byRule[sc.RuleID] = append(byRule[sc.RuleID], sc)
		}

		var maps []model.WebsiteRuleMapping
		s.db.Where("rule_id IN ?", ids).Find(&maps)
		groupsByRule := map[string][]string{}
		for _, m := range maps {
			if m.MachineGroupID != nil {
				groupsByRule[m.RuleID] = append(groupsByRule[m.RuleID], *m.MachineGroupID)
			}
		}

		for _, r := range rules {
			item := RuleResponse{WebsiteBlockingRule: r,
				Schedules: byRule[r.ID], MachineGroups: groupsByRule[r.ID]}
			if item.Schedules == nil {
				item.Schedules = []model.WebsiteBlockingSchedule{}
			}
			if item.MachineGroups == nil {
				item.MachineGroups = []string{}
			}
			out = append(out, item)
		}
	}
	return &pagination.Result{Items: out, Total: total, Page: page, PageSize: size}, nil
}

// --- lịch áp dụng ----------------------------------------------------------

type SetSchedulesRequest struct {
	Schedules []ScheduleInput `json:"schedules"`
}

type ScheduleInput struct {
	DayOfWeek []int  `json:"day_of_week"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

// SetSchedules thay toàn bộ lịch của một luật. Không có lịch nào nghĩa là luật
// áp dụng 24/7 — đó là mặc định hợp lý hơn "không bao giờ áp dụng".
func (s *WebsiteBlockService) SetSchedules(ruleID string, req *SetSchedulesRequest, actorID string) error {
	if err := s.db.First(&model.WebsiteBlockingRule{}, "id = ?", ruleID).Error; err != nil {
		return errors.New("không tìm thấy luật")
	}

	for _, sc := range req.Schedules {
		if sc.StartTime == "" || sc.EndTime == "" {
			return errors.New("lịch phải có giờ bắt đầu và giờ kết thúc")
		}
		for _, d := range sc.DayOfWeek {
			if d < 0 || d > 6 {
				return fmt.Errorf("thứ %d không hợp lệ (0 = Chủ nhật … 6 = Thứ bảy)", d)
			}
		}
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("rule_id = ?", ruleID).
			Delete(&model.WebsiteBlockingSchedule{}).Error; err != nil {
			return err
		}
		for _, sc := range req.Schedules {
			row := model.WebsiteBlockingSchedule{
				RuleID:    ruleID,
				DayOfWeek: sc.DayOfWeek,
				StartTime: sc.StartTime,
				EndTime:   sc.EndTime,
				IsActive:  true,
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		s.log("set_website_schedules", ruleID, actorID,
			map[string]interface{}{"count": len(req.Schedules)})
		return nil
	})
}

// --- phạm vi máy -----------------------------------------------------------

type SetGroupsRequest struct {
	MachineGroupIDs []string `json:"machine_group_ids"`
}

// SetGroups gán luật cho các nhóm máy. Danh sách rỗng nghĩa là áp cho MỌI máy —
// luật chặn mà không áp cho máy nào thì vô nghĩa.
func (s *WebsiteBlockService) SetGroups(ruleID string, req *SetGroupsRequest, actorID string) error {
	if err := s.db.First(&model.WebsiteBlockingRule{}, "id = ?", ruleID).Error; err != nil {
		return errors.New("không tìm thấy luật")
	}

	seen := map[string]bool{}
	clean := make([]string, 0, len(req.MachineGroupIDs))
	for _, id := range req.MachineGroupIDs {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		clean = append(clean, id)
	}
	if len(clean) > 0 {
		var n int64
		if err := s.db.Model(&model.MachineGroup{}).Where("id IN ?", clean).Count(&n).Error; err != nil {
			return err
		}
		if int(n) != len(clean) {
			return errors.New("có nhóm máy không tồn tại")
		}
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("rule_id = ?", ruleID).
			Delete(&model.WebsiteRuleMapping{}).Error; err != nil {
			return err
		}
		for _, gid := range clean {
			g := gid
			if err := tx.Create(&model.WebsiteRuleMapping{
				RuleID: ruleID, MachineGroupID: &g,
			}).Error; err != nil {
				return err
			}
		}
		s.log("set_website_groups", ruleID, actorID,
			map[string]interface{}{"count": len(clean)})
		return nil
	})
}

// --- danh sách hiệu lực cho máy trạm ---------------------------------------

type EffectiveBlocklist struct {
	MachineCode string   `json:"machine_code"`
	Domains     []string `json:"domains"`
	Allowed     []string `json:"allowed"`
	GeneratedAt string   `json:"generated_at"`
}

// EffectiveFor trả về danh sách tên miền máy trạm phải chặn NGAY LÚC NÀY.
//
// Ba bộ lọc chồng lên nhau: luật đang bật · lịch có phủ thời điểm hiện tại ·
// luật áp cho nhóm của máy này (hoặc áp cho mọi máy).
//
// Luật "allow" được trừ ra ở cuối: cho phép chặn cả một danh mục rồi mở lại vài
// tên miền cụ thể mà không phải liệt kê thủ công.
func (s *WebsiteBlockService) EffectiveFor(machineCode string, at time.Time) (*EffectiveBlocklist, error) {
	var machine model.Machine
	if err := s.db.Select("id, machine_code, group_id").
		Where("machine_code = ?", machineCode).First(&machine).Error; err != nil {
		return nil, errors.New("không tìm thấy máy")
	}

	var rules []model.WebsiteBlockingRule
	if err := s.db.Where("is_active = ?", true).Find(&rules).Error; err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return &EffectiveBlocklist{MachineCode: machineCode, Domains: []string{},
			Allowed: []string{}, GeneratedAt: at.Format(time.RFC3339)}, nil
	}

	ids := make([]string, 0, len(rules))
	for _, r := range rules {
		ids = append(ids, r.ID)
	}

	var scheds []model.WebsiteBlockingSchedule
	s.db.Where("rule_id IN ? AND is_active = ?", ids, true).Find(&scheds)
	schedByRule := map[string][]model.WebsiteBlockingSchedule{}
	for _, sc := range scheds {
		schedByRule[sc.RuleID] = append(schedByRule[sc.RuleID], sc)
	}

	var maps []model.WebsiteRuleMapping
	s.db.Where("rule_id IN ?", ids).Find(&maps)
	groupsByRule := map[string][]string{}
	for _, m := range maps {
		if m.MachineGroupID != nil {
			groupsByRule[m.RuleID] = append(groupsByRule[m.RuleID], *m.MachineGroupID)
		}
	}

	blocked := map[string]bool{}
	allowed := map[string]bool{}
	for _, r := range rules {
		if !ruleAppliesToMachine(groupsByRule[r.ID], machine.GroupID) {
			continue
		}
		if !scheduleCoversNow(schedByRule[r.ID], at) {
			continue
		}
		if r.RuleType == RuleTypeAllow {
			allowed[r.Pattern] = true
		} else {
			blocked[r.Pattern] = true
		}
	}

	domains := make([]string, 0, len(blocked))
	for d := range blocked {
		if !allowed[d] {
			domains = append(domains, d)
		}
	}
	sort.Strings(domains)

	allowList := make([]string, 0, len(allowed))
	for d := range allowed {
		allowList = append(allowList, d)
	}
	sort.Strings(allowList)

	return &EffectiveBlocklist{
		MachineCode: machineCode, Domains: domains, Allowed: allowList,
		GeneratedAt: at.Format(time.RFC3339),
	}, nil
}

// ruleAppliesToMachine: luật không gán nhóm nào thì áp cho mọi máy.
func ruleAppliesToMachine(ruleGroups []string, machineGroup *string) bool {
	if len(ruleGroups) == 0 {
		return true
	}
	if machineGroup == nil {
		return false
	}
	for _, g := range ruleGroups {
		if g == *machineGroup {
			return true
		}
	}
	return false
}

// scheduleCoversNow: luật không có lịch nào thì áp 24/7.
func scheduleCoversNow(scheds []model.WebsiteBlockingSchedule, at time.Time) bool {
	if len(scheds) == 0 {
		return true
	}
	today := int(at.Weekday())
	now := at.Format("15:04")
	for _, sc := range scheds {
		if len(sc.DayOfWeek) > 0 {
			match := false
			for _, d := range sc.DayOfWeek {
				if d == today {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		// withinCurfew so khung giờ trong ngày và xử lý được khung vắt qua nửa
		// đêm — đúng thứ cần ở đây.
		if withinCurfew(now, clockHHMM(sc.StartTime), clockHHMM(sc.EndTime)) {
			return true
		}
	}
	return false
}

func clockHHMM(s string) string {
	if len(s) >= 5 {
		return s[:5]
	}
	return s
}

// --- vi phạm ---------------------------------------------------------------

type ReportViolationRequest struct {
	Domain      string `json:"domain" binding:"required"`
	URL         string `json:"url"`
	ProcessName string `json:"process_name"`
}

func (s *WebsiteBlockService) ReportViolation(machineCode string, req *ReportViolationRequest) error {
	var machine model.Machine
	if err := s.db.Select("id").Where("machine_code = ?", machineCode).
		First(&machine).Error; err != nil {
		return errors.New("không tìm thấy máy")
	}

	domain := normalizeDomain(req.Domain)
	if domain == "" {
		return errors.New("thiếu tên miền")
	}

	v := model.WebsiteBlockingViolation{
		MachineID:   machine.ID,
		Domain:      domain,
		URL:         req.URL,
		ProcessName: req.ProcessName,
		BlockedAt:   time.Now(),
	}
	// Gắn luật nào đã chặn, nếu tìm được — báo cáo "chặn vì luật nào" hữu ích
	// hơn nhiều so với chỉ một tên miền.
	var rule model.WebsiteBlockingRule
	if err := s.db.Where("pattern = ? AND rule_type = ?", domain, RuleTypeBlock).
		First(&rule).Error; err == nil {
		v.RuleID = &rule.ID
	}
	return s.db.Create(&v).Error
}

type ViolationListRequest struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	MachineID string `form:"machine_id"`
	Domain    string `form:"domain"`
}

type ViolationRow struct {
	model.WebsiteBlockingViolation
	MachineCode string `json:"machine_code"`
}

func (s *WebsiteBlockService) ListViolations(req *ViolationListRequest) (*pagination.Result, error) {
	page, size := req.Page, req.PageSize
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}

	q := s.db.Model(&model.WebsiteBlockingViolation{})
	if req.MachineID != "" {
		q = q.Where("machine_id = ?", req.MachineID)
	}
	if req.Domain != "" {
		q = q.Where("domain ILIKE ?", "%"+strings.ToLower(req.Domain)+"%")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []model.WebsiteBlockingViolation
	if err := q.Order("blocked_at desc").Offset((page - 1) * size).Limit(size).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	out := make([]ViolationRow, 0, len(rows))
	if len(rows) > 0 {
		ids := make([]string, 0, len(rows))
		for _, r := range rows {
			ids = append(ids, r.MachineID)
		}
		var machines []struct {
			ID          string
			MachineCode string
		}
		// Unscoped: nhật ký vi phạm là chứng từ; máy đã gỡ vẫn phải hiện mã.
		s.db.Unscoped().Model(&model.Machine{}).Where("id IN ?", ids).Find(&machines)
		byID := map[string]string{}
		for _, m := range machines {
			byID[m.ID] = m.MachineCode
		}
		for _, r := range rows {
			out = append(out, ViolationRow{WebsiteBlockingViolation: r, MachineCode: byID[r.MachineID]})
		}
	}
	return &pagination.Result{Items: out, Total: total, Page: page, PageSize: size}, nil
}

func (s *WebsiteBlockService) log(action, id, actorID string, meta map[string]interface{}) {
	var actor *string
	if actorID != "" {
		actor = &actorID
	}
	s.audit.Log(&LogAuditRequest{
		Action: action, EntityType: "website_rule", EntityID: id,
		UserID: actor, Metadata: meta,
	})
}
