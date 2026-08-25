package application

import (
	"errors"
	"fmt"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/eventstore"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/rigging"
)

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
	Details any    `json:"details,omitempty"`
}

func (e *Error) Error() string { return e.Message }

func normalize(err error) error {
	if err == nil {
		return nil
	}
	var rule *rigging.RuleError
	if errors.As(err, &rule) {
		return &Error{Code: rule.Code, Message: rule.Message, Status: 422}
	}
	var version *eventstore.VersionConflict
	if errors.As(err, &version) {
		return &Error{Code: "VERSION_CONFLICT", Message: version.Error(), Status: 409, Details: map[string]int{"expected": version.Expected, "actual": version.Actual}}
	}
	var idem *eventstore.IdempotencyConflict
	if errors.As(err, &idem) {
		return &Error{Code: "IDEMPOTENCY_CONFLICT", Message: idem.Error(), Status: 409}
	}
	if errors.Is(err, eventstore.ErrNotFound) {
		return &Error{Code: "NOT_FOUND", Message: "档案或凭据不存在", Status: 404}
	}
	return &Error{Code: "INTERNAL_ERROR", Message: fmt.Sprintf("内部处理失败：%v", err), Status: 500}
}
