package db

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Conn 数据库连接配置（密码前端可传，存储时建议加密，此处简化不加密）
type Conn struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Driver   string `json:"driver"`   // mysql / postgres
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database,omitempty"` // 默认数据库
}

var (
	storePath = "data/db_connections.json"
	storeMu   sync.RWMutex
)

func setStorePath(p string) {
	storeMu.Lock()
	defer storeMu.Unlock()
	storePath = p
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
	found := false
	for i := range list {
		if list[i].ID == c.ID {
			// 若前端未传密码，保留原密码
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
	return fmt.Sprintf("conn-%d", time.Now().UnixNano())
}
