package support

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDev595UnifiedCategoryMoveInteractionDeliveryContracts(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, row := range []struct {
		table    string
		code     string
		status   string
		assignee string
	}{
		{table: "req_product", code: "PR-595-UNIFIED-CATEGORY-MOVE-INTERACTION", status: "review", assignee: "VA"},
		{table: "req_dev", code: "DEV-595-SHARED-MOVE-STATE", status: "done", assignee: "Codex"},
		{table: "req_dev", code: "DEV-595-MATERIAL-ARCHIVE", status: "done", assignee: "Codex"},
		{table: "req_dev", code: "DEV-595-PRODUCT-BOM", status: "done", assignee: "Codex"},
		{table: "req_dev", code: "DEV-595-WAREHOUSE-INVENTORY", status: "done", assignee: "Codex"},
		{table: "req_dev", code: "DEV-595-DOCS-ACCEPTANCE", status: "done", assignee: "Codex"},
		{table: "req_review", code: "REV-595-UNIFIED-CATEGORY-MOVE-INTERACTION", status: "todo", assignee: "VA"},
	} {
		requireDev595SeedRow(t, reqStore, row.table, row.code, row.status, row.assignee)
	}
	if !strings.Contains(reqStore, "docs/acceptance/2026-08-10-unified-category-move-interaction.md") {
		t.Fatal("req_store.go missing PR-595 acceptance evidence")
	}

	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-595-UNIFIED-CATEGORY-MOVE-INTERACTION",
			"物料档案、生产 BOM、商品档案和选中具体仓库且非客户库存上下文的仓内物品",
			"点击目标分类后立即移动",
			"保留各页面自己的搜索、状态、类型、仓库和分页语义",
			"warehouse_inventory_item",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-595-UNIFIED-CATEGORY-MOVE-INTERACTION",
			"请选择要移动到的分类",
			"全部分类和模板标题不能作为移动目标",
			"成功后清空勾选",
			"失败后保留勾选和移动模式",
			"部署 development",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-595 取代 PR-458 的仓库 code 归类口径",
			"物料档案",
			"商品档案",
			"仓内物品",
			"移动到分类",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-595 的内层“左侧分类树 + 右侧列表”",
			"生产 BOM",
			"移动到分类",
			"按 BOM 名称或编号搜索",
		},
		filepath.Join("docs", "acceptance", "2026-08-10-unified-category-move-interaction.md"): {
			"PR-595 四列表统一分类移动交互验收记录",
			"## RED 证据",
			"## GREEN 证据",
			"## 合并与开发部署证据",
			"## 未执行事项",
			"8c182a4cbf86a05a0bf55cae06fea34fbbc88c5f",
			"未部署 production",
			"Van 业务验收待办",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-595 delivery marker %q", rel, want)
			}
		}
	}
}

func requireDev595SeedRow(t *testing.T, src, table, code, status, assignee string) {
	t.Helper()
	pattern := regexp.MustCompile(`(?m)^[\t ]*\{table: "` + regexp.QuoteMeta(table) + `"[^\n]*code: "` + regexp.QuoteMeta(code) + `"[^\n]*status: "` + regexp.QuoteMeta(status) + `"[^\n]*assignee: "` + regexp.QuoteMeta(assignee) + `"[^\n]*\},[\t ]*$`)
	if !pattern.MatchString(src) {
		t.Fatalf("req_store.go missing one-line %s seed %s with status %s and assignee %s", table, code, status, assignee)
	}
}
