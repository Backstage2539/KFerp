package orderresolution

import (
	"encoding/json"
	"testing"
)

func TestProductionReassignRequiresExplicitUnsharedMatchingDemand(t *testing.T) {
	for _, tc := range []struct {
		name        string
		target, qty int
		shared      bool
		valid       bool
	}{{"matching", 41, 3, false, true}, {"no target", 0, 3, false, false}, {"no quantity", 41, 0, false, false}, {"partial work", 41, 2, false, false}, {"other target", 42, 3, false, false}, {"shared", 41, 3, true, false}} {
		t.Run(tc.name, func(t *testing.T) {
			c, cmd := fixture()
			raw := map[string]any{"kind": "production", "id": 19, "status": "running", "shared": tc.shared, "reassign_work_orders": []map[string]any{{"work_order_id": 31, "source_item_id": 1, "quantity": 3, "unit": "袋", "version": "work-v1", "targets": []map[string]any{{"order_id": 4, "order_item_id": 41, "remaining_quantity": 3, "version": "target-v1"}}}}}
			b, _ := json.Marshal([]any{raw})
			_ = json.Unmarshal(b, &c.Dependencies)
			b, _ = json.Marshal([]any{map[string]any{"kind": "production", "id": 19, "choice": "reassign", "reassignments": []map[string]any{{"work_order_id": 31, "target_item_id": tc.target, "quantity": tc.qty}}}})
			_ = json.Unmarshal(b, &cmd.Dispositions)
			p, err := Preview(c, cmd)
			if !tc.valid {
				if err == nil {
					t.Fatal("invalid production transfer accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Tasks) != 1 || p.Tasks[0].Permission != "production.resolve" {
				t.Fatalf("production approval missing: %+v", p.Tasks)
			}
		})
	}
}
