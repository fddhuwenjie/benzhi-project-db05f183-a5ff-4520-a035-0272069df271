package store

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"dialectcorpusreleasegate/internal/domain"
)

func TestCommitIdempotencyAndRevision(t *testing.T) {
	repo, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	batch, _ := domain.NewBatch(domain.CreateBatchInput{BatchID: "b1", Title: "t", DialectSite: "s", SourceNote: "n", ItemRange: "r"}, now)
	response := json.RawMessage(`{"revision":0}`)
	commit := Commit{Batch: batch, ExpectedRevision: -1, EventType: "created", RequestID: "r1", Payload: map[string]string{"x": "y"}, Fingerprint: "fp", Response: response, OccurredAt: now}
	if err := repo.Commit(commit); err != nil {
		t.Fatal(err)
	}
	loaded, err := repo.Load("b1")
	if err != nil || loaded.Revision != 0 {
		t.Fatalf("加载失败：%v %#v", err, loaded)
	}
	if err := repo.Commit(commit); err != nil {
		t.Fatalf("同请求重试失败：%v", err)
	}
	commit.Fingerprint = "different"
	if err := repo.Commit(commit); err == nil {
		t.Fatal("不同指纹未冲突")
	}
	loaded.Title = "new"
	commit = Commit{Batch: loaded, ExpectedRevision: 99, EventType: "changed", RequestID: "r2", Fingerprint: "fp2", OccurredAt: now}
	var conflict *RevisionConflict
	if err := repo.Commit(commit); !errors.As(err, &conflict) {
		t.Fatalf("未返回修订冲突：%v", err)
	}
}

func TestDetectsCorruptDigestAfterRestart(t *testing.T) {
	dir := t.TempDir()
	repo, _ := Open(dir)
	now := time.Now().UTC()
	batch, _ := domain.NewBatch(domain.CreateBatchInput{BatchID: "safe", Title: "t", DialectSite: "s", SourceNote: "n", ItemRange: "r"}, now)
	if err := repo.Commit(Commit{Batch: batch, ExpectedRevision: -1, EventType: "created", RequestID: "r", Fingerprint: "f", OccurredAt: now}); err != nil {
		t.Fatal(err)
	}
	path, _ := repo.path("safe")
	var value envelope
	if err := readJSON(path, &value); err != nil {
		t.Fatal(err)
	}
	value.Events[0].Digest = "tampered"
	if err := atomicJSON(path, &value); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.Load("safe"); err == nil {
		t.Fatal("损坏批次未隔离")
	}
	if len(reopened.Quarantined()) != 1 {
		t.Fatalf("隔离清单异常：%v", reopened.Quarantined())
	}
}
