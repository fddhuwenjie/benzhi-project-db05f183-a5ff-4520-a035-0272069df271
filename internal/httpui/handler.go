package httpui

import (
	"context"
	"embed"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"dialectcorpusreleasegate/internal/application"
)

//go:embed assets/*
var assets embed.FS

type Handler struct {
	service *application.Service
	logger  *slog.Logger
	mux     *http.ServeMux
}

func New(service *application.Service, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	h := &Handler{service: service, logger: logger, mux: http.NewServeMux()}
	h.routes()
	return h
}

func (h *Handler) routes() {
	h.mux.HandleFunc("GET /", h.RootHandler)
	h.mux.HandleFunc("GET /workbench", h.WorkbenchHandler)
	h.mux.HandleFunc("GET /assets/app.css", h.AssetHandler)
	h.mux.HandleFunc("GET /assets/app.js", h.AssetHandler)
	h.mux.HandleFunc("GET /api/v1/batches", h.ListBatchesHandler)
	h.mux.HandleFunc("POST /api/v1/batches", h.CreateBatchHandler)
	h.mux.HandleFunc("GET /api/v1/batches/{batchID}", h.BatchDetailHandler)
	h.mux.HandleFunc("POST /api/v1/batches/{batchID}/freeze", h.FreezeBatchHandler)
	h.mux.HandleFunc("POST /api/v1/batches/{batchID}/items", h.RegisterItemHandler)
	h.mux.HandleFunc("POST /api/v1/batches/{batchID}/annotations", h.SubmitAnnotationHandler)
	h.mux.HandleFunc("POST /api/v1/batches/{batchID}/disagreements/resolve", h.ResolveDisagreementHandler)
	h.mux.HandleFunc("POST /api/v1/batches/{batchID}/audit/preview", h.AuditPreviewHandler)
	h.mux.HandleFunc("POST /api/v1/batches/{batchID}/audits", h.CompleteAuditHandler)
	h.mux.HandleFunc("POST /api/v1/batches/{batchID}/corrections", h.CorrectItemHandler)
	h.mux.HandleFunc("POST /api/v1/batches/{batchID}/release", h.ReleaseBatchHandler)
	h.mux.HandleFunc("POST /api/v1/batches/{batchID}/verify", h.VerifyReleaseHandler)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	requestContext, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	r = r.WithContext(requestContext)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'")
	w.Header().Set("Cache-Control", "no-store")
	wrapped := &statusWriter{ResponseWriter: w, status: 200}
	h.mux.ServeHTTP(wrapped, r)
	h.logger.Info("访问", "method", r.Method, "path", r.URL.Path, "status", wrapped.status,
		"duration_ms", time.Since(started).Milliseconds(), "remote", remoteHost(r.RemoteAddr))
}

func remoteHost(value string) string {
	if index := strings.LastIndex(value, ":"); index > 0 {
		return value[:index]
	}
	return value
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
