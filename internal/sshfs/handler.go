package sshfs

import (
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	sshhost "webadmin/internal/ssh"

	"github.com/gin-gonic/gin"
	"github.com/pkg/sftp"
)

// safeRemotePath 清理远程路径，禁止 ..，统一使用 / 供 SFTP
func safeRemotePath(p string) (string, bool) {
	p = strings.TrimPrefix(p, "/")
	p = strings.ReplaceAll(p, "\\", "/")
	if p == "" || p == "." {
		return "/", true
	}
	if strings.Contains(p, "..") {
		return "", false
	}
	parts := strings.Split(p, "/")
	var clean []string
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		clean = append(clean, part)
	}
	return "/" + strings.Join(clean, "/"), true
}

func withSFTP(c *gin.Context, hostID string, fn func(*sftp.Client) error) {
	if hostID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host required"})
		return
	}
	client, err := sshhost.Connect(hostID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer client.Close()
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "version packet") || strings.Contains(msg, "unexpected EOF") || strings.Contains(msg, "unexpectedly closed") {
			msg = "SFTP 不可用：服务端未开启或未正确配置 SFTP 子系统。请确认：1) 服务端为 OpenSSH 且 sshd_config 中有 Subsystem sftp 配置；2) 若为精简版 SSH（如部分 Docker 镜像），需安装带 sftp-server 的 openssh 或启用 SFTP 后再试。"
		} else {
			msg = "SFTP: " + msg
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
		return
	}
	defer sftpClient.Close()
	if err := fn(sftpClient); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

// List GET /api/ssh/fs/list?host=id&path=...
func List(c *gin.Context) {
	hostID := c.Query("host")
	reqPath := c.Query("path")
	remotePath, ok := safeRemotePath(reqPath)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}
	withSFTP(c, hostID, func(cl *sftp.Client) error {
		entries, err := cl.ReadDir(remotePath)
		if err != nil {
			return err
		}
		list := make([]map[string]interface{}, 0, len(entries))
		for _, e := range entries {
			size := int64(0)
			if !e.IsDir() {
				size = e.Size()
			}
			mod := int64(0)
			if !e.ModTime().IsZero() {
				mod = e.ModTime().Unix()
			}
			list = append(list, map[string]interface{}{
				"name":  e.Name(),
				"size":  size,
				"isDir": e.IsDir(),
				"mod":   mod,
			})
		}
		c.JSON(http.StatusOK, gin.H{"path": remotePath, "entries": list})
		return nil
	})
}

// Download GET /api/ssh/fs/download?host=id&path=...
func Download(c *gin.Context) {
	hostID := c.Query("host")
	reqPath := c.Query("path")
	remotePath, ok := safeRemotePath(reqPath)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}
	withSFTP(c, hostID, func(cl *sftp.Client) error {
		info, err := cl.Stat(remotePath)
		if err != nil {
			return err
		}
		if info.IsDir() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot download directory"})
			return nil
		}
		f, err := cl.Open(remotePath)
		if err != nil {
			return err
		}
		defer f.Close()
		name := filepath.Base(remotePath)
		c.Header("Content-Disposition", "attachment; filename=\""+name+"\"")
		c.Header("Content-Type", "application/octet-stream")
		if info.Size() > 0 {
			c.Header("Content-Length", strconv.FormatInt(info.Size(), 10))
		}
		_, err = io.Copy(c.Writer, f)
		return err
	})
}

var previewTypes = map[string]string{
	".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png",
	".gif": "image/gif", ".webp": "image/webp", ".svg": "image/svg+xml",
	".bmp": "image/bmp", ".ico": "image/x-icon",
	".mp4": "video/mp4", ".webm": "video/webm", ".ogg": "video/ogg", ".mov": "video/quicktime",
}

// Preview GET /api/ssh/fs/preview?host=id&path=...
func Preview(c *gin.Context) {
	hostID := c.Query("host")
	reqPath := c.Query("path")
	remotePath, ok := safeRemotePath(reqPath)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}
	ext := strings.ToLower(filepath.Ext(remotePath))
	ctype, ok := previewTypes[ext]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported preview type"})
		return
	}
	withSFTP(c, hostID, func(cl *sftp.Client) error {
		info, err := cl.Stat(remotePath)
		if err != nil {
			return err
		}
		if info.IsDir() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot preview directory"})
			return nil
		}
		f, err := cl.Open(remotePath)
		if err != nil {
			return err
		}
		defer f.Close()
		name := filepath.Base(remotePath)
		c.Header("Content-Type", ctype)
		c.Header("Content-Disposition", "inline; filename=\""+name+"\"")
		c.Header("Cache-Control", "no-cache")
		if info.Size() > 0 {
			c.Header("Content-Length", strconv.FormatInt(info.Size(), 10))
		}
		_, err = io.Copy(c.Writer, f)
		return err
	})
}

const maxReadSize = 2 << 20  // 2MB
const maxWriteSize = 4 << 20 // 4MB

