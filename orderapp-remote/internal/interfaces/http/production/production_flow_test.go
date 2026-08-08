package production

import (
	"os"
	"strings"
	"testing"
)

func TestPlannedFinishedInventoryAddition(t *testing.T) {
	got := plannedFinishedInventoryAddition(227, 500)
	if got.Units != 2 || got.LooseG != 46 {
		t.Fatalf("plannedFinishedInventoryAddition() = %d units + %dg, want 2 units + 46g", got.Units, got.LooseG)
	}
}

func TestNormalizeFinishedInventoryAddition(t *testing.T) {
	got, err := normalizeFinishedInventoryAddition(227, 1, 300)
	if err != nil {
		t.Fatal(err)
	}
	if got.Units != 2 || got.LooseG != 73 {
		t.Fatalf("normalizeFinishedInventoryAddition() = %d units + %dg, want 2 units + 73g", got.Units, got.LooseG)
	}
}

func TestDefaultProductionInputGUsesBomYield(t *testing.T) {
	got := defaultProductionInputG(2270, 0.82)
	if got != 2769 {
		t.Fatalf("defaultProductionInputG() = %d, want 2769", got)
	}
}

func TestDefaultProductionInputGFallsBackToPointEight(t *testing.T) {
	got := defaultProductionInputG(2270, 0)
	if got != 2838 {
		t.Fatalf("defaultProductionInputG() = %d, want 2838", got)
	}
}

func TestActualYieldRateFromFinishedOutput(t *testing.T) {
	got, err := actualYieldRate(227, 8, 91, 2500)
	if err != nil {
		t.Fatal(err)
	}
	if got != 0.7628 {
		t.Fatalf("actualYieldRate() = %.4f, want 0.7628", got)
	}
}

func TestPlannedFinishedInventoryByInputUsesYield(t *testing.T) {
	got := plannedFinishedInventoryByInput(454, 600, 0.8)
	if got.Units != 1 || got.LooseG != 26 {
		t.Fatalf("plannedFinishedInventoryByInput() = %d units + %dg, want 1 unit + 26g", got.Units, got.LooseG)
	}
}

func TestPlannedFinishedInventoryByInputAvoidsFloatUndercount(t *testing.T) {
	got := plannedFinishedInventoryByInput(227, 600, 0.82)
	if got.Units != 2 || got.LooseG != 38 {
		t.Fatalf("plannedFinishedInventoryByInput() = %d units + %dg, want 2 units + 38g", got.Units, got.LooseG)
	}
}

func TestRunningInventoryPlanPrefersInputAndYield(t *testing.T) {
	got := runningInventoryPlan(1000, 1000, 2000, 0.8)
	if got.Units != 1 || got.LooseG != 600 {
		t.Fatalf("runningInventoryPlan() = %d units + %dg, want 1 unit + 600g", got.Units, got.LooseG)
	}
}

func TestRunningInventoryPlanFallsBackToNeed(t *testing.T) {
	got := runningInventoryPlan(1000, 1000, 0, 0.8)
	if got.Units != 1 || got.LooseG != 0 {
		t.Fatalf("runningInventoryPlan() fallback = %d units + %dg, want 1 unit + 0g", got.Units, got.LooseG)
	}
}

func TestRestoreAllocatedInventory(t *testing.T) {
	got, err := restoreAllocatedInventory(454, InvQty{Units: 1, LooseG: 10}, 500)
	if err != nil {
		t.Fatal(err)
	}
	if got.Units != 2 || got.LooseG != 56 {
		t.Fatalf("restoreAllocatedInventory() = %d units + %dg, want 2 units + 56g", got.Units, got.LooseG)
	}
}

func TestBuildProducePlanDisplayRowsIgnoresLegacyYield(t *testing.T) {
	rows := []UnprodNeedRow{
		{ProductID: 11, Product: "橘皮乌龙", SpecG: 227, GapG: 2270},
	}
	got := buildProducePlanDisplayRows(rows, map[int64]float64{11: 0.82}, nil)
	if len(got) != 1 {
		t.Fatalf("buildProducePlanDisplayRows() rows = %d, want 1", len(got))
	}
	if got[0].BomYieldRate != 1 {
		t.Fatalf("buildProducePlanDisplayRows() bom_yield_rate = %.4f, want compatibility value 1", got[0].BomYieldRate)
	}
	if got[0].InputG != 2270 {
		t.Fatalf("buildProducePlanDisplayRows() input_g = %d, want target 2270 without legacy yield inflation", got[0].InputG)
	}
}

