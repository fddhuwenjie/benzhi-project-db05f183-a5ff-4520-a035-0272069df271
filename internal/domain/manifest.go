package domain

import (
	"sort"
	"strings"
	"time"
)

func (b *CorpusBatch) CanRelease() error {
	if err := b.requireState(StateAuditing); err != nil {
		return err
	}
	if len(b.Audits) == 0 || b.Audits[len(b.Audits)-1].Outcome != "PASSED" {
		return rule("audit_required", "最近审计必须通过")
	}
	for _, item := range b.Items {
		if item.ItemState != ItemReady && item.ItemState != ItemAdjudicated {
			return rule("not_ready", "仍有条目未达到发布状态")
		}
	}
	return nil
}

func BuildManifest(b *CorpusBatch, eventDigest, releasedBy string, now time.Time) (*ReleaseManifest, error) {
	if err := b.CanRelease(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(releasedBy) == "" {
		return nil, rule("validation", "发布人不能为空")
	}
	entries := ManifestItems(b)
	rubric := RubricDigest(b)
	manifest := &ReleaseManifest{ManifestID: "manifest-" + b.BatchID, BatchID: b.BatchID,
		BatchRevision: b.Revision + 1, ItemEntries: entries, RubricDigest: rubric,
		EventChainDigest: eventDigest, ReleasedBy: strings.TrimSpace(releasedBy), ReleasedAt: now.UTC()}
	manifest.ManifestDigest = ComputeManifestDigest(manifest)
	return manifest, nil
}

func ManifestItems(b *CorpusBatch) []ManifestItem {
	ids := make([]string, 0, len(b.Items))
	for id := range b.Items {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	entries := make([]ManifestItem, 0, len(ids))
	for _, id := range ids {
		item := b.Items[id]
		entries = append(entries, ManifestItem{ItemID: id, ContentDigest: item.ContentDigest,
			Transcript: item.AdjudicatedText, Labels: cloneLabels(item.AdjudicatedLabels)})
	}
	return entries
}

func RubricDigest(b *CorpusBatch) string {
	rubric := struct {
		Version        string
		Labels         []string
		Rules          string
		Minimum, Audit float64
	}{
		b.RubricVersion, cloneLabels(b.LabelSet), b.TranscriptionRules, b.MinimumAgreement, b.AuditRatio}
	return Digest(rubric)
}

func ComputeManifestDigest(manifest *ReleaseManifest) string {
	value := struct {
		ManifestID    string         `json:"manifest_id"`
		BatchID       string         `json:"batch_id"`
		BatchRevision int64          `json:"batch_revision"`
		Items         []ManifestItem `json:"item_entries"`
		Rubric        string         `json:"rubric_digest"`
		Events        string         `json:"event_chain_digest"`
		ReleasedBy    string         `json:"released_by"`
		ReleasedAt    time.Time      `json:"released_at"`
	}{manifest.ManifestID, manifest.BatchID, manifest.BatchRevision, manifest.ItemEntries, manifest.RubricDigest,
		manifest.EventChainDigest, manifest.ReleasedBy, manifest.ReleasedAt.UTC()}
	return Digest(value)
}

func (b *CorpusBatch) ApplyRelease(manifest *ReleaseManifest, now time.Time) error {
	if manifest.BatchID != b.BatchID {
		return rule("validation", "清单批次不匹配")
	}
	b.Manifest = manifest
	b.State = StateReleased
	b.UpdatedAt = now.UTC()
	return nil
}
