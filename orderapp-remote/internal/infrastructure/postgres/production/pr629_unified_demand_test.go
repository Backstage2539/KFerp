package production

import (
	"os"
	"strings"
	"testing"
)

func TestPR629SalesDemandIgnoresLegacySupplyModeAndKeepsCustomerScope(t *testing.T) {
	b, err := os.ReadFile("unprod_summary.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	if strings.Contains(src, "COALESCE(NULLIF(oi.material_source_mode,'')") {
		t.Fatal("sales production demand must ignore legacy order item supply mode")
	}
	body := src[strings.Index(src, "func fetchUnproducedNeeds"):strings.Index(src, "type productionDemand struct")]
	if strings.Contains(body, "fetchCustomerOrderProductionDemands(ctx") {
		t.Fatal("customer order production demands must be retired from the active planning path")
	}
	for _, want := range []string{"o.customer_id", "snapshot.CustomerID = customerID", "productionQuantitySnapshotGroupKey(snapshot)"} {
		if !strings.Contains(src, want) {
			t.Fatalf("unified demand is missing %q", want)
		}
	}
}
