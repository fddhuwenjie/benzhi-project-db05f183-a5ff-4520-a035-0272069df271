package correctiongate_test

import (
	"testing"

	"dialectcorpusreleasegate/internal/application"
	"dialectcorpusreleasegate/internal/domain"
	"dialectcorpusreleasegate/internal/store"
)

func TestCompletedCorrectionGateMustStayCompleted(t *testing.T) {
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo, 1)
	_, err = service.Create(application.CreateBatchCommand{Metadata: application.Metadata{RequestID: "create", ExpectedRevision: -1},
		BatchID: "gate-history", Title: "门禁历史", DialectSite: "测试点", SourceNote: "本地", ItemRange: "u-001..u-001"})
	must(t, err)
	_, err = service.Freeze("gate-history", application.FreezeCommand{Metadata: application.Metadata{RequestID: "freeze", ExpectedRevision: 0},
		FreezeInput: domain.FreezeInput{RubricVersion: "v1", LabelSet: []string{"陈述"}, TranscriptionRules: "按原文", MinimumAgreement: 1, AuditRatio: 1}})
	must(t, err)
	_, err = service.RegisterItem("gate-history", application.RegisterItemCommand{Metadata: application.Metadata{RequestID: "item", ExpectedRevision: 1},
		RegisterItemInput: domain.RegisterItemInput{ItemID: "u-001", SourceRef: "source", ContentDigest: "digest", DurationMS: 1000,
			SpeakerCode: "speaker", AnnotatorA: "ann-a", AnnotatorB: "ann-b"}})
	must(t, err)
	for index, input := range []domain.SubmitAnnotationInput{
		{SubmissionID: "sub-a", ItemID: "u-001", Seat: "A", AnnotatorID: "ann-a", Transcript: "文本", Labels: []string{"陈述"}},
		{SubmissionID: "sub-b", ItemID: "u-001", Seat: "B", AnnotatorID: "ann-b", Transcript: "文本", Labels: []string{"陈述"}},
	} {
		_, err = service.SubmitAnnotation("gate-history", application.SubmitAnnotationCommand{
			Metadata: application.Metadata{RequestID: "sub-" + input.Seat, ExpectedRevision: int64(2 + index)}, SubmitAnnotationInput: input})
		must(t, err)
	}
	_, err = service.CompleteAudit("gate-history", application.AuditCommand{Metadata: application.Metadata{RequestID: "audit-fail", ExpectedRevision: 4},
		SampleSeed: "seed-1", Findings: []domain.AuditFinding{{ItemID: "u-001", Passed: false, Note: "需返修"}}, AuditorID: "auditor-1"})
	must(t, err)
	_, err = service.Correct("gate-history", application.CorrectCommand{Metadata: application.Metadata{RequestID: "correct", ExpectedRevision: 5},
		CorrectInput: domain.CorrectInput{ItemID: "u-001", Transcript: "修订文本", Labels: []string{"陈述"}, Reason: "审计意见", CorrectorID: "corrector"}})
	must(t, err)
	_, err = service.CompleteAudit("gate-history", application.AuditCommand{Metadata: application.Metadata{RequestID: "audit-pass", ExpectedRevision: 6},
		SampleSeed: "seed-2", Findings: []domain.AuditFinding{{ItemID: "u-001", Passed: true, Note: "返修通过"}}, AuditorID: "auditor-2"})
	must(t, err)

	detail, err := service.Detail("gate-history")
	must(t, err)
	for _, gate := range detail.Workbench.Gates {
		if gate.State == domain.StateCorrection {
			if gate.Status != "COMPLETED" {
				t.Fatalf("TestCompletedCorrectionGateMustStayCompleted: 定向返修已完成并复审通过，门禁却回退为 %s", gate.Status)
			}
			return
		}
	}
	t.Fatal("TestCompletedCorrectionGateMustStayCompleted: 缺少返修门禁")
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
