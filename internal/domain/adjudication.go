package domain

import (
	"strings"
	"time"
)

type ResolveInput struct {
	DisagreementID  string `json:"disagreement_id"`
	Resolution      string `json:"resolution"`
	Reason          string `json:"reason"`
	AdjudicatorID   string `json:"adjudicator_id"`
	EvidenceVersion string `json:"evidence_version"`
}

func (b *CorpusBatch) Resolve(input ResolveInput, now time.Time) error {
	if b.State == StateReleased {
		return b.requireState(StateAdjudicating)
	}
	caseValue := b.Disagreements[input.DisagreementID]
	if caseValue == nil {
		return rule("not_found", "分歧 %s 不存在", input.DisagreementID)
	}
	if !caseValue.ResolvedAt.IsZero() {
		return rule("already_resolved", "分歧已经裁决")
	}
	if err := b.requireState(StateAdjudicating); err != nil {
		return err
	}
	resolution := strings.TrimSpace(input.Resolution)
	if resolution == "" || strings.TrimSpace(input.Reason) == "" || strings.TrimSpace(input.AdjudicatorID) == "" || strings.TrimSpace(input.EvidenceVersion) == "" {
		return rule("validation", "裁决结果、理由、裁决员和依据版本不能为空")
	}
	if strings.TrimSpace(input.EvidenceVersion) != b.RubricVersion {
		return rule("evidence_version_mismatch", "evidence_version 必须与当前冻结的 rubric_version %s 一致", b.RubricVersion)
	}
	var labels []string
	switch caseValue.FieldPath {
	case "transcript":
		resolution = strings.Join(strings.Fields(resolution), " ")
		if resolution == "" {
			return rule("normalization_failed", "裁决转写规范化后不能为空")
		}
	case "labels":
		rawLabels := strings.Split(resolution, ",")
		seen := map[string]bool{}
		for _, raw := range rawLabels {
			label := strings.TrimSpace(raw)
			if label == "" {
				return rule("normalization_failed", "裁决标签包含空值")
			}
			if seen[label] {
				return rule("duplicate_resolution_label", "裁决标签 %s 重复", label)
			}
			seen[label] = true
		}
		labels = NormalizeLabels(rawLabels)
		allowed := map[string]bool{}
		for _, label := range b.LabelSet {
			allowed[label] = true
		}
		for _, label := range labels {
			if !allowed[label] {
				return rule("resolution_label_not_frozen", "裁决标签 %s 不在冻结标签集中", label)
			}
		}
		resolution = strings.Join(labels, ",")
	default:
		return rule("normalization_failed", "未知裁决字段 %s", caseValue.FieldPath)
	}
	caseValue.Resolution = resolution
	caseValue.Reason = strings.TrimSpace(input.Reason)
	caseValue.AdjudicatorID = strings.TrimSpace(input.AdjudicatorID)
	caseValue.EvidenceVersion = strings.TrimSpace(input.EvidenceVersion)
	caseValue.ResolvedAt = now.UTC()
	item := b.Items[caseValue.ItemID]
	if caseValue.FieldPath == "transcript" {
		item.AdjudicatedText = resolution
	}
	if caseValue.FieldPath == "labels" {
		item.AdjudicatedLabels = labels
	}
	if b.itemResolved(item.ItemID) {
		item.ItemState = ItemAdjudicated
	}
	if b.allDisagreementsResolved() {
		b.State = StateAuditing
	}
	b.UpdatedAt = now.UTC()
	return nil
}

func (b *CorpusBatch) itemResolved(itemID string) bool {
	item := b.Items[itemID]
	if item == nil || item.AdjudicatedText == "" || len(item.AdjudicatedLabels) == 0 {
		return false
	}
	for _, value := range b.Disagreements {
		if value.ItemID == itemID && value.ResolvedAt.IsZero() {
			return false
		}
	}
	return true
}
