package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"dialectcorpusreleasegate/internal/domain"
)

func eventDigest(frame EventFrame) string {
	payload := canonicalPayload(frame.Payload)
	value := struct {
		Sequence      int64           `json:"sequence"`
		Revision      int64           `json:"batch_revision"`
		Type          string          `json:"event_type"`
		At            string          `json:"occurred_at"`
		Request       string          `json:"request_id"`
		Payload       json.RawMessage `json:"payload"`
		PayloadLength int             `json:"payload_length"`
		Previous      string          `json:"previous_digest"`
	}{frame.Sequence, frame.BatchRevision, frame.EventType, frame.OccurredAt.UTC().Format("2006-01-02T15:04:05.000000000Z"), frame.RequestID, payload, len(payload), frame.PreviousDigest}
	return domain.Digest(value)
}

func canonicalPayload(raw json.RawMessage) json.RawMessage {
	var buffer bytes.Buffer
	if err := json.Compact(&buffer, raw); err != nil {
		return raw
	}
	return buffer.Bytes()
}

func validateEnvelope(value *envelope) error {
	if value.Batch == nil {
		return fmt.Errorf("聚合快照为空")
	}
	previous := ""
	for index, frame := range value.Events {
		expectedSequence := int64(index + 1)
		if frame.Sequence != expectedSequence {
			return fmt.Errorf("事件序号在 %d 处不连续", expectedSequence)
		}
		if frame.BatchRevision != expectedSequence-1 {
			return fmt.Errorf("修订在 %d 处不连续", expectedSequence)
		}
		if frame.PreviousDigest != previous {
			return fmt.Errorf("事件前序摘要在 %d 处不匹配", expectedSequence)
		}
		if frame.PayloadLength != len(canonicalPayload(frame.Payload)) {
			return fmt.Errorf("事件载荷长度在 %d 处不匹配", expectedSequence)
		}
		if frame.Digest != eventDigest(frame) {
			return fmt.Errorf("事件摘要在 %d 处不匹配", expectedSequence)
		}
		previous = frame.Digest
	}
	if value.Batch.Revision != int64(len(value.Events))-1 {
		return fmt.Errorf("快照修订与事件数量不一致")
	}
	if err := domain.ValidateSnapshot(value.Batch); err != nil {
		return fmt.Errorf("聚合快照不变量校验失败：%w", err)
	}
	return nil
}

func (r *Repository) scan() error {
	entries, err := os.ReadDir(filepath.Join(r.root, "batches"))
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		path, pathErr := r.path(name)
		if pathErr != nil {
			r.quarantined[name] = pathErr.Error()
			continue
		}
		var value envelope
		if readErr := readJSON(path, &value); readErr != nil {
			r.quarantined[name] = readErr.Error()
			continue
		}
		if checkErr := validateEnvelope(&value); checkErr != nil {
			r.quarantined[name] = checkErr.Error()
		}
	}
	return nil
}

func (r *Repository) Quarantined() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]string, len(r.quarantined))
	for key, value := range r.quarantined {
		out[key] = value
	}
	return out
}
