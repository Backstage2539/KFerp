package support

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDev599MaterialInventoryPriceUnitUnificationContracts(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, row := range []struct {
		table, code, status, assignee string
	}{
		{"req_product", "PR-599-MATERIAL-INVENTORY-PRICE-UNIT-UNIFICATION", "review", "VA"},
		{"req_dev", "DEV-599-MATERIAL-UNIT-INVARIANT", "done", "Codex"},
		{"req_dev", "DEV-599-LEGACY-WEIGHT-MIGRATION", "done", "Codex"},
		{"req_dev", "DEV-599-BOM-COST-CONVERSION", "done", "Codex"},
		{"req_dev", "DEV-599-VUE-DOCS-DEVELOPMENT-DELIVERY", "done", "Codex"},
		{"req_review", "REV-599-MATERIAL-INVENTORY-PRICE-UNIT-UNIFICATION", "todo", "VA"},
	} {
		requireDev599SeedRow(t, reqStore, row.table, row.code, row.status, row.assignee)
	}

	materials := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "MaterialsView.vue")))
	for _, forbidden := range []string{
		"采购价与成本单价单位",
		"data-field=\"cost_unit\"",
		"materialCostUnitLocked",
		"defaultMaterialCostUnit",
		"重量物料固定按元/kg",
	} {
		if strings.Contains(materials, forbidden) {
			t.Fatalf("MaterialsView.vue retains superseded independent cost-unit marker %q", forbidden)
		}
	}
	for _, want := range []string{
		"重量物料库存统一使用 kg；BOM 配方仍可按 g 录入并自动换算",
		"采购价、批次单位成本和 BOM 成本试算均按库存单位计价",
		"最近采购入库价（元/{{ draft.unit }}）",
		"cost_unit: draftMode.value ? draft.value.unit : (selected.value?.unit || draft.value.unit)",
		"isCanonicalMaterialInventoryUnit",
	} {
		if !strings.Contains(materials, want) {
			t.Fatalf("MaterialsView.vue missing unified material-unit marker %q", want)
		}
	}
	for _, rel := range []string{
		filepath.Join("frontend-vue-shell", "src", "views", "PurchaseView.vue"),
		filepath.Join("frontend-vue-shell", "src", "views", "MaterialReceiptsView.vue"),
		filepath.Join("frontend-vue-shell", "src", "views", "StockAdjustmentsView.vue"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, forbidden := range []string{"CostUnit", "cost_unit", "materialCostUnit", "selectedMaterialCostUnitLabel", "selectedPurchaseMaterialCostUnit"} {
			if strings.Contains(src, forbidden) {
				t.Fatalf("%s retains superseded independent cost-unit marker %q", rel, forbidden)
			}
		}
	}

	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-599-MATERIAL-INVENTORY-PRICE-UNIT-UNIFICATION", "cost_unit=unit", "227g", "0.227kg", "65.376元",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-599-MATERIAL-INVENTORY-PRICE-UNIT-UNIFICATION", "g/kg", "kg/kg", "规范克库存", "历史快照",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-599", "采购价单位就是库存单位", "重量物料主档只能选择 `kg`", "兼容字段", "历史重量物料", "操作日志",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-599", "227g", "0.227kg", "65.376元", "不能解释为元/g",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-599", "建议 1.974kg", "60kg", "净用量 ÷ 0.8",
		},
		filepath.Join("docs", "OP_MANUAL_STOCK.md"): {
			"PR-599", "重量物料主档统一按 kg", "规范克余额",
		},
		filepath.Join("docs", "acceptance", "2026-07-16-material-cost-unit-loss.md"): {
			"历史证据说明", "PR-599", "重量物料主档统一 kg",
		},
		filepath.Join("docs", "acceptance", "2026-08-14-material-inventory-price-unit-unification.md"): {
			"PR-599", "## RED 证据", "## GREEN 证据", "真实 PostgreSQL", "development deployed `3c632d86`", "production",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-599 marker %q", rel, want)
			}
		}
	}

	repoRoot := filepath.Dir(findAncestorForTest(t, "go.mod"))
	for rel, wants := range map[string][]string{
		"REQUIREMENTS.md": {
			"PR-599-MATERIAL-INVENTORY-PRICE-UNIT-UNIFICATION", "cost_unit=unit", "227g", "0.227kg",
		},
		"ACCEPTANCE_TESTS.md": {
			"PR-599-MATERIAL-INVENTORY-PRICE-UNIT-UNIFICATION", "g/kg", "kg/kg", "历史快照",
		},
		"ACTIVE_REQUIREMENTS.md": {
			"PR-599-MATERIAL-INVENTORY-PRICE-UNIT-UNIFICATION", "DEV-599-MATERIAL-UNIT-INVARIANT",
			"DEV-599-LEGACY-WEIGHT-MIGRATION", "DEV-599-BOM-COST-CONVERSION",
			"DEV-599-VUE-DOCS-DEVELOPMENT-DELIVERY", "REV-599-MATERIAL-INVENTORY-PRICE-UNIT-UNIFICATION",
			"development deployed `3c632d86`", "main", "production", "不操作",
		},
	} {
		src, err := os.ReadFile(filepath.Join(repoRoot, rel))
		if err != nil {
			if rel == "ACTIVE_REQUIREMENTS.md" && os.IsNotExist(err) && repoRoot == string(filepath.Separator) {
				continue
			}
			t.Fatal(err)
		}
		for _, want := range wants {
			if !strings.Contains(string(src), want) {
				t.Fatalf("%s missing PR-599 marker %q", rel, want)
			}
		}
	}
}

func requireDev599SeedRow(t *testing.T, src, table, code, status, assignee string) {
	t.Helper()
	pattern := regexp.MustCompile(`(?m)^[\t ]*\{table: "` + regexp.QuoteMeta(table) + `"[^\n]*code: "` + regexp.QuoteMeta(code) + `"[^\n]*status: "` + regexp.QuoteMeta(status) + `"[^\n]*assignee: "` + regexp.QuoteMeta(assignee) + `"[^\n]*\},[\t ]*$`)
	if !pattern.MatchString(src) {
		t.Fatalf("req_store.go missing one-line %s seed %s with status %s and assignee %s", table, code, status, assignee)
	}
}
