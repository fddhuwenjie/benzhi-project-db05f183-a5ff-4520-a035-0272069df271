package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"dialectcorpusreleasegate/internal/domain"
)

type Repository struct {
	root        string
	mu          sync.RWMutex
	quarantined map[string]string
	cache       map[string]*envelope
}

func Open(root string) (*Repository, error) {
	if root == "" {
		return nil, fmt.Errorf("数据目录不能为空")
	}
	if err := os.MkdirAll(filepath.Join(root, "batches"), 0o750); err != nil {
		return nil, err
	}
	repo := &Repository{root: root, quarantined: map[string]string{}, cache: map[string]*envelope{}}
	if err := repo.scan(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *Repository) path(batchID string) (string, error) {
	id, err := safeID(batchID)
	if err != nil {
		return "", err
	}
	return filepath.Join(r.root, "batches", id, "aggregate.json"), nil
}

func (r *Repository) loadUnlocked(batchID string) (*envelope, error) {
	if reason := r.quarantined[batchID]; reason != "" {
		return nil, &CorruptBatch{batchID, reason}
	}
	if cached := r.cache[batchID]; cached != nil {
		return cached, nil
	}
	path, err := r.path(batchID)
	if err != nil {
		return nil, err
	}
	value := &envelope{}
	if err := readJSON(path, value); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if value.Idempotency == nil {
		value.Idempotency = map[string]IdempotencyRecord{}
	}
	if err := validateEnvelope(value); err != nil {
		r.quarantined[batchID] = err.Error()
		return nil, &CorruptBatch{batchID, err.Error()}
	}
	r.cache[batchID] = value
	return value, nil
}

func (r *Repository) Load(batchID string) (*domain.CorpusBatch, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, err := r.loadUnlocked(batchID)
	if err != nil {
		return nil, err
	}
	return cloneBatch(value.Batch)
}

func (r *Repository) List() ([]*domain.CorpusBatch, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entries, err := os.ReadDir(filepath.Join(r.root, "batches"))
	if err != nil {
		return nil, err
	}
	out := make([]*domain.CorpusBatch, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || r.quarantined[entry.Name()] != "" {
			continue
		}
		value, loadErr := r.loadUnlocked(entry.Name())
		if loadErr == nil {
			batch, _ := cloneBatch(value.Batch)
			out = append(out, batch)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func cloneBatch(batch *domain.CorpusBatch) (*domain.CorpusBatch, error) {
	data, err := json.Marshal(batch)
	if err != nil {
		return nil, err
	}
	var cloned domain.CorpusBatch
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil, err
	}
	return &cloned, nil
}

// cloneEnvelope 深拷贝聚合信封，使写入路径可在持久化前修改副本而不污染缓存。
func cloneEnvelope(value *envelope) *envelope {
	if value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var cloned envelope
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil
	}
	if cloned.Idempotency == nil {
		cloned.Idempotency = map[string]IdempotencyRecord{}
	}
	return &cloned
}
