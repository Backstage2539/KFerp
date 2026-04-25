package main

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
	raw, ok := templateFuncMap()["bomURL"]
	if !ok {
		t.Fatal("templateFuncMap missing bomURL")
	}
	bomURL, ok := raw.(func() string)
	if !ok {
		t.Fatalf("bomURL has unexpected type %T", raw)
	}
	if got, want := bomURL(), bomReactURL(); got != want {
		t.Fatalf("bomURL() = %q, want %q", got, want)
	}
}

func TestBomEntrypointsUseVersionedURL(t *testing.T) {
	checks := []struct {
		path      string
		required  []string
		forbidden []string
	}{
		{
			path:      "frontend/src/bom/BomManager.tsx",
			required:  []string{"const BOM_REACT_URL = '/bom-react'", "href: BOM_REACT_URL"},
			forbidden: []string{"href: '/bom-react'"},
		},
		{
			path:      "frontend-vue-shell/src/App.vue",
			required:  []string{"const BOM_REACT_URL = '/bom-react'", "bom: { title: 'BOM配方维护', url: BOM_REACT_URL }"},
			forbidden: []string{"bom: { title: 'BOM配方维护', url: '/bom-react' }"},
		},
		{
			path: "templates/order.html",
			required: []string{
				`href="{{ bomURL }}" onclick="saveDraft()"`,
			},
			forbidden: []string{`href="/bom-react"`},
		},
		{
			path: "templates/bom.html",
			required: []string{
				`href="{{ bomURL }}"`,
			},
			forbidden: []string{`href="/bom-react"`},
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

func TestProductEditTemplateIncludesRoastLevelControls(t *testing.T) {
	b, err := os.ReadFile("templates/product_edit.html")
	if err != nil {
		t.Fatalf("ReadFile(product_edit.html): %v", err)
	}
	src := string(b)
	required := []string{
		`name="roast_level"`,
		`浅烘（82%）`,
		`中烘（81.5%）`,
		`中深烘（81%）`,
		`深烘（80%）`,
		`BOM 出品率按烘焙度自动同步`,
	}
	for _, want := range required {
		if !strings.Contains(src, want) {
			t.Fatalf("product_edit.html missing %q", want)
		}
	}
}

func TestBomTemplateShowsRoastLevelDrivenYield(t *testing.T) {
	b, err := os.ReadFile("templates/bom.html")
	if err != nil {
		t.Fatalf("ReadFile(bom.html): %v", err)
	}
	src := string(b)
	required := []string{
		`按商品档案中的烘焙度自动同步`,
		`浅烘 82%`,
		`中烘 81.5%`,
		`中深烘 81%`,
		`深烘 80%`,
		`烘焙度`,
		`按烘焙度自动同步`,
	}
	for _, want := range required {
		if !strings.Contains(src, want) {
			t.Fatalf("bom.html missing %q", want)
		}
	}
	forbidden := []string{
		`name="yield_rate"`,
		`出品率(0~1)`,
	}
	for _, bad := range forbidden {
		if strings.Contains(src, bad) {
			t.Fatalf("bom.html should not contain %q", bad)
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
