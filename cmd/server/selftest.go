package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

type smokeClient struct {
	base     string
	http     *http.Client
	revision int64
	request  int
}

func runSelftest(server *http.Server, listener net.Listener, timeout time.Duration) error {
	contextValue, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	client := &smokeClient{base: "http://" + listener.Addr().String(), http: &http.Client{Timeout: 3 * time.Second}, revision: -1}
	flowErr := make(chan error, 1)
	go func() { flowErr <- client.fullFlow(contextValue) }()
	select {
	case err := <-flowErr:
		shutdownContext, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		_ = server.Shutdown(shutdownContext)
		if err != nil {
			return err
		}
		fmt.Println("自检通过：真实 HTTP 全流程完成审计失败、限定返修、定向复审、发布与摘要验证")
		return nil
	case err := <-serveErr:
		return fmt.Errorf("自检服务提前退出：%w", err)
	case <-contextValue.Done():
		_ = server.Close()
		return fmt.Errorf("自检超时：%w", contextValue.Err())
	}
}

func (c *smokeClient) fullFlow(ctx context.Context) error {
	batchID := "selftest-batch"
	if err := c.mutate(ctx, "/api/v1/batches", map[string]any{"batch_id": batchID, "title": "自检方言语料批次",
		"dialect_site": "自检语言点", "source_note": "自检生成的本地资料", "item_range": "u-001..u-002"}); err != nil {
		return err
	}
	base := "/api/v1/batches/" + batchID
	if err := c.mutate(ctx, base+"/freeze", map[string]any{"rubric_version": "rubric-v1", "label_set": []string{"陈述", "疑问"},
		"transcription_rules": "按实际音值规范转写", "minimum_agreement": .75, "audit_ratio": 1.0}); err != nil {
		return err
	}
	items := []map[string]any{
		{"item_id": "u-001", "source_ref": "audio/001.wav", "content_digest": "sha256-audio-001", "duration_ms": 1500, "speaker_code": "SPK-1", "annotator_a": "ann-a", "annotator_b": "ann-b"},
		{"item_id": "u-002", "source_ref": "audio/002.wav", "content_digest": "sha256-audio-002", "duration_ms": 2100, "speaker_code": "SPK-2", "annotator_a": "ann-c", "annotator_b": "ann-d"},
	}
	for _, item := range items {
		if err := c.mutate(ctx, base+"/items", item); err != nil {
			return err
		}
	}
	submissions := []map[string]any{
		{"submission_id": "s-1a", "item_id": "u-001", "seat": "A", "annotator_id": "ann-a", "transcript": "今日天晴", "labels": []string{"陈述"}},
		{"submission_id": "s-1b", "item_id": "u-001", "seat": "B", "annotator_id": "ann-b", "transcript": "今天晴", "labels": []string{"陈述"}},
		{"submission_id": "s-2a", "item_id": "u-002", "seat": "A", "annotator_id": "ann-c", "transcript": "你去吗", "labels": []string{"疑问"}},
		{"submission_id": "s-2b", "item_id": "u-002", "seat": "B", "annotator_id": "ann-d", "transcript": "你去吗", "labels": []string{"陈述"}},
	}
	for _, submission := range submissions {
		if err := c.mutate(ctx, base+"/annotations", submission); err != nil {
			return err
		}
	}
	resolutions := []map[string]any{
		{"disagreement_id": "d-u-001-transcript", "resolution": "今日天晴", "reason": "清晰音段支持 A", "adjudicator_id": "judge", "evidence_version": "rubric-v1"},
		{"disagreement_id": "d-u-002-labels", "resolution": "疑问", "reason": "句末语调符合疑问", "adjudicator_id": "judge", "evidence_version": "rubric-v1"},
	}
	for _, resolution := range resolutions {
		if err := c.mutate(ctx, base+"/disagreements/resolve", resolution); err != nil {
			return err
		}
	}
	var preview struct {
		SampleItemIDs []string `json:"sample_item_ids"`
	}
	if err := c.post(ctx, base+"/audit/preview", map[string]any{"sample_seed": "seed-one"}, &preview); err != nil {
		return err
	}
	findings := make([]map[string]any, 0, len(preview.SampleItemIDs))
	for index, id := range preview.SampleItemIDs {
		findings = append(findings, map[string]any{"item_id": id, "passed": index != 0, "note": "首轮自检结论"})
	}
	if err := c.mutate(ctx, base+"/audits", map[string]any{"sample_seed": "seed-one", "auditor_id": "auditor-independent", "findings": findings}); err != nil {
		return err
	}
	failedID := preview.SampleItemIDs[0]
	transcript := "今日天晴"
	labels := []string{"陈述"}
	if failedID == "u-002" {
		transcript = "你去吗"
		labels = []string{"疑问"}
	}
	if err := c.mutate(ctx, base+"/corrections", map[string]any{"item_id": failedID, "transcript": transcript, "labels": labels,
		"reason": "修复首轮审计命中问题", "corrector_id": "corrector"}); err != nil {
		return err
	}
	var focused struct {
		SampleItemIDs []string `json:"sample_item_ids"`
	}
	if err := c.post(ctx, base+"/audit/preview", map[string]any{"sample_seed": "seed-two"}, &focused); err != nil {
		return err
	}
	if len(focused.SampleItemIDs) != 1 || focused.SampleItemIDs[0] != failedID {
		return fmt.Errorf("定向复审范围异常：%v", focused.SampleItemIDs)
	}
	if err := c.mutate(ctx, base+"/audits", map[string]any{"sample_seed": "seed-two", "auditor_id": "auditor-independent",
		"findings": []map[string]any{{"item_id": failedID, "passed": true, "note": "返修复审通过"}}}); err != nil {
		return err
	}
	if err := c.mutate(ctx, base+"/release", map[string]any{"released_by": "release-lead"}); err != nil {
		return err
	}
	var verification struct {
		Valid bool `json:"valid"`
	}
	if err := c.post(ctx, base+"/verify", map[string]any{}, &verification); err != nil {
		return err
	}
	if !verification.Valid {
		return fmt.Errorf("发布摘要重算验证未通过")
	}
	var detail struct {
		Batch struct {
			State string `json:"state"`
		} `json:"batch"`
		Timeline struct {
			Events []any `json:"events"`
		} `json:"timeline"`
	}
	if err := c.get(ctx, base, &detail); err != nil {
		return err
	}
	if detail.Batch.State != "RELEASED" || len(detail.Timeline.Events) < 10 {
		return fmt.Errorf("终态或时间线不完整")
	}
	return nil
}

func (c *smokeClient) mutate(ctx context.Context, path string, body map[string]any) error {
	c.request++
	body["request_id"] = fmt.Sprintf("selftest-%02d", c.request)
	body["expected_revision"] = c.revision
	var result struct {
		Revision int64 `json:"revision"`
	}
	if err := c.post(ctx, path, body, &result); err != nil {
		return err
	}
	c.revision = result.Revision
	return nil
}

func (c *smokeClient) post(ctx context.Context, path string, body any, target any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, target)
}

func (c *smokeClient) get(ctx context.Context, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, target)
}

func (c *smokeClient) do(req *http.Request, target any) error {
	response, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("%s %s 返回 %d：%s", req.Method, req.URL.Path, response.StatusCode, data)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("解析 %s 响应：%w", req.URL.Path, err)
	}
	return nil
}
