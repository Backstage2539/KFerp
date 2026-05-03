package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerPortalBusinessTaskRequirementSeeds(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatalf("read req_store.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"PR-CUSTOMER-PORTAL-BUSINESS-TASKS",
		"DEV-CUSTOMER-PORTAL-BUSINESS-TASK-01",
		"DEV-CUSTOMER-PORTAL-BUSINESS-TASK-02",
		"DEV-CUSTOMER-PORTAL-BUSINESS-TASK-03",
		"DEV-CUSTOMER-PORTAL-BUSINESS-TASK-04",
		"DEV-CUSTOMER-PORTAL-BUSINESS-TASK-05",
		"DEV-CUSTOMER-PORTAL-BUSINESS-TASK-06",
		"UT-CUSTOMER-PORTAL-BUSINESS-TASKS-01",
		"API-CUSTOMER-PORTAL-BUSINESS-TASKS-01",
		"REV-CUSTOMER-PORTAL-BUSINESS-TASKS-01",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("customer portal business task seed missing %q", want)
		}
	}
}
