package rigging

import "time"

type Status string

const (
	StatusDraft       Status = "draft"
	StatusInspection  Status = "inspection"
	StatusRemediation Status = "remediation"
	StatusTrialReady  Status = "trial_ready"
	StatusFreezeReady Status = "freeze_ready"
	StatusFrozen      Status = "frozen"
	StatusReleased    Status = "released"
)

type ClearanceCase struct {
	ID                   string             `json:"id"`
	Title                string             `json:"title"`
	Venue                string             `json:"venue"`
	PerformanceAt        time.Time          `json:"performanceAt"`
	ManagerName          string             `json:"managerName"`
	Status               Status             `json:"status"`
	Version              int                `json:"version"`
	CreatedAt            time.Time          `json:"createdAt"`
	FrozenAt             *time.Time         `json:"frozenAt,omitempty"`
	LoadPoints           []LoadPoint        `json:"loadPoints"`
	Items                []SuspendedItem    `json:"items"`
	Inspections          []InspectionRecord `json:"inspections"`
	Findings             []SafetyFinding    `json:"findings"`
	TrialLifts           []TrialLift        `json:"trialLifts"`
	Credential           *ReleaseCredential `json:"credential,omitempty"`
	ManifestDigest       string             `json:"manifestDigest,omitempty"`
	ConfigurationVersion int                `json:"configurationVersion,omitempty"`
	TrialStandard        *TrialStandard     `json:"trialStandard,omitempty"`
}

type LoadPoint struct {
	ID                     string `json:"id"`
	CaseID                 string `json:"caseId"`
	Label                  string `json:"label"`
	Position               string `json:"position"`
	RatedLoadKg            int64  `json:"ratedLoadKg"`
	AllocatedLoadKg        int64  `json:"allocatedLoadKg"`
	UtilizationBasisPoints int64  `json:"utilizationBasisPoints"`
}

type LoadShare struct {
	LoadPointID string `json:"loadPointId"`
	BasisPoints int64  `json:"basisPoints"`
}

type SuspendedItem struct {
	ID                 string      `json:"id"`
	CaseID             string      `json:"caseId"`
	Kind               string      `json:"kind"`
	Label              string      `json:"label"`
	SerialNumber       string      `json:"serialNumber"`
	SelfWeightKg       int64       `json:"selfWeightKg"`
	WorkingLoadLimitKg int64       `json:"workingLoadLimitKg"`
	LoadPointShares    []LoadShare `json:"loadPointShares"`
}

type CheckItem struct {
	Code   string `json:"code"`
	Passed bool   `json:"passed"`
	Note   string `json:"note,omitempty"`
}

type InspectionRecord struct {
	ID                   string      `json:"id"`
	CaseID               string      `json:"caseId"`
	Role                 string      `json:"role"`
	InspectorName        string      `json:"inspectorName"`
	ConfigurationVersion int         `json:"configurationVersion"`
	CheckItems           []CheckItem `json:"checkItems"`
	SubmittedAt          time.Time   `json:"submittedAt"`
}

type SafetyFinding struct {
	ID                string             `json:"id"`
	CaseID            string             `json:"caseId"`
	InspectionID      string             `json:"inspectionId"`
	Severity          string             `json:"severity"`
	Description       string             `json:"description"`
	Status            string             `json:"status"`
	RemediationNote   string             `json:"remediationNote,omitempty"`
	EvidenceDigest    string             `json:"evidenceDigest,omitempty"`
	ClosedBy          string             `json:"closedBy,omitempty"`
	ClosedAt          *time.Time         `json:"closedAt,omitempty"`
	ReportedBy        string             `json:"reportedBy,omitempty"`
	LinkedTrialID     string             `json:"linkedTrialId,omitempty"`
	FailedStage       string             `json:"failedStage,omitempty"`
	RemediationRounds []RemediationRound `json:"remediationRounds,omitempty"`
}

type RemediationRound struct {
	Round           int        `json:"round"`
	Remediator      string     `json:"remediator"`
	Note            string     `json:"note"`
	EvidenceDigest  string     `json:"evidenceDigest"`
	SubmittedAt     time.Time  `json:"submittedAt"`
	Decision        string     `json:"decision"`
	Reviewer        string     `json:"reviewer,omitempty"`
	ReviewedAt      *time.Time `json:"reviewedAt,omitempty"`
	RejectionReason string     `json:"rejectionReason,omitempty"`
}

type StageObservation struct {
	Stage        string     `json:"stage"`
	DurationSec  int        `json:"durationSec"`
	DeflectionMM int64      `json:"deflectionMm"`
	Stable       bool       `json:"stable"`
	Note         string     `json:"note,omitempty"`
	CompletedAt  *time.Time `json:"completedAt,omitempty"`
}

type TrialStageCriterion struct {
	Stage           string `json:"stage"`
	MinDurationSec  int    `json:"minDurationSec"`
	MaxDeflectionMM int64  `json:"maxDeflectionMm"`
}

type TrialStandard struct {
	Digest               string                `json:"digest"`
	ConfigurationVersion int                   `json:"configurationVersion"`
	Stages               []TrialStageCriterion `json:"stages"`
	AllowedReboundMM     int64                 `json:"allowedReboundMm"`
	MaxTotalDurationSec  int                   `json:"maxTotalDurationSec"`
	ConfirmedBy          string                `json:"confirmedBy"`
	ConfirmedAt          time.Time             `json:"confirmedAt"`
}

type TrialLift struct {
	ID                   string             `json:"id"`
	CaseID               string             `json:"caseId"`
	StartedAt            time.Time          `json:"startedAt"`
	DeadlineAt           time.Time          `json:"deadlineAt"`
	OperatorName         string             `json:"operatorName"`
	StageObservations    []StageObservation `json:"stageObservations"`
	Anomalies            []string           `json:"anomalies"`
	Result               string             `json:"result"`
	CompletedAt          time.Time          `json:"completedAt"`
	StandardDigest       string             `json:"standardDigest,omitempty"`
	ConfigurationVersion int                `json:"configurationVersion,omitempty"`
	FailureReasons       []TrialFailure     `json:"failureReasons,omitempty"`
}

type TrialFailure struct {
	Code      string `json:"code"`
	Stage     string `json:"stage,omitempty"`
	Reason    string `json:"reason"`
	Actual    int64  `json:"actual,omitempty"`
	Threshold int64  `json:"threshold,omitempty"`
}

type ReleaseCredential struct {
	Number         string    `json:"number"`
	CaseID         string    `json:"caseId"`
	Sequence       int       `json:"sequence"`
	FrozenVersion  int       `json:"frozenVersion"`
	ManifestDigest string    `json:"manifestDigest"`
	PerformanceAt  time.Time `json:"performanceAt"`
	IssuedBy       string    `json:"issuedBy"`
	IssuedAt       time.Time `json:"issuedAt"`
}

type AuditEvent struct {
	Sequence       uint64    `json:"sequence"`
	CaseID         string    `json:"caseId"`
	EventType      string    `json:"eventType"`
	Actor          string    `json:"actor"`
	OccurredAt     time.Time `json:"occurredAt"`
	IdempotencyKey string    `json:"idempotencyKey"`
	PayloadDigest  string    `json:"payloadDigest"`
	PreviousDigest string    `json:"previousDigest"`
}
