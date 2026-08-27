package wrapped_store_error_classification_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/application"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/eventstore"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/httpapi"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/rigging"
)

type errorEnvelope struct {
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
}

func TestWrappedStoreErrorsPreserveHTTPClassification(t *testing.T) {
	store, err := eventstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(httpapi.New(application.New(store), http.NotFoundHandler()))
	defer server.Close()

	assertError := func(method, path string, body any, wantStatus int, wantCode string) {
		t.Helper()
		response := request(t, server.Client(), method, server.URL+path, body)
		defer response.Body.Close()
		var envelope errorEnvelope
		if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
			t.Fatalf("解析错误响应失败：%v", err)
		}
		if response.StatusCode != wantStatus || envelope.Error.Code != wantCode {
			t.Fatalf("%s %s 应保留 eventstore 错误分类 %d/%s，实际 %d/%s", method, path, wantStatus, wantCode, response.StatusCode, envelope.Error.Code)
		}
	}

	assertError(http.MethodGet, "/api/v1/cases/missing", nil, http.StatusNotFound, "NOT_FOUND")

	created := request(t, server.Client(), http.MethodPost, server.URL+"/api/v1/cases", map[string]any{
		"expectedVersion": 0,
		"idempotencyKey":  "create-key",
		"actor":           "主管",
		"id":              "case-errors",
		"title":           "包装错误测试",
		"venue":           "测试剧场",
		"performanceAt":   "2026-08-25T18:00:00Z",
		"managerName":     "主管",
	})
	if created.StatusCode != http.StatusCreated {
		data, _ := io.ReadAll(created.Body)
		created.Body.Close()
		t.Fatalf("创建档案失败：%d %s", created.StatusCode, data)
	}
	created.Body.Close()

	assertError(http.MethodPost, "/api/v1/cases", map[string]any{
		"expectedVersion": 0,
		"idempotencyKey":  "create-key",
		"actor":           "主管",
		"id":              "case-errors",
		"title":           "同键但不同标题",
		"venue":           "测试剧场",
		"performanceAt":   "2026-08-25T18:00:00Z",
		"managerName":     "主管",
	}, http.StatusConflict, "IDEMPOTENCY_CONFLICT")

	configured := request(t, server.Client(), http.MethodPut, server.URL+"/api/v1/cases/case-errors/configuration", map[string]any{
		"expectedVersion": 1,
		"idempotencyKey":  "configure-key",
		"actor":           "主管",
		"loadPoints": []map[string]any{{
			"id": "point-1", "label": "主吊点", "ratedLoadKg": 1000,
		}},
		"items": []map[string]any{{
			"id": "item-1", "kind": "hoist", "label": "葫芦", "selfWeightKg": 100, "workingLoadLimitKg": 1000,
			"loadPointShares": []map[string]any{{"loadPointId": "point-1", "basisPoints": 10000}},
		}},
	})
	if configured.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(configured.Body)
		configured.Body.Close()
		t.Fatalf("配置档案失败：%d %s", configured.StatusCode, data)
	}
	configured.Body.Close()

	checks := make([]map[string]any, 0, len(rigging.RequiredChecks))
	for _, code := range rigging.RequiredChecks {
		checks = append(checks, map[string]any{"code": code, "passed": true})
	}
	assertError(http.MethodPost, "/api/v1/cases/case-errors/inspections", map[string]any{
		"expectedVersion":      1,
		"idempotencyKey":       "stale-inspection-key",
		"actor":                "操作员甲",
		"id":                   "inspection-1",
		"role":                 "operator",
		"inspectorName":        "操作员甲",
		"configurationVersion": 2,
		"checkItems":           checks,
		"findings":             []any{},
	}, http.StatusConflict, "VERSION_CONFLICT")
}

func request(t *testing.T, client *http.Client, method, url string, body any) *http.Response {
	t.Helper()
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		payload = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return response
}
