package verificationcachealias_test

import (
	"testing"
	"time"

	"dialectcorpusreleasegate/internal/application"
	"dialectcorpusreleasegate/internal/domain"
	"dialectcorpusreleasegate/internal/store"
)

func TestVerificationCacheMustIsolateCallerMutation(t *testing.T) {
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo, 2)
	now := time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)
	service.WithClock(func() time.Time { return now })

	created, err := service.Create(application.CreateBatchCommand{
		Metadata: application.Metadata{RequestID: "create", ExpectedRevision: -1},
		BatchID:  "verify-alias", Title: "验证缓存", DialectSite: "测试点",
		SourceNote: "私有复现", ItemRange: "001-001",
	})
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := service.Freeze("verify-alias", application.FreezeCommand{
		Metadata: application.Metadata{RequestID: "freeze", ExpectedRevision: created.Revision},
		FreezeInput: domain.FreezeInput{RubricVersion: "v1", LabelSet: []string{"词"},
			TranscriptionRules: "逐字转写", MinimumAgreement: 1, AuditRatio: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	registered, err := service.RegisterItem("verify-alias", application.RegisterItemCommand{
		Metadata: application.Metadata{RequestID: "register", ExpectedRevision: frozen.Revision},
		RegisterItemInput: domain.RegisterItemInput{ItemID: "001", SourceRef: "audio/001.wav",
			ContentDigest: "sha256-item-001", DurationMS: 1200, SpeakerCode: "SPK01",
			AnnotatorA: "ann-a", AnnotatorB: "ann-b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	firstSubmission, err := service.SubmitAnnotation("verify-alias", application.SubmitAnnotationCommand{
		Metadata: application.Metadata{RequestID: "submit-a", ExpectedRevision: registered.Revision},
		SubmitAnnotationInput: domain.SubmitAnnotationInput{ItemID: "001", Seat: "A", AnnotatorID: "ann-a",
			Transcript: "今天天气好", Labels: []string{"词"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	secondSubmission, err := service.SubmitAnnotation("verify-alias", application.SubmitAnnotationCommand{
		Metadata: application.Metadata{RequestID: "submit-b", ExpectedRevision: firstSubmission.Revision},
		SubmitAnnotationInput: domain.SubmitAnnotationInput{ItemID: "001", Seat: "B", AnnotatorID: "ann-b",
			Transcript: "今天天气好", Labels: []string{"词"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	audited, err := service.CompleteAudit("verify-alias", application.AuditCommand{
		Metadata:   application.Metadata{RequestID: "audit", ExpectedRevision: secondSubmission.Revision},
		SampleSeed: "stable-seed", AuditorID: "auditor",
		Findings: []domain.AuditFinding{{ItemID: "001", Passed: true, Note: "通过"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Release("verify-alias", application.ReleaseCommand{
		Metadata:   application.Metadata{RequestID: "release", ExpectedRevision: audited.Revision},
		ReleasedBy: "release-owner",
	}); err != nil {
		t.Fatal(err)
	}

	first, err := service.Verify("verify-alias")
	if err != nil {
		t.Fatal(err)
	}
	if !first.Valid {
		t.Fatalf("初次验证应通过：%+v", first)
	}
	first.Valid = false
	first.ManifestDigestMatch = false
	first.MismatchComponents = append(first.MismatchComponents, "caller-injected")

	second, err := service.Verify("verify-alias")
	if err != nil {
		t.Fatal(err)
	}
	if !second.Valid || !second.ManifestDigestMatch || len(second.MismatchComponents) != 0 {
		t.Fatalf("调用方修改污染了后续验证结果：%+v", second)
	}
}
