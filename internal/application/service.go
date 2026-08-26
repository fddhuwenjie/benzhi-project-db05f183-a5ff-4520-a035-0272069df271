package application

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"dialectcorpusreleasegate/internal/domain"
	"dialectcorpusreleasegate/internal/store"
)

type Service struct {
	repo           *store.Repository
	coordinator    *Coordinator
	now            func() time.Time
	lastObservedAt time.Time
}

func NewService(repo *store.Repository, maxParallel int) *Service {
	return &Service{repo: repo, coordinator: NewCoordinator(maxParallel), now: time.Now}
}

func (s *Service) WithClock(clock func() time.Time) {
	if clock != nil {
		s.now = clock
	}
}

// timestamp 避免时钟回拨使同一服务进程产生倒序的业务时间。
func (s *Service) timestamp() time.Time {
	observed := s.now().UTC()
	if observed.Before(s.lastObservedAt) {
		return s.lastObservedAt
	}
	s.lastObservedAt = observed
	return observed
}

func validateMetadata(meta Metadata) error {
	if strings.TrimSpace(meta.RequestID) == "" {
		return &AppError{Code: "validation", Message: "request_id 不能为空"}
	}
	if len(meta.RequestID) > 128 {
		return &AppError{Code: "validation", Message: "request_id 过长"}
	}
	return nil
}

func fingerprint(eventType string, value any) string {
	return domain.Digest(struct {
		Type  string `json:"type"`
		Value any    `json:"value"`
	}{eventType, value})
}

func replay(record *store.IdempotencyRecord) (*MutationResult, error) {
	if record.Status >= 400 {
		var appError AppError
		if err := json.Unmarshal(record.Response, &appError); err != nil {
			return nil, classify(err)
		}
		return nil, &appError
	}
	var result MutationResult
	if err := json.Unmarshal(record.Response, &result); err != nil {
		return nil, classify(err)
	}
	result.Replayed = true
	return &result, nil
}

func appErrorStatus(err *AppError) int {
	switch err.Code {
	case "not_found":
		return 404
	case "annotator_forbidden", "forbidden":
		return 403
	case "duplicate_item_id", "duplicate_source_ref", "duplicate_content_digest", "annotation_immutable", "already_resolved":
		return 409
	default:
		return 400
	}
}

func (s *Service) recordFailure(batchID string, revision int64, meta Metadata, fp string, err error) *AppError {
	appError := classify(err)
	if appError.Code == "terminal" {
		current := revision
		appError.CurrentRevision = &current
	}
	encoded, marshalErr := json.Marshal(appError)
	if marshalErr != nil {
		return classify(marshalErr)
	}
	if recordErr := s.repo.RecordFailure(batchID, revision, meta.RequestID, fp, appErrorStatus(appError), encoded, s.timestamp()); recordErr != nil {
		return classify(recordErr)
	}
	return appError
}

func (s *Service) mutate(batchID string, meta Metadata, eventType string, command any, change func(*domain.CorpusBatch) error) (*MutationResult, error) {
	if err := validateMetadata(meta); err != nil {
		return nil, err
	}
	release := s.coordinator.acquire(batchID)
	defer release()
	fp := fingerprint(eventType, command)
	if record, err := s.repo.LookupRequest(batchID, meta.RequestID, fp); err == nil && record != nil {
		return replay(record)
	} else if err != nil && err != store.ErrNotFound {
		return nil, classify(err)
	}
	batch, err := s.repo.Load(batchID)
	if err != nil {
		return nil, classify(err)
	}
	if batch.Revision != meta.ExpectedRevision {
		return nil, classify(&store.RevisionConflict{Expected: meta.ExpectedRevision, Actual: batch.Revision})
	}
	if err := change(batch); err != nil {
		return nil, s.recordFailure(batchID, batch.Revision, meta, fp, err)
	}
	now := s.timestamp()
	result := &MutationResult{BatchID: batchID, Revision: meta.ExpectedRevision + 1, State: batch.State, EventType: eventType, At: now}
	if len(batch.Audits) > 0 && (eventType == "audit.completed") {
		audit := batch.Audits[len(batch.Audits)-1]
		result.Audit = &audit
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, classify(err)
	}
	err = s.repo.Commit(store.Commit{Batch: batch, ExpectedRevision: meta.ExpectedRevision, EventType: eventType,
		RequestID: meta.RequestID, Payload: command, Fingerprint: fp, Status: 200, Response: encoded, OccurredAt: now})
	if err != nil {
		return nil, classify(err)
	}
	return result, nil
}

