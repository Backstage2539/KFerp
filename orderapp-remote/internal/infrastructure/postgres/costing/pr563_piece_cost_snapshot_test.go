package costing

import (
	"os"
	"strings"
	"testing"
)

func TestPR563CostingRepositoryReadsFrozenPieceCostWithoutParsingSpecName(t *testing.T) {
	data, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	start := strings.Index(src, "opRows, err := r.pool.Query")
	if start < 0 {
		t.Fatal("BOM operation snapshot query not found")
	}
	end := strings.Index(src[start:], "legacyOpRows, err :=")
	if end < 0 {
		t.Fatal("BOM operation snapshot query end not found")
	}
	fn := src[start : start+end]
	for _, marker := range []string{
		"cost_method",
		"piece_rate_snapshot",
		"rate_unit_snapshot",
		"&row.CostMethod",
		"&row.PieceRate",
		"&row.RateUnit",
	} {
		if !strings.Contains(fn, marker) {
			t.Fatalf("pricing trial snapshot loader missing %q", marker)
		}
	}
	for _, forbidden := range []string{"regexp", "spec_label", "sku_name"} {
		if strings.Contains(strings.ToLower(fn), forbidden) {
			t.Fatalf("piece-cost conversion must not parse SKU/spec names; found %q", forbidden)
		}
	}
}

func TestPR563ResolvedBomAggregateExcludesPieceRowsFromPerInventoryUnitCost(t *testing.T) {
	data, err := os.ReadFile("production_bom_cost.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	if !strings.Contains(src, "COALESCE(NULLIF(oc.cost_method,''),'time')='time'") {
		t.Fatal("resolved BOM aggregate must only sum time-based per-output operation snapshots")
	}
}
