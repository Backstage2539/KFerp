package production

import (
	"errors"
	"os"
	"strings"
	"testing"

	productionapp "orderapp/internal/application/production"
)

func TestUnproducedNeedsResolveCustomerProductRuleTemplates(t *testing.T) {
	b, err := os.ReadFile("unprod_summary.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"customer_product_rule_overrides cpro",
		"customer_product_rule_template_items cpti",
		"customer_product_rule_template_id",
		"p.operation_template_id_override",
		"effective_operation_template_id",
		"NULLIF(cpro.operation_template_id,0)",
		"NULLIF(cpti.operation_template_id,0)",
		"NULLIF(p.operation_template_id_override,0)",
		"NULLIF(subtype_pc.operation_template_id,0)",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("unproduced summary must resolve customer product rule operation templates; missing %q", want)
		}
	}
}

func TestUnproducedNeedsSelectsEffectiveOperationTemplateAlias(t *testing.T) {
	b, err := os.ReadFile("unprod_summary.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	if !strings.Contains(src, "n.effective_operation_template_id") {
		t.Fatalf("unproduced summary query must select the CTE operation template alias used by need rows")
	}
	if strings.Contains(src, "n.operation_template_id") {
		t.Fatalf("unproduced summary query selects n.operation_template_id, but the need CTE exposes effective_operation_template_id")
	}
}

func TestProductionDemandPartRequestTuplesKeepSnapshotSpecBoundToItsOrder(t *testing.T) {
	productIDs, specGs, orderNos := productionDemandPartRequestTuples([]UnprodNeedRow{
		{ProductID: 789, SpecG: 454, OrderNos: "SO-OLD"},
		{ProductID: 789, SpecG: 500, OrderNos: "SO-NEW"},
		{ProductID: 789, SpecG: 454, OrderNos: "SO-OLD"},
	})
	if len(productIDs) != 2 || len(specGs) != 2 || len(orderNos) != 2 {
		t.Fatalf("request tuples=%v/%v/%v, want two unique product/spec/order tuples", productIDs, specGs, orderNos)
	}
	if productIDs[0] != 789 || specGs[0] != 454 || orderNos[0] != "SO-OLD" {
		t.Fatalf("first request tuple=%d/%d/%s, want 789/454/SO-OLD", productIDs[0], specGs[0], orderNos[0])
	}
	if productIDs[1] != 789 || specGs[1] != 500 || orderNos[1] != "SO-NEW" {
		t.Fatalf("second request tuple=%d/%d/%s, want 789/500/SO-NEW", productIDs[1], specGs[1], orderNos[1])
	}
}

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

	got := buildRoastPlanRows(rows, nil, map[int64]float64{89: 1}, nil)

	if len(got) != 1 || got[0].OperationTemplateID != 22 {
		t.Fatalf("roast plan operation template = %+v, want 22", got)
	}
}

