package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSalesOrderDrawerSettingsRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-125",
		"DEV-125-01",
		"DEV-125-02",
		"DEV-125-03",
		"UT-125-01",
		"API-125-01",
		"REV-125-01",
		"订单列表销售单抽屉",
		"销售单设置抽屉",
		"公章大小",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("sales order drawer settings requirement seed missing %q", want)
		}
	}
}

func TestOrdersViewOpensSalesOrderDrawerInPlace(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrdersView.vue")))
	for _, want := range []string{
		"SalesOrderView",
		"salesOrderDrawerOpen",
		"activeSalesOrderID",
		"openSalesOrderDrawer(row)",
		`@click.prevent="openSalesOrderDrawer(row)"`,
		"sales-order-drawer-mask",
		"closeSalesOrderDrawer",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("OrdersView missing sales order drawer marker %q", want)
		}
	}
	for _, unwanted := range []string{
		"salesOrderPageUrl",
		`:href="salesOrderPageUrl(row.id)"`,
	} {
		if strings.Contains(src, unwanted) {
			t.Fatalf("OrdersView should open the sales order drawer in place, still found %q", unwanted)
		}
	}
}

func TestSalesOrderViewEmbedsSettingsDrawer(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderView.vue")))
	for _, want := range []string{
		"defineProps",
		"defineEmits",
		"embedded",
		"orderId",
		"SalesOrderSettingsView",
		"settingsDrawerOpen",
		"openSettingsDrawer",
		"closeSettingsDrawer",
		"销售单设置",
		"settings-drawer-mask",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("SalesOrderView missing embedded settings drawer marker %q", want)
		}
	}
}

func TestSalesOrderSettingsExposesSealSizeControl(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderView.vue")))
	for _, want := range []string{
		"公章大小",
		"seal-size-slider",
		`type="range"`,
		"previewSealWidthMM",
		"savePreviewSealSize",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("SalesOrderView missing seal size marker %q", want)
		}
	}
}
