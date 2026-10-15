package ssh

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Host SSH 主机配置
type Host struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Host              string `json:"host"`
	Port              int    `json:"port"` // 默认 22
	User              string `json:"user"`
	AuthType          string `json:"auth_type"` // password / key
	Password          string `json:"password,omitempty"`
	PrivateKeyPath    string `json:"private_key_path,omitempty"`
	PrivateKeyContent string `json:"private_key_content,omitempty"`
}

var (
	storePath = "data/ssh_hosts.json"
	storeMu   sync.RWMutex
)

func loadHosts() ([]Host, error) {
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
	var list []Host
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func saveHosts(list []Host) error {
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

// ListHosts 返回所有主机（密码/私钥脱敏）
func ListHosts() ([]Host, error) {
	list, err := loadHosts()
	if err != nil {
		return nil, err
	}
	out := make([]Host, len(list))
	for i, h := range list {
		out[i] = h
		if h.AuthType == "password" {
			out[i].Password = "***"
		} else {
			out[i].PrivateKeyContent = ""
			if out[i].PrivateKeyPath != "" {
				out[i].PrivateKeyPath = "***"
			}
		}
	}
	return out, nil
}

// GetHost 按 ID 获取主机（含敏感信息，仅后端用）
func GetHost(id string) (*Host, error) {
	list, err := loadHosts()
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

// SaveHost 新增或更新主机
func SaveHost(h Host) error {
	list, err := loadHosts()
	if err != nil {
		return err
	}
	if h.ID == "" {
		h.ID = generateHostID()
	}
	if h.Port <= 0 {
		h.Port = 22
	}
	found := false
	for i := range list {
		if list[i].ID == h.ID {
			if h.AuthType == "password" && (h.Password == "" || h.Password == "***") {
				h.Password = list[i].Password
			}
			if h.AuthType == "key" && h.PrivateKeyContent == "" && h.PrivateKeyPath == "" {
				h.PrivateKeyContent = list[i].PrivateKeyContent
				h.PrivateKeyPath = list[i].PrivateKeyPath
			}
			list[i] = h
			found = true
			break
		}
	}
	if !found {
		list = append(list, h)
	}
	return saveHosts(list)
}

// DeleteHost 删除主机
func DeleteHost(id string) error {
	list, err := loadHosts()
	if err != nil {
		return err
	}
	newList := make([]Host, 0, len(list))
	for _, h := range list {
		if h.ID != id {
			newList = append(newList, h)
		}
	}
	return saveHosts(newList)
}

func generateHostID() string {
	return fmt.Sprintf("ssh-%d", time.Now().UnixNano())
}
