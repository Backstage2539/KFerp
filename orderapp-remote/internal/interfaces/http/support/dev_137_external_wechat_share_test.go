package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestExternalWechatShareRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-137",
		"DEV-137-01",
		"DEV-137-02",
		"DEV-137-03",
		"UT-137-01",
		"API-137-01",
		"REV-137-01",
		"订单和出库单支持分享到微信",
		"外部分享资源统一逻辑",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("external WeChat share requirement seed missing %q", want)
		}
	}
}

func TestSalesAndDeliveryViewsUseSharedExternalShareHelper(t *testing.T) {
	salesView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderView.vue")))
	deliveryView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "DeliveryNoteView.vue")))
	helper := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "external-share.js")))
	for _, want := range []string{
		"buildShareResourcePayload",
		"shareResourceToWechat",
		"/api/share-resources",
		"分享到微信",
	} {
		if !strings.Contains(salesView+"\n"+deliveryView+"\n"+helper, want) {
			t.Fatalf("sales/delivery external share UI missing shared marker %q", want)
		}
	}
	if strings.Count(salesView+"\n"+deliveryView, "/api/share-resources") != 2 {
		t.Fatal("sales and delivery pages should each call the same /api/share-resources endpoint exactly once")
	}
}
