package production

import (
	"context"
	"fmt"
	"testing"
)

func TestCustomerProcessingBatchSplitPreservesUnknownCost(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	defer func() { _, _ = pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); pool.Close() }()
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`CREATE TABLE %[1]s.stock_batches(id bigserial primary key,batch_code text,item_type text,item_id bigint,item_name text,spec_g bigint,bom_spec_id bigint not null default 0,bom_variant_id bigint not null default 0,owner_customer_id bigint not null default 0,source_doc_type text,source_doc_id bigint,source_batch_id text,qty_g bigint,qty_units bigint,operator text,created_at timestamptz default now(),remaining_g bigint,remaining_units bigint,unit_cost numeric,quality_status text,cost_unavailable boolean not null default false);
 INSERT INTO %[1]s.stock_batches(id,batch_code,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,owner_customer_id,source_doc_type,source_doc_id,source_batch_id,qty_g,qty_units,operator,remaining_g,remaining_units,unit_cost,quality_status,cost_unavailable) VALUES(11,'SOURCE','finished_product',1,'半成品',1000,21,22,101,'prod',1,'',1000,1,'test',1000,1,12,'pass',true);
 SELECT setval(pg_get_serial_sequence('%[1]s.stock_batches','id'),11,true);`, schema))
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	got, err := splitCustomerProcessingFinishedBatchForIssueTx(ctx, tx, schema, customerProcessingMaterialReservation{ID: 1, ComponentSpecG: 1000}, customerProcessingFinishedBatchAllocation{StockBatchID: 11, BatchCode: "SOURCE", QtyG: 400}, 1, "test")
	if err != nil {
		t.Fatal(err)
	}
	var unavailable bool
	var owner, spec, variant int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT cost_unavailable,owner_customer_id,bom_spec_id,bom_variant_id FROM %s.stock_batches WHERE id=$1`, schema), got.StockBatchID).Scan(&unavailable, &owner, &spec, &variant); err != nil || !unavailable || owner != 101 || spec != 21 || variant != 22 {
		t.Fatalf("child unavailable=%v owner=%d spec=%d variant=%d err=%v", unavailable, owner, spec, variant, err)
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.stock_batches(id,batch_code,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,owner_customer_id,source_doc_type,source_doc_id,source_batch_id,qty_g,qty_units,operator,remaining_g,remaining_units,unit_cost,quality_status,cost_unavailable) VALUES(100,'OTHER','finished_product',1,'半成品',0,31,32,202,'prod',2,'',0,2,'test',0,2,19,'pass',false);SELECT setval(pg_get_serial_sequence('%[1]s.stock_batches','id'),100,true)`, schema))
	if err != nil {
		t.Fatal(err)
	}
	other, err := splitCustomerProcessingFinishedBatchForIssueTx(ctx, tx, schema, customerProcessingMaterialReservation{ID: 2, ComponentSpecG: 0}, customerProcessingFinishedBatchAllocation{StockBatchID: 100, BatchCode: "OTHER", QtyUnits: 1}, 2, "test")
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT cost_unavailable,owner_customer_id,bom_spec_id,bom_variant_id FROM %s.stock_batches WHERE id=$1`, schema), other.StockBatchID).Scan(&unavailable, &owner, &spec, &variant); err != nil || unavailable || owner != 202 || spec != 31 || variant != 32 {
		t.Fatalf("other child unavailable=%v owner=%d spec=%d variant=%d err=%v", unavailable, owner, spec, variant, err)
	}
}
