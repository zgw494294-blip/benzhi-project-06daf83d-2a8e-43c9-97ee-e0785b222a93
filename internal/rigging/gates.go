package rigging

import "strings"

type GateCheck struct {
	Code           string `json:"code"`
	Label          string `json:"label"`
	Passed         bool   `json:"passed"`
	BlockingReason string `json:"blockingReason,omitempty"`
}

type GateReport struct {
	Ready  bool        `json:"ready"`
	Checks []GateCheck `json:"checks"`
}

func EvaluateReleaseGates(c *ClearanceCase) GateReport {
	report := GateReport{Ready: true}
	add := func(code, label string, passed bool, reason string) {
		check := GateCheck{Code: code, Label: label, Passed: passed}
		if !passed {
			check.BlockingReason = reason
			report.Ready = false
		}
		report.Checks = append(report.Checks, check)
	}
	_, loadErr := CalculateLoads(c.LoadPoints, c.Items)
	add("load_configuration", "载荷配置有效且未超限", loadErr == nil, errorText(loadErr, "尚未完成有效载荷配置"))
	operator, reviewer := latestPassingInspectors(c)
	add("operator_inspection", "操作员检查完整并通过", operator != "", "缺少通过的操作员检查")
	add("independent_review", "独立复核完整并通过", reviewer != "" && !samePerson(operator, reviewer), "缺少身份独立的通过复核")
	add("findings_closed", "阻断与重大问题均已关闭", !HasOpenBlockingFindings(c), "仍有未关闭的阻断或重大问题")
	trialPassed := false
	if len(c.TrialLifts) > 0 {
		trialPassed = c.TrialLifts[len(c.TrialLifts)-1].Result == "passed"
	}
	add("trial_lift", "最近一次分阶段试吊通过", trialPassed, "尚无通过的完整试吊记录")
	if c.Status == StatusFrozen || c.Status == StatusReleased {
		add("manifest_frozen", "清单已冻结并生成摘要", c.ManifestDigest != "", "冻结摘要缺失")
	}
	return report
}

func latestPassingInspectors(c *ClearanceCase) (string, string) {
	operator, reviewer := "", ""
	for _, record := range c.Inspections {
		passed := len(record.CheckItems) > 0
		for _, item := range record.CheckItems {
			if !item.Passed {
				passed = false
				break
			}
		}
		if !passed {
			continue
		}
		if record.Role == "operator" {
			operator = record.InspectorName
		}
		if record.Role == "reviewer" {
			reviewer = record.InspectorName
		}
	}
	return operator, reviewer
}

func samePerson(left, right string) bool {
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}

func errorText(err error, fallback string) string {
	if err == nil {
		return ""
	}
	if err.Error() == "" {
		return fallback
	}
	return err.Error()
}
