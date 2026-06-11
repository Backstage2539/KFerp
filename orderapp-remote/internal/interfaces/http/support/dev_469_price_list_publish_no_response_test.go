package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev469PriceListPublishNoResponseContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-469-PRICE-LIST-PUBLISH-NO-RESPONSE",
			"DEV-469-PRICE-LIST-PUBLISH-BLOCKED-REASON",
			"DEV-469-DOCS-ACCEPTANCE",
			"REV-469-PRICE-LIST-PUBLISH-NO-RESPONSE",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue"): {
			"priceListPublishBlockedReason",
			"price-list-publish-guard",
			"product-picker-bom-warning",
			"去商品档案重新选择 BOM",
			"失效 BOM 不能重新启用",
			"暂无可发布的价格表预览",
			"请填写价格表版本号",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "costing-bean-list-version-ui.test.js"): {
			"product price-list publish action reports blocked reasons instead of doing nothing",
			"priceListPublishBlockedReason",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-469-PRICE-LIST-PUBLISH-NO-RESPONSE",
			"发布价格表",
			"BOM已失效",
			"商品档案重新选择可用 BOM",
			"不能静默无反馈",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-469-PRICE-LIST-PUBLISH-NO-RESPONSE",
			"点击 `发布价格表`",
			"商品行",
			"商品档案",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-469-PRICE-LIST-PUBLISH-NO-RESPONSE",
			"发布价格表",
			"重新选择可用 BOM",
		},
		filepath.Join("docs", "acceptance", "2026-06-11-price-list-publish-no-response.md"): {
			"PR-469-PRICE-LIST-PUBLISH-NO-RESPONSE",
			"点了发布价格表没有反应",
			"BOM已失效",
			"失效 BOM 不能重新启用",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-469 marker %q", rel, want)
			}
		}
	}
}
