package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/application"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/eventstore"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/webui"
)

func TestRoutesServeUIAndStrictJSON(t *testing.T) {
	store, err := eventstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(application.New(store), webui.Handler()))
	defer server.Close()
	response, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 || !strings.Contains(response.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("页面响应无效：%d %s", response.StatusCode, response.Header.Get("Content-Type"))
	}
	response.Body.Close()
	body := `{"expectedVersion":0,"idempotencyKey":"k","actor":"A","title":"T","venue":"V","performanceAt":"2026-08-25T18:00:00Z","managerName":"M","unknown":true}`
	response, err = http.Post(server.URL+"/api/v1/cases", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 400 {
		t.Fatalf("未知字段应返回 400，实际 %d", response.StatusCode)
	}
	if response.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("缺少安全响应头")
	}
}
