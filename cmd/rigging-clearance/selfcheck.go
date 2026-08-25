package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type checkCase struct {
	ID         string `json:"id"`
	Version    int    `json:"version"`
	Status     string `json:"status"`
	Credential *struct {
		Number string `json:"number"`
	} `json:"credential"`
}

func runSelfcheck(ctx context.Context, base string) error {
	client := &http.Client{Timeout: 4 * time.Second}
	if _, err := checkRequest(ctx, client, http.MethodGet, base+"/healthz", nil, nil); err != nil {
		return fmt.Errorf("健康检查失败: %w", err)
	}
	performance := time.Now().UTC().Add(8 * time.Hour).Truncate(time.Second)
	var c checkCase
	create := map[string]any{"expectedVersion": 0, "idempotencyKey": "selfcheck-create", "actor": "系统自检", "id": "case-selfcheck", "title": "自检演出", "venue": "自检剧场", "performanceAt": performance, "managerName": "自检主管"}
	if _, err := checkRequest(ctx, client, http.MethodPost, base+"/api/v1/cases", create, &c); err != nil {
		return err
	}
	configuration := map[string]any{"expectedVersion": c.Version, "idempotencyKey": "selfcheck-config", "actor": "自检主管", "loadPoints": []any{map[string]any{"id": "LP-1", "label": "自检主吊点", "position": "中线", "ratedLoadKg": 1000}}, "items": []any{map[string]any{"id": "ITEM-1", "kind": "hoist", "label": "自检葫芦", "serialNumber": "TEST-001", "selfWeightKg": 250, "workingLoadLimitKg": 1000, "loadPointShares": []any{map[string]any{"loadPointId": "LP-1", "basisPoints": 10000}}}}}
	var preview struct {
		ConfirmationDigest string `json:"confirmationDigest"`
	}
	if _, err := checkRequest(ctx, client, http.MethodPost, base+"/api/v1/cases/"+c.ID+"/configuration/preflight", configuration, &preview); err != nil {
		return err
	}
	if preview.ConfirmationDigest == "" {
		return fmt.Errorf("载荷配置预检未返回确认摘要")
	}
	configuration["preflightDigest"] = preview.ConfirmationDigest
	if _, err := checkRequest(ctx, client, http.MethodPut, base+"/api/v1/cases/"+c.ID+"/configuration", configuration, &c); err != nil {
		return err
	}
	configVersion := c.Version
	checks := []any{}
	for _, code := range []string{"hardware", "connection", "routing", "capacity", "clearance"} {
		checks = append(checks, map[string]any{"code": code, "passed": true})
	}
	operator := map[string]any{"expectedVersion": c.Version, "idempotencyKey": "selfcheck-operator", "actor": "操作员甲", "role": "operator", "inspectorName": "操作员甲", "configurationVersion": configVersion, "checkItems": checks, "findings": []any{}}
	if _, err := checkRequest(ctx, client, http.MethodPost, base+"/api/v1/cases/"+c.ID+"/inspections", operator, &c); err != nil {
		return err
	}
	reviewer := map[string]any{"expectedVersion": c.Version, "idempotencyKey": "selfcheck-reviewer", "actor": "复核员乙", "role": "reviewer", "inspectorName": "复核员乙", "configurationVersion": configVersion, "checkItems": checks, "findings": []any{}}
	if _, err := checkRequest(ctx, client, http.MethodPost, base+"/api/v1/cases/"+c.ID+"/inspections", reviewer, &c); err != nil {
		return err
	}
	criteria := []any{}
	for _, stage := range []string{"quarter", "half", "full", "hold"} {
		criteria = append(criteria, map[string]any{"stage": stage, "minDurationSec": 20, "maxDeflectionMm": 10})
	}
	standard := map[string]any{"expectedVersion": c.Version, "idempotencyKey": "selfcheck-standard", "actor": "自检主管", "stages": criteria, "allowedReboundMm": 5, "maxTotalDurationSec": 1800}
	if _, err := checkRequest(ctx, client, http.MethodPut, base+"/api/v1/cases/"+c.ID+"/trial-standard", standard, &c); err != nil {
		return err
	}
	completed := time.Now().UTC()
	started := completed.Add(-2 * time.Minute)
	stages := []any{}
	for index, stage := range []string{"quarter", "half", "full", "hold"} {
		stages = append(stages, map[string]any{"stage": stage, "durationSec": 30, "deflectionMm": 2, "stable": true, "completedAt": started.Add(time.Duration(index+1) * 30 * time.Second)})
	}
	trial := map[string]any{"expectedVersion": c.Version, "idempotencyKey": "selfcheck-trial", "actor": "操作员甲", "operatorName": "操作员甲", "startedAt": started, "deadlineAt": started.Add(20 * time.Minute), "stageObservations": stages, "anomalies": []any{}}
	if _, err := checkRequest(ctx, client, http.MethodPost, base+"/api/v1/cases/"+c.ID+"/trial-lifts", trial, &c); err != nil {
		return err
	}
	freeze := map[string]any{"expectedVersion": c.Version, "idempotencyKey": "selfcheck-freeze", "actor": "自检主管"}
	if _, err := checkRequest(ctx, client, http.MethodPost, base+"/api/v1/cases/"+c.ID+"/freeze", freeze, &c); err != nil {
		return err
	}
	issue := map[string]any{"expectedVersion": c.Version, "idempotencyKey": "selfcheck-issue", "actor": "自检主管", "issuedBy": "自检主管"}
	if _, err := checkRequest(ctx, client, http.MethodPost, base+"/api/v1/cases/"+c.ID+"/credentials", issue, &c); err != nil {
		return err
	}
	if c.Credential == nil || c.Status != "released" {
		return fmt.Errorf("签发后状态或凭据缺失")
	}
	var verified struct {
		Valid    bool  `json:"valid"`
		Timeline []any `json:"timeline"`
	}
	if _, err := checkRequest(ctx, client, http.MethodGet, base+"/api/v1/credentials/"+c.Credential.Number, nil, &verified); err != nil {
		return err
	}
	if !verified.Valid || len(verified.Timeline) < 7 {
		return fmt.Errorf("凭据核验或审计轨迹不完整")
	}
	var dashboard struct {
		Cases      []any `json:"cases"`
		Statistics struct {
			PendingRelease int `json:"pendingRelease"`
		} `json:"statistics"`
	}
	if _, err := checkRequest(ctx, client, http.MethodGet, base+"/api/v1/cases?riskLevel=released", nil, &dashboard); err != nil {
		return err
	}
	if len(dashboard.Cases) != 1 || dashboard.Statistics.PendingRelease != 0 {
		return fmt.Errorf("风险看板未正确排除已放行档案")
	}
	return nil
}

func checkRequest(ctx context.Context, client *http.Client, method, url string, body any, out any) (int, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return response.StatusCode, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return response.StatusCode, fmt.Errorf("%s %s 返回 %d: %s", method, url, response.StatusCode, string(data))
	}
	if out != nil && len(data) > 0 {
		if err = json.Unmarshal(data, out); err != nil {
			return response.StatusCode, err
		}
	}
	return response.StatusCode, nil
}
