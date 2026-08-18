package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev601BomSpecUiPolishContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "lib", "bom.js"): {
			"materialOptionLabel",
			"nextSpecKey",
			"物料 #",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"): {
			"materialOptionLabel",
			"nextSpecKey",
			"规格用量",
			"规格主体物料",
			"legacy-loss-display",
			"bom-spec-template-field",
			"reapply-action-spacer",
			"identity-action-spacer",
			"checkbox-row compact-checkbox",
			".spec-variant-grid .checkbox-row input",
		},
		filepath.Join("internal", "infrastructure", "postgres", "bom", "repository.go"): {
			"COALESCE(NULLIF(code,''),'')",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-601 marker %q", rel, want)
			}
		}
	}

	bomView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue")))
	template := strings.Split(bomView, "<script setup>")[0]
	for _, banned := range []string{"稳定规格键", "主投入用量", "损耗比例 %", "原料损耗比"} {
		if strings.Contains(template, banned) {
			t.Fatalf("BomView template still exposes retired control %q", banned)
		}
	}
	if strings.Contains(bomView, "versionMaterialLossRateEnabled") {
		t.Fatal("version loss-rate toggle state must be removed")
	}
}

func TestDev601MaterialOptionLabelNeverFabricatesSkuPrefix(t *testing.T) {
	bomLib := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "bom.js")))
	materialFn := bomLib[strings.Index(bomLib, "export function materialOptionLabel"):]
	materialFn = materialFn[:strings.Index(materialFn, "\n}")]
	if strings.Contains(materialFn, "SKU-") {
		t.Fatal("materialOptionLabel must not reference SKU- fabrication")
	}
	productFn := bomLib[strings.Index(bomLib, "export function bomProductCode"):]
	productFn = productFn[:strings.Index(productFn, "\n}")]
	if !strings.Contains(productFn, "SKU-") {
		t.Fatal("bomProductCode must keep SKU- fabrication for products")
	}
}
