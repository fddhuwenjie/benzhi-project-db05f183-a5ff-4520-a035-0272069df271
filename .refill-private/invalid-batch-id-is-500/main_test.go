package invalidbatchid_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dialectcorpusreleasegate/internal/application"
	"dialectcorpusreleasegate/internal/httpui"
	"dialectcorpusreleasegate/internal/store"
)

func TestInvalidBatchIDMustBeClientError(t *testing.T) {
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := httpui.New(application.NewService(repo, 1), slog.New(slog.NewTextHandler(io.Discard, nil)))
	body := `{"request_id":"invalid-id","expected_revision":-1,"batch_id":"../escape","title":"非法标识","dialect_site":"测试点","source_note":"本地","item_range":"u-001..u-001"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/batches", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("TestInvalidBatchIDMustBeClientError: 非法 batch_id 应返回 400，实际为 %d，响应为 %s", response.Code, response.Body.String())
	}
}
