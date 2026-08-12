package production

import (
	"math"
	"testing"
)

func TestBuildMultilevelManufacturingPlanUsesStockBeforeCreatingUpstreamShortage(t *testing.T) {
	product := ManufacturingItemRef{Type: "product", ID: 227, Name: "227g咖啡", Unit: "unit"}
	roasted := ManufacturingItemRef{Type: "material", ID: 10, Name: "熟豆", Unit: "g"}
	bag := ManufacturingItemRef{Type: "material", ID: 20, Name: "咖啡袋", Unit: "unit"}
	green := ManufacturingItemRef{Type: "material", ID: 30, Name: "生豆", Unit: "g"}

	plan, err := BuildMultilevelManufacturingPlan(
		[]ManufacturingDemand{{Item: product, Qty: 100, TargetWarehouse: "finished_goods"}},
		[]ManufacturingBOM{
			{
				VersionID: 1001,
				Output:    product,
				OutputQty: 1,
				Components: []ManufacturingBOMComponent{
					{Item: roasted, Qty: 227},
					{Item: bag, Qty: 1, Fixed: true},
				},
			},
			{
				VersionID: 1002,
				Output:    roasted,
				OutputQty: 1000,
				Components: []ManufacturingBOMComponent{
					{Item: green, Qty: 1250},
				},
			},
		},
		map[string]float64{
			roasted.Key(): 10000,
			bag.Key():     100,
			green.Key():   20000,
		},
	)
	if err != nil {
		t.Fatalf("BuildMultilevelManufacturingPlan: %v", err)
	}

	root := requireManufacturingPlanNode(t, plan, product.Key())
	if root.Action != ManufacturingSupplyManufacture || root.RequiredQty != 100 || root.BOMVersionID != 1001 {
		t.Fatalf("root=%+v", root)
	}
	roastedNode := requireManufacturingPlanNode(t, plan, roasted.Key())
	if roastedNode.RequiredQty != 22700 || roastedNode.StockCoveredQty != 10000 || roastedNode.ShortageQty != 12700 {
		t.Fatalf("roasted=%+v, want required=22700 stock=10000 shortage=12700", roastedNode)
	}
	if roastedNode.Action != ManufacturingSupplyManufacture || roastedNode.BOMVersionID != 1002 {
		t.Fatalf("roasted action=%+v, want manufacture from BOM 1002", roastedNode)
	}
	bagNode := requireManufacturingPlanNode(t, plan, bag.Key())
	if bagNode.RequiredQty != 100 || bagNode.StockCoveredQty != 100 || bagNode.ShortageQty != 0 || bagNode.Action != ManufacturingSupplyInventory {
		t.Fatalf("bag=%+v, want 100 fixed units fully covered", bagNode)
	}
	greenNode := requireManufacturingPlanNode(t, plan, green.Key())
	if math.Abs(greenNode.RequiredQty-15875) > 0.000001 || greenNode.StockCoveredQty != 15875 || greenNode.ShortageQty != 0 {
		t.Fatalf("green=%+v, want 15,875g for only the 12.7kg roasted shortage", greenNode)
	}
	if got := plan.ReservedByItem[roasted.Key()]; got != 10000 {
		t.Fatalf("reserved roasted=%v, want 10000 exactly once", got)
	}
}

func TestBuildMultilevelManufacturingPlanMarksLeafShortageAsPurchaseBlocker(t *testing.T) {
	product := ManufacturingItemRef{Type: "product", ID: 1, Name: "SKU", Unit: "unit"}
	packageMaterial := ManufacturingItemRef{Type: "material", ID: 2, Name: "袋", Unit: "unit"}
	plan, err := BuildMultilevelManufacturingPlan(
		[]ManufacturingDemand{{Item: product, Qty: 10}},
		[]ManufacturingBOM{{
			VersionID:  1,
			Output:     product,
			OutputQty:  1,
			Components: []ManufacturingBOMComponent{{Item: packageMaterial, Qty: 1, Fixed: true}},
		}},
		map[string]float64{},
	)
	if err != nil {
		t.Fatalf("BuildMultilevelManufacturingPlan: %v", err)
	}
	leaf := requireManufacturingPlanNode(t, plan, packageMaterial.Key())
	if leaf.Action != ManufacturingSupplyPurchase || !leaf.Blocking || leaf.ShortageQty != 10 {
		t.Fatalf("leaf=%+v, want blocking purchase shortage", leaf)
	}
}

func TestBuildMultilevelManufacturingPlanCoversFullAndZeroSemiFinishedStock(t *testing.T) {
	product := ManufacturingItemRef{Type: "product", ID: 227, Name: "227g咖啡", Unit: "unit"}
	roasted := ManufacturingItemRef{Type: "material", ID: 10, Name: "熟豆", Unit: "g"}
	green := ManufacturingItemRef{Type: "material", ID: 30, Name: "生豆", Unit: "g"}
	boms := []ManufacturingBOM{
		{VersionID: 1001, Output: product, OutputQty: 1, Components: []ManufacturingBOMComponent{{Item: roasted, Qty: 227}}},
		{VersionID: 1002, Output: roasted, OutputQty: 1000, Components: []ManufacturingBOMComponent{{Item: green, Qty: 1250}}},
	}

	for _, tc := range []struct {
		name             string
		availableRoasted float64
		wantReserved     float64
		wantShortage     float64
		wantGreen        float64
	}{
		{name: "full stock", availableRoasted: 22700, wantReserved: 22700},
		{name: "zero stock", availableRoasted: 0, wantShortage: 22700, wantGreen: 28375},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := BuildMultilevelManufacturingPlan(
				[]ManufacturingDemand{{Item: product, Qty: 100}},
				boms,
				map[string]float64{roasted.Key(): tc.availableRoasted, green.Key(): 100000},
			)
			if err != nil {
				t.Fatalf("BuildMultilevelManufacturingPlan: %v", err)
			}
			roastedNode := requireManufacturingPlanNode(t, plan, roasted.Key())
			if roastedNode.StockCoveredQty != tc.wantReserved || roastedNode.ShortageQty != tc.wantShortage {
				t.Fatalf("roasted=%+v, want stock=%v shortage=%v", roastedNode, tc.wantReserved, tc.wantShortage)
			}
			if got := plan.ReservedByItem[roasted.Key()]; got != tc.wantReserved {
				t.Fatalf("reserved roasted=%v, want %v", got, tc.wantReserved)
			}
			if tc.wantGreen == 0 {
				for _, node := range plan.Nodes {
					if node.Item.Key() == green.Key() {
						t.Fatalf("full roasted inventory must not explode upstream green demand: %+v", node)
					}
				}
			} else if greenNode := requireManufacturingPlanNode(t, plan, green.Key()); greenNode.RequiredQty != tc.wantGreen {
				t.Fatalf("green=%+v, want required=%v", greenNode, tc.wantGreen)
			}
		})
	}
}

