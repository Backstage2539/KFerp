package appmain

import "testing"

func TestIsRetailOrderTypeName(t *testing.T) {
	if !isRetailOrderTypeName("零售") {
		t.Fatalf("expected 零售 to be retail")
	}
	if !isRetailOrderTypeName("Retail Order") {
		t.Fatalf("expected Retail Order to be retail")
	}
	if isRetailOrderTypeName("批发") {
		t.Fatalf("expected 批发 not to be retail")
	}
}
