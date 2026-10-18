package production

import (
	"context"
	"fmt"
	"testing"
)

// An already received first batch must not make the remaining frozen stock
// insufficient just because each receipt independently rounds up to grams.
func TestBOMSpecPartialReceiptDeductsCumulativeRoundedMaterialDelta(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); pool.Close() })
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
	CREATE TABLE %s.production_logs(running_item_id BIGINT,input_g BIGINT,finished_units BIGINT,finished_loose_g BIGINT,finished_total_g BIGINT);
	INSERT INTO %s.production_logs VALUES(57,6811,12,0,0);
	`, schema, schema))
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	run := ProduceRunRow{ID: 57, BomSpecID: 463, PlanUnits: 12, InputG: 6810,
		MaterialSnapshot: `[{"material_id":6,"material_name":"孟连水洗A","unit":"kg","source":"bom","consume_unit":"kg","qty_per_unit":0.28375,"output_qty":1,"output_unit":"袋"}]`}
	needs, err := calcRunningItemMaterialNeedsTx(ctx, tx, schema, run, InvQty{Units: 12})
	if err != nil {
		t.Fatal(err)
	}
	if len(needs) != 1 || needs[0].DeductG != 3405 {
		t.Fatalf("last 12 bags material needs = %+v, want remaining 3405g after first 3406g of frozen 6811g", needs)
	}
	// A second running item has no receipt history and must still require its
	// entire rounded quantity; history must never leak across work orders.
	run.ID = 58
	needs, err = calcRunningItemMaterialNeedsTx(ctx, tx, schema, run, InvQty{Units: 12})
	if err != nil {
		t.Fatal(err)
	}
	if len(needs) != 1 || needs[0].DeductG != 3406 {
		t.Fatalf("unrelated run needs=%+v, want 3406g", needs)
	}
}