func TestNormalizeBomComponentTypeAcceptsProductionBomProductComponents(t *testing.T) {
	for _, input := range []string{"product", "finished_product"} {
		if got := normalizeBomComponentType(input); got != "finished_product" {
			t.Fatalf("normalizeBomComponentType(%q) = %q, want finished_product", input, got)
		}
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

	got := buildRoastPlanMaterialRatios(rows, map[string][]planBomItem{})

	if len(got) != 1 {
		t.Fatalf("material ratios = %+v, want one row", got)
	}
	if got[0].MaterialName != "速溶咖啡" || got[0].MaterialUnit != "g" || got[0].RatioPct != 100 {
		t.Fatalf("instant coffee material ratio = %+v", got[0])
	}
}

func TestProductionPlanBomMaterialLossRateUsesResolvedVersionMetadata(t *testing.T) {
	if got := productionPlanBomMaterialLossRate(latestUsableBomRoute{YieldRate: 0.8, BomMaterialLossRate: 0.2}); got != 0.2 {
		t.Fatalf("productionPlanBomMaterialLossRate() = %.4f, want 0.2000", got)
	}
	if got := productionPlanBomMaterialLossRate(latestUsableBomRoute{YieldRate: 0.8}); got != 0 {
		t.Fatalf("legacy yield must not imply the BOM version loss checkbox, got %.4f", got)
	}
}

func TestProductionBomSummaryOnlyTreatsTypedConfigurationErrorsAsRowWarnings(t *testing.T) {
	if !isProductionBomConfigurationError(productionBomConfigurationErrorf("product BOM not configured: 测试商品")) {
		t.Fatal("typed BOM configuration error must remain a row-level warning")
	}
	if isProductionBomConfigurationError(errors.New("connection interrupted")) {
		t.Fatal("database and connection errors must propagate instead of becoming BOM configuration warnings")
	}
}

func TestCalcProducePlanMaterialsUsesDictionaryGramQuantities(t *testing.T) {
	rows := []productionapp.UnprodNeedRow{{
		ProductID: 556,
		Product:   "熟豆-白巧坚果拼配",
		SpecG:     454,
		GapG:      908,
	}}
	bomMap := map[string][]planBomItem{
		producePlanDemandKey(rows[0].ProductID, rows[0].ParentProductID, rows[0].SpecG, rows[0].SalesSpecSnapshotJSON): {
			{MaterialName: "哥伦比亚EP", MaterialUnit: "g", ConsumeUnit: "g", QtyPerUnit: 114},
			{MaterialName: "孟连水洗A", MaterialUnit: "g", ConsumeUnit: "g", QtyPerUnit: 284},
			{MaterialName: "生豆-巴布亚之光-石光", MaterialUnit: "g", ConsumeUnit: "g", QtyPerUnit: 171},
		},
	}

	got := calcProducePlanMaterialsFromFinalInputs(rows, map[string]int64{
		producePlanDemandKey(rows[0].ProductID, rows[0].ParentProductID, rows[0].SpecG, rows[0].SalesSpecSnapshotJSON): 1135,
	}, bomMap, defaultPlanParams())

	assertMaterialNeed(t, got, "哥伦比亚EP", 228, "g")
	assertMaterialNeed(t, got, "孟连水洗A", 568, "g")
	assertMaterialNeed(t, got, "生豆-巴布亚之光-石光", 342, "g")
}

func TestPlanSummaryBomItemsUseDefaultBomLatestUsableVersion(t *testing.T) {
	srcBytes, err := os.ReadFile("plan_queries.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(srcBytes)
	for _, want := range []string{
		"EXISTS (SELECT 1 FROM %s.production_bom_version_items item WHERE item.version_id=v.id)",
		"CASE WHEN pbom.id=COALESCE(NULLIF(ppc.production_bom_id,0), pbb.bom_id, 0)",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("plan summary BOM item query must use current usable default BOM priority; missing %q", want)
		}
	}
	if strings.Contains(src, "CASE WHEN COALESCE(NULLIF(ppc.production_bom_version_id,0), pbb.bom_version_id, 0)>0") {
		t.Fatalf("plan summary BOM item query must not prioritize stale production_bom_version_id")
	}
}

func TestMergeMaterialAvailabilityFallsBackToNonWeightPurchaseQuantity(t *testing.T) {
	materials := []productionapp.MaterialNeed{{Name: "豆袋", Qty: 21, Unit: "个"}}

	got := mergeMaterialAvailability(materials, []productionapp.MaterialPlanRow{{
		MaterialName:        "豆袋",
		Unit:                "个",
		RequiredUnits:       21,
		AvailableG:          0,
		RawG:                0,
		ShortageG:           0,
		PurchaseSuggestionG: 0,
	}})

	if len(got) != 1 || got[0].PurchaseSuggestionG != 21 || got[0].ShortageG != 21 {
		t.Fatalf("non-weight availability = %+v, want purchase and shortage 21个", got)
	}

	got = mergeMaterialAvailability([]productionapp.MaterialNeed{{Name: "豆袋", Qty: 21, Unit: "个"}}, nil)
	if len(got) != 1 || got[0].PurchaseSuggestionG != 21 || got[0].ShortageG != 21 {
		t.Fatalf("missing non-weight availability = %+v, want purchase and shortage 21个", got)
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
