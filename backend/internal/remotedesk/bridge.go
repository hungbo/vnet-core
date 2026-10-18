// Package remotedesk nối trình xem noVNC trên trang quản trị với TightVNC trên
// máy trạm: WebSocket (binary frame) ⇄ TCP cổng 5900, đúng vai trò websockify.
// Cầu nối chỉ chép byte, không đọc giao thức RFB.
package remotedesk

import (
	"net"

	"github.com/gorilla/websocket"
)

// Port là cổng TightVNC Server mặc định trên máy trạm.
const Port = "5900"

// Bridge chép byte hai chiều cho tới khi một bên đóng, rồi đóng cả hai.
func Bridge(ws *websocket.Conn, tcp net.Conn) {
	done := make(chan struct{}, 2)

	// TCP → WebSocket. Đây là goroutine DUY NHẤT ghi vào ws.
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := tcp.Read(buf)
			if n > 0 {
				if ws.WriteMessage(websocket.BinaryMessage, buf[:n]) != nil {
					break
				}
			}
			if err != nil {
				break
			}
		}
		done <- struct{}{}
	}()

	// WebSocket → TCP.
	go func() {
		for {
			_, data, err := ws.ReadMessage()
			if err != nil {
				break
			}
			if _, err := tcp.Write(data); err != nil {
				break
			}
		}
		done <- struct{}{}
	}()

	<-done
	ws.Close()
	tcp.Close()
	<-done
}
