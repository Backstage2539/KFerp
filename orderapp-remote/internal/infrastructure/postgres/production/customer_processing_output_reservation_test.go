package production

import (
	"os"
	"strings"
	"testing"
)

func TestFinishConvertsCustomerProcessingOutputReservationToOrderBatchAllocation(t *testing.T) {
	running, err := os.ReadFile("running_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(running), "allocateCustomerProcessingOutputReservationsTx(") < 2 {
		t.Fatal("single and multi-output completion must both convert customer processing reservations")
	}
	helper, err := os.ReadFile("customer_processing_output_reservation.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"customer_processing_output_reservations",
		"customer_processing_output_conversions",
		"order_stock_batch_allocations",
		"converted_qty",
		"processing_output_convert",
		"completeOrderIfAllRunningDone",
	} {
		if !strings.Contains(string(helper), want) {
			t.Fatalf("processing output conversion missing %q", want)
		}
	}
}
