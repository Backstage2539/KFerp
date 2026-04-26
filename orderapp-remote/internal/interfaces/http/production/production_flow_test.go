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

func TestBuildProducePlanDisplayRowsUsesDefaultInputG(t *testing.T) {
	rows := []UnprodNeedRow{
		{ProductID: 11, Product: "橘皮乌龙", SpecG: 227, GapG: 2270},
	}
	got := buildProducePlanDisplayRows(rows, map[int64]float64{11: 0.82}, nil)
	if len(got) != 1 {
		t.Fatalf("buildProducePlanDisplayRows() rows = %d, want 1", len(got))
	}
	if got[0].BomYieldRate != 0.82 {
		t.Fatalf("buildProducePlanDisplayRows() bom_yield_rate = %.4f, want 0.82", got[0].BomYieldRate)
	}
	if got[0].InputG != 2769 {
		t.Fatalf("buildProducePlanDisplayRows() input_g = %d, want 2769", got[0].InputG)
	}
}

func TestBuildProducePlanDisplayRowsFallsBackToPointEight(t *testing.T) {
	rows := []UnprodNeedRow{
		{ProductID: 12, Product: "晨曦-娜伊", SpecG: 227, GapG: 2270},
	}
	got := buildProducePlanDisplayRows(rows, nil, nil)
	if len(got) != 1 {
		t.Fatalf("buildProducePlanDisplayRows() rows = %d, want 1", len(got))
	}
	if got[0].BomYieldRate != 0.8 {
		t.Fatalf("buildProducePlanDisplayRows() bom_yield_rate = %.4f, want 0.8", got[0].BomYieldRate)
	}
	if got[0].InputG != 2838 {
		t.Fatalf("buildProducePlanDisplayRows() input_g = %d, want 2838", got[0].InputG)
	}
}

func TestVueShellProducePlanIsNoLongerTemplateDriven(t *testing.T) {
	body, err := os.ReadFile("frontend-vue-shell/src/App.vue")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, needle := range []string{"ProducePlanView", "producePlan: ProducePlanView", "producePlan: { title: '生产计划/开始生产'"} {
		if !strings.Contains(content, needle) {
			t.Fatalf("App.vue missing %q", needle)
		}
	}
	body, err = os.ReadFile("internal/interfaces/http/support/static_frontend_routes.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `target := "/vue-shell?view=producePlan"`) {
		t.Fatal("static_frontend_routes.go missing producePlan redirect")
	}
}

func TestMarshalMaterialConsumptionSummary(t *testing.T) {
	got, err := marshalMaterialConsumptionSummary([]materialConsumptionSummaryItem{
		{MaterialID: 1, MaterialName: "卡蒂姆水洗", Unit: "g", DeductG: 1200},
		{MaterialID: 9, MaterialName: "豆袋", Unit: "个", DeductUnits: 8},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, needle := range []string{`"material_id":1`, `"material_name":"卡蒂姆水洗"`, `"deduct_g":1200`, `"material_name":"豆袋"`, `"deduct_units":8`} {
		if !strings.Contains(text, needle) {
			t.Fatalf("material summary json missing %q in %s", needle, text)
		}
	}
}
