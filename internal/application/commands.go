package application

import (
	"time"

	"dialectcorpusreleasegate/internal/domain"
)

type Metadata struct {
	RequestID        string `json:"request_id"`
	ExpectedRevision int64  `json:"expected_revision"`
}

type CreateBatchCommand struct {
	Metadata
	BatchID     string `json:"batch_id"`
	Title       string `json:"title"`
	DialectSite string `json:"dialect_site"`
	SourceNote  string `json:"source_note"`
	ItemRange   string `json:"item_range"`
}

type FreezeCommand struct {
	Metadata
	domain.FreezeInput
}
type RegisterItemCommand struct {
	Metadata
	domain.RegisterItemInput
}
type SubmitAnnotationCommand struct {
	Metadata
	domain.SubmitAnnotationInput
}
type ResolveCommand struct {
	Metadata
	domain.ResolveInput
}

type AuditCommand struct {
	Metadata
	SampleSeed string                `json:"sample_seed"`
	Findings   []domain.AuditFinding `json:"findings"`
	AuditorID  string                `json:"auditor_id"`
}

type CorrectCommand struct {
	Metadata
	domain.CorrectInput
}
type ReleaseCommand struct {
	Metadata
	ReleasedBy string `json:"released_by"`
}

type MutationResult struct {
	BatchID   string                  `json:"batch_id"`
	Revision  int64                   `json:"revision"`
	State     domain.BatchState       `json:"state"`
	EventType string                  `json:"event_type"`
	Replayed  bool                    `json:"replayed,omitempty"`
	Audit     *domain.AuditRound      `json:"audit,omitempty"`
	Manifest  *domain.ReleaseManifest `json:"manifest,omitempty"`
	At        time.Time               `json:"at"`
}

type Verification struct {
	BatchID               string   `json:"batch_id"`
	Valid                 bool     `json:"valid"`
	ManifestDigestMatch   bool     `json:"manifest_digest_match"`
	RubricDigestMatch     bool     `json:"rubric_digest_match"`
	EventChainDigestMatch bool     `json:"event_chain_digest_match"`
	StoredDigest          string   `json:"stored_digest"`
	ComputedDigest        string   `json:"computed_digest"`
	MismatchComponents    []string `json:"mismatch_components"`
	MismatchItemIDs       []string `json:"mismatch_item_ids"`
}
