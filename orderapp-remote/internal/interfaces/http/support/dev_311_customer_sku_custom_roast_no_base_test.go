package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev311CustomerSkuCustomRoastNoBaseProduct(t *testing.T) {
	reqStore := string(readDev311File(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-311-CUSTOM-ROAST-NO-BASE",
		"DEV-311-CUSTOM-ROAST-NO-BASE",
		"UT-311-CUSTOM-ROAST-NO-BASE",
		"API-311-CUSTOM-ROAST-NO-BASE",
		"REV-311-CUSTOM-ROAST-NO-BASE",
	} {
		if !strings.Contains(reqStore, want) {
			t.Fatalf("req_store.go missing custom roast no-base seed %q", want)
		}
	}

	productSettings := string(readDev311File(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	for _, want := range []string{
		`v-if="customForm.product_kind !== 'green_bean' && customForm.custom_type !== 'custom_roast'" class="wide-field"`,
		`customForm.value.custom_type !== 'custom_roast'`,
		`customForm.value.custom_type === 'custom_roast'`,
	} {
		if !strings.Contains(productSettings, want) {
			t.Fatalf("ProductSettingsView.vue missing custom roast no-base wiring %q", want)
		}
	}
	if strings.Contains(productSettings, `value="custom_blend">定制拼配 BOM`) {
		t.Fatalf("ProductSettingsView.vue should not expose custom blend BOM as a creation custom type")
	}

	productSettingsLib := string(readDev311File(t, filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js")))
	for _, want := range []string{
		`payload.custom_type === 'custom_roast'`,
		`payload.base_product_id = 0`,
		`payload.copy_bom = false`,
		`payload.copy_price_tiers = false`,
	} {
		if !strings.Contains(productSettingsLib, want) {
			t.Fatalf("product-settings.js missing custom roast payload guard %q", want)
		}
	}

	requirements := string(readDev311File(t, "docs/REQUIREMENTS.md"))
	if !strings.Contains(requirements, "定制烘焙度不选择基础产品") {
		t.Fatalf("docs/REQUIREMENTS.md missing custom roast no-base requirement")
	}
}

func readDev311File(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
