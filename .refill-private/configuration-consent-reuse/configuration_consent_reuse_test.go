package configurationconsentreuse_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/application"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/eventstore"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/httpapi"
)

type caseResponse struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

type preflightResponse struct {
	ConfirmationDigest   string `json:"confirmationDigest"`
	RequiresConfirmation bool   `json:"requiresConfirmation"`
}

func serveJSON(t *testing.T, handler http.Handler, method, path string, body any, dst any) int {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if dst != nil {
		if err := json.Unmarshal(recorder.Body.Bytes(), dst); err != nil {
			t.Fatalf("解析 %s %s 响应失败：%v；响应=%s", method, path, err, recorder.Body.String())
		}
	}
	return recorder.Code
}

func TestConfigurationRequestReuseCannotInheritInvalidationConsent(t *testing.T) {
	store, err := eventstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := httpapi.New(application.New(store), http.NotFoundHandler())

	var created caseResponse
	status := serveJSON(t, handler, http.MethodPost, "/api/v1/cases", map[string]any{
		"expectedVersion": 0,
		"idempotencyKey":  "create-consent-case",
		"actor":           "主管甲",
		"id":              "case-consent-reuse",
		"title":           "确认状态隔离测试",
		"venue":           "实验剧场",
		"performanceAt":   "2026-08-25T18:00:00Z",
		"managerName":     "主管甲",
	}, &created)
	if status != http.StatusCreated {
		t.Fatalf("创建档案失败：HTTP %d", status)
	}

	points := []map[string]any{{
		"id": "point-1", "label": "一号吊点", "position": "台口", "ratedLoadKg": 1000,
	}}
	items := []map[string]any{{
		"id": "item-1", "kind": "hoist", "label": "一号葫芦", "serialNumber": "H-001",
		"selfWeightKg": 200, "workingLoadLimitKg": 1000,
		"loadPointShares": []map[string]any{{"loadPointId": "point-1", "basisPoints": 10000}},
	}}
	var configured caseResponse
	status = serveJSON(t, handler, http.MethodPut, "/api/v1/cases/"+created.ID+"/configuration", map[string]any{
		"expectedVersion": created.Version,
		"idempotencyKey":  "initial-configuration",
		"actor":           "主管甲",
		"loadPoints":      points,
		"items":           items,
	}, &configured)
	if status != http.StatusOK {
		t.Fatalf("初始配置失败：HTTP %d", status)
	}

	checks := []map[string]any{}
	for _, code := range []string{"hardware", "connection", "routing", "capacity", "clearance"} {
		checks = append(checks, map[string]any{"code": code, "passed": true})
	}
	var inspected caseResponse
	status = serveJSON(t, handler, http.MethodPost, "/api/v1/cases/"+created.ID+"/inspections", map[string]any{
		"expectedVersion":      configured.Version,
		"idempotencyKey":       "operator-inspection",
		"actor":                "操作员乙",
		"id":                   "inspection-1",
		"role":                 "operator",
		"inspectorName":        "操作员乙",
		"configurationVersion": configured.Version,
		"checkItems":           checks,
		"findings":             []any{},
	}, &inspected)
	if status != http.StatusOK {
		t.Fatalf("提交检查失败：HTTP %d", status)
	}

	changedItems := []map[string]any{{
		"id": "item-1", "kind": "hoist", "label": "一号葫芦", "serialNumber": "H-001",
		"selfWeightKg": 250, "workingLoadLimitKg": 1000,
		"loadPointShares": []map[string]any{{"loadPointId": "point-1", "basisPoints": 10000}},
	}}
	var preview preflightResponse
	status = serveJSON(t, handler, http.MethodPost, "/api/v1/cases/"+created.ID+"/configuration/preflight", map[string]any{
		"expectedVersion":     inspected.Version,
		"loadPoints":          points,
		"items":               changedItems,
		"confirmInvalidation": true,
	}, &preview)
	if status != http.StatusOK || !preview.RequiresConfirmation || preview.ConfirmationDigest == "" {
		t.Fatalf("预检没有识别检查记录失效：HTTP %d，结果=%+v", status, preview)
	}

	var submission map[string]any
	status = serveJSON(t, handler, http.MethodPut, "/api/v1/cases/"+created.ID+"/configuration", map[string]any{
		"expectedVersion": inspected.Version,
		"idempotencyKey":  "unconfirmed-reconfiguration",
		"actor":           "主管甲",
		"loadPoints":      points,
		"items":           changedItems,
		"preflightDigest": preview.ConfirmationDigest,
	}, &submission)
	if status != http.StatusConflict {
		t.Fatalf("省略 confirmInvalidation 的新请求必须独立返回 HTTP 409，实际 HTTP %d，响应=%v", status, submission)
	}
	errorBody, _ := submission["error"].(map[string]any)
	if errorBody["code"] != "INVALIDATION_CONFIRMATION_REQUIRED" {
		t.Fatalf("应返回 INVALIDATION_CONFIRMATION_REQUIRED，实际响应=%v", submission)
	}
}
