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
			required:  []string{"const BOM_REACT_URL = '/bom-react?rev=20260424-2'", "href: BOM_REACT_URL"},
			forbidden: []string{"href: '/bom-react'"},
		},
		{
			path:      "frontend-vue-shell/src/App.vue",
			required:  []string{"const BOM_REACT_URL = '/bom-react?rev=20260424-2'", "bom: { title: 'BOM配方维护', url: BOM_REACT_URL }"},
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
