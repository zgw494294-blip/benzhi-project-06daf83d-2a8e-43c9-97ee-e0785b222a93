package application

import (
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/rigging"
)

func (s *Service) PreviewConfiguration(cmd PreviewConfiguration) (rigging.ConfigurationPreflight, error) {
	c, err := s.load(cmd.CaseID)
	if err != nil {
		return rigging.ConfigurationPreflight{}, err
	}
	if cmd.ExpectedVersion != c.Version {
		return rigging.ConfigurationPreflight{}, &Error{Code: "VERSION_CONFLICT", Message: "预检版本已过期，请刷新档案后重新预检", Status: 409, Details: map[string]int{"expected": cmd.ExpectedVersion, "actual": c.Version}}
	}
	if err = rigging.RequireMutable(c); err != nil {
		return rigging.ConfigurationPreflight{}, normalize(err)
	}
	if c.Status != rigging.StatusDraft && c.Status != rigging.StatusInspection && c.Status != rigging.StatusRemediation && c.Status != rigging.StatusTrialReady && c.Status != rigging.StatusFreezeReady {
		return rigging.ConfigurationPreflight{}, normalize(rigging.Rule("INVALID_STATE", "当前状态不能修改载荷配置"))
	}
	for i := range cmd.LoadPoints {
		cmd.LoadPoints[i].CaseID = c.ID
	}
	for i := range cmd.Items {
		cmd.Items[i].CaseID = c.ID
	}
	result, err := rigging.BuildConfigurationPreflight(c, cmd.ExpectedVersion, cmd.LoadPoints, cmd.Items)
	return result, normalize(err)
}

func (s *Service) Configure(cmd SetConfiguration) (*rigging.ClearanceCase, error) {
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
	if err = rigging.RequireMutable(c); err != nil {
		return nil, normalize(err)
	}
	if c.Status != rigging.StatusDraft && c.Status != rigging.StatusInspection && c.Status != rigging.StatusRemediation && c.Status != rigging.StatusTrialReady && c.Status != rigging.StatusFreezeReady {
		return nil, normalize(rigging.Rule("INVALID_STATE", "当前状态不能修改载荷配置"))
	}
	for i := range cmd.LoadPoints {
		cmd.LoadPoints[i].CaseID = c.ID
	}
	for i := range cmd.Items {
		cmd.Items[i].CaseID = c.ID
	}
	summary, err := rigging.CalculateLoads(cmd.LoadPoints, cmd.Items)
	if err != nil {
		return nil, normalize(err)
	}
	cmd.LoadPoints = summary.Points
	preflight, err := rigging.BuildConfigurationPreflight(c, cmd.ExpectedVersion, cmd.LoadPoints, cmd.Items)
	if err != nil {
		return nil, normalize(err)
	}
	if len(c.LoadPoints) > 0 && cmd.PreflightDigest == "" {
		return nil, &Error{Code: "PREFLIGHT_REQUIRED", Message: "覆盖当前载荷配置前必须先完成预检", Status: 409}
	}
	if cmd.PreflightDigest != "" && cmd.PreflightDigest != preflight.ConfirmationDigest {
		return nil, &Error{Code: "PREFLIGHT_CONFLICT", Message: "预检摘要与当前版本或候选配置不一致，请重新预检", Status: 409}
	}
	if preflight.RequiresConfirmation && !cmd.ConfirmInvalidation {
		return nil, &Error{Code: "INVALIDATION_CONFIRMATION_REQUIRED", Message: "必须确认检查、问题或试吊成果将失效后才能提交", Status: 409, Details: preflight.InvalidatedResults}
	}
	event, err := rigging.NewEvent(rigging.EventConfigurationSet, c.ID, cmd.Actor, s.now(), rigging.ConfigurationData{LoadPoints: cmd.LoadPoints, Items: cmd.Items, NextStatus: rigging.StatusInspection, ConfigurationVersion: c.Version + 1})
	if err != nil {
		return nil, normalize(err)
	}
	return s.commit(cmd.CommandMeta, []rigging.Event{event}, request)
}