func TestBuildProducePlanDisplayRowsDefaultsToNoLoss(t *testing.T) {
	rows := []UnprodNeedRow{
		{ProductID: 12, Product: "晨曦-娜伊", SpecG: 227, GapG: 2270},
	}
	got := buildProducePlanDisplayRows(rows, nil, nil)
	if len(got) != 1 {
		t.Fatalf("buildProducePlanDisplayRows() rows = %d, want 1", len(got))
	}
	if got[0].BomYieldRate != 1 {
		t.Fatalf("buildProducePlanDisplayRows() bom_yield_rate = %.4f, want compatibility value 1", got[0].BomYieldRate)
	}
	if got[0].InputG != 2270 {
		t.Fatalf("buildProducePlanDisplayRows() input_g = %d, want target 2270", got[0].InputG)
	}
}

func TestBuildRoastPlanRowsIgnoresLegacyYield(t *testing.T) {
	rows := []UnprodNeedRow{{ProductID: 12, Product: "曲奇拼配", SpecG: 1000, GapG: 1000}}
	got := buildRoastPlanRows(rows, nil, map[int64]float64{12: 0.25})
	if len(got) != 1 || got[0].FinalInputG != 1000 || got[0].YieldRate != 1 {
		t.Fatalf("roast plan = %+v, want 1000g target and compatibility rate 1", got)
	}
}

func TestCalcRoastSplitsIgnoresLegacyYield(t *testing.T) {
	rows := []UnprodNeedRow{{ProductID: 12, Product: "曲奇拼配", SpecG: 1000, GapG: 1000}}
	got := calcRoastSplits(rows, nil, 0.25)
	if len(got) != 1 || got[0].TotalKg != "1" || got[0].YieldPctStr != "100%" {
		t.Fatalf("roast splits = %+v, want 1kg target and compatibility rate 100%%", got)
	}
}

func TestCurrentPlanCompatibilityPayloadHasNoLegacyExpectedLossCopy(t *testing.T) {
	src, err := os.ReadFile("internal/interfaces/http/production/roast_split.go")
	if err != nil {
		t.Fatal(err)
	}
	content := string(src)
	for _, forbidden := range []string{"损耗比展示", "预期损耗", "预计损耗", "预期产出率"} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("current roast split payload must not describe the compatibility field as legacy overall loss; found %q", forbidden)
		}
	}
	rows := []UnprodNeedRow{{ProductID: 12, Product: "曲奇拼配", SpecG: 1000, GapG: 1000}}
	got := calcRoastSplits(rows, nil, 0.01)
	if len(got) != 1 || got[0].YieldPctStr != "100%" {
		t.Fatalf("current roast split compatibility payload = %+v, want fixed 100%%", got)
	}
	p := defaultProducePlanParams()
	if p.YieldRate != 1 {
		t.Fatalf("current no-BOM plan compatibility yield_rate = %.4f, want 1", p.YieldRate)
	}
	p.YieldRate = 0.01
	materials := calcNoBomProducePlanMaterials(rows[0], p)
	if len(materials) == 0 || materials[0].Qty != 1000 {
		t.Fatalf("current no-BOM plan must ignore legacy overall yield; materials=%+v", materials)
	}
}

func TestVueShellProducePlanIsNoLongerTemplateDriven(t *testing.T) {
	body, err := os.ReadFile("frontend-vue-shell/src/App.vue")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, needle := range []string{"ProducePlanView", "producePlan: ProducePlanView"} {
		if !strings.Contains(content, needle) {
			t.Fatalf("App.vue missing %q", needle)
		}
	}
	body, err = os.ReadFile("frontend-vue-shell/src/lib/menu-ia.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "producePlan") || !strings.Contains(string(body), "生产计划") {
		t.Fatal("menu-ia.js missing producePlan title")
	}
	body, err = os.ReadFile("internal/interfaces/http/support/static_frontend_routes.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `target := "/vue-shell?view=producePlan"`) {
		t.Fatal("static_frontend_routes.go missing producePlan redirect")
	}
}
