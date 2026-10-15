package vnc

import (
	"io"
	"log"
	"net"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  32 * 1024,
	WriteBufferSize: 32 * 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// HandlerRaw 处理 VNC WebSocket：将浏览器 WebSocket 与远端 VNC 服务 TCP 双向转发
func HandlerRaw(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	portStr := r.URL.Query().Get("port")
	if host == "" {
		http.Error(w, "missing host", http.StatusBadRequest)
		return
	}
	if portStr == "" {
		portStr = "5900"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		http.Error(w, "invalid port", http.StatusBadRequest)
		return
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	tcpConn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Printf("vnc proxy: dial %s: %v", addr, err)
		http.Error(w, "无法连接 VNC 服务: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer tcpConn.Close()

	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("vnc proxy: ws upgrade: %v", err)
		return
	}
	defer wsConn.Close()

	// 双向转发：WebSocket <-> TCP
	done := make(chan struct{})
	go func() {
		defer func() { done <- struct{}{} }()
		for {
			mt, data, err := wsConn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					return
				}
				log.Printf("vnc proxy: ws read: %v", err)
				return
			}
			switch mt {
			case websocket.BinaryMessage:
				if _, err := tcpConn.Write(data); err != nil {
					log.Printf("vnc proxy: tcp write: %v", err)
					return
				}
			case websocket.TextMessage:
				// noVNC 有时会发文本（如设置）；按二进制写入 VNC 一般不需要，为兼容可写
				if _, err := tcpConn.Write(data); err != nil {
					log.Printf("vnc proxy: tcp write: %v", err)
					return
				}
			case websocket.CloseMessage:
				return
			}
		}
	}()
	go func() {
		defer func() { done <- struct{}{} }()
		buf := make([]byte, 32*1024)
		for {
			n, err := tcpConn.Read(buf)
			if n > 0 {
				if err := wsConn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
					log.Printf("vnc proxy: ws write: %v", err)
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					log.Printf("vnc proxy: tcp read: %v", err)
				}
				return
			}
		}
	}()

	<-done
}
