package rigging

import "strings"

func ValidateFinding(c *ClearanceCase, finding SafetyFinding) error {
	if err := RequireMutable(c); err != nil {
		return err
	}
	if finding.ID == "" || strings.TrimSpace(finding.Description) == "" {
		return Rule("INVALID_FINDING", "问题标识和描述不能为空")
	}
	if finding.Severity != "blocking" && finding.Severity != "major" && finding.Severity != "minor" {
		return Rule("INVALID_SEVERITY", "问题严重级别无效")
	}
	for _, inspection := range c.Inspections {
		if inspection.ID == finding.InspectionID {
			return nil
		}
	}
	return Rule("INSPECTION_NOT_FOUND", "问题必须关联现有检查记录")
}

func FindingReporter(c *ClearanceCase, finding SafetyFinding) string {
	if strings.TrimSpace(finding.ReportedBy) != "" {
		return finding.ReportedBy
	}
	for _, inspection := range c.Inspections {
		if inspection.ID == finding.InspectionID {
			return inspection.InspectorName
		}
	}
	return ""
}

func ValidateRemediation(note, digest string) error {
	if strings.TrimSpace(note) == "" || len(strings.TrimSpace(digest)) < 8 {
		return Rule("REMEDIATION_EVIDENCE_REQUIRED", "整改说明和不少于 8 字符的证据摘要均为必填")
	}
	return nil
}

func ValidateFindingClose(c *ClearanceCase, id, reviewer string) error {
	return ValidateFindingReview(c, id, latestRound(c, id), "approved", reviewer, "")
}

func latestRound(c *ClearanceCase, id string) int {
	for _, finding := range c.Findings {
		if finding.ID == id {
			return len(finding.RemediationRounds)
		}
	}
	return 0
}

func ValidateFindingReview(c *ClearanceCase, id string, round int, decision, reviewer, reason string) error {
	for _, finding := range c.Findings {
		if finding.ID != id {
			continue
		}
		if finding.Status == "closed" {
			return Rule("FINDING_CLOSED", "已关闭问题不能再次复核")
		}
		if finding.Status != "remediated" {
			return Rule("FINDING_NOT_REMEDIATED", "问题尚未提交整改证据")
		}
		if len(finding.RemediationRounds) == 0 || round != finding.RemediationRounds[len(finding.RemediationRounds)-1].Round {
			return Rule("REMEDIATION_ROUND_CONFLICT", "只能复核最新整改轮次")
		}
		if finding.RemediationRounds[len(finding.RemediationRounds)-1].Decision != "pending" {
			return Rule("REMEDIATION_ALREADY_REVIEWED", "最新整改轮次已经复核")
		}
		if strings.EqualFold(strings.TrimSpace(FindingReporter(c, finding)), strings.TrimSpace(reviewer)) {
			return Rule("CLOSER_NOT_INDEPENDENT", "原发现者不能确认关闭该问题")
		}
		if strings.TrimSpace(reviewer) == "" {
			return Rule("REVIEWER_REQUIRED", "关闭复核人不能为空")
		}
		if decision != "approved" && decision != "rejected" {
			return Rule("INVALID_REVIEW_DECISION", "复核决定必须为 approved 或 rejected")
		}
		if decision == "rejected" && strings.TrimSpace(reason) == "" {
			return Rule("REJECTION_REASON_REQUIRED", "驳回整改时必须填写原因")
		}
		return nil
	}
	return Rule("FINDING_NOT_FOUND", "问题不存在")
}

func HasOpenBlockingFindings(c *ClearanceCase) bool {
	for _, f := range c.Findings {
		if (f.Severity == "blocking" || f.Severity == "major") && f.Status != "closed" {
			return true
		}
	}
	return false
}
