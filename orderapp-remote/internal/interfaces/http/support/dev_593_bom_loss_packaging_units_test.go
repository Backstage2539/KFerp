package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev593BomLossPackagingUnitsContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-593-BOM-LOSS-PACKAGING-UNITS",
			"DEV-593-BOM-MIXED-CONSUMPTION",
			"DEV-593-BOM-LOSS-ZONES",
			"DEV-593-BOM-PACKAGING-COST",
			"REV-593-BOM-LOSS-PACKAGING-UNITS",
		},
		filepath.Join("..", "REQUIREMENTS.md"): {
			"PR-593-BOM-LOSS-PACKAGING-UNITS",
			"有损耗的配方",
			"无损耗的配方",
		},
		filepath.Join("..", "ACCEPTANCE_TESTS.md"): {
			"PR-593-BOM-LOSS-PACKAGING-UNITS",
			"固定用量包材",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-593-BOM-LOSS-PACKAGING-UNITS",
			"固定用量包材",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"有损耗的配方",
			"无损耗的配方",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"): {
			"selectedMaterialLossZone",
			"componentInventoryConsumeUnitOptions",
			"有损耗的配方",
			"无损耗的配方",
			"损耗只作用于物料的比例 % 行",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-593 marker %q", rel, want)
			}
		}
	}

	costingSource := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "costing", "repository.go")))
	for _, want := range []string{"NULLIF(m.unit,'')", "NOT IN ('ratio_pct','g_per_bag')"} {
		if !strings.Contains(costingSource, want) {
			t.Fatalf("costing repository missing fixed packaging cost marker %q", want)
		}
	}
	if strings.Contains(costingSource, "m.cost_unit") {
		t.Fatal("PR-599 supersedes cost_unit-first costing; fixed packaging must use the unified material inventory unit")
	}

	for _, rel := range []string{
		filepath.Join("internal", "application", "bom", "service.go"),
		filepath.Join("internal", "infrastructure", "postgres", "bom", "repository.go"),
		filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		if strings.Contains(src, "原料损耗比开启后，组件消耗单位只能使用比例") || strings.Contains(src, "开启后组件消耗单位只能使用比例") {
			t.Fatalf("%s still enforces the obsolete ratio-only BOM restriction", rel)
		}
	}
}
