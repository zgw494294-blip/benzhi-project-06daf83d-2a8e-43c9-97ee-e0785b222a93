package rigging

import "testing"

func TestCalculateLoadsPreservesWholeWeightAndUtilization(t *testing.T) {
	points := []LoadPoint{{ID: "a", Label: "A", RatedLoadKg: 10}, {ID: "b", Label: "B", RatedLoadKg: 10}}
	items := []SuspendedItem{{ID: "i", Kind: "scenery", Label: "布景", SelfWeightKg: 3, WorkingLoadLimitKg: 20, LoadPointShares: []LoadShare{{LoadPointID: "a", BasisPoints: 5000}, {LoadPointID: "b", BasisPoints: 5000}}}}
	summary, err := CalculateLoads(points, items)
	if err != nil {
		t.Fatal(err)
	}
	if summary.TotalLoadKg != 3 {
		t.Fatalf("总载荷 = %d", summary.TotalLoadKg)
	}
	var allocated int64
	for _, point := range summary.Points {
		allocated += point.AllocatedLoadKg
	}
	if allocated != 3 {
		t.Fatalf("分配载荷未闭合：%d", allocated)
	}
}

func TestCalculateLoadsRejectsUnsafeConfiguration(t *testing.T) {
	points := []LoadPoint{{ID: "a", Label: "A", RatedLoadKg: 100}}
	item := SuspendedItem{ID: "i", Kind: "hoist", Label: "葫芦", SelfWeightKg: 120, WorkingLoadLimitKg: 200, LoadPointShares: []LoadShare{{LoadPointID: "a", BasisPoints: 10000}}}
	if _, err := CalculateLoads(points, []SuspendedItem{item}); err == nil {
		t.Fatal("应拒绝吊点超限")
	}
	item.SelfWeightKg = 20
	item.LoadPointShares[0].BasisPoints = 9000
	if _, err := CalculateLoads(points, []SuspendedItem{item}); err == nil {
		t.Fatal("应拒绝分配不闭合")
	}
}
