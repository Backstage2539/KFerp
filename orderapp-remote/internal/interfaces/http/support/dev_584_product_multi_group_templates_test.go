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
		{table: "req_dev", code: "DEV-584-PRICE-LIST-INHERIT-PRODUCT-GROUPS", status: "done", assignee: "Codex"},
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
			"分组模板只维护模板与分类树",
			"由各功能页面多选自己使用的分组模板",
			"商品价格表不得维护独立 `price_list` 分组模板引用",
			"商品价格表不得维护独立 `price_list` 分组模板引用，而是继承商品档案选择的分组模板",
			"PR-534 的“无用途绑定即通用候选”口径由本需求替代",
			"收起任一大类时必须隐藏它的全部小类标题和商品行",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-584-PRODUCT-MULTI-GROUP-TEMPLATES",
			"重复 `field_key` 只显示一次并使用前序模板定义",
			"商品档案、物料档案、生产 BOM、仓库库存分别在自己的页面多选并保存所用模板",
			"商品档案一次内联展示所有已选 `product_catalog` 模板分类",
			"没有选择模板时只显示全部分类平铺区且移动按钮禁用",
			"价格表商品类型只出现与这两份模板一一对应的两个选项",
			"历史 `price_list` usage 即使存在也不能增加类型",
			"商品档案收起大类后，其全部小类标题和商品行同时隐藏",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-584-PRODUCT-MULTI-GROUP-TEMPLATES",
			"有序 `industry_field_template_ids`",
			"重复字段键以前序模板定义为准",
			"在商品档案页面多选要使用的分组模板",
			"未选择的模板不会进入商品档案分类",
			"未引用任何分组模板时只显示 `全部分类` 平铺区",
			"仍可从列表底部进入模板设置",
			"收起大类会隐藏全部后代分类标题和商品行",
			"保留小类自身的折叠状态",
		},
		filepath.Join("docs", "OP_MANUAL_SETTINGS_AUDIT.md"): {
			"PR-584-PRODUCT-MULTI-GROUP-TEMPLATES",
			"分组模板页只维护模板与分类树",
			"不在分组模板页选择功能",
			"商品价格表不提供独立的分组模板选择",
			"取消引用只会隐藏对应业务入口",
			"不会删除模板、分类、既有对象归类或历史快照",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-584-PRODUCT-MULTI-GROUP-TEMPLATES",
			"多份行业字段模板",
			"重复字段键以前序模板定义为准",
			"商品价格表继承商品档案已选的分组模板",
			"每个已选模板对应一种商品类型",
			"不读取历史 `price_list` 引用",
			"已发布价格表快照不会回改",
		},
		filepath.Join("docs", "acceptance", "2026-08-07-product-multi-group-templates.md"): {
			"PR-584 商品行业字段与功能自选分组模板验收记录",
			"## 验收口径修正",
			"## RED 证据",
			"## GREEN 证据",
			"## 开发环境部署证据",
			"## Van 业务验收",
			"PostgreSQL 16.13",
			"相关价格表定向用例 51/51 通过",
			"四个功能自选模板",
			"不再执行浏览器或 development 业务验收",
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
