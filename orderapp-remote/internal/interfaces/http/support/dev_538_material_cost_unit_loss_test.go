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
		},
		filepath.Join("internal", "infrastructure", "postgres", "materials", "repository.go"): {
			"assertMaterialCostUnitReadOnly", "成本计价单位", "cost_unit",
		},
		filepath.Join("internal", "infrastructure", "postgres", "bom", "repository.go"): {
			"yieldRate := 1.0",
		},
		filepath.Join("internal", "infrastructure", "postgres", "bom", "schema.go"): {
			"production_bom_versions ALTER COLUMN yield_rate SET DEFAULT 1.0000",
		},
		filepath.Join("internal", "infrastructure", "postgres", "costing", "repository.go"): {
			"m.cost_unit", "unit_cost_unit",
		},
		filepath.Join("internal", "application", "costing", "service.go"): {
			"整体预期损耗", "原料损耗", "连续放大标准制造成本", "整体预期损耗率设为 0",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "MaterialsView.vue"): {
			"成本计价单位", "成本计价单位保存后不可修改", "采购价（元/{{ draft.cost_unit }}）",
		},
		filepath.Join("..", "REQUIREMENTS.md"): {
			"PR-538-MATERIAL-COST-UNIT-LOSS", "成本计价单位", "yield_rate=1", "80.50元/kg", "82.54元/kg", "生产环境禁止部署",
		},
		filepath.Join("..", "ACCEPTANCE_TESTS.md"): {
			"PR-538-MATERIAL-COST-UNIT-LOSS", "cost_unit=kg", "连续放大", "102.68元/kg", "生产环境未部署、未写入、未切换入口",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-538-MATERIAL-COST-UNIT-LOSS", "重量物料", "cost_unit", "双损耗连续放大", "开发数据库",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"K80. 物料成本计价单位与生产 BOM 损耗口径修正", "54元/kg", "yield_rate=1", "80.50元/kg", "82.54元/kg",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-538-MATERIAL-COST-UNIT-LOSS", "库存单位管数量，成本计价单位管单价", "采购价（元/kg）",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-538 物料成本计价单位与双损耗说明", "54元/kg", "80.50元/kg", "82.54元/kg", "102.68元/kg",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-538-MATERIAL-COST-UNIT-LOSS", "yield_rate=1", "连续放大",
		},
		filepath.Join("docs", "acceptance", "2026-07-16-material-cost-unit-loss.md"): {
			"PR-538 物料成本计价单位与生产 BOM 损耗口径修正验收", "RED（实现前）", "GREEN（实现后定向验证）", "生产环境未部署、未写入、未切换入口",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-538 marker %q", rel, want)
			}
		}
	}

	rootRequirements := string(readOrderAppFileForTest(t, filepath.Join("..", "REQUIREMENTS.md")))
	docsRequirements := string(readOrderAppFileForTest(t, filepath.Join("docs", "REQUIREMENTS.md")))
	if got, want := dev538MarkdownSectionBody(rootRequirements, "PR-538-MATERIAL-COST-UNIT-LOSS"), dev538MarkdownSectionBody(docsRequirements, "PR-538-MATERIAL-COST-UNIT-LOSS"); got != want {
		t.Fatal("root and orderapp-remote/docs PR-538 requirement bodies must stay mirrored")
	}

	rootAcceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "ACCEPTANCE_TESTS.md")))
	docsAcceptance := string(readOrderAppFileForTest(t, filepath.Join("docs", "ACCEPTANCE_TESTS.md")))
	if got, want := dev538MarkdownSectionBody(rootAcceptance, "PR-538-MATERIAL-COST-UNIT-LOSS"), dev538MarkdownSectionBody(docsAcceptance, "PR-538-MATERIAL-COST-UNIT-LOSS"); got != want {
		t.Fatal("root and orderapp-remote/docs PR-538 acceptance bodies must stay mirrored")
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
