package snapshotrollbackretry_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/application"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/eventstore"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/rigging"
)

func TestSnapshotFailureRetryKeepsEventChainRecoverable(t *testing.T) {
	dir := t.TempDir()
	store, err := eventstore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	service := application.New(store)
	now := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
	service.SetClock(func() time.Time { return now })

	created, err := service.Create(application.CreateCase{
		CommandMeta:   application.CommandMeta{ExpectedVersion: 0, IdempotencyKey: "create-snapshot-retry", Actor: "舞台主管"},
		ID:            "case-snapshot-retry",
		Title:         "快照失败重试复现",
		Venue:         "实验剧场",
		ManagerName:   "舞台主管",
		PerformanceAt: now.Add(8 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	configuration := application.SetConfiguration{
		CommandMeta: application.CommandMeta{ExpectedVersion: created.Version, IdempotencyKey: "configure-snapshot-retry", Actor: "舞台主管"},
		CaseID:      created.ID,
		LoadPoints:  []rigging.LoadPoint{{ID: "point-1", CaseID: created.ID, Label: "主吊点", RatedLoadKg: 1000}},
		Items: []rigging.SuspendedItem{{
			ID: "item-1", CaseID: created.ID, Kind: "hoist", Label: "主葫芦", SelfWeightKg: 200, WorkingLoadLimitKg: 1000,
			LoadPointShares: []rigging.LoadShare{{LoadPointID: "point-1", BasisPoints: 10000}},
		}},
	}
	configured, err := service.Configure(configuration)
	if err != nil {
		t.Fatal(err)
	}
	checks := make([]rigging.CheckItem, 0, len(rigging.RequiredChecks))
	for _, code := range rigging.RequiredChecks {
		checks = append(checks, rigging.CheckItem{Code: code, Passed: true})
	}
	command := application.SubmitInspection{
		CommandMeta:          application.CommandMeta{ExpectedVersion: configured.Version, IdempotencyKey: "inspect-snapshot-retry", Actor: "操作员甲"},
		CaseID:               configured.ID,
		ID:                   "inspection-snapshot-retry",
		Role:                 "operator",
		InspectorName:        "操作员甲",
		ConfigurationVersion: configured.ConfigurationVersion,
		CheckItems:           checks,
	}

	snapshotPath := filepath.Join(dir, "projection.json")
	if err = os.Remove(snapshotPath); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(snapshotPath, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Inspect(command); err == nil {
		t.Fatal("快照路径失效时首次提交应向调用方报告错误")
	}

	if err = os.Remove(snapshotPath); err != nil {
		t.Fatal(err)
	}
	retried, err := service.Inspect(command)
	if err != nil {
		t.Fatalf("同一 idempotencyKey 重试应返回已持久化结果：%v", err)
	}
	if retried.Version != configured.Version+1 {
		t.Fatalf("重试结果版本错误：got %d want %d", retried.Version, configured.Version+1)
	}

	reopened, err := eventstore.Open(dir)
	if err != nil {
		t.Fatalf("快照故障恢复并重试后事件链必须仍可重启：%v", err)
	}
	recovered, err := reopened.Get(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Version != retried.Version || recovered.Status != rigging.StatusInspection || len(recovered.Inspections) != 1 {
		t.Fatalf("重启投影与重试响应不一致：status=%s version=%d", recovered.Status, recovered.Version)
	}
}
