package rigging

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

type ConfigurationImpact struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
	Label string `json:"label"`
}

type PointLoadChange struct {
	ID                     string `json:"id"`
	Label                  string `json:"label"`
	Change                 string `json:"change"`
	BeforeUtilizationBP    int64  `json:"beforeUtilizationBasisPoints"`
	AfterUtilizationBP     int64  `json:"afterUtilizationBasisPoints"`
	BeforeRemainingKg      int64  `json:"beforeRemainingKg"`
	AfterRemainingKg       int64  `json:"afterRemainingKg"`
	HighUtilizationWarning string `json:"highUtilizationWarning,omitempty"`
}

type EntityChange struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Change string `json:"change"`
}

type ConfigurationDiff struct {
	TotalLoadBeforeKg int64             `json:"totalLoadBeforeKg"`
	TotalLoadAfterKg  int64             `json:"totalLoadAfterKg"`
	TotalLoadDeltaKg  int64             `json:"totalLoadDeltaKg"`
	LoadPoints        []PointLoadChange `json:"loadPoints"`
	Items             []EntityChange    `json:"items"`
}

type ConfigurationPreflight struct {
	CaseID               string                `json:"caseId"`
	BaseVersion          int                   `json:"baseVersion"`
	ConfirmationDigest   string                `json:"confirmationDigest"`
	LoadSummary          LoadSummary           `json:"loadSummary"`
	Diff                 ConfigurationDiff     `json:"diff"`
	InvalidatedResults   []ConfigurationImpact `json:"invalidatedResults"`
	RequiresConfirmation bool                  `json:"requiresConfirmation"`
}

func ConfigurationDigest(caseID string, version int, points []LoadPoint, items []SuspendedItem) (string, error) {
	p := append([]LoadPoint(nil), points...)
	i := append([]SuspendedItem(nil), items...)
	sort.Slice(p, func(a, b int) bool { return p[a].ID < p[b].ID })
	for n := range i {
		i[n].CaseID = caseID
		sort.Slice(i[n].LoadPointShares, func(a, b int) bool { return i[n].LoadPointShares[a].LoadPointID < i[n].LoadPointShares[b].LoadPointID })
	}
	sort.Slice(i, func(a, b int) bool { return i[a].ID < i[b].ID })
	b, err := json.Marshal(struct {
		CaseID  string          `json:"caseId"`
		Version int             `json:"version"`
		Points  []LoadPoint     `json:"loadPoints"`
		Items   []SuspendedItem `json:"items"`
	}{caseID, version, p, i})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func BuildConfigurationPreflight(c *ClearanceCase, version int, points []LoadPoint, items []SuspendedItem) (ConfigurationPreflight, error) {
	after, err := CalculateLoads(points, items)
	if err != nil {
		return ConfigurationPreflight{}, err
	}
	before, _ := CalculateLoads(c.LoadPoints, c.Items)
	digest, err := ConfigurationDigest(c.ID, version, after.Points, items)
	if err != nil {
		return ConfigurationPreflight{}, err
	}
	result := ConfigurationPreflight{CaseID: c.ID, BaseVersion: version, ConfirmationDigest: digest, LoadSummary: after}
	result.Diff.TotalLoadBeforeKg, result.Diff.TotalLoadAfterKg = before.TotalLoadKg, after.TotalLoadKg
	result.Diff.TotalLoadDeltaKg = after.TotalLoadKg - before.TotalLoadKg
	oldPoints := map[string]LoadPoint{}
	for _, p := range before.Points {
		oldPoints[p.ID] = p
	}
	newPoints := map[string]LoadPoint{}
	for _, p := range after.Points {
		newPoints[p.ID] = p
	}
	ids := map[string]bool{}
	for id := range oldPoints {
		ids[id] = true
	}
	for id := range newPoints {
		ids[id] = true
	}
	for id := range ids {
		old, hadOld := oldPoints[id]
		next, hasNew := newPoints[id]
		change := "changed"
		if !hadOld {
			change = "added"
			old = LoadPoint{ID: id}
		}
		if !hasNew {
			change = "removed"
			next = LoadPoint{ID: id}
		}
		if hadOld && hasNew && old == next {
			continue
		}
		label := next.Label
		if label == "" {
			label = old.Label
		}
		row := PointLoadChange{ID: id, Label: label, Change: change, BeforeUtilizationBP: old.UtilizationBasisPoints, AfterUtilizationBP: next.UtilizationBasisPoints, BeforeRemainingKg: old.RatedLoadKg - old.AllocatedLoadKg, AfterRemainingKg: next.RatedLoadKg - next.AllocatedLoadKg}
		if hasNew && next.UtilizationBasisPoints >= 8000 {
			row.HighUtilizationWarning = "吊点利用率达到 80% 以上，请复核承载余量"
		}
		result.Diff.LoadPoints = append(result.Diff.LoadPoints, row)
	}
	sort.Slice(result.Diff.LoadPoints, func(i, j int) bool { return result.Diff.LoadPoints[i].ID < result.Diff.LoadPoints[j].ID })
	oldItems := map[string]SuspendedItem{}
	for _, x := range c.Items {
		oldItems[x.ID] = x
	}
	newItems := map[string]SuspendedItem{}
	for _, x := range items {
		newItems[x.ID] = x
	}
	itemIDs := map[string]bool{}
	for id := range oldItems {
		itemIDs[id] = true
	}
	for id := range newItems {
		itemIDs[id] = true
	}
	for id := range itemIDs {
		old, hadOld := oldItems[id]
		next, hasNew := newItems[id]
		change := "changed"
		if !hadOld {
			change = "added"
		}
		if !hasNew {
			change = "removed"
		}
		oldJSON, _ := json.Marshal(old)
		nextJSON, _ := json.Marshal(next)
		if hadOld && hasNew && string(oldJSON) == string(nextJSON) {
			continue
		}
		label := next.Label
		if label == "" {
			label = old.Label
		}
		result.Diff.Items = append(result.Diff.Items, EntityChange{ID: id, Label: label, Change: change})
	}
	sort.Slice(result.Diff.Items, func(i, j int) bool { return result.Diff.Items[i].ID < result.Diff.Items[j].ID })
	if len(c.Inspections) > 0 {
		result.InvalidatedResults = append(result.InvalidatedResults, ConfigurationImpact{"inspections", len(c.Inspections), "检查记录将失效"})
	}
	if len(c.Findings) > 0 {
		result.InvalidatedResults = append(result.InvalidatedResults, ConfigurationImpact{"findings", len(c.Findings), "问题及整改记录将从当前配置移除"})
	}
	if len(c.TrialLifts) > 0 {
		result.InvalidatedResults = append(result.InvalidatedResults, ConfigurationImpact{"trialLifts", len(c.TrialLifts), "试吊记录将失效"})
	}
	result.RequiresConfirmation = len(result.InvalidatedResults) > 0
	return result, nil
}
