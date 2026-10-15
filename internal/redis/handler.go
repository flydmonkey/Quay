package redis

import (
	"context"
	"crypto/tls"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const connectTimeout = 10 * time.Second

// redisDoCloser 兼容 standalone 与 cluster 的接口
type redisDoCloser interface {
	Do(ctx context.Context, args ...interface{}) *redis.Cmd
	Type(ctx context.Context, key string) *redis.StatusCmd
	Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd
	Close() error
}

func clientFromConn(conn *Conn) (redisDoCloser, error) {
	port := conn.Port
	if port == 0 {
		port = 6379
	}
	addr := conn.Host + ":" + strconv.Itoa(port)

	var tlsConfig *tls.Config
	if conn.UseTLS {
		tlsConfig = &tls.Config{InsecureSkipVerify: conn.InsecureSkipVerify}
	}

	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	if conn.Cluster {
		opt := &redis.ClusterOptions{
			Addrs:        []string{addr},
			Password:     conn.Password,
			TLSConfig:    tlsConfig,
			DialTimeout:  connectTimeout,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 10 * time.Second,
		}
		client := redis.NewClusterClient(opt)
		if err := client.Ping(ctx).Err(); err != nil {
			_ = client.Close()
			return nil, err
		}
		return client, nil
	}

	opt := &redis.Options{
		Addr:         addr,
		Password:     conn.Password,
		DB:           conn.DB,
		TLSConfig:    tlsConfig,
		DialTimeout:  connectTimeout,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	client := redis.NewClient(opt)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

// replyToJSON 将 Redis 返回的 interface{} 转为 JSON 可序列化类型
func replyToJSON(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case []byte:
		return string(val)
	case string:
		return val
	case int64:
		return val
	case []interface{}:
		out := make([]interface{}, len(val))
		for i, item := range val {
			out[i] = replyToJSON(item)
		}
		return out
	default:
		return v
	}
}

// ListConnections GET /api/redis/connections
func ListConnections(c *gin.Context) {
	list, err := ListConns()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"list": list})
}

// SaveConnection POST /api/redis/connections
func SaveConnection(c *gin.Context) {
	var conn Conn
	if err := c.ShouldBindJSON(&conn); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if conn.Name == "" || conn.Host == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and host required"})
		return
	}
	if conn.Port == 0 {
		conn.Port = 6379
	}
	if err := SaveConn(conn); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": conn.ID})
}

// DeleteConnection DELETE /api/redis/connections/:id
func DeleteConnection(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	if err := DeleteConn(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ExecRequest 执行命令请求
type ExecRequest struct {
	ConnectionID string   `json:"connectionId"`
	DB           int      `json:"db"`
	Command      []string `json:"command"`
}

// ExecCommand POST /api/redis/exec
func ExecCommand(c *gin.Context) {
	var req ExecRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ConnectionID == "" || len(req.Command) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "connectionId and command required"})
		return
	}
	conn, err := GetConn(req.ConnectionID)
	if err != nil || conn == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "connection not found"})
		return
	}
	client, err := clientFromConn(conn)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "connect failed: " + err.Error()})
		return
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if !conn.Cluster && req.DB != conn.DB {
		if err := client.Do(ctx, "SELECT", req.DB).Err(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "SELECT db: " + err.Error()})
			return
		}
	}

	args := make([]interface{}, len(req.Command))
	for i, s := range req.Command {
		args[i] = s
	}
	cmd := client.Do(ctx, args...)
	if err := cmd.Err(); err != nil {
		if err == redis.Nil {
			c.JSON(http.StatusOK, gin.H{"result": nil})
			return
		}
		c.JSON(http.StatusOK, gin.H{"error": err.Error()})
		return
	}
	val, err := cmd.Result()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": replyToJSON(val)})
}

// KeysRequest 列出 key（SCAN）
type KeysRequest struct {
	ConnectionID string `json:"connectionId" form:"connectionId"`
	DB           int    `json:"db" form:"db"`
	Pattern      string `json:"pattern" form:"pattern"`
	Cursor       uint64 `json:"cursor" form:"cursor"`
	Count        int64  `json:"count" form:"count"`
}

// Keys GET /api/redis/keys?connectionId= &pattern=* &cursor=0 &count=100
func Keys(c *gin.Context) {
	var req KeysRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ConnectionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "connectionId required"})
		return
	}
	if req.Pattern == "" {
		req.Pattern = "*"
	}
	if req.Count <= 0 {
		req.Count = 100
	}
	conn, err := GetConn(req.ConnectionID)
	if err != nil || conn == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "connection not found"})
		return
	}
	client, err := clientFromConn(conn)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "connect failed: " + err.Error()})
		return
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if !conn.Cluster && req.DB != conn.DB {
		if err := client.Do(ctx, "SELECT", req.DB).Err(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "SELECT db: " + err.Error()})
			return
		}
	}

	keys, nextCursor, err := client.Scan(ctx, req.Cursor, req.Pattern, req.Count).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"keys": keys, "cursor": nextCursor})
}

// GetTypeRequest 查询 key 类型
type GetTypeRequest struct {
	ConnectionID string `json:"connectionId"`
	DB           int    `json:"db"`
	Key          string `json:"key"`
}

// GetType POST /api/redis/type
func GetType(c *gin.Context) {
	var req GetTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ConnectionID == "" || req.Key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "connectionId and key required"})
		return
	}
	conn, err := GetConn(req.ConnectionID)
	if err != nil || conn == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "connection not found"})
		return
	}
	client, err := clientFromConn(conn)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "connect failed: " + err.Error()})
		return
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if !conn.Cluster && req.DB != conn.DB {
		if err := client.Do(ctx, "SELECT", req.DB).Err(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "SELECT db: " + err.Error()})
			return
		}
	}

	typ, err := client.Type(ctx, req.Key).Result()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"type": strings.ToLower(typ)})
}
