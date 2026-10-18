package service

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Magic packet đúng chuẩn: 6 byte 0xFF rồi MAC lặp 16 lần. Sai một byte là
// card mạng bỏ qua và máy không bật — không có lỗi nào báo về.
func TestMagicPacket(t *testing.T) {
	pkt, err := magicPacket("AA-BB-CC-DD-EE-01")
	require.NoError(t, err)
	require.Len(t, pkt, 102)
	assert.Equal(t, bytes.Repeat([]byte{0xFF}, 6), pkt[:6])
	mac := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01}
	for i := 0; i < 16; i++ {
		assert.Equal(t, mac, pkt[6+i*6:12+i*6])
	}

	_, err = magicPacket("không-phải-mac")
	assert.Error(t, err)
}

func TestWolTargets(t *testing.T) {
	assert.Equal(t, []string{"192.168.1.255:9", "255.255.255.255:9"}, wolTargets("192.168.1.37"))
	// Chưa có IP thì vẫn phát broadcast chung.
	assert.Equal(t, []string{"255.255.255.255:9"}, wolTargets(""))
}

// Gói phải thật sự rời khỏi socket: nghe ở một cổng cục bộ rồi gửi tới đó.
func TestSendMagicPacket_DenNoi(t *testing.T) {
	ln, err := net.ListenPacket("udp4", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	pkt, _ := magicPacket("aa:bb:cc:dd:ee:02")
	require.NoError(t, sendMagicPacket(pkt, []string{ln.LocalAddr().String()}))

	buf := make([]byte, 200)
	_ = ln.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := ln.ReadFrom(buf)
	require.NoError(t, err)
	assert.Equal(t, pkt, buf[:n])
}

func TestMachineWake_TuChoiKhiKhongTheBat(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))
	const mayThu = "6f1c2a7e-0d4b-4c9a-9f3e-1a2b3c4d5e6f"

	// Mã máy sai định dạng bị chặn trước khi chạm DB — không lộ lỗi SQL.
	_, err := svc.Wake("abc", "")
	assert.EqualError(t, err, "không tìm thấy máy")

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code", "status", "mac_address"}).
			AddRow(mayThu, "PC-01", "available", "aa:bb:cc:dd:ee:03"))
	_, err = svc.Wake(mayThu, "")
	assert.ErrorIs(t, err, ErrWakeAlreadyOn)

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE id = \$1`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code", "status", "mac_address"}).
			AddRow(mayThu, "PC-01", "offline", ""))
	_, err = svc.Wake(mayThu, "")
	assert.ErrorIs(t, err, ErrWakeNoMAC)
	assert.NoError(t, mock.ExpectationsWereMet())
}
