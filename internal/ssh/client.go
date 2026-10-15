package ssh

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// 连接超时
const dialTimeout = 15 * time.Second

// Connect 根据主机 ID 建立 SSH 连接，调用方需负责 Close
func Connect(hostID string) (*ssh.Client, error) {
	h, err := GetHost(hostID)
	if err != nil || h == nil {
		return nil, fmt.Errorf("SSH 主机不存在: %s", hostID)
	}
	port := h.Port
	if port <= 0 {
		port = 22
	}
	addr := fmt.Sprintf("%s:%d", h.Host, port)

	config := &ssh.ClientConfig{
		User:            h.User,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         dialTimeout,
	}

	if h.AuthType == "key" {
		var keyBytes []byte
		if h.PrivateKeyContent != "" {
			keyBytes = []byte(h.PrivateKeyContent)
		} else if h.PrivateKeyPath != "" {
			keyBytes, err = os.ReadFile(h.PrivateKeyPath)
			if err != nil {
				return nil, fmt.Errorf("读取私钥失败: %w", err)
			}
		} else {
			return nil, fmt.Errorf("证书登录需配置私钥路径或私钥内容")
		}
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("解析私钥失败: %w", err)
		}
		config.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	} else {
		// 同时提供 password 与 keyboard-interactive，兼容仅支持 keyboard-interactive 密码认证的服务端
		config.Auth = []ssh.AuthMethod{
			ssh.Password(h.Password),
			ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				if len(questions) == 0 {
					return nil, nil
				}
				// 对密码类提问统一用配置的密码应答
				answers := make([]string, len(questions))
				for i := range questions {
					answers[i] = h.Password
				}
				return answers, nil
			}),
		}
	}

	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return nil, fmt.Errorf("SSH 连接失败（无法连到 %s）：%w。请确认主机、端口(默认22)、防火墙与 SSH 服务", addr, err)
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		_ = conn.Close()
		hint := "请确认：1) 端口为 SSH(22)，未误连到 443/80 等；2) 用户名与密码/私钥正确；3) 本机可用 ssh -v 用户@主机 -p 端口 对比测试。"
		switch {
		case err.Error() == "ssh: handshake failed: EOF":
			hint = "握手被对端关闭(EOF)，常见原因：连错端口(如连到 HTTPS)、中间代理、或服务端限制。请用命令行 ssh -v 用户@主机 -p 端口 测试同一地址是否可连。"
		case strings.Contains(err.Error(), "unable to authenticate") || strings.Contains(err.Error(), "no supported methods remain"):
			hint = "认证失败。若报文中出现 attempted methods [none password]，通常表示服务端只允许证书登录、不接受密码。请在「管理 SSH 连接」里将该主机改为「证书（私钥）」并填写私钥路径或粘贴私钥内容后重试。"
		}
		return nil, fmt.Errorf("SSH 连接失败（握手失败）：%w。%s", err, hint)
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	return client, nil
}
