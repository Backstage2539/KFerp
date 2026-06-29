package inventory

import (
	"os"
	"strings"
	"testing"
)

func TestInventoryProductsHideTemplateRemovedDerivedSKUs(t *testing.T) {
	src, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	source := string(src)
	for _, marker := range []string{
		"COALESCE(auto_derived_sku,false)",
		"derived_spec_status",
		"COALESCE(NULLIF(derived_spec_status,''),'active')<>'template_removed'",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("inventory product candidate SQL missing %q", marker)
		}
	}
}
