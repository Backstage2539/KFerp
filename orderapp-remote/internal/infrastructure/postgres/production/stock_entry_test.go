package production

import (
	"strings"
	"testing"

	productionapp "orderapp/internal/application/production"
)

func TestWorkOrderLedgerWhereTreatsWorkOrderAndRunningItemAsSameEvidenceSet(t *testing.T) {
	where, args := workOrderLedgerWhere(productionapp.WorkOrderLedgerQuery{WorkOrderID: 88, RunningItemID: 99})
	sql := strings.Join(where, " AND ")

	if len(args) != 2 || args[0] != int64(88) || args[1] != int64(99) {
		t.Fatalf("args = %#v, want work_order_id and running_item_id", args)
	}
	if !strings.Contains(sql, "se.work_order_id=$1") || !strings.Contains(sql, "se.running_item_id=$2") {
		t.Fatalf("ledger where missing work order or running item predicate: %s", sql)
	}
	if !strings.Contains(sql, ") OR (se.running_item_id=$2") {
		t.Fatalf("ledger where should OR work_order_id and running_item_id evidence, got: %s", sql)
	}
}

func TestJobCardStartRequiresRunningWorkOrderWithRunningItem(t *testing.T) {
	tests := []struct {
		name          string
		workOrder     string
		runningItemID int64
		want          bool
	}{
		{name: "running", workOrder: "running", runningItemID: 99, want: true},
		{name: "partially completed", workOrder: "partially_completed", runningItemID: 99, want: true},
		{name: "released", workOrder: "released", runningItemID: 0, want: false},
		{name: "running without item", workOrder: "running", runningItemID: 0, want: false},
		{name: "completed", workOrder: "completed", runningItemID: 99, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := jobCardStartAllowedForWorkOrder(tc.workOrder, tc.runningItemID); got != tc.want {
				t.Fatalf("jobCardStartAllowedForWorkOrder(%q, %d)=%v, want %v", tc.workOrder, tc.runningItemID, got, tc.want)
			}
		})
	}
}
