package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	cookieName = "quay_session"
	ttl        = 7 * 24 * time.Hour
)

// Config 是单账号登录配置。
type Config struct {
	Username string
	Password string
	Secret   []byte
	TTL      time.Duration
}

// LoadConfig 读取环境变量。未设置的用户名和密码各自回退为 admin。
// 未设置会话密钥时随机生成，进程重启后旧 Cookie 失效。
func LoadConfig() Config {
	user := os.Getenv("QUAY_USER")
	if user == "" {
		user = "admin"
	}
	pass := os.Getenv("QUAY_PASSWORD")
	if pass == "" {
		pass = "admin"
	}
	secret := []byte(os.Getenv("QUAY_SESSION_SECRET"))
	if len(secret) == 0 {
		secret = make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			panic("auth: generate session secret: " + err.Error())
		}
	}
	return Config{Username: user, Password: pass, Secret: secret, TTL: ttl}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Login 校验账号并写入会话 Cookie。失败时不区分原因。
func (c Config) Login(ctx *gin.Context) {
	var req loginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil || !c.credentialsMatch(req.Username, req.Password) {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}
	c.SetSession(ctx.Writer, ctx.Request)
	ctx.JSON(http.StatusOK, gin.H{"ok": true})
}

// Logout 清除会话 Cookie。没有有效会话时也可以调用。
func (c Config) Logout(ctx *gin.Context) {
	c.ClearSession(ctx.Writer, ctx.Request)
	ctx.JSON(http.StatusOK, gin.H{"ok": true})
}

// Me 返回当前配置的用户名。调用前必须已经通过会话校验。
func (c Config) Me(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"username": c.Username})
}

// Middleware 放行登录、退出，其余请求要求有效 Cookie。
func (c Config) Middleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if ctx.Request.Method == http.MethodPost {
			switch ctx.Request.URL.Path {
			case "/api/login", "/api/logout":
				ctx.Next()
				return
			}
		}
		if !c.Authenticated(ctx.Request) {
			writeUnauthorized(ctx.Writer)
			ctx.Abort()
			return
		}
		ctx.Next()
	}
}

// Authenticated 报告请求是否携带未过期且签名正确的会话 Cookie。
func (c Config) Authenticated(r *http.Request) bool {
	ck, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}
	return c.verify(ck.Value, time.Now())
}

// SetSession 写入 7 天有效的 HttpOnly Cookie。
func (c Config) SetSession(w http.ResponseWriter, r *http.Request) {
	exp := time.Now().Add(c.TTL)
	http.SetCookie(w, c.cookie(c.sign(exp), int(c.TTL.Seconds()), exp, r.TLS != nil))
}

// ClearSession 用 Max-Age=0 覆盖会话 Cookie。
func (c Config) ClearSession(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, c.cookie("", -1, time.Unix(0, 0), r.TLS != nil))
}

// WriteUnauthorized 写出统一的未登录响应。
func WriteUnauthorized(w http.ResponseWriter) {
	writeUnauthorized(w)
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "未登录"})
}

func (c Config) cookie(value string, maxAge int, exp time.Time, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     cookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   maxAge,
		Expires:  exp,
	}
}

func (c Config) credentialsMatch(user, pass string) bool {
	return digestEqual(user, c.Username)&digestEqual(pass, c.Password) == 1
}

func digestEqual(got, want string) int {
	g := sha256.Sum256([]byte(got))
	w := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(g[:], w[:])
}

func (c Config) sign(exp time.Time) string {
	msg := strconv.FormatInt(exp.Unix(), 10)
	return base64.RawURLEncoding.EncodeToString([]byte(msg)) + "." + base64.RawURLEncoding.EncodeToString(c.mac([]byte(msg)))
}

func (c Config) verify(value string, now time.Time) bool {
	left, right, ok := strings.Cut(value, ".")
	if !ok || strings.Contains(right, ".") {
		return false
	}
	msg, err := base64.RawURLEncoding.DecodeString(left)
	if err != nil {
		return false
	}
	sig, err := base64.RawURLEncoding.DecodeString(right)
	if err != nil {
		return false
	}
	if !hmac.Equal(sig, c.mac(msg)) {
		return false
	}
	sec, err := strconv.ParseInt(string(msg), 10, 64)
	if err != nil {
		return false
	}
	return now.Before(time.Unix(sec, 0))
}

func (c Config) mac(msg []byte) []byte {
	h := hmac.New(sha256.New, c.Secret)
	_, _ = h.Write(msg)
	return h.Sum(nil)
}
