package application

import (
	"encoding/json"
	"reflect"
	"sort"

	"dialectcorpusreleasegate/internal/domain"
	"dialectcorpusreleasegate/internal/store"
)

func (s *Service) Release(batchID string, command ReleaseCommand) (*MutationResult, error) {
	if err := validateMetadata(command.Metadata); err != nil {
		return nil, err
	}
	unlock := s.coordinator.acquire(batchID)
	defer unlock()
	fp := fingerprint("batch.released", command)
	if record, err := s.repo.LookupRequest(batchID, command.RequestID, fp); err == nil && record != nil {
		return replay(record)
	} else if err != nil {
		return nil, classify(err)
	}
	batch, err := s.repo.Load(batchID)
	if err != nil {
		return nil, classify(err)
	}
	if batch.Revision != command.ExpectedRevision {
		return nil, classify(&store.RevisionConflict{Expected: command.ExpectedRevision, Actual: batch.Revision})
	}
	now := s.now().UTC()
	eventPayload := struct {
		ReleasedBy string `json:"released_by"`
	}{command.ReleasedBy}
	terminalDigest, err := s.repo.ProspectiveEventDigest(batchID, command.ExpectedRevision, "batch.released", command.RequestID, eventPayload, now)
	if err != nil {
		return nil, classify(err)
	}
	manifest, err := domain.BuildManifest(batch, terminalDigest, command.ReleasedBy, now)
	if err != nil {
		return nil, s.recordFailure(batchID, batch.Revision, command.Metadata, fp, err)
	}
	if err := batch.ApplyRelease(manifest, now); err != nil {
		return nil, s.recordFailure(batchID, batch.Revision, command.Metadata, fp, err)
	}
	result := &MutationResult{BatchID: batchID, Revision: command.ExpectedRevision + 1, State: batch.State,
		EventType: "batch.released", At: now, Manifest: manifest}
	encoded, _ := json.Marshal(result)
	err = s.repo.Commit(store.Commit{Batch: batch, ExpectedRevision: command.ExpectedRevision, EventType: "batch.released",
		RequestID: command.RequestID, Payload: eventPayload,
		Fingerprint: fp, Status: 200, Response: encoded, OccurredAt: now, Manifest: manifest})
	if err != nil {
		return nil, classify(err)
	}
	return result, nil
}

func (s *Service) Verify(batchID string) (*Verification, error) {
	s.verificationMu.RLock()
	if cached := s.verificationCache[batchID]; cached != nil {
		s.verificationMu.RUnlock()
		return cached, nil
	}
	s.verificationMu.RUnlock()

	batch, err := s.repo.Load(batchID)
	if err != nil {
		return nil, classify(err)
	}
	if batch.State != domain.StateReleased || batch.Manifest == nil {
		return nil, &AppError{Code: "invalid_state", Message: "仅已发布批次可执行清单验证"}
	}
	manifest, err := s.repo.LoadManifest(batchID)
	if err != nil {
		if batch.Manifest == nil {
			return nil, classify(err)
		}
		manifest = batch.Manifest
	}
	storedContentDigest := domain.ComputeManifestDigest(manifest)
	rubricDigest := domain.RubricDigest(batch)
	timeline, timelineErr := s.repo.Timeline(batchID)
	if timelineErr != nil {
		return nil, classify(timelineErr)
	}
	expectedItems := domain.ManifestItems(batch)
	expectedManifest := *batch.Manifest
	expectedManifest.ItemEntries = expectedItems
	expectedManifest.RubricDigest = rubricDigest
	expectedManifest.EventChainDigest = timeline.ChainDigest
	computed := domain.ComputeManifestDigest(&expectedManifest)
	mismatchedItems := compareManifestItems(manifest.ItemEntries, expectedItems)
	result := &Verification{BatchID: batchID, StoredDigest: manifest.ManifestDigest, ComputedDigest: computed,
		ManifestDigestMatch:   manifest.ManifestDigest == storedContentDigest && manifest.ManifestDigest == computed && len(mismatchedItems) == 0,
		RubricDigestMatch:     manifest.RubricDigest == rubricDigest,
		EventChainDigestMatch: manifest.EventChainDigest != "" && manifest.EventChainDigest == timeline.ChainDigest,
		MismatchComponents:    []string{}, MismatchItemIDs: mismatchedItems}
	if !result.ManifestDigestMatch {
		result.MismatchComponents = append(result.MismatchComponents, "manifest")
	}
	if !result.RubricDigestMatch {
		result.MismatchComponents = append(result.MismatchComponents, "rubric")
	}
	if !result.EventChainDigestMatch {
		result.MismatchComponents = append(result.MismatchComponents, "event_chain")
	}
	result.Valid = result.ManifestDigestMatch && result.RubricDigestMatch && result.EventChainDigestMatch && batch.State == domain.StateReleased
	s.verificationMu.Lock()
	s.verificationCache[batchID] = result
	s.verificationMu.Unlock()
	return result, nil
}

func compareManifestItems(stored, expected []domain.ManifestItem) []string {
	storedByID := make(map[string]domain.ManifestItem, len(stored))
	expectedByID := make(map[string]domain.ManifestItem, len(expected))
	for _, item := range stored {
		storedByID[item.ItemID] = item
	}
	for _, item := range expected {
		expectedByID[item.ItemID] = item
	}
	ids := map[string]bool{}
	for id, current := range expectedByID {
		if value, ok := storedByID[id]; !ok || !reflect.DeepEqual(value, current) {
			ids[id] = true
		}
	}
	for id := range storedByID {
		if _, ok := expectedByID[id]; !ok {
			ids[id] = true
		}
	}
	out := make([]string, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
