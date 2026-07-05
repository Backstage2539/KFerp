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

func TestPreviewProductionPlanOperationSplitsShowsCoverageAndMaterialGap(t *testing.T) {
	items := []productionapp.ProductionPlanItem{{
		ID:             51,
		ProductName:    "熟豆-白巧坚果拼配",
		PlannedG:       20000,
		PlannedOutputG: 20000,
		ProcessSnapshotJSON: `{"operations":[
			{"seq":1,"operation":"烘焙"},
			{"seq":2,"operation":"除石"}
		]}`,
		MaterialSnapshot: `[
			{"material_name":"孟连水洗A","unit":"g","source":"bom","consume_unit":"ratio_pct","ratio_pct":50}
		]`,
	}}
	splits := []productionapp.ProductionPlanOperationSplit{
		{ProductionPlanItemID: 51, OperationSeq: 1, Operation: "烘焙", PlannedQtyG: 12000},
		{ProductionPlanItemID: 51, OperationSeq: 2, Operation: "除石", PlannedQtyG: 12000},
	}

	got := previewProductionPlanOperationSplits(items, splits)
	if got.CoverageSummary.RequiredG != 20000 || got.CoverageSummary.ArrangedG != 12000 || got.CoverageSummary.DiffG != -8000 || got.CoverageSummary.Status != "short" {
		t.Fatalf("coverage summary = %+v, want 20kg required / 12kg arranged / short", got.CoverageSummary)
	}
	if len(got.OperationCoverage) != 2 || got.OperationCoverage[0].Status != "short" || got.OperationCoverage[1].DiffG != -8000 {
		t.Fatalf("operation coverage = %+v, want two short operations", got.OperationCoverage)
	}
	if len(got.MaterialSummary) != 1 {
		t.Fatalf("material summary = %+v, want one row", got.MaterialSummary)
	}
	material := got.MaterialSummary[0]
	if material.Name != "孟连水洗A" || material.RequiredQty != 10000 || material.ArrangedQty != 6000 || material.DiffQty != -4000 || material.Status != "short" {
		t.Fatalf("material preview = %+v, want required 10000 arranged 6000 short", material)
	}
}

func TestPreviewProductionPlanOperationSplitsUsesMinimumOperationCoverageForMaterial(t *testing.T) {
	items := []productionapp.ProductionPlanItem{{
		ID:             52,
		ProductName:    "熟豆-曜石",
		PlannedG:       20000,
		PlannedOutputG: 20000,
		ProcessSnapshotJSON: `{"operations":[
			{"seq":1,"operation":"烘焙"},
			{"seq":2,"operation":"包装"}
		]}`,
		MaterialSnapshot: `[
			{"material_name":"豆袋","unit":"个","source":"packaging"}
		]`,
	}}
	splits := []productionapp.ProductionPlanOperationSplit{
		{ProductionPlanItemID: 52, OperationSeq: 1, Operation: "烘焙", PlannedQtyG: 24000},
		{ProductionPlanItemID: 52, OperationSeq: 2, Operation: "包装", PlannedQtyG: 20000},
	}

	got := previewProductionPlanOperationSplits(items, splits)
	if got.CoverageSummary.RequiredG != 20000 || got.CoverageSummary.ArrangedG != 20000 || got.CoverageSummary.Status != "matched" {
		t.Fatalf("coverage summary = %+v, want matched by minimum operation coverage", got.CoverageSummary)
	}
	if got.OperationCoverage[0].Status != "over" || got.OperationCoverage[1].Status != "matched" {
		t.Fatalf("operation coverage = %+v, want over then matched", got.OperationCoverage)
	}
	if got.MaterialSummary[0].RequiredQty != 20000 || got.MaterialSummary[0].ArrangedQty != 20000 || got.MaterialSummary[0].Status != "matched" {
		t.Fatalf("material preview = %+v, want no duplicate material demand from over-covered first operation", got.MaterialSummary[0])
	}
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
