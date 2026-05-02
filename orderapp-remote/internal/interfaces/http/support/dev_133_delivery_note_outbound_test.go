package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDeliveryNoteOutboundRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-133",
		"DEV-133-01",
		"DEV-133-02",
		"DEV-133-03",
		"UT-133-01",
		"API-133-01",
		"REV-133-01",
		"订单发货后维护出库单",
		"出库单预览",
		"出库单 PDF",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("delivery note outbound requirement seed missing %q", want)
		}
	}
}

func TestOrdersViewOpensDeliveryNoteDrawerInPlace(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrdersView.vue")))
	for _, want := range []string{
		"DeliveryNoteView",
		"deliveryNoteDrawerOpen",
		"activeDeliveryNoteID",
		"openDeliveryNoteDrawer(row)",
		`@click.prevent="openDeliveryNoteDrawer(row)"`,
		"delivery-note-drawer-mask",
		"closeDeliveryNoteDrawer",
		"出库单",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("OrdersView missing delivery note drawer marker %q", want)
		}
	}
}

func TestDeliveryNoteVueShellWiring(t *testing.T) {
	app := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "App.vue")))
	menu := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js")))
	authz := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "authz", "schema.go")))
	for _, want := range []string{
		"import DeliveryNoteView from './views/DeliveryNoteView.vue'",
		"deliveryNote: DeliveryNoteView",
		"deliveryNote: '出库单'",
		`"deliveryNote":`,
	} {
		if !strings.Contains(app+"\n"+menu+"\n"+authz, want) {
			t.Fatalf("delivery note vue shell wiring missing %q", want)
		}
	}
}

func TestDeliveryNoteViewRequiresPreviewBeforeGenerate(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "DeliveryNoteView.vue")))
	for _, want := range []string{
		"出库单预览",
		"confirmGenerateDeliveryNote",
		"!preview",
		"/api/orders/${orderID.value}/delivery-note-preview",
		"/api/orders/${orderID.value}/delivery-notes",
		"deliveryNoteDownloadUrl",
		"出库日期",
		"出库仓库",
		"快递单号",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("DeliveryNoteView missing preview/generate marker %q", want)
		}
	}
}
