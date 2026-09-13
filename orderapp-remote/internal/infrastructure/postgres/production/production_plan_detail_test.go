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

func TestAggregateProductionPlanMaterialSummaryDoesNotApplyBomLossTwice(t *testing.T) {
	got := aggregateProductionPlanMaterialSummary([]productionapp.ProductionPlanItem{{
		ID:             557,
		SpecG:          454,
		PlannedG:       7751,
		PlannedOutputG: 6356,
		MaterialSnapshot: `[
			{"material_name":"如目达摩生豆","unit":"g","source":"bom","consume_unit":"ratio_pct","ratio_pct":100,"material_loss_rate":0.18,"input_includes_material_loss":true}
		]`,
	}})

	assertProductionPlanMaterial(t, got, "如目达摩生豆", 7751, "g")
}

func TestProductionPlanOutputUnitsUsesFrozenSalesSpecCount(t *testing.T) {
	item := productionapp.ProductionPlanItem{
		OutputType:          "product",
		SalesSpecCount:      20,
		PlannedInventoryQty: 20,
		InventoryUnit:       "袋",
	}
	if got := productionPlanOutputUnits(item); got != 20 {
		t.Fatalf("productionPlanOutputUnits() = %d, want frozen 20 bags", got)
	}
}

func TestAggregateProductionPlanMaterialSummaryUsesFrozenSalesCounts(t *testing.T) {
	items := []productionapp.ProductionPlanItem{
		{ID: 1, OutputType: "product", SalesSpecCount: 10, PlannedInventoryQty: 10, InventoryUnit: "袋", MaterialSnapshot: `[{"material_id":91,"material_name":"227g咖啡袋","unit":"条","source":"packaging"}]`},
		{ID: 2, OutputType: "product", SalesSpecCount: 8, PlannedInventoryQty: 8, InventoryUnit: "袋", MaterialSnapshot: `[{"material_id":91,"material_name":"227g咖啡袋","unit":"条","source":"packaging"}]`},
		{ID: 3, OutputType: "product", SalesSpecCount: 2, PlannedInventoryQty: 2, InventoryUnit: "袋", MaterialSnapshot: `[{"material_id":91,"material_name":"227g咖啡袋","unit":"条","source":"packaging"}]`},
	}

	got := aggregateProductionPlanMaterialSummary(items)
	assertProductionPlanMaterial(t, got, "227g咖啡袋", 20, "条")
}

func TestFreezeManufacturingOutputMaterialSnapshotPreventsDoubleLoss(t *testing.T) {
	raw := `[{"material_id":7,"material_name":"生豆","unit":"g","source":"bom","consume_unit":"ratio_pct","ratio_pct":100,"material_loss_rate":0.18,"loss_calculation_mode":"yield_denominator"}]`
	frozen, err := freezeManufacturingOutputMaterialSnapshot(raw)
	if err != nil {
		t.Fatalf("freezeManufacturingOutputMaterialSnapshot() error = %v", err)
	}
	item := productionapp.ProductionPlanItem{
		OutputType:       "material",
		OutputQty:        4.54,
		OutputUnit:       "kg",
		PlannedG:         5537,
		PlannedOutputG:   4540,
		MaterialSnapshot: frozen,
	}
	needs, err := productionPlanItemConsumptionNeeds(item)
	if err != nil {
		t.Fatalf("productionPlanItemConsumptionNeeds() error = %v", err)
	}
	if len(needs) != 1 || needs[0].DeductG != 5537 {
		t.Fatalf("frozen upstream needs = %+v, want one 5537g requirement without applying 18%% twice", needs)
	}
}

