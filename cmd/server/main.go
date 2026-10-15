package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"webadmin/internal/db"
	"webadmin/internal/fs"
	"webadmin/internal/mongo"
	"webadmin/internal/redis"
	"webadmin/internal/ssh"
	"webadmin/internal/sshfs"
	"webadmin/internal/terminal"
	"webadmin/internal/vnc"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	// 基础健康检查
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	// 预留 API 分组
	api := r.Group("/api")
	{
		api.GET("/info", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"app": "quay",
				"env": os.Getenv("APP_ENV"),
			})
		})
		// 文件管理：列表、下载、上传、新建文件夹、删除
		api.GET("/fs/list", fs.List)
		api.GET("/fs/download", fs.Download)
		api.POST("/fs/upload", fs.Upload)
		api.POST("/fs/mkdir", fs.Mkdir)
		api.POST("/fs/delete", fs.Delete)
		api.GET("/fs/read", fs.Read)
		api.POST("/fs/write", fs.Write)
		api.GET("/fs/preview", fs.Preview)
		// 数据库管理
		api.GET("/db/connections", db.ListConnections)
		api.POST("/db/connections", db.SaveConnection)
		api.DELETE("/db/connections/:id", db.DeleteConnection)
		api.GET("/db/tree", db.GetTree)
		api.POST("/db/query", db.QueryResult)
		// MongoDB
		api.GET("/mongo/connections", mongo.ListConnections)
		api.POST("/mongo/connections", mongo.SaveConnection)
		api.DELETE("/mongo/connections/:id", mongo.DeleteConnection)
		api.GET("/mongo/tree", mongo.GetTree)
		api.POST("/mongo/query", mongo.QueryResult)
		// Redis
		api.GET("/redis/connections", redis.ListConnections)
		api.POST("/redis/connections", redis.SaveConnection)
		api.DELETE("/redis/connections/:id", redis.DeleteConnection)
		api.POST("/redis/exec", redis.ExecCommand)
		api.GET("/redis/keys", redis.Keys)
		api.POST("/redis/type", redis.GetType)
		// SSH 主机
		api.GET("/ssh/hosts", ssh.ListHostsHandler)
		api.POST("/ssh/hosts", ssh.SaveHostHandler)
		api.DELETE("/ssh/hosts/:id", ssh.DeleteHostHandler)
		// VNC 连接
		api.GET("/vnc/connections", vnc.ListConnections)
		api.GET("/vnc/connections/:id", vnc.GetConnection)
		api.POST("/vnc/connections", vnc.SaveConnection)
		api.DELETE("/vnc/connections/:id", vnc.DeleteConnection)
		api.POST("/vnc/check", vnc.CheckConnection)
		// SSH 远程文件：列表、下载、上传、新建目录、删除、读、写、预览
		api.GET("/ssh/fs/list", sshfs.List)
		api.GET("/ssh/fs/download", sshfs.Download)
		api.GET("/ssh/fs/read", sshfs.Read)
		api.GET("/ssh/fs/preview", sshfs.Preview)
		api.POST("/ssh/fs/upload", sshfs.Upload)
		api.POST("/ssh/fs/mkdir", sshfs.Mkdir)
		api.POST("/ssh/fs/delete", sshfs.Delete)
		api.POST("/ssh/fs/write", sshfs.Write)
	}

	// Web 终端：用原始 http 处理 WebSocket 升级，避免 Gin 中间件缓冲导致连接一直 pending
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if strings.EqualFold(req.Header.Get("Upgrade"), "websocket") {
			switch req.URL.Path {
			case "/api/terminal":
				terminal.HandlerRaw(w, req)
				return
			case "/api/vnc/ws":
				vnc.HandlerRaw(w, req)
				return
			}
		}
		r.ServeHTTP(w, req)
	})

	// 静态前端文件托管（Vue 构建产物放到 ./web/dist）
	webDir := "./web/dist"
	if _, err := os.Stat(webDir); err == nil {
		r.Static("/assets", webDir+"/assets")
		r.StaticFile("/favicon.ico", webDir+"/favicon.ico")
		r.GET("/", func(c *gin.Context) {
			c.File(webDir + "/index.html")
		})
		r.NoRoute(func(c *gin.Context) {
			c.File(webDir + "/index.html")
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{Addr: ":" + port, Handler: wrapped}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server start failed: %v", err)
	}
}
