package remotedesk

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Một TCP listener giả làm TightVNC: gửi lời chào RFB rồi echo lại mọi thứ.
// Byte phải đi nguyên vẹn cả hai chiều, và đóng trình xem phải đóng luôn TCP.
func TestBridge_ChepHaiChieuVaDongCaHai(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	vncClosed := make(chan struct{})
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		c.Write([]byte("RFB 003.008\n"))
		io.Copy(c, c)
		close(vncClosed)
	}()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tcp, err := net.Dial("tcp", ln.Addr().String())
		require.NoError(t, err)
		ws, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		require.NoError(t, err)
		Bridge(ws, tcp)
	}))
	defer srv.Close()

	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	require.NoError(t, err)

	mt, data, err := ws.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, websocket.BinaryMessage, mt)
	assert.Equal(t, "RFB 003.008\n", string(data))

	require.NoError(t, ws.WriteMessage(websocket.BinaryMessage, []byte{1, 2, 3}))
	_, data, err = ws.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, []byte{1, 2, 3}, data)

	ws.Close()
	<-vncClosed
}