func (s *Service) Create(command CreateBatchCommand) (*MutationResult, error) {
	if err := validateMetadata(command.Metadata); err != nil {
		return nil, err
	}
	if command.ExpectedRevision != -1 {
		return nil, &AppError{Code: "validation", Message: "创建批次的 expected_revision 必须为 -1"}
	}
	release := s.coordinator.acquire(command.BatchID)
	defer release()
	fp := fingerprint("batch.created", command)
	if record, err := s.repo.LookupRequest(command.BatchID, command.RequestID, fp); err == nil && record != nil {
		return replay(record)
	} else if err != nil && err != store.ErrNotFound {
		return nil, classify(err)
	}
	now := s.timestamp()
	batch, err := domain.NewBatch(domain.CreateBatchInput{BatchID: command.BatchID, Title: command.Title,
		DialectSite: command.DialectSite, SourceNote: command.SourceNote, ItemRange: command.ItemRange}, now)
	if err != nil {
		return nil, classify(err)
	}
	result := &MutationResult{BatchID: batch.BatchID, Revision: 0, State: batch.State, EventType: "batch.created", At: now}
	encoded, _ := json.Marshal(result)
	err = s.repo.Commit(store.Commit{Batch: batch, ExpectedRevision: -1, EventType: "batch.created", RequestID: command.RequestID,
		Payload: command, Fingerprint: fp, Status: 201, Response: encoded, OccurredAt: now})
	if err != nil {
		return nil, classify(err)
	}
	return result, nil
}

func (s *Service) Freeze(batchID string, command FreezeCommand) (*MutationResult, error) {
	return s.mutate(batchID, command.Metadata, "rubric.frozen", command, func(batch *domain.CorpusBatch) error { return batch.Freeze(command.FreezeInput, s.timestamp()) })
}

func (s *Service) RegisterItem(batchID string, command RegisterItemCommand) (*MutationResult, error) {
	return s.mutate(batchID, command.Metadata, "item.registered", command, func(batch *domain.CorpusBatch) error {
		return batch.RegisterItem(command.RegisterItemInput, s.timestamp())
	})
}

func (s *Service) SubmitAnnotation(batchID string, command SubmitAnnotationCommand) (*MutationResult, error) {
	return s.mutate(batchID, command.Metadata, "annotation.submitted", command, func(batch *domain.CorpusBatch) error {
		return batch.SubmitAnnotation(command.SubmitAnnotationInput, s.timestamp())
	})
}

func (s *Service) Resolve(batchID string, command ResolveCommand) (*MutationResult, error) {
	return s.mutate(batchID, command.Metadata, "disagreement.resolved", command, func(batch *domain.CorpusBatch) error { return batch.Resolve(command.ResolveInput, s.timestamp()) })
}

func (s *Service) CompleteAudit(batchID string, command AuditCommand) (*MutationResult, error) {
	return s.mutate(batchID, command.Metadata, "audit.completed", command, func(batch *domain.CorpusBatch) error {
		round, err := batch.BeginAudit(command.SampleSeed)
		if err != nil {
			return err
		}
		return batch.CompleteAudit(*round, command.Findings, command.AuditorID, s.timestamp())
	})
}

func (s *Service) Correct(batchID string, command CorrectCommand) (*MutationResult, error) {
	return s.mutate(batchID, command.Metadata, "item.corrected", command, func(batch *domain.CorpusBatch) error { return batch.Correct(command.CorrectInput, s.timestamp()) })
}

func (s *Service) String() string { return fmt.Sprintf("Service(%p)", s.repo) }
