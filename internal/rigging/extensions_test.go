package rigging

import (
	"testing"
	"time"
)

func TestConfigurationPreflightReportsCapacityAndInvalidation(t *testing.T) {
	c := &ClearanceCase{ID: "case-1", Version: 5,
		LoadPoints:  []LoadPoint{{ID: "main", CaseID: "case-1", Label: "主吊点", RatedLoadKg: 1000, AllocatedLoadKg: 600, UtilizationBasisPoints: 6000}},
		Items:       []SuspendedItem{{ID: "old", CaseID: "case-1", Kind: "scenery", Label: "旧景片", SelfWeightKg: 600, WorkingLoadLimitKg: 1000, LoadPointShares: []LoadShare{{LoadPointID: "main", BasisPoints: 10000}}}},
		Inspections: []InspectionRecord{{ID: "operator"}, {ID: "reviewer"}},
	}
	points := []LoadPoint{{ID: "main", CaseID: c.ID, Label: "主吊点", RatedLoadKg: 1000}}
	items := []SuspendedItem{{ID: "new", CaseID: c.ID, Kind: "scenery", Label: "新景片", SelfWeightKg: 850, WorkingLoadLimitKg: 1000, LoadPointShares: []LoadShare{{LoadPointID: "main", BasisPoints: 10000}}}}
	preview, err := BuildConfigurationPreflight(c, c.Version, points, items)
	if err != nil {
		t.Fatal(err)
	}
	if preview.LoadSummary.Points[0].UtilizationBasisPoints != 8500 || preview.Diff.LoadPoints[0].AfterRemainingKg != 150 {
		t.Fatalf("预检承载差异错误：%+v", preview.Diff.LoadPoints)
	}
	if !preview.RequiresConfirmation || len(preview.InvalidatedResults) != 1 || preview.InvalidatedResults[0].Count != 2 {
		t.Fatalf("未报告两份检查失效：%+v", preview.InvalidatedResults)
	}
}

func TestTrialThresholdFailureAndRiskSummary(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	c := &ClearanceCase{ID: "case-1", Status: StatusTrialReady, ConfigurationVersion: 2, PerformanceAt: now.Add(10 * time.Hour),
		LoadPoints:  []LoadPoint{{ID: "main", Label: "主吊点", RatedLoadKg: 1000}},
		Items:       []SuspendedItem{{ID: "item", Kind: "hoist", Label: "葫芦", SelfWeightKg: 100, WorkingLoadLimitKg: 1000, LoadPointShares: []LoadShare{{LoadPointID: "main", BasisPoints: 10000}}}},
		Inspections: []InspectionRecord{{Role: "operator", InspectorName: "甲", CheckItems: []CheckItem{{Code: "hardware", Passed: true}}}, {Role: "reviewer", InspectorName: "乙", CheckItems: []CheckItem{{Code: "hardware", Passed: true}}}},
	}
	c.TrialStandard = &TrialStandard{Digest: "standard", ConfigurationVersion: 2, AllowedReboundMM: 3, MaxTotalDurationSec: 600}
	for _, stage := range RequiredTrialStages {
		c.TrialStandard.Stages = append(c.TrialStandard.Stages, TrialStageCriterion{Stage: stage, MinDurationSec: 20, MaxDeflectionMM: 10})
	}
	started := now.Add(-2 * time.Minute)
	observations := []StageObservation{}
	for index, stage := range RequiredTrialStages {
		completed := started.Add(time.Duration(index+1) * 30 * time.Second)
		deflection := int64(2)
		if stage == "full" {
			deflection = 15
		}
		observations = append(observations, StageObservation{Stage: stage, DurationSec: 30, DeflectionMM: deflection, Stable: true, CompletedAt: &completed})
	}
	trial := TrialLift{StartedAt: started, DeadlineAt: started.Add(10 * time.Minute), StageObservations: observations, StandardDigest: "standard", ConfigurationVersion: 2}
	if err := ValidateTrial(c, trial, now); err != nil {
		t.Fatal(err)
	}
	failures := EvaluateTrial(c, trial, now)
	if len(failures) == 0 || failures[0].Code != "DEFLECTION_EXCEEDED" || failures[0].Stage != "full" {
		t.Fatalf("未按满载挠度阈值判定失败：%+v", failures)
	}
	c.Findings = []SafetyFinding{{Severity: "major", Status: "open"}}
	risk, err := BuildRiskSummary(c, now)
	if err != nil {
		t.Fatal(err)
	}
	if risk.Level != RiskUrgent || risk.OpenMajorCount != 1 || !risk.PendingRelease {
		t.Fatalf("风险摘要错误：%+v", risk)
	}
}
