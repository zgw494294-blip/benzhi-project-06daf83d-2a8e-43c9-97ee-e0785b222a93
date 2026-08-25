package rigging

import (
	"encoding/json"
	"time"
)

const (
	EventCaseCreated            = "CaseCreated"
	EventConfigurationSet       = "ConfigurationSet"
	EventInspectionSubmitted    = "InspectionSubmitted"
	EventFindingAdded           = "FindingAdded"
	EventFindingRemediated      = "FindingRemediated"
	EventFindingClosed          = "FindingClosed"
	EventFindingReviewDecided   = "FindingReviewDecided"
	EventTrialStandardConfirmed = "TrialStandardConfirmed"
	EventTrialRecorded          = "TrialRecorded"
	EventManifestFrozen         = "ManifestFrozen"
	EventCredentialIssued       = "CredentialIssued"
)

type Event struct {
	Type       string          `json:"type"`
	CaseID     string          `json:"caseId"`
	Actor      string          `json:"actor"`
	OccurredAt time.Time       `json:"occurredAt"`
	Data       json.RawMessage `json:"data"`
}

func NewEvent(kind, caseID, actor string, at time.Time, value any) (Event, error) {
	b, err := json.Marshal(value)
	return Event{Type: kind, CaseID: caseID, Actor: actor, OccurredAt: at.UTC(), Data: b}, err
}

type CreatedData struct {
	Title, Venue, ManagerName string
	PerformanceAt             time.Time
}
type ConfigurationData struct {
	LoadPoints           []LoadPoint     `json:"loadPoints"`
	Items                []SuspendedItem `json:"items"`
	NextStatus           Status          `json:"nextStatus"`
	ConfigurationVersion int             `json:"configurationVersion,omitempty"`
}
type InspectionData struct {
	Inspection InspectionRecord `json:"inspection"`
}
type FindingData struct {
	Finding SafetyFinding `json:"finding"`
}
type RemediationData struct {
	ID             string           `json:"id"`
	Round          RemediationRound `json:"round"`
	Note           string           `json:"note,omitempty"`
	EvidenceDigest string           `json:"evidenceDigest,omitempty"`
}
type CloseFindingData struct {
	ID, Reviewer string
	ClosedAt     time.Time
}
type FindingReviewData struct {
	ID         string    `json:"id"`
	Round      int       `json:"round"`
	Decision   string    `json:"decision"`
	Reviewer   string    `json:"reviewer"`
	Reason     string    `json:"reason,omitempty"`
	ReviewedAt time.Time `json:"reviewedAt"`
}
type TrialStandardData struct {
	Standard TrialStandard `json:"standard"`
}
type TrialData struct {
	Trial TrialLift `json:"trial"`
}
type FreezeData struct {
	Digest   string
	FrozenAt time.Time
}
type CredentialData struct {
	Credential ReleaseCredential `json:"credential"`
}
