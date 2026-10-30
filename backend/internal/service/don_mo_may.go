package service

import (
	"log"
	"sync"
	"time"

	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/model"
	"gorm.io/gorm"
)

// Đơn món khách đã gọi mà quầy chưa thu tiền: "pending" là khách đang gọi, quầy
// chưa duyệt; "confirmed" là đã làm món nhưng chưa thanh toán. Đơn nạp tiền
// không tính — đó là tiền khách đưa vào, không phải tiền khách còn nợ.
var trangThaiDonMo = []string{"pending", "confirmed"}

// DonMoMay là phần đơn chưa thu tiền của một máy: trang Máy trạm và Bảng điều
// khiển tô màu khác cho máy đó.
type DonMoMay struct {
	MachineID    string `json:"machine_id"`
	PendingCount int    `json:"pending_count"`
	UnpaidCount  int    `json:"unpaid_count"`
	Amount       int64  `json:"amount"`
}

// DonMoTheoMay gom đơn chưa thu tiền theo máy, cho GET /machines/open-orders.
func (s *MachineService) DonMoTheoMay() ([]DonMoMay, error) {
	return donMoTheoMay(s.db)
}

// donMoVaMay đọc mọi đơn chưa thu tiền và máy mà từng đơn được tính cho
// (order_id → machine_id). Đơn không tính cho máy nào thì không có trong map.
//
// Đơn đi theo KHÁCH: đơn của hội viên tính cho máy họ ngồi GẦN NHẤT — máy đang
// ngồi, hoặc máy vừa bỏ về. Gọi trên máy cũ rồi đổi máy, hay gọi ở quầy, vẫn hiện
// đúng chỗ khách ngồi; khách trả máy hay tắt máy thì máy đó còn đỏ để quầy thấy.
// Đơn khách vãng lai, và hội viên chưa ngồi máy nào, thì tính cho máy đã gọi món.
func donMoVaMay(db *gorm.DB) ([]model.Order, map[string]string, error) {
	var dons []model.Order
	if err := db.Select("id", "status", "member_id", "machine_id", "final_amount").
		Where("status IN ? AND order_type <> ?", trangThaiDonMo, model.OrderTypeTopup).
		Find(&dons).Error; err != nil {
		return nil, nil, err
	}
	if len(dons) == 0 {
		return nil, map[string]string{}, nil
	}

	coDon := map[string]bool{}
	var hoiVien []string
	for _, d := range dons {
		if d.MemberID != nil && !coDon[*d.MemberID] {
			coDon[*d.MemberID] = true
			hoiVien = append(hoiVien, *d.MemberID)
		}
	}
	// Phiên đang chạy luôn là phiên mới nhất của hội viên, nên một câu lo cả khách
	// đang ngồi lẫn khách vừa rời máy.
	ganNhat := map[string]string{} // member_id → machine_id
	if len(hoiVien) > 0 {
		var rows []struct{ MemberID, MachineID string }
		if err := db.Model(&model.MachineSession{}).
			Select("DISTINCT ON (member_id) member_id, machine_id").
			Where("member_id IN ?", hoiVien).
			Order("member_id, started_at DESC").
			Scan(&rows).Error; err != nil {
			return nil, nil, err
		}
		for _, r := range rows {
			ganNhat[r.MemberID] = r.MachineID
		}
	}

	mayCuaDon := map[string]string{}
	for _, d := range dons {
		may := ""
		if d.MemberID != nil {
			may = ganNhat[*d.MemberID]
		}
		if may == "" && d.MachineID != nil {
			may = *d.MachineID
		}
		if may != "" { // rỗng: gọi ở quầy, khách chưa ngồi máy nào
			mayCuaDon[d.ID] = may
		}
	}
	return dons, mayCuaDon, nil
}

