package terminal

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// resizePayload 前端发来的终端尺寸
type resizePayload struct {
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

// Handler 供 Gin 路由使用；实际升级请用 HandlerRaw 避免中间件缓冲导致 pending
func Handler(c *gin.Context) {
	HandlerRaw(c.Writer, c.Request)
}

// HandlerRaw 用原始 http.ResponseWriter 处理 WebSocket；支持 ?host=id 走 SSH，否则本地 PTY
func HandlerRaw(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("terminal ws upgrade: %v", err)
		return
	}
	defer conn.Close()

	hostID := r.URL.Query().Get("host")
	if hostID != "" {
		runSSHTerminal(conn, hostID)
		return
	}
	runLocalTerminal(conn, r)
}

func runSSHTerminal(conn *websocket.Conn, hostID string) {
	cols, rows := uint16(80), uint16(24)
	sess, err := runSSH(hostID, cols, rows)
	if err != nil {
		log.Printf("ssh run: %v", err)
		_ = conn.WriteMessage(websocket.TextMessage, []byte("SSH 连接失败: "+err.Error()+"\r\n"))
		return
	}
	defer sess.cleanup()

	var writeMu sync.Mutex
	writeWS := func(data []byte) {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.WriteMessage(websocket.BinaryMessage, data)
	}

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := sess.stdout.Read(buf)
			if n > 0 {
				writeWS(buf[:n])
			}
			if err != nil {
				break
			}
		}
	}()
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := sess.stderr.Read(buf)
			if n > 0 {
				writeWS(buf[:n])
			}
			if err != nil {
				break
			}
		}
	}()

	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		switch mt {
		case websocket.TextMessage:
			var r resizePayload
			if json.Unmarshal(data, &r) == nil && r.Cols >= 2 && r.Rows >= 2 {
				sess.resize(r.Cols, r.Rows)
				continue
			}
			_, _ = sess.stdin.Write(data)
		case websocket.BinaryMessage:
			_, _ = sess.stdin.Write(data)
		}
	}
}

func runLocalTerminal(conn *websocket.Conn, r *http.Request) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	if sh := r.URL.Query().Get("shell"); sh != "" {
		shell = sh
	}
	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	sz := &pty.Winsize{Cols: 80, Rows: 24}
	ptmx, err := pty.StartWithSize(cmd, sz)
	if err != nil {
		log.Printf("pty start: %v", err)
		_ = conn.WriteMessage(websocket.TextMessage, []byte("pty start failed: "+err.Error()+"\r\n"))
		return
	}
	defer func() {
		_ = ptmx.Close()
		_ = cmd.Process.Kill()
	}()

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				if e := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); e != nil {
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					log.Printf("pty read: %v", err)
				}
				return
			}
		}
	}()

	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		switch mt {
		case websocket.TextMessage:
			var r resizePayload
			if json.Unmarshal(data, &r) == nil && r.Cols >= 2 && r.Rows >= 2 {
				_ = pty.Setsize(ptmx, &pty.Winsize{Cols: r.Cols, Rows: r.Rows})
				continue
			}
			_, _ = ptmx.Write(data)
		case websocket.BinaryMessage:
			_, _ = ptmx.Write(data)
		}
	}
}
