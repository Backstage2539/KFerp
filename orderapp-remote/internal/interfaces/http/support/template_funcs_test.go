package support

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestRetailPriceLinesOnlyReturnsConfiguredSpecs(t *testing.T) {
	got := retailPriceLines(42, 0, 50, 0)
	want := []string{"100g 42.00", "227g 50.00"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("retailPriceLines() = %#v, want %#v", got, want)
	}
}

func TestTemplateFuncMapExposesBomURL(t *testing.T) {
	raw, ok := TemplateFuncMap()["bomURL"]
	if !ok {
		t.Fatal("templateFuncMap missing bomURL")
	}
	bomURL, ok := raw.(func() string)
	if !ok {
		t.Fatalf("bomURL has unexpected type %T", raw)
	}
	if got, want := bomURL(), "/vue-shell?view=productionConfig&tab=bom"; got != want {
		t.Fatalf("bomURL() = %q, want %q", got, want)
	}
}

func TestBomEntrypointsUseVueURL(t *testing.T) {
	checks := []struct {
		path      string
		required  []string
		forbidden []string
	}{
		{
			path:      "frontend-vue-shell/src/views/ProductionSettingsView.vue",
			required:  []string{"import BomView from './BomView.vue'", "key: 'bom'", "label: '生产 BOM'"},
			forbidden: []string{"BOM_REACT_URL", "legacyUrl: BOM_REACT_URL"},
		},
		{
			path:     "frontend-vue-shell/src/lib/menu-ia.js",
			required: []string{"bom: '生产 BOM'"},
		},
	}

	for _, tc := range checks {
		b, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatalf("ReadFile(%q): %v", tc.path, err)
		}
		src := string(b)
		for _, want := range tc.required {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing %q", tc.path, want)
			}
		}
		for _, bad := range tc.forbidden {
			if strings.Contains(src, bad) {
				t.Fatalf("%s still contains %q", tc.path, bad)
			}
		}
	}
}

func TestMaterialSummaryTextFormatsWeightAndCountItems(t *testing.T) {
	raw := `[{"unit":"g","deduct_g":568,"material_id":1,"deduct_units":0,"material_name":"孟连水洗5T批次"},{"unit":"个","deduct_g":0,"material_id":2,"deduct_units":1,"material_name":"227g豆袋"}]`
	got := materialSummaryText(raw)
	want := "孟连水洗5T批次 扣减 568g\n227g豆袋 扣减 1个"
	if got != want {
		t.Fatalf("materialSummaryText() = %q, want %q", got, want)
	}
}

func TestMaterialSummaryTextFallsBackForInvalidJSON(t *testing.T) {
	got := materialSummaryText("not-json")
	if got != "not-json" {
		t.Fatalf("materialSummaryText() = %q, want raw string", got)
	}
}
