package domain

import (
	"crypto/sha256"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"time"
)

func deterministicSample(ids []string, ratio float64, seed string) []string {
	type ranked struct {
		id    string
		score [32]byte
	}
	values := make([]ranked, 0, len(ids))
	for _, id := range ids {
		values = append(values, ranked{id: id, score: sha256.Sum256([]byte(seed + "\x00" + id))})
	}
	sort.Slice(values, func(i, j int) bool {
		return strings.Compare(string(values[i].score[:]), string(values[j].score[:])) < 0
	})
	count := int(math.Ceil(float64(len(ids)) * ratio))
	if count < 1 && len(ids) > 0 {
		count = 1
	}
	out := make([]string, 0, count)
	for i := 0; i < count && i < len(values); i++ {
		out = append(out, values[i].id)
	}
	sort.Strings(out)
	return out
}

func (b *CorpusBatch) BeginAudit(seed string) (*AuditRound, error) {
	if err := b.requireState(StateAuditing, StateCorrection); err != nil {
		return nil, err
	}
	if strings.TrimSpace(seed) == "" {
		return nil, rule("validation", "审计种子不能为空")
	}
	seed = strings.TrimSpace(seed)
	ids := make([]string, 0)
	if b.State == StateCorrection {
		if len(b.Audits) == 0 {
			return nil, rule("invalid_state", "缺少返修前审计")
		}
		for _, id := range b.Audits[len(b.Audits)-1].CorrectionItemIDs {
			if item := b.Items[id]; item != nil && item.ItemState == ItemReady {
				ids = append(ids, id)
			} else {
				return nil, rule("correction_pending", "返修范围尚未全部完成")
			}
		}
		if len(ids) == 0 {
			return nil, rule("correction_pending", "返修范围尚未全部完成")
		}
	} else {
		for id := range b.Items {
			ids = append(ids, id)
		}
		ids = deterministicSample(ids, b.AuditRatio, seed)
	}
	sort.Strings(ids)
	round := AuditRound{AuditID: fmt.Sprintf("audit-%d", len(b.Audits)+1), BatchID: b.BatchID,
		RoundNumber: len(b.Audits) + 1, SampleSeed: seed, SampleItemIDs: ids, Outcome: "PENDING"}
	return &round, nil
}

func (b *CorpusBatch) CompleteAudit(round AuditRound, findings []AuditFinding, auditor string, now time.Time) error {
	if err := b.requireState(StateAuditing, StateCorrection); err != nil {
		return err
	}
	expectedRound, err := b.BeginAudit(round.SampleSeed)
	if err != nil {
		return err
	}
	if round.AuditID != expectedRound.AuditID || round.RoundNumber != expectedRound.RoundNumber ||
		round.BatchID != expectedRound.BatchID || !slices.Equal(round.SampleItemIDs, expectedRound.SampleItemIDs) {
		return rule("audit_sample_mismatch", "审计样本集合、种子或轮次与确定性预览不一致")
	}
	if strings.TrimSpace(auditor) == "" {
		return rule("validation", "独立审计员不能为空")
	}
	auditor = strings.TrimSpace(auditor)
	for _, item := range b.Items {
		if auditor == item.AnnotatorA || auditor == item.AnnotatorB {
			return rule("auditor_not_independent", "审计员不能是本批次标注员")
		}
	}
	for _, disagreement := range b.Disagreements {
		if auditor == disagreement.AdjudicatorID {
			return rule("auditor_not_independent", "审计员不能是本批次裁决员")
		}
	}
	if len(findings) != len(round.SampleItemIDs) {
		return rule("validation", "必须提交全部样本结论")
	}
	expected := map[string]bool{}
	for _, id := range round.SampleItemIDs {
		expected[id] = true
	}
	failed := make([]string, 0)
	seen := map[string]bool{}
	for index := range findings {
		findings[index].Note = strings.TrimSpace(findings[index].Note)
		finding := findings[index]
		if !expected[finding.ItemID] || seen[finding.ItemID] {
			return rule("validation", "审计结论不属于当前样本或重复")
		}
		if finding.Note == "" {
			return rule("finding_note_required", "条目 %s 的审计 note 不能为空", finding.ItemID)
		}
		seen[finding.ItemID] = true
		if !finding.Passed {
			failed = append(failed, finding.ItemID)
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].ItemID < findings[j].ItemID })
	sort.Strings(failed)
	round.Findings = findings
	round.AuditorID = auditor
	round.CompletedAt = now.UTC()
	round.SampleCount = len(findings)
	round.FailedCount = len(failed)
	round.PassedCount = len(findings) - len(failed)
	if round.SampleCount > 0 {
		round.PassRate = float64(round.PassedCount) / float64(round.SampleCount)
	}
	if len(failed) > 0 {
		round.Outcome = "FAILED"
		round.CorrectionStatus = "IN_PROGRESS"
		round.CorrectionItemIDs = failed
		b.State = StateCorrection
		for _, id := range failed {
			b.Items[id].ItemState = ItemCorrection
		}
	} else {
		round.Outcome = "PASSED"
		round.CorrectionStatus = "NOT_REQUIRED"
		b.State = StateAuditing
	}
	b.Audits = append(b.Audits, round)
	b.UpdatedAt = now.UTC()
	return nil
}

type CorrectInput struct {
	ItemID      string   `json:"item_id"`
	Transcript  string   `json:"transcript"`
	Labels      []string `json:"labels"`
	Reason      string   `json:"reason"`
	CorrectorID string   `json:"corrector_id"`
}

func (b *CorpusBatch) Correct(input CorrectInput, now time.Time) error {
	if err := b.requireState(StateCorrection); err != nil {
		return err
	}
	inScope := false
	if len(b.Audits) > 0 {
		for _, id := range b.Audits[len(b.Audits)-1].CorrectionItemIDs {
			if id == input.ItemID {
				inScope = true
				break
			}
		}
	}
	item := b.Items[input.ItemID]
	if !inScope || item == nil || item.ItemState != ItemCorrection {
		return rule("out_of_scope", "条目不在本轮有界返修范围")
	}
	if strings.TrimSpace(input.Transcript) == "" || strings.TrimSpace(input.Reason) == "" || strings.TrimSpace(input.CorrectorID) == "" {
		return rule("validation", "返修文本、理由和返修人不能为空")
	}
	labels := NormalizeLabels(input.Labels)
	if len(labels) == 0 {
		return rule("validation", "返修标签不能为空")
	}
	allowed := map[string]bool{}
	for _, label := range b.LabelSet {
		allowed[label] = true
	}
	for _, label := range labels {
		if !allowed[label] {
			return rule("validation", "返修标签不在冻结标签集中")
		}
	}
	item.AdjudicatedText = strings.TrimSpace(input.Transcript)
	item.AdjudicatedLabels = labels
	item.ItemState = ItemReady
	allCorrected := true
	if len(b.Audits) > 0 {
		for _, id := range b.Audits[len(b.Audits)-1].CorrectionItemIDs {
			if current := b.Items[id]; current == nil || current.ItemState != ItemReady {
				allCorrected = false
				break
			}
		}
		if allCorrected {
			b.Audits[len(b.Audits)-1].CorrectionStatus = "COMPLETED"
		}
	}
	b.UpdatedAt = now.UTC()
	return nil
}
