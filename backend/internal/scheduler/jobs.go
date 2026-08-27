package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/utils"
	"gorm.io/gorm"
)

// HeartbeatTimeout is how long a machine may stay silent before it is treated
// as offline. The agent reports every 15s, so three missed reports.
const HeartbeatTimeout = 45 * time.Second

// offlineSessionTimeout: máy mất tín hiệu (tắt máy, rút mạng) quá ngần này mà
// không đăng xuất thì tự đóng phiên. Rộng hơn HeartbeatTimeout (dùng để đánh
// dấu máy offline) để bỏ qua chớp mạng ngắn — đóng phiên là việc nặng hơn đổi
// trạng thái hiển thị.
const offlineSessionTimeout = 2 * time.Minute

// Bảng lịch sử phần cứng lớn nhanh nhất hệ thống: mỗi máy ghi một dòng cho mỗi
// lần báo cáo, tức 4 dòng/phút — một quán 50 máy sinh khoảng 288 nghìn dòng mỗi
// ngày. Không có gì dọn thì nó chỉ có một hướng đi.
//
// Xoá theo lô: một câu DELETE trên vài triệu dòng giữ khoá lâu và thổi phồng
// bảng. Mỗi lượt chạy cũng có trần, để lần dọn đầu tiên trên bảng đã phình to
// không chiếm database hàng chục phút.
const (
	hardwareHistoryBatch     = 5000
	hardwareHistoryMaxPerRun = 500000
)

// idempotencyKeyTTL là thời gian một khoá chống-nạp-trùng còn hiệu lực.
const idempotencyKeyTTL = 24 * time.Hour

// Register wires the standard job set. Every job here replaces a business rule
// that was previously written down but never executed.
func Register(s *Scheduler, db *gorm.DB, wsHub *hub.Hub, sessions *service.SessionService, curfew *service.CurfewService, members *service.MemberService, hardwareHistoryDays int) {
	s.Add(Job{
		Name:     "hardware:prune-history",
		Interval: time.Hour,
		// Chạy ngay lúc khởi động: mỗi giờ một lần mà máy chủ khởi động lại
		// thường xuyên hơn thế thì tác vụ này không bao giờ tới lượt.
		RunAtStart: true,
		Run: func(ctx context.Context) error {
			return pruneHardwareHistory(db, hardwareHistoryDays)
		},
	})
	s.Add(Job{
		Name:     "idempotency:prune",
		Interval: time.Hour,
		Run:      func(ctx context.Context) error { return pruneIdempotencyKeys(db) },
	})
	s.Add(Job{
		Name:     "machines:mark-offline",
		Interval: 30 * time.Second,
		Run:      func(ctx context.Context) error { return markStaleMachinesOffline(db, wsHub) },
	})
	s.Add(Job{
		Name:     "bookings:expire",
		Interval: time.Minute,
		Run:      func(ctx context.Context) error { return expireBookings(db) },
	})
	s.Add(Job{
		Name:     "sessions:enforce-limits",
		Interval: time.Minute,
		Run:      func(ctx context.Context) error { return enforceSessionLimits(db, sessions, wsHub) },
	})
	s.Add(Job{
		Name:     "curfew:enforce",
		Interval: time.Minute,
		Run:      func(ctx context.Context) error { return curfew.EnforceNow(sessions, wsHub) },
	})
	s.Add(Job{
		Name:     "members:refresh-tier",
		Interval: 15 * time.Minute,
		Run: func(ctx context.Context) error {
			// Cùng một hàm với nút "Xếp lại hạng" trong admin — hai bản sao của
			// cùng quy tắc là hai chỗ để lệch nhau.
			moved, err := members.RefreshTiers()
			if moved > 0 {
				log.Printf("[scheduler] moved %d members between tiers", moved)
			}
			return err
		},
	})
}

