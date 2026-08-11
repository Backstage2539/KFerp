package support

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDev598MaterialOutputMultilevelManufacturingContracts(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	rows := []struct {
		table, code, status, assignee string
	}{
		{"req_product", "PR-597-SEMI-FINISHED-PACKAGING-BOM", "done", "Codex"},
		{"req_product", "PR-598-MATERIAL-OUTPUT-MULTILEVEL-MANUFACTURING", "review", "VA"},
		{"req_dev", "DEV-598-MATERIAL-SEMI-FINISHED-CAPABILITY", "done", "Codex"},
		{"req_dev", "DEV-598-TYPED-BOM-OUTPUT", "done", "Codex"},
		{"req_dev", "DEV-598-BOM-GRAPH-DEFAULTS-VALIDATION", "done", "Codex"},
		{"req_dev", "DEV-598-MULTILEVEL-NET-REQUIREMENTS", "done", "Codex"},
		{"req_dev", "DEV-598-WORKORDER-DEPENDENCIES", "done", "Codex"},
		{"req_dev", "DEV-598-MATERIAL-MANUFACTURE-STOCK", "done", "Codex"},
		{"req_dev", "DEV-598-RECURSIVE-COSTING", "done", "Codex"},
		{"req_dev", "DEV-598-VUE-MANUFACTURING-WORKFLOW", "done", "Codex"},
		{"req_dev", "DEV-598-AUDIT-COMPAT-DOCS-DELIVERY", "done", "Codex"},
		{"req_review", "REV-598-MATERIAL-OUTPUT-MULTILEVEL-MANUFACTURING", "todo", "VA"},
	}
	for _, row := range rows {
		requireDev598SeedRow(t, reqStore, row.table, row.code, row.status, row.assignee)
	}
	pr597Tombstones := regexp.MustCompile(`(?m)^[\t ]*\{table: "req_product"[^\n]*code: "PR-597-SEMI-FINISHED-PACKAGING-BOM"[^\n]*\},[\t ]*$`).FindAllString(reqStore, -1)
	if len(pr597Tombstones) != 1 {
		t.Fatalf("req_store.go must retain exactly one PR-597 product tombstone, got %d", len(pr597Tombstones))
	}
	for _, want := range []string{"已回退", "PR-598", "no DEV-597 completion rows retained"} {
		if !strings.Contains(pr597Tombstones[0], want) {
			t.Fatalf("PR-597 product tombstone missing %q", want)
		}
	}
	if regexp.MustCompile(`code: "(?:DEV|REV)-597-`).MatchString(reqStore) {
		t.Fatal("req_store.go must not seed unverified PR-597 DEV or review rows")
	}
	for _, want := range []string{
		"frontend find full 983/983",
		"Vite build GREEN 2.08s",
		"production HTTP real PostgreSQL full GREEN 86.736s",
		"BOM/material/catalog/costing/stock real PostgreSQL GREEN",
		"default switch cycle and legacy repair GREEN",
		"direct product complete/partial/multi/cancel GREEN",
		"audit atomic rollback and cancel Note GREEN",
		"merge/deploy pending",
		"browser/manual acceptance not run",
		"production environment out of scope",
	} {
		if !strings.Contains(reqStore, want) {
			t.Fatalf("req_store.go missing current PR-598 delivery evidence %q", want)
		}
	}

	for rel, wants := range map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "views", "MaterialsView.vue"): {
			"is_semi_finished", "can_manufacture", "产出该物料的 BOM", "使用该物料的 BOM", "returnNavigation",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"): {
			"output_type", "output_material_id", "产出对象", "设为产出对象默认 BOM", "BusinessGroupInlineWorkspace",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue"): {
			"manufacturing_plan", "库存覆盖", "净缺口", "上游依赖",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "WorkOrdersView.vue"): {
			"产出对象", "上游依赖", "workOrderUpstreamBlockerLabel",
		},
		filepath.Join("frontend-vue-shell", "src", "components", "ProductionExecutionHubDrawer.vue"): {
			"产出对象", "上游依赖", "executionHubUpstreamBlockers",
		},
		filepath.Join("frontend-vue-shell", "src", "App.vue"):                         {"stockManual: OperationManualView"},
		filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js"):               {"stockManual", "库存作业手册"},
		filepath.Join("frontend-vue-shell", "src", "lib", "operation-manuals.js"):     {"stockManual", "OP_MANUAL_STOCK.md"},
		filepath.Join("internal", "infrastructure", "postgres", "authz", "schema.go"): {`"stockManual":`, `"stock.read"`},
		filepath.Join("docs", "REQUIREMENTS.md"):                                      {"PR-597", "已撤回", "PR-598-MATERIAL-OUTPUT-MULTILEVEL-MANUFACTURING", "is_semi_finished", "can_manufacture"},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"):                                  {"PR-598-MATERIAL-OUTPUT-MULTILEVEL-MANUFACTURING", "任意有效物料", "库存覆盖", "净缺口"},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"):                     {"PR-598", "是否半成品", "可制造能力", "产出该物料的 BOM"},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"):                              {"PR-598", "产出对象", "递归", "上游依赖"},
		filepath.Join("docs", "OP_MANUAL_STOCK.md"):                                   {"PR-598", "物料工单", "目标仓库", "批次"},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"):                                 {"PR-598", "递归成本", "各层 BOM"},
		filepath.Join("docs", "OPERATION_MANUALS.md"):                                 {"OP_MANUAL_STOCK.md", "库存作业手册"},
		filepath.Join("docs", "acceptance", "2026-08-11-material-output-multilevel-manufacturing.md"): {
			"PR-598", "## RED 证据", "## GREEN 证据", "find 全量 983 / 983", "Vite 2.08s", "production HTTP 真实 PostgreSQL 全包 86.736s",
			"BOM / material / catalog / costing / stock 真实 PostgreSQL", "默认切换循环", "旧库 repair",
			"direct product complete / partial / multi / cancel", "最终审计原子回滚", "取消 Note",
			"浏览器人工验收未执行", "合并与 development 部署均未执行", "production 环境不在自动验收范围内",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-598 marker %q", rel, want)
			}
		}
	}

	for _, rel := range []string{
		filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"),
		filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue"),
		filepath.Join("frontend-vue-shell", "src", "views", "WorkOrdersView.vue"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, forbidden := range []string{"bom_kind", "spec_packaging_bom_id", "semi_finished_packaging_required", "两段式"} {
			if strings.Contains(src, forbidden) {
				t.Fatalf("%s retains withdrawn implementation symbol %q", rel, forbidden)
			}
		}
	}

	repoRoot := filepath.Dir(findAncestorForTest(t, "go.mod"))
	for rel, wants := range map[string][]string{
		"REQUIREMENTS.md":     {"PR-597", "已撤回", "PR-598-MATERIAL-OUTPUT-MULTILEVEL-MANUFACTURING"},
		"ACCEPTANCE_TESTS.md": {"PR-598-MATERIAL-OUTPUT-MULTILEVEL-MANUFACTURING", "任意有效物料"},
		"ACTIVE_REQUIREMENTS.md": {
			"PR-598-MATERIAL-OUTPUT-MULTILEVEL-MANUFACTURING", "REV-598-MATERIAL-OUTPUT-MULTILEVEL-MANUFACTURING",
			"find 全量 983/983", "Vite 2.08s", "production HTTP 真实 PostgreSQL 全包 86.736s",
			"BOM / material / catalog / costing / stock 真实 PostgreSQL GREEN", "默认切换循环 / 旧库 repair GREEN",
			"direct product complete / partial / multi / cancel GREEN", "最终审计原子回滚 / 取消 Note GREEN",
			"Merge/deploy: pending", "浏览器 / 人工业务验收未执行", "production 环境不在自动验收范围内",
		},
	} {
		src, err := os.ReadFile(filepath.Join(repoRoot, rel))
		if err != nil {
			// The release Dockerfile intentionally copies only the durable root
			// governance documents into the isolated build context. Keep ACTIVE
			// mandatory in a real checkout while allowing that established image
			// safety gate to run without widening the release artifact contract.
			if rel == "ACTIVE_REQUIREMENTS.md" && os.IsNotExist(err) && repoRoot == string(filepath.Separator) {
				continue
			}
			t.Fatal(err)
		}
		for _, want := range wants {
			if !strings.Contains(string(src), want) {
				t.Fatalf("%s missing PR-598 marker %q", rel, want)
			}
		}
	}
}

func requireDev598SeedRow(t *testing.T, src, table, code, status, assignee string) {
	t.Helper()
	pattern := regexp.MustCompile(`(?m)^[\t ]*\{table: "` + regexp.QuoteMeta(table) + `"[^\n]*code: "` + regexp.QuoteMeta(code) + `"[^\n]*status: "` + regexp.QuoteMeta(status) + `"[^\n]*assignee: "` + regexp.QuoteMeta(assignee) + `"[^\n]*\},[\t ]*$`)
	if !pattern.MatchString(src) {
		t.Fatalf("req_store.go missing one-line %s seed %s with status %s and assignee %s", table, code, status, assignee)
	}
}