func TestBuildMultilevelManufacturingPlanRejectsTypedCycle(t *testing.T) {
	a := ManufacturingItemRef{Type: "material", ID: 1, Name: "A", Unit: "g"}
	b := ManufacturingItemRef{Type: "product", ID: 1, Name: "B", Unit: "g"}
	_, err := BuildMultilevelManufacturingPlan(
		[]ManufacturingDemand{{Item: a, Qty: 1}},
		[]ManufacturingBOM{
			{VersionID: 1, Output: a, OutputQty: 1, Components: []ManufacturingBOMComponent{{Item: b, Qty: 1}}},
			{VersionID: 2, Output: b, OutputQty: 1, Components: []ManufacturingBOMComponent{{Item: a, Qty: 1}}},
		},
		nil,
	)
	if err == nil {
		t.Fatal("expected typed BOM cycle error")
	}
}

func TestBuildMultilevelManufacturingPlanAllocatesSharedSupplierShortagePerConsumerEdge(t *testing.T) {
	root := ManufacturingItemRef{Type: "product", ID: 1, Name: "组合成品", Unit: "unit"}
	first := ManufacturingItemRef{Type: "material", ID: 10, Name: "半成品A", Unit: "g"}
	second := ManufacturingItemRef{Type: "material", ID: 20, Name: "半成品B", Unit: "g"}
	shared := ManufacturingItemRef{Type: "material", ID: 30, Name: "共用组件", Unit: "g"}

	plan, err := BuildMultilevelManufacturingPlan(
		[]ManufacturingDemand{{Item: root, Qty: 1}},
		[]ManufacturingBOM{
			{VersionID: 100, Output: root, OutputQty: 1, Components: []ManufacturingBOMComponent{{Item: first, Qty: 1}, {Item: second, Qty: 1}}},
			{VersionID: 101, Output: first, OutputQty: 1, Components: []ManufacturingBOMComponent{{Item: shared, Qty: 6}}},
			{VersionID: 102, Output: second, OutputQty: 1, Components: []ManufacturingBOMComponent{{Item: shared, Qty: 4}}},
			{VersionID: 103, Output: shared, OutputQty: 1},
		},
		map[string]float64{shared.Key(): 3},
	)
	if err != nil {
		t.Fatalf("BuildMultilevelManufacturingPlan: %v", err)
	}

	sharedNode := requireManufacturingPlanNode(t, plan, shared.Key())
	if sharedNode.RequiredQty != 10 || sharedNode.StockCoveredQty != 3 || sharedNode.ShortageQty != 7 {
		t.Fatalf("shared node=%+v, want one merged upstream shortage of 7", sharedNode)
	}
	firstEdge := requireManufacturingPlanEdge(t, plan, first.Key(), shared.Key())
	if firstEdge.RequiredQty != 6 || firstEdge.StockCoveredQty != 3 || firstEdge.ShortageQty != 3 {
		t.Fatalf("first edge=%+v, want required=6 covered=3 shortage=3", firstEdge)
	}
	secondEdge := requireManufacturingPlanEdge(t, plan, second.Key(), shared.Key())
	if secondEdge.RequiredQty != 4 || secondEdge.StockCoveredQty != 0 || secondEdge.ShortageQty != 4 {
		t.Fatalf("second edge=%+v, want required=4 covered=0 shortage=4", secondEdge)
	}
	if firstEdge.ShortageQty+secondEdge.ShortageQty != sharedNode.ShortageQty {
		t.Fatalf("edge shortages=%v+%v, want merged supplier shortage %v exactly once", firstEdge.ShortageQty, secondEdge.ShortageQty, sharedNode.ShortageQty)
	}
}

func requireManufacturingPlanNode(t *testing.T, plan ManufacturingPlan, key string) ManufacturingPlanNode {
	t.Helper()
	for _, node := range plan.Nodes {
		if node.Item.Key() == key {
			return node
		}
	}
	t.Fatalf("plan node %q missing: %+v", key, plan.Nodes)
	return ManufacturingPlanNode{}
}

func requireManufacturingPlanEdge(t *testing.T, plan ManufacturingPlan, consumerKey, supplierKey string) ManufacturingSupplyEdge {
	t.Helper()
	for _, edge := range plan.Edges {
		if edge.ConsumerKey == consumerKey && edge.SupplierKey == supplierKey {
			return edge
		}
	}
	t.Fatalf("plan edge %q -> %q missing: %+v", consumerKey, supplierKey, plan.Edges)
	return ManufacturingSupplyEdge{}
}
