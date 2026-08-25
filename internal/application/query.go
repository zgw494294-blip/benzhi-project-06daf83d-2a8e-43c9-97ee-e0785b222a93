package application

import (
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/rigging"
	"sort"
	"strings"
	"time"
)

type CaseListFilter struct {
	Status          string
	Venue           string
	RiskLevel       string
	PerformanceFrom *time.Time
	PerformanceTo   *time.Time
}

type CaseListItem struct {
	*rigging.ClearanceCase
	Risk rigging.RiskSummary `json:"risk"`
}

type RiskStatistics struct {
	Counts         map[rigging.RiskLevel]int `json:"counts"`
	PendingRelease int                       `json:"pendingRelease"`
	Total          int                       `json:"total"`
}

type CaseDashboard struct {
	Cases      []CaseListItem `json:"cases"`
	Statistics RiskStatistics `json:"statistics"`
}

type CaseDetail struct {
	Case           *rigging.ClearanceCase  `json:"case"`
	LoadSummary    rigging.LoadSummary     `json:"loadSummary"`
	Gates          rigging.GateReport      `json:"gates"`
	FrozenManifest *rigging.FrozenManifest `json:"frozenManifest,omitempty"`
	Tasks          []string                `json:"tasks"`
	Timeline       []rigging.AuditEvent    `json:"timeline"`
}
type CredentialVerification struct {
	Valid      bool                       `json:"valid"`
	Credential *rigging.ReleaseCredential `json:"credential"`
	CaseTitle  string                     `json:"caseTitle"`
	Venue      string                     `json:"venue"`
	Timeline   []rigging.AuditEvent       `json:"timeline"`
}

func (s *Service) Detail(id string) (CaseDetail, error) {
	c, err := s.load(id)
	if err != nil {
		return CaseDetail{}, err
	}
	summary, _ := rigging.CalculateLoads(c.LoadPoints, c.Items)
	timeline, err := s.store.Timeline(id)
	if err != nil {
		return CaseDetail{}, normalize(err)
	}
	detail := CaseDetail{Case: c, LoadSummary: summary, Gates: rigging.EvaluateReleaseGates(c), Tasks: tasks(c), Timeline: timeline}
	if c.Status == rigging.StatusFrozen || c.Status == rigging.StatusReleased {
		manifest := rigging.BuildFrozenManifest(c)
		detail.FrozenManifest = &manifest
	}
	return detail, nil
}
func (s *Service) List() ([]*rigging.ClearanceCase, error) {
	v, err := s.store.List()
	return v, normalize(err)
}

