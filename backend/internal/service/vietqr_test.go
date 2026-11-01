package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCRC16CCITT_GiaTriKiemChuan(t *testing.T) {
	assert.Equal(t, uint16(0x29B1), crc16CCITT([]byte("123456789")))
}

func TestVietQRPayload_CauTruc(t *testing.T) {
	p := vietQRPayload("970422", "0123456789", 50000, "VN7K2Q9M")
	want := "000201" + "010212" +
		"3854" + "0010A000000727" + "0124" + "0006970422" + "01100123456789" + "0208QRIBFTTA" +
		"5303704" + "540550000" + "5802VN" + "6212" + "0808VN7K2Q9M" + "6304"
	require.True(t, strings.HasPrefix(p, want), p)
	assert.Len(t, p, len(want)+4)
	assert.Equal(t, p[len(p)-4:], strings.ToUpper(p[len(p)-4:]))
}

func TestQRPNGDataURL(t *testing.T) {
	url, err := qrPNGDataURL(vietQRPayload("970436", "1234", 10000, "VNABCDEF"), 256)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(url, "data:image/png;base64,"))
}
