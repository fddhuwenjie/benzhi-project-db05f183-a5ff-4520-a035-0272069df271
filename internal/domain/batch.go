package domain

import (
	"strings"
	"time"
)

type CreateBatchInput struct {
	BatchID     string `json:"batch_id"`
	Title       string `json:"title"`
	DialectSite string `json:"dialect_site"`
	SourceNote  string `json:"source_note"`
	ItemRange   string `json:"item_range"`
}

type FreezeInput struct {
	RubricVersion      string   `json:"rubric_version"`
	LabelSet           []string `json:"label_set"`
	TranscriptionRules string   `json:"transcription_rules"`
	MinimumAgreement   float64  `json:"minimum_agreement"`
	AuditRatio         float64  `json:"audit_ratio"`
}

func NewBatch(input CreateBatchInput, now time.Time) (*CorpusBatch, error) {
	if strings.TrimSpace(input.BatchID) == "" {
		return nil, rule("validation", "batch_id 不能为空")
	}
	if strings.TrimSpace(input.Title) == "" {
		return nil, rule("validation", "批次标题不能为空")
	}
	if strings.TrimSpace(input.DialectSite) == "" {
		return nil, rule("validation", "语言点不能为空")
	}
	if strings.TrimSpace(input.SourceNote) == "" {
		return nil, rule("validation", "来源说明不能为空")
	}
	if strings.TrimSpace(input.ItemRange) == "" {
		return nil, rule("validation", "条目范围不能为空")
	}
	return &CorpusBatch{
		BatchID: strings.TrimSpace(input.BatchID), Title: strings.TrimSpace(input.Title),
		DialectSite: strings.TrimSpace(input.DialectSite), SourceNote: strings.TrimSpace(input.SourceNote),
		ItemRange: strings.TrimSpace(input.ItemRange), State: StateDraft, Revision: 0,
		CreatedAt: now.UTC(), UpdatedAt: now.UTC(), Items: map[string]*UtteranceItem{},
		Submissions: map[string]AnnotationSubmission{}, Disagreements: map[string]*DisagreementCase{},
	}, nil
}

func (b *CorpusBatch) Freeze(input FreezeInput, now time.Time) error {
	if err := b.requireState(StateDraft); err != nil {
		return err
	}
	labels := NormalizeLabels(input.LabelSet)
	if strings.TrimSpace(input.RubricVersion) == "" || len(labels) == 0 || strings.TrimSpace(input.TranscriptionRules) == "" {
		return rule("validation", "规范版本、标签集和转写规则均不能为空")
	}
	if input.MinimumAgreement < 0 || input.MinimumAgreement > 1 {
		return rule("validation", "最低一致率必须在 0 到 1 之间")
	}
	if input.AuditRatio <= 0 || input.AuditRatio > 1 {
		return rule("validation", "抽审比例必须大于 0 且不超过 1")
	}
	b.RubricVersion = strings.TrimSpace(input.RubricVersion)
	b.LabelSet = labels
	b.TranscriptionRules = strings.TrimSpace(input.TranscriptionRules)
	b.MinimumAgreement = input.MinimumAgreement
	b.AuditRatio = input.AuditRatio
	b.State = StateAnnotating
	b.UpdatedAt = now.UTC()
	return nil
}

func (b *CorpusBatch) requireState(states ...BatchState) error {
	if b.State == StateReleased {
		return rule("terminal", "已发布批次不可修改")
	}
	for _, state := range states {
		if b.State == state {
			return nil
		}
	}
	return rule("invalid_state", "状态 %s 不允许执行该操作", b.State)
}

func (b *CorpusBatch) allItemsReadyForAdjudication() bool {
	if len(b.Items) == 0 {
		return false
	}
	for _, item := range b.Items {
		if item.ItemState == ItemRegistered || item.ItemState == ItemSubmitted {
			return false
		}
	}
	return true
}

func (b *CorpusBatch) allDisagreementsResolved() bool {
	for _, disagreement := range b.Disagreements {
		if disagreement.ResolvedAt.IsZero() {
			return false
		}
	}
	return true
}
