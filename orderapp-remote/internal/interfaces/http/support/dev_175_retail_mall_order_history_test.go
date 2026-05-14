package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRetailMallCustomersCanReachOrderHistoryFromMiniapp(t *testing.T) {
	capabilities := string(readOrderAppFileForTest(t, filepath.Join("..", "miniapp", "src", "utils", "capabilities.ts")))
	mallPage := string(readOrderAppFileForTest(t, filepath.Join("..", "miniapp", "src", "pages", "mall", "mall.vue")))
	service := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "customerportal", "service.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md")))

	for _, want := range []string{
		"capabilities: ['product_order', 'direct_ship', 'shipping_query', 'mall']",
		"/pages/service/service?key=orders",
	} {
		if !strings.Contains(capabilities, want) {
			t.Fatalf("miniapp capabilities missing retail mall order history marker %q", want)
		}
	}
	for _, want := range []string{
		"function openOrders()",
		"uni.navigateTo({ url: '/pages/service/service?key=orders' })",
		"我的订单",
	} {
		if !strings.Contains(mallPage, want) {
			t.Fatalf("miniapp mall page missing order-history marker %q", want)
		}
	}
	if !strings.Contains(service, "CapabilityMall") || !strings.Contains(service, "ServiceKeyOrders") {
		t.Fatal("customer portal service must allow mall customers to query order history")
	}
	for _, want := range []string{
		"商城页顶部保留“我的订单”",
		"查看商城订单记录",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("customer portal manual missing retail mall order-history instruction %q", want)
		}
	}
}

func TestRetailMallOrderHistoryRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-175-RETAIL-MALL-ORDER-HISTORY",
		"DEV-175-RETAIL-MALL-ORDER-HISTORY",
		"UT-175-RETAIL-MALL-ORDER-HISTORY",
		"API-175-RETAIL-MALL-ORDER-HISTORY",
		"REV-175-RETAIL-MALL-ORDER-HISTORY",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}
