package httpui

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dialectcorpusreleasegate/internal/application"
	"dialectcorpusreleasegate/internal/store"
)

func newTestHandler(t *testing.T) http.Handler {
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return New(application.NewService(repo, 2), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestWorkbenchAndSecurityHeaders(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/workbench", nil)
	response := httptest.NewRecorder()
	h.ServeHTTP(response, req)
	if response.Code != 200 || !strings.Contains(response.Body.String(), "<body>") {
		t.Fatalf("工作台响应异常：%d", response.Code)
	}
	if response.Header().Get("Content-Security-Policy") == "" || response.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("安全响应头缺失")
	}
}

func TestRejectsCrossOriginMutation(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/batches", strings.NewReader(`{}`))
	req.Host = "127.0.0.1:19081"
	req.Header.Set("Origin", "https://attacker.example")
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	h.ServeHTTP(response, req)
	if response.Code != http.StatusForbidden {
		t.Fatalf("跨源请求状态=%d", response.Code)
	}
}
