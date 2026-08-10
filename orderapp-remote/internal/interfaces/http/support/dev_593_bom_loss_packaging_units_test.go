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
			"REV-593-BOM-LOSS-PACKAGING-UNITS",
		},
		filepath.Join("..", "REQUIREMENTS.md"): {
			"PR-593-BOM-LOSS-PACKAGING-UNITS",
			"损耗原料",
			"非损耗物料（含包材）",
		},
		filepath.Join("..", "ACCEPTANCE_TESTS.md"): {
			"PR-593-BOM-LOSS-PACKAGING-UNITS",
			"固定用量包材",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"损耗原料",
			"非损耗物料（含包材）",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"): {
			"selectedMaterialLossZone",
			"损耗原料",
			"非损耗物料（含包材）",
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
