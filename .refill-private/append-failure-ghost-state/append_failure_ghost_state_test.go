package appendfailureghoststate_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/application"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/eventstore"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/httpapi"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/rigging"
)

func TestFailedAppendDoesNotPublishGhostCase(t *testing.T) {
	dir := t.TempDir()
	store, err := eventstore.Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	logPath := filepath.Join(dir, "events.log")
	if err := os.Mkdir(logPath, 0700); err != nil {
		t.Fatalf("create deterministic log blocker: %v", err)
	}

	handler := httpapi.New(application.New(store), http.NotFoundHandler())
	payload, err := json.Marshal(map[string]any{
		"id":              "case-ghost",
		"title":           "幽灵提交复现",
		"venue":           "一号舞台",
		"performanceAt":   "2030-01-02T03:04:05Z",
		"managerName":     "主管甲",
		"expectedVersion": 0,
		"idempotencyKey":  "ghost-create-1",
		"actor":           "主管甲",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	create := httptest.NewRequest(http.MethodPost, "/api/v1/cases", bytes.NewReader(payload))
	create.Header.Set("Content-Type", "application/json")
	createResult := httptest.NewRecorder()
	handler.ServeHTTP(createResult, create)
	if createResult.Code != http.StatusInternalServerError {
		t.Fatalf("fixture did not reach append failure: status=%d body=%s", createResult.Code, createResult.Body.String())
	}

	detail := httptest.NewRequest(http.MethodGet, "/api/v1/cases/case-ghost", nil)
	detailResult := httptest.NewRecorder()
	handler.ServeHTTP(detailResult, detail)

	if err := os.Remove(logPath); err != nil {
		t.Fatalf("remove log blocker: %v", err)
	}
	reopened, err := eventstore.Open(dir)
	if err != nil {
		t.Fatalf("restart Open() error = %v", err)
	}
	if _, err := reopened.Get("case-ghost"); !errors.Is(err, eventstore.ErrNotFound) {
		t.Fatalf("uncommitted case unexpectedly survived restart: %v", err)
	}
	var failures []string
	if detailResult.Code != http.StatusNotFound {
		failures = append(failures, "failed append leaked a ghost case before restart")
	}

	batchDir := t.TempDir()
	batchStore, err := eventstore.Open(batchDir)
	if err != nil {
		t.Fatalf("batch Open() error = %v", err)
	}
	occurredAt := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	created, err := rigging.NewEvent(rigging.EventCaseCreated, "case-partial", "主管乙", occurredAt, rigging.CreatedData{
		Title: "整批原子性复现", Venue: "二号舞台", ManagerName: "主管乙", PerformanceAt: occurredAt.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("NewEvent() error = %v", err)
	}
	invalid := rigging.Event{Type: "UnsupportedAtomicityEvent", CaseID: "case-partial", Actor: "主管乙", OccurredAt: occurredAt, Data: json.RawMessage(`{}`)}
	if _, _, err := batchStore.Commit(0, []rigging.Event{created, invalid}, "主管乙", "invalid-batch-1", map[string]string{"caseId": "case-partial"}); err == nil {
		t.Fatal("invalid event batch unexpectedly committed")
	}
	if _, err := batchStore.Get("case-partial"); !errors.Is(err, eventstore.ErrNotFound) {
		failures = append(failures, "invalid event batch partially polluted the live projection")
	}
	reopenedBatch, err := eventstore.Open(batchDir)
	if err != nil {
		failures = append(failures, "invalid event batch was persisted before complete validation")
	} else if _, err := reopenedBatch.Get("case-partial"); !errors.Is(err, eventstore.ErrNotFound) {
		failures = append(failures, "invalid event batch survived restart")
	}
	if len(failures) > 0 {
		t.Fatalf("transactional isolation violated: %s", strings.Join(failures, "; "))
	}
}
