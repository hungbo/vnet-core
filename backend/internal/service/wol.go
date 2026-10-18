package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/vnet/core/internal/model"
	"gorm.io/gorm"
)

// Bật máy từ xa bằng Wake-on-LAN.
//
// Khác mọi lệnh điều khiển khác: máy đang TẮT thì không có WebSocket nào để
// gửi lệnh xuống. Máy chủ VNET nằm cùng mạng LAN với máy trạm, nên tự phát
// một "magic packet" UDP ra broadcast — card mạng của máy trạm (vẫn còn điện
// khi máy tắt) nhận ra địa chỉ MAC của mình trong gói và bật nguồn.
//
// Máy trạm phải bật Wake-on-LAN trong BIOS và trên card mạng; Windows cần tắt
// Fast Startup. Không bật được thì gói vẫn đi, chỉ là không có máy nào thức.

var (
	// ErrWakeNoMAC: máy chưa từng gửi nhịp tim nên chưa có địa chỉ MAC.
	ErrWakeNoMAC = errors.New("máy chưa báo địa chỉ MAC — cần bật máy và chạy phần mềm VNET ít nhất một lần")
	// ErrWakeAlreadyOn: máy đang gửi nhịp tim, tức là đang bật.
	ErrWakeAlreadyOn = errors.New("máy đang bật")
)

// wolPort: cổng 9 (discard) là cổng card mạng nghe magic packet phổ biến nhất.
const wolPort = 9

// magicPacket: 6 byte 0xFF rồi địa chỉ MAC lặp 16 lần — 102 byte.
func magicPacket(mac string) ([]byte, error) {
	hw, err := net.ParseMAC(strings.TrimSpace(mac))
	if err != nil || len(hw) != 6 {
		return nil, fmt.Errorf("địa chỉ MAC %q không hợp lệ", mac)
	}
	pkt := make([]byte, 0, 102)
	for i := 0; i < 6; i++ {
		pkt = append(pkt, 0xFF)
	}
	for i := 0; i < 16; i++ {
		pkt = append(pkt, hw...)
	}
	return pkt, nil
}

// wolTargets: broadcast của mạng con nơi máy trạm từng báo IP, cộng
// 255.255.255.255. Mạng con lấy theo /24 — gần như mọi quán net dùng
// 192.168.x.0/24. Gửi cả hai vì một số router/OS chặn 255.255.255.255 hoặc
// gửi nó ra card mạng sai khi máy chủ có nhiều card.
func wolTargets(lastIP string) []string {
	out := []string{}
	if ip := net.ParseIP(strings.TrimSpace(lastIP)).To4(); ip != nil {
		out = append(out, fmt.Sprintf("%d.%d.%d.255:%d", ip[0], ip[1], ip[2], wolPort))
	}
	return append(out, fmt.Sprintf("255.255.255.255:%d", wolPort))
}

// sendMagicPacket phát gói tới từng đích; chỉ lỗi khi KHÔNG đích nào gửi được.
func sendMagicPacket(pkt []byte, targets []string) error {
	lc := net.ListenConfig{Control: func(_, _ string, c syscall.RawConn) error {
		var serr error
		if err := c.Control(func(fd uintptr) { serr = batBroadcast(fd) }); err != nil {
			return err
		}
		return serr
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, err := lc.ListenPacket(ctx, "udp4", ":0")
	if err != nil {
		return fmt.Errorf("không mở được cổng gửi Wake-on-LAN: %w", err)
	}
	defer func() { _ = conn.Close() }()

	var lastErr error
	sent := 0
	for _, t := range targets {
		addr, err := net.ResolveUDPAddr("udp4", t)
		if err != nil {
			lastErr = err
			continue
		}
		if _, err := conn.WriteTo(pkt, addr); err != nil {
			lastErr = err
			continue
		}
		sent++
	}
	if sent == 0 {
		return fmt.Errorf("không gửi được gói Wake-on-LAN: %v", lastErr)
	}
	return nil
}

// WakeResult cho trang quản trị biết gói đã đi tới đâu.
type WakeResult struct {
	MachineCode string   `json:"machine_code"`
	MacAddress  string   `json:"mac_address"`
	Targets     []string `json:"targets"`
}

// Wake gửi magic packet tới một máy đang tắt.
func (s *MachineService) Wake(id, actorID string) (*WakeResult, error) {
	if !uuidHopLe.MatchString(id) {
		return nil, errors.New("không tìm thấy máy")
	}
	var m model.Machine
	if err := s.db.Where("id = ?", id).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("không tìm thấy máy")
		}
		return nil, err
	}
	if m.Status != "offline" {
		return nil, ErrWakeAlreadyOn
	}
	if strings.TrimSpace(m.MacAddress) == "" {
		return nil, ErrWakeNoMAC
	}
	pkt, err := magicPacket(m.MacAddress)
	if err != nil {
		return nil, err
	}
	targets := wolTargets(m.IPAddress)
	if err := sendMagicPacket(pkt, targets); err != nil {
		return nil, err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "remote_wake",
		EntityType: "machine",
		EntityID:   m.ID,
		UserID:     optionalUUID(actorID),
		Metadata: map[string]interface{}{
			"machine_code": m.MachineCode,
			"mac_address":  m.MacAddress,
			"targets":      targets,
		},
	})
	return &WakeResult{MachineCode: m.MachineCode, MacAddress: m.MacAddress, Targets: targets}, nil
}
