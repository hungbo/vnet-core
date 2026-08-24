package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/vnet/core/internal/model"
)

// Bộ đánh giá điều kiện là logic thuần, không chạm database — kiểm trực tiếp
// thay vì qua sqlmock. Đây cũng là lớp lỗi mà sqlmock không bắt được.

func cond(key, value string) model.PromotionCondition {
	return model.PromotionCondition{ConditionKey: key, ConditionValue: value}
}

func TestPromotionConditions(t *testing.T) {
	// Thứ tư, 2026-08-19, 19:30.
	at := time.Date(2026, 8, 19, 19, 30, 0, 0, time.Local)
	if at.Weekday() != time.Wednesday {
		t.Fatalf("mốc thời gian kiểm sai: %v", at.Weekday())
	}

	base := PromotionContext{
		MemberID:    "m1",
		Amount:      100000,
		Quantity:    3,
		CategoryIDs: []string{"cat-a", "cat-b"},
		At:          at,
	}
	group := func() string { return "grp-vip" }

	tests := []struct {
		name  string
		conds []model.PromotionCondition
		want  bool
	}{
		{"không điều kiện thì luôn đúng", nil, true},

		{"đủ tiền tối thiểu", []model.PromotionCondition{cond(CondMinAmount, "50000")}, true},
		{"thiếu tiền tối thiểu", []model.PromotionCondition{cond(CondMinAmount, "200000")}, false},
		{"đúng bằng ngưỡng vẫn tính là đủ", []model.PromotionCondition{cond(CondMinAmount, "100000")}, true},

		{"đủ số lượng", []model.PromotionCondition{cond(CondMinQuantity, "3")}, true},
		{"thiếu số lượng", []model.PromotionCondition{cond(CondMinQuantity, "4")}, false},

		{"đúng nhóm hội viên", []model.PromotionCondition{cond(CondMemberGroup, `"grp-vip"`)}, true},
		{"nhóm nằm trong danh sách", []model.PromotionCondition{cond(CondMemberGroup, `["grp-a","grp-vip"]`)}, true},
		{"sai nhóm hội viên", []model.PromotionCondition{cond(CondMemberGroup, `"grp-thuong"`)}, false},

		{"đúng thứ trong tuần", []model.PromotionCondition{cond(CondDayOfWeek, "3")}, true},
		{"thứ nằm trong danh sách", []model.PromotionCondition{cond(CondDayOfWeek, "[0,3,6]")}, true},
		{"sai thứ", []model.PromotionCondition{cond(CondDayOfWeek, "[0,6]")}, false},

		{"trong khung giờ", []model.PromotionCondition{cond(CondTimeRange, `{"from":"18:00","to":"22:00"}`)}, true},
		{"ngoài khung giờ", []model.PromotionCondition{cond(CondTimeRange, `{"from":"08:00","to":"11:00"}`)}, false},
		{"khung giờ vắt qua nửa đêm", []model.PromotionCondition{cond(CondTimeRange, `{"from":"18:00","to":"02:00"}`)}, true},
		{"khung giờ có giây vẫn so đúng", []model.PromotionCondition{cond(CondTimeRange, `{"from":"18:00:00","to":"22:00:00"}`)}, true},

		{"đơn có món thuộc danh mục", []model.PromotionCondition{cond(CondProductCategory, `["cat-b"]`)}, true},
		{"đơn không có món thuộc danh mục", []model.PromotionCondition{cond(CondProductCategory, `["cat-z"]`)}, false},

		// Nhiều điều kiện là AND.
		{"tất cả điều kiện đúng", []model.PromotionCondition{
			cond(CondMinAmount, "50000"), cond(CondDayOfWeek, "3"),
		}, true},
		{"một điều kiện sai làm hỏng cả cụm", []model.PromotionCondition{
			cond(CondMinAmount, "50000"), cond(CondDayOfWeek, "0"),
		}, false},

		// Phòng thủ: dữ liệu hỏng không được biến thành "áp cho tất cả".
		{"khoá lạ thì không áp", []model.PromotionCondition{cond("nguoi_dep_trai", "true")}, false},
		{"jsonb hỏng thì không áp", []model.PromotionCondition{cond(CondMinAmount, "{lỗi")}, false},
		{"giá trị sai kiểu thì không áp", []model.PromotionCondition{cond(CondMinAmount, `"nhiều"`)}, false},
		{"khung giờ thiếu vế thì không áp", []model.PromotionCondition{cond(CondTimeRange, `{"from":"18:00"}`)}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := conditionsPass(tc.conds, base, group); got != tc.want {
				t.Errorf("được %v, mong %v", got, tc.want)
			}
		})
	}
}

