package application

import (
	"errors"

	"dialectcorpusreleasegate/internal/domain"
	"dialectcorpusreleasegate/internal/store"
)

type AppError struct {
	Code            string `json:"code"`
	Message         string `json:"message"`
	CurrentRevision *int64 `json:"current_revision,omitempty"`
}

func (e *AppError) Error() string { return e.Message }

func classify(err error) *AppError {
	if err == nil {
		return nil
	}
	var revision *store.RevisionConflict
	if errors.As(err, &revision) {
		current := revision.Actual
		return &AppError{Code: "revision_conflict", Message: err.Error(), CurrentRevision: &current}
	}
	var idempotency *store.IdempotencyConflict
	if errors.As(err, &idempotency) {
		return &AppError{Code: "idempotency_conflict", Message: err.Error()}
	}
	var corrupt *store.CorruptBatch
	if errors.As(err, &corrupt) {
		return &AppError{Code: "batch_quarantined", Message: err.Error()}
	}
	var invalidBatchID *store.InvalidBatchID
	if errors.As(err, &invalidBatchID) {
		return &AppError{Code: "validation", Message: err.Error()}
	}
	if errors.Is(err, store.ErrNotFound) {
		return &AppError{Code: "not_found", Message: "批次不存在"}
	}
	if code := domain.ErrorCode(err); code != "internal_error" {
		return &AppError{Code: code, Message: err.Error()}
	}
	return &AppError{Code: "internal_error", Message: err.Error()}
}
