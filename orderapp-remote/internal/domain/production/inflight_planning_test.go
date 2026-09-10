package production

import "testing"

func TestMultilevelSupplyAllocatesStockThenInflightOnlyOnce(t *testing.T) {
	p1 := ManufacturingItemRef{Type: "product", ID: 1, Unit: "袋"}
	p2 := ManufacturingItemRef{Type: "product", ID: 2, Unit: "袋"}
	m := ManufacturingItemRef{Type: "material", ID: 10, Unit: "kg"}
	raw := ManufacturingItemRef{Type: "material", ID: 20, Unit: "kg"}
	boms := []ManufacturingBOM{
		{VersionID: 1, Output: p1, OutputQty: 1, Components: []ManufacturingBOMComponent{{Item: m, Qty: 1}}},
		{VersionID: 2, Output: p2, OutputQty: 1, Components: []ManufacturingBOMComponent{{Item: m, Qty: 1}}},
		{VersionID: 3, Output: m, OutputQty: 1, Components: []ManufacturingBOMComponent{{Item: raw, Qty: 1.25}}},
	}
	p, err := BuildMultilevelManufacturingPlanWithSupply([]ManufacturingDemand{{Item: p1, Qty: 10}, {Item: p2, Qty: 10}}, boms, map[string]float64{m.Key(): 5, raw.Key(): 10}, map[string]float64{m.Key(): 8})
	if err != nil {
		t.Fatal(err)
	}
	n := requireManufacturingPlanNode(t, p, m.Key())
	if n.StockCoveredQty != 5 || n.InflightCoveredQty != 8 || n.ShortageQty != 7 {
		t.Fatalf("coverage=%+v", n)
	}
	var inflight float64
	for _, e := range p.Edges {
		if e.SupplierKey == m.Key() {
			inflight += e.InflightCoveredQty
		}
	}
	if inflight != 8 {
		t.Fatalf("shared supply counted %v times", inflight)
	}
	if requireManufacturingPlanNode(t, p, raw.Key()).RequiredQty != 8.75 {
		t.Fatal("raw material demand must only cover new production")
	}
}