func TestPromotionConditions_MemberGroupWithoutMember(t *testing.T) {
	// Đơn khách vãng lai: điều kiện theo nhóm hội viên phải trượt, không được
	// coi "không có nhóm" là khớp với mọi nhóm.
	pctx := PromotionContext{Amount: 100000, At: time.Now()}
	noGroup := func() string { return "" }
	conds := []model.PromotionCondition{cond(CondMemberGroup, `"grp-vip"`)}
	if conditionsPass(conds, pctx, noGroup) {
		t.Error("đơn không có hội viên vẫn khớp điều kiện nhóm")
	}
}

func reward(t string, v interface{}) model.PromotionReward {
	b, _ := json.Marshal(v)
	return model.PromotionReward{RewardType: t, RewardValue: string(b)}
}

func TestDiscountFromRewards(t *testing.T) {
	tests := []struct {
		name    string
		rewards []model.PromotionReward
		amount  int64
		want    int64
	}{
		{"giảm theo phần trăm", []model.PromotionReward{
			reward(RewardDiscountPercent, map[string]interface{}{"percent": 10}),
		}, 100000, 10000},

		{"giảm số tiền cố định", []model.PromotionReward{
			reward(RewardDiscountAmount, map[string]interface{}{"amount": 20000}),
		}, 100000, 20000},

		{"phần trăm bị chặn bởi mức trần", []model.PromotionReward{
			reward(RewardDiscountPercent, map[string]interface{}{"percent": 50, "max_discount": 30000}),
		}, 100000, 30000},

		{"trần cao hơn mức giảm thì không ảnh hưởng", []model.PromotionReward{
			reward(RewardDiscountPercent, map[string]interface{}{"percent": 10, "max_discount": 30000}),
		}, 100000, 10000},

		{"nhiều thưởng trong cùng khuyến mãi thì cộng dồn", []model.PromotionReward{
			reward(RewardDiscountPercent, map[string]interface{}{"percent": 10}),
			reward(RewardDiscountAmount, map[string]interface{}{"amount": 5000}),
		}, 100000, 15000},

		{"phần trăm trên 100 bị kẹp về 100", []model.PromotionReward{
			reward(RewardDiscountPercent, map[string]interface{}{"percent": 500}),
		}, 100000, 100000},

		{"loại thưởng không phải giảm giá bị bỏ qua", []model.PromotionReward{
			reward("bonus_points", map[string]interface{}{"amount": 50000}),
		}, 100000, 0},

		{"phần trăm âm bị bỏ qua", []model.PromotionReward{
			reward(RewardDiscountPercent, map[string]interface{}{"percent": -10}),
		}, 100000, 0},

		{"jsonb hỏng bị bỏ qua", []model.PromotionReward{
			{RewardType: RewardDiscountAmount, RewardValue: "{hỏng"},
		}, 100000, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := discountFromRewards(tc.rewards, tc.amount); got != tc.want {
				t.Errorf("được %d, mong %d", got, tc.want)
			}
		})
	}
}

func TestValidatePromotionRules(t *testing.T) {
	if err := ValidatePromotionRules([]string{CondMinAmount}, []string{RewardDiscountPercent}); err != nil {
		t.Errorf("khoá hợp lệ bị từ chối: %v", err)
	}
	if err := ValidatePromotionRules([]string{"min_amout"}, nil); err == nil {
		t.Error("khoá gõ sai phải bị từ chối")
	}
	if err := ValidatePromotionRules(nil, []string{"discount_percentage"}); err == nil {
		t.Error("loại thưởng gõ sai phải bị từ chối")
	}
}
