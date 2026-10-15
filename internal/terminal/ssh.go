package terminal

import (
	"fmt"
	"io"
	"os"

	sshhost "webadmin/internal/ssh"

	"golang.org/x/crypto/ssh"
)

// sshSession 封装一次 SSH 会话的读写与 resize
type sshSession struct {
	stdin   io.Writer
	stdout  io.Reader
	stderr  io.Reader
	resize  func(cols, rows uint16)
	cleanup func()
}

// runSSH 建立 SSH 连接并启动远程 shell
func runSSH(hostID string, cols, rows uint16) (*sshSession, error) {
	h, err := sshhost.GetHost(hostID)
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
		config.Auth = []ssh.AuthMethod{ssh.Password(h.Password)}
	}

	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("SSH 连接失败: %w", err)
	}

	sess, err := client.NewSession()
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("创建会话失败: %w", err)
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := sess.RequestPty("xterm-256color", int(rows), int(cols), modes); err != nil {
		_ = sess.Close()
		_ = client.Close()
		return nil, fmt.Errorf("请求 PTY 失败: %w", err)
	}

	stdin, err := sess.StdinPipe()
	if err != nil {
		_ = sess.Close()
		_ = client.Close()
		return nil, err
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		_ = sess.Close()
		_ = client.Close()
		return nil, err
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		_ = sess.Close()
		_ = client.Close()
		return nil, err
	}

	if err := sess.Shell(); err != nil {
		_ = sess.Close()
		_ = client.Close()
		return nil, fmt.Errorf("启动 shell 失败: %w", err)
	}

	resize := func(c, r uint16) {
		if c < 2 {
			c = 80
		}
		if r < 2 {
			r = 24
		}
		_ = sess.WindowChange(int(r), int(c))
	}

	cleanup := func() {
		_ = sess.Close()
		_ = client.Close()
	}

	return &sshSession{
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
		resize:  resize,
		cleanup: cleanup,
	}, nil
}
