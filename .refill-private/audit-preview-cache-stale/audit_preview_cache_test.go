package audit_preview_cache_stale_test

import (
	"slices"
	"testing"
	"time"

	"dialectcorpusreleasegate/internal/application"
	"dialectcorpusreleasegate/internal/domain"
	"dialectcorpusreleasegate/internal/store"
)

func TestAuditPreviewCacheMustFollowCorrectionScope(t *testing.T) {
	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	batch := auditReadyBatch(now)
	if err := repo.Commit(store.Commit{
		Batch: batch, ExpectedRevision: -1, EventType: "fixture.created", RequestID: "fixture-create",
		Payload: map[string]string{"state": "AUDITING"}, Fingerprint: "fixture", OccurredAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo, 2)
	service.WithClock(func() time.Time { return now })

	first, err := service.AuditPreview(batch.BatchID, "reused-seed")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(first.SampleItemIDs, []string{"u-001", "u-002"}) {
		t.Fatalf("首轮全量样本异常：%v", first.SampleItemIDs)
	}
	completed, err := service.CompleteAudit(batch.BatchID, application.AuditCommand{
		Metadata:   application.Metadata{RequestID: "audit-failed", ExpectedRevision: 0},
		SampleSeed: "reused-seed", AuditorID: "independent-auditor",
		Findings: []domain.AuditFinding{
			{ItemID: "u-001", Passed: false, Note: "需要返修"},
			{ItemID: "u-002", Passed: true, Note: "检查通过"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Correct(batch.BatchID, application.CorrectCommand{
		Metadata: application.Metadata{RequestID: "correct-one", ExpectedRevision: completed.Revision},
		CorrectInput: domain.CorrectInput{ItemID: "u-001", Transcript: "返修文本", Labels: []string{"陈述"},
			Reason: "修复审计问题", CorrectorID: "corrector"},
	}); err != nil {
		t.Fatal(err)
	}

	focused, err := service.AuditPreview(batch.BatchID, "reused-seed")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(focused.SampleItemIDs, []string{"u-001"}) {
		t.Fatalf("返修后复用 seed 必须按当前批次状态重算定向范围，got %v", focused.SampleItemIDs)
	}
}

func auditReadyBatch(now time.Time) *domain.CorpusBatch {
	items := map[string]*domain.UtteranceItem{}
	for index, id := range []string{"u-001", "u-002"} {
		items[id] = &domain.UtteranceItem{
			ItemID: id, BatchID: "preview-cache-batch", SourceRef: "audio/" + id + ".wav",
			ContentDigest: "sha256-" + id, DurationMS: int64(1000 + index), SpeakerCode: "SPK",
			AnnotatorA: "ann-a-" + id, AnnotatorB: "ann-b-" + id, ItemState: domain.ItemReady,
			PreflightStatus: "PASSED", PreflightWarnings: []string{}, CanAnnotate: true, RegisteredAt: now,
			AdjudicatedText: "已裁决文本", AdjudicatedLabels: []string{"陈述"},
		}
	}
	return &domain.CorpusBatch{
		BatchID: "preview-cache-batch", Title: "缓存复现批次", DialectSite: "测试语言点",
		SourceNote: "确定性私有复现", ItemRange: "u-001..u-002", State: domain.StateAuditing,
		RubricVersion: "rubric-v1", LabelSet: []string{"陈述"}, TranscriptionRules: "按规范转写",
		MinimumAgreement: 1, AuditRatio: 1, CreatedAt: now, UpdatedAt: now, Items: items,
		Submissions: map[string]domain.AnnotationSubmission{}, Disagreements: map[string]*domain.DisagreementCase{}, Audits: []domain.AuditRound{},
	}
}
