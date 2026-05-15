package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDeliveryNoteSealAndOrderDrawerRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-140",
		"DEV-140-01",
		"DEV-140-02",
		"UT-140-01",
		"API-140-01",
		"REV-140-01",
		"出库单公章设置",
		"订单抽屉",
		"快递信息合并",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("delivery note seal/order drawer requirement seed missing %q", want)
		}
	}
}

func TestDeliveryNoteViewSupportsSharedSealSettings(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "DeliveryNoteView.vue")))
	settings := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CompanySealSettingsView.vue")))
	for _, want := range []string{
		"CompanySealSettingsView",
		"settingsDrawerOpen",
		"公章设置",
		"PDFStampPreview",
		"@placement-commit=\"savePDFPreviewSealPosition\"",
		"savePDFPreviewSealPosition",
		"/api/settings/sales-order/seal-position",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("DeliveryNoteView missing seal marker %q", want)
		}
	}
	for _, want := range []string{
		"公章设置",
		"seal-position-stage",
		"/api/settings/sales-order/seal",
		"/api/settings/sales-order/seal/remove-background",
		"/api/settings/sales-order/seal-position",
	} {
		if !strings.Contains(settings, want) {
			t.Fatalf("CompanySealSettingsView missing marker %q", want)
		}
	}
}

func TestOrdersViewUsesSlimListAndOrderDetailDrawer(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrdersView.vue")))
	for _, want := range []string{
		"orderDetailDrawerOpen",
		"activeOrderDetail",
		"openOrderDetailDrawer(row)",
		`@click.prevent="openOrderDetailDrawer(row)"`,
		"快递信息",
		"订单状态",
		"senderDisplay(row)",
		"status-stack",
		"drawerTrackingNo",
		"fillOrderTracking",
		"/api/orders/${activeOrderDetail.value.id}/shipping-tracking",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("OrdersView missing drawer/list marker %q", want)
		}
	}
	for _, old := range []string{"tracking-box", "trackingInputs", "row-sender"} {
		if strings.Contains(src, old) {
			t.Fatalf("OrdersView should move top-level tracking/list sender UI out of the list, still found %q", old)
		}
	}
}
