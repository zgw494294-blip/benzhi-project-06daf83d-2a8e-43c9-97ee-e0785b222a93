package trial_failure_partial_commit_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/application"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/eventstore"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/rigging"
)

func TestFailedTrialCannotCommitWithoutAutomaticFinding(t *testing.T) {
	dir := t.TempDir()
	store, err := eventstore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
	caseID := "case-partial-trial"
	checks := make([]rigging.CheckItem, 0, len(rigging.RequiredChecks))
	for _, code := range rigging.RequiredChecks {
		checks = append(checks, rigging.CheckItem{Code: code, Passed: true})
	}
	criteria := make([]rigging.TrialStageCriterion, 0, len(rigging.RequiredTrialStages))
	for _, stage := range rigging.RequiredTrialStages {
		criteria = append(criteria, rigging.TrialStageCriterion{Stage: stage, MinDurationSec: 10, MaxDeflectionMM: 5})
	}
	standard := rigging.TrialStandard{ConfigurationVersion: 2, Stages: criteria, AllowedReboundMM: 2, MaxTotalDurationSec: 120, ConfirmedBy: "主管", ConfirmedAt: now}
	standard.Digest, err = rigging.TrialStandardDigest(standard)
	if err != nil {
		t.Fatal(err)
	}
	events := []rigging.Event{
		mustEvent(t, rigging.EventCaseCreated, caseID, "主管", now, rigging.CreatedData{Title: "边缘场景", Venue: "测试剧场", ManagerName: "主管", PerformanceAt: now.Add(8 * time.Hour)}),
		mustEvent(t, rigging.EventConfigurationSet, caseID, "主管", now, rigging.ConfigurationData{NextStatus: rigging.StatusInspection, ConfigurationVersion: 2}),
		mustEvent(t, rigging.EventInspectionSubmitted, caseID, "操作员甲", now, rigging.InspectionData{Inspection: rigging.InspectionRecord{ID: "operator-check", CaseID: caseID, Role: "operator", InspectorName: "操作员甲", ConfigurationVersion: 2, CheckItems: checks, SubmittedAt: now}}),
		mustEvent(t, rigging.EventInspectionSubmitted, caseID, "复核员乙", now, rigging.InspectionData{Inspection: rigging.InspectionRecord{ID: "reviewer-check", CaseID: caseID, Role: "reviewer", InspectorName: "复核员乙", ConfigurationVersion: 2, CheckItems: checks, SubmittedAt: now}}),
		mustEvent(t, rigging.EventTrialStandardConfirmed, caseID, "主管", now, rigging.TrialStandardData{Standard: standard}),
	}
	prepared, _, err := store.Commit(0, events, "测试准备", "prepare-trial", map[string]string{"caseId": caseID})
	if err != nil {
		t.Fatal(err)
	}

	logPath := filepath.Join(dir, "events.log")
	backupPath := filepath.Join(dir, "events.before-finding")
	clockCalls := 0
	var barrierErr error
	service := application.New(store)
	service.SetClock(func() time.Time {
		clockCalls++
		if clockCalls == 2 {
			if barrierErr = os.Rename(logPath, backupPath); barrierErr == nil {
				barrierErr = os.Mkdir(logPath, 0700)
			}
		}
		return now
	})
	started := now.Add(-80 * time.Second)
	stages := make([]rigging.StageObservation, 0, len(rigging.RequiredTrialStages))
	for i, stage := range rigging.RequiredTrialStages {
		completedAt := started.Add(time.Duration(i+1) * 10 * time.Second)
		stages = append(stages, rigging.StageObservation{Stage: stage, DurationSec: 10, DeflectionMM: 1, Stable: i != 0, CompletedAt: &completedAt})
	}
	_, err = service.RecordTrial(application.RecordTrial{
		CommandMeta:  application.CommandMeta{ExpectedVersion: prepared.Version, IdempotencyKey: "record-failed-trial", Actor: "操作员甲"},
		CaseID:       caseID,
		ID:           "failed-trial",
		OperatorName: "操作员甲",
		StartedAt:    started,
		DeadlineAt:   now.Add(40 * time.Second),
		Stages:       stages,
	})
	if barrierErr != nil {
		t.Fatalf("建立确定性日志失效屏障失败：%v", barrierErr)
	}
	if err == nil {
		t.Fatal("自动问题项追加时 events.log 已失效，调用应返回错误")
	}
	if err = os.Remove(logPath); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(backupPath, logPath); err != nil {
		t.Fatal(err)
	}
	reopened, err := eventstore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := reopened.Get(caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered.TrialLifts) != 0 || len(recovered.Findings) != 0 || recovered.Version != prepared.Version {
		t.Fatalf("失败调用留下半提交：trialLifts=%d findings=%d version=%d，调用前 version=%d", len(recovered.TrialLifts), len(recovered.Findings), recovered.Version, prepared.Version)
	}
}

func mustEvent(t *testing.T, kind, caseID, actor string, at time.Time, value any) rigging.Event {
	t.Helper()
	event, err := rigging.NewEvent(kind, caseID, actor, at, value)
	if err != nil {
		t.Fatal(err)
	}
	return event
}
