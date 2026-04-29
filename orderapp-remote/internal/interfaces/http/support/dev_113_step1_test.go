package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSalesOrderPDFRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, needle := range []string{
		"PR-SALES-ORDER-001",
		"DEV-SALES-ORDER-001",
		"UT-SALES-ORDER-001",
		"API-SALES-ORDER-001",
		"REV-SALES-ORDER-001",
	} {
		if !strings.Contains(src, needle) {
			t.Fatalf("sales order pdf requirement seed missing %q", needle)
		}
	}
}
