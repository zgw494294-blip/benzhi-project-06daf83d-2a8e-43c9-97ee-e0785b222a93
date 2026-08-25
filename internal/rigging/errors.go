package rigging

import "fmt"

type RuleError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *RuleError) Error() string { return e.Message }

func Rule(code, format string, args ...any) error {
	return &RuleError{Code: code, Message: fmt.Sprintf(format, args...)}
}

func IsImmutable(c *ClearanceCase) bool {
	return c.Status == StatusFrozen || c.Status == StatusReleased
}

func RequireMutable(c *ClearanceCase) error {
	if IsImmutable(c) {
		return Rule("CASE_FROZEN", "档案已冻结，业务数据不可修改")
	}
	return nil
}
