package rigging

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

var RequiredTrialStages = []string{"quarter", "half", "full", "hold"}

func ValidateTrialStandard(c *ClearanceCase, standard TrialStandard) error {
	if err := RequireMutable(c); err != nil {
		return err
	}
	if c.Status != StatusTrialReady {
		return Rule("INVALID_STATE", "仅试吊就绪档案可以确认判定标准")
	}
	if standard.ConfigurationVersion != c.ConfigurationVersion {
		return Rule("TRIAL_STANDARD_STALE", "试吊标准绑定的配置版本已过期，请重新确认")
	}
	if strings.TrimSpace(standard.ConfirmedBy) == "" || !samePerson(standard.ConfirmedBy, c.ManagerName) {
		return Rule("MANAGER_REQUIRED", "试吊判定标准必须由档案中的舞台机械主管确认")
	}
	if len(standard.Stages) != len(RequiredTrialStages) {
		return Rule("INVALID_TRIAL_STANDARD", "试吊标准必须包含四个既定阶段")
	}
	minimumTotal := 0
	for i, criterion := range standard.Stages {
		if criterion.Stage != RequiredTrialStages[i] {
			return Rule("TRIAL_STAGE_ORDER", "试吊标准阶段顺序无效")
		}
		if criterion.MinDurationSec <= 0 || criterion.MinDurationSec > 1800 || criterion.MaxDeflectionMM < 0 || criterion.MaxDeflectionMM > 100000 {
			return Rule("INVALID_TRIAL_THRESHOLD", "试吊阶段时长或挠度阈值无效")
		}
		minimumTotal += criterion.MinDurationSec
	}
	if standard.AllowedReboundMM < 0 || standard.AllowedReboundMM > 100000 || standard.MaxTotalDurationSec <= 0 || standard.MaxTotalDurationSec > 1800 {
		return Rule("INVALID_TRIAL_THRESHOLD", "试吊回差或总时限阈值无效")
	}
	if minimumTotal > standard.MaxTotalDurationSec {
		return Rule("INVALID_TRIAL_THRESHOLD", "四阶段最短持续时间之和不能超过试吊总时限")
	}
	return nil
}

