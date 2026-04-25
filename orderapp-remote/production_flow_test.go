package main

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

func TestBuildProducePlanDisplayRowsUsesDefaultInputG(t *testing.T) {
	rows := []UnprodNeedRow{
		{ProductID: 11, Product: "橘皮乌龙", SpecG: 227, GapG: 2270},
	}
	got := buildProducePlanDisplayRows(rows, map[int64]float64{11: 0.82})
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
	got := buildProducePlanDisplayRows(rows, nil)
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

func TestProduceRunningTemplateContainsFinishedInventoryFields(t *testing.T) {
	body, err := os.ReadFile("templates/produce_running.html")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, needle := range []string{"finished_units", "finished_loose_g", "烘焙剩余请填入散装余量"} {
		if !strings.Contains(content, needle) {
			t.Fatalf("produce_running.html missing %q", needle)
		}
	}
}

func TestUnproducedTemplateContainsInputGFields(t *testing.T) {
	body, err := os.ReadFile("templates/unprod_summary.html")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, needle := range []string{"投料数(g)", "input_g_", "startProduction()"} {
		if !strings.Contains(content, needle) {
			t.Fatalf("unprod_summary.html missing %q", needle)
		}
	}
}
