package sales

import (
	app "orderapp/internal/application/sales"
	"testing"
)

func TestNamedOrderSnapshotRejectsUnlistedProductAndSpecificationEvenWithManualPrice(t *testing.T) {
	table := app.BeanListVersionOption{ID: 9, ListType: "commercial", TableName: "227g"}
	raw := []byte(`{"price_rows":[{"product_id":10,"bom_spec_id":227,"bom_variant_id":1,"min_qty":1,"max_qty":20,"final_unit_price":30}]}`)
	for _, tc := range []struct {
		product, spec, qty int64
		bad                bool
	}{{10, 227, 2, false}, {10, 1000, 2, true}, {11, 227, 2, true}, {10, 227, 100, true}} {
		err := validateNamedOrderItemSnapshot(raw, table, tc.product, tc.spec, 1, 0, tc.qty, "袋", 0)
		if (err != nil) != tc.bad {
			t.Fatalf("%+v err=%v", tc, err)
		}
	}
}