func (s *Service) Dashboard(filter CaseListFilter) (CaseDashboard, error) {
	validStatuses := map[string]bool{"": true}
	for _, status := range []rigging.Status{rigging.StatusDraft, rigging.StatusInspection, rigging.StatusRemediation, rigging.StatusTrialReady, rigging.StatusFreezeReady, rigging.StatusFrozen, rigging.StatusReleased} {
		validStatuses[string(status)] = true
	}
	if !validStatuses[filter.Status] {
		return CaseDashboard{}, &Error{Code: "INVALID_STATUS_FILTER", Message: "status 查询参数无效", Status: 400}
	}
	validRisks := map[string]bool{"": true, string(rigging.RiskNormal): true, string(rigging.RiskNear): true, string(rigging.RiskUrgent): true, string(rigging.RiskOverdue): true, string(rigging.RiskReleased): true}
	if !validRisks[filter.RiskLevel] {
		return CaseDashboard{}, &Error{Code: "INVALID_RISK_LEVEL", Message: "riskLevel 查询参数无效", Status: 400}
	}
	if filter.PerformanceFrom != nil && filter.PerformanceTo != nil && filter.PerformanceTo.Before(*filter.PerformanceFrom) {
		return CaseDashboard{}, &Error{Code: "INVALID_PERFORMANCE_WINDOW", Message: "performanceTo 不能早于 performanceFrom", Status: 400}
	}
	s.dashboardMu.Lock()
	defer s.dashboardMu.Unlock()
	all := s.dashboardCases
	if all == nil {
		var err error
		all, err = s.store.List()
		if err != nil {
			return CaseDashboard{}, normalize(err)
		}
		s.dashboardCases = all
	}
	counts := map[rigging.RiskLevel]int{rigging.RiskNormal: 0, rigging.RiskNear: 0, rigging.RiskUrgent: 0, rigging.RiskOverdue: 0, rigging.RiskReleased: 0}
	result := CaseDashboard{Statistics: RiskStatistics{Counts: counts}}
	now := s.now().UTC()
	for _, c := range all {
		risk, riskErr := rigging.BuildRiskSummary(c, now)
		if riskErr != nil {
			return CaseDashboard{}, normalize(riskErr)
		}
		if filter.Status != "" && string(c.Status) != filter.Status {
			continue
		}
		if filter.Venue != "" && !strings.EqualFold(strings.TrimSpace(c.Venue), strings.TrimSpace(filter.Venue)) {
			continue
		}
		if filter.PerformanceFrom != nil && c.PerformanceAt.Before(*filter.PerformanceFrom) {
			continue
		}
		if filter.PerformanceTo != nil && c.PerformanceAt.After(*filter.PerformanceTo) {
			continue
		}
		result.Statistics.Counts[risk.Level]++
		result.Statistics.Total++
		if risk.PendingRelease {
			result.Statistics.PendingRelease++
		}
		if filter.RiskLevel != "" && string(risk.Level) != filter.RiskLevel {
			continue
		}
		caseCopy, err := rigging.Clone(c)
		if err != nil {
			return CaseDashboard{}, normalize(err)
		}
		result.Cases = append(result.Cases, CaseListItem{ClearanceCase: caseCopy, Risk: risk})
	}
	priority := map[rigging.RiskLevel]int{rigging.RiskOverdue: 0, rigging.RiskUrgent: 1, rigging.RiskNear: 2, rigging.RiskNormal: 3, rigging.RiskReleased: 4}
	sort.SliceStable(result.Cases, func(i, j int) bool {
		left, right := result.Cases[i], result.Cases[j]
		if priority[left.Risk.Level] != priority[right.Risk.Level] {
			return priority[left.Risk.Level] < priority[right.Risk.Level]
		}
		if !left.PerformanceAt.Equal(right.PerformanceAt) {
			return left.PerformanceAt.Before(right.PerformanceAt)
		}
		return left.ID < right.ID
	})
	return result, nil
}
func (s *Service) Verify(number string) (CredentialVerification, error) {
	c, err := s.store.FindCredential(number)
	if err != nil {
		return CredentialVerification{}, normalize(err)
	}
	digest, err := rigging.ManifestDigest(c)
	if err != nil {
		return CredentialVerification{}, normalize(err)
	}
	timeline, err := s.store.Timeline(c.ID)
	if err != nil {
		return CredentialVerification{}, normalize(err)
	}
	valid := c.Credential != nil && c.Credential.ManifestDigest == c.ManifestDigest && digest == c.ManifestDigest
	return CredentialVerification{Valid: valid, Credential: c.Credential, CaseTitle: c.Title, Venue: c.Venue, Timeline: timeline}, nil
}

func tasks(c *rigging.ClearanceCase) []string {
	switch c.Status {
	case rigging.StatusDraft:
		return []string{"登记吊点与悬挂设备"}
	case rigging.StatusInspection:
		return []string{"提交操作员检查", "提交独立复核员检查"}
	case rigging.StatusRemediation:
		return []string{"提交问题整改证据", "由独立人员复核关闭", "必要时重新检查"}
	case rigging.StatusTrialReady:
		if c.TrialStandard == nil {
			return []string{"由舞台机械主管确认试吊判定标准", "执行分阶段有界试吊"}
		}
		return []string{"按已确认阈值执行全阶段试吊"}
	case rigging.StatusFreezeReady:
		return []string{"由主管确认并冻结清单"}
	case rigging.StatusFrozen:
		return []string{"由舞台机械主管签发放行凭据"}
	default:
		return []string{"核验凭据并保留审计轨迹"}
	}
}
