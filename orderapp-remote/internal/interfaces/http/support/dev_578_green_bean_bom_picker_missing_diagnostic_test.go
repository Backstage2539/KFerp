package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev578GreenBeanBomPickerAndMissingBomDiagnosticContract(t *testing.T) {
	files := []struct {
		path    string
		needles []string
	}{
		{
			path: filepath.Join("internal", "application", "costing", "service.go"),
			needles: []string{
				"pricingRuleTrialRejectMissingPublishedBom",
				"pricingRuleTrialHasIndependentPositiveOperationCost",
				"options.loaded",
				"input.BomVersionID = 0",
				"未配置可用于试算的已发布生产 BOM",
				"生产管理 → 生产 BOM",
			},
		},
		{
			path: filepath.Join("internal", "infrastructure", "postgres", "costing", "repository.go"),
			needles: []string{
				"JOIN pricing_rule_trial_selected_products selected ON selected.product_id=pb.output_product_id",
				"AND v.status='published'",
				"AND v.id=$1",
			},
		},
		{
			path: filepath.Join("internal", "application", "bom", "service.go"),
			needles: []string{
				"func (s *Service) Products(ctx context.Context) ([]Option, error)",
				"return s.repo.Products(ctx)",
			},
		},
		{
			path: filepath.Join("frontend-vue-shell", "src", "lib", "bom.js"),
			needles: []string{
				"isProductionBomOutputProductCandidate",
				"isActiveBomProductOption(row)",
			},
		},
		{
			path: filepath.Join("internal", "interfaces", "http", "support", "req_store.go"),
			needles: []string{
				"PR-578-GREEN-BEAN-BOM-PICKER-MISSING-BOM-DIAGNOSTIC",
				"DEV-578-MISSING-PUBLISHED-BOM-DIAGNOSTIC",
				"DEV-578-GREEN-BEAN-OUTPUT-CANDIDATE",
				"REV-578-GREEN-BEAN-BOM-PICKER-MISSING-BOM-DIAGNOSTIC",
			},
		},
		{
			path:    filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"),
			needles: []string{"PR-578", "搜索“生豆”", "仅保存空草稿不会成为可试算成本来源"},
		},
		{
			path:    filepath.Join("docs", "OP_MANUAL_COSTING.md"),
			needles: []string{"PR-578", "未配置可用于试算的已发布生产 BOM"},
		},
		{
			path:    filepath.Join("docs", "OP_MANUAL_GREEN_BEAN_SALES.md"),
			needles: []string{"产出商品搜索“生豆”", "维护真实组件并发布后再试算"},
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
