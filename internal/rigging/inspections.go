package rigging

import "strings"

var RequiredChecks = []string{"hardware", "connection", "routing", "capacity", "clearance"}

func ValidateInspection(c *ClearanceCase, record InspectionRecord) error {
	if err := RequireMutable(c); err != nil {
		return err
	}
	if c.Status != StatusInspection && c.Status != StatusRemediation {
		return Rule("INVALID_STATE", "当前状态不能提交检查")
	}
	if record.Role != "operator" && record.Role != "reviewer" {
		return Rule("INVALID_INSPECTION_ROLE", "检查角色必须为 operator 或 reviewer")
	}
	if strings.TrimSpace(record.InspectorName) == "" {
		return Rule("INSPECTOR_REQUIRED", "检查者姓名不能为空")
	}
	configurationVersion := c.Version
	if len(c.Inspections) > 0 {
		configurationVersion = c.Inspections[0].ConfigurationVersion
	}
	if record.ConfigurationVersion != configurationVersion {
		return Rule("CONFIGURATION_CHANGED", "检查所依据的配置版本不是当前版本")
	}
	seen := map[string]bool{}
	for _, item := range record.CheckItems {
		seen[item.Code] = true
	}
	for _, code := range RequiredChecks {
		if !seen[code] {
			return Rule("CHECKLIST_INCOMPLETE", "缺少必检项 %s", code)
		}
	}
	for _, prior := range c.Inspections {
		if prior.Role != record.Role && strings.EqualFold(strings.TrimSpace(prior.InspectorName), strings.TrimSpace(record.InspectorName)) {
			return Rule("INSPECTOR_NOT_INDEPENDENT", "操作员与独立复核员必须由不同人员担任")
		}
	}
	return nil
}

func HasBothPassingInspections(c *ClearanceCase) bool {
	roles := map[string]bool{}
	for _, inspection := range c.Inspections {
		passed := true
		for _, item := range inspection.CheckItems {
			if !item.Passed {
				passed = false
			}
		}
		if passed {
			roles[inspection.Role] = true
		}
	}
	return roles["operator"] && roles["reviewer"]
}
