package dashboard_cache_stale_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/application"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/eventstore"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/httpapi"
)

func TestDashboardCacheInvalidatesAfterCommittedTransition(t *testing.T) {
	store, err := eventstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := httpapi.New(application.New(store), http.NotFoundHandler())

	create := map[string]any{
		"id": "cache-case", "expectedVersion": 0, "idempotencyKey": "cache-create", "actor": "缓存测试主管",
		"title": "缓存一致性场次", "venue": "实验剧场", "managerName": "缓存测试主管",
		"performanceAt": time.Date(2032, 6, 1, 12, 0, 0, 0, time.UTC),
	}
	requestJSON(t, handler, http.MethodPost, "/api/v1/cases", create, http.StatusCreated)

	var draftDashboard dashboardResponse
	draftBody := requestJSON(t, handler, http.MethodGet, "/api/v1/cases?status=draft", nil, http.StatusOK)
	if err := json.Unmarshal(draftBody, &draftDashboard); err != nil {
		t.Fatal(err)
	}
	if len(draftDashboard.Cases) != 1 || draftDashboard.Cases[0].Status != "draft" {
		t.Fatalf("建立缓存前应看到 draft 档案，实际为 %+v", draftDashboard.Cases)
	}

	configuration := map[string]any{
		"expectedVersion": 1, "idempotencyKey": "cache-config", "actor": "缓存测试主管",
		"loadPoints": []any{map[string]any{"id": "LP-1", "label": "主吊点", "position": "中线", "ratedLoadKg": 1000}},
		"items": []any{map[string]any{
			"id": "ITEM-1", "kind": "hoist", "label": "主葫芦", "serialNumber": "CACHE-001",
			"selfWeightKg": 250, "workingLoadLimitKg": 1000,
			"loadPointShares": []any{map[string]any{"loadPointId": "LP-1", "basisPoints": 10000}},
		}},
	}
	requestJSON(t, handler, http.MethodPut, "/api/v1/cases/cache-case/configuration", configuration, http.StatusOK)

	var detail detailResponse
	detailBody := requestJSON(t, handler, http.MethodGet, "/api/v1/cases/cache-case", nil, http.StatusOK)
	if err := json.Unmarshal(detailBody, &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Case.Status != "inspection" || detail.Case.Version != 2 {
		t.Fatalf("配置提交应已进入 inspection/version 2，实际为 %+v", detail.Case)
	}

	var inspectionDashboard dashboardResponse
	inspectionBody := requestJSON(t, handler, http.MethodGet, "/api/v1/cases?status=inspection", nil, http.StatusOK)
	if err := json.Unmarshal(inspectionBody, &inspectionDashboard); err != nil {
		t.Fatal(err)
	}
	if len(inspectionDashboard.Cases) != 1 || inspectionDashboard.Cases[0].ID != "cache-case" || inspectionDashboard.Cases[0].Version != 2 {
		t.Fatalf("提交成功后 dashboard 必须读取 inspection/version 2，缓存却返回 %+v", inspectionDashboard.Cases)
	}
}

type dashboardResponse struct {
	Cases []struct {
		ID      string `json:"id"`
		Status  string `json:"status"`
		Version int    `json:"version"`
	} `json:"cases"`
}

type detailResponse struct {
	Case struct {
		Status  string `json:"status"`
		Version int    `json:"version"`
	} `json:"case"`
}

func requestJSON(t *testing.T, handler http.Handler, method, target string, value any, wantStatus int) []byte {
	t.Helper()
	var body bytes.Buffer
	if value != nil {
		if err := json.NewEncoder(&body).Encode(value); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, target, &body)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != wantStatus {
		t.Fatalf("%s %s 状态码=%d，响应=%s", method, target, recorder.Code, recorder.Body.String())
	}
	return recorder.Body.Bytes()
}
