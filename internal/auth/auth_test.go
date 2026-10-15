package auth

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func testConfig() Config {
	return Config{
		Username: "admin",
		Password: "s3cret-pass",
		Secret:   []byte("test-secret"),
		TTL:      time.Hour,
	}
}

func TestSignAndVerify(t *testing.T) {
	cfg := testConfig()
	exp := time.Now().Add(time.Hour).Truncate(time.Second)
	value := cfg.sign(exp)
	if !cfg.verify(value, time.Now()) {
		t.Fatal("fresh cookie should verify")
	}
	if cfg.verify(value, exp) {
		t.Fatal("cookie at exact expiry should be rejected")
	}
	if cfg.verify(value, exp.Add(time.Second)) {
		t.Fatal("expired cookie should be rejected")
	}

	dot := strings.IndexByte(value, '.')
	sig := value[dot+1:]
	tampered := value[:dot+1] + flip(sig[0]) + sig[1:]
	if cfg.verify(tampered, time.Now()) {
		t.Fatal("tampered cookie should be rejected")
	}
	if cfg.verify("not-a-cookie", time.Now()) {
		t.Fatal("malformed cookie should be rejected")
	}
}

func TestLoginAndMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testConfig()
	r := gin.New()
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	api := r.Group("/api")
	api.Use(cfg.Middleware())
	api.POST("/login", cfg.Login)
	api.POST("/logout", cfg.Logout)
	api.GET("/auth/me", cfg.Me)
	api.GET("/info", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"app": "quay"})
	})

	health := get(t, r, "/api/health", nil)
	if health.Code != http.StatusOK {
		t.Fatalf("health = %d", health.Code)
	}

	info := get(t, r, "/api/info", nil)
	if info.Code != http.StatusUnauthorized || strings.TrimSpace(info.Body.String()) != `{"error":"未登录"}` {
		t.Fatalf("info without cookie = %d %s", info.Code, info.Body.String())
	}

	bad := postJSON(t, r, "/api/login", map[string]string{"username": "admin", "password": "nope"}, nil)
	if bad.Code != http.StatusUnauthorized {
		t.Fatalf("bad login = %d", bad.Code)
	}
	if strings.Contains(bad.Body.String(), cfg.Password) {
		t.Fatalf("error response leaked password: %s", bad.Body.String())
	}
	if strings.TrimSpace(bad.Body.String()) != `{"error":"用户名或密码错误"}` {
		t.Fatalf("bad login body = %s", bad.Body.String())
	}

	empty := postJSON(t, r, "/api/login", map[string]string{"username": "", "password": ""}, nil)
	if empty.Code != http.StatusUnauthorized {
		t.Fatalf("empty login = %d", empty.Code)
	}
	broken := postRaw(t, r, "/api/login", "{", nil)
	if broken.Code != http.StatusUnauthorized {
		t.Fatalf("broken login = %d", broken.Code)
	}

	ok := postJSON(t, r, "/api/login", map[string]string{"username": "admin", "password": cfg.Password}, nil)
	if ok.Code != http.StatusOK {
		t.Fatalf("login = %d %s", ok.Code, ok.Body.String())
	}
	res := ok.Result()
	defer res.Body.Close()
	cookies := res.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d", len(cookies))
	}
	ck := cookies[0]
	if ck.Name != "quay_session" || !ck.HttpOnly || ck.Path != "/" || ck.SameSite != http.SameSiteLaxMode || ck.Secure {
		t.Fatalf("cookie = %+v", ck)
	}

	me := get(t, r, "/api/auth/me", ck)
	if me.Code != http.StatusOK || strings.TrimSpace(me.Body.String()) != `{"username":"admin"}` {
		t.Fatalf("me = %d %s", me.Code, me.Body.String())
	}
	again := get(t, r, "/api/info", ck)
	if again.Code != http.StatusOK {
		t.Fatalf("info with cookie = %d", again.Code)
	}

	secureReq := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString(`{"username":"admin","password":"s3cret-pass"}`))
	secureReq.Header.Set("Content-Type", "application/json")
	secureReq.TLS = &tls.ConnectionState{}
	secureRes := httptest.NewRecorder()
	r.ServeHTTP(secureRes, secureReq)
	if !secureRes.Result().Cookies()[0].Secure {
		t.Fatal("https login should set Secure")
	}

	out := postJSON(t, r, "/api/logout", nil, ck)
	if out.Code != http.StatusOK {
		t.Fatalf("logout = %d", out.Code)
	}
	cleared := out.Result().Cookies()
	if len(cleared) != 1 || cleared[0].MaxAge >= 0 {
		t.Fatalf("cleared cookie = %+v", cleared)
	}
	header := out.Header().Get("Set-Cookie")
	if !strings.Contains(header, "Max-Age=0") {
		t.Fatalf("logout Set-Cookie = %s", header)
	}

	anonLogout := postJSON(t, r, "/api/logout", nil, nil)
	if anonLogout.Code != http.StatusOK {
		t.Fatalf("anonymous logout = %d", anonLogout.Code)
	}
}

func flip(b byte) string {
	if b == 'A' {
		return "B"
	}
	return "A"
}

func get(t *testing.T, h http.Handler, path string, ck *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if ck != nil {
		req.AddCookie(ck)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func postJSON(t *testing.T, h http.Handler, path string, body any, ck *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	return postRaw(t, h, path, buf.String(), ck)
}

func postRaw(t *testing.T, h http.Handler, path, body string, ck *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if ck != nil {
		req.AddCookie(ck)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestWriteUnauthorizedJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteUnauthorized(rec)
	res := rec.Result()
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusUnauthorized || strings.TrimSpace(string(b)) != `{"error":"未登录"}` {
		t.Fatalf("unauthorized = %d %s", res.StatusCode, b)
	}
}
