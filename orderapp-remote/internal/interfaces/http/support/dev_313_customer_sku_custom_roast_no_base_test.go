package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev313CustomerSkuCustomRoastNoBaseProduct(t *testing.T) {
	reqStore := string(readDev313File(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-313-CUSTOM-ROAST-NO-BASE",
		"DEV-313-CUSTOM-ROAST-NO-BASE",
		"UT-313-CUSTOM-ROAST-NO-BASE",
		"API-313-CUSTOM-ROAST-NO-BASE",
		"REV-313-CUSTOM-ROAST-NO-BASE",
	} {
		if !strings.Contains(reqStore, want) {
			t.Fatalf("req_store.go missing custom roast no-base seed %q", want)
		}
	}

		productSettings := string(readDev313File(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
		for _, want := range []string{
			`@submit.prevent="createSku"`,
			"buildSkuCreatePayload(skuContextCustomerID.value, skuForm.value)",
		} {
		if !strings.Contains(productSettings, want) {
			t.Fatalf("ProductSettingsView.vue missing unified SKU create wiring %q", want)
		}
	}
	for _, forbidden := range []string{
			`v-else class="custom-product-form product-drawer-form"`,
			`value="custom_blend">定制拼配 BOM`,
			"skuForm.product_type_category_id",
			"skuForm.product_subtype_category_id",
		} {
		if strings.Contains(productSettings, forbidden) {
			t.Fatalf("ProductSettingsView.vue should not expose legacy custom SKU create wiring %q", forbidden)
		}
	}

	productSettingsLib := string(readDev313File(t, filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js")))
	for _, want := range []string{
		"buildSkuCreatePayload",
		"product_config_template_id",
	} {
		if !strings.Contains(productSettingsLib, want) {
			t.Fatalf("product-settings.js missing unified SKU payload marker %q", want)
		}
	}
	skuPayloadStart := strings.Index(productSettingsLib, "export function buildSkuCreatePayload")
	skuPayloadEnd := strings.Index(productSettingsLib, "export function buildSkuCopyPayload")
	if skuPayloadStart < 0 || skuPayloadEnd <= skuPayloadStart {
		t.Fatalf("product-settings.js missing buildSkuCreatePayload function block")
	}
	if strings.Contains(productSettingsLib[skuPayloadStart:skuPayloadEnd], "special_attrs_json") {
		t.Fatalf("buildSkuCreatePayload should not write legacy SKU special_attrs_json after product production config migration")
	}

	requirements := string(readDev313File(t, "docs/REQUIREMENTS.md"))
	if !strings.Contains(requirements, "定制烘焙度不选择基础产品") {
		t.Fatalf("docs/REQUIREMENTS.md missing custom roast no-base requirement")
	}
}

func readDev313File(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
