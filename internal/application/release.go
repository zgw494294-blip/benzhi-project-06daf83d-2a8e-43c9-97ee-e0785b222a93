package application

import (
	"encoding/json"
	"fmt"
	"strings"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/rigging"
)

func (s *Service) ConfirmTrialStandard(cmd ConfirmTrialStandard) (*rigging.ClearanceCase, error) {
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
	now := s.now().UTC()
	standard := rigging.TrialStandard{ConfigurationVersion: c.ConfigurationVersion, Stages: cmd.Stages, AllowedReboundMM: cmd.AllowedReboundMM, MaxTotalDurationSec: cmd.MaxTotalDurationSec, ConfirmedBy: clean(cmd.Actor), ConfirmedAt: now}
	if err = rigging.ValidateTrialStandard(c, standard); err != nil {
		return nil, normalize(err)
	}
	standard.Digest, err = rigging.TrialStandardDigest(standard)
	if err != nil {
		return nil, normalize(err)
	}
	event, err := rigging.NewEvent(rigging.EventTrialStandardConfirmed, c.ID, cmd.Actor, now, rigging.TrialStandardData{Standard: standard})
	if err != nil {
		return nil, normalize(err)
	}
	return s.commit(cmd.CommandMeta, []rigging.Event{event}, cmd)
}

func (s *Service) RecordTrial(cmd RecordTrial) (*rigging.ClearanceCase, error) {
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
		cmd.ID = newID("trial")
	}
	if c.TrialStandard == nil {
		return nil, normalize(rigging.Rule("TRIAL_STANDARD_REQUIRED", "试吊开始前必须由舞台机械主管确认当前配置的判定标准"))
	}
	completed := s.now().UTC()
	trial := rigging.TrialLift{ID: cmd.ID, CaseID: c.ID, StartedAt: cmd.StartedAt.UTC(), DeadlineAt: cmd.DeadlineAt.UTC(), OperatorName: clean(cmd.OperatorName), StageObservations: cmd.Stages, Anomalies: cmd.Anomalies, CompletedAt: completed, StandardDigest: cmd.StandardDigest, ConfigurationVersion: c.ConfigurationVersion}
	if trial.StandardDigest == "" {
		trial.StandardDigest = c.TrialStandard.Digest
	}
	if err = rigging.ValidateTrial(c, trial, completed); err != nil {
		return nil, normalize(err)
	}
	trial.FailureReasons = rigging.EvaluateTrial(c, trial, completed)
	if len(trial.FailureReasons) == 0 {
		trial.Result = "passed"
	} else {
		trial.Result = "failed"
	}
	event, e := rigging.NewEvent(rigging.EventTrialRecorded, c.ID, cmd.Actor, completed, rigging.TrialData{Trial: trial})
	if e != nil {
		return nil, normalize(e)
	}
	events := []rigging.Event{event}
	if trial.Result == "failed" {
		failureJSON, _ := json.Marshal(trial.FailureReasons)
		stage := ""
		if len(trial.FailureReasons) > 0 {
			stage = trial.FailureReasons[0].Stage
		}
		finding := rigging.SafetyFinding{ID: newID("finding"), CaseID: c.ID, Severity: "blocking", Description: "试吊自动判定失败：" + string(failureJSON), Status: "open", ReportedBy: trial.OperatorName, LinkedTrialID: trial.ID, FailedStage: stage}
		findingEvent, err := rigging.NewEvent(rigging.EventFindingAdded, c.ID, "系统试吊判定", completed, rigging.FindingData{Finding: finding})
		if err != nil {
			return nil, normalize(err)
		}
		events = append(events, findingEvent)
	}
	return s.commit(cmd.CommandMeta, events, request)
}

func (s *Service) Freeze(cmd FreezeManifest) (*rigging.ClearanceCase, error) {
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
	if c.Status != rigging.StatusFreezeReady {
		return nil, normalize(rigging.Rule("FREEZE_GATE_FAILED", "仅通过试吊的档案可以冻结"))
	}
	digest, err := rigging.ManifestDigest(c)
	if err != nil {
		return nil, normalize(err)
	}
	now := s.now().UTC()
	event, e := rigging.NewEvent(rigging.EventManifestFrozen, c.ID, cmd.Actor, now, rigging.FreezeData{Digest: digest, FrozenAt: now})
	if e != nil {
		return nil, normalize(e)
	}
	return s.commit(cmd.CommandMeta, []rigging.Event{event}, cmd)
}

func (s *Service) Issue(cmd IssueCredential) (*rigging.ClearanceCase, error) {
	s.commitMu.Lock()
	defer s.commitMu.Unlock()
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
	if c.Status != rigging.StatusFrozen {
		return nil, normalize(rigging.Rule("ISSUE_GATE_FAILED", "仅冻结档案可以签发凭据"))
	}
	if clean(cmd.IssuedBy) == "" || !strings.EqualFold(clean(cmd.IssuedBy), clean(c.ManagerName)) {
		return nil, normalize(rigging.Rule("MANAGER_REQUIRED", "凭据必须由档案中的舞台机械主管签发"))
	}
	sequence := s.nextCredentialSequence + 1
	now := s.now().UTC()
	number := fmt.Sprintf("RC-%s-%04d-%s", c.PerformanceAt.UTC().Format("20060102"), sequence, shortID(c.ID))
	credential := rigging.ReleaseCredential{Number: number, CaseID: c.ID, Sequence: sequence, FrozenVersion: c.Version, ManifestDigest: c.ManifestDigest, PerformanceAt: c.PerformanceAt, IssuedBy: clean(cmd.IssuedBy), IssuedAt: now}
	event, e := rigging.NewEvent(rigging.EventCredentialIssued, c.ID, cmd.Actor, now, rigging.CredentialData{Credential: credential})
	if e != nil {
		return nil, normalize(e)
	}
	result, _, err := s.store.Commit(cmd.ExpectedVersion, []rigging.Event{event}, cmd.Actor, cmd.IdempotencyKey, cmd)
	if err != nil {
		return nil, normalize(err)
	}
	s.nextCredentialSequence = sequence
	return result, nil
}

func shortID(id string) string {
	r := []rune(id)
	if len(r) > 8 {
		return string(r[len(r)-8:])
	}
	return id
}
