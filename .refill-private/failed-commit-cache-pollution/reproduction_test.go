package failed_commit_cache_pollution

import (
	"os"
	"path/filepath"
	"testing"

	"dialectcorpusreleasegate/internal/application"
	"dialectcorpusreleasegate/internal/domain"
	"dialectcorpusreleasegate/internal/store"
)

func TestFailedCommitMustNotPolluteRepositoryCache(t *testing.T) {
	root := t.TempDir()
	repo, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo, 2)
	created, err := service.Create(application.CreateBatchCommand{
		Metadata:    application.Metadata{RequestID: "create-cache-case", ExpectedRevision: -1},
		BatchID:     "cache-case",
		Title:       "缓存事务边界",
		DialectSite: "测试语言点",
		SourceNote:  "测试来源",
		ItemRange:   "001-001",
	})
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := service.Freeze("cache-case", application.FreezeCommand{
		Metadata: application.Metadata{RequestID: "freeze-cache-case", ExpectedRevision: created.Revision},
		FreezeInput: domain.FreezeInput{
			RubricVersion: "v1", LabelSet: []string{"陈述"}, TranscriptionRules: "逐字转写",
			MinimumAgreement: 1, AuditRatio: 1,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	aggregatePath := filepath.Join(root, "batches", "cache-case", "aggregate.json")
	original, err := os.ReadFile(aggregatePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(aggregatePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(aggregatePath, 0o750); err != nil {
		t.Fatal(err)
	}

	_, commitErr := service.RegisterItem("cache-case", application.RegisterItemCommand{
		Metadata: application.Metadata{RequestID: "register-cache-case", ExpectedRevision: frozen.Revision},
		RegisterItemInput: domain.RegisterItemInput{
			ItemID: "001", SourceRef: "audio/001.wav", ContentDigest: "sha256:001", DurationMS: 1000,
			SpeakerCode: "SPK-1", AnnotatorA: "ann-a", AnnotatorB: "ann-b",
		},
	})
	if commitErr == nil {
		t.Fatal("测试前置条件失败：目标原子替换应当失败")
	}

	if err := os.Remove(aggregatePath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(aggregatePath, original, 0o640); err != nil {
		t.Fatal(err)
	}
	detail, err := service.Detail("cache-case")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Batch.Revision != frozen.Revision || len(detail.Batch.Items) != 0 || len(detail.Timeline.Events) != 2 {
		t.Fatalf("TestFailedCommitMustNotPolluteRepositoryCache: 写失败后仍看到未落盘状态 revision=%d items=%d events=%d",
			detail.Batch.Revision, len(detail.Batch.Items), len(detail.Timeline.Events))
	}
}
