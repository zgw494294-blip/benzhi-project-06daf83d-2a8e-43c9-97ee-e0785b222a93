package application

import (
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/eventstore"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/rigging"
)

func TestCompleteWorkflowWithRemediationAndRecovery(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "events")
	store, err := eventstore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
	service := New(store)
	service.SetClock(func() time.Time { return now })
	meta := func(version int, key, actor string) CommandMeta {
		return CommandMeta{ExpectedVersion: version, IdempotencyKey: key, Actor: actor}
	}
	c, err := service.Create(CreateCase{CommandMeta: meta(0, "create", "主管"), ID: "case-1", Title: "首演", Venue: "大剧场", ManagerName: "主管", PerformanceAt: now.Add(8 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	points := []rigging.LoadPoint{{ID: "p1", Label: "一号点", RatedLoadKg: 1000}}
	items := []rigging.SuspendedItem{{ID: "i1", Kind: "hoist", Label: "葫芦", SelfWeightKg: 200, WorkingLoadLimitKg: 1000, LoadPointShares: []rigging.LoadShare{{LoadPointID: "p1", BasisPoints: 10000}}}}
	c, err = service.Configure(SetConfiguration{CommandMeta: meta(c.Version, "config", "主管"), CaseID: c.ID, LoadPoints: points, Items: items})
	if err != nil {
		t.Fatal(err)
	}
	configurationVersion := c.Version
	checks := func(passed bool) []rigging.CheckItem {
		out := []rigging.CheckItem{}
		for _, code := range rigging.RequiredChecks {
			out = append(out, rigging.CheckItem{Code: code, Passed: passed})
		}
		return out
	}
	c, err = service.Inspect(SubmitInspection{CommandMeta: meta(c.Version, "operator-1", "操作员甲"), CaseID: c.ID, ID: "in-1", Role: "operator", InspectorName: "操作员甲", ConfigurationVersion: configurationVersion, CheckItems: checks(false), Findings: []FindingInput{{ID: "f-1", Severity: "blocking", Description: "连接销未锁定"}}})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.Inspect(SubmitInspection{CommandMeta: meta(c.Version, "reviewer", "复核员乙"), CaseID: c.ID, ID: "in-2", Role: "reviewer", InspectorName: "复核员乙", ConfigurationVersion: configurationVersion, CheckItems: checks(true)})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.Remediate(RemediateFinding{CommandMeta: meta(c.Version, "remediate", "操作员甲"), CaseID: c.ID, FindingID: "f-1", Note: "已安装锁销并复核扭矩", EvidenceDigest: "sha256:evidence-123"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.CloseFinding(CloseFinding{CommandMeta: meta(c.Version, "bad-close", "操作员甲"), CaseID: c.ID, FindingID: "f-1", Reviewer: "操作员甲"}); err == nil {
		t.Fatal("原发现者不应关闭问题")
	}
	c, err = service.ReviewFinding(ReviewFinding{CommandMeta: meta(c.Version, "reject", "复核员乙"), CaseID: c.ID, FindingID: "f-1", Reviewer: "复核员乙", Round: 1, Decision: "rejected", RejectionReason: "证据未显示锁销防脱状态"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Findings[0].Status != "open" || c.Findings[0].RemediationRounds[0].Decision != "rejected" {
		t.Fatal("驳回后问题应回到待整改并保留首轮决定")
	}
	c, err = service.Remediate(RemediateFinding{CommandMeta: meta(c.Version, "remediate-2", "操作员甲"), CaseID: c.ID, FindingID: "f-1", Note: "补拍防脱锁销并复核扭矩", EvidenceDigest: "sha256:evidence-456"})
	if err != nil {
		t.Fatal(err)
	}
	review := ReviewFinding{CommandMeta: meta(c.Version, "approve-2", "复核员乙"), CaseID: c.ID, FindingID: "f-1", Reviewer: "复核员乙", Round: 2, Decision: "approved"}
	c, err = service.ReviewFinding(review)
	if err != nil {
		t.Fatal(err)
	}
	approvedVersion := c.Version
	c, err = service.ReviewFinding(review)
	if err != nil || c.Version != approvedVersion {
		t.Fatalf("相同幂等键重放不应重复复核事件：%v", err)
	}
	c, err = service.Inspect(SubmitInspection{CommandMeta: meta(c.Version, "operator-2", "操作员甲"), CaseID: c.ID, ID: "in-3", Role: "operator", InspectorName: "操作员甲", ConfigurationVersion: configurationVersion, CheckItems: checks(true)})
	if err != nil {
		t.Fatal(err)
	}
	criteria := []rigging.TrialStageCriterion{}
	for _, stage := range rigging.RequiredTrialStages {
		criteria = append(criteria, rigging.TrialStageCriterion{Stage: stage, MinDurationSec: 20, MaxDeflectionMM: 10})
	}
	c, err = service.ConfirmTrialStandard(ConfirmTrialStandard{CommandMeta: meta(c.Version, "standard", "主管"), CaseID: c.ID, Stages: criteria, AllowedReboundMM: 5, MaxTotalDurationSec: 1800})
	if err != nil {
		t.Fatal(err)
	}
	started := now.Add(-2 * time.Minute)
	stages := []rigging.StageObservation{}
	for index, stage := range rigging.RequiredTrialStages {
		completedAt := started.Add(time.Duration(index+1) * 30 * time.Second)
		stages = append(stages, rigging.StageObservation{Stage: stage, DurationSec: 30, Stable: true, CompletedAt: &completedAt})
	}
	c, err = service.RecordTrial(RecordTrial{CommandMeta: meta(c.Version, "trial", "操作员甲"), CaseID: c.ID, OperatorName: "操作员甲", StartedAt: started, DeadlineAt: started.Add(20 * time.Minute), Stages: stages})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.Freeze(FreezeManifest{CommandMeta: meta(c.Version, "freeze", "主管"), CaseID: c.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Configure(SetConfiguration{CommandMeta: meta(c.Version, "late-config", "主管"), CaseID: c.ID, LoadPoints: points, Items: items}); err == nil {
		t.Fatal("冻结后不应修改配置")
	}
	c, err = service.Issue(IssueCredential{CommandMeta: meta(c.Version, "issue", "主管"), CaseID: c.ID, IssuedBy: "主管"})
	if err != nil {
		t.Fatal(err)
	}
	verified, err := service.Verify(c.Credential.Number)
	if err != nil || !verified.Valid {
		t.Fatalf("凭据核验失败：%v", err)
	}
	reopened, err := eventstore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := reopened.Get(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Status != rigging.StatusReleased || recovered.Version != c.Version {
		t.Fatalf("恢复结果错误：%s v%d", recovered.Status, recovered.Version)
	}
}

func TestIdempotencyReplayAndConflict(t *testing.T) {
	store, err := eventstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := New(store)
	cmd := CreateCase{CommandMeta: CommandMeta{ExpectedVersion: 0, IdempotencyKey: "same", Actor: "主管"}, ID: "case-a", Title: "A", Venue: "V", ManagerName: "M", PerformanceAt: time.Now().Add(time.Hour)}
	first, err := service.Create(cmd)
	if err != nil {
		t.Fatal(err)
	}
	again, err := service.Create(cmd)
	if err != nil || again.Version != first.Version {
		t.Fatalf("幂等重放失败：%v", err)
	}
	cmd.Title = "B"
	if _, err = service.Create(cmd); err == nil {
		t.Fatal("同键不同请求应冲突")
	}
}
