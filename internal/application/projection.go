package application

import (
	"sort"
	"sync"

	"dialectcorpusreleasegate/internal/domain"
)

var projectionItemOrderCache = struct {
	sync.Mutex
	values map[string][]string
}{values: map[string][]string{}}

type WorkbenchProjection struct {
	Gates               []GateView             `json:"gates"`
	Items               []ItemView             `json:"items"`
	Tasks               []TaskView             `json:"tasks"`
	Metrics             MetricsView            `json:"metrics"`
	AgreementSummary    AgreementSummaryView   `json:"agreement_summary"`
	LowAgreementItems   []LowAgreementItemView `json:"low_agreement_items"`
	AnnotatorStatistics []AnnotatorView        `json:"annotator_statistics"`
}

type GateView struct {
	State  domain.BatchState `json:"state"`
	Label  string            `json:"label"`
	Status string            `json:"status"`
	Reason string            `json:"reason,omitempty"`
}
type ItemView struct {
	ItemID            string                     `json:"item_id"`
	State             domain.ItemState           `json:"state"`
	AnnotatorA        string                     `json:"annotator_a"`
	AnnotatorB        string                     `json:"annotator_b"`
	SeatASubmitted    bool                       `json:"seat_a_submitted"`
	SeatBSubmitted    bool                       `json:"seat_b_submitted"`
	Agreement         float64                    `json:"agreement"`
	OpenDisagreements int                        `json:"open_disagreements"`
	SourceRef         string                     `json:"source_ref"`
	ContentDigest     string                     `json:"content_digest"`
	PreflightStatus   string                     `json:"preflight_status"`
	CanAnnotate       bool                       `json:"can_annotate"`
	AgreementStats    domain.AgreementStatistics `json:"agreement_statistics"`
}
type AgreementSummaryView struct {
	ComparedItems       int     `json:"compared_items"`
	TranscriptAgreement float64 `json:"transcript_agreement"`
	LabelsAgreement     float64 `json:"labels_agreement"`
	WeightedAverage     float64 `json:"weighted_average"`
}
type DifferenceView struct {
	DisagreementID string `json:"disagreement_id"`
	Field          string `json:"field"`
	ValueA         string `json:"value_a"`
	ValueB         string `json:"value_b"`
}
type LowAgreementItemView struct {
	ItemID      string           `json:"item_id"`
	Agreement   float64          `json:"agreement"`
	Minimum     float64          `json:"minimum_agreement"`
	Differences []DifferenceView `json:"differences"`
}
type AnnotatorView struct {
	AnnotatorID      string  `json:"annotator_id"`
	AssignedSeats    int     `json:"assigned_seats"`
	SubmittedSeats   int     `json:"submitted_seats"`
	UnsubmittedSeats int     `json:"unsubmitted_seats"`
	AverageAgreement float64 `json:"average_agreement"`
}
type TaskView struct {
	Kind        string `json:"kind"`
	ItemID      string `json:"item_id,omitempty"`
	ReferenceID string `json:"reference_id,omitempty"`
	Label       string `json:"label"`
}
type MetricsView struct {
	TotalItems        int `json:"total_items"`
	SubmittedSeats    int `json:"submitted_seats"`
	OpenDisagreements int `json:"open_disagreements"`
	AuditRounds       int `json:"audit_rounds"`
	CorrectionPending int `json:"correction_pending"`
}

