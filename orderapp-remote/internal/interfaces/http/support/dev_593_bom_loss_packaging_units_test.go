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
			"PR-600-BOM-SPEC-GROUP-MANUFACTURE-ONLY-SEMI-FINISHED",
			"PR-600 取代 PR-593/594 的双区域与同配方混用口径",
		},
		filepath.Join("..", "ACCEPTANCE_TESTS.md"): {
			"PR-593-BOM-LOSS-PACKAGING-UNITS",
			"固定用量包材",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-593-BOM-LOSS-PACKAGING-UNITS",
			"PR-600",
			"页面只显示一个组件列表",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"每个配方只允许一种模式",
			"固定模式可录入 `0.227kg 熟豆 + 1个袋子`",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"): {
			"recipeConsumeMode",
			"同一配方不能混合使用比例 % 和固定用量",
			"原料损耗比开启后，所有组件消耗单位必须为比例 %",
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

	serviceSource := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "bom", "service.go")))
	for _, want := range []string{"ValidateProductionBomRecipeMode", "ratio and fixed consume units cannot be mixed", "material_loss_rate requires all components to use material ratio_pct"} {
		if !strings.Contains(serviceSource, want) {
			t.Fatalf("bom service missing PR-600 recipe-mode validation %q", want)
		}
	}
	repositorySource := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "bom", "repository.go")))
	if !strings.Contains(repositorySource, "ValidateProductionBomRecipeMode") {
		t.Fatal("bom repository publish/save path missing PR-600 recipe-mode validation")
	}
	bomSource := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue")))
	for _, obsolete := range []string{"selectedMaterialLossZone", "有损耗的配方", "无损耗的配方"} {
		if strings.Contains(bomSource, obsolete) {
			t.Fatalf("BomView.vue restored superseded PR-593 recipe zone %q", obsolete)
		}
	}
}
