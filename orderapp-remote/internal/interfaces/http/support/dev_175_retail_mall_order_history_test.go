package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRetailMallCustomersCanReachOrderHistoryFromMiniapp(t *testing.T) {
	capabilities := string(readOrderAppFileForTest(t, filepath.Join("..", "miniapp", "src", "utils", "capabilities.ts")))
	tabBar := string(readOrderAppFileForTest(t, filepath.Join("..", "miniapp", "src", "components", "MainTabBar.vue")))
	service := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "customerportal", "service.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md")))

	for _, want := range []string{
		"key: 'mall'",
		"url: '/pages/mall/mall'",
	} {
		if !strings.Contains(capabilities, want) {
			t.Fatalf("miniapp capabilities missing retail mall order history marker %q", want)
		}
	}
	for _, want := range []string{
		"订单",
		"/pages/service/service?key=orders",
		"uni.reLaunch",
	} {
		if !strings.Contains(tabBar, want) {
			t.Fatalf("miniapp main tab bar missing order-history marker %q", want)
		}
	}
	if !strings.Contains(service, "CapabilityMall") || !strings.Contains(service, "ServiceKeyOrders") {
		t.Fatal("customer portal service must allow mall customers to query order history")
	}
	for _, want := range []string{
		"底部“订单”",
		"查看已提交商城订单记录",
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
