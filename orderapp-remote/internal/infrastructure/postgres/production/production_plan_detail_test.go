package production

import (
	"testing"

	productionapp "orderapp/internal/application/production"
)

func TestAggregateProductionPlanMaterialSummaryUsesPlanSnapshots(t *testing.T) {
	items := []productionapp.ProductionPlanItem{
		{
			ID:             51,
			SpecG:          454,
			PlannedG:       1000,
			PlannedOutputG: 908,
			MaterialSnapshot: `[
				{"material_name":"纸板","unit":"张","ratio_pct":100,"source":"packaging"},
				{"material_name":"油墨","unit":"g","ratio_pct":5,"source":"bom"}
			]`,
		},
		{
			ID:             52,
			SpecG:          454,
			PlannedG:       500,
			PlannedOutputG: 454,
			MaterialSnapshot: `[
				{"material_name":"纸板","unit":"张","ratio_pct":100,"source":"packaging"},
				{"material_name":"油墨","unit":"g","ratio_pct":5,"source":"bom"}
			]`,
		},
	}

	got := aggregateProductionPlanMaterialSummary(items)
	assertProductionPlanMaterial(t, got, "纸板", 3, "张")
	assertProductionPlanMaterial(t, got, "油墨", 75, "g")
}

func TestAggregateProductionPlanMaterialSummarySkipsInvalidSnapshots(t *testing.T) {
	got := aggregateProductionPlanMaterialSummary([]productionapp.ProductionPlanItem{{
		ID:               51,
		PlannedG:         1000,
		MaterialSnapshot: `{not valid json`,
	}})
	if len(got) != 0 {
		t.Fatalf("invalid material snapshot summary = %+v, want empty", got)
	}
}

func TestAggregateProductionPlanMaterialSummaryRoundsOutputUnitsUp(t *testing.T) {
	got := aggregateProductionPlanMaterialSummary([]productionapp.ProductionPlanItem{{
		ID:             53,
		SpecG:          454,
		PlannedG:       600,
		PlannedOutputG: 455,
		MaterialSnapshot: `[
			{"material_name":"包装盒","unit":"个","source":"packaging"}
		]`,
	}})
	assertProductionPlanMaterial(t, got, "包装盒", 2, "个")
}

func TestAggregateProductionPlanMaterialSummaryUsesDictionaryGramQuantities(t *testing.T) {
	got := aggregateProductionPlanMaterialSummary([]productionapp.ProductionPlanItem{{
		ID:             54,
		SpecG:          454,
		PlannedG:       1135,
		PlannedOutputG: 908,
		MaterialSnapshot: `[
			{"material_name":"哥伦比亚EP","unit":"g","source":"bom","consume_unit":"g","qty_per_unit":114},
			{"material_name":"孟连水洗A","unit":"g","source":"bom","consume_unit":"g","qty_per_unit":284},
			{"material_name":"生豆-巴布亚之光-石光","unit":"g","source":"bom","consume_unit":"g","qty_per_unit":171}
		]`,
	}})

	assertProductionPlanMaterial(t, got, "哥伦比亚EP", 228, "g")
	assertProductionPlanMaterial(t, got, "孟连水洗A", 568, "g")
	assertProductionPlanMaterial(t, got, "生豆-巴布亚之光-石光", 342, "g")
}

func assertProductionPlanMaterial(t *testing.T, rows []productionapp.MaterialNeed, name string, qty int64, unit string) {
	t.Helper()
	for _, row := range rows {
		if row.Name == name {
			if row.Qty != qty || row.Unit != unit {
				t.Fatalf("material %s = %+v, want qty=%d unit=%s", name, row, qty, unit)
			}
			return
		}
	}
	t.Fatalf("material summary missing %s in %+v", name, rows)
}
