package vnc

import (
	"net"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// CheckConnection POST /api/vnc/check 预检：尝试连接 VNC，将错误信息返回给前端
func CheckConnection(c *gin.Context) {
	var body struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if body.Host == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "主机必填"})
		return
	}
	if body.Port <= 0 {
		body.Port = 5900
	}
	addr := net.JoinHostPort(body.Host, strconv.Itoa(body.Port))
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
		return
	}
	conn.Close()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ListConnections GET /api/vnc/connections
func ListConnections(c *gin.Context) {
	list, err := ListConns()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"list": list})
}

// GetConnection GET /api/vnc/connections/:id（返回含密码，仅用于连接）
func GetConnection(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 必填"})
		return
	}
	conn, err := GetConn(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if conn == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "连接不存在"})
		return
	}
	c.JSON(http.StatusOK, conn)
}

// SaveConnection POST /api/vnc/connections
func SaveConnection(c *gin.Context) {
	var conn Conn
	if err := c.ShouldBindJSON(&conn); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if conn.Host == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "主机必填"})
		return
	}
	if conn.Name == "" {
		conn.Name = conn.Host
	}
	if err := SaveConn(conn); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": conn.ID})
}

// DeleteConnection DELETE /api/vnc/connections/:id
func DeleteConnection(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 必填"})
		return
	}
	if err := DeleteConn(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