// markStaleMachinesOffline flips machines that stopped reporting. LastHeartbeat
// was written on every agent report and then never read by anything, so a
// machine that lost power stayed "available" forever.
func markStaleMachinesOffline(db *gorm.DB, wsHub *hub.Hub) error {
	cutoff := time.Now().Add(-HeartbeatTimeout)

	var stale []model.Machine
	if err := db.Where("status <> ? AND last_heartbeat IS NOT NULL AND last_heartbeat < ?", "offline", cutoff).
		Find(&stale).Error; err != nil {
		return err
	}
	if len(stale) == 0 {
		return nil
	}

	ids := make([]string, 0, len(stale))
	for _, m := range stale {
		ids = append(ids, m.ID)
	}
	if err := db.Model(&model.Machine{}).Where("id IN ?", ids).Update("status", "offline").Error; err != nil {
		return err
	}

	for _, m := range stale {
		// Trạng thái của cả dàn máy là thông tin vận hành, chỉ quầy cần thấy.
		wsHub.BroadcastToType(hub.Event{
			Type: "machine:status",
			Data: map[string]interface{}{
				"machine_id":   m.ID,
				"machine_code": m.MachineCode,
				"status":       "offline",
			},
		}, hub.ClientTypeAdmin)
	}
	log.Printf("[scheduler] marked %d machines offline", len(stale))
	return nil
}

// expireBookings releases slots nobody showed up for. Without this an expired
// booking blocked its machine's time range permanently.
func expireBookings(db *gorm.DB) error {
	// A grace period after the slot start before the seat is given away.
	const grace = 15 * time.Minute
	cutoff := utils.VietnamTime().Add(-grace)

	res := db.Model(&model.MachineBooking{}).
		Where("status = ? AND booked_from < ?", "pending", cutoff).
		Update("status", "no_show")
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		log.Printf("[scheduler] expired %d no-show bookings", res.RowsAffected)
	}

	// Checked-in bookings whose slot has passed are finished, which also frees
	// the range for future bookings.
	done := db.Model(&model.MachineBooking{}).
		Where("status = ? AND booked_to < ?", "checked_in", utils.VietnamTime()).
		Update("status", "completed")
	if done.Error != nil {
		return done.Error
	}
	return nil
}

// enforceSessionLimits trừ tiền cho từng phiên đang chạy rồi đóng những phiên đã
// tiêu hết thứ chúng được bán: hết khung giờ, hết phút trả trước, hoặc hết tiền.
//
// Trừ tiền và quyết định dừng nằm trong CÙNG một tác vụ, không tách làm hai:
// hai tác vụ cùng chạy mỗi phút trên cùng tập phiên là công thức để trừ tiền hai
// lần.
func enforceSessionLimits(db *gorm.DB, sessions *service.SessionService, wsHub *hub.Hub) error {
	now := utils.VietnamTime()

	// Đóng trước những phiên còn sót sau khi máy reboot mà không ai ngồi lại.
	// Đường đăng nhập đã lo trường hợp có người ngồi vào; đây lo trường hợp máy
	// bật lại rồi bỏ đó, để đồng hồ không chạy hoài trên một màn hình khoá.
	if n, err := sessions.EndSessionsStaleAfterReboot(""); err != nil {
		log.Printf("[scheduler] đóng phiên sót sau reboot: %v", err)
	} else if n > 0 {
		log.Printf("[scheduler] đóng %d phiên sót sau khi máy reboot", n)
	}

	// Máy tắt/rút mạng mà không đăng xuất: mất tín hiệu quá ngưỡng thì đóng
	// phiên, chốt sổ tại nhịp tim cuối để không tính phần máy đã tắt.
	if n, err := sessions.EndSessionsOnOfflineMachines(offlineSessionTimeout); err != nil {
		log.Printf("[scheduler] đóng phiên máy mất tín hiệu: %v", err)
	} else if n > 0 {
		log.Printf("[scheduler] đóng %d phiên do máy mất tín hiệu", n)
	}

	var active []model.MachineSession
	if err := db.Where("is_active = ?", true).Find(&active).Error; err != nil {
		return err
	}

	for _, sess := range active {
		// Trừ tiền trước, rồi mới xét dừng: phút cuối cùng khách ngồi vẫn phải
		// được tính. ChargeTick khoá dòng phiên nên không đụng nhau với nhân
		// viên đang bấm "Trả máy".
		tick, err := sessions.ChargeTick(sess.ID)
		if err != nil {
			log.Printf("[scheduler] charge %s: %v", sess.ID, err)
			continue
		}

		// Gói khung giờ có TotalMinutes = 0 nên RemainingMinutes = 0. Không loại
		// nó ra khỏi nhánh minutes_exhausted thì điều kiện "đã chơi >= 0 phút"
		// đúng ngay từ lượt đầu và khách vừa mua gói bị đá ra sau chưa đầy một
		// phút.
		isFixedSlot := sess.ComboID != nil && sess.ComboType == "fixed_slot"

		reason := ""
		switch {
		case sess.SlotEnd != nil && now.After(*sess.SlotEnd):
			reason = "slot_ended"
		case !isFixedSlot && sess.RemainingMinutes != nil && *sess.RemainingMinutes > 0 &&
			int(now.Sub(sess.StartedAt).Minutes()) >= *sess.RemainingMinutes:
			reason = "minutes_exhausted"
		case tick.OutOfCash:
			reason = "out_of_balance"
		}
		if reason == "" {
			continue
		}

		if _, err := sessions.EndSession(sess.ID); err != nil {
			log.Printf("[scheduler] auto-end %s: %v", sess.ID, err)
			continue
		}
		// Thiếu machine_code thì thông báo cho nhân viên chỉ hiện UUID — họ cần
		// biết MÁY NÀO vừa được giải phóng, không phải mã phiên.
		machineCode := ""
		db.Model(&model.Machine{}).Where("id = ?", sess.MachineID).
			Pluck("machine_code", &machineCode)

		wsHub.SendToAdminsAndMachine(machineCode, hub.Event{
			Type: "session:auto-ended",
			Data: map[string]interface{}{
				"session_id":   sess.ID,
				"machine_id":   sess.MachineID,
				"machine_code": machineCode,
				"reason":       reason,
			},
		})

		// Hết tiền thì khoá luôn màn hình: đóng phiên mà để máy mở là khách vẫn
		// dùng máy miễn phí cho tới khi có người đi qua. Dùng đúng lệnh mà nút
		// "Khoá" ở trang Máy đang gửi.
		if reason == "out_of_balance" && machineCode != "" {
			_ = wsHub.SendToMachine(machineCode, hub.Event{
				Type: "remote:lock",
				Data: map[string]interface{}{
					"machine_code": machineCode,
					"action":       "lock",
					"payload":      map[string]interface{}{"reason": "Hết số dư — vui lòng nạp thêm tại quầy"},
				},
			})
		}
		log.Printf("[scheduler] auto-ended session %s (%s)", sess.ID, reason)
	}
	return nil
}

