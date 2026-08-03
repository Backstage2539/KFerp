package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev577GreenBeanPricingEmptyPublishedBomContract(t *testing.T) {
	files := []struct {
		path    string
		needles []string
	}{
		{
			path: filepath.Join("internal", "application", "costing", "service.go"),
			needles: []string{
				"ComponentCount",
				"LatestNonEmptyDraftVersionID",
				"pricingRuleTrialRejectEmptyPublishedBom",
				"当前已发布生产 BOM",
				"生产管理 → 生产 BOM",
			},
		},
		{
			path: filepath.Join("internal", "infrastructure", "postgres", "costing", "repository.go"),
			needles: []string{
				"production_bom_version_items component",
				"latest_nonempty_draft",
				"draft.status='draft'",
			},
		},
		{
			path: filepath.Join("internal", "infrastructure", "postgres", "bom", "repository.go"),
			needles: []string{
				"production_bom_version_items source_item",
				"binding_version_candidates AS",
				"RETURNING version_id",
			},
		},
		{
			path: filepath.Join("internal", "interfaces", "http", "support", "req_store.go"),
			needles: []string{
				"PR-577-GREEN-BEAN-PRICING-EMPTY-PUBLISHED-BOM",
				"DEV-577-EMPTY-PUBLISHED-BOM-DIAGNOSTIC",
				"DEV-577-LEGACY-BINDING-GUARD",
				"DEV-577-COST-SOURCE-SAFETY",
				"REV-577-GREEN-BEAN-PRICING-EMPTY-PUBLISHED-BOM",
			},
		},
		{
			path:    filepath.Join("docs", "OP_MANUAL_COSTING.md"),
			needles: []string{"PR-577", "已发布但没有组件", "不会暗中读取草稿成本"},
		},
		{
			path:    filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"),
			needles: []string{"published 生产 BOM 必须有真实组件", "不会自动读取或发布草稿"},
		},
		{
			path:    filepath.Join("docs", "OP_MANUAL_GREEN_BEAN_SALES.md"),
			needles: []string{"生豆价格计算模板试算为 0", "商品形态为生豆"},
		},
		{
			path: filepath.Join("docs", "acceptance", "2026-08-04-green-bean-pricing-empty-published-bom.md"),
			needles: []string{
				"TDD RED 证据",
				"GREEN 与数据库级证据",
				"Van 验收清单",
				"production：未部署",
			},
		},
	}

	for _, file := range files {
		body := string(readOrderAppFileForTest(t, file.path))
		for _, needle := range file.needles {
			if !strings.Contains(body, needle) {
				t.Fatalf("%s missing %q", file.path, needle)
			}
		}
	}
}
