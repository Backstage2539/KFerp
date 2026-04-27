package customer

import (
	"os"
	"strings"
	"testing"
)

func TestFetchCustomerDashboardCoalescesEmptyAggregates(t *testing.T) {
	src, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, want := range []string{
		"COALESCE(SUM(CASE WHEN COALESCE(o.pay_status_id,0) <> 2 THEN 1 ELSE 0 END),0) AS unpaid",
		"COALESCE(SUM(CASE WHEN COALESCE(o.ship_status_id,0) IN (0,1,2) THEN 1 ELSE 0 END),0) AS unshipped",
		"COALESCE(SUM(CASE WHEN $2>0 AND COALESCE(o.process_status_id,0) = $2 THEN 1 ELSE 0 END),0) AS in_prod",
		"COALESCE(SUM(CASE WHEN $3>0 AND COALESCE(o.process_status_id,0) = $3 THEN 1 ELSE 0 END),0) AS in_ship",
		"COALESCE(SUM(CASE WHEN COALESCE(o.pay_status_id,0)=2 AND COALESCE(o.ship_status_id,0) IN (3,4) THEN 1 ELSE 0 END),0) AS completed",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("fetchCustomerDashboard missing aggregate null guard %q", want)
		}
	}
}
