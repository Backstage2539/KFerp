package bom

import (
	"math"
	"os"
	"strings"
	"testing"
)

func TestPR563BomOperationSnapshotSchemaFreezesPieceCostBasis(t *testing.T) {
	data, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, marker := range []string{
		"cost_method TEXT NOT NULL DEFAULT 'time'",
		"piece_rate_snapshot NUMERIC(14,4) NOT NULL DEFAULT 0",
		"rate_unit_snapshot TEXT NOT NULL DEFAULT ''",
		"ADD COLUMN IF NOT EXISTS cost_method",
		"ADD COLUMN IF NOT EXISTS piece_rate_snapshot",
		"ADD COLUMN IF NOT EXISTS rate_unit_snapshot",
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("BOM operation snapshot schema missing %q", marker)
		}
	}
}

func TestPR563BomOperationSnapshotCostPreservesTimeAndPieceMethods(t *testing.T) {
	timeCost, timeUnit, ok := calculateBomOperationSnapshotCost("time", 0, 120, 20, 15, "kg", "kg")
	if !ok || math.Abs(timeCost-2.6666666667) > 1e-9 || timeUnit != "kg" {
		t.Fatalf("time snapshot = %.10f/%s ok=%v, want 2.6666666667/kg", timeCost, timeUnit, ok)
	}

	pieceCost, pieceUnit, ok := calculateBomOperationSnapshotCost("piece", 0.5, 300, 5, 20, "件", "kg")
	if !ok || pieceCost != 0.5 || pieceUnit != "sales_spec_count" {
		t.Fatalf("piece snapshot = %.4f/%s ok=%v, want 0.5/sales_spec_count", pieceCost, pieceUnit, ok)
	}
}

func TestPR563BomPublisherReadsAndWritesPieceCostSnapshotFields(t *testing.T) {
	data, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	start := strings.Index(src, "func refreshProductionBomVersionOperationCostSnapshotsTx")
	if start < 0 {
		t.Fatal("operation snapshot publisher not found")
	}
	end := strings.Index(src[start:], "\nfunc convertBomOperationBatchQty")
	if end < 0 {
		t.Fatal("operation snapshot publisher end not found")
	}
	fn := src[start : start+end]
	for _, marker := range []string{
		"c.cost_method",
		"c.piece_rate",
		"costMethod",
		"pieceRate",
		"rateUnit",
		"piece_rate_snapshot",
		"rate_unit_snapshot",
	} {
		if !strings.Contains(fn, marker) {
			t.Fatalf("operation snapshot publisher missing %q", marker)
		}
	}
}
