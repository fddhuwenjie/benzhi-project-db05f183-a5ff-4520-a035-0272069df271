package application

import (
	"testing"

	"dialectcorpusreleasegate/internal/domain"
	"dialectcorpusreleasegate/internal/store"
)

func TestFailedRegistrationIsIdempotentWithoutRevisionChange(t *testing.T) {
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(repo, 2)
	created, err := service.Create(CreateBatchCommand{Metadata: Metadata{RequestID: "create", ExpectedRevision: -1},
		BatchID: "b1", Title: "批次", DialectSite: "语言点", SourceNote: "来源", ItemRange: "001-002"})
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := service.Freeze("b1", FreezeCommand{Metadata: Metadata{RequestID: "freeze", ExpectedRevision: created.Revision},
		FreezeInput: domain.FreezeInput{RubricVersion: "v1", LabelSet: []string{"A"}, TranscriptionRules: "规则", MinimumAgreement: 1, AuditRatio: 1}})
	if err != nil {
		t.Fatal(err)
	}
	invalid := RegisterItemCommand{Metadata: Metadata{RequestID: "invalid-item", ExpectedRevision: frozen.Revision},
		RegisterItemInput: domain.RegisterItemInput{ItemID: "003", SourceRef: "a", ContentDigest: "d", DurationMS: 0,
			SpeakerCode: "SPK", AnnotatorA: "one", AnnotatorB: "two"}}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := service.RegisterItem("b1", invalid); err == nil || err.(*AppError).Code != "invalid_duration" {
			t.Fatalf("第 %d 次失败响应异常：%v", attempt+1, err)
		}
	}
	detail, err := service.Detail("b1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Batch.Revision != frozen.Revision || len(detail.Batch.Items) != 0 || len(detail.Timeline.Events) != 2 {
		t.Fatalf("失败命令改变了业务事实：revision=%d items=%d events=%d", detail.Batch.Revision, len(detail.Batch.Items), len(detail.Timeline.Events))
	}
}
