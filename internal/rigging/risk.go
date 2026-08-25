package rigging

import (
	"math"
	"time"
)

type RiskLevel string

const (
	RiskReleased RiskLevel = "released"
	RiskNormal   RiskLevel = "normal"
	RiskNear     RiskLevel = "near"
	RiskUrgent   RiskLevel = "urgent"
	RiskOverdue  RiskLevel = "overdue"
)

type RiskSummary struct {
	Level                  RiskLevel `json:"level"`
	RemainingHours         float64   `json:"remainingHours"`
	PendingRelease         bool      `json:"pendingRelease"`
	OpenBlockingCount      int       `json:"openBlockingCount"`
	OpenMajorCount         int       `json:"openMajorCount"`
	MissingInspectionRoles []string  `json:"missingInspectionRoles"`
	LatestTrialResult      string    `json:"latestTrialResult,omitempty"`
	NextTask               string    `json:"nextTask"`
	BlockingReasons        []string  `json:"blockingReasons"`
	CredentialNumber       string    `json:"credentialNumber,omitempty"`
}

func BuildRiskSummary(c *ClearanceCase, now time.Time) (RiskSummary, error) {
	if c.PerformanceAt.IsZero() || c.PerformanceAt.Year() < 2000 || c.PerformanceAt.Year() > 2200 {
		return RiskSummary{}, Rule("INVALID_PERFORMANCE_TIME", "档案演出时间无效")
	}
	hours := c.PerformanceAt.Sub(now).Hours()
	result := RiskSummary{RemainingHours: math.Round(hours*100) / 100, PendingRelease: c.Status != StatusReleased}
	if c.Credential != nil {
		result.CredentialNumber = c.Credential.Number
	}
	for _, f := range c.Findings {
		if f.Status == "closed" {
			continue
		}
		if f.Severity == "blocking" {
			result.OpenBlockingCount++
		}
		if f.Severity == "major" {
			result.OpenMajorCount++
		}
	}
	op, reviewer := latestPassingInspectors(c)
	if op == "" {
		result.MissingInspectionRoles = append(result.MissingInspectionRoles, "operator")
	}
	if reviewer == "" {
		result.MissingInspectionRoles = append(result.MissingInspectionRoles, "reviewer")
	}
	if len(c.TrialLifts) > 0 {
		result.LatestTrialResult = c.TrialLifts[len(c.TrialLifts)-1].Result
	}
	if c.Status == StatusReleased {
		result.Level, result.NextTask = RiskReleased, "核验放行凭据"
		return result, nil
	}
	if hours < 0 {
		result.Level = RiskOverdue
	} else if hours <= 12 {
		result.Level = RiskUrgent
	} else if hours <= 48 {
		result.Level = RiskNear
	} else {
		result.Level = RiskNormal
	}
	report := EvaluateReleaseGates(c)
	for _, check := range report.Checks {
		if !check.Passed {
			result.BlockingReasons = append(result.BlockingReasons, check.BlockingReason)
		}
	}
	result.NextTask = nextRiskTask(c)
	return result, nil
}

func nextRiskTask(c *ClearanceCase) string {
	switch c.Status {
	case StatusDraft:
		return "登记并预检载荷配置"
	case StatusInspection:
		return "完成双人检查"
	case StatusRemediation:
		return "完成问题整改与独立复核"
	case StatusTrialReady:
		if c.TrialStandard == nil {
			return "主管确认试吊判定标准"
		}
		return "执行全阶段试吊"
	case StatusFreezeReady:
		return "冻结当前清单"
	case StatusFrozen:
		return "签发放行凭据"
	default:
		return "核验放行凭据"
	}
}
