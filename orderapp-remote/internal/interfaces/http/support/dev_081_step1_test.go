package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCostingCommercialTierSchemeRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-081",
		"DEV-081-01",
		"DEV-081-02",
		"UT-081-01",
		"API-081-01",
		"REV-081-01",
		"Nenka",
		"24-49kg",
		"2包-7包",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("requirements seed missing %q", want)
		}
	}
}