func TrialStandardDigest(standard TrialStandard) (string, error) {
	b, err := json.Marshal(struct {
		ConfigurationVersion int                   `json:"configurationVersion"`
		Stages               []TrialStageCriterion `json:"stages"`
		AllowedReboundMM     int64                 `json:"allowedReboundMm"`
		MaxTotalDurationSec  int                   `json:"maxTotalDurationSec"`
	}{standard.ConfigurationVersion, standard.Stages, standard.AllowedReboundMM, standard.MaxTotalDurationSec})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func ValidateTrial(c *ClearanceCase, trial TrialLift, now time.Time) error {
	if err := RequireMutable(c); err != nil {
		return err
	}
	if c.Status != StatusTrialReady {
		return Rule("INVALID_STATE", "档案尚未进入试吊就绪状态")
	}
	if HasOpenBlockingFindings(c) || !HasBothPassingInspections(c) {
		return Rule("TRIAL_GATE_FAILED", "试吊前必须完成双人合格检查并关闭阻断问题")
	}
	if trial.StartedAt.IsZero() || trial.DeadlineAt.IsZero() || !trial.DeadlineAt.After(trial.StartedAt) {
		return Rule("INVALID_TRIAL_WINDOW", "试吊时间窗口无效")
	}
	if len(trial.StageObservations) != len(RequiredTrialStages) {
		return Rule("TRIAL_INCOMPLETE", "试吊必须且只能提交四个阶段")
	}
	seen := map[string]bool{}
	var prior time.Time
	for i, stage := range trial.StageObservations {
		if stage.Stage != RequiredTrialStages[i] {
			return Rule("TRIAL_STAGE_ORDER", "试吊阶段必须按 quarter、half、full、hold 顺序提交")
		}
		if seen[stage.Stage] {
			return Rule("DUPLICATE_TRIAL_STAGE", "试吊阶段不能重复")
		}
		seen[stage.Stage] = true
		if stage.DurationSec <= 0 {
			return Rule("INVALID_STAGE", "试吊阶段时长必须为正数")
		}
		if stage.DeflectionMM < 0 {
			return Rule("INVALID_STAGE", "试吊阶段挠度不能为负数")
		}
		if stage.CompletedAt != nil {
			if stage.CompletedAt.Before(trial.StartedAt) || stage.CompletedAt.After(trial.DeadlineAt) || stage.CompletedAt.After(now) || (!prior.IsZero() && !stage.CompletedAt.After(prior)) {
				return Rule("TRIAL_STAGE_TIME_INVALID", "试吊阶段完成时间必须在窗口内严格递增")
			}
			prior = *stage.CompletedAt
		} else if c.TrialStandard != nil {
			return Rule("TRIAL_STAGE_TIME_REQUIRED", "使用主管确认标准时必须记录每个试吊阶段的完成时间")
		}
	}
	if c.TrialStandard != nil && (trial.StandardDigest != c.TrialStandard.Digest || trial.ConfigurationVersion != c.ConfigurationVersion) {
		return Rule("TRIAL_STANDARD_STALE", "试吊使用的判定标准或配置版本已过期")
	}
	return nil
}

func EvaluateTrial(c *ClearanceCase, trial TrialLift, now time.Time) []TrialFailure {
	standard := c.TrialStandard
	if standard == nil {
		standard = &TrialStandard{MaxTotalDurationSec: 1800, AllowedReboundMM: 100000}
		for _, stage := range RequiredTrialStages {
			standard.Stages = append(standard.Stages, TrialStageCriterion{Stage: stage, MinDurationSec: 1, MaxDeflectionMM: 100000})
		}
	}
	failures := []TrialFailure{}
	if now.After(trial.DeadlineAt) || trial.DeadlineAt.Sub(trial.StartedAt) > time.Duration(standard.MaxTotalDurationSec)*time.Second {
		failures = append(failures, TrialFailure{Code: "TRIAL_TIME_EXCEEDED", Reason: "试吊超过主管确认的总时限", Actual: int64(now.Sub(trial.StartedAt).Seconds()), Threshold: int64(standard.MaxTotalDurationSec)})
	}
	for i, observation := range trial.StageObservations {
		criterion := standard.Stages[i]
		if observation.DurationSec < criterion.MinDurationSec {
			failures = append(failures, TrialFailure{Code: "STAGE_DURATION_SHORT", Stage: observation.Stage, Reason: "阶段持续时间不足", Actual: int64(observation.DurationSec), Threshold: int64(criterion.MinDurationSec)})
		}
		if observation.DeflectionMM > criterion.MaxDeflectionMM {
			failures = append(failures, TrialFailure{Code: "DEFLECTION_EXCEEDED", Stage: observation.Stage, Reason: "阶段挠度超过阈值", Actual: observation.DeflectionMM, Threshold: criterion.MaxDeflectionMM})
		}
		if !observation.Stable {
			failures = append(failures, TrialFailure{Code: "STAGE_UNSTABLE", Stage: observation.Stage, Reason: "阶段观测不稳定"})
		}
	}
	full, hold := trial.StageObservations[2].DeflectionMM, trial.StageObservations[3].DeflectionMM
	rebound := full - hold
	if rebound < 0 {
		rebound = -rebound
	}
	if rebound > standard.AllowedReboundMM {
		failures = append(failures, TrialFailure{Code: "REBOUND_EXCEEDED", Stage: "hold", Reason: "满载保持后的回差超过阈值", Actual: rebound, Threshold: standard.AllowedReboundMM})
	}
	for _, anomaly := range trial.Anomalies {
		if strings.TrimSpace(anomaly) != "" {
			failures = append(failures, TrialFailure{Code: "MANUAL_ANOMALY", Reason: strings.TrimSpace(anomaly)})
		}
	}
	return failures
}

func TrialPassed(trial TrialLift) bool {
	return trial.Result == "passed" || (len(trial.FailureReasons) == 0 && len(trial.Anomalies) == 0)
}
