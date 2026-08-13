package support

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDev538MaterialCostUnitLossContracts(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, row := range []struct {
		table  string
		code   string
		status string
	}{
		{table: "req_product", code: "PR-538-MATERIAL-COST-UNIT-LOSS", status: "review"},
		{table: "req_dev", code: "DEV-538-MATERIAL-COST-UNIT", status: "done"},
		{table: "req_dev", code: "DEV-538-BOM-DEFAULT-LOSS", status: "done"},
		{table: "req_dev", code: "DEV-538-TRIAL-COST-LOSS-CLARITY", status: "done"},
		{table: "req_dev", code: "DEV-538-DOCS-DEPLOY", status: "done"},
		{table: "req_review", code: "REV-538-MATERIAL-COST-UNIT-LOSS", status: "todo"},
	} {
		requireDev538SeedRow(t, reqStore, row.table, row.code, row.status)
	}

	for rel, wants := range map[string][]string{
		filepath.Join("internal", "application", "materials", "service.go"): {
			"CostUnit", "json:\"cost_unit\"",
		},
		filepath.Join("internal", "infrastructure", "postgres", "materials", "schema.go"): {
			"cost_unit TEXT NOT NULL DEFAULT 'kg'",
			"ALTER COLUMN cost_unit SET DEFAULT 'kg'",
			"materials_unit_cost_unit_match",
		},
		filepath.Join("internal", "infrastructure", "postgres", "materials", "repository.go"): {
			"assertMaterialCostUnitMatchesInventoryUnit", "采购价与成本单价单位必须与库存单位一致", "cost_unit",
		},
		filepath.Join("internal", "infrastructure", "postgres", "bom", "repository.go"): {
			"yieldRate := 1.0",
		},
		filepath.Join("internal", "infrastructure", "postgres", "bom", "schema.go"): {
			"production_bom_versions ALTER COLUMN yield_rate SET DEFAULT 1.0000",
		},
		filepath.Join("internal", "infrastructure", "postgres", "costing", "repository.go"): {
			"m.unit", "unit_cost_unit",
		},
		filepath.Join("internal", "application", "costing", "service.go"): {
			"标准制造成本", "配方比例", "原料加耗", "计价比例",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "MaterialsView.vue"): {
			"重量物料库存统一使用 kg", "采购价、批次单位成本和 BOM 成本试算均按库存单位计价", "采购价（元/{{ draft.unit }}）", "cost_unit: draftMode.value ? draft.value.unit",
		},
		filepath.Join("..", "REQUIREMENTS.md"): {
			"PR-538-MATERIAL-COST-UNIT-LOSS", "PR-599", "cost_unit", "始终等于 `unit`", "yield_rate=1", "80.50元/kg", "82.54元/kg",
		},
		filepath.Join("..", "ACCEPTANCE_TESTS.md"): {
			"PR-538-MATERIAL-COST-UNIT-LOSS", "PR-599", "g/kg → kg/kg", "102.68元/kg",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-538-MATERIAL-COST-UNIT-LOSS", "重量物料", "cost_unit", "PR-592", "64.6875元/kg", "67.2942元/kg",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"K80. 物料成本计价单位与生产 BOM 损耗口径修正", "PR-599", "g/kg → kg/kg", "yield_rate=0.8", "64.6875元/kg", "67.2942元/kg",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-599-MATERIAL-INVENTORY-PRICE-UNIT-UNIFICATION", "采购价单位就是库存单位", "重量物料主档只能选择 `kg`", "cost_unit",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-592-BOM-LOSS-GROSS-INPUT", "54元/kg", "64.6875元/kg", "67.2942元/kg", "83.47元/kg",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-592-BOM-LOSS-GROSS-INPUT", "÷ (1 - 原料损耗率)", "唯一配置损耗",
		},
		filepath.Join("docs", "acceptance", "2026-07-16-material-cost-unit-loss.md"): {
			"PR-538 物料成本计价单位与生产 BOM 损耗口径修正验收", "历史证据说明", "PR-599", "RED（实现前）", "GREEN（实现后定向验证）", "生产环境未部署、未写入、未切换入口",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-538 marker %q", rel, want)
			}
		}
	}

	materialsView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "MaterialsView.vue")))
	for _, forbidden := range []string{"data-field=\"cost_unit\"", "采购价与成本单价单位保存后不可修改", "采购价（元/{{ draft.cost_unit }}）"} {
		if strings.Contains(materialsView, forbidden) {
			t.Fatalf("PR-538 historical contract must not restore superseded two-unit UI marker %q", forbidden)
		}
	}
	costingRepository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "costing", "repository.go")))
	if strings.Contains(costingRepository, "m.cost_unit") {
		t.Fatal("current costing must read the unified material inventory unit instead of the compatibility cost_unit field")
	}

	costingService := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "costing", "service.go")))
	for _, forbidden := range []string{"整体预期损耗", "连续放大标准制造成本", "整体预期损耗率设为 0"} {
		if strings.Contains(costingService, forbidden) {
			t.Fatalf("current costing service must not restore removed double-loss warning %q", forbidden)
		}
	}
}

func requireDev538SeedRow(t *testing.T, src, table, code, status string) {
	t.Helper()
	pattern := regexp.MustCompile(`(?m)^[\t ]*\{table: "` + regexp.QuoteMeta(table) + `"[^\n]*code: "` + regexp.QuoteMeta(code) + `"[^\n]*status: "` + regexp.QuoteMeta(status) + `"[^\n]*\},[\t ]*$`)
	if !pattern.MatchString(src) {
		t.Fatalf("req_store.go missing one-line %s seed %s with status %s", table, code, status)
	}
}

func dev538MarkdownSectionBody(src, marker string) string {
	markerIndex := strings.Index(src, marker)
	if markerIndex < 0 {
		return ""
	}
	headingEnd := strings.Index(src[markerIndex:], "\n")
	if headingEnd < 0 {
		return ""
	}
	bodyStart := markerIndex + headingEnd + 1
	body := src[bodyStart:]
	if nextHeading := strings.Index(body, "\n##"); nextHeading >= 0 {
		body = body[:nextHeading]
	}
	return strings.TrimSpace(body)
}
