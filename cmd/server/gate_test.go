package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"webadmin/internal/auth"

	"github.com/gin-gonic/gin"
)

func TestWebSocketRequiresSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := auth.Config{
		Username: "admin",
		Password: "s3cret-pass",
		Secret:   []byte("test-secret"),
		TTL:      time.Hour,
	}
	srv := httptest.NewServer(newHandler(cfg))
	defer srv.Close()

	for _, path := range []string{"/api/terminal", "/api/vnc/ws"} {
		req, err := http.NewRequest(http.MethodGet, srv.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Connection", "Upgrade")
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Sec-WebSocket-Version", "13")
		req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != http.StatusUnauthorized || strings.TrimSpace(string(body)) != `{"error":"未登录"}` {
			t.Fatalf("%s = %d %s", path, res.StatusCode, body)
		}
		if strings.EqualFold(res.Header.Get("Upgrade"), "websocket") {
			t.Fatalf("%s upgraded without a session", path)
		}
	}
}