// pruneHardwareHistory xoá số đo cũ hơn số ngày được cấu hình.
//
// days <= 0 nghĩa là giữ lại tất cả — lối thoát cho ai muốn tự quản lý bằng tay,
// và cũng là cách tắt tác vụ này mà không phải sửa code.
// pruneIdempotencyKeys dọn khoá chống-nạp-trùng đã quá hạn.
//
// Khoá chỉ cần sống đủ lâu để bắt được một lần gửi lại: cú bấm đúp, trình duyệt
// thử lại sau khi mất mạng, nhân viên bấm lại vì tưởng hỏng. Một ngày là thừa
// rộng cho mọi trường hợp đó, và giữ lâu hơn chỉ làm bảng phình ra.
func pruneIdempotencyKeys(db *gorm.DB) error {
	res := db.Exec(`DELETE FROM idempotency_keys WHERE created_at < ?`, time.Now().Add(-idempotencyKeyTTL))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		log.Printf("[scheduler] đã xoá %d khoá idempotency quá hạn", res.RowsAffected)
	}
	return nil
}

func pruneHardwareHistory(db *gorm.DB, days int) error {
	if days <= 0 {
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	var total int64

	for total < hardwareHistoryMaxPerRun {
		// PostgreSQL không cho DELETE ... LIMIT, nên chọn id trước rồi xoá theo
		// id — cũng là cách để mỗi lô là một giao dịch ngắn.
		res := db.Exec(`DELETE FROM machine_hardware_snapshots WHERE id IN (
			SELECT id FROM machine_hardware_snapshots WHERE created_at < ? LIMIT ?)`,
			cutoff, hardwareHistoryBatch)
		if res.Error != nil {
			return res.Error
		}
		total += res.RowsAffected
		if res.RowsAffected < hardwareHistoryBatch {
			break
		}
	}

	if total > 0 {
		log.Printf("[scheduler] đã xoá %d dòng lịch sử phần cứng cũ hơn %d ngày", total, days)
	}
	return nil
}
