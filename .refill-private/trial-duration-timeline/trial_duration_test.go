package trial_duration_timeline_test

import (
	"testing"
	"time"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/application"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/eventstore"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/rigging"
)

func TestTrialDurationsMustFitRecordedTimeline(t *testing.T) {
	store, err := eventstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
	service := application.New(store)
	service.SetClock(func() time.Time { return now })
	meta := func(version int, key, actor string) application.CommandMeta {
		return application.CommandMeta{ExpectedVersion: version, IdempotencyKey: key, Actor: actor}
	}
	c, err := service.Create(application.CreateCase{CommandMeta: meta(0, "create", "主管"), ID: "case-trial-time", Title: "试吊时间线", Venue: "大剧场", ManagerName: "主管", PerformanceAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.Configure(application.SetConfiguration{CommandMeta: meta(c.Version, "configure", "主管"), CaseID: c.ID, LoadPoints: []rigging.LoadPoint{{ID: "p", Label: "吊点", RatedLoadKg: 100}}, Items: []rigging.SuspendedItem{{ID: "i", Kind: "hoist", Label: "设备", SelfWeightKg: 10, WorkingLoadLimitKg: 100, LoadPointShares: []rigging.LoadShare{{LoadPointID: "p", BasisPoints: 10000}}}}})
	if err != nil {
		t.Fatal(err)
	}
	checks := make([]rigging.CheckItem, 0, len(rigging.RequiredChecks))
	for _, code := range rigging.RequiredChecks {
		checks = append(checks, rigging.CheckItem{Code: code, Passed: true})
	}
	configurationVersion := c.ConfigurationVersion
	c, err = service.Inspect(application.SubmitInspection{CommandMeta: meta(c.Version, "operator", "操作员"), CaseID: c.ID, ID: "operator", Role: "operator", InspectorName: "操作员", ConfigurationVersion: configurationVersion, CheckItems: checks})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.Inspect(application.SubmitInspection{CommandMeta: meta(c.Version, "reviewer", "复核员"), CaseID: c.ID, ID: "reviewer", Role: "reviewer", InspectorName: "复核员", ConfigurationVersion: configurationVersion, CheckItems: checks})
	if err != nil {
		t.Fatal(err)
	}
	criteria := make([]rigging.TrialStageCriterion, 0, len(rigging.RequiredTrialStages))
	for _, stage := range rigging.RequiredTrialStages {
		criteria = append(criteria, rigging.TrialStageCriterion{Stage: stage, MinDurationSec: 20, MaxDeflectionMM: 10})
	}
	c, err = service.ConfirmTrialStandard(application.ConfirmTrialStandard{CommandMeta: meta(c.Version, "standard", "主管"), CaseID: c.ID, Stages: criteria, AllowedReboundMM: 5, MaxTotalDurationSec: 100})
	if err != nil {
		t.Fatal(err)
	}
	started := now.Add(-80 * time.Second)
	observations := make([]rigging.StageObservation, 0, len(rigging.RequiredTrialStages))
	for i, stage := range rigging.RequiredTrialStages {
		completed := started.Add(time.Duration(i+1) * time.Second)
		observations = append(observations, rigging.StageObservation{Stage: stage, DurationSec: 20, DeflectionMM: 0, Stable: true, CompletedAt: &completed})
	}
	c, err = service.RecordTrial(application.RecordTrial{CommandMeta: meta(c.Version, "trial", "操作员"), CaseID: c.ID, ID: "trial", OperatorName: "操作员", StartedAt: started, DeadlineAt: started.Add(100 * time.Second), Stages: observations})
	if err != nil {
		return
	}
	if c.TrialLifts[len(c.TrialLifts)-1].Result == "passed" {
		t.Fatalf("四个声称各持续 20 秒的阶段在仅 4 秒的完成时间线上仍被判定通过")
	}
}