func TestProductionPlanReadinessReportsAllBlockingSections(t *testing.T) {
	detail := productionapp.ProductionPlanDetail{
		ID:     41,
		Status: "draft",
		Items: []productionapp.ProductionPlanItem{{
			ID: 51, ProductName: "初晓商品", SalesSpecCount: 20,
			ProcessSnapshotJSON: `{"operations":[{"seq":1,"operation":"包装"}]}`,
		}},
		ComponentSources: []productionapp.ProductionPlanComponentSource{{
			ID: 61, ProductionPlanItemID: 51, ComponentName: "咖啡袋", RequiredUnits: 20,
		}},
		SupplyGaps: []productionapp.ProductionPlanSupplyGap{{ID: 71, ProductionPlanItemID: 51, ItemName: "标签", Status: "unresolved"}},
	}

	got := productionPlanReadiness(detail)
	if got.CanSubmit || got.BlockingCount != 3 {
		t.Fatalf("readiness = %+v, want three blocking issues", got)
	}
	wantCodes := map[string]bool{"component_source_missing": true, "operation_split_missing": true, "supply_gap": true}
	for _, issue := range got.Issues {
		delete(wantCodes, issue.Code)
	}
	if len(wantCodes) != 0 {
		t.Fatalf("readiness issues = %+v, missing codes %+v", got.Issues, wantCodes)
	}
}

func TestProductionPlanReadinessRequiresRefreshForLegacyUpstreamLossSnapshot(t *testing.T) {
	detail := productionapp.ProductionPlanDetail{
		ID: 109, Status: "draft",
		Items: []productionapp.ProductionPlanItem{{
			ID: 51, OutputType: "material", OutputName: "初晓熟豆", OutputQty: 4.54, OutputUnit: "kg", PlannedG: 5640, PlannedOutputG: 4540,
			MaterialSnapshot:    `[{"material_id":7,"material_name":"咖啡生豆","unit":"kg","ratio_pct":100,"material_loss_rate":0.18}]`,
			ProcessSnapshotJSON: `{"operations":[]}`,
		}},
	}
	readiness := productionPlanReadiness(detail)
	if readiness.CanSubmit || readiness.BlockingCount != 1 || readiness.Issues[0].Code != "legacy_loss_snapshot" {
		t.Fatalf("readiness = %+v, want explicit supply refresh blocker", readiness)
	}
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

func TestPreviewProductionPlanOperationSplitsReturnsNativeTaskQuantities(t *testing.T) {
	items := []productionapp.ProductionPlanItem{
		{
			ID: 51, OutputType: "material", OutputName: "初晓熟豆", OutputQty: 4.54, OutputUnit: "kg",
			PlannedG: 5640, PlannedOutputG: 4540,
			ProcessSnapshotJSON: `{"operations":[{"seq":1,"operation_id":701,"operation":"滚筒烘焙"}]}`,
		},
		{
			ID: 61, ProductName: "初晓-商品", SpecG: 227, SalesSpecCount: 20, PlannedG: 5640, PlannedOutputG: 4540,
			SalesSpecSnapshotJSON: `{"spec_label":"227g","sales_unit":"袋"}`,
			ProcessSnapshotJSON:   `{"operations":[{"seq":2,"operation_id":702,"operation":"手工装袋"}]}`,
		},
	}
	splits := []productionapp.ProductionPlanOperationSplit{
		{ProductionPlanItemID: 51, OperationSeq: 1, OperationID: 701, Operation: "滚筒烘焙", BatchSizeUnit: "kg", PlannedQty: 5.64, PlannedQtyG: 5640},
		{ProductionPlanItemID: 61, OperationSeq: 2, OperationID: 702, Operation: "手工装袋", BatchSizeUnit: "件", PlannedQty: 18, PlannedQtyG: 4086},
	}

	got := previewProductionPlanOperationSplits(items, splits)
	if len(got.OperationCoverage) != 2 {
		t.Fatalf("operation coverage = %+v, want two rows", got.OperationCoverage)
	}
	roast := got.OperationCoverage[0]
	if roast.RequiredQty != 5.64 || roast.ArrangedQty != 5.64 || roast.DiffQty != 0 || roast.Unit != "kg" || roast.Status != "matched" {
		t.Fatalf("roast native coverage = %+v, want 5.64kg matched", roast)
	}
	pack := got.OperationCoverage[1]
	if pack.RequiredQty != 20 || pack.ArrangedQty != 18 || pack.DiffQty != -2 || pack.Unit != "袋" || pack.Status != "short" {
		t.Fatalf("package native coverage = %+v, want 20 bags required / 18 arranged", pack)
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
