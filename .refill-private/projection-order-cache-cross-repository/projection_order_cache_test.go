package projection_order_cache_cross_repository_test

import (
	"testing"

	"dialectcorpusreleasegate/internal/application"
	"dialectcorpusreleasegate/internal/domain"
	"dialectcorpusreleasegate/internal/store"
)

func TestProjectionOrderCacheMustNotCrossRepositories(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("第二个独立 Repository 的详情投影发生崩溃：%v", recovered)
		}
	}()

	first := serviceWithSingleItem(t, "001-001", "001")
	firstDetail, err := first.Detail("shared-batch")
	if err != nil {
		t.Fatal(err)
	}
	if got := firstDetail.Workbench.Items[0].ItemID; got != "001" {
		t.Fatalf("首个 Repository 的条目异常：%s", got)
	}

	second := serviceWithSingleItem(t, "002-002", "002")
	secondDetail, err := second.Detail("shared-batch")
	if err != nil {
		t.Fatal(err)
	}
	if len(secondDetail.Workbench.Items) != 1 || secondDetail.Workbench.Items[0].ItemID != "002" {
		t.Fatalf("第二个 Repository 复用了其他生命周期的条目顺序：%+v", secondDetail.Workbench.Items)
	}
}

func serviceWithSingleItem(t *testing.T, itemRange, itemID string) *application.Service {
	t.Helper()
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo, 1)
	created, err := service.Create(application.CreateBatchCommand{
		Metadata: application.Metadata{RequestID: "create", ExpectedRevision: -1},
		BatchID:  "shared-batch", Title: "批次", DialectSite: "语言点", SourceNote: "来源", ItemRange: itemRange,
	})
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := service.Freeze("shared-batch", application.FreezeCommand{
		Metadata:    application.Metadata{RequestID: "freeze", ExpectedRevision: created.Revision},
		FreezeInput: domain.FreezeInput{RubricVersion: "v1", LabelSet: []string{"A"}, TranscriptionRules: "规则", MinimumAgreement: 1, AuditRatio: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.RegisterItem("shared-batch", application.RegisterItemCommand{
		Metadata: application.Metadata{RequestID: "register", ExpectedRevision: frozen.Revision},
		RegisterItemInput: domain.RegisterItemInput{ItemID: itemID, SourceRef: "audio-" + itemID, ContentDigest: "digest-" + itemID,
			DurationMS: 1000, SpeakerCode: "SPK", AnnotatorA: "annotator-a", AnnotatorB: "annotator-b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}
