package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev625ManualBOMAndOrderTierDeliveryContracts(t *testing.T) {
	checks := map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-625-MANUAL-BOM-AND-ORDER-TIER-FIX",
			"DEV-625-MANUAL-BOM-PUBLISH",
			"DEV-625-ORDER-TIER-API-FIELDS",
			"DEV-625-DUAL-ENV-DELIVERY",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"# PR-625-MANUAL-BOM-AND-ORDER-TIER-FIX",
			"48袋+",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"## PR-625-MANUAL-BOM-AND-ORDER-TIER-FIX",
			"商品-速溶-黑咖",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"手工维护的规格组不要求规格模板来源",
		},
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"): {
			"件数阶梯按接口返回的起止数量匹配",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue"): {
			"50 袋会匹配 48 袋以上档",
		},
		filepath.Join("docs", "acceptance", "2026-09-03-manual-bom-and-order-tier-fix.md"): {
			"# PR-625",
			"RED",
			"GREEN",
		},
	}
	for path, markers := range checks {
		body := string(readOrderAppFileForTest(t, path))
		for _, marker := range markers {
			if !strings.Contains(body, marker) {
				t.Errorf("%s missing %q", path, marker)
			}
		}
	}
}
