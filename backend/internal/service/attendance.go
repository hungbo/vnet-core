package service

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
)

// Điểm danh hằng ngày.
//
// Hội viên vào quán điểm danh một lần mỗi ngày và nhận thưởng vào số dư khuyến
// mãi. Điểm danh liên tiếp nhiều ngày thì mốc chuỗi cho thêm thưởng.
//
// Tiền thưởng vào bonus_balance chứ không vào balance: đó là tiền quán tặng,
// không phải tiền khách nạp — trộn hai loại sẽ làm sai sổ khi hoàn tiền.

type AttendanceService struct {
	db    *gorm.DB
	audit *AuditService
}

func NewAttendanceService(db *gorm.DB, audit *AuditService) *AttendanceService {
	return &AttendanceService{db: db, audit: audit}
}

// Cấu hình nằm ở nhóm cài đặt "attendance". Để 0 thì điểm danh vẫn ghi nhận
// nhưng không tặng gì — quán không muốn tặng tiền vẫn dùng được tính năng.
const (
	attendanceGroup       = "attendance"
	keyDailyBonus         = "daily_bonus"
	keyStreakBonus        = "streak_bonus"
	keyStreakEvery        = "streak_every"
	defaultStreakEveryDay = 7
)

type attendanceConfig struct {
	DailyBonus  int64
	StreakBonus int64
	StreakEvery int
}

func (s *AttendanceService) config() attendanceConfig {
	cfg := attendanceConfig{StreakEvery: defaultStreakEveryDay}
	var rows []model.SystemSetting
	if err := s.db.Where("group_name = ?", attendanceGroup).Find(&rows).Error; err != nil {
		return cfg
	}
	for _, r := range rows {
		v := strings.TrimSpace(decodeSettingValue(r.Value))
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			continue
		}
		switch r.Key {
		case keyDailyBonus:
			cfg.DailyBonus = n
		case keyStreakBonus:
			cfg.StreakBonus = n
		case keyStreakEvery:
			if n > 0 {
				cfg.StreakEvery = int(n)
			}
		}
	}
	return cfg
}

type CheckinResult struct {
	Date          string `json:"date"`
	StreakDays    int    `json:"streak_days"`
	RewardAmount  int64  `json:"reward_amount"`
	StreakReached bool   `json:"streak_reached"`
	BonusAfter    int64  `json:"bonus_after"`
}

var ErrAlreadyCheckedIn = errors.New("hôm nay đã điểm danh rồi")

// Checkin ghi nhận một lượt điểm danh cho ngày hôm nay.
func (s *AttendanceService) Checkin(memberID, actorID string) (*CheckinResult, error) {
	if memberID == "" {
		return nil, errors.New("thiếu mã hội viên")
	}
	// Quán tắt điểm danh ở trang Cài đặt thì chặn ngay tại đây, không chỉ ẩn nút
	// trên máy trạm: ẩn nút không ngăn được ai gọi thẳng API.
	if !FeatureEnabled(s.db, "attendance_enabled") {
		return nil, errors.New("quán đang tắt tính năng điểm danh")
	}

	cfg := s.config()
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var res CheckinResult
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var member model.Member
		if err := tx.First(&member, "id = ?", memberID).Error; err != nil {
			return errors.New("không tìm thấy hội viên")
		}

		// Chuỗi ngày: nếu hôm qua có điểm danh thì nối tiếp, không thì về 1.
		var yesterday model.MemberAttendance
		streak := 1
		if err := tx.Where("member_id = ? AND checkin_date = ?", memberID,
			today.AddDate(0, 0, -1)).First(&yesterday).Error; err == nil {
			streak = yesterday.StreakDays + 1
		}

		reward := cfg.DailyBonus
		streakReached := cfg.StreakEvery > 0 && streak%cfg.StreakEvery == 0
		if streakReached {
			reward += cfg.StreakBonus
		}

		record := model.MemberAttendance{
			MemberID:      memberID,
			CheckinDate:   today,
			CheckinAt:     now,
			StreakDays:    streak,
			RewardAmount:  reward,
			RewardClaimed: reward > 0,
		}
		// Ràng buộc duy nhất ở database là thứ chặn điểm danh hai lần, kể cả
		// khi hai request tới cùng lúc.
		if err := tx.Create(&record).Error; err != nil {
			if isUniqueViolation(err) {
				return ErrAlreadyCheckedIn
			}
			return err
		}

		bonusAfter := member.BonusBalance
		if reward > 0 {
			bonusAfter += reward
			trans := model.MemberTransaction{
				MemberID:        memberID,
				TransactionType: "attendance_bonus",
				Amount:          reward,
				BalanceBefore:   member.Balance,
				BalanceAfter:    member.Balance,
				BonusBefore:     member.BonusBalance,
				BonusAfter:      bonusAfter,
				ReferenceID:     &record.ID,
				Description:     "Thưởng điểm danh ngày " + today.Format("02/01/2006"),
			}
			if actorID != "" {
				trans.CreatedBy = &actorID
			}
			if err := tx.Create(&trans).Error; err != nil {
				return err
			}
			if err := tx.Model(&member).Update("bonus_balance", bonusAfter).Error; err != nil {
				return err
			}
		}

		res = CheckinResult{
			Date: today.Format("2006-01-02"), StreakDays: streak,
			RewardAmount: reward, StreakReached: streakReached, BonusAfter: bonusAfter,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.audit.Log(&LogAuditRequest{
		Action: "attendance_checkin", EntityType: "member", EntityID: memberID,
		UserID: optionalUUID(actorID),
		Metadata: map[string]interface{}{
			"date": res.Date, "streak": res.StreakDays, "reward": res.RewardAmount,
		},
	})
	return &res, nil
}

type AttendanceStatus struct {
	CheckedInToday bool   `json:"checked_in_today"`
	StreakDays     int    `json:"streak_days"`
	NextReward     int64  `json:"next_reward"`
	StreakEvery    int    `json:"streak_every"`
	LastCheckin    string `json:"last_checkin,omitempty"`
}

// Status cho biết hôm nay đã điểm danh chưa và chuỗi đang là bao nhiêu.
func (s *AttendanceService) Status(memberID string) (*AttendanceStatus, error) {
	if !FeatureEnabled(s.db, "attendance_enabled") {
		return nil, errors.New("quán đang tắt tính năng điểm danh")
	}
	cfg := s.config()
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	st := &AttendanceStatus{StreakEvery: cfg.StreakEvery, NextReward: cfg.DailyBonus}

	var last model.MemberAttendance
	err := s.db.Where("member_id = ?", memberID).
		Order("checkin_date desc").First(&last).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return st, nil
		}
		return nil, err
	}

	st.LastCheckin = last.CheckinDate.Format("2006-01-02")
	switch {
	case sameDate(last.CheckinDate, today):
		st.CheckedInToday = true
		st.StreakDays = last.StreakDays
	case sameDate(last.CheckinDate, today.AddDate(0, 0, -1)):
		// Điểm danh hôm nay sẽ nối tiếp chuỗi.
		st.StreakDays = last.StreakDays
	default:
		// Đứt chuỗi: hôm nay điểm danh sẽ bắt đầu lại từ 1.
		st.StreakDays = 0
	}

	next := st.StreakDays + 1
	if cfg.StreakEvery > 0 && next%cfg.StreakEvery == 0 {
		st.NextReward += cfg.StreakBonus
	}
	return st, nil
}

type AttendanceListRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	MemberID string `form:"member_id"`
	Date     string `form:"date"`
}

type AttendanceRow struct {
	model.MemberAttendance
	MemberName     string `json:"member_name"`
	MemberUsername string `json:"member_username"`
}

func (s *AttendanceService) List(req *AttendanceListRequest) (*pagination.Result, error) {
	page, size := req.Page, req.PageSize
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}

	q := s.db.Model(&model.MemberAttendance{})
	if req.MemberID != "" {
		q = q.Where("member_id = ?", req.MemberID)
	}
	if req.Date != "" {
		d, err := time.Parse("2006-01-02", req.Date)
		if err != nil {
			return nil, errors.New("ngày sai định dạng, cần YYYY-MM-DD")
		}
		q = q.Where("checkin_date = ?", d)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []model.MemberAttendance
	if err := q.Order("checkin_at desc").Offset((page - 1) * size).Limit(size).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	out := make([]AttendanceRow, 0, len(rows))
	if len(rows) > 0 {
		ids := make([]string, 0, len(rows))
		for _, r := range rows {
			ids = append(ids, r.MemberID)
		}
		var members []struct {
			ID       string
			FullName string
			Username string
		}
		s.db.Model(&model.Member{}).Where("id IN ?", ids).Find(&members)
		byID := map[string]struct {
			ID       string
			FullName string
			Username string
		}{}
		for _, m := range members {
			byID[m.ID] = m
		}
		for _, r := range rows {
			row := AttendanceRow{MemberAttendance: r}
			if m, ok := byID[r.MemberID]; ok {
				row.MemberName, row.MemberUsername = m.FullName, m.Username
			}
			out = append(out, row)
		}
	}

	return &pagination.Result{Items: out, Total: total, Page: page, PageSize: size}, nil
}

// sameDate so hai mốc theo NGÀY LỊCH, không theo thời điểm.
//
// time.Equal so instant: cột date của PostgreSQL được driver trả về là nửa đêm
// THEO UTC, còn "hôm nay" tính ở máy chủ là nửa đêm theo giờ địa phương. Hai
// giá trị cùng chỉ một ngày nhưng khác instant, nên Equal luôn sai — và chuỗi
// ngày bị báo về 0 dù hôm qua vẫn điểm danh.
func sameDate(a, b time.Time) bool {
	return a.Format("2006-01-02") == b.Format("2006-01-02")
}

// isUniqueViolation nhận ra lỗi vỡ ràng buộc duy nhất của PostgreSQL (SQLSTATE
// 23505) mà không cần phụ thuộc trực tiếp vào driver.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") ||
		strings.Contains(strings.ToLower(msg), "duplicate key value")
}
