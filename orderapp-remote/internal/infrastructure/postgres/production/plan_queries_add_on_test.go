package production

import (
	productionapp "orderapp/internal/application/production"
	"os"
	"strings"
	"testing"
)

func TestSplitProductionDemandRowByPartsKeepsAddOnSelectable(t *testing.T) {
	row := UnprodNeedRow{
		ProductID:  554,
		Product:    "榛巧拼配",
		OrderNos:   "SO-OLD,SO-NEW",
		SpecG:      454,
		NeedUnits:  5,
		NeedG:      2270,
		AvailableG: 0,
		GapG:       2270,
	}
	parts := []productionDemandPart{
		{
			ProductID:         554,
			SpecG:             454,
			OrderNo:           "SO-OLD",
			NeedUnits:         2,
			ForceProduceUnits: 0,
			State: productionDemandPlanState{
				Status:           "in_production",
				ProductionPlanID: 4921,
				ProductionPlanNo: "PP-OLD",
			},
		},
		{
			ProductID:         554,
			SpecG:             454,
			OrderNo:           "SO-NEW",
			NeedUnits:         3,
			ForceProduceUnits: 0,
			State:             productionDemandPlanState{Status: "unplanned"},
		},
	}

	got := splitProductionDemandRowByParts(row, parts)
	if len(got) != 2 {
		t.Fatalf("split rows = %d, want 2: %+v", len(got), got)
	}
	var addOn, planned *UnprodNeedRow
	for i := range got {
		switch got[i].OrderNos {
		case "SO-NEW":
			addOn = &got[i]
		case "SO-OLD":
			planned = &got[i]
		}
	}
	if planned == nil || planned.DemandStatus != "in_production" || planned.DemandSelectable {
		t.Fatalf("planned row = %+v, want in_production and not selectable", planned)
	}
	if addOn == nil {
		t.Fatalf("add-on row missing from %+v", got)
	}
	if addOn.DemandStatus != "unplanned" || !addOn.DemandSelectable {
		t.Fatalf("add-on status/selectable = %s/%v, want unplanned/true", addOn.DemandStatus, addOn.DemandSelectable)
	}
	if addOn.NeedUnits != 3 || addOn.NeedG != 1362 || addOn.GapG != 1362 {
		t.Fatalf("add-on quantities = units %d need %d gap %d, want 3/1362/1362", addOn.NeedUnits, addOn.NeedG, addOn.GapG)
	}
}

func TestSelectedProductionPlanStartNeedsKeepsAddOnOrdersWhenOlderOrdersArePlanned(t *testing.T) {
	row := UnprodNeedRow{
		ProductID:  554,
		Product:    "榛巧拼配",
		OrderNos:   "SO-OLD,SO-NEW",
		SpecG:      454,
		NeedUnits:  5,
		NeedG:      2270,
		AvailableG: 0,
		GapG:       2270,
	}
	parts := []productionDemandPart{
		{
			ProductID: 554,
			SpecG:     454,
			OrderNo:   "SO-OLD",
			NeedUnits: 2,
			State: productionDemandPlanState{
				Status:           "in_production",
				ProductionPlanID: 4921,
				ProductionPlanNo: "PP-OLD",
			},
		},
		{
			ProductID: 554,
			SpecG:     454,
			OrderNo:   "SO-NEW",
			NeedUnits: 3,
			State:     productionDemandPlanState{Status: "unplanned"},
		},
	}

	rows := unprodRowsToApp(splitProductionDemandRowByParts(row, parts))
	needs := selectedProductionPlanStartNeeds(rows, map[string]bool{"554-454": true})

	if len(needs) != 1 {
		t.Fatalf("selected start needs = %d, want 1: %+v", len(needs), needs)
	}
	want := productionapp.StartNeed{
		ProductID:   554,
		ProductName: "榛巧拼配",
		SpecG:       454,
		GapG:        1362,
		OrderNos:    "SO-NEW",
	}
	if needs[0] != want {
		t.Fatalf("selected start need = %+v, want %+v", needs[0], want)
	}
}

func TestSplitProductionDemandRowByPartsKeepsDirectProductUnitGapSelectable(t *testing.T) {
	row := UnprodNeedRow{
		ProductID:                1066,
		Product:                  "盒装挂耳",
		OrderNos:                 "SO-BOX",
		SpecLabel:                "盒",
		SalesUnit:                "盒",
		SpecG:                    0,
		NeedUnits:                2,
		SalesSpecCount:           2,
		InventoryQtyPerSalesUnit: 1,
		InventoryUnit:            "盒",
		NeedInventoryQty:         2,
		AvailableInventoryQty:    0,
		GapInventoryQty:          2,
	}
	parts := []productionDemandPart{{
		ProductID: 1066,
		SpecG:     0,
		OrderNo:   "SO-BOX",
		NeedUnits: 2,
		State:     productionDemandPlanState{Status: "unplanned"},
	}}

	got := splitProductionDemandRowByParts(row, parts)
	if len(got) != 1 {
		t.Fatalf("split direct-product rows = %d, want 1: %+v", len(got), got)
	}
	if !got[0].DemandSelectable || got[0].GapInventoryQty != 2 || got[0].GapSalesSpecCount != 2 || got[0].GapG != 0 {
		t.Fatalf("direct-product unit gap = %+v, want selectable 2盒 with zero legacy grams", got[0])
	}
}

func TestMergeDripPlanRowsSuppressesLegacyGramProjectionForDirectProductDemand(t *testing.T) {
	direct := productionapp.UnprodNeedRow{
		ProductID:                1066,
		SelectionKey:             "1066-0",
		Product:                  "盒装挂耳",
		SpecLabel:                "盒",
		SalesUnit:                "盒",
		InventoryQtyPerSalesUnit: 1,
		InventoryUnit:            "盒",
		GapInventoryQty:          2,
	}
	legacyDrip := productionapp.UnprodNeedRow{
		ProductID:      1066,
		Product:        "盒装挂耳",
		SpecG:          10,
		GapG:           20,
		ProductionKind: "drip_bag",
	}

	got := mergeDripPlanRows([]productionapp.UnprodNeedRow{direct}, []productionapp.UnprodNeedRow{legacyDrip})
	if len(got) != 1 || got[0].SelectionKey != "1066-0" || got[0].SpecG != 0 {
		t.Fatalf("merged direct-product demand = %+v, want only authoritative product-unit row", got)
	}
}

func TestDripProductionPlanNeedsUseOrderPriceSnapshotUnitConversion(t *testing.T) {
	b, err := os.ReadFile("plan_queries.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	start := strings.Index(src, "func (r Repository) fetchDripPlanNeeds")
	if start < 0 {
		t.Fatalf("missing fetchDripPlanNeeds")
	}
	end := strings.Index(src[start:], "func defaultPlanParams")
	if end < 0 {
		t.Fatalf("missing fetchDripPlanNeeds end marker")
	}
	fn := src[start : start+end]
	for _, want := range []string{
		"price_source_json",
		"inventory_conversion_json",
		"inventory_unit",
		"sales_unit",
		"COALESCE(NULLIF(oi.sales_unit,''), NULLIF(oi.unit,''), oi.price_source_json->>'price_unit')",
	} {
		if !strings.Contains(fn, want) {
			t.Fatalf("drip production demand must use order price snapshot unit conversion; missing %q", want)
		}
	}
	for _, banned := range []string{
		"= 'box'",
		`= "box"`,
	} {
		if strings.Contains(fn, banned) {
			t.Fatalf("drip production demand must not hard-code box conversion; found %q", banned)
		}
	}
}