// donMoTheoMay gom đơn chưa thu tiền theo máy, máy tính theo donMoVaMay.
func donMoTheoMay(db *gorm.DB) ([]DonMoMay, error) {
	dons, mayCuaDon, err := donMoVaMay(db)
	if err != nil {
		return nil, err
	}

	theoMay := map[string]*DonMoMay{}
	var thuTu []string
	for _, d := range dons {
		may := mayCuaDon[d.ID]
		if may == "" {
			continue // không có máy để tô
		}
		m := theoMay[may]
		if m == nil {
			m = &DonMoMay{MachineID: may}
			theoMay[may] = m
			thuTu = append(thuTu, may)
		}
		if d.Status == "pending" {
			m.PendingCount++
		} else {
			m.UnpaidCount++
		}
		m.Amount += d.FinalAmount
	}

	out := make([]DonMoMay, 0, len(thuTu))
	for _, id := range thuTu {
		out = append(out, *theoMay[id])
	}
	return out, nil
}

// donMoCuaMay trả ID các đơn chưa thu tiền đang tính cho một máy — đúng những đơn
// làm máy đó đỏ, nên hộp "Đơn đang gọi" mở được cả khi khách đã rời máy hay máy đã
// tắt (không còn phiên, không còn hội viên để tra theo).
func donMoCuaMay(db *gorm.DB, machineID string) ([]string, error) {
	dons, mayCuaDon, err := donMoVaMay(db)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for _, d := range dons {
		if mayCuaDon[d.ID] == machineID {
			ids = append(ids, d.ID)
		}
	}
	return ids, nil
}

// donTheoKhach ghi máy hội viên vừa ngồi vào các đơn chưa thu tiền của họ: khách
// đổi máy (quầy đổi hộ, hay tự đăng xuất rồi đăng nhập máy khác) thì cột Máy ở
// trang Đơn hàng đi theo khách, không nằm lại máy cũ. Đơn gọi ở quầy lúc khách
// chưa ngồi máy nào cũng nhận máy ở đây.
func donTheoKhach(tx *gorm.DB, memberID, machineID string) error {
	return tx.Model(&model.Order{}).
		Where("member_id = ? AND status IN ? AND order_type <> ?", memberID, trangThaiDonMo, model.OrderTypeTopup).
		Where("machine_id IS NULL OR machine_id <> ?", machineID).
		Update("machine_id", machineID).Error
}

// baoDonMoTrongVong: mỗi phiên báo động nhiều nhất một lần trong quãng này. Máy
// tắt thì dịch vụ nền, giao diện và cửa sổ phụ cùng rớt kết nối, rồi chừng hai
// phút sau tác vụ nền đóng phiên vì mất tín hiệu — quầy chỉ cần nghe MỘT lần.
const baoDonMoTrongVong = 5 * time.Minute

var (
	daBaoDonMoMu sync.Mutex
	daBaoDonMo   = map[string]time.Time{} // session_id → lúc báo
)

// danhDauBaoDonMo ghi nhận lần báo cho phiên; false nếu phiên vừa được báo.
func danhDauBaoDonMo(sessionID string, now time.Time) bool {
	daBaoDonMoMu.Lock()
	defer daBaoDonMoMu.Unlock()
	if t, ok := daBaoDonMo[sessionID]; ok && now.Sub(t) < baoDonMoTrongVong {
		return false
	}
	for id, t := range daBaoDonMo {
		if now.Sub(t) >= baoDonMoTrongVong {
			delete(daBaoDonMo, id)
		}
	}
	daBaoDonMo[sessionID] = now
	return true
}

