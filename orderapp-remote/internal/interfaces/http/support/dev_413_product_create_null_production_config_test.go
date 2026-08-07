package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev413ProductCreateNullProductionConfigSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-413-PRODUCT-CREATE-NULL-PRODUCTION-CONFIG",
		"DEV-413-PRODUCT-CREATE-NULL-CONFIG-GUARD",
		"UT-413-PRODUCT-CREATE-NULL-PRODUCTION-CONFIG",
		"REV-413-PRODUCT-CREATE-NULL-PRODUCTION-CONFIG",
		"expected_loss_rate",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("req_store.go missing product create null production config marker %q", want)
		}
	}
}

func TestDev413ProductCreateNullProductionConfigSourceMarkers(t *testing.T) {
	sources := map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"buildProductProductionConfigForm",
			"const sourceConfig = config && typeof config === 'object' ? config : {}",
			"sourceConfig.expected_loss_rate ?? sourceProduct.expected_loss_rate ?? 0",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"buildProductProductionConfigForm",
			"function defaultProductProductionConfigForm(config = {}, product = {})",
			"return buildProductProductionConfigForm(config, product, industryFieldTemplatesForConfig(config))",
			"productProductionConfigForm.value = defaultProductProductionConfigForm(config, row)",
			"await openProductProductionConfig(createdProductForConfig)",
		},
	}

	for rel, wants := range sources {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing product create null production config marker %q", rel, want)
			}
		}
	}
}
