package support

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDev584ProductMultiGroupTemplatesDeliveryContracts(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, row := range []struct {
		table    string
		code     string
		status   string
		assignee string
	}{
		{table: "req_product", code: "PR-584-PRODUCT-MULTI-GROUP-TEMPLATES", status: "review", assignee: "VA"},
		{table: "req_dev", code: "DEV-584-INDUSTRY-FIELD-MULTI-TEMPLATE", status: "done", assignee: "Codex"},
		{table: "req_dev", code: "DEV-584-GROUP-USAGE-MULTI-REFERENCE", status: "done", assignee: "Codex"},
		{table: "req_dev", code: "DEV-584-PRODUCT-GROUP-UNION-COLLAPSE", status: "done", assignee: "Codex"},
		{table: "req_dev", code: "DEV-584-DOCS-ACCEPTANCE-DEPLOY", status: "done", assignee: "Codex"},
	} {
		requireDev584SeedRow(t, reqStore, row.table, row.code, row.status, row.assignee)
	}
	for _, want := range []string{
		"OP_MANUAL_INVENTORY_MATERIALS.md",
		"OP_MANUAL_SETTINGS_AUDIT.md",
		"OP_MANUAL_COSTING.md",
		"docs/acceptance/2026-08-07-product-multi-group-templates.md",
	} {
		if !strings.Contains(reqStore, want) {
			t.Fatalf("req_store.go missing PR-584 delivery evidence %q", want)
		}
	}

	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-584-PRODUCT-MULTI-GROUP-TEMPLATES",
			"industry_field_template_ids",
			"同名 `field_key` 只展示一项并以前序模板定义为准",
			"只有具有明确 active 用途引用的模板才进入对应功能",
			"PR-534 的“无用途绑定即通用候选”口径由本需求替代",
			"收起任一大类时必须隐藏它的全部小类标题和商品行",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-584-PRODUCT-MULTI-GROUP-TEMPLATES",
			"重复 `field_key` 只显示一次并使用前序模板定义",
			"商品档案一次展示所有明确引用 `product_catalog` 的模板分类",
			"没有明确引用模板时列表平铺",
			"商品档案收起大类后，其全部小类标题和商品行同时隐藏",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-584-PRODUCT-MULTI-GROUP-TEMPLATES",
			"有序 `industry_field_template_ids`",
			"重复字段键以前序模板定义为准",
			"未引用任何分组模板时按平铺列表展示",
			"收起大类会隐藏全部后代分类标题和商品行",
			"保留小类自身的折叠状态",
		},
		filepath.Join("docs", "OP_MANUAL_SETTINGS_AUDIT.md"): {
			"PR-584-PRODUCT-MULTI-GROUP-TEMPLATES",
			"功能引用”多选",
			"只有明确启用引用的模板",
			"取消引用只会隐藏对应业务入口",
			"不会删除模板、分类、既有对象归类或历史快照",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-584-PRODUCT-MULTI-GROUP-TEMPLATES",
			"多份行业字段模板",
			"重复字段键以前序模板定义为准",
			"已发布价格表快照不会回改",
		},
		filepath.Join("docs", "acceptance", "2026-08-07-product-multi-group-templates.md"): {
			"PR-584 商品行业字段与功能分组多模板引用验收记录",
			"## RED 证据",
			"## GREEN 证据",
			"## 开发环境部署证据",
			"## Van 业务验收",
			"PostgreSQL 16.13",
			"共 887 项全部通过",
			"无未解决 P0/P1/P2",
			"production 不在范围内",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-584 delivery marker %q", rel, want)
			}
		}
	}
}

func requireDev584SeedRow(t *testing.T, src, table, code, status, assignee string) {
	t.Helper()
	pattern := regexp.MustCompile(`(?m)^[\t ]*\{table: "` + regexp.QuoteMeta(table) + `"[^\n]*code: "` + regexp.QuoteMeta(code) + `"[^\n]*status: "` + regexp.QuoteMeta(status) + `"[^\n]*assignee: "` + regexp.QuoteMeta(assignee) + `"[^\n]*\},[\t ]*$`)
	if !pattern.MatchString(src) {
		t.Fatalf("req_store.go missing one-line %s seed %s with status %s and assignee %s", table, code, status, assignee)
	}
}
