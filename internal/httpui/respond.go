package httpui

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"dialectcorpusreleasegate/internal/application"
)

const maxBodyBytes = 1 << 20

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, &application.AppError{Code: "invalid_json", Message: "JSON 请求无效：" + err.Error()})
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeError(w, &application.AppError{Code: "invalid_json", Message: "请求体只能包含一个 JSON 对象"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	app, ok := err.(*application.AppError)
	if !ok {
		app = &application.AppError{Code: "internal_error", Message: err.Error()}
	}
	status := http.StatusBadRequest
	switch app.Code {
	case "not_found":
		status = http.StatusNotFound
	case "already_exists", "revision_conflict", "idempotency_conflict", "already_resolved", "duplicate_item_id",
		"duplicate_source_ref", "duplicate_content_digest", "annotation_immutable":
		status = http.StatusConflict
	case "forbidden", "annotator_forbidden":
		status = http.StatusForbidden
	case "internal_error", "batch_quarantined":
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, map[string]any{"error": app})
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := r.Host
	if strings.EqualFold(parsed.Scheme, "http") || strings.EqualFold(parsed.Scheme, "https") {
		return strings.EqualFold(parsed.Host, host)
	}
	return false
}

func requireSameOrigin(w http.ResponseWriter, r *http.Request) bool {
	if !sameOrigin(r) {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "cross_origin", "message": "拒绝跨源变更请求"}})
		return false
	}
	if content := r.Header.Get("Content-Type"); !strings.HasPrefix(strings.ToLower(content), "application/json") {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]any{"error": map[string]string{"code": "content_type", "message": "Content-Type 必须是 application/json"}})
		return false
	}
	return true
}
