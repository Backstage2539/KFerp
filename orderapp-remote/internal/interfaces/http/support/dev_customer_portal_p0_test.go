package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerPortalP0RequirementSeeds(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatalf("read req_store.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"PR-CUSTOMER-PORTAL-P0",
		"DEV-CUSTOMER-PORTAL-P0-01",
		"DEV-CUSTOMER-PORTAL-P0-02",
		"DEV-CUSTOMER-PORTAL-P0-03",
		"DEV-CUSTOMER-PORTAL-P0-04",
		"UT-CUSTOMER-PORTAL-P0-01",
		"API-CUSTOMER-PORTAL-P0-01",
		"REV-CUSTOMER-PORTAL-P0-01",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("customer portal P0 seed missing %q", want)
		}
	}
}
