package httpui

import (
	"net/http"

	"dialectcorpusreleasegate/internal/application"
)

func (h *Handler) ListBatchesHandler(w http.ResponseWriter, r *http.Request) {
	values, err := h.service.List()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"batches": values})
}

func (h *Handler) CreateBatchHandler(w http.ResponseWriter, r *http.Request) {
	if !requireSameOrigin(w, r) {
		return
	}
	var command application.CreateBatchCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	result, err := h.service.Create(command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) BatchDetailHandler(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Detail(r.PathValue("batchID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) FreezeBatchHandler(w http.ResponseWriter, r *http.Request) {
	if !requireSameOrigin(w, r) {
		return
	}
	var command application.FreezeCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	result, err := h.service.Freeze(r.PathValue("batchID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) RegisterItemHandler(w http.ResponseWriter, r *http.Request) {
	if !requireSameOrigin(w, r) {
		return
	}
	var command application.RegisterItemCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	result, err := h.service.RegisterItem(r.PathValue("batchID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) SubmitAnnotationHandler(w http.ResponseWriter, r *http.Request) {
	if !requireSameOrigin(w, r) {
		return
	}
	var command application.SubmitAnnotationCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	result, err := h.service.SubmitAnnotation(r.PathValue("batchID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) ResolveDisagreementHandler(w http.ResponseWriter, r *http.Request) {
	if !requireSameOrigin(w, r) {
		return
	}
	var command application.ResolveCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	result, err := h.service.Resolve(r.PathValue("batchID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) AuditPreviewHandler(w http.ResponseWriter, r *http.Request) {
	if !requireSameOrigin(w, r) {
		return
	}
	var request struct {
		SampleSeed string `json:"sample_seed"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	result, err := h.service.AuditPreview(r.PathValue("batchID"), request.SampleSeed)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) CompleteAuditHandler(w http.ResponseWriter, r *http.Request) {
	if !requireSameOrigin(w, r) {
		return
	}
	var command application.AuditCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	result, err := h.service.CompleteAudit(r.PathValue("batchID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) CorrectItemHandler(w http.ResponseWriter, r *http.Request) {
	if !requireSameOrigin(w, r) {
		return
	}
	var command application.CorrectCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	result, err := h.service.Correct(r.PathValue("batchID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) ReleaseBatchHandler(w http.ResponseWriter, r *http.Request) {
	if !requireSameOrigin(w, r) {
		return
	}
	var command application.ReleaseCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	result, err := h.service.Release(r.PathValue("batchID"), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) VerifyReleaseHandler(w http.ResponseWriter, r *http.Request) {
	if !requireSameOrigin(w, r) {
		return
	}
	var request struct{}
	if !decodeJSON(w, r, &request) {
		return
	}
	result, err := h.service.Verify(r.PathValue("batchID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
