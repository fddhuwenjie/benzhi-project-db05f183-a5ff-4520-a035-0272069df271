package domain

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maximumDurationMS int64 = 24 * 60 * 60 * 1000

var (
	dashRangePattern = regexp.MustCompile(`^([^0-9]*[0-9]+)-([^0-9]*[0-9]+)$`)
	itemIDPattern    = regexp.MustCompile(`^([^0-9]*)([0-9]+)$`)
	speakerPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
)

type RegisterItemInput struct {
	ItemID        string `json:"item_id"`
	SourceRef     string `json:"source_ref"`
	ContentDigest string `json:"content_digest"`
	DurationMS    int64  `json:"duration_ms"`
	SpeakerCode   string `json:"speaker_code"`
	AnnotatorA    string `json:"annotator_a"`
	AnnotatorB    string `json:"annotator_b"`
}

func (b *CorpusBatch) RegisterItem(input RegisterItemInput, now time.Time) error {
	if err := b.requireState(StateAnnotating); err != nil {
		return err
	}
	id := strings.TrimSpace(input.ItemID)
	if id == "" || strings.TrimSpace(input.SourceRef) == "" || strings.TrimSpace(input.ContentDigest) == "" {
		return rule("validation", "条目身份、来源引用和内容摘要不能为空")
	}
	if input.DurationMS <= 0 || input.DurationMS > maximumDurationMS {
		return rule("invalid_duration", "语音时长必须大于 0 且不超过 %d 毫秒", maximumDurationMS)
	}
	a := strings.TrimSpace(input.AnnotatorA)
	bb := strings.TrimSpace(input.AnnotatorB)
	if a == "" || bb == "" {
		return rule("validation", "两个标注席位均需分配")
	}
	if a == bb {
		return rule("seat_conflict", "同一标注员不能占用双重席位")
	}
	speaker := strings.TrimSpace(input.SpeakerCode)
	if !speakerPattern.MatchString(speaker) {
		return rule("invalid_speaker_code", "speaker_code 必须由字母、数字、连字符或下划线组成，且长度不超过 64")
	}
	if err := validateItemIDInRange(id, b.ItemRange); err != nil {
		return err
	}
	if _, exists := b.Items[id]; exists {
		return rule("duplicate_item_id", "条目编号 %s 已在批次中登记，冲突条目为 %s", id, id)
	}
	source := strings.TrimSpace(input.SourceRef)
	digest := strings.TrimSpace(input.ContentDigest)
	existingIDs := make([]string, 0, len(b.Items))
	for existingID := range b.Items {
		existingIDs = append(existingIDs, existingID)
	}
	sort.Strings(existingIDs)
	for _, existingID := range existingIDs {
		item := b.Items[existingID]
		if item.SourceRef == source {
			return rule("duplicate_source_ref", "source_ref 与条目 %s 冲突", existingID)
		}
	}
	for _, existingID := range existingIDs {
		item := b.Items[existingID]
		if item.ContentDigest == digest {
			return rule("duplicate_content_digest", "content_digest 与条目 %s 冲突", existingID)
		}
	}
	b.Items[id] = &UtteranceItem{ItemID: id, BatchID: b.BatchID, SourceRef: source,
		ContentDigest: digest, DurationMS: input.DurationMS, SpeakerCode: speaker,
		AnnotatorA: a, AnnotatorB: bb, ItemState: ItemRegistered, PreflightStatus: "PASSED",
		PreflightWarnings: []string{}, CanAnnotate: true, RegisteredAt: now.UTC()}
	b.UpdatedAt = now.UTC()
	return nil
}

func validateItemIDInRange(itemID, itemRange string) error {
	compact := strings.ReplaceAll(strings.TrimSpace(itemRange), " ", "")
	var endpoints []string
	if strings.Contains(compact, "..") {
		endpoints = strings.SplitN(compact, "..", 2)
	} else if parts := dashRangePattern.FindStringSubmatch(compact); len(parts) == 3 {
		endpoints = parts[1:]
	}
	if len(endpoints) != 2 {
		return rule("invalid_item_range", "批次冻结的条目范围 %q 无法解析", itemRange)
	}
	startParts := itemIDPattern.FindStringSubmatch(endpoints[0])
	endParts := itemIDPattern.FindStringSubmatch(endpoints[1])
	if len(startParts) != 3 || len(endParts) != 3 {
		return rule("invalid_item_range", "批次冻结的条目范围 %q 无法解析", itemRange)
	}
	start, startErr := strconv.ParseInt(startParts[2], 10, 64)
	end, endErr := strconv.ParseInt(endParts[2], 10, 64)
	if startErr != nil || endErr != nil || start > end {
		return rule("invalid_item_range", "批次冻结的条目范围 %q 无效", itemRange)
	}
	if startParts[1] != endParts[1] {
		return rule("invalid_item_range", "条目范围起止前缀必须一致")
	}
	idParts := itemIDPattern.FindStringSubmatch(itemID)
	if len(idParts) != 3 {
		return rule("item_id_format", "item_id %q 不符合冻结范围格式", itemID)
	}
	if idParts[1] != startParts[1] {
		return rule("item_id_format", "item_id %q 的前缀与冻结范围不一致", itemID)
	}
	width := len(startParts[2])
	if len(endParts[2]) > width {
		width = len(endParts[2])
	}
	if len(idParts[2]) != width {
		return rule("item_id_format", "item_id %q 的数字宽度应为 %d", itemID, width)
	}
	number, err := strconv.ParseInt(idParts[2], 10, 64)
	if err != nil || number < start || number > end {
		return rule("item_id_out_of_range", "item_id %q 超出冻结范围 %s", itemID, itemRange)
	}
	return nil
}

func submissionKey(itemID, seat string) string { return itemID + ":" + seat }
