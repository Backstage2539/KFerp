package production

import (
	"testing"

	productionapp "orderapp/internal/application/production"
)

func TestInstantCoffeeNoBomMaterialsUseInstantCoffeeRawMaterial(t *testing.T) {
	row := productionapp.UnprodNeedRow{
		ProductID:      88,
		Product:        "冻干美式",
		SpecG:          100,
		GapG:           500,
		ProductionKind: "instant_coffee",
	}

	got := calcNoBomProducePlanMaterials(row, defaultPlanParams())

	assertMaterialNeed(t, got, "速溶咖啡", 500, "g")
	assertMaterialNeed(t, got, "速溶-盒子", 5, "个")
	assertNoMaterialNeed(t, got, "冻干美式 生豆")
	assertNoMaterialNeed(t, got, "豆袋")
}

func TestNoBomRawMaterialUsesProductTypeNameForInstantCoffee(t *testing.T) {
	row := productionapp.UnprodNeedRow{
		ProductID:       89,
		Product:         "冻干美式",
		SpecG:           100,
		GapG:            500,
		ProductionKind:  "roasted_bean",
		ProductTypeName: "速溶咖啡",
	}

	got := calcNoBomProducePlanMaterials(row, defaultPlanParams())

	assertMaterialNeed(t, got, "速溶咖啡", 500, "g")
	assertMaterialNeed(t, got, "速溶-盒子", 5, "个")
	assertNoMaterialNeed(t, got, "冻干美式 生豆")
}

func TestBuildRoastPlanRowsCarriesOperationTemplateID(t *testing.T) {
	rows := []productionapp.UnprodNeedRow{{
		ProductID:                89,
		Product:                  "冻干美式",
		SpecG:                    100,
		GapG:                     500,
		ProductTypeName:          "速溶咖啡",
		ProductSubtypeName:       "冻干速溶",
		ProductSubtypeCategoryID: 12,
		OperationTemplateID:      22,
	}}

	got := buildRoastPlanRows(rows, nil, map[int64]float64{89: 1})

	if len(got) != 1 || got[0].OperationTemplateID != 22 {
		t.Fatalf("roast plan operation template = %+v, want 22", got)
	}
}

func TestBuildRoastPlanMaterialRatiosUsesInstantCoffeeRawMaterial(t *testing.T) {
	rows := []productionapp.UnprodNeedRow{{
		ProductID:      88,
		Product:        "冻干美式",
		SpecG:          100,
		GapG:           500,
		ProductionKind: "instant_coffee",
	}}

	got := buildRoastPlanMaterialRatios(rows, map[int64][]planBomItem{})

	if len(got) != 1 {
		t.Fatalf("material ratios = %+v, want one row", got)
	}
	if got[0].MaterialName != "速溶咖啡" || got[0].MaterialUnit != "g" || got[0].RatioPct != 100 {
		t.Fatalf("instant coffee material ratio = %+v", got[0])
	}
}

func assertMaterialNeed(t *testing.T, rows []productionapp.MaterialNeed, name string, qty int64, unit string) {
	t.Helper()
	for _, row := range rows {
		if row.Name == name && row.Qty == qty && row.Unit == unit {
			return
		}
	}
	t.Fatalf("material needs = %+v, missing %s %d%s", rows, name, qty, unit)
}

func assertNoMaterialNeed(t *testing.T, rows []productionapp.MaterialNeed, name string) {
	t.Helper()
	for _, row := range rows {
		if row.Name == name {
			t.Fatalf("material needs should not include %q: %+v", name, rows)
		}
	}
}
