package domain

import (
	"fmt"
	"strings"
	"time"
)

type SubmitAnnotationInput struct {
	SubmissionID string   `json:"submission_id"`
	ItemID       string   `json:"item_id"`
	Seat         string   `json:"seat"`
	AnnotatorID  string   `json:"annotator_id"`
	Transcript   string   `json:"transcript"`
	Labels       []string `json:"labels"`
}

func (b *CorpusBatch) SubmitAnnotation(input SubmitAnnotationInput, now time.Time) error {
	if b.State == StateReleased {
		return b.requireState(StateAnnotating)
	}
	item := b.Items[input.ItemID]
	if item == nil {
		return rule("not_found", "条目 %s 不存在", input.ItemID)
	}
	seat := strings.ToUpper(strings.TrimSpace(input.Seat))
	if seat != "A" && seat != "B" {
		return rule("validation", "seat 必须是 A 或 B")
	}
	key := submissionKey(item.ItemID, seat)
	if _, exists := b.Submissions[key]; exists {
		return rule("annotation_immutable", "该席位已经提交，独立结果不可覆盖")
	}
	if err := b.requireState(StateAnnotating); err != nil {
		return err
	}
	expected := item.AnnotatorA
	if seat == "B" {
		expected = item.AnnotatorB
	}
	if strings.TrimSpace(input.AnnotatorID) != expected {
		return rule("annotator_forbidden", "annotator_id 与该条目既有席位分配不符")
	}
	if strings.TrimSpace(input.Transcript) == "" {
		return rule("missing_transcript", "转写不能为空")
	}
	labels := NormalizeLabels(input.Labels)
	if len(labels) == 0 {
		return rule("missing_labels", "至少提交一个标签")
	}
	allowed := make(map[string]bool, len(b.LabelSet))
	for _, value := range b.LabelSet {
		allowed[value] = true
	}
	for _, value := range labels {
		if !allowed[value] {
			return rule("label_not_frozen", "标签 %s 不在冻结标签集中", value)
		}
	}
	submission := AnnotationSubmission{SubmissionID: strings.TrimSpace(input.SubmissionID), BatchID: b.BatchID,
		ItemID: item.ItemID, Seat: seat, AnnotatorID: expected, Transcript: strings.TrimSpace(input.Transcript),
		Labels: labels, RubricVersion: b.RubricVersion, SubmittedAt: now.UTC()}
	if submission.SubmissionID == "" {
		submission.SubmissionID = fmt.Sprintf("sub-%s-%s", item.ItemID, strings.ToLower(seat))
	}
	submission.SubmissionDigest = Digest(struct {
		Item, Seat, Text string
		Labels           []string
	}{item.ItemID, seat, submission.Transcript, labels})
	b.Submissions[key] = submission
	other := "A"
	if seat == "A" {
		other = "B"
	}
	peer, ready := b.Submissions[submissionKey(item.ItemID, other)]
	if !ready {
		item.ItemState = ItemSubmitted
		b.UpdatedAt = now.UTC()
		return nil
	}
	a := submission
	bb := peer
	if a.Seat == "B" {
		a, bb = bb, a
	}
	b.compare(item, a, bb)
	if b.allItemsReadyForAdjudication() {
		if b.allDisagreementsResolved() {
			b.State = StateAuditing
		} else {
			b.State = StateAdjudicating
		}
	}
	b.UpdatedAt = now.UTC()
	return nil
}

func (b *CorpusBatch) compare(item *UtteranceItem, a, bb AnnotationSubmission) {
	matched := 0
	transcriptMatched := a.Transcript == bb.Transcript
	labelsMatched := strings.Join(a.Labels, "\x1f") == strings.Join(bb.Labels, "\x1f")
	if transcriptMatched {
		matched++
		item.AdjudicatedText = a.Transcript
	} else {
		b.addDisagreement(item.ItemID, "transcript", a.Transcript, bb.Transcript)
	}
	if labelsMatched {
		matched++
		item.AdjudicatedLabels = cloneLabels(a.Labels)
	} else {
		b.addDisagreement(item.ItemID, "labels", strings.Join(a.Labels, ","), strings.Join(bb.Labels, ","))
	}
	item.Agreement = float64(matched) / 2
	item.AgreementStats = AgreementStatistics{Compared: true, TranscriptAgreement: transcriptMatched,
		LabelsAgreement: labelsMatched, Overall: item.Agreement}
	if matched == 2 {
		item.ItemState = ItemAdjudicated
	} else {
		item.ItemState = ItemDisputed
	}
}

func (b *CorpusBatch) addDisagreement(itemID, field, a, bb string) {
	id := fmt.Sprintf("d-%s-%s", itemID, field)
	b.Disagreements[id] = &DisagreementCase{DisagreementID: id, BatchID: b.BatchID, ItemID: itemID,
		FieldPath: field, ValueA: a, ValueB: bb}
}
