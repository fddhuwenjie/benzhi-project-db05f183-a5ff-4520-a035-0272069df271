package trailingjson_test

import (
	"os"
	"path/filepath"
	"testing"

	"dialectcorpusreleasegate/internal/application"
	"dialectcorpusreleasegate/internal/store"
)

func TestTrailingJSONMustQuarantineBatch(t *testing.T) {
	root := t.TempDir()
	repo, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo, 1)
	_, err = service.Create(application.CreateBatchCommand{
		Metadata:    application.Metadata{RequestID: "create-1", ExpectedRevision: -1},
		BatchID:     "trailing-json",
		Title:       "尾随数据检查",
		DialectSite: "测试点",
		SourceNote:  "本地",
		ItemRange:   "u-001..u-001",
	})
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(root, "batches", "trailing-json", "aggregate.json")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.WriteString("\n{}\n"); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, quarantined := reopened.Quarantined()["trailing-json"]; !quarantined {
		t.Fatalf("TestTrailingJSONMustQuarantineBatch: aggregate.json 含尾随 JSON 值却未被隔离")
	}
}
