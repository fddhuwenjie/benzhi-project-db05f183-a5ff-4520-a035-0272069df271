package domain

import "time"

type BatchState string

const (
	StateDraft        BatchState = "DRAFT"
	StateAnnotating   BatchState = "ANNOTATING"
	StateAdjudicating BatchState = "ADJUDICATING"
	StateAuditing     BatchState = "AUDITING"
	StateCorrection   BatchState = "CORRECTION"
	StateReleased     BatchState = "RELEASED"
)

type CorpusBatch struct {
	BatchID            string                          `json:"batch_id"`
	Title              string                          `json:"title"`
	DialectSite        string                          `json:"dialect_site"`
	SourceNote         string                          `json:"source_note"`
	ItemRange          string                          `json:"item_range"`
	State              BatchState                      `json:"state"`
	RubricVersion      string                          `json:"rubric_version"`
	LabelSet           []string                        `json:"label_set"`
	TranscriptionRules string                          `json:"transcription_rules"`
	MinimumAgreement   float64                         `json:"minimum_agreement"`
	AuditRatio         float64                         `json:"audit_ratio"`
	Revision           int64                           `json:"revision"`
	CreatedAt          time.Time                       `json:"created_at"`
	UpdatedAt          time.Time                       `json:"updated_at"`
	Items              map[string]*UtteranceItem       `json:"items"`
	Submissions        map[string]AnnotationSubmission `json:"submissions"`
	Disagreements      map[string]*DisagreementCase    `json:"disagreements"`
	Audits             []AuditRound                    `json:"audits"`
	Manifest           *ReleaseManifest                `json:"manifest,omitempty"`
}

type ItemState string

const (
	ItemRegistered  ItemState = "REGISTERED"
	ItemSubmitted   ItemState = "SUBMITTED"
	ItemDisputed    ItemState = "DISPUTED"
	ItemAdjudicated ItemState = "ADJUDICATED"
	ItemCorrection  ItemState = "CORRECTION"
	ItemReady       ItemState = "READY"
)

type UtteranceItem struct {
	ItemID            string              `json:"item_id"`
	BatchID           string              `json:"batch_id"`
	SourceRef         string              `json:"source_ref"`
	ContentDigest     string              `json:"content_digest"`
	DurationMS        int64               `json:"duration_ms"`
	SpeakerCode       string              `json:"speaker_code"`
	AnnotatorA        string              `json:"annotator_a"`
	AnnotatorB        string              `json:"annotator_b"`
	ItemState         ItemState           `json:"item_state"`
	PreflightStatus   string              `json:"preflight_status"`
	PreflightWarnings []string            `json:"preflight_warnings"`
	CanAnnotate       bool                `json:"can_annotate"`
	RegisteredAt      time.Time           `json:"registered_at"`
	AdjudicatedText   string              `json:"adjudicated_text"`
	AdjudicatedLabels []string            `json:"adjudicated_labels"`
	Agreement         float64             `json:"agreement"`
	AgreementStats    AgreementStatistics `json:"agreement_statistics"`
}

type AgreementStatistics struct {
	Compared            bool    `json:"compared"`
	TranscriptAgreement bool    `json:"transcript_agreement"`
	LabelsAgreement     bool    `json:"labels_agreement"`
	Overall             float64 `json:"overall"`
}

type AnnotationSubmission struct {
	SubmissionID     string    `json:"submission_id"`
	BatchID          string    `json:"batch_id"`
	ItemID           string    `json:"item_id"`
	Seat             string    `json:"seat"`
	AnnotatorID      string    `json:"annotator_id"`
	Transcript       string    `json:"transcript"`
	Labels           []string  `json:"labels"`
	RubricVersion    string    `json:"rubric_version"`
	SubmittedAt      time.Time `json:"submitted_at"`
	SubmissionDigest string    `json:"submission_digest"`
}

type DisagreementCase struct {
	DisagreementID  string    `json:"disagreement_id"`
	BatchID         string    `json:"batch_id"`
	ItemID          string    `json:"item_id"`
	FieldPath       string    `json:"field_path"`
	ValueA          string    `json:"value_a"`
	ValueB          string    `json:"value_b"`
	Resolution      string    `json:"resolution"`
	Reason          string    `json:"reason"`
	AdjudicatorID   string    `json:"adjudicator_id"`
	EvidenceVersion string    `json:"evidence_version"`
	ResolvedAt      time.Time `json:"resolved_at"`
}

type AuditFinding struct {
	ItemID string `json:"item_id"`
	Passed bool   `json:"passed"`
	Note   string `json:"note"`
}

type AuditRound struct {
	AuditID           string         `json:"audit_id"`
	BatchID           string         `json:"batch_id"`
	RoundNumber       int            `json:"round_number"`
	SampleSeed        string         `json:"sample_seed"`
	SampleItemIDs     []string       `json:"sample_item_ids"`
	Findings          []AuditFinding `json:"findings"`
	Outcome           string         `json:"outcome"`
	CorrectionItemIDs []string       `json:"correction_item_ids"`
	AuditorID         string         `json:"auditor_id"`
	CompletedAt       time.Time      `json:"completed_at"`
	SampleCount       int            `json:"sample_count"`
	PassedCount       int            `json:"passed_count"`
	FailedCount       int            `json:"failed_count"`
	PassRate          float64        `json:"pass_rate"`
	CorrectionStatus  string         `json:"correction_status"`
}

type ManifestItem struct {
	ItemID        string   `json:"item_id"`
	ContentDigest string   `json:"content_digest"`
	Transcript    string   `json:"transcript"`
	Labels        []string `json:"labels"`
}

type ReleaseManifest struct {
	ManifestID       string         `json:"manifest_id"`
	BatchID          string         `json:"batch_id"`
	BatchRevision    int64          `json:"batch_revision"`
	ItemEntries      []ManifestItem `json:"item_entries"`
	RubricDigest     string         `json:"rubric_digest"`
	EventChainDigest string         `json:"event_chain_digest"`
	ManifestDigest   string         `json:"manifest_digest"`
	ReleasedBy       string         `json:"released_by"`
	ReleasedAt       time.Time      `json:"released_at"`
}
