package production

import (
	productionapp "orderapp/internal/application/production"
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
