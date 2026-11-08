package orderresolution

import (
	"encoding/json"
	"testing"
)

func TestShipmentQuantitiesRequireExactHistoricalAllocation(t *testing.T) {
	raw := func(s string) map[string]json.RawMessage {
		t.Helper()
		var m map[string]json.RawMessage
		if err := json.Unmarshal([]byte(s), &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	item := raw(`{"id":7,"product_id":1,"bom_spec_id":2,"bom_variant_id":4,"qty":3}`)
	deduction := raw(`{"order_id":6,"product_id":1,"bom_spec_id":2,"bom_variant_id":3,"batch_id":9,"batch_code":"ORIGINAL","deducted_units":3}`)
	allocation := raw(`{"order_id":6,"order_item_id":7,"product_id":1,"bom_spec_id":2,"bom_variant_id":3,"batch_id":9,"batch_code":"ORIGINAL","allocated_units":3,"warehouse":"finished_goods"}`)
	q, issue := shipmentQuantities([]map[string]json.RawMessage{item}, []map[string]json.RawMessage{deduction}, []map[string]json.RawMessage{allocation})
	if issue != "" || q[7] != 3 {
		t.Fatalf("verified original allocation lost: %v %s", q, issue)
	}
	for field, value := range map[string]string{"order_id": "99", "order_item_id": "99", "product_id": "99", "bom_spec_id": "99", "bom_variant_id": "4", "batch_id": "99", "batch_code": "\"OTHER\"", "allocated_units": "2", "allocated_g": "1", "spec_g": "1", "warehouse": "\"\""} {
		t.Run(field, func(t *testing.T) {
			changed := map[string]json.RawMessage{}
			for k, v := range allocation {
				changed[k] = v
			}
			changed[field] = json.RawMessage(value)
			_, issue := shipmentQuantities([]map[string]json.RawMessage{item}, []map[string]json.RawMessage{deduction}, []map[string]json.RawMessage{changed})
			if issue == "" {
				t.Fatal("wrong historical allocation accepted")
			}
		})
	}
}