func BuildWorkbenchProjection(batch *domain.CorpusBatch) WorkbenchProjection {
	projection := WorkbenchProjection{Gates: buildGates(batch), Items: []ItemView{}, Tasks: []TaskView{},
		LowAgreementItems: []LowAgreementItemView{}, AnnotatorStatistics: []AnnotatorView{}}
	itemIDs := projectionItemIDs(batch)
	openByItem := map[string]int{}
	differencesByItem := map[string][]DifferenceView{}
	disagreementIDs := make([]string, 0, len(batch.Disagreements))
	for id, disagreement := range batch.Disagreements {
		if disagreement.ResolvedAt.IsZero() {
			openByItem[disagreement.ItemID]++
			disagreementIDs = append(disagreementIDs, id)
			differencesByItem[disagreement.ItemID] = append(differencesByItem[disagreement.ItemID], DifferenceView{
				DisagreementID: id, Field: disagreement.FieldPath, ValueA: disagreement.ValueA, ValueB: disagreement.ValueB})
		}
	}
	sort.Strings(disagreementIDs)
	type annotatorAccumulator struct {
		assigned, submitted, compared int
		agreement                     float64
	}
	annotators := map[string]*annotatorAccumulator{}
	transcriptMatched, labelsMatched, compared := 0, 0, 0
	for _, id := range itemIDs {
		item := batch.Items[id]
		aSubmitted := hasSubmission(batch, id, "A")
		bSubmitted := hasSubmission(batch, id, "B")
		projection.Items = append(projection.Items, ItemView{ItemID: id, State: item.ItemState, AnnotatorA: item.AnnotatorA,
			AnnotatorB: item.AnnotatorB, SeatASubmitted: aSubmitted, SeatBSubmitted: bSubmitted,
			Agreement: item.Agreement, OpenDisagreements: openByItem[id], SourceRef: item.SourceRef,
			ContentDigest: item.ContentDigest, PreflightStatus: item.PreflightStatus, CanAnnotate: item.CanAnnotate,
			AgreementStats: item.AgreementStats})
		if item.AgreementStats.Compared {
			compared++
			if item.AgreementStats.TranscriptAgreement {
				transcriptMatched++
			}
			if item.AgreementStats.LabelsAgreement {
				labelsMatched++
			}
			if item.Agreement < batch.MinimumAgreement {
				projection.LowAgreementItems = append(projection.LowAgreementItems, LowAgreementItemView{ItemID: id,
					Agreement: item.Agreement, Minimum: batch.MinimumAgreement, Differences: differencesByItem[id]})
				projection.Tasks = append(projection.Tasks, TaskView{Kind: "low_agreement", ItemID: id, Label: "低一致项待裁决"})
			}
		}
		for _, assignment := range []struct {
			id        string
			submitted bool
		}{{item.AnnotatorA, aSubmitted}, {item.AnnotatorB, bSubmitted}} {
			acc := annotators[assignment.id]
			if acc == nil {
				acc = &annotatorAccumulator{}
				annotators[assignment.id] = acc
			}
			acc.assigned++
			if assignment.submitted {
				acc.submitted++
			}
			if item.AgreementStats.Compared {
				acc.compared++
				acc.agreement += item.Agreement
			}
		}
		if batch.State == domain.StateAnnotating {
			if !aSubmitted {
				projection.Tasks = append(projection.Tasks, TaskView{Kind: "annotation", ItemID: id, Label: "等待 A 席独立标注"})
			}
			if !bSubmitted {
				projection.Tasks = append(projection.Tasks, TaskView{Kind: "annotation", ItemID: id, Label: "等待 B 席独立标注"})
			}
		}
		if item.ItemState == domain.ItemCorrection {
			projection.Tasks = append(projection.Tasks, TaskView{Kind: "correction", ItemID: id, Label: "完成审计命中返修"})
			projection.Metrics.CorrectionPending++
		}
		if aSubmitted {
			projection.Metrics.SubmittedSeats++
		}
		if bSubmitted {
			projection.Metrics.SubmittedSeats++
		}
	}
	for _, id := range disagreementIDs {
		disagreement := batch.Disagreements[id]
		projection.Tasks = append(projection.Tasks, TaskView{Kind: "adjudication", ItemID: disagreement.ItemID,
			ReferenceID: id, Label: "裁决字段 " + disagreement.FieldPath})
	}
	projection.Metrics.TotalItems = len(batch.Items)
	projection.Metrics.OpenDisagreements = len(disagreementIDs)
	projection.Metrics.AuditRounds = len(batch.Audits)
	projection.AgreementSummary.ComparedItems = compared
	if compared > 0 {
		projection.AgreementSummary.TranscriptAgreement = float64(transcriptMatched) / float64(compared)
		projection.AgreementSummary.LabelsAgreement = float64(labelsMatched) / float64(compared)
		projection.AgreementSummary.WeightedAverage = float64(transcriptMatched+labelsMatched) / float64(compared*2)
	}
	annotatorIDs := make([]string, 0, len(annotators))
	for id := range annotators {
		annotatorIDs = append(annotatorIDs, id)
	}
	sort.Strings(annotatorIDs)
	for _, id := range annotatorIDs {
		acc := annotators[id]
		view := AnnotatorView{AnnotatorID: id, AssignedSeats: acc.assigned, SubmittedSeats: acc.submitted,
			UnsubmittedSeats: acc.assigned - acc.submitted}
		if acc.compared > 0 {
			view.AverageAgreement = acc.agreement / float64(acc.compared)
		}
		projection.AnnotatorStatistics = append(projection.AnnotatorStatistics, view)
	}
	return projection
}

func projectionItemIDs(batch *domain.CorpusBatch) []string {
	projectionItemOrderCache.Lock()
	defer projectionItemOrderCache.Unlock()
	if cached, ok := projectionItemOrderCache.values[batch.BatchID]; ok && len(cached) == len(batch.Items) {
		return cached
	}
	ids := make([]string, 0, len(batch.Items))
	for id := range batch.Items {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	projectionItemOrderCache.values[batch.BatchID] = ids
	return ids
}

func hasSubmission(batch *domain.CorpusBatch, itemID, seat string) bool {
	_, exists := batch.Submissions[itemID+":"+seat]
	return exists
}

func buildGates(batch *domain.CorpusBatch) []GateView {
	states := []domain.BatchState{domain.StateDraft, domain.StateAnnotating, domain.StateAdjudicating, domain.StateAuditing, domain.StateCorrection, domain.StateReleased}
	labels := []string{"建档与规范", "双人独立标注", "分歧裁决", "独立审计", "定向返修", "发布冻结"}
	current := 0
	for index, state := range states {
		if batch.State == state {
			current = index
		}
	}
	gates := make([]GateView, 0, len(states))
	for index, state := range states {
		status := "PENDING"
		if index < current {
			status = "COMPLETED"
		}
		if index == current {
			status = "CURRENT"
		}
		gate := GateView{State: state, Label: labels[index], Status: status}
		if state == domain.StateAdjudicating && batch.State == domain.StateAnnotating {
			gate.Reason = "等待所有条目完成双席提交"
		}
		if state == domain.StateAuditing && len(batch.Disagreements) > 0 && !allResolved(batch) {
			gate.Reason = "等待全部字段分歧闭合"
		}
		if state == domain.StateReleased && !latestAuditPassed(batch) {
			gate.Reason = "等待最近一轮独立审计通过"
		}
		gates = append(gates, gate)
	}
	return gates
}

func allResolved(batch *domain.CorpusBatch) bool {
	for _, disagreement := range batch.Disagreements {
		if disagreement.ResolvedAt.IsZero() {
			return false
		}
	}
	return true
}
func latestAuditPassed(batch *domain.CorpusBatch) bool {
	return len(batch.Audits) > 0 && batch.Audits[len(batch.Audits)-1].Outcome == "PASSED"
}
