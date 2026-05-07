package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSalesOrderPreviewSealToolsRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-124",
		"DEV-124-01",
		"DEV-124-02",
		"DEV-124-03",
		"UT-124-01",
		"API-124-01",
		"REV-124-01",
		"销售单预览公章拖动",
		"公章去除背景",
		"收款码并排展示",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("sales order preview seal tools requirement seed missing %q", want)
		}
	}
}

func TestSalesOrderPreviewSupportsDraggingSealAndInlinePaymentCodes(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderView.vue")))
	for _, want := range []string{
		`ref="previewSealStage"`,
		`@pointerdown.stop="startPreviewSealDrag"`,
		"savePreviewSealPosition",
		"/api/settings/sales-order/seal-position",
		"payment-code-preview-list",
		"single-payment-code",
		"payment-code-stack",
		"payment-code-preview-card",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("SalesOrderView missing preview seal/payment-code marker %q", want)
		}
	}
	if strings.Contains(src, "pointer-events: none;") && strings.Contains(src, ".seal-stamp-preview") && !strings.Contains(src, "cursor: move") {
		t.Fatalf("preview seal should be draggable, not pointer-events none without a move cursor")
	}
}

func TestSalesOrderSettingsExposesSealBackgroundRemoval(t *testing.T) {
	settings := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderSettingsView.vue")))
	api := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "sales_order_settings.go")))
	for _, want := range []string{
		"去除背景",
		"removeSealBackground",
		"/api/settings/sales-order/seal/remove-background",
	} {
		if !strings.Contains(settings, want) && !strings.Contains(api, want) {
			t.Fatalf("seal background removal marker missing %q", want)
		}
	}
}

func TestSalesOrderPDFRendersPaymentCodesWithAdaptiveLayout(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "pdf", "sales_order_pdf.go")))
	for _, want := range []string{
		"renderPaymentCodes",
		"salesOrderPaymentCodeMetrics",
		"Stacked",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("sales order PDF missing adaptive payment code marker %q", want)
		}
	}
}
