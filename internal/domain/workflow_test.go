package domain

import (
	"testing"
	"time"
)

func TestWorkflowWithBoundedCorrection(t *testing.T) {
	now := time.Date(2026, 8, 27, 1, 2, 3, 0, time.UTC)
	batch, err := NewBatch(CreateBatchInput{"b1", "测试批次", "语言点", "来源", "u1..u2"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := batch.Freeze(FreezeInput{"v1", []string{"疑问", "陈述"}, "规则", .8, 1}, now); err != nil {
		t.Fatal(err)
	}
	items := []RegisterItemInput{
		{ItemID: "u1", SourceRef: "a", ContentDigest: "d1", DurationMS: 1, SpeakerCode: "SPK-1", AnnotatorA: "a", AnnotatorB: "b"},
		{ItemID: "u2", SourceRef: "b", ContentDigest: "d2", DurationMS: 2, SpeakerCode: "SPK-2", AnnotatorA: "c", AnnotatorB: "d"},
	}
	for _, item := range items {
		if err := batch.RegisterItem(item, now); err != nil {
			t.Fatal(err)
		}
	}
	submissions := []SubmitAnnotationInput{
		{ItemID: "u1", Seat: "A", AnnotatorID: "a", Transcript: "甲", Labels: []string{"陈述"}},
		{ItemID: "u1", Seat: "B", AnnotatorID: "b", Transcript: "乙", Labels: []string{"陈述"}},
		{ItemID: "u2", Seat: "A", AnnotatorID: "c", Transcript: "丙", Labels: []string{"疑问"}},
		{ItemID: "u2", Seat: "B", AnnotatorID: "d", Transcript: "丙", Labels: []string{"疑问"}},
	}
	for _, submission := range submissions {
		if err := batch.SubmitAnnotation(submission, now); err != nil {
			t.Fatal(err)
		}
	}
	if batch.State != StateAdjudicating {
		t.Fatalf("状态=%s", batch.State)
	}
	if err := batch.Resolve(ResolveInput{"d-u1-transcript", "甲", "证据", "judge", "v1"}, now); err != nil {
		t.Fatal(err)
	}
	if batch.State != StateAuditing {
		t.Fatalf("裁决后状态=%s", batch.State)
	}
	round, err := batch.BeginAudit("seed")
	if err != nil {
		t.Fatal(err)
	}
	findings := []AuditFinding{{ItemID: round.SampleItemIDs[0], Passed: false, Note: "失败"}, {ItemID: round.SampleItemIDs[1], Passed: true, Note: "通过"}}
	if err := batch.CompleteAudit(*round, findings, "auditor", now); err != nil {
		t.Fatal(err)
	}
	failed := round.SampleItemIDs[0]
	other := round.SampleItemIDs[1]
	if err := batch.Correct(CorrectInput{ItemID: other, Transcript: "越界", Labels: []string{"陈述"}, Reason: "x", CorrectorID: "c"}, now); ErrorCode(err) != "out_of_scope" {
		t.Fatalf("越界返修未拒绝：%v", err)
	}
	item := batch.Items[failed]
	if err := batch.Correct(CorrectInput{ItemID: failed, Transcript: item.AdjudicatedText, Labels: item.AdjudicatedLabels, Reason: "修复", CorrectorID: "c"}, now); err != nil {
		t.Fatal(err)
	}
	focused, err := batch.BeginAudit("new-seed")
	if err != nil {
		t.Fatal(err)
	}
	if len(focused.SampleItemIDs) != 1 || focused.SampleItemIDs[0] != failed {
		t.Fatalf("复审范围=%v", focused.SampleItemIDs)
	}
}

func TestRejectsDoubleSeatAndTerminalMutation(t *testing.T) {
	now := time.Now()
	batch, _ := NewBatch(CreateBatchInput{"b", "t", "s", "n", "r"}, now)
	_ = batch.Freeze(FreezeInput{"v", []string{"L"}, "r", 1, 1}, now)
	err := batch.RegisterItem(RegisterItemInput{ItemID: "i", SourceRef: "s", ContentDigest: "d", DurationMS: 1, AnnotatorA: "same", AnnotatorB: "same"}, now)
	if ErrorCode(err) != "seat_conflict" {
		t.Fatalf("双席位未拒绝：%v", err)
	}
	batch.State = StateReleased
	err = batch.RegisterItem(RegisterItemInput{}, now)
	if ErrorCode(err) != "terminal" {
		t.Fatalf("终态修改未拒绝：%v", err)
	}
}

func TestDeterministicSample(t *testing.T) {
	ids := []string{"d", "a", "c", "b"}
	one := deterministicSample(ids, .5, "fixed")
	two := deterministicSample([]string{"b", "c", "a", "d"}, .5, "fixed")
	if Digest(one) != Digest(two) {
		t.Fatalf("相同集合与种子得到不同样本：%v / %v", one, two)
	}
}

func TestRegistrationPreflightAndFieldAgreement(t *testing.T) {
	now := time.Now().UTC()
	batch, _ := NewBatch(CreateBatchInput{"preflight", "预检", "语言点", "来源", "001-002"}, now)
	if err := batch.Freeze(FreezeInput{"v1", []string{"陈述", "疑问"}, "规则", .8, 1}, now); err != nil {
		t.Fatal(err)
	}
	first := RegisterItemInput{ItemID: "001", SourceRef: "a.wav", ContentDigest: "sha256:a", DurationMS: 1000,
		SpeakerCode: "SPK-1", AnnotatorA: "ann-a", AnnotatorB: "ann-b"}
	if err := batch.RegisterItem(first, now); err != nil {
		t.Fatal(err)
	}
	duplicate := RegisterItemInput{ItemID: "002", SourceRef: "b.wav", ContentDigest: "sha256:a", DurationMS: 1000,
		SpeakerCode: "SPK-2", AnnotatorA: "ann-c", AnnotatorB: "ann-d"}
	if err := batch.RegisterItem(duplicate, now); ErrorCode(err) != "duplicate_content_digest" {
		t.Fatalf("重复摘要错误=%v", err)
	}
	if len(batch.Items) != 1 {
		t.Fatalf("失败预检改变了条目数：%d", len(batch.Items))
	}
	duplicate.ContentDigest = "sha256:b"
	if err := batch.RegisterItem(duplicate, now); err != nil {
		t.Fatal(err)
	}
	inputs := []SubmitAnnotationInput{
		{ItemID: "001", Seat: "A", AnnotatorID: "ann-a", Transcript: "同文", Labels: []string{"陈述"}},
		{ItemID: "001", Seat: "B", AnnotatorID: "ann-b", Transcript: "同文", Labels: []string{"疑问"}},
	}
	for _, input := range inputs {
		if err := batch.SubmitAnnotation(input, now); err != nil {
			t.Fatal(err)
		}
	}
	item := batch.Items["001"]
	if !item.AgreementStats.TranscriptAgreement || item.AgreementStats.LabelsAgreement || item.Agreement != .5 {
		t.Fatalf("字段一致率异常：%+v", item.AgreementStats)
	}
	if len(batch.Disagreements) != 1 || batch.Disagreements["d-001-labels"] == nil {
		t.Fatalf("标签分歧异常：%v", batch.Disagreements)
	}
}

func TestAdjudicationNormalizationAndEvidenceVersion(t *testing.T) {
	now := time.Now().UTC()
	batch, _ := NewBatch(CreateBatchInput{"resolve", "裁决", "语言点", "来源", "001-001"}, now)
	_ = batch.Freeze(FreezeInput{"rubric-v2", []string{"A", "B"}, "规则", .8, 1}, now)
	_ = batch.RegisterItem(RegisterItemInput{ItemID: "001", SourceRef: "a", ContentDigest: "d", DurationMS: 1,
		SpeakerCode: "SPK", AnnotatorA: "one", AnnotatorB: "two"}, now)
	_ = batch.SubmitAnnotation(SubmitAnnotationInput{ItemID: "001", Seat: "A", AnnotatorID: "one", Transcript: "文本", Labels: []string{"A"}}, now)
	_ = batch.SubmitAnnotation(SubmitAnnotationInput{ItemID: "001", Seat: "B", AnnotatorID: "two", Transcript: "文本", Labels: []string{"B"}}, now)
	id := "d-001-labels"
	err := batch.Resolve(ResolveInput{id, "A,A", "理由", "judge", "rubric-v2"}, now)
	if ErrorCode(err) != "duplicate_resolution_label" || !batch.Disagreements[id].ResolvedAt.IsZero() {
		t.Fatalf("重复裁决标签未原子拒绝：%v", err)
	}
	err = batch.Resolve(ResolveInput{id, "A,B", "理由", "judge", "rubric-v1"}, now)
	if ErrorCode(err) != "evidence_version_mismatch" || !batch.Disagreements[id].ResolvedAt.IsZero() {
		t.Fatalf("过期证据版本未原子拒绝：%v", err)
	}
	if err := batch.Resolve(ResolveInput{id, "B,A", "理由", "judge", "rubric-v2"}, now); err != nil {
		t.Fatal(err)
	}
	if batch.State != StateAuditing || batch.Items["001"].ItemState != ItemAdjudicated {
		t.Fatalf("裁决后状态异常：%s/%s", batch.State, batch.Items["001"].ItemState)
	}
	if err := batch.Resolve(ResolveInput{id, "A", "再改", "judge", "rubric-v2"}, now); ErrorCode(err) != "already_resolved" {
		t.Fatalf("重复裁决未返回 already_resolved：%v", err)
	}
}
