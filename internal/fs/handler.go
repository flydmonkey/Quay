package fs

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Root 为文件操作允许的根目录，可通过环境变量 WEBADMIN_FS_ROOT 设置，默认为当前工作目录
func getRoot() string {
	root := os.Getenv("WEBADMIN_FS_ROOT")
	if root == "" {
		root = "."
	}
	abs, _ := filepath.Abs(root)
	return abs
}

// safePath 将请求的 path 限制在 root 下，若越界返回空字符串和 false
func safePath(root, reqPath string) (string, bool) {
	reqPath = strings.TrimPrefix(reqPath, "/")
	if reqPath == "" || reqPath == "." {
		return root, true
	}
	// 禁止往上走
	if strings.Contains(reqPath, "..") {
		return "", false
	}
	joined := filepath.Join(root, filepath.Clean(reqPath))
	abs, err := filepath.Abs(joined)
	if err != nil {
		return "", false
	}
	if !strings.HasPrefix(abs, root) {
		return "", false
	}
	return abs, true
}

// FileInfo 目录列表项
type FileInfo struct {
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"isDir"`
	Mod   int64  `json:"mod"`
}

// List 列出目录内容 GET /api/fs/list?path=...
func List(c *gin.Context) {
	root := getRoot()
	reqPath := c.Query("path")
	abs, ok := safePath(root, reqPath)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "not a directory"})
		return
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	list := make([]FileInfo, 0, len(entries))
	for _, e := range entries {
		finfo, _ := e.Info()
		size := int64(0)
		if finfo != nil && !e.IsDir() {
			size = finfo.Size()
		}
		mod := int64(0)
		if finfo != nil {
			mod = finfo.ModTime().Unix()
		}
		list = append(list, FileInfo{
			Name:  e.Name(),
			Size:  size,
			IsDir: e.IsDir(),
			Mod:   mod,
		})
	}
	// 相对路径用于前端展示/继续请求
	rel, _ := filepath.Rel(root, abs)
	if rel == "" || rel == "." {
		rel = "/"
	} else if !strings.HasPrefix(rel, "/") {
		rel = "/" + rel
	}
	c.JSON(http.StatusOK, gin.H{"path": rel, "root": root, "entries": list})
}

// Download 下载文件 GET /api/fs/download?path=...
func Download(c *gin.Context) {
	root := getRoot()
	reqPath := c.Query("path")
	if reqPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}
	abs, ok := safePath(root, reqPath)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot download directory"})
		return
	}
	f, err := os.Open(abs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer f.Close()
	name := filepath.Base(abs)
	c.Header("Content-Disposition", "attachment; filename=\""+name+"\"")
	c.Header("Content-Type", "application/octet-stream")
	if info.Size() > 0 {
		c.Header("Content-Length", strconv.FormatInt(info.Size(), 10))
	}
	io.Copy(c.Writer, f)
}

// 预览用：仅允许的图片/视频扩展名 -> Content-Type
var previewTypes = map[string]string{
	".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png",
	".gif": "image/gif", ".webp": "image/webp", ".svg": "image/svg+xml",
	".bmp": "image/bmp", ".ico": "image/x-icon",
	".mp4": "video/mp4", ".webm": "video/webm", ".ogg": "video/ogg", ".mov": "video/quicktime",
}

// Preview 预览图片/视频 GET /api/fs/preview?path=... 返回 inline，浏览器直接展示
func Preview(c *gin.Context) {
	root := getRoot()
	reqPath := c.Query("path")
	if reqPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}
	abs, ok := safePath(root, strings.TrimPrefix(reqPath, "/"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot preview directory"})
		return
	}
	ext := strings.ToLower(filepath.Ext(abs))
	ctype, ok := previewTypes[ext]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported preview type"})
		return
	}
	f, err := os.Open(abs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer f.Close()
	name := filepath.Base(abs)
	c.Header("Content-Type", ctype)
	c.Header("Content-Disposition", "inline; filename=\""+name+"\"")
	c.Header("Cache-Control", "no-cache")
	if info.Size() > 0 {
		c.Header("Content-Length", strconv.FormatInt(info.Size(), 10))
	}
	io.Copy(c.Writer, f)
}

// Upload 上传文件 POST /api/fs/upload 表单字段 path（目录路径）, file（可多个）
func Upload(c *gin.Context) {
	root := getRoot()
	dirPath := c.PostForm("path")
	if dirPath == "" {
		dirPath = "/"
	}
	abs, ok := safePath(root, strings.TrimPrefix(dirPath, "/"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(abs, 0755); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else if !info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "upload path is not a directory"})
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "multipart form required"})
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
	var saved []string
	for _, fh := range files {
		dst := filepath.Join(abs, filepath.Base(fh.Filename))
		// 再次确保目标在 root 下
		dstAbs, _ := filepath.Abs(dst)
		if !strings.HasPrefix(dstAbs, root) {
			continue
		}
		if err := c.SaveUploadedFile(fh, dst); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "saved": saved})
			return
		}
		saved = append(saved, fh.Filename)
	}
	c.JSON(http.StatusOK, gin.H{"saved": saved})
}

// Mkdir 新建文件夹 POST /api/fs/mkdir 表单或 JSON: path（父目录）, name（文件夹名）
func Mkdir(c *gin.Context) {
	root := getRoot()
	var path, name string
	if strings.Contains(c.GetHeader("Content-Type"), "application/json") {
		var body struct {
			Path string `json:"path"`
			Name string `json:"name"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "path and name required"})
			return
		}
		path, name = body.Path, body.Name
	} else {
		path = c.PostForm("path")
		name = c.PostForm("name")
	}
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}
	if strings.Contains(name, "/") || strings.Contains(name, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid name"})
		return
	}
	if path == "" {
		path = "/"
	}
	parentAbs, ok := safePath(root, strings.TrimPrefix(path, "/"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}
	newDir := filepath.Join(parentAbs, name)
	newDirAbs, err := filepath.Abs(newDir)
	if err != nil || !strings.HasPrefix(newDirAbs, root) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}
	if err := os.Mkdir(newDirAbs, 0755); err != nil {
		if os.IsExist(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "directory already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"path": newDirAbs, "name": name})
}

// Delete 删除文件或文件夹 POST /api/fs/delete 表单或 JSON: path（相对根目录的路径）
func Delete(c *gin.Context) {
	root := getRoot()
	var path string
	if strings.Contains(c.GetHeader("Content-Type"), "application/json") {
		var body struct {
			Path string `json:"path"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
			return
		}
		path = body.Path
	} else {
		path = c.PostForm("path")
	}
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}
	abs, ok := safePath(root, strings.TrimPrefix(path, "/"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}
	// 禁止删除根目录本身
	if abs == root {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete root"})
		return
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if info.IsDir() {
		if err := os.RemoveAll(abs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		if err := os.Remove(abs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

const maxEditSize = 2 << 20 // 2MB

// Read 读取文本文件内容，用于编辑 GET /api/fs/read?path=...
func Read(c *gin.Context) {
	root := getRoot()
	reqPath := c.Query("path")
	if reqPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}
	abs, ok := safePath(root, strings.TrimPrefix(reqPath, "/"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot read directory"})
		return
	}
	if info.Size() > maxEditSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file too large to edit (max 2MB)"})
		return
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 按 UTF-8 返回；若非合法 UTF-8 则原样返回
	content := string(data)
	c.JSON(http.StatusOK, gin.H{"path": reqPath, "content": content})
}

// Write 保存文本文件 POST /api/fs/write JSON: path, content
func Write(c *gin.Context) {
	root := getRoot()
	var body struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path and content required"})
		return
	}
	if body.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}
	abs, ok := safePath(root, strings.TrimPrefix(body.Path, "/"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}
	info, err := os.Stat(abs)
	if err == nil && info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot write to directory"})
		return
	}
	parent := filepath.Dir(abs)
	if err := os.MkdirAll(parent, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := os.WriteFile(abs, []byte(body.Content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