// donMoCuaKhach đếm đơn chưa thu tiền của khách vừa rời máy: mọi đơn của hội
// viên đó, cộng đơn khách vãng lai gọi trên chính máy này.
func donMoCuaKhach(db *gorm.DB, memberID, machineID string) (soDon, dangGoi int, tong int64, err error) {
	var r struct {
		SoDon   int
		DangGoi int
		Tong    int64
	}
	q := db.Model(&model.Order{}).
		Select("COUNT(*) AS so_don, COUNT(*) FILTER (WHERE status = 'pending') AS dang_goi, COALESCE(SUM(final_amount), 0) AS tong").
		Where("status IN ? AND order_type <> ?", trangThaiDonMo, model.OrderTypeTopup)
	if memberID != "" {
		q = q.Where("member_id = ? OR (member_id IS NULL AND machine_id = ?)", memberID, machineID)
	} else {
		q = q.Where("member_id IS NULL AND machine_id = ?", machineID)
	}
	err = q.Scan(&r).Error
	return r.SoDon, r.DangGoi, r.Tong, err
}

// baoDonMoKhiRoiMay phát "order:unpaid-left" cho quầy khi khách rời máy (trả
// máy, hết tiền, tắt máy) mà còn đơn chưa thu tiền. Trang quản trị kêu báo động
// để nhân viên kịp gọi khách lại trước khi họ ra khỏi quán.
//
// lyDo: "ended" — phiên vừa đóng; "offline" — máy tắt ngang hoặc mất kết nối.
func baoDonMoKhiRoiMay(db *gorm.DB, h *hub.Hub, sessionID, memberID string, machine *model.Machine, lyDo string) {
	if h == nil {
		return
	}
	soDon, dangGoi, tong, err := donMoCuaKhach(db, memberID, machine.ID)
	if err != nil {
		log.Printf("[orders] không kiểm được đơn chưa trả của máy %s: %v", machine.MachineCode, err)
		return
	}
	if soDon == 0 || !danhDauBaoDonMo(sessionID, time.Now()) {
		return
	}
	tenKhach := ""
	if memberID != "" {
		var m model.Member
		if db.Select("full_name", "username").Where("id = ?", memberID).First(&m).Error == nil {
			tenKhach = tenHoiVien(&m)
		}
	}
	h.BroadcastToType(hub.Event{
		Type: "order:unpaid-left",
		Data: map[string]interface{}{
			"machine_id":    machine.ID,
			"machine_code":  machine.MachineCode,
			"member_name":   tenKhach,
			"reason":        lyDo,
			"order_count":   soDon,
			"pending_count": dangGoi,
			"amount":        tong,
		},
	}, hub.ClientTypeAdmin)
}

// choMayNoiLai: máy trạm rớt WebSocket thì 5 giây sau tự nối lại (client/src/ws.go),
// và giao diện nối lại mỗi lần đăng nhập. Hết quãng này mà mã máy không còn kết
// nối nào — kể cả kết nối 24/7 của dịch vụ nền — thì coi như máy đã tắt.
var choMayNoiLai = 15 * time.Second

// TheoDoiMayTatKhiConDon báo động khi máy đang có khách bị TẮT NGANG (không trả
// máy) mà khách còn đơn chưa thu tiền.
//
// Không có cái này thì tín hiệu duy nhất là tác vụ nền đóng phiên vì mất nhịp
// tim — hơn hai phút sau, lúc đó khách đã ra khỏi quán.
func TheoDoiMayTatKhiConDon(db *gorm.DB, h *hub.Hub) {
	h.OnDisconnect(func(c *hub.Client) {
		ma := c.MachineCode()
		if ma == "" {
			return
		}
		time.AfterFunc(choMayNoiLai, func() {
			if h.IsMachineOnline(ma) {
				return
			}
			var may model.Machine
			if db.Select("id", "machine_code").Where("machine_code = ?", ma).First(&may).Error != nil {
				return
			}
			var phien model.MachineSession
			if db.Select("id", "member_id").Where("machine_id = ? AND is_active = ?", may.ID, true).
				First(&phien).Error != nil {
				return // máy không có khách: tắt máy trống là chuyện thường
			}
			memberID := ""
			if phien.MemberID != nil {
				memberID = *phien.MemberID
			}
			baoDonMoKhiRoiMay(db, h, phien.ID, memberID, &may, "offline")
		})
	})
}
