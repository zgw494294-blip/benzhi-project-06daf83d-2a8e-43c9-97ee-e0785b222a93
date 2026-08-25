package rigging

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

type FrozenManifest struct {
	CaseID        string             `json:"caseId"`
	PerformanceAt string             `json:"performanceAt"`
	LoadPoints    []LoadPoint        `json:"loadPoints"`
	Items         []SuspendedItem    `json:"items"`
	Inspections   []InspectionRecord `json:"inspections"`
	Findings      []SafetyFinding    `json:"findings"`
	TrialLifts    []TrialLift        `json:"trialLifts"`
	TrialStandard *TrialStandard     `json:"trialStandard,omitempty"`
}

func ManifestDigest(c *ClearanceCase) (string, error) {
	m := BuildFrozenManifest(c)
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func BuildFrozenManifest(c *ClearanceCase) FrozenManifest {
	m := FrozenManifest{CaseID: c.ID, PerformanceAt: c.PerformanceAt.UTC().Format(timeFormat), LoadPoints: append([]LoadPoint(nil), c.LoadPoints...), Items: append([]SuspendedItem(nil), c.Items...), Inspections: append([]InspectionRecord(nil), c.Inspections...), Findings: append([]SafetyFinding(nil), c.Findings...), TrialLifts: append([]TrialLift(nil), c.TrialLifts...), TrialStandard: c.TrialStandard}
	sort.Slice(m.LoadPoints, func(i, j int) bool { return m.LoadPoints[i].ID < m.LoadPoints[j].ID })
	sort.Slice(m.Items, func(i, j int) bool { return m.Items[i].ID < m.Items[j].ID })
	sort.Slice(m.Inspections, func(i, j int) bool { return m.Inspections[i].ID < m.Inspections[j].ID })
	sort.Slice(m.Findings, func(i, j int) bool { return m.Findings[i].ID < m.Findings[j].ID })
	return m
}

const timeFormat = "2006-01-02T15:04:05.000000000Z07:00"
