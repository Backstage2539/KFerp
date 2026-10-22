package orderweight_test

import (
	"context"
	"fmt"
	"testing"

	weights "orderapp/internal/infrastructure/postgres/orderweight"
)

func TestFreezeOrderSkipsRetiredZeroButStillRejectsNegativeQuantity(t *testing.T) {
	p, s := weightDB(t)
	ctx := context.Background()
	if _, err := p.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_items(id,order_id,product_id,bom_spec_id,bom_variant_id,qty,unit,weight_snapshot_json) VALUES(2,1,999,0,0,0,'袋','{"historical":"retained"}')`, s)); err != nil {
		t.Fatal(err)
	}
	tx, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = weights.FreezeOrder(ctx, tx, s, 1); err != nil {
		t.Fatalf("retired row prevented valid lines from freezing: %v", err)
	}
	var raw string
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT weight_snapshot_json->>'historical' FROM %s.order_items WHERE id=2`, s)).Scan(&raw); err != nil || raw != "retained" {
		t.Fatalf("retired historical snapshot rewritten: %q %v", raw, err)
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_items SET qty=-1 WHERE id=2`, s)); err != nil {
		t.Fatal(err)
	}
	if err = weights.FreezeOrder(ctx, tx, s, 1); err == nil {
		t.Fatal("negative quantity was hidden instead of rejected")
	}
}
