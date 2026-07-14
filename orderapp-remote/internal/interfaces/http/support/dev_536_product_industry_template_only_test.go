package support

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDev536ProductIndustryTemplateOnlyContracts(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, row := range []struct {
		table  string
		code   string
		status string
	}{
		{table: "req_product", code: "PR-536-PRODUCT-INDUSTRY-TEMPLATE-ONLY", status: "review"},
		{table: "req_dev", code: "DEV-536-FRONTEND-TEMPLATE-PROJECTION", status: "done"},
		{table: "req_dev", code: "DEV-536-BACKEND-TEMPLATE-CONSTRAINT", status: "done"},
		{table: "req_dev", code: "DEV-536-LEGACY-FIELD-CLEANUP", status: "done"},
		{table: "req_dev", code: "DEV-536-DOCS-ACCEPTANCE", status: "done"},
		{table: "req_review", code: "REV-536-PRODUCT-INDUSTRY-TEMPLATE-ONLY", status: "todo"},
	} {
		requireDev536SeedRow(t, reqStore, row.table, row.code, row.status)
	}
	if !strings.Contains(reqStore, "OP_MANUAL_INVENTORY_MATERIALS.md; OP_MANUAL_COSTING.md; docs/ACCEPTANCE_TESTS.md") {
		t.Fatal("req_store.go must record both current PR-536 workflow manuals as DEV-536-DOCS-ACCEPTANCE evidence")
	}

	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-536-PRODUCT-INDUSTRY-TEMPLATE-ONLY",
			"PR-439 历史需求口径",
			"响应必须包含 `fields`，其值为 `[]`，不得为 `null` 或省略",
			"应用服务在 `industry_field_template_id=0` 时先清空",
			"PostgreSQL 仓储对直接调用方重复执行无模板防御",
			"`CopyProduct` 当前只复制商品主档基础资料、单位模板引用、单位覆盖和库存相关主数据",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-536-PRODUCT-INDUSTRY-TEMPLATE-ONLY",
			"PR-439 历史验收口径",
			"PR-536 当前替代口径见 K78，仍待验收",
			"响应必须包含 `fields`，其值为 `[]`，不得为 `null` 或省略",
			"应用服务先清空无模板字段",
			"PostgreSQL 仓储负责模板成员校验",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"取消行业字段模板会清空商品行业字段",
			"只复制商品主档基础资料、单位模板引用、单位覆盖和库存相关主数据",
			"不复制商品生产配置、工艺路线、预期损耗率",
			"重新配置工艺路线和预期损耗率",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"复制为商品档案不复制行业字段模板或行业字段值",
			"不复制商品生产配置、工艺路线、预期损耗率",
			"在新商品配置中重新配置工艺路线和预期损耗率",
		},
		filepath.Join("docs", "acceptance", "2026-07-14-product-industry-template-only.md"): {
			"PR-536 商品行业字段仅来源于模板验收",
			"非 nil 空切片",
			"PostgreSQL 仓储负责模板成员校验",
			"OP_MANUAL_COSTING.md",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-536 marker %q", rel, want)
			}
		}
	}
}

func requireDev536SeedRow(t *testing.T, src, table, code, status string) {
	t.Helper()
	pattern := regexp.MustCompile(`(?m)^[\t ]*\{table: "` + regexp.QuoteMeta(table) + `"[^\n]*code: "` + regexp.QuoteMeta(code) + `"[^\n]*status: "` + regexp.QuoteMeta(status) + `"[^\n]*\},[\t ]*$`)
	if !pattern.MatchString(src) {
		t.Fatalf("req_store.go missing one-line %s seed %s with status %s", table, code, status)
	}
}
