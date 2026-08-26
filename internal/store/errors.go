package store

import "fmt"

var ErrNotFound = fmt.Errorf("批次不存在")

type RevisionConflict struct{ Expected, Actual int64 }

func (e *RevisionConflict) Error() string {
	return fmt.Sprintf("修订冲突：期望 %d，当前 %d", e.Expected, e.Actual)
}

type IdempotencyConflict struct{ RequestID string }

func (e *IdempotencyConflict) Error() string {
	return fmt.Sprintf("request_id %s 已用于不同请求", e.RequestID)
}

type CorruptBatch struct{ BatchID, Reason string }

func (e *CorruptBatch) Error() string {
	return fmt.Sprintf("批次 %s 已隔离：%s", e.BatchID, e.Reason)
}
