package rigging

import (
	"encoding/json"
	"fmt"
)

func Apply(c *ClearanceCase, event Event) error {
	switch event.Type {
	case EventCaseCreated:
		var d CreatedData
		if err := json.Unmarshal(event.Data, &d); err != nil {
			return err
		}
		*c = ClearanceCase{ID: event.CaseID, Title: d.Title, Venue: d.Venue, ManagerName: d.ManagerName, PerformanceAt: d.PerformanceAt, Status: StatusDraft, CreatedAt: event.OccurredAt}
	case EventConfigurationSet:
		var d ConfigurationData
		if err := json.Unmarshal(event.Data, &d); err != nil {
			return err
		}
		c.LoadPoints = d.LoadPoints
		c.Items = d.Items
		c.Status = d.NextStatus
		c.Inspections = nil
		c.Findings = nil
		c.TrialLifts = nil
		c.TrialStandard = nil
		if d.ConfigurationVersion > 0 {
			c.ConfigurationVersion = d.ConfigurationVersion
		} else {
			c.ConfigurationVersion = c.Version + 1
		}
	case EventInspectionSubmitted:
		var d InspectionData
		if err := json.Unmarshal(event.Data, &d); err != nil {
			return err
		}
		c.Inspections = append(c.Inspections, d.Inspection)
		if HasBothPassingInspections(c) && !HasOpenBlockingFindings(c) {
			c.Status = StatusTrialReady
		}
	case EventFindingAdded:
		var d FindingData
		if err := json.Unmarshal(event.Data, &d); err != nil {
			return err
		}
		c.Findings = append(c.Findings, d.Finding)
		c.Status = StatusRemediation
	case EventFindingRemediated:
		var d RemediationData
		if err := json.Unmarshal(event.Data, &d); err != nil {
			return err
		}
		for i := range c.Findings {
			if c.Findings[i].ID == d.ID {
				if d.Round.Round == 0 {
					d.Round = RemediationRound{Round: len(c.Findings[i].RemediationRounds) + 1, Remediator: event.Actor, Note: d.Note, EvidenceDigest: d.EvidenceDigest, SubmittedAt: event.OccurredAt, Decision: "pending"}
				}
				c.Findings[i].RemediationNote = d.Round.Note
				c.Findings[i].EvidenceDigest = d.Round.EvidenceDigest
				c.Findings[i].Status = "remediated"
				c.Findings[i].RemediationRounds = append(c.Findings[i].RemediationRounds, d.Round)
			}
		}
	case EventFindingClosed:
		var d CloseFindingData
		if err := json.Unmarshal(event.Data, &d); err != nil {
			return err
		}
		for i := range c.Findings {
			if c.Findings[i].ID == d.ID {
				c.Findings[i].Status = "closed"
				c.Findings[i].ClosedBy = d.Reviewer
				c.Findings[i].ClosedAt = &d.ClosedAt
			}
		}
		if HasBothPassingInspections(c) && !HasOpenBlockingFindings(c) {
			c.Status = StatusTrialReady
		}
	case EventFindingReviewDecided:
		var d FindingReviewData
		if err := json.Unmarshal(event.Data, &d); err != nil {
			return err
		}
		for i := range c.Findings {
			if c.Findings[i].ID != d.ID {
				continue
			}
			rounds := c.Findings[i].RemediationRounds
			if len(rounds) > 0 && rounds[len(rounds)-1].Round == d.Round {
				r := &c.Findings[i].RemediationRounds[len(rounds)-1]
				r.Decision, r.Reviewer, r.RejectionReason = d.Decision, d.Reviewer, d.Reason
				r.ReviewedAt = &d.ReviewedAt
			}
			if d.Decision == "approved" {
				c.Findings[i].Status, c.Findings[i].ClosedBy, c.Findings[i].ClosedAt = "closed", d.Reviewer, &d.ReviewedAt
			} else {
				c.Findings[i].Status = "open"
			}
		}
		if HasBothPassingInspections(c) && !HasOpenBlockingFindings(c) {
			c.Status = StatusTrialReady
		} else {
			c.Status = StatusRemediation
		}
	case EventTrialStandardConfirmed:
		var d TrialStandardData
		if err := json.Unmarshal(event.Data, &d); err != nil {
			return err
		}
		c.TrialStandard = &d.Standard
	case EventTrialRecorded:
		var d TrialData
		if err := json.Unmarshal(event.Data, &d); err != nil {
			return err
		}
		c.TrialLifts = append(c.TrialLifts, d.Trial)
		if d.Trial.Result == "passed" {
			c.Status = StatusFreezeReady
		} else {
			c.Status = StatusRemediation
		}
	case EventManifestFrozen:
		var d FreezeData
		if err := json.Unmarshal(event.Data, &d); err != nil {
			return err
		}
		c.ManifestDigest = d.Digest
		c.FrozenAt = &d.FrozenAt
		c.Status = StatusFrozen
	case EventCredentialIssued:
		var d CredentialData
		if err := json.Unmarshal(event.Data, &d); err != nil {
			return err
		}
		c.Credential = &d.Credential
		c.Status = StatusReleased
	default:
		return fmt.Errorf("unknown event type %q", event.Type)
	}
	c.Version++
	return nil
}

func Clone(c *ClearanceCase) (*ClearanceCase, error) {
	b, e := json.Marshal(c)
	if e != nil {
		return nil, e
	}
	var out ClearanceCase
	e = json.Unmarshal(b, &out)
	return &out, e
}
