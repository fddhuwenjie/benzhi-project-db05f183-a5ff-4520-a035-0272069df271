package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"dialectcorpusreleasegate/internal/domain"
)

func (r *Repository) Commit(input Commit) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	path, err := r.path(input.Batch.BatchID)
	if err != nil {
		return err
	}
	var value *envelope
	current, loadErr := r.loadUnlocked(input.Batch.BatchID)
	if errors.Is(loadErr, ErrNotFound) {
		if input.ExpectedRevision != -1 {
			return &RevisionConflict{input.ExpectedRevision, -1}
		}
		value = &envelope{Idempotency: map[string]IdempotencyRecord{}}
	} else if loadErr != nil {
		return loadErr
	} else {
		value = cloneEnvelope(current)
		if value == nil {
			return errors.New("聚合快照深拷贝失败")
		}
	}
	if existing, ok := value.Idempotency[input.RequestID]; ok {
		if existing.Fingerprint != input.Fingerprint {
			return &IdempotencyConflict{input.RequestID}
		}
		return nil
	}
	if value.Batch != nil && value.Batch.Revision != input.ExpectedRevision {
		return &RevisionConflict{input.ExpectedRevision, value.Batch.Revision}
	}
	manifestPath := ""
	if input.Manifest != nil {
		id, safeErr := safeID(input.Manifest.BatchID)
		if safeErr != nil {
			return safeErr
		}
		manifestPath = filepath.Join(r.root, "batches", id, "manifest.json")
		if exists(manifestPath) {
			return errors.New("不可变发布清单已经存在")
		}
	}
	payload, err := json.Marshal(input.Payload)
	if err != nil {
		return err
	}
	previous := ""
	if len(value.Events) > 0 {
		previous = value.Events[len(value.Events)-1].Digest
	}
	input.Batch.Revision = input.ExpectedRevision + 1
	input.Batch.UpdatedAt = input.OccurredAt.UTC()
	frame := EventFrame{Sequence: int64(len(value.Events) + 1), BatchRevision: input.Batch.Revision,
		EventType: input.EventType, OccurredAt: input.OccurredAt.UTC(), RequestID: input.RequestID,
		Payload: payload, PayloadLength: len(payload), PreviousDigest: previous}
	frame.Digest = eventDigest(frame)
	value.Batch = input.Batch
	value.Events = append(value.Events, frame)
	value.Idempotency[input.RequestID] = IdempotencyRecord{RequestID: input.RequestID, Fingerprint: input.Fingerprint,
		Status: input.Status, Response: append([]byte(nil), input.Response...), CreatedAt: input.OccurredAt.UTC()}
	if err := atomicJSON(path, value); err != nil {
		return err
	}
	if input.Manifest != nil {
		if err := atomicJSON(manifestPath, input.Manifest); err != nil {
			return err
		}
	}
	r.cache[input.Batch.BatchID] = value
	return nil
}

func (r *Repository) LookupRequest(batchID, requestID, fingerprint string) (*IdempotencyRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, err := r.loadUnlocked(batchID)
	if err != nil {
		return nil, err
	}
	record, ok := value.Idempotency[requestID]
	if !ok {
		return nil, nil
	}
	if record.Fingerprint != fingerprint {
		return nil, &IdempotencyConflict{requestID}
	}
	return &record, nil
}

// RecordFailure 保存不会推进修订号的确定性命令结果，使客户端可安全重试失败命令。
func (r *Repository) RecordFailure(batchID string, expectedRevision int64, requestID, fingerprint string, status int, response json.RawMessage, occurredAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, err := r.loadUnlocked(batchID)
	if err != nil {
		return err
	}
	if existing, ok := current.Idempotency[requestID]; ok {
		if existing.Fingerprint != fingerprint {
			return &IdempotencyConflict{requestID}
		}
		return nil
	}
	if current.Batch.Revision != expectedRevision {
		return &RevisionConflict{Expected: expectedRevision, Actual: current.Batch.Revision}
	}
	value := cloneEnvelope(current)
	if value == nil {
		return errors.New("聚合快照深拷贝失败")
	}
	value.Idempotency[requestID] = IdempotencyRecord{RequestID: requestID, Fingerprint: fingerprint,
		Status: status, Response: append([]byte(nil), response...), CreatedAt: occurredAt.UTC()}
	path, err := r.path(batchID)
	if err != nil {
		return err
	}
	if err := atomicJSON(path, value); err != nil {
		return err
	}
	r.cache[batchID] = value
	return nil
}

func (r *Repository) Timeline(batchID string) (Timeline, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, err := r.loadUnlocked(batchID)
	if err != nil {
		return Timeline{}, err
	}
	digest := ""
	if len(value.Events) > 0 {
		digest = value.Events[len(value.Events)-1].Digest
	}
	return Timeline{Events: append([]EventFrame(nil), value.Events...), ChainDigest: digest}, nil
}

func (r *Repository) ProspectiveEventDigest(batchID string, expectedRevision int64, eventType, requestID string, payload any, occurredAt time.Time) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, err := r.loadUnlocked(batchID)
	if err != nil {
		return "", err
	}
	if value.Batch.Revision != expectedRevision {
		return "", &RevisionConflict{Expected: expectedRevision, Actual: value.Batch.Revision}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	previous := ""
	if len(value.Events) > 0 {
		previous = value.Events[len(value.Events)-1].Digest
	}
	frame := EventFrame{Sequence: int64(len(value.Events) + 1), BatchRevision: expectedRevision + 1,
		EventType: eventType, OccurredAt: occurredAt.UTC(), RequestID: requestID, Payload: encoded,
		PayloadLength: len(encoded), PreviousDigest: previous}
	return eventDigest(frame), nil
}

func (r *Repository) LoadManifest(batchID string) (*domain.ReleaseManifest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, err := safeID(batchID)
	if err != nil {
		return nil, err
	}
	var manifest domain.ReleaseManifest
	err = readJSON(filepath.Join(r.root, "batches", id, "manifest.json"), &manifest)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return &manifest, err
}
