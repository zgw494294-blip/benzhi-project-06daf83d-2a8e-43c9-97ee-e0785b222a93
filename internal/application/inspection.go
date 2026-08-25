package application

import (
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/rigging"
)

func (s *Service) Inspect(cmd SubmitInspection) (*rigging.ClearanceCase, error) {
	if err := cmd.CommandMeta.validate(); err != nil {
		return nil, err
	}
	request := cmd
	if prior, found, err := s.replay(cmd.IdempotencyKey, request); err != nil || found {
		return prior, err
	}
	c, err := s.load(cmd.CaseID)
	if err != nil {
		return nil, err
	}
	if cmd.ID == "" {
		cmd.ID = newID("inspection")
	}
	record := rigging.InspectionRecord{ID: cmd.ID, CaseID: c.ID, Role: cmd.Role, InspectorName: clean(cmd.InspectorName), ConfigurationVersion: cmd.ConfigurationVersion, CheckItems: cmd.CheckItems, SubmittedAt: s.now().UTC()}
	if err = rigging.ValidateInspection(c, record); err != nil {
		return nil, normalize(err)
	}
	inspectionEvent, err := rigging.NewEvent(rigging.EventInspectionSubmitted, c.ID, cmd.Actor, s.now(), rigging.InspectionData{Inspection: record})
	if err != nil {
		return nil, normalize(err)
	}
	events := []rigging.Event{inspectionEvent}
	working, _ := rigging.Clone(c)
	_ = rigging.Apply(working, inspectionEvent)
	for _, input := range cmd.Findings {
		if input.ID == "" {
			input.ID = newID("finding")
		}
		finding := rigging.SafetyFinding{ID: input.ID, CaseID: c.ID, InspectionID: record.ID, Severity: input.Severity, Description: clean(input.Description), Status: "open"}
		if err = rigging.ValidateFinding(working, finding); err != nil {
			return nil, normalize(err)
		}
		event, e := rigging.NewEvent(rigging.EventFindingAdded, c.ID, cmd.Actor, s.now(), rigging.FindingData{Finding: finding})
		if e != nil {
			return nil, normalize(e)
		}
		events = append(events, event)
		_ = rigging.Apply(working, event)
	}
	return s.commit(cmd.CommandMeta, events, request)
}

func (s *Service) Remediate(cmd RemediateFinding) (*rigging.ClearanceCase, error) {
	if err := cmd.CommandMeta.validate(); err != nil {
		return nil, err
	}
	if prior, found, err := s.replay(cmd.IdempotencyKey, cmd); err != nil || found {
		return prior, err
	}
	c, err := s.load(cmd.CaseID)
	if err != nil {
		return nil, err
	}
	if err = rigging.RequireMutable(c); err != nil {
		return nil, normalize(err)
	}
	if err = rigging.ValidateRemediation(cmd.Note, cmd.EvidenceDigest); err != nil {
		return nil, normalize(err)
	}
	found := false
	for _, f := range c.Findings {
		if f.ID == cmd.FindingID {
			found = true
			if f.Status == "closed" {
				return nil, normalize(rigging.Rule("FINDING_CLOSED", "问题已经关闭"))
			}
			if f.Status == "remediated" {
				return nil, normalize(rigging.Rule("REMEDIATION_PENDING_REVIEW", "最新整改轮次正在等待复核"))
			}
		}
	}
	if !found {
		return nil, normalize(rigging.Rule("FINDING_NOT_FOUND", "问题不存在"))
	}
	now := s.now().UTC()
	roundNumber := 1
	for _, f := range c.Findings {
		if f.ID == cmd.FindingID {
			roundNumber = len(f.RemediationRounds) + 1
		}
	}
	round := rigging.RemediationRound{Round: roundNumber, Remediator: clean(cmd.Actor), Note: clean(cmd.Note), EvidenceDigest: clean(cmd.EvidenceDigest), SubmittedAt: now, Decision: "pending"}
	event, e := rigging.NewEvent(rigging.EventFindingRemediated, c.ID, cmd.Actor, now, rigging.RemediationData{ID: cmd.FindingID, Round: round})
	if e != nil {
		return nil, normalize(e)
	}
	return s.commit(cmd.CommandMeta, []rigging.Event{event}, cmd)
}

func (s *Service) CloseFinding(cmd CloseFinding) (*rigging.ClearanceCase, error) {
	c, err := s.load(cmd.CaseID)
	if err != nil {
		return nil, err
	}
	round := 0
	for _, f := range c.Findings {
		if f.ID == cmd.FindingID {
			round = len(f.RemediationRounds)
		}
	}
	return s.ReviewFinding(ReviewFinding{CommandMeta: cmd.CommandMeta, CaseID: cmd.CaseID, FindingID: cmd.FindingID, Reviewer: cmd.Reviewer, Decision: "approved", Round: round})
}

func (s *Service) ReviewFinding(cmd ReviewFinding) (*rigging.ClearanceCase, error) {
	if err := cmd.CommandMeta.validate(); err != nil {
		return nil, err
	}
	if prior, found, err := s.replay(cmd.IdempotencyKey, cmd); err != nil || found {
		return prior, err
	}
	c, err := s.load(cmd.CaseID)
	if err != nil {
		return nil, err
	}
	if err = rigging.ValidateFindingReview(c, cmd.FindingID, cmd.Round, cmd.Decision, cmd.Reviewer, cmd.RejectionReason); err != nil {
		return nil, normalize(err)
	}
	now := s.now().UTC()
	event, e := rigging.NewEvent(rigging.EventFindingReviewDecided, c.ID, cmd.Actor, now, rigging.FindingReviewData{ID: cmd.FindingID, Round: cmd.Round, Decision: cmd.Decision, Reviewer: clean(cmd.Reviewer), Reason: clean(cmd.RejectionReason), ReviewedAt: now})
	if e != nil {
		return nil, normalize(e)
	}
	return s.commit(cmd.CommandMeta, []rigging.Event{event}, cmd)
}