// Read GET /api/ssh/fs/read?host=id&path=...
func Read(c *gin.Context) {
	hostID := c.Query("host")
	reqPath := c.Query("path")
	remotePath, ok := safeRemotePath(reqPath)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}
	withSFTP(c, hostID, func(cl *sftp.Client) error {
		info, err := cl.Stat(remotePath)
		if err != nil {
			return err
		}
		if info.IsDir() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot read directory"})
			return nil
		}
		if info.Size() > maxReadSize {
			c.JSON(http.StatusBadRequest, gin.H{"error": "file too large"})
			return nil
		}
		f, err := cl.Open(remotePath)
		if err != nil {
			return err
		}
		defer f.Close()
		content, err := io.ReadAll(f)
		if err != nil {
			return err
		}
		c.JSON(http.StatusOK, gin.H{"path": remotePath, "content": string(content)})
		return nil
	})
}

// Write POST /api/ssh/fs/write JSON: host, path, content
func Write(c *gin.Context) {
	var body struct {
		Host    string `json:"host"`
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host, path, content required"})
		return
	}
	remotePath, ok := safeRemotePath(body.Path)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}
	if len(body.Content) > maxWriteSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content too large"})
		return
	}
	withSFTP(c, body.Host, func(cl *sftp.Client) error {
		dir := filepath.Dir(remotePath)
		if dir != "/" {
			if err := cl.MkdirAll(dir); err != nil {
				// 可能已存在
			}
		}
		f, err := cl.Create(remotePath)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.Write([]byte(body.Content))
		if err != nil {
			return err
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return nil
	})
}

// Mkdir POST /api/ssh/fs/mkdir JSON or form: host, path, name
func Mkdir(c *gin.Context) {
	hostID := c.Query("host")
	if hostID == "" {
		hostID = c.PostForm("host")
	}
	var path, name string
	if strings.Contains(c.GetHeader("Content-Type"), "application/json") {
		var body struct {
			Host string `json:"host"`
			Path string `json:"path"`
			Name string `json:"name"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "host, path, name required"})
			return
		}
		hostID, path, name = body.Host, body.Path, body.Name
	} else {
		path = c.PostForm("path")
		name = c.PostForm("name")
	}
	if hostID == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host and name required"})
		return
	}
	if strings.Contains(name, "/") || strings.Contains(name, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid name"})
		return
	}
	parent, ok := safeRemotePath(path)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}
	newPath := parent + "/" + name
	if parent == "/" {
		newPath = "/" + name
	}
	withSFTP(c, hostID, func(cl *sftp.Client) error {
		err := cl.Mkdir(newPath)
		if err != nil {
			return err
		}
		c.JSON(http.StatusOK, gin.H{"path": newPath})
		return nil
	})
}

// Delete POST /api/ssh/fs/delete JSON or form: host, path
func Delete(c *gin.Context) {
	hostID := c.Query("host")
	if hostID == "" {
		hostID = c.PostForm("host")
	}
	var path string
	if strings.Contains(c.GetHeader("Content-Type"), "application/json") {
		var body struct {
			Host string `json:"host"`
			Path string `json:"path"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "host and path required"})
			return
		}
		hostID, path = body.Host, body.Path
	} else {
		path = c.PostForm("path")
	}
	if hostID == "" || path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host and path required"})
		return
	}
	remotePath, ok := safeRemotePath(path)
	if !ok || remotePath == "/" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}
	withSFTP(c, hostID, func(cl *sftp.Client) error {
		info, err := cl.Stat(remotePath)
		if err != nil {
			return err
		}
		if info.IsDir() {
			err = cl.RemoveAll(remotePath)
		} else {
			err = cl.Remove(remotePath)
		}
		if err != nil {
			return err
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return nil
	})
}

// Upload POST /api/ssh/fs/upload form: host, path, file(s)
func Upload(c *gin.Context) {
	hostID := c.PostForm("host")
	dirPath := c.PostForm("path")
	if hostID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host required"})
		return
	}
	if dirPath == "" {
		dirPath = "/"
	}
	remoteDir, ok := safeRemotePath(dirPath)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "multipart required"})
		return
	}
	files := form.File["file"]
	if len(files) == 0 {
		files = form.File["files"]
	}
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no file in field 'file' or 'files'"})
		return
	}
	withSFTP(c, hostID, func(cl *sftp.Client) error {
		if remoteDir != "/" {
			if err := cl.MkdirAll(remoteDir); err != nil {
				// ignore
			}
		}
		var saved []string
		for _, fh := range files {
			dest := remoteDir + "/" + fh.Filename
			if remoteDir == "/" {
				dest = "/" + fh.Filename
			}
			if strings.Contains(fh.Filename, "..") {
				continue
			}
			src, err := fh.Open()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "saved": saved})
				return nil
			}
			dst, err := cl.Create(dest)
			if err != nil {
				_ = src.Close()
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "saved": saved})
				return nil
			}
			_, err = io.Copy(dst, src)
			_ = src.Close()
			_ = dst.Close()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "saved": saved})
				return nil
			}
			saved = append(saved, fh.Filename)
		}
		c.JSON(http.StatusOK, gin.H{"saved": saved})
		return nil
	})
}
