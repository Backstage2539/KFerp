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
