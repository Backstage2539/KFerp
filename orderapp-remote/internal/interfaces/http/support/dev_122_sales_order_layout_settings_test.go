package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSalesOrderLayoutCompanySettingsRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-122",
		"DEV-122-01",
		"DEV-122-02",
		"DEV-122-03",
		"UT-122-01",
		"API-122-01",
		"REV-122-01",
		"全局公司设置",
		"销售单预览与PDF一致",
		"公章位置",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("sales order layout/company setting requirement seed missing %q", want)
		}
	}
}

func TestSalesOrderPreviewRendersPDFAssetsAndMultilineText(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderView.vue")))
	for _, want := range []string{
		"PDFStampPreview",
		":pdf-url=\"salesOrderPreviewPDFUrl\"",
		"preview-label=\"PREVIEW 预览版\"",
		"previewSealUrl",
		"salesOrderPreviewPlacements",
		"onPreviewPDFLoaded",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("SalesOrderView missing sales-order PDF parity marker %q", want)
		}
	}
}

func TestSalesOrderSettingsMovesCompanyNameAndUsesSharedSealAssets(t *testing.T) {
	settings := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderSettingsView.vue")))
	for _, forbidden := range []string{
		`v-model.trim="form.company_name"`,
		`<span>公司名称</span>`,
		"seal-position-stage",
		"beginSalesOrderSealDrag",
	} {
		if strings.Contains(settings, forbidden) {
			t.Fatalf("SalesOrderSettingsView should not keep company name setting marker %q", forbidden)
		}
	}
	for _, want := range []string{
		"selectSeal",
		"/api/settings/sales-order/seals",
		"/api/settings/sales-order/seal/select",
		"/api/settings/sales-order/seal/remove-background",
		"apiGet('/api/settings/sales-order')",
		"apiSend('/api/settings/sales-order'",
	} {
		if !strings.Contains(settings, want) {
			t.Fatalf("SalesOrderSettingsView missing shared seal settings marker %q", want)
		}
	}
	if strings.Contains(settings, "fetch('/api/settings/sales-order") {
		t.Fatalf("SalesOrderSettingsView should use the shared API client for authenticated sales order setting requests")
	}
}

func TestGlobalCompanyProfileVueWiring(t *testing.T) {
	app := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "App.vue")))
	menu := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js")))
	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CompanyProfileView.vue")))
	for _, want := range []string{
		"CompanyProfileView",
		"companyProfile",
	} {
		if !strings.Contains(app, want) && !strings.Contains(menu, want) {
			t.Fatalf("global company profile Vue wiring missing %q", want)
		}
	}
	for _, want := range []string{
		"/api/company/profile",
		"公司名称",
		"公司地址",
		"联系电话",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("CompanyProfileView missing %q", want)
		}
	}
}
