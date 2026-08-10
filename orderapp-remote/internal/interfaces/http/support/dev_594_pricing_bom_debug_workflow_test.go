package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev594PricingBomDebugWorkflowContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-594-PRICING-BOM-DEBUG-WORKFLOW",
			"DEV-594-PRICING-FIXED-BOM-COST",
			"DEV-594-PRICING-BOM-ROUNDTRIP",
			"DEV-594-PRICING-RULE-UPDATE",
			"DEV-594-BOM-LOSS-LABELS",
			"REV-594-PRICING-BOM-DEBUG-WORKFLOW",
		},
		filepath.Join("..", "REQUIREMENTS.md"): {
			"PR-594-PRICING-BOM-DEBUG-WORKFLOW",
			"草稿，仅供试算",
			"更新参数到价格计算模板",
		},
		filepath.Join("..", "ACCEPTANCE_TESTS.md"): {
			"PR-594-PRICING-BOM-DEBUG-WORKFLOW",
			"配置BOM",
			"有损耗的配方",
			"无损耗的配方",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-594-PRICING-BOM-DEBUG-WORKFLOW",
			"草稿，仅供试算",
			"返回价格试算",
			"更新参数到价格计算模板",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"有损耗的配方",
			"无损耗的配方",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-594 marker %q", rel, want)
			}
		}
	}

	serviceSource := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "costing", "service.go")))
	for _, want := range []string{
		"allowDraftBom",
		"草稿 BOM 仅支持单次价格试算",
		"草稿 BOM，仅供试算；不会进入正式价格表或发布快照",
	} {
		if !strings.Contains(serviceSource, want) {
			t.Fatalf("costing service missing interactive draft guard %q", want)
		}
	}

	for _, rel := range []string{
		filepath.Join("internal", "infrastructure", "postgres", "costing", "repository.go"),
		filepath.Join("internal", "infrastructure", "postgres", "costing", "production_bom_cost.go"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{"l.qty_units", "(l.qty_g > 0 OR l.qty_units > 0)"} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing discrete packaging valuation marker %q", rel, want)
			}
		}
	}

	productSettingsSource := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	for _, want := range []string{
		"navigatePricingRuleTrialBom",
		"storePricingRuleTrialReturnState",
		"takePricingRuleTrialReturnState",
		"配置BOM",
		"返回价格试算",
		"updatePricingRuleFromTrial",
		"buildPricingRuleUpdateFromTrial",
		"更新参数到价格计算模板",
	} {
		if !strings.Contains(productSettingsSource, want) {
			t.Fatalf("ProductSettingsView.vue missing trial workflow marker %q", want)
		}
	}

	bomSource := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue")))
	for _, want := range []string{"有损耗的配方", "无损耗的配方"} {
		if !strings.Contains(bomSource, want) {
			t.Fatalf("BomView.vue missing renamed recipe zone %q", want)
		}
	}
	for _, obsolete := range []string{
		"比例用量应用当前 BOM 原料损耗比",
		"固定用量和商品组件不参与原料损耗",
	} {
		if strings.Contains(bomSource, obsolete) {
			t.Fatalf("BomView.vue still contains obsolete recipe-zone copy %q", obsolete)
		}
	}
}
