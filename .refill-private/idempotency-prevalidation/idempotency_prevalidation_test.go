package idempotency_prevalidation_test

import (
	"testing"
	"time"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/application"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/eventstore"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/rigging"
)

func TestIdempotentCommandsReplayBeforeGeneratedStateAndValidation(t *testing.T) {
	store, err := eventstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
	service := application.New(store)
	service.SetClock(func() time.Time { return now })
	generated := application.CreateCase{CommandMeta: application.CommandMeta{ExpectedVersion: 0, IdempotencyKey: "generated-create", Actor: "主管"}, Title: "自动标识", Venue: "大剧场", ManagerName: "主管", PerformanceAt: now.Add(time.Hour)}
	if _, err = service.Create(generated); err != nil {
		t.Fatal(err)
	}
	_, createReplayErr := service.Create(generated)

	c, err := service.Create(application.CreateCase{CommandMeta: application.CommandMeta{ExpectedVersion: 0, IdempotencyKey: "explicit-create", Actor: "主管"}, ID: "case-config", Title: "配置重放", Venue: "大剧场", ManagerName: "主管", PerformanceAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	configure := application.SetConfiguration{CommandMeta: application.CommandMeta{ExpectedVersion: c.Version, IdempotencyKey: "configure", Actor: "主管"}, CaseID: c.ID, LoadPoints: []rigging.LoadPoint{{ID: "p", Label: "吊点", RatedLoadKg: 100}}, Items: []rigging.SuspendedItem{{ID: "i", Kind: "hoist", Label: "设备", SelfWeightKg: 10, WorkingLoadLimitKg: 100, LoadPointShares: []rigging.LoadShare{{LoadPointID: "p", BasisPoints: 10000}}}}}
	c, err = service.Configure(configure)
	if err != nil {
		t.Fatal(err)
	}
	checks := make([]rigging.CheckItem, 0, len(rigging.RequiredChecks))
	for _, code := range rigging.RequiredChecks {
		checks = append(checks, rigging.CheckItem{Code: code, Passed: true})
	}
	_, err = service.Inspect(application.SubmitInspection{CommandMeta: application.CommandMeta{ExpectedVersion: c.Version, IdempotencyKey: "inspect", Actor: "操作员"}, CaseID: c.ID, ID: "inspection", Role: "operator", InspectorName: "操作员", ConfigurationVersion: c.ConfigurationVersion, CheckItems: checks})
	if err != nil {
		t.Fatal(err)
	}
	_, configureReplayErr := service.Configure(configure)
	if createReplayErr != nil || configureReplayErr != nil {
		t.Fatalf("相同请求未在生成新 ID 和读取新聚合状态前重放：create=%v configure=%v", createReplayErr, configureReplayErr)
	}
}
