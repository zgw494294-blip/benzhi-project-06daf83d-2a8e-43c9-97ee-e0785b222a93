package rigging

import (
	"sort"
	"strings"
)

type LoadSummary struct {
	TotalLoadKg int64       `json:"totalLoadKg"`
	Points      []LoadPoint `json:"points"`
}

func CalculateLoads(points []LoadPoint, items []SuspendedItem) (LoadSummary, error) {
	if len(points) == 0 {
		return LoadSummary{}, Rule("LOAD_POINTS_REQUIRED", "至少登记一个吊点")
	}
	index := make(map[string]int, len(points))
	out := append([]LoadPoint(nil), points...)
	for i := range out {
		if strings.TrimSpace(out[i].ID) == "" || strings.TrimSpace(out[i].Label) == "" {
			return LoadSummary{}, Rule("INVALID_LOAD_POINT", "吊点标识和名称不能为空")
		}
		if out[i].RatedLoadKg <= 0 {
			return LoadSummary{}, Rule("RATING_REQUIRED", "吊点 %s 缺少有效额定载荷", out[i].Label)
		}
		if _, exists := index[out[i].ID]; exists {
			return LoadSummary{}, Rule("DUPLICATE_LOAD_POINT", "吊点标识重复")
		}
		out[i].AllocatedLoadKg = 0
		out[i].UtilizationBasisPoints = 0
		index[out[i].ID] = i
	}
	var total int64
	itemIDs := map[string]bool{}
	for _, item := range items {
		if item.ID == "" || item.Label == "" || item.Kind == "" {
			return LoadSummary{}, Rule("INVALID_ITEM", "悬挂设备标识、类别和名称不能为空")
		}
		if itemIDs[item.ID] {
			return LoadSummary{}, Rule("DUPLICATE_ITEM", "悬挂设备标识重复")
		}
		itemIDs[item.ID] = true
		if item.SelfWeightKg < 0 || item.WorkingLoadLimitKg <= 0 {
			return LoadSummary{}, Rule("INVALID_ITEM_CAPACITY", "设备 %s 重量或额定能力无效", item.Label)
		}
		if item.SelfWeightKg > item.WorkingLoadLimitKg {
			return LoadSummary{}, Rule("ITEM_OVER_LIMIT", "设备 %s 自重超过额定能力", item.Label)
		}
		var shares int64
		shareIDs := map[string]bool{}
		for _, share := range item.LoadPointShares {
			if share.BasisPoints <= 0 {
				return LoadSummary{}, Rule("INVALID_SHARE", "设备 %s 存在非正分配比例", item.Label)
			}
			idx, ok := index[share.LoadPointID]
			if !ok {
				return LoadSummary{}, Rule("UNKNOWN_LOAD_POINT", "设备 %s 引用了未知吊点", item.Label)
			}
			if shareIDs[share.LoadPointID] {
				return LoadSummary{}, Rule("DUPLICATE_SHARE", "设备 %s 重复分配到同一吊点", item.Label)
			}
			shareIDs[share.LoadPointID] = true
			shares += share.BasisPoints
			_ = idx
		}
		if shares != 10000 {
			return LoadSummary{}, Rule("SHARES_NOT_CLOSED", "设备 %s 的分配比例必须合计 10000", item.Label)
		}
		var assigned int64
		for i, share := range item.LoadPointShares {
			load := item.SelfWeightKg * share.BasisPoints / 10000
			if i == len(item.LoadPointShares)-1 {
				load = item.SelfWeightKg - assigned
			}
			out[index[share.LoadPointID]].AllocatedLoadKg += load
			assigned += load
		}
		total += item.SelfWeightKg
	}
	if len(items) == 0 {
		return LoadSummary{}, Rule("ITEMS_REQUIRED", "至少登记一件悬挂设备")
	}
	for i := range out {
		out[i].UtilizationBasisPoints = out[i].AllocatedLoadKg * 10000 / out[i].RatedLoadKg
		if out[i].AllocatedLoadKg > out[i].RatedLoadKg {
			return LoadSummary{}, Rule("LOAD_POINT_OVER_LIMIT", "吊点 %s 超过额定载荷", out[i].Label)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return LoadSummary{TotalLoadKg: total, Points: out}, nil
}
