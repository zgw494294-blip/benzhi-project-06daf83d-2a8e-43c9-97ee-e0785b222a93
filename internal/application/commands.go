package application

import (
	"time"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/rigging"
)

type CommandMeta struct {
	ExpectedVersion int    `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
	Actor           string `json:"actor"`
}
type CreateCase struct {
	CommandMeta
	ID, Title, Venue, ManagerName string
	PerformanceAt                 time.Time
}
type SetConfiguration struct {
	CommandMeta
	CaseID              string
	LoadPoints          []rigging.LoadPoint     `json:"loadPoints"`
	Items               []rigging.SuspendedItem `json:"items"`
	PreflightDigest     string                  `json:"preflightDigest"`
	ConfirmInvalidation bool                    `json:"confirmInvalidation"`
}
type PreviewConfiguration struct {
	CaseID          string
	ExpectedVersion int
	LoadPoints      []rigging.LoadPoint
	Items           []rigging.SuspendedItem
}
type FindingInput struct{ ID, Severity, Description string }
type SubmitInspection struct {
	CommandMeta
	CaseID, ID, Role, InspectorName string
	ConfigurationVersion            int
	CheckItems                      []rigging.CheckItem
	Findings                        []FindingInput
}
type RemediateFinding struct {
	CommandMeta
	CaseID, FindingID, Note, EvidenceDigest string
}
type CloseFinding struct {
	CommandMeta
	CaseID, FindingID, Reviewer string
}
type ReviewFinding struct {
	CommandMeta
	CaseID, FindingID, Reviewer, Decision, RejectionReason string
	Round                                                  int
}
type ConfirmTrialStandard struct {
	CommandMeta
	CaseID              string
	Stages              []rigging.TrialStageCriterion
	AllowedReboundMM    int64
	MaxTotalDurationSec int
}
type RecordTrial struct {
	CommandMeta
	CaseID, ID, OperatorName string
	StartedAt, DeadlineAt    time.Time
	Stages                   []rigging.StageObservation
	Anomalies                []string
	StandardDigest           string
}
type FreezeManifest struct {
	CommandMeta
	CaseID string
}
type IssueCredential struct {
	CommandMeta
	CaseID, IssuedBy string
}

func (m CommandMeta) validate() error {
	if m.IdempotencyKey == "" {
		return &Error{Code: "IDEMPOTENCY_KEY_REQUIRED", Message: "idempotencyKey 不能为空", Status: 400}
	}
	if m.Actor == "" {
		return &Error{Code: "ACTOR_REQUIRED", Message: "actor 不能为空", Status: 400}
	}
	if m.ExpectedVersion < 0 {
		return &Error{Code: "INVALID_VERSION", Message: "expectedVersion 不能为负数", Status: 400}
	}
	return nil
}
