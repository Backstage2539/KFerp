package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPR667CompleteUIFlowRequirementSeeds(t *testing.T) {
	source := readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	for _, want := range []string{
		"PR-668-PR667-COMPLETE-UI-FLOW",
		"DEV-694-SHARED-PROCESSING-DETAIL",
		"DEV-695-ERP-CUSTOMER-PROCESSING-UI",
		"DEV-696-ERP-PRODUCTION-TRACE-UI",
		"DEV-697-MINIAPP-PROCESSING-UI",
		"DEV-698-MINIAPP-ORDER-INVENTORY-UI",
		"DEV-699-VISUAL-FLOW-DELIVERY",
		"REV-668-PR667-COMPLETE-UI-FLOW",
	} {
		if !strings.Contains(string(source), want) {
			t.Fatalf("PR-667 complete UI-flow requirement seed missing %q", want)
		}
	}
}
