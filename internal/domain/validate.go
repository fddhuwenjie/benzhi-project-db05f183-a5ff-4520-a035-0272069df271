package domain

import (
	"fmt"
	"strings"
)

// ValidateSnapshot 校验从持久化层重读的聚合内部引用和终态约束。
func ValidateSnapshot(batch *CorpusBatch) error {
	if batch == nil {
		return fmt.Errorf("批次快照为空")
	}
	if strings.TrimSpace(batch.BatchID) == "" || strings.TrimSpace(batch.Title) == "" {
		return fmt.Errorf("批次身份或标题为空")
	}
	if batch.Revision < 0 {
		return fmt.Errorf("批次修订不能为负数")
	}
	if batch.CreatedAt.IsZero() || batch.UpdatedAt.IsZero() || batch.UpdatedAt.Before(batch.CreatedAt) {
		return fmt.Errorf("批次时间戳无效")
	}
	if batch.Items == nil || batch.Submissions == nil || batch.Disagreements == nil {
		return fmt.Errorf("聚合集合未初始化")
	}
	validState := map[BatchState]bool{StateDraft: true, StateAnnotating: true, StateAdjudicating: true,
		StateAuditing: true, StateCorrection: true, StateReleased: true}
	if !validState[batch.State] {
		return fmt.Errorf("未知批次状态 %q", batch.State)
	}
	if batch.State != StateDraft {
		if batch.RubricVersion == "" || len(batch.LabelSet) == 0 || batch.TranscriptionRules == "" {
			return fmt.Errorf("非草稿批次缺少冻结规范")
		}
		if batch.MinimumAgreement < 0 || batch.MinimumAgreement > 1 || batch.AuditRatio <= 0 || batch.AuditRatio > 1 {
			return fmt.Errorf("冻结质量阈值越界")
		}
	}
	allowedLabels := map[string]bool{}
	for index, label := range batch.LabelSet {
		if label == "" || allowedLabels[label] {
			return fmt.Errorf("冻结标签集包含空值或重复值")
		}
		if index > 0 && batch.LabelSet[index-1] > label {
			return fmt.Errorf("冻结标签集未规范排序")
		}
		allowedLabels[label] = true
	}
	sources := map[string]string{}
	digests := map[string]string{}
	for key, item := range batch.Items {
		if item == nil || key != item.ItemID || item.BatchID != batch.BatchID {
			return fmt.Errorf("条目 %q 的身份引用不一致", key)
		}
		if item.AnnotatorA == "" || item.AnnotatorB == "" || item.AnnotatorA == item.AnnotatorB {
			return fmt.Errorf("条目 %s 的双席位无效", key)
		}
		if item.DurationMS <= 0 || item.DurationMS > maximumDurationMS || item.ContentDigest == "" || item.SourceRef == "" {
			return fmt.Errorf("条目 %s 的来源事实无效", key)
		}
		if err := validateItemIDInRange(item.ItemID, batch.ItemRange); err != nil {
			return fmt.Errorf("条目 %s 不在冻结范围：%w", key, err)
		}
		if !speakerPattern.MatchString(item.SpeakerCode) {
			return fmt.Errorf("条目 %s 的 speaker_code 无效", key)
		}
		if other := sources[item.SourceRef]; other != "" {
			return fmt.Errorf("条目 %s 与 %s 的 source_ref 重复", key, other)
		}
		if other := digests[item.ContentDigest]; other != "" {
			return fmt.Errorf("条目 %s 与 %s 的 content_digest 重复", key, other)
		}
		sources[item.SourceRef], digests[item.ContentDigest] = key, key
		if item.PreflightStatus != "PASSED" || !item.CanAnnotate || item.RegisteredAt.IsZero() {
			return fmt.Errorf("条目 %s 未通过登记预检", key)
		}
		if item.AgreementStats.Compared && item.AgreementStats.Overall != item.Agreement {
			return fmt.Errorf("条目 %s 的一致率统计不匹配", key)
		}
		if err := validateLabels(item.AdjudicatedLabels, allowedLabels); err != nil {
			return fmt.Errorf("条目 %s 的裁决标签无效：%w", key, err)
		}
	}
	for key, submission := range batch.Submissions {
		item := batch.Items[submission.ItemID]
		if item == nil || submission.BatchID != batch.BatchID || key != submissionKey(submission.ItemID, submission.Seat) {
			return fmt.Errorf("提交 %q 的聚合引用不一致", key)
		}
		expected := item.AnnotatorA
		if submission.Seat == "B" {
			expected = item.AnnotatorB
		} else if submission.Seat != "A" {
			return fmt.Errorf("提交 %q 包含未知席位", key)
		}
		if submission.AnnotatorID != expected || submission.RubricVersion != batch.RubricVersion {
			return fmt.Errorf("提交 %q 的席位人员或规范版本不一致", key)
		}
		if submission.SubmissionDigest == "" || submission.Transcript == "" {
			return fmt.Errorf("提交 %q 缺少内容或摘要", key)
		}
		if err := validateLabels(submission.Labels, allowedLabels); err != nil {
			return fmt.Errorf("提交 %q 标签无效：%w", key, err)
		}
	}
	for key, disagreement := range batch.Disagreements {
		if disagreement == nil || key != disagreement.DisagreementID || disagreement.BatchID != batch.BatchID || batch.Items[disagreement.ItemID] == nil {
			return fmt.Errorf("分歧 %q 的聚合引用不一致", key)
		}
		if disagreement.FieldPath != "transcript" && disagreement.FieldPath != "labels" {
			return fmt.Errorf("分歧 %q 的字段路径未知", key)
		}
		if !disagreement.ResolvedAt.IsZero() && (disagreement.Resolution == "" || disagreement.Reason == "" || disagreement.AdjudicatorID == "" || disagreement.EvidenceVersion == "") {
			return fmt.Errorf("分歧 %q 的裁决证据不完整", key)
		}
		if !disagreement.ResolvedAt.IsZero() && disagreement.EvidenceVersion != batch.RubricVersion {
			return fmt.Errorf("分歧 %q 的证据版本与冻结规范不一致", key)
		}
	}
	for index, audit := range batch.Audits {
		if audit.RoundNumber != index+1 || audit.BatchID != batch.BatchID || audit.AuditID == "" {
			return fmt.Errorf("第 %d 轮审计身份或轮次不连续", index+1)
		}
		if audit.Outcome != "PASSED" && audit.Outcome != "FAILED" {
			return fmt.Errorf("第 %d 轮审计结论无效", index+1)
		}
		if len(audit.SampleItemIDs) != len(audit.Findings) || audit.AuditorID == "" || audit.CompletedAt.IsZero() {
			return fmt.Errorf("第 %d 轮审计证据不完整", index+1)
		}
		if audit.SampleCount != len(audit.Findings) || audit.PassedCount+audit.FailedCount != audit.SampleCount {
			return fmt.Errorf("第 %d 轮审计统计不一致", index+1)
		}
		if audit.SampleCount > 0 && audit.PassRate != float64(audit.PassedCount)/float64(audit.SampleCount) {
			return fmt.Errorf("第 %d 轮审计通过率不一致", index+1)
		}
		for _, id := range audit.SampleItemIDs {
			if batch.Items[id] == nil {
				return fmt.Errorf("第 %d 轮审计引用未知条目 %s", index+1, id)
			}
		}
	}
	if batch.State == StateReleased {
		if batch.Manifest == nil || batch.Manifest.BatchID != batch.BatchID || batch.Manifest.BatchRevision != batch.Revision {
			return fmt.Errorf("发布终态缺少匹配清单")
		}
		if batch.Manifest.ManifestDigest != ComputeManifestDigest(batch.Manifest) {
			return fmt.Errorf("发布清单摘要不匹配")
		}
	} else if batch.Manifest != nil {
		return fmt.Errorf("非发布状态不应包含发布清单")
	}
	return nil
}

func validateLabels(labels []string, allowed map[string]bool) error {
	previous := ""
	for _, label := range labels {
		if !allowed[label] {
			return fmt.Errorf("标签 %q 不在冻结集合中", label)
		}
		if previous != "" && previous >= label {
			return fmt.Errorf("标签未规范排序或存在重复")
		}
		previous = label
	}
	return nil
}
