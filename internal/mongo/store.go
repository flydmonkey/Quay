package mongo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Conn MongoDB 连接配置（使用 URI）
type Conn struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URI  string `json:"uri"` // mongodb://[user:pass@]host[:port][/db][?options]
}

var (
	storePath = "data/mongo_connections.json"
	storeMu   sync.RWMutex
)

// maskURI 将 URI 中的密码替换为 ***
func maskURI(uri string) string {
	// mongodb://user:password@host -> mongodb://user:***@host
	re := regexp.MustCompile(`([^:]+://[^:]+:)([^@]+)(@)`)
	return re.ReplaceAllString(uri, "${1}***${3}")
}

func loadConns() ([]Conn, error) {
	storeMu.RLock()
	p := storePath
	storeMu.RUnlock()
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var list []Conn
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func saveConns(list []Conn) error {
	storeMu.RLock()
	p := storePath
	storeMu.RUnlock()
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}

// ListConns 返回所有连接（URI 密码脱敏）
func ListConns() ([]Conn, error) {
	list, err := loadConns()
	if err != nil {
		return nil, err
	}
	out := make([]Conn, len(list))
	for i, c := range list {
		out[i] = c
		out[i].URI = maskURI(c.URI)
	}
	return out, nil
}

// GetConn 按 ID 获取连接（含完整 URI，仅后端用）
func GetConn(id string) (*Conn, error) {
	list, err := loadConns()
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].ID == id {
			return &list[i], nil
		}
	}
	return nil, nil
}

// SaveConn 新增或更新连接
func SaveConn(c Conn) error {
	list, err := loadConns()
	if err != nil {
		return err
	}
	if c.ID == "" {
		c.ID = generateID()
	}
	found := false
	for i := range list {
		if list[i].ID == c.ID {
			if c.URI == "" || c.URI == "***" || strings.Contains(c.URI, "***") {
				c.URI = list[i].URI
			}
			list[i] = c
			found = true
			break
		}
	}
	if !found {
		list = append(list, c)
	}
	return saveConns(list)
}

// DeleteConn 删除连接
func DeleteConn(id string) error {
	list, err := loadConns()
	if err != nil {
		return err
	}
	newList := make([]Conn, 0, len(list))
	for _, c := range list {
		if c.ID != id {
			newList = append(newList, c)
		}
	}
	return saveConns(newList)
}

func generateID() string {
	return fmt.Sprintf("mongo-%d", time.Now().UnixNano())
}
