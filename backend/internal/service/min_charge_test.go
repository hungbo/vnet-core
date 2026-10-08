package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/model"
)

func TestApMucToiThieu(t *testing.T) {
	hv := "m1"
	goi := "g1"
	phienLe := &model.MachineSession{MemberID: &hv}
	phienGoi := &model.MachineSession{MemberID: &hv, ComboID: &goi}
	phienKhach := &model.MachineSession{}

	cases := []struct {
		ten            string
		phien          *model.MachineSession
		tien, gia, muc int64
		mong           int64
	}{
		{"dưới 1 phút: nâng lên mức tối thiểu", phienLe, 0, 10000, 3000, 3000},
		{"chơi 10 phút (1.667₫): vẫn mức tối thiểu", phienLe, 1667, 10000, 3000, 3000},
		{"chơi 30 phút: tính theo phút, không cộng thêm", phienLe, 5000, 10000, 3000, 5000},
		{"tắt cài đặt", phienLe, 0, 10000, 0, 0},
		{"gói cước đã trả trước", phienGoi, 0, 10000, 3000, 0},
		{"máy chưa có giá", phienLe, 0, 0, 3000, 0},
		{"phiên không có hội viên", phienKhach, 0, 10000, 3000, 0},
	}
	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			assert.Equal(t, c.mong, apMucToiThieu(c.phien, c.tien, c.gia, c.muc))
		})
	}
}

func TestMucToiThieu_KhongGanThiBang0(t *testing.T) {
	db, _ := newMockDB(t)
	svc := NewSessionService(db, nil, NewAuditService(db))
	assert.Equal(t, int64(0), svc.mucToiThieu())
	svc.minCharge = func() int64 { return -5 }
	assert.Equal(t, int64(0), svc.mucToiThieu(), "giá trị âm coi như tắt")
}

// Đồng hồ máy trạm phải về 0 đúng phút máy chủ dừng phiên: 8.000₫/giờ với
// 188.000₫ (gồm 500₫ trả trước) là đúng 1410 phút, ở mọi phút đã chơi — kể cả
// khi tiền các phút đã chơi bị làm tròn lên và số dư còn lại có phần lẻ.
func TestMocHetTien_KhongHutPhutDoLamTron(t *testing.T) {
	batDau := time.Date(2026, 10, 3, 22, 9, 20, 0, time.UTC)
	phien := &model.MachineSession{StartedAt: batDau, PricePerHour: 8000}
	tong := int64(188000)
	for _, elapsed := range []int{0, 3, 4, 5, 7, 100, 1409} {
		daThu := (int64(elapsed)*8000 + 59) / 60
		if daThu < 500 {
			daThu = 500
		}
		tienPhut := (int64(elapsed)*8000 + 59) / 60
		mem := model.Member{Balance: tong - daThu}
		until := mocHetTien(phien, mem, elapsed, daThu-tienPhut)
		require.NotNil(t, until)
		assert.Equal(t, batDau.Add(1410*time.Minute), *until, "elapsed=%d", elapsed)
	}
}
