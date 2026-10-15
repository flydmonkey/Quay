package redis

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Conn Redis 连接配置
type Conn struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Host               string `json:"host"`
	Port               int    `json:"port"`
	Password           string `json:"password,omitempty"`
	DB                 int    `json:"db"`
	Cluster            bool   `json:"cluster"`              // 是否集群模式
	UseTLS             bool   `json:"use_tls"`              // 是否使用 TLS/SSL
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`  // TLS 是否跳过证书验证
}

var (
	storePath = "data/redis_connections.json"
	storeMu   sync.RWMutex
)

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

// ListConns 返回所有连接（密码脱敏）
func ListConns() ([]Conn, error) {
	list, err := loadConns()
	if err != nil {
		return nil, err
	}
	out := make([]Conn, len(list))
	for i, c := range list {
		out[i] = c
		out[i].Password = "***"
	}
	return out, nil
}

// GetConn 按 ID 获取连接（含密码，仅后端用）
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
	if c.Port == 0 {
		c.Port = 6379
	}
	found := false
	for i := range list {
		if list[i].ID == c.ID {
			if c.Password == "" || c.Password == "***" {
				c.Password = list[i].Password
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
	return fmt.Sprintf("redis-%d", time.Now().UnixNano())
}
