package store

import (
	"encoding/json"
	"time"

	"dialectcorpusreleasegate/internal/domain"
)

type EventFrame struct {
	Sequence       int64           `json:"sequence"`
	BatchRevision  int64           `json:"batch_revision"`
	EventType      string          `json:"event_type"`
	OccurredAt     time.Time       `json:"occurred_at"`
	RequestID      string          `json:"request_id"`
	Payload        json.RawMessage `json:"payload"`
	PayloadLength  int             `json:"payload_length"`
	PreviousDigest string          `json:"previous_digest"`
	Digest         string          `json:"digest"`
}

type IdempotencyRecord struct {
	RequestID   string          `json:"request_id"`
	Fingerprint string          `json:"fingerprint"`
	Status      int             `json:"status"`
	Response    json.RawMessage `json:"response"`
	CreatedAt   time.Time       `json:"created_at"`
}

type envelope struct {
	Batch       *domain.CorpusBatch          `json:"batch"`
	Events      []EventFrame                 `json:"events"`
	Idempotency map[string]IdempotencyRecord `json:"idempotency"`
}

type Commit struct {
	Batch            *domain.CorpusBatch
	ExpectedRevision int64
	EventType        string
	RequestID        string
	Payload          any
	Fingerprint      string
	Status           int
	Response         json.RawMessage
	OccurredAt       time.Time
	Manifest         *domain.ReleaseManifest
}

type Timeline struct {
	Events      []EventFrame `json:"events"`
	ChainDigest string       `json:"chain_digest"`
}
