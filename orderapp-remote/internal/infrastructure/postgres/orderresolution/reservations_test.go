package orderresolution

import (
	"encoding/json"
	"testing"
)

func reservationRows(t *testing.T, raw string) []map[string]json.RawMessage {
	t.Helper()
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestReservationDeductionsCannotBeAppliedTwice(t *testing.T) {
	items := reservationRows(t, `[{"id":1,"product_id":911,"spec":"1000g","qty":6,"unit":"袋"}]`)
	allocations := reservationRows(t, `[{"id":11,"product_id":911,"spec_g":1000,"batch_id":1,"batch_code":"B1","allocated_units":3,"allocated_g":3000},{"id":12,"product_id":911,"spec_g":1000,"batch_id":1,"batch_code":"B1","allocated_units":3,"allocated_g":3000}]`)
	deductions := reservationRows(t, `[{"id":21,"product_id":911,"spec_g":1000,"batch_id":1,"batch_code":"B1","deducted_units":2,"deducted_g":2000}]`)
	if _, issue := reservationDependencies(items, allocations, deductions); issue == "" {
		t.Fatal("ambiguous shipment deducted from both reservations; must require reconciliation")
	}
	deps, issue := reservationDependencies(items, allocations[:1], deductions)
	if issue != "" || len(deps) != 1 || deps[0].Quantity != 1 {
		t.Fatalf("unique deduction: %+v %s", deps, issue)
	}
	deps, issue = reservationDependencies(items, allocations, nil)
	if issue != "" || len(deps) != 2 || deps[0].Quantity+deps[1].Quantity != 6 {
		t.Fatalf("independent unshipped reservations: %+v %s", deps, issue)
	}
}
