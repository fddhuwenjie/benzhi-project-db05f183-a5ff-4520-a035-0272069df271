package application

import (
	"errors"
	"sort"

	"dialectcorpusreleasegate/internal/domain"
	"dialectcorpusreleasegate/internal/store"
)

type BatchDetail struct {
	Batch                *domain.CorpusBatch        `json:"batch"`
	PendingDisagreements []*domain.DisagreementCase `json:"pending_disagreements"`
	AuditProgress        AuditProgress              `json:"audit_progress"`
	Timeline             store.Timeline             `json:"timeline"`
	Workbench            WorkbenchProjection        `json:"workbench"`
	DisagreementSummary  DisagreementSummary        `json:"disagreement_summary"`
	ManifestSummary      *ManifestSummary           `json:"manifest_summary,omitempty"`
}

type DisagreementSummary struct {
	Open             int      `json:"open"`
	Closed           int      `json:"closed"`
	EvidenceVersions []string `json:"evidence_versions"`
}

type ManifestSummary struct {
	TotalItems       int    `json:"total_items"`
	ReleaseRevision  int64  `json:"release_revision"`
	ReleasedBy       string `json:"released_by"`
	ManifestPrefix   string `json:"manifest_digest_prefix"`
	RubricPrefix     string `json:"rubric_digest_prefix"`
	EventChainPrefix string `json:"event_chain_digest_prefix"`
	IntegrityValid   bool   `json:"integrity_valid"`
}

type AuditProgress struct {
	Rounds            int      `json:"rounds"`
	LastOutcome       string   `json:"last_outcome,omitempty"`
	CorrectionPending []string `json:"correction_pending"`
}

func (s *Service) Detail(batchID string) (*BatchDetail, error) {
	if err := validateBatchID(batchID); err != nil {
		return nil, err
	}
	batch, err := s.repo.Load(batchID)
	if err != nil {
		return nil, classify(err)
	}
	if batch.State == domain.StateReleased {
		stored, manifestErr := s.repo.LoadManifest(batchID)
		if manifestErr == nil {
			batch.Manifest = stored
		} else if !errors.Is(manifestErr, store.ErrNotFound) {
			return nil, classify(manifestErr)
		}
	}
	timeline, err := s.repo.Timeline(batchID)
	if err != nil {
		return nil, classify(err)
	}
	pending := make([]*domain.DisagreementCase, 0)
	disagreementSummary := DisagreementSummary{EvidenceVersions: []string{}}
	versions := map[string]bool{}
	for _, value := range batch.Disagreements {
		if value.ResolvedAt.IsZero() {
			pending = append(pending, value)
			disagreementSummary.Open++
		} else {
			disagreementSummary.Closed++
			if !versions[value.EvidenceVersion] {
				versions[value.EvidenceVersion] = true
				disagreementSummary.EvidenceVersions = append(disagreementSummary.EvidenceVersions, value.EvidenceVersion)
			}
		}
	}
	sort.Strings(disagreementSummary.EvidenceVersions)
	sort.Slice(pending, func(i, j int) bool { return pending[i].DisagreementID < pending[j].DisagreementID })
	progress := AuditProgress{Rounds: len(batch.Audits), CorrectionPending: []string{}}
	if len(batch.Audits) > 0 {
		last := batch.Audits[len(batch.Audits)-1]
		progress.LastOutcome = last.Outcome
		for _, id := range last.CorrectionItemIDs {
			if batch.Items[id].ItemState == domain.ItemCorrection {
				progress.CorrectionPending = append(progress.CorrectionPending, id)
			}
		}
	}
	result := &BatchDetail{Batch: batch, PendingDisagreements: pending, AuditProgress: progress, Timeline: timeline,
		Workbench: BuildWorkbenchProjection(batch), DisagreementSummary: disagreementSummary}
	if batch.Manifest != nil {
		result.ManifestSummary = &ManifestSummary{TotalItems: len(batch.Manifest.ItemEntries), ReleaseRevision: batch.Manifest.BatchRevision,
			ReleasedBy: batch.Manifest.ReleasedBy, ManifestPrefix: digestPrefix(batch.Manifest.ManifestDigest),
			RubricPrefix: digestPrefix(batch.Manifest.RubricDigest), EventChainPrefix: digestPrefix(batch.Manifest.EventChainDigest),
			IntegrityValid: manifestIntegrityValid(batch.Manifest)}
	}
	return result, nil
}

func manifestIntegrityValid(manifest *domain.ReleaseManifest) bool {
	if manifest == nil || manifest.ManifestID == "" || manifest.BatchID == "" || manifest.RubricDigest == "" || manifest.EventChainDigest == "" {
		return false
	}
	for index := 1; index < len(manifest.ItemEntries); index++ {
		if manifest.ItemEntries[index-1].ItemID >= manifest.ItemEntries[index].ItemID {
			return false
		}
	}
	return manifest.ManifestDigest == domain.ComputeManifestDigest(manifest)
}

func digestPrefix(value string) string {
	if len(value) <= 12 {
		return value
	}
	return value[:12]
}

func (s *Service) List() ([]*domain.CorpusBatch, error) {
	values, err := s.repo.List()
	if err != nil {
		return nil, classify(err)
	}
	return values, nil
}

func (s *Service) AuditPreview(batchID, seed string) (*domain.AuditRound, error) {
	if err := validateBatchID(batchID); err != nil {
		return nil, err
	}
	batch, err := s.repo.Load(batchID)
	if err != nil {
		return nil, classify(err)
	}
	round, err := batch.BeginAudit(seed)
	if err != nil {
		return nil, classify(err)
	}
	return round, nil
}
