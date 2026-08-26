package credentialsequencerestart_test

import (
	"strings"
	"testing"
	"time"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/application"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/eventstore"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/rigging"
)

func TestCredentialSequenceSurvivesServiceRestart(t *testing.T) {
	dir := t.TempDir()
	store, err := eventstore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2028, time.March, 18, 10, 0, 0, 0, time.UTC)
	seedFrozenCase(t, store, "case-alpha", "一号场", now, "seed-alpha")
	seedFrozenCase(t, store, "case-bravo", "二号场", now.Add(time.Hour), "seed-bravo")

	firstService := application.New(store)
	firstService.SetClock(func() time.Time { return now.Add(2 * time.Hour) })
	first, err := firstService.Issue(application.IssueCredential{
		CommandMeta: application.CommandMeta{ExpectedVersion: 2, IdempotencyKey: "issue-alpha", Actor: "主管甲"},
		CaseID:      "case-alpha",
		IssuedBy:    "主管甲",
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Credential == nil || first.Credential.Sequence != 1 {
		t.Fatalf("首次签发序号异常: %#v", first.Credential)
	}

	reopened, err := eventstore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	secondService := application.New(reopened)
	secondService.SetClock(func() time.Time { return now.Add(3 * time.Hour) })
	second, err := secondService.Issue(application.IssueCredential{
		CommandMeta: application.CommandMeta{ExpectedVersion: 2, IdempotencyKey: "issue-bravo", Actor: "主管乙"},
		CaseID:      "case-bravo",
		IssuedBy:    "主管乙",
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.Credential == nil {
		t.Fatal("重启后未生成凭据")
	}
	if second.Credential.Sequence != 2 || !strings.Contains(second.Credential.Number, "-0002-") {
		t.Fatalf("重启后签发序号未延续: first=%d second=%d number=%s", first.Credential.Sequence, second.Credential.Sequence, second.Credential.Number)
	}
}

func seedFrozenCase(t *testing.T, store *eventstore.Store, id, venue string, performanceAt time.Time, key string) {
	t.Helper()
	created, err := rigging.NewEvent(rigging.EventCaseCreated, id, "建档员", performanceAt.Add(-24*time.Hour), rigging.CreatedData{
		Title:         id,
		Venue:         venue,
		ManagerName:   map[string]string{"case-alpha": "主管甲", "case-bravo": "主管乙"}[id],
		PerformanceAt: performanceAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	frozenAt := performanceAt.Add(-time.Hour)
	frozen, err := rigging.NewEvent(rigging.EventManifestFrozen, id, "冻结员", frozenAt, rigging.FreezeData{Digest: "manifest-" + id, FrozenAt: frozenAt})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = store.Commit(0, []rigging.Event{created, frozen}, "准备员", key, map[string]string{"caseID": id}); err != nil {
		t.Fatal(err)
	}
}
