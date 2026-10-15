package ssh

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ListHostsHandler GET /api/ssh/hosts
func ListHostsHandler(c *gin.Context) {
	list, err := ListHosts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"list": list})
}

// SaveHostHandler POST /api/ssh/hosts
func SaveHostHandler(c *gin.Context) {
	var h Host
	if err := c.ShouldBindJSON(&h); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if h.Host == "" || h.User == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host 和 user 必填"})
		return
	}
	if h.AuthType != "password" && h.AuthType != "key" {
		h.AuthType = "password"
	}
	if h.AuthType == "key" && h.PrivateKeyContent == "" && h.PrivateKeyPath == "" && h.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "证书登录需填写私钥路径或私钥内容"})
		return
	}
	if err := SaveHost(h); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": h.ID})
}

// DeleteHostHandler DELETE /api/ssh/hosts/:id
func DeleteHostHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 必填"})
		return
	}
	if err := DeleteHost(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
