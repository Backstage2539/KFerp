package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev553ProductionPlanParentBomUnitConversionContracts(t *testing.T) {
	checks := map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-553-PRODUCTION-PLAN-PARENT-BOM-UNIT-CONVERSION",
			"DEV-553-ORDER-CONVERSION-SNAPSHOT",
			"DEV-553-PARENT-BOM-RESOLUTION",
			"DEV-553-PRODUCTION-FREEZE",
			"DEV-553-DOCS-DATA-DELIVERY",
			"REV-553-PRODUCTION-PLAN-PARENT-BOM-UNIT-CONVERSION",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "unprod_summary.go"): {
			"production_quantity_snapshot",
			"inventory_qty_per_sales_unit",
			"parent_product_id",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "production_plan.go"): {
			"bom_source_product_id",
			"sales_spec_count",
			"planned_inventory_qty",
			"bom_inherited",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "produce-plan.js"): {
			"productionPlanItemQuantitySummary",
			"继承父商品BOM",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-553-PRODUCTION-PLAN-PARENT-BOM-UNIT-CONVERSION",
			"具体 SKU BOM 优先",
			"父商品 BOM",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-553-PRODUCTION-PLAN-PARENT-BOM-UNIT-CONVERSION",
			"4件",
			"0.454Kg/件",
			"1.816Kg",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"具体 SKU BOM 优先",
			"继承父商品 BOM",
			"规格名称只用于显示",
		},
	}
	for rel, wants := range checks {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-553 marker %q", rel, want)
			}
		}
	}
}

func TestDev553ProductionDemandDoesNotParseWeightFromSpecLabel(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "production", "unprod_summary.go")))
	if strings.Contains(src, "regexp_replace(COALESCE(oi.spec") {
		t.Fatal("production demand must use the frozen conversion snapshot, not parse digits from oi.spec")
	}
}
